# Goals and Non-Goals

## Goals

These are the verifiable success criteria for the project. Each goal is independently testable.

### G1 — SSO Authentication via Entra ID

A user assigned to the Entra Enterprise Application can authenticate to SigNoz via OIDC using MSAL and land on the SigNoz dashboard — no local password required.

### G2 — Group-Based Role Mapping

Entra group memberships are read at login time and mapped to SigNoz's existing role model (admin vs general user). A user in the designated "admin" group gets admin privileges; others get viewer/general access.

### G3 — Just-in-Time User Provisioning

On first SSO login, a SigNoz user account is automatically created (JIT provisioning) with name, email, and role populated from Entra claims and group membership.

### G4 — Single-Command Deployment

A single `docker compose up` command deploys a complete, working SigNoz instance (query-service, frontend, alertmanager, OTel collector, PostgreSQL) with Entra SSO pre-configured via environment variables.

### G5 — Environment-Variable-Only Configuration

All Entra-specific configuration (tenant ID, client ID, client secret, group-to-role mappings) is provided exclusively through environment variables — no config files to mount or edit inside the container.

---

## Non-Goals

The following items are explicitly **out of scope** for this project.

### NG1 — SAML Support

OIDC only. No SAML federation.

### NG2 — Other Identity Providers

Entra ID only. No Okta, Google Workspace, or generic OIDC provider support.

### NG3 — SCIM / Full User Sync

JIT provisioning on first login only. No ongoing lifecycle sync from Entra.

### NG4 — Custom Role Creation

We map to SigNoz's existing role model only. No new role types are introduced.

### NG5 — Multi-Tenancy

Single-tenant deployment only. One Entra tenant, one SigNoz instance.

### NG6 — Migration Tooling

No tool to migrate existing local SigNoz users to Entra SSO.

### NG7 — Frontend UI Changes

Backend auth changes only. Login redirects to Entra; no custom login page work.
