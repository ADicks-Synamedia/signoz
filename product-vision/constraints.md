# Constraints

These constraints bound all design and implementation decisions for the project.

## C1 — Licensing

MUST NOT read, reference, or use any code under `ee/` or `cmd/enterprise/` (SigNoz Enterprise License). All work is in the MIT-licensed community edition code only. Every file touched or created must be compatible with the MIT licence.

## C2 — Protocol

OIDC only, implemented via Microsoft's MSAL libraries. No SAML, no WS-Federation, no proprietary protocols.

## C3 — Compatibility

Must integrate with SigNoz's existing community-edition auth interfaces — not replace them wholesale. Existing local-auth users must continue to function if present. The SSO adapter adds a new authentication path alongside the existing one.

## C4 — Timing

Working prototype needed this week (by end of week of 2026-04-13). Priority is a functional end-to-end flow, not production hardening.

## C5 — Deployment Model

Must work as a Docker Compose stack with PostgreSQL as the backing store. No external dependencies beyond Microsoft Entra ID. No Kubernetes, no Helm charts, no managed database services required.

## C6 — Configuration Surface

All configuration is via environment variables. No mounted config files, no admin UI for SSO setup, no database-stored configuration for Entra parameters.
