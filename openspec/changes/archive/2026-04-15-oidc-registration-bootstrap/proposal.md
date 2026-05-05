## Why

The OIDC CallbackAuthN provider (Milestone 1) exists but is not wired into SigNoz. Without registration in the provider map, the session module cannot route OIDC callbacks to it. Without a startup bootstrap, operators have no way to configure the Entra SSO AuthDomain via environment variables — they would need to make raw API calls to create the AuthDomain entry.

## What Changes

- **Register OIDC provider** in `pkg/signoz/authn.go`: add `authtypes.AuthNProviderOIDC → oidccallbackauthn.AuthN` to the provider map
- **Add startup bootstrap** in `pkg/signoz/entrabootstrap.go`: reads `SIGNOZ_ENTRA_*` environment variables on startup and creates/updates an `AuthDomain` with `OIDCConfig` and `RoleMapping` in the database
- **Call bootstrap from `signoz.New()`** after migrations and sqlstore initialization

## Capabilities

### New Capabilities
- `oidc-registration-bootstrap`: Registration of the OIDC CallbackAuthN provider and environment-variable-based bootstrap for Entra SSO AuthDomain configuration

### Modified Capabilities

## Impact

- **Modified**: `pkg/signoz/authn.go` — add OIDC provider to the map
- **Modified**: `pkg/signoz/signoz.go` — call bootstrap function after auth initialization
- **New**: `pkg/signoz/entrabootstrap.go` — bootstrap logic reading env vars
- **No changes to interfaces**: uses existing `AuthDomainStore`, `OIDCConfig`, `RoleMapping` types
- **No new dependencies**
