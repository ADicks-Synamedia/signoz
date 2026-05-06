## 1. OpenSpec Artifacts

- [x] 1.1 Draft `proposal.md` with Why / What Changes / Capabilities / Impact
- [x] 1.2 Draft `design.md` capturing D1–D5 (retry strategy, call-site relocation, cleanup, test seam, env scope)
- [x] 1.3 Draft spec deltas under `specs/oidc-registration-bootstrap/spec.md` and `specs/docker-compose-entra-sso/spec.md`

## 2. Bootstrap Retry Logic

- [x] 2.1 Replace the no-org skip in `pkg/signoz/entrabootstrap.go:45-48` with a retry loop:
  - Use `context.WithTimeout(ctx, bootstrapOrgWaitTimeout)` with `bootstrapOrgWaitTimeout = 90 * time.Second` (package const)
  - Poll via `time.NewTicker(bootstrapOrgPollInterval)` with `bootstrapOrgPollInterval = 2 * time.Second` (package const)
  - On each tick, re-call `orgGetter.ListByOwnedKeyRange(ctx)`; break on first non-empty result
  - On `ctx.Done()` from timeout, return an error naming `SIGNOZ_USER_ROOT_EMAIL` and `SIGNOZ_USER_ROOT_PASSWORD`
- [x] 2.2 Add a comment on `bootstrapOrgWaitTimeout` referencing `pkg/modules/user/impluser/service.go:58` (the 10s ticker that the budget is sized against)
- [x] 2.3 Refactor signature so timeout and poll interval are injectable for tests (production callers get the package defaults via a thin wrapper that uses the consts)

## 3. Startup Wiring

- [x] 3.1 Remove the `BootstrapEntraSSO` call at `pkg/signoz/signoz.go:397`
- [x] 3.2 Stash the dependencies (`authDomainStore`, `orgGetter`, logger) needed for bootstrap as fields on `*SigNoz` (or capture them inside `New` and bind to a closure exposed via the new method)
- [x] 3.3 Add public method `(*SigNoz).BootstrapEntraSSO(ctx context.Context) error` that delegates to the package-level `BootstrapEntraSSO` with the captured deps and default timeout/poll
- [x] 3.4 In `cmd/community/server.go`: insert `signoz.BootstrapEntraSSO(ctx)` between `signoz.Start(ctx)` (current line 127) and `signoz.Wait(ctx)` (current line 129). **Do not reorder** `server.Start` vs `signoz.Start` — keep current ordering.
- [x] 3.5 On bootstrap error in `cmd/community/server.go`: call `server.Stop(ctx)` (best-effort, log any stop error), then `return err` so `runServer` exits non-zero. Verify the registry teardown still happens via context cancellation as the process exits.

## 4. Tests

- [x] 4.1 Extend `mockOrgGetter` so `ListByOwnedKeyRange` can return `nil` for the first N calls and populated slice afterward (counter-based or channel-based)
- [x] 4.2 New test `TestBootstrap_WaitsForOrgAndSucceeds`: 2 empty calls then populated → AuthDomain created, no error, completes well under timeout (use injected `100ms` timeout / `20ms` poll for fast wall-clock)
- [x] 4.3 New test `TestBootstrap_TimesOutWhenNoOrgAppears`: empty forever → returns error containing `SIGNOZ_USER_ROOT_EMAIL` and `SIGNOZ_USER_ROOT_PASSWORD`
- [x] 4.4 Update `TestBootstrap_SkipsWhenNoOrgExists` to reflect new behavior (rename or adjust — `SSO_ENABLED=false` still skips with no wait; the new tests cover the wait paths)
- [x] 4.5 Verify `TestCreateCallbackAuthNSession_FullOIDCFlow` still passes (constructs AuthDomain explicitly, should be unaffected)
- [x] 4.6 Run all `pkg/signoz/` tests via `docker run --rm -v "$(pwd):/workspace" -w /workspace golang:1.25 go test ./pkg/signoz/ -v -count=1`

## 5. Operator Config

- [x] 5.1 Add `COMPOSE_PROJECT_NAME=signoz` as the first variable in `deploy/docker/.env.example` with explanatory comment
- [x] 5.2 Add a `--- Required for SSO-First Deployments ---` section in `.env.example` containing `SIGNOZ_USER_ROOT_ENABLED`, `SIGNOZ_USER_ROOT_EMAIL`, `SIGNOZ_USER_ROOT_PASSWORD`, `SIGNOZ_USER_ROOT_ORG_NAME` with placeholder values and inline comments explaining the break-glass admin role
- [x] 5.3 Forward `SIGNOZ_USER_ROOT_*` env vars in `deploy/docker/docker-compose-entra-sso.yaml` (`environment:` list under the `signoz` service)
- [x] 5.4 `git rm --cached deploy/docker/.env` (preserves the operator's working copy; the file is already in `.gitignore`)

## 6. Operator Guide Updates

- [x] 6.1 In `docs/operator-guide.md`, add a new subsection under `3. SigNoz Deployment` titled "Required: bootstrap admin user" documenting `SIGNOZ_USER_ROOT_*` and the break-glass role
- [x] 6.2 Update `3a. Prepare the Environment File` to mention `COMPOSE_PROJECT_NAME` and clarify that `.env.example` is the complete template
- [x] 6.3 Remove or rewrite any "restart after first signup" instruction (the retry loop obviates it)
- [x] 6.4 In the Environment Variable Reference table (3b), add rows for `SIGNOZ_USER_ROOT_ENABLED/EMAIL/PASSWORD/ORG_NAME` and `COMPOSE_PROJECT_NAME`

## 7. Verification

- [x] 7.1 `docker run --rm -v "$(pwd):/workspace" -w /workspace golang:1.25 go build ./...` — compiles
- [x] 7.2 `docker run --rm -v "$(pwd):/workspace" -w /workspace golang:1.25 go test ./pkg/signoz/ -v -count=1` — all tests pass
- [x] 7.3 `cd deploy/docker && docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` — validates with no errors
- [x] 7.4 Verify `git status` is clean post-implementation, with `deploy/docker/.env` untracked (and ignored)

## 8. Review and Archive

- [ ] 8.1 Request Reviewer via Orchestrator
- [ ] 8.2 Address review feedback (if any)
- [x] 8.3 Archive change via `/opsx:archive`
- [ ] 8.4 Commit changes (separate commits as appropriate; co-author tag included)
- [ ] 8.5 Push branch and open PR
- [ ] 8.6 Send `DONE:` + PR URL + test summary to Orchestrator
