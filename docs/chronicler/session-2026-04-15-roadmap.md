# Session note — 2026-04-15 — Roadmap and Phase 1 Milestones

**Main session agent:** PragmaticTPM  
**Duration (approx):** single session

## What happened

PragmaticTPM read the full Vision (overview, goals, constraints, glossary, personas), Architecture document, ADR-0001, and all prior session notes. Produced the Roadmap and Phase 1 milestone breakdown.

### Key observations from the architecture review

1. **Very little new code required**: The SigNoz auth system was designed for multiple providers. The `CallbackAuthN` interface, `OIDCConfig` type, OIDC callback route, role mapping, and JIT provisioning all exist. The adapter fills a gap that the framework was already expecting.

2. **Three natural milestones**: The architecture's own implementation plan (Section 12) maps cleanly to three milestones — core OIDC provider, registration + bootstrap + role mapping, and deployment packaging. These are the milestones adopted in Phase 1.

3. **Two phases, not three**: Given the this-week constraint (C4), the roadmap collapses to two phases. Phase 1 is the working prototype (all five Vision goals met). Phase 2 is hardening and documentation — scoped after Phase 1 ships.

## Decisions made

- **Two-phase roadmap**: Phase 1 (working end-to-end SSO) and Phase 2 (hardening, docs, test coverage). Phase 2 is explicitly out of the this-week window.
- **Three milestones in Phase 1**: (1) OIDC CallbackAuthN provider, (2) registration + bootstrap + role mapping, (3) Docker Compose + operator config.
- **Milestone 2 combines bootstrap and role mapping**: The architecture separates "Registration & Bootstrap" and "Group-to-role mapping" but they are tightly coupled — the bootstrap seeds the AuthDomain with the RoleMapping config, and integration testing requires both. Combining them into one milestone reduces handoff overhead.

## Artefacts produced

- `docs/roadmap.md` — Two-phase roadmap replacing the stub
- `docs/phases/phase-1.md` — Phase 1 detail with three milestones, each having capability description and acceptance criteria
- `docs/chronicler/session-2026-04-15-roadmap.md` — This session note

## Open questions punted

- **Milestone ordering for implementation**: Milestones are numbered for logical dependency order (1 → 2 → 3) but a single developer could interleave them. The ordering is guidance, not a hard gate.
- **Phase 2 scope**: Intentionally left as a paragraph rather than milestones. Phase 2 will be scoped properly after Phase 1 ships and real gaps are visible.

## Next session should

Begin implementation of Phase 1, Milestone 1 — the `oidccallbackauthn` package.
