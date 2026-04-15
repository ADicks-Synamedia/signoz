# ADR-0001: Entra SSO Integration Approach

**Status**: Accepted  
**Date**: 2026-04-15  
**Deciders**: BossArchitect, Orchestrator, Human stakeholder

## Context

The project requires adding Azure Entra ID single sign-on to SigNoz's community edition. SigNoz already has an authentication system with multiple provider types. The key decision is how the Entra SSO adapter integrates with this existing system.

## Decision

**Implement the Entra SSO adapter as a new `CallbackAuthN` provider using the existing OIDC infrastructure already partially built into SigNoz.**

Specifically:

1. Create a new package `pkg/authn/callbackauthn/oidccallbackauthn/` that implements the `authn.CallbackAuthN` interface
2. Use `coreos/go-oidc/v3` (already in `go.mod`) for OIDC discovery and ID token verification
3. Use `golang.org/x/oauth2` (already in `go.mod`) for the authorization code exchange
4. Register the provider as `authtypes.AuthNProviderOIDC` in `pkg/signoz/authn.go`
5. Leverage the existing `OIDCConfig`, `RoleMapping`, `AuthDomain`, and `CallbackIdentity` types — no new types needed
6. Use a startup bootstrap to seed `AuthDomain` configuration from `SIGNOZ_ENTRA_*` environment variables
7. Rely on the existing `CreateCallbackAuthNSession()` flow for JIT provisioning and role assignment

## Alternatives Considered

### Alternative A: Use Microsoft's MSAL Go SDK

**Description**: Use `github.com/AzureAD/microsoft-authentication-library-for-go` (MSAL) directly for token acquisition and validation.

**Rejected because**:
- MSAL Go is designed for Azure SDK integration scenarios, not standalone OIDC relying party implementations
- `go-oidc` is already used by the existing Google SSO provider, providing a consistent implementation pattern
- MSAL would add a new dependency when the required libraries are already present
- The OIDC standard protocol is sufficient — we don't need Azure-specific token features

### Alternative B: Build a Standalone Auth Service

**Description**: Create a separate microservice that handles Entra SSO and communicates with SigNoz via an internal API.

**Rejected because**:
- Adds deployment complexity (another container, another network hop)
- SigNoz already has clean auth interfaces designed for multiple providers
- Would require an internal API protocol that doesn't exist today
- Contradicts the "single `docker compose up`" deployment goal

### Alternative C: Modify the Enterprise Edition's SSO Code

**Description**: Adapt the enterprise edition's existing SSO implementation for the community edition.

**Rejected because**:
- The `ee/` directory is under the SigNoz Enterprise License — we cannot read, reference, or use this code (Constraint C1)
- This is a hard licensing constraint, not a preference

### Alternative D: Generic OIDC Provider (Not Entra-Specific)

**Description**: Build a fully generic OIDC provider that works with any identity provider, not just Entra.

**Rejected because**:
- The vision explicitly scopes this to Entra ID only (Non-Goal NG2)
- A generic provider adds complexity for edge cases across different IdPs
- The existing `OIDCConfig` type is already somewhat generic — the adapter can be made more generic later if needed
- This-week delivery timeline favors focused scope

However, the implementation naturally ends up fairly generic since it uses standard OIDC. The Entra-specific parts are limited to: (a) issuer URL format, (b) group claim handling, and (c) the bootstrap mechanism for env var configuration. Generalizing later would be straightforward.

## Consequences

### Positive

- **Zero changes to existing auth interfaces** — the `CallbackAuthN` interface, session module, and role mapping logic are reused unchanged
- **No new Go dependencies** — `go-oidc` and `oauth2` are already in the module
- **No database schema changes** — `AuthDomain` and `OIDCConfig` types already exist with the needed fields
- **Route already registered** — `/api/v1/complete/oidc` callback handler exists
- **Consistent implementation** — follows the same pattern as the Google SSO provider
- **JIT provisioning for free** — `GetOrCreateUser()` already handles first-login account creation

### Negative

- **Env var bootstrap is non-standard** — SigNoz normally stores AuthDomain config via API. The env var bootstrap is adapter-specific code
- **Single-tenant limitation** — the bootstrap assumes one Entra tenant, one SigNoz instance. Multi-tenant would need a different config approach
- **No group overage handling** — if a user is in >150 Entra groups, the groups claim is replaced with an overage indicator. Handling this requires a Microsoft Graph API call, which is out of scope for the prototype

### Risks

- **Entra issuer mismatch** — Entra's `iss` claim format may differ from the discovery URL. Mitigated by `OIDCConfig.IssuerAlias` field (already exists)
- **Group claim format** — Entra sends group object IDs (GUIDs), not display names. Operators must use GUIDs in env var configuration. This needs clear documentation
