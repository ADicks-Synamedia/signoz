## Why

Milestones 1 and 2 deliver the OIDC provider and env-var bootstrap. Operators need deployment packaging to actually use Entra SSO — a Compose overlay that injects the required environment variables and a documented .env template.

## What Changes

- **New**: `deploy/docker/docker-compose-entra-sso.yaml` — Compose overlay extending the base file with `SIGNOZ_ENTRA_*` env vars on the signoz service
- **New**: `deploy/docker/.env.example` — template with all required and optional variables, inline comments
- **New**: `deploy/docker/docker-compose-postgres.yaml` — optional overlay for PostgreSQL instead of SQLite

## Capabilities

### New Capabilities
- `docker-compose-entra-sso`: Docker Compose deployment packaging for Entra SSO with environment variable configuration

### Modified Capabilities

## Impact

- **No code changes** — config/deployment files only
- **No new dependencies**
