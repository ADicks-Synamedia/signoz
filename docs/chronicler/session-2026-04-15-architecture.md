# Session note — 2026-04-15 — Architecture

**Main session agent:** BossArchitect  
**Duration (approx):** single session

## What happened

BossArchitect explored the SigNoz community-edition codebase to understand the existing authentication system, then produced the architecture document, contributing guide, and first ADR for the Entra SSO adapter.

### Codebase exploration findings

1. **Auth system is well-structured**: The `authn.CallbackAuthN` interface provides a clean extension point. The Google SSO implementation (`googlecallbackauthn`) serves as a direct template.

2. **OIDC infrastructure already exists**: SigNoz has `AuthNProviderOIDC` constant, `OIDCConfig` struct, OIDC callback route (`/api/v1/complete/oidc`), and handler (`CreateSessionByOIDCCallback`). Only the actual `CallbackAuthN` implementation is missing.

3. **Role mapping is built-in**: The `RoleMapping` type already supports group-to-role mappings with correct precedence logic and the three SigNoz roles (ADMIN, EDITOR, VIEWER).

4. **JIT provisioning is handled**: `GetOrCreateUser()` in the session module already handles first-login account creation with role assignment.

5. **No new dependencies needed**: `coreos/go-oidc/v3` and `golang.org/x/oauth2` are already in `go.mod`.

6. **Database supports both SQLite and PostgreSQL**: The `sqlstore` package supports both, configurable via environment variables.

## Decisions made

- **Integration approach**: Implement a new `CallbackAuthN` provider in `pkg/authn/callbackauthn/oidccallbackauthn/`, following the Google SSO pattern. This requires modifying only `pkg/signoz/authn.go` to register the new provider.

- **No MSAL SDK**: Use `go-oidc` + standard OAuth2 (already in the codebase) instead of Microsoft's MSAL Go library. MSAL is for Azure SDK scenarios, not standalone OIDC.

- **Env var bootstrap**: Create a startup mechanism to seed `AuthDomain` configuration from `SIGNOZ_ENTRA_*` environment variables, since the vision requires env-var-only configuration.

- **No new dependencies**: All required libraries are already in `go.mod`.

- **No database schema changes**: Existing `AuthDomain`, `OIDCConfig`, and `RoleMapping` types have all needed fields.

- **Docker Compose overlay**: Provide an SSO-specific compose overlay file rather than modifying the base compose file.

## Artefacts produced

- `docs/architecture.md` — Full architecture document covering integration approach, auth flow, module boundaries, deployment, configuration, testability, and observability
- `CONTRIBUTING.md` — Developer guide for the Entra SSO adapter (replaces previous SigNoz contributing guide)
- `docs/decisions/0001-entra-sso-integration-approach.md` — ADR documenting the integration approach decision with four alternatives considered
- `docs/chronicler/session-2026-04-15-architecture.md` — This session note

## Key insight

The integration is remarkably straightforward because SigNoz's auth system was designed for multiple providers. The existing OIDC types and callback route mean the adapter is essentially "filling in a gap" — the framework expected an OIDC provider but none had been implemented for the community edition.

## Open questions punted

- **Group overage handling**: When Entra users are in >150 groups, the `groups` claim is replaced with an overage indicator. Handling this requires a Microsoft Graph API call, which is out of scope for the prototype.
- **Role update on subsequent logins**: Currently, `GetOrCreateUser()` only sets roles on first login. Subsequent logins return the existing user without updating roles. This means group membership changes in Entra won't be reflected until the user is deleted and re-provisioned. Flagged as a future enhancement.
- **Multi-org support**: The bootstrap assumes a single organization. If the SigNoz instance has multiple orgs, the bootstrap needs to know which org to configure.

## Next session should

Hand off to implementation — create the `oidccallbackauthn` package, register the provider, create the env var bootstrap, and produce the Docker Compose overlay.
