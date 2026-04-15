## Context

The base `deploy/docker/docker-compose.yaml` runs SigNoz with SQLite and no SSO. Operators need a Compose overlay pattern to add Entra SSO configuration without modifying the base file.

## Goals / Non-Goals

**Goals:**
- Compose overlay that extends the base file with Entra env vars
- Documented .env.example with all variables
- Optional PostgreSQL overlay
- `docker compose config` validation passes

**Non-Goals:**
- Helm charts or Kubernetes manifests
- Multi-tenant deployment configurations

## Decisions

### D1: Compose overlay pattern
Use `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml up -d`. The overlay only adds environment variables to the existing `signoz` service — no new services, volumes, or networks.

### D2: .env.example location
Place at `deploy/docker/.env.example` alongside the compose files. Docker Compose automatically reads `.env` from the compose file directory.

### D3: PostgreSQL overlay is optional
Provide `docker-compose-postgres.yaml` for teams that want PostgreSQL. Uses the standard `postgres:16` image with a health check.
