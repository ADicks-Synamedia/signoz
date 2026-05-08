## Context

PR #2 (`fix/sso-first-deployment-org-bootstrap`) shipped the bootstrap-wait fix, the `SIGNOZ_USER_ROOT_*` plumbing, and the operator guide — but a runtime gap was discovered post-merge: `deploy/docker/docker-compose.yaml:112` pins `image: signoz/signoz:${VERSION:-v0.119.0}`, which is the upstream **enterprise** variant (built from `cmd/enterprise/Dockerfile`). Our Entra SSO adapter and `BootstrapEntraSSO` invocation live in `cmd/community/server.go:129`; the enterprise main never calls them. Operators following the guide today get a container with `"variant":"enterprise"` in its logs and zero awareness of `SIGNOZ_ENTRA_*`.

We cannot modify `ee/` or `cmd/enterprise/` source (license boundary). The supported community-build pipeline already exists in `Makefile:117-167`:
- `go-build-community-%` cross-compiles `signoz-community` into `target/$(OS)-$*/`
- `js-build` runs `yarn build` in `frontend/`
- `docker-build-community-%` runs `docker build -f cmd/community/Dockerfile -t $(DOCKER_REGISTRY_COMMUNITY):$(VERSION)-$*`

The community Dockerfile (`cmd/community/Dockerfile:13-15`) is an **assembly stage**: it `COPY`s pre-built `target/${OS}-${TARGETARCH}/signoz-community` and `frontend/build/` artifacts into Alpine. It is not self-contained — it depends on Make pre-steps to produce its inputs.

This change closes the runtime gap by overriding the `signoz` service image in the SSO compose overlay to point at a locally-built community tag, and documents the build step operators must run beforehand.

**Constraints:**
- No source modifications to `ee/` or `cmd/enterprise/`.
- No modifications to `cmd/community/Dockerfile` (would expand scope into upstream-territory build refactor).
- No modifications to the existing community Make targets (additive only).
- Must not affect non-SSO deployments (the base `docker-compose.yaml` continues to use the upstream image when the SSO overlay is absent).

**Stakeholders:**
- Operators deploying SigNoz with Entra SSO (primary).
- Future contributors maintaining the community-image build pipeline (secondary — the convenience target should be obvious and idiomatic).
- BossArchitect signed off on the approach below (Option A + convenience target with refinements).

## Goals / Non-Goals

**Goals:**
- Make `docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml up -d` produce a running container of the **community** variant containing the SSO adapter, with one extra build step that operators can run unambiguously.
- Keep the runtime image tag visible and operator-controllable (no Compose-managed implicit builds).
- Document the build step prominently in `docs/operator-guide.md` so the operator workflow is self-contained.
- Provide a single Make target operators can run to produce a correctly-tagged image without a separate re-tag step.

**Non-Goals:**
- Not refactoring `cmd/community/Dockerfile` into a multi-stage self-contained build. (Right answer long-term, but out of scope and risks upstream merge conflicts.)
- Not publishing a community image to a public registry. (Out of scope; future work.)
- Not removing or modifying the upstream image reference in `docker-compose.yaml`. The base file remains as-is so non-SSO users are unaffected.
- Not changing the existing `docker-build-community-%` per-arch targets.

## Decisions

### D1: Compose `image:` override (Option A) — not a `build:` directive

**Decision**: Add `services.signoz.image: signoz-community:${SIGNOZ_COMMUNITY_TAG:-local}` to `docker-compose-entra-sso.yaml`. Operators must produce the image via `make` before `docker compose up`.

**Rationale**:
1. `cmd/community/Dockerfile` is an assembly stage that depends on pre-built `target/linux-amd64/signoz-community` and `frontend/build/`. A Compose `build:` directive would fail without a Make pre-step, so it would not actually simplify operator workflow.
2. Implementing self-contained build inside Compose would require rewriting `cmd/community/Dockerfile` as a multi-stage golang+node+alpine build — out of scope for a deployment-config fix and high-conflict-risk against upstream.
3. Compose-managed builds skip rebuilds without `--build`. Operators forgetting `--build` after `git pull` and running stale binaries is a known support burden; explicit Make + `image:` makes the build step visible.

**Alternatives considered**:
- **Compose `build:` directive**: rejected per #1 above. Possible future revisit if the Dockerfile is ever made self-contained.
- **Override the registry prefix** (e.g., `docker.io/signoz/signoz-community:...`): rejected. Tagging under a registry path the image was never pushed to confuses operators reading `docker images` (looks like a published image). Bare `signoz-community:local` is unambiguously local-only. (BossArchitect refinement.)

### D2: Convenience Make target `docker-build-community-local`

**Decision**: Add a single Make target that wraps `docker-build-community-$(ARCH)` and re-tags the result to `signoz-community:local` (the default the compose overlay expects). The target uses the existing `ARCH ?= $(shell uname -m | …)` auto-detection from `Makefile:8`, so M-series Mac and Linux/amd64 operators both run the same command.

**Sketch** (final form lives in `tasks.md`):
```make
.PHONY: docker-build-community-local
docker-build-community-local: docker-build-community-$(ARCH) ## Build community image and tag :local for SSO compose overlay (auto-detects host arch)
	@docker tag $(DOCKER_REGISTRY_COMMUNITY):$(VERSION)-$(ARCH) signoz-community:local
	@echo ">> tagged signoz-community:local — run: docker compose -f deploy/docker/docker-compose.yaml -f deploy/docker/docker-compose-entra-sso.yaml up -d"
```

**Rationale**:
- Operators fumble the tag-mismatch step. Convenience target collapses build + re-tag + next-step echo into one command.
- Two lines of Make — near-zero maintenance cost vs. real support cost of tag mismatches yielding `manifest unknown` errors.
- Auto-detected `ARCH` works for amd64 and arm64 hosts without operator decision.
- Naming `docker-build-community-local` matches the existing `docker-build-community-*` family so it surfaces in `make help` as part of that group.
- The trailing `echo` reduces operator round-trips back to the docs after a successful build.

**Alternatives considered**:
- **`sso-deploy-build` name**: rejected per BossArchitect — reads awkward and doesn't slot into the existing target family.
- **Docs alone, no target**: rejected. Re-tagging is exactly the step where operators fumble; encoding it in Make is two lines and pays for itself the first time it prevents a support ticket.
- **Auto-publishing/pushing to a registry**: out of scope.

### D3: Bare image name `signoz-community:local` (no registry prefix)

**Decision**: The compose override uses `image: signoz-community:${SIGNOZ_COMMUNITY_TAG:-local}` — bare image name, no `docker.io/signoz/...` prefix. The convenience target tags as `signoz-community:local` directly.

**Rationale** (BossArchitect refinement):
- Tagging under `docker.io/signoz/signoz-community:local` would look like a published image when operators run `docker images`, despite never being published. Confusing.
- Bare `signoz-community:local` makes its local-only nature obvious.
- The `DOCKER_REGISTRY_COMMUNITY` variable is still used inside `docker-build-community-%` for the per-arch tag (e.g., `docker.io/signoz/signoz-community:<version>-amd64`); we only re-tag *the local convenience copy* to the bare name.

**Alternatives considered**:
- **Keep registry prefix in compose override**: rejected per above.

### D4: Operator-facing env var `SIGNOZ_COMMUNITY_TAG`

**Decision**: The image override uses `${SIGNOZ_COMMUNITY_TAG:-local}` so operators who want to pin to a specific build (e.g., the `<branch>-<sha>-amd64` tag from `docker-build-community-amd64`) can do so via `.env` without editing the overlay.

**Rationale**:
- Default `:local` works with the convenience target out of the box.
- Operators with custom build pipelines (e.g., CI-built images they pull instead of build locally) can override via `.env`.
- Pattern matches the existing `${VERSION:-v0.119.0}` in the base compose file — no new convention.

### D5: Operator guide gets a new top-of-file section

**Decision**: Add a "Build the community image" section to `docs/operator-guide.md` placed **before** the existing Prerequisites section, with cross-reference. It explains:
1. Why the upstream `signoz/signoz` image cannot service SSO-first deployments (variant difference, `BootstrapEntraSSO` lives in `cmd/community/server.go` only) — load-bearing rationale up front.
2. Build prerequisites: Go toolchain, Node + Yarn (for `js-build`), Docker.
3. The single-command path: `make docker-build-community-local`.
4. The manual two-step fallback (for operators who want to inspect what the convenience target does).
5. The expected resulting image tag (`signoz-community:local`) so operators can verify with `docker images` before `docker compose up`.

The existing Prerequisites section gains a one-line cross-reference to the new section so readers reaching it via the table of contents are not lost.

**Rationale**:
- Placing the build step before "Azure Entra Configuration" is the right ordering — operators can do the build while the Azure portal walkthrough is still ahead of them.
- "Why upstream image won't work" is the load-bearing rationale that prevents operators from second-guessing the build step. Stating it up front prevents support tickets.

### D6: Semantic shift in `docker-compose-entra-sso.yaml`

**Decision** (documenting, not a behavior change): the SSO overlay was previously **env-only** (no `services.signoz.image:` line). This change expands it to **env + image override**.

**Rationale**:
- This is a meaningful but localized semantic shift worth flagging so future maintainers understand why the overlay is no longer a "pure env layer".
- The shift is justified by the variant-mismatch problem — without overriding `image:`, no amount of env vars makes SSO work.
- Documented here in `design.md`; flagged in PR description; not surfaced in operator-facing docs (operators don't need to know overlay anatomy).

### D7: `.env.example` documents `SIGNOZ_COMMUNITY_TAG` as a commented optional override

**Decision**: Add a commented `# SIGNOZ_COMMUNITY_TAG=local` entry near the top of `.env.example`, alongside `COMPOSE_PROJECT_NAME`, with an explanatory comment. Default behavior (variable unset) flows through to `:local`, which matches the convenience target.

**Rationale**:
- `.env.example` is the discovery surface for env vars; an undocumented override variable is invisible.
- Keeping it commented signals "optional override" rather than "required setting".

## Risks / Trade-offs

- **[Risk] Operators run `make docker-build-community-amd64` (per-arch target) and forget the re-tag step** → Mitigation: convenience target `docker-build-community-local` does both. Docs lead with the convenience target; the manual two-step is shown second as a "what just happened" explanation.

- **[Risk] M-series Mac operators on `arm64` hosts wonder which arch to build** → Mitigation: convenience target uses `$(ARCH)` auto-detection (existing `Makefile:8` logic). Documented in `design.md` and called out in the doc section.

- **[Risk] Operators with pre-existing `signoz-community:local` images get stale builds after a `git pull`** → Mitigation: documented in operator guide ("re-run `make docker-build-community-local` after pulling new code"). Compose `image:` semantics make this visible — `docker compose up` won't auto-rebuild, so the stale image stays put until the operator rebuilds. Chosen explicitly over `build:` directive precisely so the build step is visible (D1).

- **[Risk] `js-build` can be slow on first run (Yarn cold cache)** → Mitigation: documented as a one-time cost. Subsequent builds reuse the Yarn cache. Not a blocker.

- **[Risk] Operator forgets to set `SIGNOZ_USER_ROOT_*` and the bootstrap retry loop times out at 90s with a confusing error** → Out of scope here (PR #2 territory) but the existing operator guide covers it. The new "Build the community image" section does not duplicate; it cross-references.

- **[Trade-off] The override expands the SSO overlay from "env-only" to "env + image"** (D6) → Accepted. The variant mismatch makes a pure-env overlay incapable of solving the problem. Documented for future maintainers.

- **[Trade-off] We require Go + Node/Yarn + Docker on operator hosts** → Accepted. This is the existing community-build prerequisite chain; we're just calling it out for operators who previously assumed they only needed Docker. Future work could publish a community image to a public registry to remove this — out of scope.

## Migration Plan

This is an additive deployment-config change. No data migration. Steps:
1. Operators on PR #2 + this fix: pull the merged branch, run `make docker-build-community-local`, then `docker compose up`. The convenience target produces `signoz-community:local`, which the new compose overlay references.
2. Operators with existing `signoz-community:local` from a prior manual build: their existing tag still works; they just need to ensure it's freshly built off the merged code.
3. Rollback: revert this change. The overlay returns to env-only behavior; operators are back to the (broken) state where the upstream enterprise image is used. There is no destructive migration.

## Open Questions

- None at design time. BossArchitect signed off on Option A, the convenience target naming, and the `signoz-community:local` bare-name decision. If review surfaces additional concerns we can revisit before archive.
