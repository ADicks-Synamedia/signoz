# Review: community-image-for-sso-deployment

> **Reviewer**: Reviewer (openspec-agents-image)
> **Date**: 2026-05-08
> **Change**: `openspec/changes/community-image-for-sso-deployment/`
> **Branch**: `fix/community-image-for-sso-deployment`
> **Verdict**: PASS WITH CHANGES (1 WARNING, 1 SUGGESTION; no CRITICALs)

---

## Summary

The change closes the runtime gap left by the prior Entra SSO PR by overriding the `signoz` service image in the SSO compose overlay to point at a locally-built community variant, and by adding an operator-facing build path (Makefile convenience target + new section in the operator guide). The four edits land cleanly: compose overlay override, `.env.example` documentation, Makefile target, and an "Build the Community Image" section placed before Prerequisites in `docs/operator-guide.md`. OpenSpec strict validation passes, the convenience target builds and tags successfully, and `docker compose config` resolves to the community image as intended.

The change is correct on the load-bearing question (operator following the guide will end up running the community variant). The findings below are about polish: one stale section reference left over from the Table of Contents renumbering, and a foot-gun in the new "pin to a specific build" guidance that interacts badly with Decision D3 (bare image name).

---

## Dimension 1 — Completeness

### Tasks

All implementation tasks (1.x through 6.x) are checked complete in `tasks.md`. Tasks 7.1–7.5 are intentionally deferred to post-review; nothing pending in the implementation scope.

| Task group | Status | Notes |
|---|---|---|
| 1. Compose overlay override | Complete | `docker-compose-entra-sso.yaml:5` adds `image: signoz-community:${SIGNOZ_COMMUNITY_TAG:-local}` as the first key under the service. Verified `docker compose ... config` resolves to `signoz-community:local` (default) and to `signoz-community:v0.119.0-amd64` with the env var set. |
| 2. .env.example | Complete | `deploy/docker/.env.example:15-20` adds the commented `SIGNOZ_COMMUNITY_TAG` block with rationale and a cross-reference to `operator-guide.md`. |
| 3. Makefile target | Complete | `Makefile:171-175` declares `.PHONY: docker-build-community-local`, depends on `docker-build-community-$(ARCH)` (transitively pulling in `go-build-community-%` and `js-build`), tags to `signoz-community:local`, prints next-step compose command. `make help \| grep -i community` shows the target alongside the existing family. |
| 4. Operator guide | Complete | New section 1 before Prerequisites with prerequisites, one-command path, two-step fallback, verification, and rebuild-after-pull guidance. Table of Contents updated. |
| 5. Build verification | Complete | `signoz-community:local` is present in `docker images` (verified). |
| 6. Spec/change validation | Complete | `openspec validate community-image-for-sso-deployment --strict` returns `is valid`. Postgres overlay still composes cleanly with the SSO overlay layered on. |

### Specs

The new spec at `specs/docker-compose-entra-sso/spec.md` declares two `ADDED` requirements and one `MODIFIED` requirement. Each scenario has a corresponding implementation:

- **"Overlay declares image override on signoz service"** → `docker-compose-entra-sso.yaml:5`. Note: the spec scenario allows either `docker.io/signoz/signoz-community` or the bare name. The implementation chose the bare name `signoz-community` per design D3, which the scenario language accommodates.
- **"Compose config resolves with default tag"** → verified manually; renders `signoz-community:local`.
- **"Compose config honors operator override"** → verified manually; renders `signoz-community:v0.119.0-amd64`.
- **"Operator guide documents the build step"** → all five sub-conditions met (heading, variant rationale, `make` invocation, prerequisites list, resulting tag named).
- **"All variables present"** (modified `.env.example` requirement) → all listed variables present including the new `SIGNOZ_COMMUNITY_TAG` commented entry.

No requirements unaddressed.

---

## Dimension 2 — Correctness

### Will an operator following the guide end up with a working SSO-enabled SigNoz?

Yes, on the golden path. The new section 1 leads with the load-bearing rationale (variant difference, where `BootstrapEntraSSO` is wired), gives a single command (`make docker-build-community-local`) that produces the exact tag the overlay references, and verifies the result in `docker images`. The compose overlay's `image:` field then resolves to the locally-built community image — confirmed by `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml config` rendering `image: signoz-community:local` for the `signoz` service.

The Makefile dependency chain is sound: `docker-build-community-local` depends on `docker-build-community-$(ARCH)`, which itself depends on `go-build-community-% js-build` (Makefile:165). Operators who run only the convenience target still get the cross-compiled binary and frontend build because Make resolves the prerequisite chain transitively.

The auto-detected `ARCH` (Makefile:8) handles both Linux/amd64 and M-series Mac/arm64 hosts via `uname -m | sed`, so the convenience target works without operator decision on either of the two supported architectures.

### Issues found

**[WARNING] Operator-guide section 4a still references "step 2" after section renumbering.**
- **Location**: `docs/operator-guide.md:201` — "**`SIGNOZ_ENTRA_*`** — Entra app registration values from step 2."
- **Why it's wrong**: The Table of Contents update moved Azure Entra Configuration from section 2 to section 3 (the new section 1 is "Build the Community Image"). All other intra-doc references were updated (lines 121, 175, 362, 380, 388, 407 all correctly say "step 3a/3d/3e"). Line 201 was missed.
- **Operator impact**: An operator following 4a is told to fill in `SIGNOZ_ENTRA_*` from "step 2", but step 2 is now Prerequisites, which has no Entra configuration — they must guess that the writer meant section 3 (Azure Entra Configuration). Likely recoverable but adds friction in a guide that otherwise reads cleanly.
- **Fix**: Change `from step 2` → `from section 3` (or `from step 3b/3c`, since client ID and secret come from those subsections).

**[SUGGESTION] The "pin a specific build" guidance in 1e creates a foot-gun against design D3.**
- **Location**: `docs/operator-guide.md:79` — "set `SIGNOZ_COMMUNITY_TAG` in your `.env` to the per-arch tag (e.g., `SIGNOZ_COMMUNITY_TAG=fix-sso-deploy-abc1234-amd64`)".
- **Why it's a foot-gun**: The compose overlay interpolates as `signoz-community:${SIGNOZ_COMMUNITY_TAG:-local}` — bare repo name. But `make docker-build-community-amd64` produces an image tagged `docker.io/signoz/signoz-community:<branch>-<sha>-amd64`, not `signoz-community:<branch>-<sha>-amd64`. Verified directly:
  - `docker images` after build shows two entries for the same image ID: `signoz/signoz-community:<branch>-<sha>-amd64` and `signoz-community:local`.
  - `SIGNOZ_COMMUNITY_TAG=fix-sso-deploy-abc1234-amd64 docker compose ... config` renders `image: signoz-community:fix-sso-deploy-abc1234-amd64` — a tag that does not exist locally.
  - Operator running `docker compose up` against this would see `Unable to find image 'signoz-community:fix-sso-deploy-abc1234-amd64' locally`, then a registry pull attempt, then `manifest unknown` (the bare-name image was never pushed). The exact failure mode design D3 was meant to avoid.
- **Why SUGGESTION not WARNING**: The default golden path (`SIGNOZ_COMMUNITY_TAG` left commented, convenience target run) does not hit this. The foot-gun only triggers for the optional "pin to a specific build" branch, which an operator is unlikely to take on first deploy. But the instructions as written are wrong.
- **Two acceptable fixes**:
  1. Change the example to a tag the operator must produce themselves with `docker tag`: e.g., "build the per-arch image, then `docker tag docker.io/signoz/signoz-community:<branch>-<sha>-amd64 signoz-community:<branch>-<sha>-amd64`, then set `SIGNOZ_COMMUNITY_TAG=<branch>-<sha>-amd64`." Verbose but correct.
  2. Remove the example entirely and instead say: "If you want to pin to a specific build, change the overlay's image reference manually or maintain your own tag." Avoids the foot-gun without re-litigating D3.
- The `.env.example` block (`deploy/docker/.env.example:15-20`) does not have this problem — it only documents the default `:local` value and points to the operator guide.

### What was checked but found correct

- **Image override actually wins**: confirmed by `docker compose config` rendering. The base file's `image: signoz/signoz:${VERSION:-v0.119.0}` is correctly superseded by the overlay's `image: signoz-community:${SIGNOZ_COMMUNITY_TAG:-local}`.
- **Postgres overlay still composes**: `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml -f docker-compose-postgres.yaml config` exits 0 and renders the community image override (postgres adds a sidecar without touching `services.signoz.image`).
- **The HA overlay (`docker-compose.ha.yaml`) is out of scope** and continues to use the upstream image. The change does not claim to support HA + SSO; non-goal in design.md.
- **Convenience target's `@echo` next-step instruction** correctly references both compose files in the right order. Pasting it copy-paste works.
- **Build prerequisites list** (Go, Node/Yarn, Docker) matches what `docker-build-community-%` actually requires through its transitive `go-build-community-%` and `js-build` dependencies.
- **OpenSpec validation passes strict mode.**

---

## Dimension 3 — Coherence

### Architecture / convention fit

- The change does **not** modify any Go source, any `ee/` content, or `cmd/enterprise/`. License-boundary constraint respected.
- The change does **not** modify the existing `docker-build-community-%` per-arch targets, the base `docker-compose.yaml`, or `cmd/community/Dockerfile`. Additive at the deployment-config layer only, matching the design's stated scope.
- The Makefile target is named `docker-build-community-local`, which surfaces in `make help` alphabetically next to the existing `docker-build-community` family. Naming convention is consistent.
- The compose `${VAR:-default}` syntax in the overlay matches the base compose file's `${VERSION:-v0.119.0}` convention. No new conventions introduced.
- The bare-name choice (D3) is documented in `design.md` with rationale; future maintainers reading the overlay will find the explanation. The semantic shift from "env-only overlay" to "env + image overlay" (D6) is also explicitly flagged in design.md.

### Documentation fit

- Section placement (before Prerequisites) is correct: operators must build before they can deploy, and the existing Prerequisites cross-references the new section explicitly (line 85).
- The "rebuild after `git pull`" callout (1e) preempts a known support pattern — operators forgetting to rebuild and seeing stale behavior.
- The `.env.example` entry is co-located with `COMPOSE_PROJECT_NAME` near the top of the file, which is the highest-visibility location for a discovery surface.

No coherence issues found.

---

## Findings Summary

| Severity | Count | IDs |
|---|---|---|
| CRITICAL | 0 | — |
| WARNING | 1 | Stale `step 2` reference at `docs/operator-guide.md:201` |
| SUGGESTION | 1 | Foot-gun in "pin specific build" guidance at `docs/operator-guide.md:79` |

---

## Recommendation

**PASS WITH CHANGES.** The implementation is sound on the golden path and on the spec scenarios. Address the WARNING before archive (one-line edit). The SUGGESTION can be addressed in a follow-up if the team wants to tighten the optional override branch, but it does not block the merge — most operators will not exercise that path.
