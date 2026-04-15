# Review: oidc-callbackauthn-provider (Milestone 1)

> **Reviewer**: Reviewer  
> **Date**: 2026-04-15  
> **Branch**: `feature/oidc-callbackauthn-provider`  
> **Verdict**: **APPROVED** (with warnings)

---

## Summary

The OIDC CallbackAuthN provider implementation is solid, well-structured, and closely follows the established `googlecallbackauthn` pattern. All tasks in `tasks.md` are checked. The core acceptance criteria from `docs/phases/phase-1.md` Milestone 1 are met: the interface compiles, `LoginURL` generates a correct authorization URL, `HandleCallback` exchanges a code and returns a `CallbackIdentity`, and unit tests pass against a mock OIDC server. The code is clean, the error handling is thorough, and the test coverage is good.

No CRITICAL findings. Two WARNINGs and three SUGGESTIONs are documented below.

---

## Dimension 1: Completeness

### Tasks (tasks.md)

All 23 task items (sections 1–6) are checked. Verified each against the implementation:

| Section | Status | Notes |
|---|---|---|
| 1. Package Scaffold | Complete | Struct, interface check, constructor all present |
| 2. LoginURL | Complete | Discovery, scopes, redirect path, state, domain type check |
| 3. HandleCallback | Complete | State parse, discovery, code exchange, token verify, claim extraction, email verification, error query param |
| 4. ProviderInfo | Complete | Returns `nil` relay state |
| 5. Test Tooling | Complete | Mock OIDC server, RSA key gen, mock store |
| 6. Unit Tests | Complete | 6.1–6.9 all have corresponding test functions |

### Acceptance Criteria (docs/phases/phase-1.md, Milestone 1)

| Criterion | Met? | Evidence |
|---|---|---|
| `var _ authn.CallbackAuthN = (*AuthN)(nil)` compiles | YES | `authn.go:26` and `authn_test.go:31` |
| `LoginURL()` returns authorization URL with correct params | YES | `TestLoginURL_ReturnsCorrectAuthorizationURL` verifies client_id, redirect_uri, scope (openid, email, profile), response_type, state |
| `HandleCallback()` exchanges code, verifies token, returns `CallbackIdentity` | YES | `TestHandleCallback_Success` — mock token endpoint, verified email/name/groups extraction |
| Unit tests pass with mock OIDC provider | YES | All test functions use `httptest.NewServer` mock provider |

### Spec Requirements (specs/oidc-callback-authn/spec.md)

| Requirement | Met? | Notes |
|---|---|---|
| Interface compliance | YES | |
| Constructor | YES | |
| LoginURL generates OIDC authorization URL | YES | |
| LoginURL rejects non-OIDC domain | YES | `TestLoginURL_RejectsNonOIDCAuthDomain` |
| LoginURL handles issuer alias | YES | `newOIDCProvider` uses `InsecureIssuerURLContext` |
| HandleCallback exchanges code and verifies token | YES | |
| HandleCallback error from provider | YES | `TestHandleCallback_ErrorFromProvider` |
| HandleCallback invalid state | YES | `TestHandleCallback_InvalidState` |
| HandleCallback extracts claims via ClaimMapping (default) | YES | `TestHandleCallback_Success` |
| HandleCallback extracts claims via ClaimMapping (custom) | PARTIAL | See WARNING-1 |
| HandleCallback missing email claim | PARTIAL | See WARNING-2 |
| HandleCallback checks email verification | YES | Two tests: rejected + allowed when skipped |
| HandleCallback handles issuer alias for verification | PARTIAL | See WARNING-1 |
| ProviderInfo returns nil relay state | YES | `TestProviderInfo_ReturnsNilRelayState` |

---

## Dimension 2: Correctness

### WARNING-1: Missing `SkipIssuerCheck` when `IssuerAlias` is set

**Severity**: WARNING

**Location**: `pkg/authn/callbackauthn/oidccallbackauthn/authn.go:112`

**Issue**: The spec (`spec.md:86-90`) and design doc (`design.md:45-46`) both state that when `OIDCConfig.IssuerAlias` is non-empty, the token verifier SHOULD be created with `SkipIssuerCheck: true`. The current implementation creates the verifier identically regardless of whether `IssuerAlias` is set:

```go
verifier := oidcProvider.Verifier(&oidc.Config{ClientID: oidcConfig.ClientID})
```

Expected (per design doc D3):

```go
verifierConfig := &oidc.Config{ClientID: oidcConfig.ClientID}
if oidcConfig.IssuerAlias != "" {
    verifierConfig.SkipIssuerCheck = true
}
verifier := oidcProvider.Verifier(verifierConfig)
```

**Why the test passes anyway**: The test's mock OIDC server sets its discovery `issuer` field to the alias value, so `go-oidc`'s internal provider issuer becomes the alias. The test token's `iss` claim also uses the alias. They match, so verification succeeds without `SkipIssuerCheck`. However, in a real-world Azure Entra deployment, there are configurations where the token's `iss` claim may differ from the discovery document's `issuer` field (e.g., v1 vs v2 token formats), and `SkipIssuerCheck` would be needed.

**Impact**: Does not block archive because the most common Entra configuration works correctly (the test proves this). However, some Entra tenant configurations may fail at token verification time. Recommend fixing before Milestone 3 (real Entra testing).

### WARNING-2: Missing test coverage for two spec scenarios

**Severity**: WARNING

**Location**: `pkg/authn/callbackauthn/oidccallbackauthn/authn_test.go`

**Issue**: Two scenarios from the spec lack dedicated test coverage:

1. **Custom claim mapping** (`spec.md:67-69`): The spec requires a scenario where `ClaimMapping.Email` is set to `"preferred_email"` and the token uses that custom key. No test exercises custom `ClaimMapping` values — all tests use the defaults. The implementation code at `authn.go:126-152` correctly reads from `claimMapping`, so this is a test gap, not a code bug.

2. **Missing email claim** (`spec.md:72-73`): The spec requires a scenario where the mapped email claim is absent from the token. The implementation handles this at `authn.go:128-131` by checking for empty email and returning an error. No test covers this path.

**Impact**: Does not block archive. The code paths are correct by inspection. However, these tests would provide regression safety for claim extraction logic, which is a security-sensitive area.

### Correctness of core logic

- **Token exchange**: Correctly injects custom HTTP client via `oauth2.HTTPClient` context key for both discovery (`authn.go:164`) and token exchange (`authn.go:94`). This is an improvement over the Google provider which doesn't inject for exchange.
- **Email verification**: Three-way handling is correct: absent claim rejected (`authn.go:135-137`), `false` rejected (`authn.go:139-141`), skip when configured (`authn.go:133`). This is more thorough than the Google provider which only checks `bool`.
- **Group extraction**: `extractGroups` handles `[]interface{}`, `[]string`, and missing key correctly. Non-string elements are converted via `fmt.Sprintf("%v", item)` which is reasonable.
- **Error handling**: Consistent pattern of logging then returning typed errors. `oauth2.RetrieveError` is unwrapped for better error messages at `authn.go:97-101`.
- **State parameter**: Correctly uses `authtypes.NewState`/`NewStateFromString` for round-tripping domain ID and site URL.

### Security review

- No SQL injection, XSS, or command injection vectors.
- Token verification uses JWKS from discovery (not hardcoded keys).
- Client secret is passed via `oauth2.Config` (standard library handling).
- `InsecureIssuerURLContext` is correctly gated on `IssuerAlias != ""` — not applied unconditionally.
- Email is validated via `valuer.NewEmail()` before being returned in the identity.

---

## Dimension 3: Coherence

### Pattern consistency with googlecallbackauthn

| Aspect | Google Provider | OIDC Provider | Consistent? |
|---|---|---|---|
| Struct fields | `store`, `settings`, `httpClient` | `store`, `settings`, `httpClient` | YES |
| Constructor signature | `New(ctx, store, providerSettings)` | `New(ctx, store, providerSettings)` | YES |
| Scoped provider settings | Module path string | Module path string | YES |
| `oauth2Config` helper | Takes `siteURL, authDomain, provider` | Takes `siteURL, oidcConfig, provider` | YES — minor signature change appropriate since OIDC doesn't need full authDomain |
| Error handling pattern | Log + typed error | Log + typed error | YES |
| `ProviderInfo` | Returns `nil` relay state | Returns `nil` relay state | YES |
| Domain type validation | In `LoginURL` | In `LoginURL` | YES |

### SUGGESTION-1: `oauth2Config` signature differs slightly from Google provider

**Severity**: SUGGESTION

The Google provider's `oauth2Config` takes `(siteURL, authDomain, provider)` while the OIDC provider takes `(siteURL, oidcConfig, provider)`. The OIDC version is arguably cleaner (passes only what's needed), but the inconsistency is notable. This is fine as-is — just documenting the divergence.

### SUGGESTION-2: `email_verified` handling is more defensive than Google's

**Severity**: SUGGESTION

The OIDC provider (`authn.go:133-142`) handles three cases for `email_verified`: absent, `false`, and non-boolean. The Google provider only checks `claims.EmailVerified` as a bool. The OIDC provider's approach is more robust for diverse OIDC providers where `email_verified` might be absent (e.g., Azure Entra doesn't always include it). This is a good divergence — noting it for documentation purposes.

### SUGGESTION-3: Consider adding `email_verified` claim absence test to Google provider too

**Severity**: SUGGESTION

The `TestHandleCallback_MissingEmailVerifiedClaimRejected` test covers a real-world scenario (Entra not including `email_verified`). The Google provider could benefit from similar handling, but that's outside this change's scope.

### Architecture alignment

- Package location `pkg/authn/callbackauthn/oidccallbackauthn/` follows the established pattern (`pkg/authn/callbackauthn/googlecallbackauthn/`).
- Dependencies are limited to existing packages — no new `go.mod` entries required.
- No changes to existing interfaces, types, or modules.
- The implementation correctly defers provider registration to Milestone 2 as specified.

---

## Findings Summary

| # | Severity | Finding | Location |
|---|---|---|---|
| W-1 | WARNING | Missing `SkipIssuerCheck: true` when `IssuerAlias` is set | `authn.go:112` |
| W-2 | WARNING | Missing tests for custom claim mapping and absent email claim | `authn_test.go` |
| S-1 | SUGGESTION | `oauth2Config` signature differs slightly from Google pattern | `authn.go:178` |
| S-2 | SUGGESTION | `email_verified` handling is more defensive than Google — good divergence | `authn.go:133-142` |
| S-3 | SUGGESTION | Consider porting absent `email_verified` test to Google provider | Out of scope |

**CRITICAL**: 0  
**WARNING**: 2  
**SUGGESTION**: 3

---

## Verdict

**APPROVED** — The implementation meets all Milestone 1 acceptance criteria. The two warnings are real gaps but do not block archive: W-1 affects only edge-case Entra configurations (the common case works), and W-2 is a test coverage gap where the code is correct by inspection. Both should be addressed before Milestone 3's real-world Entra testing.
