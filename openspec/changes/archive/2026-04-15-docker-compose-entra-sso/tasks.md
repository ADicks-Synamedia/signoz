## 1. Compose Overlay

- [x] 1.1 Create `deploy/docker/docker-compose-entra-sso.yaml` with SIGNOZ_ENTRA_* env vars on the signoz service

## 2. Environment Template

- [x] 2.1 Create `deploy/docker/.env.example` with all required/optional variables documented

## 3. PostgreSQL Overlay

- [x] 3.1 Create `deploy/docker/docker-compose-postgres.yaml` with PostgreSQL service and signoz config

## 4. Validation

- [x] 4.1 Run `docker compose config` to validate the compose overlay
- [x] 4.2 Run `docker compose config` to validate the combined SSO + PostgreSQL overlays
