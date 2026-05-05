## Context

Milestone 1 delivered the `oidccallbackauthn.AuthN` provider. It implements `authn.CallbackAuthN` but is not registered in the provider map. The session module's `CreateCallbackAuthNSession` (line 128 of `implsession/module.go`) looks up providers from this map — without registration, OIDC callbacks fail with "authn provider not found".

The operator also needs a way to seed the `AuthDomain` configuration from environment variables so that `docker compose up` with `SIGNOZ_ENTRA_*` vars is sufficient to enable SSO.

## Goals / Non-Goals

**Goals:**
- Register the OIDC `CallbackAuthN` provider in `pkg/signoz/authn.go`
- Implement startup bootstrap that reads `SIGNOZ_ENTRA_*` env vars and creates/updates the AuthDomain
- Group-to-role mapping: admin group GUID → ADMIN, editor group GUID → EDITOR, default → VIEWER
- Bootstrap idempotently — create on first run, update on subsequent runs if config changes

**Non-Goals:**
- Multi-tenant support (one Entra tenant, one org)
- Frontend UI for SSO configuration
- Full integration test with real Entra tenant (manual testing only)

## Decisions

### D1: Bootstrap as a separate function in `pkg/signoz/`

Create `pkg/signoz/entrabootstrap.go` with `BootstrapEntraSSO(ctx, logger, sqlstore, orgGetter)`. Called from `signoz.New()` after migrations and auth initialization. Keeps bootstrap concern separate from provider creation.

**Alternative**: Expand `NewAuthNs` to accept `AuthDomainStore` and bootstrap inside it. Rejected because it mixes provider creation with database seeding and requires changing the callback signature across multiple files.

### D2: Find org via `orgGetter.ListByOwnedKeyRange()`

Single-org deployment: use `ListByOwnedKeyRange()` to get the first (and only) org. If no org exists yet (first startup), skip bootstrap with a warning — the org will be created when the first user signs up, and bootstrap will succeed on the next restart.

### D3: Upsert pattern for AuthDomain

1. Try `GetByNameAndOrgID(domain, orgID)` 
2. If not found → `Create` new AuthDomain
3. If found → `Update` with current env var values

This makes the bootstrap idempotent and handles env var changes between restarts.

### D4: Issuer URL construction

Construct the Entra issuer URL from tenant ID: `https://login.microsoftonline.com/{tenant}/v2.0`

Set `IssuerAlias` to empty string for v2.0 endpoints (the v2.0 discovery document's issuer matches the URL). If operators need the v1.0 alias quirk, they can set `SIGNOZ_ENTRA_ISSUER_ALIAS` in the future.

## Risks / Trade-offs

- **[Risk] Bootstrap runs before org exists on first startup** → Mitigation: skip with info log, works on restart after org creation
- **[Trade-off] Env vars read directly via `os.Getenv`** → Simple and matches the constraint (C6: env-var-only config). No config struct needed for this prototype
