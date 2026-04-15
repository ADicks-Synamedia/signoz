## Context

SigNoz's community-edition auth system is built around the `authn.CallbackAuthN` interface for redirect-based SSO providers. The Google SSO provider (`googlecallbackauthn`) is the existing reference implementation. The OIDC provider type (`authtypes.AuthNProviderOIDC`) is already defined with a full `OIDCConfig` struct, callback route (`/api/v1/complete/oidc`), and session-module handling — but no `CallbackAuthN` implementation exists for it.

The new package `pkg/authn/callbackauthn/oidccallbackauthn/` fills this gap. It uses the same libraries as the Google provider (`coreos/go-oidc/v3`, `golang.org/x/oauth2`) and follows the same structural pattern.

## Goals / Non-Goals

**Goals:**
- Implement `authn.CallbackAuthN` for OIDC, verified by compile-time interface check
- Generate correct authorization URLs using OIDC discovery
- Exchange authorization codes and verify ID tokens
- Extract user claims (email, name, groups) into `CallbackIdentity` using configurable claim mapping
- Handle Azure Entra's issuer-alias quirk where discovery URL differs from token `iss` claim
- Comprehensive unit tests using a mock OIDC provider

**Non-Goals:**
- Provider registration in `pkg/signoz/authn.go` (Milestone 2)
- Environment-variable bootstrap for `AuthDomain` configuration (Milestone 2)
- Integration tests with the full session module (Milestone 2)
- UserInfo endpoint support (the `GetUserInfo` field exists on `OIDCConfig` but is out of scope for this milestone)
- Group overage handling for Entra (>150 groups requires Microsoft Graph API call)

## Decisions

### D1: Follow googlecallbackauthn as structural template

The new package mirrors `googlecallbackauthn` in structure: same struct fields (`store`, `settings`, `httpClient`), same constructor signature, same helper method pattern (`oauth2Config`). This keeps the codebase consistent and makes code review straightforward.

**Alternative**: Build a more generic OIDC adapter with pluggable claim extractors. Rejected because the existing pattern is simple and works, and the `OIDCConfig.ClaimMapping` already provides the claim extraction flexibility needed.

### D2: Use OIDCConfig.ClaimMapping for dynamic claim extraction

Rather than hardcoding claim field names like the Google provider does (`json:"email"`, `json:"name"`), the OIDC provider reads claim keys from `OIDCConfig.ClaimMapping` (which defaults to `email`, `name`, `groups`, `role`). This is necessary because different OIDC providers use different claim names.

Claims are extracted into a `map[string]interface{}` and then the mapped keys are looked up. Groups may be `[]string` or `[]interface{}` in the raw token claims.

### D3: Issuer alias handling via InsecureIssuerURLContext

Azure Entra's OIDC discovery URL (`https://login.microsoftonline.com/{tenant}/v2.0`) may differ from the `iss` claim in tokens (which can be `https://sts.windows.net/{tenant}/`). The `go-oidc` library's `oidc.NewProvider()` validates that the discovery document's `issuer` field matches the URL it was fetched from.

When `IssuerAlias` is set:
1. Use `oidc.InsecureIssuerURLContext` to create a context that tells `go-oidc` to fetch discovery from `OIDCConfig.Issuer` but accept `IssuerAlias` as the issuer in the discovery document
2. Create the verifier with `SkipIssuerCheck: true` since we've already validated the issuer during discovery

When `IssuerAlias` is empty, standard issuer validation applies.

**Alternative**: Always skip issuer check. Rejected because it weakens security for non-Entra OIDC providers that don't have this quirk.

### D4: Scopes include "openid", "email", "profile"

Unlike the Google provider which uses only `["email", "profile"]`, the OIDC provider includes `"openid"` as required by the OIDC specification. Entra group claims are configured in the Entra App Registration token configuration, not via scopes, so no additional scopes are needed for groups.

### D5: Mock OIDC server for testing

Unit tests use `httptest.NewServer` to serve:
- `/.well-known/openid-configuration` — discovery document with issuer, authorization endpoint, token endpoint, JWKS URI
- `/jwks` — JSON Web Key Set with the test RSA public key
- `/token` — token exchange endpoint that returns a signed ID token

A test helper generates RSA key pairs and signs ID tokens with configurable claims. This allows testing the full `HandleCallback` flow without any external dependencies.

## Risks / Trade-offs

- **[Risk] go-oidc InsecureIssuerURLContext is marked "insecure"** → This is the library's intended mechanism for providers with aliased issuers. The "insecure" prefix means it relaxes standard validation, which is exactly what's needed for Entra. Mitigation: only activated when `IssuerAlias` is explicitly configured.
- **[Risk] Group claim may contain non-string values** → Some OIDC providers return groups as nested objects. Mitigation: parse claims into `map[string]interface{}` and handle `[]interface{}` by converting each element to string.
- **[Trade-off] No caching of OIDC provider/discovery** → Each `LoginURL` and `HandleCallback` call creates a new `oidc.Provider`, which fetches the discovery document. This matches the Google provider's pattern and is acceptable for the prototype. Caching can be added later if needed.

## Test Tooling

The mock OIDC server is implemented as test helpers within `authn_test.go`:
- `newMockOIDCServer()` — starts an `httptest.NewServer` serving discovery, JWKS, and token endpoints
- `newTestRSAKey()` — generates an RSA key pair for signing test tokens
- `newTestIDToken()` — creates and signs a JWT with configurable claims (email, name, groups, issuer, audience, expiry)
- Mock `AuthNStore` — implements `GetAuthDomainFromID` returning a preconfigured `AuthDomain` with `OIDCConfig`
