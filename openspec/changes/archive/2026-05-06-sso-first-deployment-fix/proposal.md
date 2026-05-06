## Why

The `oidc-registration-bootstrap` capability shipped with a known gap: when `BootstrapEntraSSO` runs at startup and no organization exists yet (`pkg/signoz/entrabootstrap.go:45-48`), it logs a warning and returns `nil`. On a stock CE deployment, the first org is created either via UI self-registration or via the `SIGNOZ_USER_ROOT_*` reconciler. In an SSO-first deployment, the UI signup path is unreachable and the root-user reconciler is disabled by default — so no org is ever created, `BootstrapEntraSSO` permanently no-ops, and the OpAMP server rejects every collector with `cannot create agent without orgId` (`pkg/query-service/app/opamp/opamp_server.go:103-136`).

A regression also slipped past the previous merge: `COMPOSE_PROJECT_NAME=signoz` was added to `deploy/docker/.env.example` to keep container/volume/network prefixes consistent, but the line never reached `main`.

Finally, `deploy/docker/.env` is currently tracked in the repository even though `.env` is ignored via `.gitignore`. Tracking the operator's secrets file is wrong: it shows up in `git status` after every operator edit, risks leaking secrets, and conflicts with the canonical "operators copy `.env.example` to `.env`" workflow.

## What Changes

- **Operator-facing config**: document and template the `SIGNOZ_USER_ROOT_*` env vars in `deploy/docker/.env.example` and `deploy/docker/docker-compose-entra-sso.yaml`. Operators of SSO-first deployments are expected to set them so the user reconciler creates the first org and a break-glass admin.
- **`BootstrapEntraSSO` waits for the first org instead of giving up**. Replace the skip-on-no-org branch with a bounded retry loop driven by a `context.WithTimeout`: poll `orgGetter.ListByOwnedKeyRange()` every 2s until either an org is found or 90s elapses. On timeout, return a non-nil error naming the `SIGNOZ_USER_ROOT_*` vars so operators can self-diagnose.
- **Move bootstrap call out of `signoz.New`** so it can run after the registry has started the user reconciler. Add a public `BootstrapEntraSSO(ctx)` method on `*SigNoz` and invoke it from `cmd/community/server.go` between `signoz.Start(ctx)` (where the user reconciler begins ticking) and `server.Start(ctx)` (where HTTP traffic begins). Reorder so HTTP traffic only flows once the AuthDomain exists.
- **Restore `COMPOSE_PROJECT_NAME=signoz`** in `.env.example`.
- **Untrack `deploy/docker/.env`** via `git rm`. The file is already gitignored; removing it from the index aligns repo state with the documented `.env.example → .env` workflow.
- **Update `docs/operator-guide.md`**: require `SIGNOZ_USER_ROOT_*`, explain the break-glass admin role, clarify the `.env.example → .env` copy workflow, and remove the "restart after first signup" workaround that the retry loop obviates.
- **Tests**: extend `pkg/signoz/entrabootstrap_test.go` to cover the retry success path (org appears mid-poll) and the retry-exhaustion failure path. Update the existing `TestBootstrap_SkipsWhenNoOrgExists` case to reflect the new behavior.

## Capabilities

### Modified Capabilities
- `oidc-registration-bootstrap`: bootstrap waits for the first org rather than skipping; bootstrap is invoked from the cmd-level startup sequence after the user reconciler is running.
- `docker-compose-entra-sso`: `.env.example` documents `SIGNOZ_USER_ROOT_*` and `COMPOSE_PROJECT_NAME`; compose overlay forwards `SIGNOZ_USER_ROOT_*` to the signoz service; `deploy/docker/.env` is no longer tracked.

### New Capabilities
None.

## Impact

- **Modified**: `pkg/signoz/entrabootstrap.go` — replace skip-on-no-org with retry loop and `context.WithTimeout(90s)` budget
- **Modified**: `pkg/signoz/signoz.go` — remove inline `BootstrapEntraSSO` call at line 397; expose dependencies on the `SigNoz` struct so a new method can call into them
- **Modified**: `cmd/community/server.go` — call `signoz.BootstrapEntraSSO(ctx)` between `signoz.Start(ctx)` and `server.Start(ctx)`
- **Modified**: `pkg/signoz/entrabootstrap_test.go` — cover retry success and timeout via injectable timeout/poll
- **Modified**: `pkg/signoz/entrabootstrap_integration_test.go` — confirm bootstrap → callback path still passes
- **Modified**: `deploy/docker/.env.example` — add `COMPOSE_PROJECT_NAME=signoz` and `SIGNOZ_USER_ROOT_*` block
- **Modified**: `deploy/docker/docker-compose-entra-sso.yaml` — forward `SIGNOZ_USER_ROOT_*` to the signoz container
- **Removed (from index)**: `deploy/docker/.env` — `git rm` so this gitignored file stops being tracked
- **Modified**: `docs/operator-guide.md` — require root-user vars, document break-glass admin, simplify first-boot instructions
- **No new dependencies**
