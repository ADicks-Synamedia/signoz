## Why

SigNoz community edition has a `CallbackAuthN` interface and a complete OIDC callback route (`/api/v1/complete/oidc`) already wired up, but no `CallbackAuthN` implementation exists for the OIDC provider type. This means organizations using Azure Entra ID (or any OIDC provider) cannot authenticate via SSO. The `authtypes.AuthNProviderOIDC` constant, `OIDCConfig` struct, and session-module callback handling are all in place — only the provider implementation is missing.

## What Changes

- **New package** `pkg/authn/callbackauthn/oidccallbackauthn/` implementing `authn.CallbackAuthN` for OIDC authorization code flow
- `LoginURL()` constructs the OIDC authorization URL using discovery from `OIDCConfig.Issuer`, with scopes `openid email profile`, correct redirect URI, and state parameter
- `HandleCallback()` exchanges authorization code for tokens, verifies the ID token via OIDC discovery/JWKS, extracts user claims (email, name, groups) using `OIDCConfig.ClaimMapping`, and returns a `CallbackIdentity`
- `ProviderInfo()` returns standard OIDC provider info (no relay state path)
- Handles Azure Entra issuer-alias quirk where the discovery URL differs from the `iss` claim, via `OIDCConfig.IssuerAlias`
- Uses only existing dependencies: `coreos/go-oidc/v3` for discovery and token verification, `golang.org/x/oauth2` for code exchange
- Unit tests with `httptest.NewServer` mock OIDC provider (discovery, JWKS, token endpoint)

## Capabilities

### New Capabilities
- `oidc-callback-authn`: OIDC CallbackAuthN provider that implements the `authn.CallbackAuthN` interface, enabling OIDC-based SSO authentication via the authorization code flow with ID token verification and claim extraction

### Modified Capabilities
<!-- No existing specs to modify — this is the first capability -->

## Impact

- **New code**: `pkg/authn/callbackauthn/oidccallbackauthn/authn.go` and `authn_test.go`
- **No changes to existing interfaces**: `authn.CallbackAuthN`, `OIDCConfig`, `AuthDomain`, `CallbackIdentity` are all used as-is
- **No new dependencies**: `coreos/go-oidc/v3` and `golang.org/x/oauth2` already in `go.mod`
- **No database changes**: uses existing `AuthDomain` and `OIDCConfig` types
- **No API changes**: `/api/v1/complete/oidc` callback route already registered
- **Registration in `pkg/signoz/authn.go`** will be done in a subsequent milestone (Milestone 2) — this milestone focuses only on the provider implementation and tests
