# Review: docker-compose-entra-sso (Milestone 3)

> **Reviewer**: Reviewer3
> **Date**: 2026-04-15
> **Change**: `openspec/changes/docker-compose-entra-sso/`
> **Verdict**: PASS

---

## Summary

Milestone 3 delivers Docker Compose deployment packaging for Entra SSO: a compose overlay (`docker-compose-entra-sso.yaml`), an operator configuration template (`.env.example`), and a pre-existing PostgreSQL overlay (`docker-compose-postgres.yaml`). All three artifacts are well-structured, follow existing patterns in the repository, and pass compose config validation.

---

## Dimension 1 — Correctness

### Compose Overlay Targets the Right Service

The overlay extends the `signoz` service, which matches the service name in `deploy/docker/docker-compose.yaml:110`. The merged config produces a single `signoz` service with all original environment variables plus the 8 `SIGNOZ_ENTRA_*` additions. Verified by running `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` — all variables appear correctly merged.

### All Required Variables Are Present

The overlay injects all 8 variables documented in the architecture (`docs/architecture.md` section 6.1 and 9.1):

| Variable | In Overlay | Default | Matches Architecture |
|---|---|---|---|
| `SIGNOZ_ENTRA_SSO_ENABLED` | Yes | `false` | Yes |
| `SIGNOZ_ENTRA_TENANT_ID` | Yes | (none) | Yes |
| `SIGNOZ_ENTRA_CLIENT_ID` | Yes | (none) | Yes |
| `SIGNOZ_ENTRA_CLIENT_SECRET` | Yes | (none) | Yes |
| `SIGNOZ_ENTRA_DOMAIN` | Yes | (none) | Yes |
| `SIGNOZ_ENTRA_ADMIN_GROUP_ID` | Yes | `""` (empty) | Yes |
| `SIGNOZ_ENTRA_EDITOR_GROUP_ID` | Yes | `""` (empty) | Yes |
| `SIGNOZ_ENTRA_DEFAULT_ROLE` | Yes | `VIEWER` | Yes |

### Default Values Are Sensible

- `SIGNOZ_ENTRA_SSO_ENABLED` defaults to `false` — SSO is opt-in. Correct.
- `SIGNOZ_ENTRA_DEFAULT_ROLE` defaults to `VIEWER` — least privilege. Correct.
- Optional group IDs default to empty string — no group mapping by default. Correct.
- Required variables (TENANT_ID, CLIENT_ID, CLIENT_SECRET, DOMAIN) have no default — compose warns but doesn't fail, forcing the operator to set them. Correct.

### Compose Config Validation Passes

Both validation commands succeed (exit 0) with only expected warnings:
- `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` — warnings about unset required vars and obsolete `version` key.
- `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml -f docker-compose-postgres.yaml config` — same warnings plus postgres overlay.

---

## Dimension 2 — Completeness

### .env.example Documents Every Variable

All 8 `SIGNOZ_ENTRA_*` variables are present in `deploy/docker/.env.example`:
- **Required** (uncommented with placeholder values): `SIGNOZ_ENTRA_SSO_ENABLED`, `SIGNOZ_ENTRA_TENANT_ID`, `SIGNOZ_ENTRA_CLIENT_ID`, `SIGNOZ_ENTRA_CLIENT_SECRET`, `SIGNOZ_ENTRA_DOMAIN`
- **Optional** (commented out with descriptions): `SIGNOZ_ENTRA_ADMIN_GROUP_ID`, `SIGNOZ_ENTRA_EDITOR_GROUP_ID`, `SIGNOZ_ENTRA_DEFAULT_ROLE`

Each variable has an inline comment explaining what it is and where to find the value in the Azure Portal.

### Azure Portal Setup Instructions Are Correct

The `.env.example` (lines 38-44) documents the 4 required Azure Portal steps:
1. App registration with redirect URI `https://<signoz-host>:8080/api/v1/complete/oidc` and single-tenant account type
2. Client secret creation
3. Token configuration with groups claim
4. User/group assignment via Enterprise Applications

The redirect URI matches the OIDC callback route already registered in the codebase at `pkg/apiserver/signozapiserver/session.go`, as documented in `docs/architecture.md` section 3.1.

### Usage Instructions Present

The file header (line 5) documents the exact compose command: `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml up -d`.

---

## Dimension 3 — Consistency

### Follows Existing Overlay Pattern

The overlay follows the exact same pattern as `deploy/docker/docker-compose-postgres.yaml`:
- Same `version: "3"` header
- Same structure: `services: > signoz: > environment:` with only env var additions
- No new services, volumes, or networks — minimal diff surface

### Consistent With Architecture Document

The overlay contents exactly match the deployment shape described in `docs/architecture.md` section 8.1 (lines 306-323). The `.env.example` variables match the configuration table in section 9.1 (lines 377-389).

### Consistent With Phase 1 Acceptance Criteria

`docs/phases/phase-1.md` Milestone 3 acceptance criteria are fully addressed:
- [x] `docker-compose-entra-sso.yaml` exists and is valid YAML extending the base compose
- [x] `.env.example` exists with all required and optional variables documented
- [x] `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` succeeds
- [x] E2E test criteria is documented (requires real Entra tenant — not automatable in review)

---

## Findings

### SUGGESTION-01: Drop obsolete `version` key from new overlay

**File**: `deploy/docker/docker-compose-entra-sso.yaml:1`

Both `docker-compose.yaml` and `docker-compose-postgres.yaml` use `version: "3"`, which Docker Compose V2 flags as obsolete in every validation run. Since this is a **new** file, it has the opportunity to omit the key. However, consistency with the existing files may be intentionally preferred. If the team plans to clean up the `version` key across all compose files, this could be done in a single pass.

**Severity**: SUGGESTION
**Impact**: Cosmetic — suppresses one deprecation warning for the overlay file.

### SUGGESTION-02: Consider adding `SIGNOZ_ENTRA_REDIRECT_URI` as an optional override

**File**: `deploy/docker/docker-compose-entra-sso.yaml`, `deploy/docker/.env.example`

The redirect URI is hard-documented as `https://<signoz-host>:8080/api/v1/complete/oidc` in the `.env.example` setup instructions. If an operator deploys SigNoz behind a reverse proxy on a different port or path prefix, the redirect URI registered in Entra must match the actual callback URL. Currently the operator must configure this only in Azure Portal (not as a SigNoz env var), which is correct for the current architecture since the callback route is fixed. No action needed now, but worth noting for future TLS/reverse-proxy documentation.

**Severity**: SUGGESTION
**Impact**: Documentation enhancement for non-standard deployments.

### WARNING-01: PostgreSQL overlay listed in proposal but pre-existed

**File**: `openspec/changes/docker-compose-entra-sso/proposal.md:9`

The proposal lists `deploy/docker/docker-compose-postgres.yaml` as a **New** file, but the file already exists in the base compose directory and was referenced by the architecture document as an existing optional overlay. The change did not create this file — it was already part of the repository. This is a minor inaccuracy in the proposal artifact, not in the implementation itself.

**Severity**: WARNING
**Impact**: Misleading change log — reviewers may expect to find postgres overlay changes in this changeset when it was already present.

---

## Verdict

**PASS** — The Milestone 3 implementation meets all acceptance criteria. The compose overlay correctly extends the base compose with all required `SIGNOZ_ENTRA_*` variables, the `.env.example` comprehensively documents every variable with accurate Azure Portal setup instructions, and compose config validation passes. The findings are minor suggestions and one documentation accuracy warning. No CRITICAL issues found.
