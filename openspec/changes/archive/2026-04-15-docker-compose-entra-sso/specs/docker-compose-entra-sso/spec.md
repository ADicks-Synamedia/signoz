## ADDED Requirements

### Requirement: Compose overlay injects Entra env vars
`deploy/docker/docker-compose-entra-sso.yaml` SHALL be valid YAML that extends the base compose file by adding `SIGNOZ_ENTRA_*` environment variables to the `signoz` service.

#### Scenario: Compose config validation
- **WHEN** `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` is run
- **THEN** the command succeeds with exit code 0

### Requirement: .env.example documents all variables
`deploy/docker/.env.example` SHALL contain all required and optional `SIGNOZ_ENTRA_*` variables with placeholder values and inline comments.

#### Scenario: All variables present
- **WHEN** the .env.example file is read
- **THEN** it contains SIGNOZ_ENTRA_SSO_ENABLED, SIGNOZ_ENTRA_TENANT_ID, SIGNOZ_ENTRA_CLIENT_ID, SIGNOZ_ENTRA_CLIENT_SECRET, SIGNOZ_ENTRA_DOMAIN, SIGNOZ_ENTRA_ADMIN_GROUP_ID, SIGNOZ_ENTRA_EDITOR_GROUP_ID, SIGNOZ_ENTRA_DEFAULT_ROLE

### Requirement: PostgreSQL overlay is valid
`deploy/docker/docker-compose-postgres.yaml` SHALL be valid YAML that adds a PostgreSQL service and configures the signoz service to use it.

#### Scenario: PostgreSQL compose config validation
- **WHEN** `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml -f docker-compose-postgres.yaml config` is run
- **THEN** the command succeeds with exit code 0
