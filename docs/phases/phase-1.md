# Phase 1 — Working Entra SSO End-to-End

> **Target**: End of week 2026-04-13 (Constraint C4)  
> **Prerequisite**: Vision and Architecture complete  
> **Outcome**: `docker compose up` → Entra login → SigNoz dashboard with correct role

---

## Milestone 1 — OIDC CallbackAuthN Provider

### Capability

Create the `pkg/authn/callbackauthn/oidccallbackauthn/` package that implements `authn.CallbackAuthN`. The provider handles three responsibilities: generating the Entra authorization URL (including tenant-specific issuer, client ID, scopes, redirect URI, and state parameter), exchanging the authorization code for tokens and verifying the ID token via OIDC discovery, and extracting user claims (email, name, groups) into a `CallbackIdentity`. It uses `coreos/go-oidc/v3` for discovery and token verification and `golang.org/x/oauth2` for the code exchange — both already in `go.mod`. The implementation follows the `googlecallbackauthn` package as its structural template and handles the Entra-specific issuer-alias quirk via the existing `OIDCConfig.IssuerAlias` field.

### Acceptance Criteria

We know this is done when:

- `var _ authn.CallbackAuthN = (*oidccallbackauthn.AuthN)(nil)` compiles — the provider satisfies the interface.
- `LoginURL()` returns a URL pointing at `https://login.microsoftonline.com/{tenant}/v2.0/authorize` with correct `client_id`, `redirect_uri`, `scope`, and `state` parameters.
- `HandleCallback()` exchanges a code with a mock OIDC server, verifies the ID token, and returns a `CallbackIdentity` with email, name, and groups populated from token claims.
- Unit tests pass using `httptest.NewServer` as a mock OIDC provider (mock discovery document, mock JWKS, test-signed ID tokens).

---

## Milestone 2 — Registration, Bootstrap, and Role Mapping

### Capability

Register the OIDC provider in `pkg/signoz/authn.go` so that SigNoz's session module can route OIDC callbacks to it. Implement a startup bootstrap that reads `SIGNOZ_ENTRA_*` environment variables and creates or updates the `AuthDomain` entry in the database with the corresponding `OIDCConfig` and `RoleMapping`. This makes the full auth flow work end-to-end: the session module looks up the `AuthDomain` for the user's email domain, finds the OIDC provider config, calls `LoginURL` / `HandleCallback`, maps Entra group GUIDs to SigNoz roles (ADMIN, EDITOR, VIEWER) via `RoleMapping.NewRoleFromCallbackIdentity()`, and calls `GetOrCreateUser()` for JIT provisioning. No changes to the session module, role-mapping logic, or JIT provisioning code are needed — the existing framework handles it.

### Acceptance Criteria

We know this is done when:

- `pkg/signoz/authn.go` registers `authtypes.AuthNProviderOIDC` → `oidccallbackauthn` in the provider map.
- On startup with `SIGNOZ_ENTRA_SSO_ENABLED=true` and required env vars set, an `AuthDomain` row exists in the database for the configured domain with correct `OIDCConfig` (issuer, client ID, client secret, claim mapping) and `RoleMapping` (group GUIDs → roles, default role).
- An integration test exercises the full `CreateCallbackAuthNSession` path: mock OIDC provider → code exchange → token verification → group-to-role mapping → JIT user creation → JWT session token returned.
- A user in the configured admin group gets the ADMIN role; a user with no matching group gets the configured default role (VIEWER).

---

## Milestone 3 — Docker Compose and Operator Configuration

### Capability

Create `deploy/docker/docker-compose-entra-sso.yaml` as a Compose overlay that injects `SIGNOZ_ENTRA_*` environment variables into the SigNoz query-service container. Provide a `.env.example` documenting every variable with placeholder values and inline comments. Optionally provide a `docker-compose-postgres.yaml` overlay for teams that want PostgreSQL instead of the default SQLite. The operator experience is: copy `.env.example` to `.env`, fill in Entra tenant details and group GUIDs, run `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml up -d`.

### Acceptance Criteria

We know this is done when:

- `deploy/docker/docker-compose-entra-sso.yaml` exists and is valid YAML that extends the base compose file with all `SIGNOZ_ENTRA_*` environment variables.
- `.env.example` exists at the repository root (or `deploy/docker/`) with all required and optional variables documented.
- `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` succeeds (compose config validation).
- End-to-end test with a real Entra tenant: operator fills in `.env`, runs compose up, navigates to SigNoz, is redirected to Entra login, authenticates, and lands on the SigNoz dashboard with the correct role.
