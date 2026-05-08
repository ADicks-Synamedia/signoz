## MODIFIED Requirements

### Requirement: .env.example documents all variables
`deploy/docker/.env.example` SHALL contain `COMPOSE_PROJECT_NAME=signoz`, all required and optional `SIGNOZ_ENTRA_*` variables, and the `SIGNOZ_USER_ROOT_*` variables required for SSO-first deployments. All variables SHALL appear with placeholder values and inline comments. The file SHALL be the complete template an operator copies to `.env`.

#### Scenario: All variables present
- **WHEN** the `.env.example` file is read
- **THEN** it contains:
  - `COMPOSE_PROJECT_NAME=signoz`
  - `SIGNOZ_ENTRA_SSO_ENABLED`, `SIGNOZ_ENTRA_TENANT_ID`, `SIGNOZ_ENTRA_CLIENT_ID`, `SIGNOZ_ENTRA_CLIENT_SECRET`, `SIGNOZ_ENTRA_DOMAIN`
  - `SIGNOZ_ENTRA_ADMIN_GROUP_ID`, `SIGNOZ_ENTRA_EDITOR_GROUP_ID`, `SIGNOZ_ENTRA_DEFAULT_ROLE`
  - `SIGNOZ_USER_ROOT_ENABLED`, `SIGNOZ_USER_ROOT_EMAIL`, `SIGNOZ_USER_ROOT_PASSWORD`, `SIGNOZ_USER_ROOT_ORG_NAME`

## ADDED Requirements

### Requirement: Compose overlay forwards SIGNOZ_USER_ROOT_* env vars
`deploy/docker/docker-compose-entra-sso.yaml` SHALL forward `SIGNOZ_USER_ROOT_ENABLED`, `SIGNOZ_USER_ROOT_EMAIL`, `SIGNOZ_USER_ROOT_PASSWORD`, and `SIGNOZ_USER_ROOT_ORG_NAME` from the host environment into the `signoz` service so the user reconciler creates the first organization on first boot.

#### Scenario: Compose config exposes root-user vars
- **WHEN** `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` is run with the four `SIGNOZ_USER_ROOT_*` variables set in the environment
- **THEN** the rendered config includes those variables on the `signoz` service's `environment` list and the command exits with code 0

### Requirement: Operator .env file is not tracked
`deploy/docker/.env` SHALL NOT be tracked in the repository. The file is operator-supplied (created by copying `.env.example`) and may contain secrets.

#### Scenario: Operator .env is gitignored and not in the index
- **WHEN** the repository is freshly cloned
- **THEN** `deploy/docker/.env` does not exist in the working tree until the operator creates it
- **AND** `.gitignore` matches `.env` so any future operator edits are not staged accidentally
