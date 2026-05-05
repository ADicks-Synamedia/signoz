# Product Vision

This directory is owned by **ProductOwner** and contains the product's reason for existing:
its intent, users, goals, non-goals, and constraints.

It is distinct from `openspec/specs/`, which describes *system behaviour*. The Vision is *why*;
the spec is *what*.

Expected contents (created during the intake interview):

- `overview.md` — elevator pitch, problem, users
- `goals.md` — success criteria and non-goals
- `constraints.md` — regulatory, commercial, technical constraints
- `glossary.md` — user-facing vocabulary
- optionally `personas/product-user.md` and `personas/product-admin-user.md`

To start the intake interview:

```
claude --agent openspec-agents:product-owner
```
