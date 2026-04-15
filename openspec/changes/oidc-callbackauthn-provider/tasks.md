## 1. Package Scaffold and Interface Compliance

- [x] 1.1 Create `pkg/authn/callbackauthn/oidccallbackauthn/authn.go` with package declaration, imports, and `AuthN` struct (fields: `store`, `settings`, `httpClient`)
- [x] 1.2 Add compile-time interface check `var _ authn.CallbackAuthN = (*AuthN)(nil)`
- [x] 1.3 Implement `New(ctx, store, providerSettings)` constructor following `googlecallbackauthn.New` pattern

## 2. LoginURL Implementation

- [x] 2.1 Implement `LoginURL()` — create OIDC provider via discovery, build `oauth2.Config` with scopes `["openid", "email", "profile"]` and redirect path `/api/v1/complete/oidc`, return `AuthCodeURL` with state
- [x] 2.2 Add issuer-alias handling in `LoginURL()` — when `OIDCConfig.IssuerAlias` is non-empty, use `oidc.InsecureIssuerURLContext` for discovery
- [x] 2.3 Add auth domain type validation — reject non-OIDC auth domains with `ErrCodeAuthDomainMismatch`

## 3. HandleCallback Implementation

- [x] 3.1 Implement `HandleCallback()` — parse state, get AuthDomain from store, create OIDC provider, exchange code, extract raw ID token
- [x] 3.2 Add issuer-alias handling in `HandleCallback()` — use `oidc.InsecureIssuerURLContext` for discovery and `SkipIssuerCheck` in verifier config when `IssuerAlias` is set
- [x] 3.3 Implement claim extraction using `OIDCConfig.ClaimMapping` — extract email, name, groups, role from token claims via mapped keys
- [x] 3.4 Add email verification check — reject unverified emails unless `InsecureSkipEmailVerified` is true
- [x] 3.5 Handle error responses from provider — check for `error` query parameter and log with description

## 4. ProviderInfo Implementation

- [x] 4.1 Implement `ProviderInfo()` — return `&AuthNProviderInfo{RelayStatePath: nil}`

## 5. Test Tooling

- [x] 5.1 Create `authn_test.go` with mock OIDC server (`httptest.NewServer`) serving discovery document, JWKS endpoint, and token exchange endpoint
- [x] 5.2 Create test helper to generate RSA key pair and sign test ID tokens (JWT) with configurable claims
- [x] 5.3 Create mock `AuthNStore` implementation returning preconfigured `AuthDomain` with `OIDCConfig`

## 6. Unit Tests

- [x] 6.1 Test: interface compliance compiles
- [x] 6.2 Test: `LoginURL` returns correct authorization URL with expected parameters
- [x] 6.3 Test: `LoginURL` rejects non-OIDC auth domain
- [x] 6.4 Test: `HandleCallback` succeeds with valid code, returns `CallbackIdentity` with correct email, name, groups
- [x] 6.5 Test: `HandleCallback` returns error when provider returns error query param
- [x] 6.6 Test: `HandleCallback` returns error for invalid state
- [x] 6.7 Test: `HandleCallback` rejects unverified email when `InsecureSkipEmailVerified` is false
- [x] 6.8 Test: `HandleCallback` with issuer alias succeeds
- [x] 6.9 Run `go test ./pkg/authn/callbackauthn/oidccallbackauthn/...` and verify all tests pass
