# Architecture — Azure Entra ID SSO Adapter for SigNoz Community Edition

> **Status**: Draft  
> **Date**: 2026-04-15  
> **Audience**: Developers, reviewers, and operators of the Entra SSO adapter

---

## 1. Overview

This document describes how the Azure Entra ID SSO adapter integrates with SigNoz's existing community-edition (MIT-licensed) authentication system. The adapter adds Entra ID as a new OIDC-based authentication path alongside the existing email/password and Google SSO mechanisms. It does not replace or modify the existing auth system — it augments it by implementing the same `CallbackAuthN` interface that the Google provider already uses.

### 1.1 Design Principles

- **Augment, don't replace** — the existing auth system continues to work unchanged.
- **Follow existing patterns** — the Google CallbackAuthN implementation is the template.
- **Minimal surface** — one new package, environment-variable configuration, no database schema changes.
- **No enterprise code** — nothing under `ee/` or `cmd/enterprise/` is read, referenced, or used.

---

## 2. Technology Stack

| Layer | Technology | Notes |
|---|---|---|
| Backend | Go 1.25 | SigNoz query-service |
| OIDC library | `coreos/go-oidc/v3` | Already in `go.mod` — handles OIDC discovery, token verification |
| OAuth2 | `golang.org/x/oauth2` | Already in `go.mod` — handles authorization code flow |
| JWT | `golang-jwt/jwt/v5` | Already in `go.mod` — SigNoz internal session tokens |
| HTTP router | `gorilla/mux` | Already in `go.mod` |
| SQL store | SQLite (default) or PostgreSQL | Via `pkg/sqlstore` — both already supported |
| ORM | `uptrace/bun` | Used throughout SigNoz for DB models |
| Frontend | React | No changes required — existing login redirect flow handles callback-based SSO |
| Identity Provider | Microsoft Entra ID | OIDC Authorization Code flow with PKCE optional |
| Deployment | Docker Compose | Full SigNoz stack + optional PostgreSQL |

### 2.1 Dependencies (Direct, New)

No new Go dependencies are required. The adapter uses:

- `github.com/coreos/go-oidc/v3` (already v3.17.0 in `go.mod`)
- `golang.org/x/oauth2` (already in `go.mod`)

All dependencies are pinned via `go.sum`.

---

## 3. SSO Integration Approach

### 3.1 Existing Auth Architecture

SigNoz's auth system is built around these core abstractions (all in `pkg/`):

```
authn.AuthN (interface)               — marker interface for auth providers
├── authn.PasswordAuthN               — email/password authentication
│   └── emailpasswordauthn.AuthN      — implementation
└── authn.CallbackAuthN               — redirect-based SSO authentication
    └── googlecallbackauthn.AuthN     — Google OIDC implementation
```

**Key interfaces** (`pkg/authn/authn.go`):

```go
type CallbackAuthN interface {
    LoginURL(context.Context, *url.URL, *authtypes.AuthDomain) (string, error)
    HandleCallback(context.Context, url.Values) (*authtypes.CallbackIdentity, error)
    ProviderInfo(context.Context, *authtypes.AuthDomain) *authtypes.AuthNProviderInfo
}
```

**Auth flow orchestration** (`pkg/modules/session/implsession/module.go`):

The `CreateCallbackAuthNSession()` method:
1. Looks up the `CallbackAuthN` provider by `AuthNProvider` type
2. Calls `HandleCallback()` to get a `CallbackIdentity` (name, email, groups, role)
3. Looks up the `AuthDomain` config for role mapping
4. Maps groups/role to a SigNoz managed role via `RoleMapping.NewRoleFromCallbackIdentity()`
5. Calls `GetOrCreateUser()` for JIT provisioning
6. Creates a JWT token via `tokenizer.CreateToken()`
7. Redirects the user back to the frontend with the token

**Existing OIDC infrastructure**:

SigNoz already has:
- `authtypes.AuthNProviderOIDC` constant defined
- `authtypes.OIDCConfig` struct with issuer, clientID, clientSecret, claimMapping
- `AuthDomainConfig` validates OIDC config on unmarshal
- `/api/v1/complete/oidc` callback route registered in `signozapiserver/session.go`
- `CreateSessionByOIDCCallback` handler in `implsession/handler.go`

What's **missing** is the actual `CallbackAuthN` implementation for OIDC.

### 3.2 Integration Point

The adapter implements `authn.CallbackAuthN` and is registered in `pkg/signoz/authn.go`:

```go
// Current state — only email/password and Google:
func NewAuthNs(...) (map[authtypes.AuthNProvider]authn.AuthN, error) {
    return map[authtypes.AuthNProvider]authn.AuthN{
        authtypes.AuthNProviderEmailPassword: emailPasswordAuthN,
        authtypes.AuthNProviderGoogleAuth:    googleCallbackAuthN,
    }, nil
}

// After adapter — add OIDC:
func NewAuthNs(...) (map[authtypes.AuthNProvider]authn.AuthN, error) {
    return map[authtypes.AuthNProvider]authn.AuthN{
        authtypes.AuthNProviderEmailPassword: emailPasswordAuthN,
        authtypes.AuthNProviderGoogleAuth:    googleCallbackAuthN,
        authtypes.AuthNProviderOIDC:          oidcCallbackAuthN,    // NEW
    }, nil
}
```

### 3.3 What Changes, What Doesn't

| Component | Changes? | Details |
|---|---|---|
| `pkg/authn/authn.go` | No | Interface unchanged |
| `pkg/types/authtypes/authn.go` | No | `AuthNProviderOIDC` already defined |
| `pkg/types/authtypes/oidc.go` | No | `OIDCConfig` already defined |
| `pkg/types/authtypes/mapping.go` | No | `RoleMapping` already supports group mappings |
| `pkg/types/authtypes/domain.go` | No | Already validates OIDC config |
| `pkg/modules/session/implsession/` | No | Generic callback handling already works for any `CallbackAuthN` |
| `pkg/apiserver/signozapiserver/session.go` | No | `/api/v1/complete/oidc` route already registered |
| `pkg/signoz/authn.go` | **Yes** | Register the new OIDC `CallbackAuthN` provider |
| `pkg/authn/callbackauthn/oidccallbackauthn/` | **New** | New package implementing `CallbackAuthN` for OIDC |
| `deploy/docker/docker-compose-entra-sso.yaml` | **New** | SSO-specific compose file |

---

## 4. Module Boundaries

### 4.1 New Package

```
pkg/authn/callbackauthn/oidccallbackauthn/
├── authn.go         # CallbackAuthN implementation
└── authn_test.go    # Unit tests
```

This package:
- **Implements**: `authn.CallbackAuthN` interface
- **Depends on**: `pkg/types/authtypes`, `coreos/go-oidc/v3`, `golang.org/x/oauth2`, `pkg/factory`
- **Is depended on by**: `pkg/signoz/authn.go` (provider registration)

### 4.2 Dependency Graph

```
                    ┌─────────────────────────────┐
                    │  pkg/signoz/authn.go         │
                    │  (provider registration)     │
                    └──────────┬──────────────────┘
                               │ imports
            ┌──────────────────┼──────────────────────┐
            │                  │                       │
            ▼                  ▼                       ▼
   emailpasswordauthn   googlecallbackauthn   oidccallbackauthn
   (existing)           (existing)            (NEW)
            │                  │                       │
            └──────────────────┼───────────────────────┘
                               │ all implement
                               ▼
                      authn.CallbackAuthN
                      (or authn.PasswordAuthN)
```

---

## 5. Auth Flow

### 5.1 OIDC Authorization Code Flow (Step-by-Step)

```
User                   SigNoz Frontend      SigNoz Backend          Entra ID
 │                          │                      │                     │
 │  1. Navigate to /        │                      │                     │
 │─────────────────────────>│                      │                     │
 │                          │                      │                     │
 │  2. GET /api/v2/sessions/context?email=user@corp.com&ref=...         │
 │                          │─────────────────────>│                     │
 │                          │                      │                     │
 │                          │  3. Lookup AuthDomain │                     │
 │                          │     for "corp.com"   │                     │
 │                          │     → OIDC config    │                     │
 │                          │                      │                     │
 │                          │  4. Return session context with SSO URL    │
 │                          │<─────────────────────│                     │
 │                          │                      │                     │
 │  5. Redirect to Entra login URL                 │                     │
 │<─────────────────────────│                      │                     │
 │                          │                      │                     │
 │  6. Authenticate with Entra (password, MFA, etc.)                    │
 │─────────────────────────────────────────────────────────────────────>│
 │                          │                      │                     │
 │  7. Entra redirects to /api/v1/complete/oidc?code=...&state=...     │
 │─────────────────────────────────────────────────>│                     │
 │                          │                      │                     │
 │                          │  8. Exchange code for tokens               │
 │                          │                      │────────────────────>│
 │                          │                      │<────────────────────│
 │                          │                      │                     │
 │                          │  9. Verify ID token   │                     │
 │                          │     Extract claims:   │                     │
 │                          │     - email           │                     │
 │                          │     - name            │                     │
 │                          │     - groups          │                     │
 │                          │                      │                     │
 │                          │  10. Map groups → role │                     │
 │                          │      via RoleMapping  │                     │
 │                          │                      │                     │
 │                          │  11. GetOrCreateUser  │                     │
 │                          │      (JIT provision)  │                     │
 │                          │                      │                     │
 │                          │  12. Create JWT token │                     │
 │                          │                      │                     │
 │  13. Redirect to frontend with token in query params                 │
 │<─────────────────────────────────────────────────│                     │
 │                          │                      │                     │
 │  14. Frontend stores token, renders dashboard   │                     │
 │─────────────────────────>│                      │                     │
```

### 5.2 Entra-Specific Details

**Issuer URL**: `https://login.microsoftonline.com/{tenant-id}/v2.0`

**Issuer Alias**: The `OIDCConfig.IssuerAlias` field exists specifically for Azure Entra, whose discovery URL differs from the `iss` claim value. The adapter should set the OIDC verifier's issuer URL to the alias when provided.

**Scopes**: `openid`, `email`, `profile`. Group claims are configured in the Entra App Registration (Token Configuration → Add groups claim), not via scopes.

**Group claims**: Entra includes group object IDs (GUIDs) in the `groups` claim of the ID token. The claim key is configurable via `OIDCConfig.ClaimMapping.Groups` (defaults to `"groups"`).

**Token endpoint**: Standard OIDC token exchange. Entra supports `client_secret_post` and `client_secret_basic`.

---

## 6. Group-to-Role Mapping

The existing `RoleMapping` struct (`pkg/types/authtypes/mapping.go`) handles this:

```go
type RoleMapping struct {
    DefaultRole      string            `json:"defaultRole"`       // e.g., "VIEWER"
    GroupMappings    map[string]string  `json:"groupMappings"`     // group GUID → role
    UseRoleAttribute bool              `json:"useRoleAttribute"`
}
```

### 6.1 Configuration via Environment Variables

Since all Entra config flows through the `AuthDomain` configuration stored in the database, and the vision requires environment-variable-only configuration, the adapter needs a bootstrap mechanism:

**Option A (recommended for this-week prototype)**: On startup, if the environment variables are set, the adapter creates/updates the `AuthDomain` entry in the database automatically. This runs in the SigNoz initialization path.

**Environment variables**:

| Variable | Required | Description |
|---|---|---|
| `SIGNOZ_ENTRA_SSO_ENABLED` | Yes | `true` to enable Entra SSO |
| `SIGNOZ_ENTRA_TENANT_ID` | Yes | Azure Entra tenant ID (GUID) |
| `SIGNOZ_ENTRA_CLIENT_ID` | Yes | App registration client ID |
| `SIGNOZ_ENTRA_CLIENT_SECRET` | Yes | App registration client secret |
| `SIGNOZ_ENTRA_ADMIN_GROUP_ID` | No | Entra group GUID for SigNoz admin role |
| `SIGNOZ_ENTRA_EDITOR_GROUP_ID` | No | Entra group GUID for SigNoz editor role |
| `SIGNOZ_ENTRA_DEFAULT_ROLE` | No | Default role if no group match (default: `VIEWER`) |
| `SIGNOZ_ENTRA_DOMAIN` | Yes | Email domain for SSO (e.g., `synamedia.com`) |

### 6.2 Mapping Logic

The `NewRoleFromCallbackIdentity()` method already implements the correct precedence:

1. If `UseRoleAttribute` is true and a `role` claim exists → use that role directly
2. If `GroupMappings` has entries and user has matching groups → use the highest-privilege match (ADMIN > EDITOR > VIEWER)
3. If `DefaultRole` is set → use that
4. Otherwise → `VIEWER`

For the Entra adapter, group object IDs from the token's `groups` claim are matched against the `GroupMappings` keys.

---

## 7. JIT Provisioning

Just-in-time provisioning is already implemented in the session module's `CreateCallbackAuthNSession()` method (`pkg/modules/session/implsession/module.go:128`):

```go
newUser, err = module.userSetter.GetOrCreateUser(ctx, newUser, user.WithRoleNames([]string{signozManagedRole}))
```

`GetOrCreateUser()` (`pkg/modules/user/impluser/setter.go:663`):
1. Looks up an existing non-deleted user by email and orgID
2. If found with `pending_invite` status → activates the user and grants roles
3. If found and active → returns existing user (no role update on subsequent logins)
4. If not found → creates a new user with the specified role

No changes are needed for JIT provisioning. The OIDC adapter returns a `CallbackIdentity` with name, email, orgID, groups, and state — the session module does the rest.

---

## 8. Deployment Shape

### 8.1 Docker Compose (SSO-specific)

A new `deploy/docker/docker-compose-entra-sso.yaml` extends the base compose file:

```yaml
# deploy/docker/docker-compose-entra-sso.yaml
version: "3"

services:
  signoz:
    environment:
      - SIGNOZ_ENTRA_SSO_ENABLED=${SIGNOZ_ENTRA_SSO_ENABLED:-false}
      - SIGNOZ_ENTRA_TENANT_ID=${SIGNOZ_ENTRA_TENANT_ID}
      - SIGNOZ_ENTRA_CLIENT_ID=${SIGNOZ_ENTRA_CLIENT_ID}
      - SIGNOZ_ENTRA_CLIENT_SECRET=${SIGNOZ_ENTRA_CLIENT_SECRET}
      - SIGNOZ_ENTRA_ADMIN_GROUP_ID=${SIGNOZ_ENTRA_ADMIN_GROUP_ID:-}
      - SIGNOZ_ENTRA_EDITOR_GROUP_ID=${SIGNOZ_ENTRA_EDITOR_GROUP_ID:-}
      - SIGNOZ_ENTRA_DEFAULT_ROLE=${SIGNOZ_ENTRA_DEFAULT_ROLE:-VIEWER}
      - SIGNOZ_ENTRA_DOMAIN=${SIGNOZ_ENTRA_DOMAIN}
```

**Usage**:
```bash
docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml up -d
```

### 8.2 Optional PostgreSQL Override

For teams that prefer PostgreSQL over SQLite:

```yaml
# deploy/docker/docker-compose-postgres.yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: signoz
      POSTGRES_PASSWORD: ${SIGNOZ_PG_PASSWORD:-signoz}
      POSTGRES_DB: signoz
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U signoz"]
      interval: 10s
      timeout: 5s
      retries: 5

  signoz:
    environment:
      - SIGNOZ_SQLSTORE_PROVIDER=postgres
      - SIGNOZ_SQLSTORE_POSTGRES_DSN=postgres://signoz:${SIGNOZ_PG_PASSWORD:-signoz}@postgres:5432/signoz?sslmode=disable
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  postgres-data:
```

### 8.3 Full Stack

The complete deployment includes:
- **signoz** — Query service + frontend (port 8080)
- **clickhouse** — Time-series telemetry store
- **zookeeper-1** — ClickHouse coordination
- **otel-collector** — OpenTelemetry collector (ports 4317, 4318)
- **signoz-telemetrystore-migrator** — ClickHouse schema migrations
- **postgres** (optional) — Replaces SQLite for the SQL store

---

## 9. Configuration

### 9.1 All Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `SIGNOZ_ENTRA_SSO_ENABLED` | Yes | `false` | Master switch for Entra SSO |
| `SIGNOZ_ENTRA_TENANT_ID` | Yes | — | Azure Entra tenant ID (GUID) |
| `SIGNOZ_ENTRA_CLIENT_ID` | Yes | — | App registration client ID (GUID) |
| `SIGNOZ_ENTRA_CLIENT_SECRET` | Yes | — | App registration client secret |
| `SIGNOZ_ENTRA_DOMAIN` | Yes | — | Email domain to match for SSO (e.g., `corp.com`) |
| `SIGNOZ_ENTRA_ADMIN_GROUP_ID` | No | — | Entra group object ID → ADMIN role |
| `SIGNOZ_ENTRA_EDITOR_GROUP_ID` | No | — | Entra group object ID → EDITOR role |
| `SIGNOZ_ENTRA_DEFAULT_ROLE` | No | `VIEWER` | Default SigNoz role when no group matches |
| `SIGNOZ_TOKENIZER_JWT_SECRET` | Yes | — | JWT signing secret (existing SigNoz config) |
| `SIGNOZ_SQLSTORE_PROVIDER` | No | `sqlite` | `sqlite` or `postgres` |
| `SIGNOZ_SQLSTORE_POSTGRES_DSN` | If postgres | — | PostgreSQL connection string |
| `SIGNOZ_SQLSTORE_SQLITE_PATH` | If sqlite | `/var/lib/signoz/signoz.db` | SQLite database path |

### 9.2 Entra App Registration Requirements

In the Azure portal, the operator must:

1. **Register an application** (App Registrations → New registration)
   - Redirect URI: `https://<signoz-host>:8080/api/v1/complete/oidc`
   - Supported account types: Single tenant
2. **Create a client secret** (Certificates & secrets → New client secret)
3. **Configure token claims** (Token configuration → Add groups claim → Security groups)
4. **Assign users** (Enterprise Applications → Users and groups)

---

## 10. Testability

### 10.1 Unit Tests

The OIDC adapter can be tested without a real Entra tenant:

- **Mock OIDC provider**: Use `httptest.NewServer` to serve a mock `.well-known/openid-configuration` and JWKS endpoint. Issue test ID tokens signed with a test RSA key.
- **Interface compliance**: Verify the adapter satisfies `authn.CallbackAuthN` at compile time with `var _ authn.CallbackAuthN = (*AuthN)(nil)`.
- **Claim mapping**: Unit test `RoleMapping.NewRoleFromCallbackIdentity()` — already exists.
- **Token verification**: Test that the adapter correctly validates issuer, audience, and signature.

### 10.2 Integration Tests

- **Full callback flow**: Use a mock OIDC server that returns a valid authorization code. Test the complete `CreateCallbackAuthNSession` path including JIT provisioning.
- **Database**: Tests should use the existing `sqlstoretest` package which provides an in-memory SQLite store.

### 10.3 Manual / E2E Testing

- **With real Entra tenant**: Provide a `.env.example` with placeholder values. The developer configures their own Entra test tenant and runs `docker compose up`.
- **Without Entra**: Document how to use a local OIDC test server like [Dex](https://dexidp.io/) or a simple Go test server that mimics Entra's endpoints.

---

## 11. Observability

### 11.1 Logging

The adapter uses SigNoz's structured logging via `factory.ScopedProviderSettings`:

```go
settings.Logger().InfoContext(ctx, "oidc: user authenticated", slog.String("email", claims.Email))
settings.Logger().ErrorContext(ctx, "oidc: failed to verify token", errors.Attr(err))
```

**Auth events to log** (following the Google adapter's pattern):

| Event | Level | When |
|---|---|---|
| `oidc: user authenticated` | INFO | Successful authentication |
| `oidc: failed to get token` | ERROR | Token exchange failure |
| `oidc: failed to verify token` | ERROR | ID token verification failure |
| `oidc: invalid state` | ERROR | State parameter mismatch |
| `oidc: email is not verified` | ERROR | Unverified email (if check enabled) |
| `oidc: missing or invalid claims` | ERROR | Required claims absent |
| `oidc: unexpected issuer` | ERROR | Token issuer doesn't match config |

### 11.2 Metrics (Future)

No custom metrics for the prototype. SigNoz's existing HTTP middleware already tracks request counts and latencies on the callback endpoint. Future work could add:
- `signoz_sso_login_total` counter (by provider, result)
- `signoz_sso_jit_provision_total` counter

---

## 12. Implementation Plan

### Phase 1 — OIDC CallbackAuthN Implementation (Core)
1. Create `pkg/authn/callbackauthn/oidccallbackauthn/authn.go`
2. Implement `LoginURL()` — construct Entra authorization URL
3. Implement `HandleCallback()` — exchange code, verify token, extract claims
4. Implement `ProviderInfo()` — return nil relay state (standard OIDC)
5. Unit tests with mock OIDC provider

### Phase 2 — Registration & Bootstrap
1. Register the OIDC adapter in `pkg/signoz/authn.go`
2. Create startup bootstrap that reads `SIGNOZ_ENTRA_*` env vars and creates/updates the AuthDomain
3. Integration tests

### Phase 3 — Deployment
1. Create `deploy/docker/docker-compose-entra-sso.yaml`
2. Create `.env.example` with all Entra variables
3. End-to-end test with a real Entra tenant

---

## Appendix A — File Inventory (Existing, Relevant)

| File | Purpose |
|---|---|
| `pkg/authn/authn.go` | `CallbackAuthN` interface definition |
| `pkg/authn/callbackauthn/googlecallbackauthn/authn.go` | Reference implementation (Google OIDC) |
| `pkg/types/authtypes/authn.go` | `AuthNProviderOIDC` constant, `CallbackIdentity` struct |
| `pkg/types/authtypes/oidc.go` | `OIDCConfig` struct (issuer, clientID, clientSecret, claimMapping) |
| `pkg/types/authtypes/domain.go` | `AuthDomain`, `AuthDomainConfig` — stores SSO config per domain |
| `pkg/types/authtypes/mapping.go` | `RoleMapping`, `AttributeMapping` — group-to-role logic |
| `pkg/types/authtypes/role.go` | Managed roles (signoz-admin, signoz-editor, signoz-viewer) |
| `pkg/types/user.go` | `User` struct, `UserStore` interface |
| `pkg/types/role.go` | `Role` type (ADMIN, EDITOR, VIEWER) |
| `pkg/modules/session/implsession/module.go` | `CreateCallbackAuthNSession()` — orchestrates callback flow |
| `pkg/modules/session/implsession/handler.go` | HTTP handlers including `CreateSessionByOIDCCallback` |
| `pkg/modules/user/impluser/setter.go` | `GetOrCreateUser()` — JIT provisioning |
| `pkg/signoz/authn.go` | Provider registration — **the file to modify** |
| `pkg/signoz/signoz.go` | Top-level wiring |
| `pkg/apiserver/signozapiserver/session.go` | Route registration (`/api/v1/complete/oidc`) |
| `deploy/docker/docker-compose.yaml` | Base Docker Compose |
| `pkg/sqlstore/config.go` | SQLite/PostgreSQL config |
| `pkg/tokenizer/jwttokenizer/provider.go` | JWT session token creation |

