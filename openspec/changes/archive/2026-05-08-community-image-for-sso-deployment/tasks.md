## 1. Compose overlay image override

- [x] 1.1 Edit `deploy/docker/docker-compose-entra-sso.yaml`: add `image: signoz-community:${SIGNOZ_COMMUNITY_TAG:-local}` to the `signoz` service definition. Place it as the first key under `services.signoz` (above `environment`) so the override is visually prominent.
- [x] 1.2 Verify `docker compose -f deploy/docker/docker-compose.yaml -f deploy/docker/docker-compose-entra-sso.yaml config` exits 0 with `SIGNOZ_COMMUNITY_TAG` unset and the rendered config shows `image: signoz-community:local` for the `signoz` service.
- [x] 1.3 Verify the same compose config command exits 0 with `SIGNOZ_COMMUNITY_TAG=v0.119.0-amd64` exported and the rendered config shows `image: signoz-community:v0.119.0-amd64`.

## 2. .env.example documentation

- [x] 2.1 Edit `deploy/docker/.env.example`: add a commented `# SIGNOZ_COMMUNITY_TAG=local` entry near the top of the file (alongside `COMPOSE_PROJECT_NAME`), with an inline comment explaining it overrides the local-build image tag used by `docker-compose-entra-sso.yaml`.
- [x] 2.2 Confirm the comment cross-references the new operator-guide section (e.g., "see Build the community image in operator-guide.md").

## 3. Makefile convenience target

- [x] 3.1 Edit `Makefile`: add a `.PHONY: docker-build-community-local` target.
- [x] 3.2 Implement the target body so it depends on `docker-build-community-$(ARCH)`, then `docker tag $(DOCKER_REGISTRY_COMMUNITY):$(VERSION)-$(ARCH) signoz-community:local`.
- [x] 3.3 Add a help-discoverable `## ` description that mentions "SSO overlay" so `make help | grep sso` finds it.
- [x] 3.4 Append an `@echo` line that prints the next-step `docker compose ... up -d` command after a successful tag.
- [x] 3.5 Run `make help | grep -i community` and confirm the new target appears alongside the existing `docker-build-community-*` family.

## 4. Operator guide doc section

- [x] 4.1 Edit `docs/operator-guide.md`: insert a new top-level section titled "Build the community image" placed BEFORE the existing "Prerequisites" section (between the introduction and section 1).
- [x] 4.2 Update the Table of Contents to include the new section.
- [x] 4.3 In the new section, write a short paragraph explaining why the upstream `signoz/signoz` image cannot service SSO-first deployments — name the variant difference and that `BootstrapEntraSSO` is only invoked from `cmd/community/server.go`. Do NOT cite specific line numbers (they will rot).
- [x] 4.4 List build-time prerequisites: Go toolchain, Node.js + Yarn, Docker.
- [x] 4.5 Document the single-command path: `make docker-build-community-local` (auto-detects host architecture).
- [x] 4.6 Document the manual two-step fallback for operators who want to inspect what the convenience target does: `make docker-build-community-amd64` (or `arm64`) followed by `docker tag docker.io/signoz/signoz-community:<version>-amd64 signoz-community:local`.
- [x] 4.7 Document the expected resulting image tag (`signoz-community:local`) and a `docker images | grep signoz-community` verification step.
- [x] 4.8 Add a one-line cross-reference in the existing "Prerequisites" section pointing to the new "Build the community image" section.
- [x] 4.9 Note in the new section that operators should re-run `make docker-build-community-local` after pulling new code, since `docker compose up` does not auto-rebuild.

## 5. Build verification

- [x] 5.1 Run `make docker-build-community-amd64` end-to-end (Go cross-compile + js-build + docker build) and confirm a `docker.io/signoz/signoz-community:<version>-amd64` image is produced. (Slow — first js-build run can take several minutes.)
- [x] 5.2 Run `make docker-build-community-local` and confirm `signoz-community:local` appears in `docker images`.
- [x] 5.3 Run `docker compose -f deploy/docker/docker-compose.yaml -f deploy/docker/docker-compose-entra-sso.yaml config` and confirm the rendered `signoz` service references `signoz-community:local`.

## 6. Spec/change validation

- [x] 6.1 Run `openspec validate community-image-for-sso-deployment --strict` and confirm it passes.
- [x] 6.2 Sanity-check that the overlay still validates with the postgres overlay layered on: `docker compose -f deploy/docker/docker-compose.yaml -f deploy/docker/docker-compose-entra-sso.yaml -f deploy/docker/docker-compose-postgres.yaml config`.

## 7. Review and archive

- [x] 7.1 Request Reviewer (via Orchestrator) to inspect proposal, design, specs, tasks, and the eventual implementation diff.
- [x] 7.2 Address any Reviewer feedback.
- [ ] 7.3 Run `openspec archive community-image-for-sso-deployment` to move the change into the archive.
- [ ] 7.4 Commit, push the branch, open the PR.
- [ ] 7.5 Notify Orchestrator with `DONE: <PR URL>`.
