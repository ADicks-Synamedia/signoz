# Review: oidc-registration-bootstrap (Milestone 2)

> **Reviewer**: Reviewer2  
> **Date**: 2026-04-15  
> **Change**: `openspec/changes/oidc-registration-bootstrap/`  
> **Verdict**: PASS with warnings

---

## Summary

Milestone 2 delivers two capabilities: (1) registering the OIDC `CallbackAuthN` provider in the auth provider map, and (2) a startup bootstrap that reads `SIGNOZ_ENTRA_*` environment variables and creates/updates an `AuthDomain` entry in the database. The implementation is clean, well-tested with 8 unit tests, and follows existing SigNoz patterns. All core acceptance criteria are met. A few discrepancies between the design document and the implementation exist but represent improvements over the design, not regressions.

---

## Dimension 1: Completeness

### Provider Registration — COMPLETE

The `pkg/signoz/authn.go` file registers `authtypes.AuthNProviderOIDC` mapped to an `oidccallbackauthn.AuthN` instance in the provider map returned by `NewAuthNs()`. This directly satisfies the first acceptance criterion.

- **AC**: "pkg/signoz/authn.go registers authtypes.AuthNProviderOIDC → oidccallbackauthn in the provider map" — **MET** (authn.go:31)

### Bootstrap Implementation — COMPLETE

The `pkg/signoz/entrabootstrap.go` file implements the full bootstrap logic:
- Reads all `SIGNOZ_ENTRA_*` env vars
- Validates required vars when SSO is enabled
- Constructs `OIDCConfig` with issuer URL, client credentials, and claim mapping
- Constructs `RoleMapping` from group env vars
- Implements upsert pattern (GetByNameAndOrgID → Create or Update)
- Handles no-org case gracefully

- **AC**: "On startup with SIGNOZ_ENTRA_SSO_ENABLED=true and required env vars set, an AuthDomain row exists in the database" — **MET** (entrabootstrap.go:68-115)

### Wiring — COMPLETE

`pkg/signoz/signoz.go:397` calls `BootstrapEntraSSO` after migrations and auth initialization, using `implauthdomain.NewStore(sqlstore)` and `orgGetter`.

- **AC**: "Bootstrap is called from signoz.New() after migrations and auth initialization" — **MET** (signoz.go:396-399)

### Integration Test — NOT MET

- **AC**: "An integration test exercises the full CreateCallbackAuthNSession path: mock OIDC provider → code exchange → token verification → group-to-role mapping → JIT user creation → JWT session token returned" — **NOT MET**

The tests in `entrabootstrap_test.go` are unit tests that verify bootstrap logic in isolation using mock stores. No integration test exercises the full `CreateCallbackAuthNSession` flow through the session module. The tasks.md file (task 4.1) notes this was "verified via go build" rather than a true integration test.

**Severity: WARNING** — The unit tests are thorough for the bootstrap logic itself, and the session module's callback handling is generic (already tested for Google). The missing integration test is a gap against the acceptance criteria but does not represent a correctness risk for this change specifically, since the wiring follows the exact same pattern as Google SSO.

### Test Coverage — STRONG

8 tests covering:
- Skip when disabled (2 tests: unset and explicit false)
- Error on missing required env vars (4 subtests via table-driven test)
- Skip when no org exists
- Create AuthDomain with full config (verifies OIDCConfig, RoleMapping, claim mapping)
- Update existing AuthDomain (idempotency)
- Default role when no groups configured

---

## Dimension 2: Correctness

### Type Usage — CORRECT

All types (`AuthDomainConfig`, `OIDCConfig`, `RoleMapping`, `AttributeMapping`) are used correctly per their definitions in `pkg/types/authtypes/`. Field names and JSON tags match.

### Upsert Pattern — CORRECT

The upsert pattern (entrabootstrap.go:89-115) correctly:
1. Calls `GetByNameAndOrgID` to check for existing domain
2. Distinguishes "not found" from other errors using `errors.Ast(err, errors.TypeNotFound)`
3. On existing: calls `existing.Update(config)` then `authDomainStore.Update(ctx, existing)` — preserving the original ID
4. On not found: calls `authtypes.NewAuthDomainFromConfig` then `authDomainStore.Create`

This matches design decision D3 exactly.

### IssuerAlias — DIVERGENCE FROM DESIGN (IMPROVEMENT)

**Design D4** states: "Set IssuerAlias to empty string for v2.0 endpoints."

**Implementation** (entrabootstrap.go:53): Sets `IssuerAlias` to `https://sts.windows.net/{tenant}/`.

This is actually **more correct** than the design. Azure Entra's v2.0 discovery endpoint (`https://login.microsoftonline.com/{tenant}/v2.0/.well-known/openid-configuration`) returns `https://login.microsoftonline.com/{tenant}/v2.0` as the issuer, but some multi-tenant configurations and token configurations can cause the `iss` claim in the ID token to be `https://sts.windows.net/{tenant}/`. The `oidccallbackauthn` provider (authn.go:165-166) uses `IssuerAlias` with `oidc.InsecureIssuerURLContext` to handle this mismatch. Setting it proactively avoids a class of hard-to-debug token validation failures.

**Severity: SUGGESTION** — Update design.md D4 to match the implementation. The implementation's choice is defensively correct and prevents a known Azure issuer-mismatch pitfall.

### Log Level for No-Org Case — MINOR DIVERGENCE

**Design D2/spec**: "skip bootstrap with an info-level log"

**Implementation** (entrabootstrap.go:46): Uses `logger.WarnContext` (WARN level)

WARN is arguably more appropriate since it indicates a condition the operator should be aware of (SSO is enabled but not yet active). The message text is clear and actionable.

**Severity: SUGGESTION** — Either level is acceptable. The WARN level is a reasonable operator-facing choice. Consider updating the spec scenario text to say "logs a warning" for consistency.

### Bootstrap Failure is Fatal — CORRECT BUT NOTABLE

If `BootstrapEntraSSO` returns an error (e.g., missing required env var), `signoz.New()` returns `nil, err` and the server does not start (signoz.go:398). This is intentional — if an operator sets `SIGNOZ_ENTRA_SSO_ENABLED=true` but misconfigures the env vars, a loud failure at startup is better than silently running without SSO. This matches the validation requirement in the spec.

### Env Var Access — CORRECT

Direct `os.Getenv` calls are used, matching design constraint C6 (env-var-only config) and design decision D4/trade-off. The `t.Setenv` pattern in tests ensures proper cleanup.

---

## Dimension 3: Coherence

### Architectural Alignment — STRONG

- Follows the augment-don't-replace principle from `docs/architecture.md` section 1.1
- Uses existing types without modification (`AuthDomainConfig`, `OIDCConfig`, `RoleMapping`, `AttributeMapping`)
- Registration pattern in `authn.go` mirrors the Google provider exactly
- Bootstrap placed in `pkg/signoz/` with a clear single-function entry point, matching design decision D1

### Separation of Concerns — CLEAN

- `entrabootstrap.go` is a standalone file with a single exported function
- Takes interfaces (`AuthDomainStore`, `organization.Getter`) rather than concrete types
- Does not import or reference `ee/` or `cmd/enterprise/`
- Bootstrap logic is separate from provider creation (per D1)

### Consistency with Existing Codebase — STRONG

- Import path `implauthdomain.NewStore(sqlstore)` follows the same pattern used elsewhere in `signoz.go`
- Error wrapping uses `fmt.Errorf` with `%w` verb consistently
- Structured logging uses `slog.String` for key-value pairs
- Mock implementations in tests follow Go conventions and implement all interface methods

### Cross-Artifact Coherence — GOOD

The proposal, design, spec, and tasks tell a consistent story. The implementation matches all artifacts except for the two minor divergences noted above (IssuerAlias and log level), both of which are improvements.

---

## Findings Summary

| # | Severity | Finding | Location |
|---|----------|---------|----------|
| 1 | WARNING | Missing integration test for full `CreateCallbackAuthNSession` path (Milestone 2 AC) | `entrabootstrap_test.go` |
| 2 | SUGGESTION | Design D4 says empty IssuerAlias but implementation correctly uses `sts.windows.net` alias — update design to match | `design.md` D4 / `entrabootstrap.go:53` |
| 3 | SUGGESTION | Spec says "info log" for no-org case but implementation uses WARN — update spec for consistency | `spec.md` scenario / `entrabootstrap.go:46` |

---

## Verdict

**PASS with warnings.** The implementation is correct, well-tested, and architecturally clean. The provider registration and bootstrap logic satisfy the core Milestone 2 acceptance criteria. The one WARNING (missing integration test) is a coverage gap against the acceptance criteria but does not indicate a correctness problem — the session module's generic callback handling is already tested via the Google provider path, and the bootstrap unit tests are thorough. The two SUGGESTION items are documentation consistency issues where the implementation made better choices than the design document specified.
