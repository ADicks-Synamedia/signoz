# Persona: Deployer / Operator

## Who they are

A DevOps engineer, infrastructure engineer, or platform team member responsible for deploying and operating the SigNoz Docker Compose stack. They manage the host environment, container orchestration, PostgreSQL database, networking, and upgrades. They may or may not be the same person as the Platform Admin.

## What they want

- **Single-command deployment**: `docker compose up` brings up a complete, working SigNoz instance with SSO pre-configured. No multi-step manual setup after containers are running.
- **Environment-variable configuration**: All Entra SSO settings are injected via environment variables in the Compose file or `.env` file. No config files to mount, template, or manage separately.
- **Observable operations**: Clear container logs, health checks, and predictable failure modes. When something goes wrong (bad credentials, unreachable Entra endpoint), the error is obvious and actionable.
- **Upgrade path**: Ability to update SigNoz containers without losing SSO configuration or user data.

## What they do NOT want

- To modify source code or rebuild container images to enable SSO.
- To manage complex secrets management infrastructure beyond what Docker Compose supports.
- To debug auth protocol details — that is the Platform Admin's domain.

## Key interactions

1. Clones the repository or pulls the Docker Compose configuration.
2. Populates environment variables (either in `.env` or directly in the Compose file) with Entra SSO parameters provided by the Platform Admin.
3. Runs `docker compose up -d` and verifies all containers are healthy.
4. Monitors container logs and resource usage in steady state.
5. Performs upgrades by pulling new images and re-deploying.

## Success looks like

The operator treats SSO as just another set of environment variables in the Compose stack. Deployment is no more complex with SSO enabled than without it. Containers start cleanly, and failures produce clear log messages.
