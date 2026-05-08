## Why

Our Entra SSO adapter ships in source on this branch but never reaches the runtime container. `deploy/docker/docker-compose.yaml:112` pins `image: signoz/signoz:${VERSION:-v0.119.0}`, which is the upstream **enterprise** variant — confirmed by the runtime log line `"variant":"enterprise"` and by the fact that `cmd/enterprise/Dockerfile` is what produces that public tag. `BootstrapEntraSSO` is only invoked from `cmd/community/server.go:129`; the enterprise main never calls it. Result: an operator who follows `docs/operator-guide.md` today still gets a container that has zero awareness of `SIGNOZ_ENTRA_*`, the OIDC AuthDomain is never created, and SSO logins fail with no useful diagnostic.

We cannot modify `ee/` or `cmd/enterprise/` source (license boundary). The fix is to give operators a clear, supported path to build and run the **community** image — which already contains the SSO adapter — instead of pulling the upstream enterprise image.

## What Changes

- **Override the `signoz` service image** in `deploy/docker/docker-compose-entra-sso.yaml` so the SSO overlay points at the locally-built community image tag instead of inheriting the upstream `signoz/signoz:v0.119.0` from the base compose file.
- **Document the build step** in `docs/operator-guide.md` with a new top-of-file section (before "Prerequisites") that explains why the upstream image cannot service SSO-first deployments and walks operators through producing the community image with `make`.
- **Add prerequisites** for the build: Go toolchain, Node + Yarn (for the frontend), Docker. These are already needed for development but were not previously called out for operators.
- **Optionally add a `make` convenience target** that bundles `go-build-community-<arch>` + `js-build` + `docker-build-community-<arch>` + tag-to-`:local` into one step. (Pending BossArchitect sign-off — see [design.md](./design.md).)
- **Document the resulting image tag** so operators know what to expect from the build and can verify before `docker compose up`.

This change is **strictly additive** at the deployment-config layer. It does not modify any Go source, any `cmd/enterprise/` source, any `ee/` source, or the existing build pipeline. It modifies one compose overlay, one doc file, and (conditionally) adds one Makefile target.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `docker-compose-entra-sso`: the SSO compose overlay overrides the `signoz` service image so it points at the community-variant image (which contains the Entra SSO adapter) rather than inheriting the upstream enterprise image from the base compose file. The capability gains a requirement that the overlay declare an image override and a corresponding `.env.example` entry that documents the override variable.

## Impact

- **Modified**: `deploy/docker/docker-compose-entra-sso.yaml` — add `image:` override on the `signoz` service pointing at `docker.io/signoz/signoz-community:${SIGNOZ_COMMUNITY_TAG:-local}` (or equivalent — final tag pattern decided in `design.md`).
- **Modified**: `deploy/docker/.env.example` — add a commented `# SIGNOZ_COMMUNITY_TAG=local` entry near the top so operators discover the variable.
- **Modified**: `docs/operator-guide.md` — add a new "Build the community image" section near the top, plus a prerequisite for Go/Node/Yarn/Docker.
- **Possibly added**: `Makefile` — convenience target `sso-deploy-build` (or similar) that wraps `docker-build-community-$(ARCH)` and re-tags to `:local`. Pending BossArchitect.
- **Not modified**: any Go source, any `ee/` or `cmd/enterprise/` source, the base `docker-compose.yaml`, the existing `cmd/community/Dockerfile`, the existing build targets in `Makefile`.
- **No new runtime dependencies** for the deployed container. The build prerequisites (Go, Node, Yarn, Docker) are already implicit for anyone building from source — this change just calls them out for operators.
- **Operator workflow change**: operators must now run a build step before `docker compose up` for SSO deployments. This is a one-time cost per branch/version; the resulting image is cached locally.
