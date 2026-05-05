# Contributing — Azure Entra ID SSO Adapter for SigNoz

This guide covers development workflow for the Entra SSO adapter project. It targets contributors working on the OIDC integration code within the SigNoz community edition.

---

## Prerequisites

- **Go 1.25+** — the module requires Go 1.25 (`go.mod`)
- **Docker & Docker Compose** — for running the full SigNoz stack
- **Node.js 18+** — only if working on the frontend (not required for SSO backend work)
- **An Azure Entra ID test tenant** — for end-to-end testing (free tier available at [Azure Portal](https://portal.azure.com))

---

## Local Dev Setup

### 1. Clone and branch

```bash
git clone https://github.com/SigNoz/signoz.git
cd signoz
git checkout -b your-feature-branch
```

### 2. Install Go dependencies

```bash
go mod download
```

### 3. Run the backend (query-service)

```bash
# From the repo root
go run ./cmd/signoz/ --config ./cmd/signoz/config.yaml
```

Or build and run:

```bash
go build -o signoz ./cmd/signoz/
./signoz --config ./cmd/signoz/config.yaml
```

### 4. Run with Docker Compose (full stack)

```bash
cd deploy/docker

# Base stack (no SSO)
docker compose up -d

# With Entra SSO overlay
cp .env.example .env
# Edit .env with your Entra configuration
docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml up -d
```

### 5. Verify

Open `http://localhost:8080`. If SSO is configured, entering an email matching the configured domain should redirect to Entra login.

---

## Project Structure (SSO-Relevant)

```
pkg/
├── authn/
│   ├── authn.go                              # CallbackAuthN interface
│   └── callbackauthn/
│       ├── googlecallbackauthn/authn.go       # Reference: Google OIDC
│       └── oidccallbackauthn/authn.go         # NEW: Entra/OIDC adapter
├── types/authtypes/
│   ├── authn.go                               # AuthNProvider constants
│   ├── oidc.go                                # OIDCConfig
│   ├── domain.go                              # AuthDomain
│   └── mapping.go                             # RoleMapping, AttributeMapping
├── modules/session/implsession/
│   ├── module.go                              # CreateCallbackAuthNSession
│   └── handler.go                             # HTTP handlers
├── signoz/
│   └── authn.go                               # Provider registration
deploy/docker/
├── docker-compose.yaml                        # Base stack
└── docker-compose-entra-sso.yaml              # SSO overlay
```

---

## How to Run Tests

### Unit tests (all)

```bash
go test ./...
```

### Unit tests (SSO adapter only)

```bash
go test ./pkg/authn/callbackauthn/oidccallbackauthn/...
```

### Unit tests (session module — covers callback flow)

```bash
go test ./pkg/modules/session/...
```

### With verbose output

```bash
go test -v -count=1 ./pkg/authn/callbackauthn/oidccallbackauthn/...
```

### Race detection

```bash
go test -race ./pkg/authn/callbackauthn/oidccallbackauthn/...
```

---

## Commit Message Conventions

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type | Use for |
|---|---|
| `feat` | New feature (e.g., OIDC adapter implementation) |
| `fix` | Bug fix |
| `refactor` | Code restructuring without behavior change |
| `test` | Adding or updating tests |
| `docs` | Documentation only |
| `chore` | Build, CI, dependency updates |

### Scope

Use `entra-sso` or the specific package name:

```
feat(entra-sso): implement OIDC callback handler
fix(oidccallbackauthn): handle issuer alias for Azure endpoints
test(entra-sso): add mock OIDC provider integration tests
docs(entra-sso): add deployment guide for operators
```

### Examples

```
feat(entra-sso): implement CallbackAuthN for OIDC/Entra ID

Adds oidccallbackauthn package that implements the authn.CallbackAuthN
interface using coreos/go-oidc for OIDC discovery and token verification.
Supports Entra's issuer alias, group claims, and claim mapping.

Closes #123
```

---

## Dependency Rules

- **Pin all direct dependencies** — they are tracked in `go.mod` and `go.sum`.
- **No new dependencies** for the OIDC adapter — `coreos/go-oidc/v3` and `golang.org/x/oauth2` are already in the module.
- **Do not add MSAL Go SDK** — the vision mentions MSAL, but `go-oidc` + standard OAuth2 is the correct Go-idiomatic approach. MSAL for Go is primarily for Azure SDK integration, not standalone OIDC.
- Before adding any new dependency, check if an existing one serves the purpose.
- Run `go mod tidy` after any dependency change.

---

## Licensing Constraint

**MUST NOT read, reference, or use any code under `ee/` or `cmd/enterprise/`.** These directories are under the SigNoz Enterprise License. All code in this project must be compatible with the MIT license.

You may read and build upon anything else in the repository.

---

## OpenSpec Workflow

This project uses OpenSpec for structured change management:

1. **Vision** — `product-vision/` contains the signed-off project vision
2. **Architecture** — `docs/architecture.md` describes how the adapter integrates
3. **Decisions** — `docs/decisions/` contains ADRs for key choices
4. **Changes** — Each feature/fix follows the OpenSpec artifact workflow

When making a change:
- Check for an existing OpenSpec change proposal before starting
- Reference the relevant change ID in commit messages and PRs
- Update architecture docs if your change affects module boundaries or data flow

---

## Code Review Expectations

### For authors

- One PR per logical change (one feature, one bug fix, one refactor)
- Include tests for new code — mock OIDC provider for unit tests
- Ensure `go vet`, `go test`, and `go build` pass
- Update `docs/architecture.md` if you change the integration design
- Do not commit secrets, `.env` files, or client credentials

### For reviewers

- Verify no `ee/` or `cmd/enterprise/` imports
- Check that the `CallbackAuthN` interface contract is maintained
- Verify structured logging uses `slog` with context
- Confirm environment variable names follow the `SIGNOZ_ENTRA_` prefix convention
- Test the auth flow mentally: LoginURL → Entra → callback → token exchange → claims → role mapping → JIT provision → JWT → redirect

---

## Troubleshooting

### OIDC discovery fails

Check that `SIGNOZ_ENTRA_TENANT_ID` is correct. The discovery URL is:
```
https://login.microsoftonline.com/{tenant-id}/v2.0/.well-known/openid-configuration
```

### Token verification fails

Entra's issuer in the ID token (`iss` claim) may differ from the discovery URL. The `OIDCConfig.IssuerAlias` field handles this — ensure it's set if needed.

### Group claims missing

In Azure Portal: App Registration → Token configuration → Add groups claim → Select "Security groups". If the user is in more than 150 groups, Entra returns an overage indicator instead of inline groups — this is a known limitation.

### Role mapping not working

Check that the Entra group object IDs (GUIDs) in your env vars match exactly. Use Azure Portal → Groups → select group → copy Object ID.
