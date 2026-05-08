## Context

The original `oidc-registration-bootstrap` change shipped with a no-org skip path (`pkg/signoz/entrabootstrap.go:45-48`) intended for development scenarios where someone might enable SSO before signing up a first user. In an **SSO-first** production deployment that path is the bug: there is no UI signup to fall back to, so once `BootstrapEntraSSO` returns `nil` on a fresh database the AuthDomain is never created. Every collector then fails with `cannot create agent without orgId` (`pkg/query-service/app/opamp/opamp_server.go:103-136`).

The standard SigNoz CE escape hatch is `SIGNOZ_USER_ROOT_*` (`pkg/modules/user/config.go:17-22`). The user reconciler (`pkg/modules/user/impluser/service.go:51-79`) ticks every 10 seconds; on its first successful call to `CreateFirstUser` it creates the organization and a root admin user, then hangs on `<-s.stopC`. So if `BootstrapEntraSSO` waits long enough, it will see the org appear — provided the wait runs *after* the reconciler has started ticking.

## Goals / Non-Goals

**Goals:**
- `BootstrapEntraSSO` succeeds on a fresh database when `SIGNOZ_USER_ROOT_*` is configured.
- Failure mode is loud: if no org appears within budget, the operator gets an error pointing at `SIGNOZ_USER_ROOT_*` and the process exits non-zero.
- Fix the missing `COMPOSE_PROJECT_NAME=signoz` in `.env.example`.
- Untrack `deploy/docker/.env` (already gitignored) so operator-edited files no longer pollute `git status`.
- Update the operator guide.

**Non-Goals:**
- Generalizing the bootstrap to other SSO providers.
- Reworking the user reconciler.
- Multi-org deployments.

## Decisions

### D1: Bounded retry loop, linear poll, fail-loud on timeout

When `BootstrapEntraSSO` finds zero orgs, poll once every 2 seconds via `time.NewTicker` inside a `select` on `ctx.Done()`. The total budget is enforced by `context.WithTimeout(parent, 90*time.Second)` derived from the parent startup context. Linear because the user reconciler ticks at a fixed 10s interval and we just need to outlast it; no jitter (single-process, single-waiter, no thundering-herd risk). A ticker (not `time.Sleep`) so cancellation via `ctx.Done()` is immediate.

We chose **fail-loud over warn-and-continue** because partial bring-up was the original bug — the warn-and-skip code at `entrabootstrap.go:45-48` is exactly what masked the misconfiguration in production. Returning a non-nil error and aborting startup gives operators the cheapest possible signal: they see one log line at the deploy console.

The error message names the actionable env vars verbatim: `"timed out after 90s waiting for first organization to be created; for SSO-first deployments set SIGNOZ_USER_ROOT_EMAIL and SIGNOZ_USER_ROOT_PASSWORD so the root user reconciler creates the bootstrap org"`.

The 90s budget is ~9 reconciler ticks: comfortable headroom on a slow first-boot DB without making operators wait minutes. Both timeout and poll interval are package-level named constants (`bootstrapOrgWaitTimeout`, `bootstrapOrgPollInterval`) so they're discoverable and tunable. A doc comment near `bootstrapOrgWaitTimeout` references `pkg/modules/user/impluser/service.go:58` so a future reader knows why 90s and not 30s.

### D2: Lift the bootstrap call out of `signoz.New`

The bootstrap **cannot** run synchronously inside `signoz.New` because the user reconciler hasn't started yet at that point. Reading `cmd/community/server.go:67-129`:

```
signoz, err := signoz.New(...)        // line 67  — bootstrap currently runs HERE
server.Start(ctx)                      // line 122 — HTTP traffic begins
signoz.Start(ctx)                      // line 127 — registry + userService start
signoz.Wait(ctx)                       // line 129 — block until services exit
```

`signoz.Start(ctx)` calls `Registry.Start` (`pkg/factory/registry.go:81-133`), which spawns a goroutine per registered service — including `userService`, registered at `signoz.go:500`. The reconciler does not begin its 10s ticker until *after* `signoz.New` returns. A synchronous wait inside `signoz.New` would block on a condition that no concurrent code can satisfy.

**Resolution**: remove the call from `signoz.go:397`, expose a public method `(*SigNoz).BootstrapEntraSSO(ctx)` that closes over the necessary dependencies (`authDomainStore`, `orgGetter`, logger), and invoke it from `cmd/community/server.go` between `signoz.Start(ctx)` and `signoz.Wait(ctx)`:

```go
signoz, err := signoz.New(...)             // bootstrap removed from here
...
server, err := app.NewServer(config, signoz)
if err := server.Start(ctx); err != nil { return err }   // unchanged
signoz.Start(ctx)                                         // registry up, reconciler ticking

if err := signoz.BootstrapEntraSSO(ctx); err != nil {     // NEW — synchronous wait, fail-loud
    logger.ErrorContext(ctx, "entra sso bootstrap failed", errors.Attr(err))
    return err
}

if err := signoz.Wait(ctx); err != nil { ... }            // unchanged
```

We deliberately do **not** reorder `server.Start` vs `signoz.Start`. Reasons (per BossArchitect):
1. The existing dependency ordering in the registry (`userService` depends on `authz`, enforced via `healthyC` channels in `registry.go:84-110`) is the right place for "wait for the user reconciler to be ready" logic. The cmd layer should not duplicate that.
2. The "no agent traffic until SSO configured" property the reorder would buy us is weaker than first claimed: OpAMP agents reconnect on failure indefinitely, so a 0–90s window where agents bounce is operationally indistinguishable from the 0–10s window they already bounce during cold start.
3. `server.NewServer` and `server.Start` may have ordering assumptions on registry state that a reorder would silently break. Keeping the existing order is the safer change.

### D3: Cleanup on bootstrap failure

Returning `err` from `runServer` after `signoz.Start(ctx)` has spawned goroutines means the existing teardown path (`server.Stop` and registry stop in `signoz.Wait`'s normal exit) doesn't run. We address this with explicit `defer`-style cleanup in `cmd/community/server.go` so a bootstrap failure triggers `server.Stop(ctx)` and lets the registry's contexts cancel naturally as the process exits. The cleanup pattern: capture the bootstrap error, call `server.Stop`, then return the bootstrap error.

### D4: Test coverage strategy

The existing unit tests use a `mockOrgGetter` that returns a fixed slice. To exercise the retry path, extend `mockOrgGetter` so its `ListByOwnedKeyRange` impl can return `nil` for the first N calls and the populated slice afterward. Two new tests:

- `TestBootstrap_WaitsForOrgAndSucceeds` — org slice empty for first 2 polls then populated → AuthDomain created on 3rd attempt without exceeding the timeout.
- `TestBootstrap_TimesOutWhenNoOrgAppears` — org slice stays empty → non-nil error after timeout, error message contains both `SIGNOZ_USER_ROOT_EMAIL` and `SIGNOZ_USER_ROOT_PASSWORD`.

To keep tests fast (sub-second), `BootstrapEntraSSO` accepts injectable timeout and poll-interval parameters with package-level defaults used by production callers. Tests pass `100*time.Millisecond` / `20*time.Millisecond` so the full retry-exhaustion case completes in ~100ms wall-clock.

We also retain `TestBootstrap_SkipsWhenNoOrgExists` semantically — under the new behavior with `SSO_ENABLED=false` no wait is performed (still skip-fast), and under `SSO_ENABLED=true` the wait + timeout path is what's tested instead.

### D5: `COMPOSE_PROJECT_NAME=signoz` and `.env` untracking

`COMPOSE_PROJECT_NAME=signoz` keeps Docker container/volume/network prefixes deterministic across `docker compose` invocations. Without it, Compose derives the project name from the working directory, which breaks documentation that references container names like `signoz-signoz-1`. Re-apply as the first variable in `.env.example`.

`deploy/docker/.env` is gitignored (`.gitignore` line 42) but currently tracked. `git rm --cached deploy/docker/.env` removes it from the index without altering the operator's local working copy. After this change, the canonical workflow is:

```
cp deploy/docker/.env.example deploy/docker/.env
# edit .env with real secrets
```

The operator's `.env` is never tracked again because `.gitignore` already covers it.

## Risks / Trade-offs

- **[Risk] 10s reconciler interval is now load-bearing for the 90s budget**: if the user reconciler's tick interval changes upstream, the bootstrap budget needs updating. Mitigation: a comment on `bootstrapOrgWaitTimeout` referencing `pkg/modules/user/impluser/service.go:58`.
- **[Risk] OpAMP traffic during the 0–90s wait**: agents may bounce off the API while the bootstrap is waiting. Acceptable per D2: agents reconnect indefinitely and the existing cold-start window already exhibits the same behavior.
- **[Risk] Naming coupling**: `SigNoz.BootstrapEntraSSO` couples the public API to one SSO provider. If Okta or another provider lands later, rename to `BootstrapAuthDomains(ctx)` and dispatch internally. Not a blocker for this change.
- **[Trade-off] Public method on `*SigNoz` widens the surface area slightly**: acceptable — the method does one specific thing and the alternative (free function with explicit deps) requires plumbing three internal types out of the `SigNoz` struct.
