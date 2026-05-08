## ADDED Requirements

### Requirement: Compose overlay overrides signoz service image to community variant
`deploy/docker/docker-compose-entra-sso.yaml` SHALL override the `image:` field of the `signoz` service so that the running container is the community variant (which contains the Entra SSO adapter and `BootstrapEntraSSO` invocation in `cmd/community/server.go`) rather than the upstream enterprise variant inherited from `deploy/docker/docker-compose.yaml`. The override SHALL reference an image tag that an operator can produce locally via the existing `Makefile` community Docker build targets, and the tag SHALL be parameterizable via an environment variable so operators can pin to a specific build without editing the overlay.

#### Scenario: Overlay declares image override on signoz service
- **WHEN** `deploy/docker/docker-compose-entra-sso.yaml` is parsed as YAML
- **THEN** the `services.signoz.image` key is present
- **AND** the value references the community image registry path (`docker.io/signoz/signoz-community` or the value of `DOCKER_REGISTRY_COMMUNITY` from `Makefile`)
- **AND** the tag portion uses an env-var substitution with a sensible default (e.g., `${SIGNOZ_COMMUNITY_TAG:-local}`) so operators can override per-deployment

#### Scenario: Compose config resolves with default tag
- **WHEN** `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` is run with `SIGNOZ_COMMUNITY_TAG` unset
- **THEN** the command exits 0
- **AND** the rendered `signoz` service image matches the community-variant image with the documented default tag

#### Scenario: Compose config honors operator override
- **WHEN** `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` is run with `SIGNOZ_COMMUNITY_TAG=v0.119.0-amd64` exported
- **THEN** the rendered `signoz` service image ends with the tag `v0.119.0-amd64`

### Requirement: Operator-built community image is the supported runtime for SSO deployments
The deployment documentation under `docs/operator-guide.md` SHALL include a section that (a) explains why the upstream `signoz/signoz` image cannot service SSO-first deployments (the SSO adapter is only compiled into the community variant, and `BootstrapEntraSSO` is only invoked by `cmd/community/server.go`), (b) instructs operators to build the community image with the existing `Makefile` targets before running the compose overlay, (c) lists build prerequisites (Go toolchain, Node + Yarn for the frontend, Docker), and (d) documents the resulting image tag operators can expect so they can verify the build before `docker compose up`.

#### Scenario: Operator guide documents the build step
- **WHEN** `docs/operator-guide.md` is read
- **THEN** there is a section (heading title containing "build" and "community", case-insensitive) that appears before or in the Prerequisites section
- **AND** the section explains the variant difference and why the upstream image is unsuitable
- **AND** the section includes a `make` invocation that produces the community image (e.g., `make docker-build-community-amd64` or a documented convenience target)
- **AND** the section lists Go, Node/Yarn, and Docker as build-time prerequisites
- **AND** the section names the resulting image tag operators should see in `docker images` after a successful build

## MODIFIED Requirements

### Requirement: .env.example documents all variables
`deploy/docker/.env.example` SHALL contain `COMPOSE_PROJECT_NAME=signoz`, all required and optional `SIGNOZ_ENTRA_*` variables, the `SIGNOZ_USER_ROOT_*` variables required for SSO-first deployments, and a documented entry for the community image tag override variable used by `docker-compose-entra-sso.yaml`. All variables SHALL appear with placeholder values and inline comments. The file SHALL be the complete template an operator copies to `.env`.

#### Scenario: All variables present
- **WHEN** the `.env.example` file is read
- **THEN** it contains:
  - `COMPOSE_PROJECT_NAME=signoz`
  - `SIGNOZ_ENTRA_SSO_ENABLED`, `SIGNOZ_ENTRA_TENANT_ID`, `SIGNOZ_ENTRA_CLIENT_ID`, `SIGNOZ_ENTRA_CLIENT_SECRET`, `SIGNOZ_ENTRA_DOMAIN`
  - `SIGNOZ_ENTRA_ADMIN_GROUP_ID`, `SIGNOZ_ENTRA_EDITOR_GROUP_ID`, `SIGNOZ_ENTRA_DEFAULT_ROLE`
  - `SIGNOZ_USER_ROOT_ENABLED`, `SIGNOZ_USER_ROOT_EMAIL`, `SIGNOZ_USER_ROOT_PASSWORD`, `SIGNOZ_USER_ROOT_ORG_NAME`
  - A commented entry for `SIGNOZ_COMMUNITY_TAG` (or whatever variable name the overlay uses) with an explanatory inline comment so operators discover the override
