# Roadmap — Azure Entra ID SSO Adapter for SigNoz Community Edition

> **Last updated**: 2026-04-15  
> **Owner**: PragmaticTPM  
> **Constraint**: Working prototype by end of week 2026-04-13 (Constraint C4)

---

## Phase 1 — Working Entra SSO End-to-End

**Goal**: A developer can run `docker compose up`, authenticate via Entra ID, and land on the SigNoz dashboard with the correct role — admin or viewer — assigned from Entra group membership. This is the core deliverable.

Phase 1 covers three milestones: (1) the OIDC `CallbackAuthN` provider implementation, (2) group-to-role mapping with JIT user provisioning and environment-variable bootstrap, and (3) Docker Compose packaging with a `.env.example` for operators. By the end of Phase 1 every goal in the Vision (G1–G5) is met at prototype quality.

See [`docs/phases/phase-1.md`](phases/phase-1.md) for milestone details and acceptance criteria.

## Phase 2 — Hardening, Documentation, and Test Coverage

Polish the prototype into something a team can confidently hand to a platform operator. Expand unit and integration test coverage (mock OIDC provider tests, full callback-flow integration tests). Write an operator-facing setup guide covering Entra App Registration, environment variables, and troubleshooting. Add structured logging for all auth events listed in the architecture doc. Address known gaps flagged during Phase 1 (e.g., group overage indicator handling, role-update-on-subsequent-login behaviour).

Phase 2 is scoped after Phase 1 ships and is not part of the this-week deliverable.
