# Session note — 2026-04-15 — Vision intake

**Main session agent:** Orchestrator → ProductOwner
**Duration (approx):** single session

## What happened

The Orchestrator conducted the Vision intake interview directly with the human. All answers were collected and signed off. ProductOwner then produced the canonical Vision files from the signed-off answers.

## Decisions made

- **Scope**: Azure Entra ID SSO adapter for SigNoz community edition (MIT-licensed) only.
- **Protocol**: OIDC via MSAL. No SAML.
- **Identity provider**: Entra ID only. No other IdPs.
- **Provisioning model**: JIT on first SSO login. No SCIM or ongoing sync.
- **Role model**: Map to SigNoz's existing roles (admin, general user) via Entra group claims. No custom roles.
- **Deployment**: Docker Compose with PostgreSQL. All config via environment variables.
- **Licensing constraint**: No enterprise-edition code (`ee/`, `cmd/enterprise/`) may be read, referenced, or used.
- **Frontend**: No UI changes. Backend auth changes only; login redirects to Entra.
- **Tenancy**: Single-tenant only.
- **Timing**: Working prototype this week (week of 2026-04-13).

## Artefacts produced or updated

- `product-vision/overview.md` — elevator pitch, problem statement, audiences
- `product-vision/goals.md` — 5 goals, 7 non-goals
- `product-vision/constraints.md` — 6 constraints (licensing, protocol, compatibility, timing, deployment, configuration)
- `product-vision/glossary.md` — 14 user-facing terms
- `product-vision/personas/product-user.md` — SSO End User persona
- `product-vision/personas/product-admin-user.md` — Platform Admin persona
- `product-vision/personas/deployer-operator.md` — Deployer/Operator persona

## Open questions punted

- None. All vision-level questions were answered during the intake interview.

## Next session should

Hand off to **BossArchitect** for architecture and technical design.
