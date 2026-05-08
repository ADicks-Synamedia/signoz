# Operator Deployment Guide — SigNoz with Azure Entra ID SSO

> **Audience**: Platform engineers and IT administrators deploying SigNoz with Entra ID single sign-on  
> **Last updated**: 2026-05-06

This guide walks through deploying SigNoz Community Edition with Azure Entra ID (formerly Azure AD) as the identity provider. By the end, users in your organization will log in to SigNoz via Entra, with their SigNoz role determined by Entra security group membership.

---

## Table of Contents

1. [Build the Community Image](#1-build-the-community-image)
2. [Prerequisites](#2-prerequisites)
3. [Azure Entra Configuration](#3-azure-entra-configuration)
4. [SigNoz Deployment](#4-signoz-deployment)
5. [Verification](#5-verification)
6. [Troubleshooting](#6-troubleshooting)
7. [Security Considerations](#7-security-considerations)

---

## 1. Build the Community Image

**You must build a custom SigNoz container image before running the SSO compose overlay.** The upstream `signoz/signoz` image published to Docker Hub is the **enterprise** variant. Our Entra ID SSO adapter and its bootstrap logic ship in the **community** variant only — `BootstrapEntraSSO` is wired into the community server entry point (`cmd/community/server.go`); the enterprise entry point does not invoke it. Running the upstream image with `SIGNOZ_ENTRA_*` env vars produces a container that silently ignores them, the OIDC AuthDomain is never created, and SSO logins fail with no useful diagnostic.

Building the community variant locally is the supported path until a community image is published.

### 1a. Build prerequisites

The community image is assembled from artifacts produced by Go and the frontend toolchain. Operators need:

- **Go** (matches the version pinned in `go.mod`) — required for the cross-compiled `signoz-community` binary.
- **Node.js + Yarn** — required for `yarn install && yarn build` of the React frontend (`frontend/build/`).
- **Docker** — required for the final `docker build` step that assembles the Alpine-based runtime image.

These are required only at build time. The resulting container image runs on plain Docker like any published image.

### 1b. Build the image (one command)

From the repository root:

```bash
make docker-build-community-local
```

This auto-detects your host architecture (`amd64` or `arm64`), runs the Go cross-compile, builds the frontend, builds the Docker image, and tags it `signoz-community:local` — which is the default tag the SSO compose overlay references. The first run can take several minutes (cold Yarn cache, full Go module download); subsequent runs are faster.

On success, the target prints the next-step `docker compose ... up -d` command.

### 1c. Manual two-step fallback

If you want to inspect what the convenience target does, the equivalent two commands are:

```bash
# 1. Build the per-arch image (use amd64 or arm64 to match your host)
make docker-build-community-amd64

# 2. Re-tag to the name the SSO compose overlay expects
docker tag docker.io/signoz/signoz-community:<branch>-<sha>-amd64 signoz-community:local
```

The `<branch>-<sha>-amd64` portion comes from `Makefile`'s default `VERSION` (current branch name and short SHA). Run `docker images | grep signoz-community` to see the exact tag the build produced.

### 1d. Verify the build

```bash
docker images | grep signoz-community
```

You should see at least two entries:

- `docker.io/signoz/signoz-community   <branch>-<sha>-<arch>` — produced by the per-arch build.
- `signoz-community                    local` — the alias the SSO compose overlay references.

### 1e. Rebuild after pulling new code

`docker compose up -d` does **not** rebuild the image automatically. After `git pull` (or any local change to Go or frontend code), re-run `make docker-build-community-local` before the next `docker compose up -d`. Otherwise Compose will reuse the stale `signoz-community:local` image.

If you want to pin a specific build instead of always pointing at `:local`, give it a stable name first and reference that name via `SIGNOZ_COMMUNITY_TAG`. The compose overlay expects a tag under the bare `signoz-community` repository (no `signoz/` prefix), so the per-arch image produced by `make` needs to be re-tagged before it can be pinned:

```bash
# 1. Re-tag the per-arch image under the bare repository with a stable label
docker tag docker.io/signoz/signoz-community:<branch>-<sha>-amd64 signoz-community:my-pin

# 2. In your .env, set:
#    SIGNOZ_COMMUNITY_TAG=my-pin
# Compose will then resolve image: signoz-community:my-pin on the signoz service.
```

Run `docker images | grep signoz-community` to confirm both the per-arch tag and your pinned alias are present before `docker compose up -d`.

---

## 2. Prerequisites

> **Note**: If you have not already produced the community image, see [Build the Community Image](#1-build-the-community-image) above. The Docker Compose stack will not start without it.

Before you begin, make sure you have:

- **Docker and Docker Compose** installed (Compose V2 recommended).
- **Azure Entra ID tenant** with administrator access (you will need to create app registrations and security groups).
- **A domain or hostname for SigNoz** — either a real domain (e.g., `signoz.corp.com`) or `localhost` for local testing.
- **TLS termination** (production only) — a reverse proxy such as nginx, Caddy, or Traefik that terminates HTTPS in front of SigNoz.
- **Credentials for a bootstrap admin user** — for SSO-first deployments you must set `SIGNOZ_USER_ROOT_*` so the server creates the first organization on its own (see [4a. Prepare the Environment File](#4a-prepare-the-environment-file)). This admin is also a break-glass login if Entra is misconfigured or unreachable.

### Why TLS is required for production

Azure Entra ID requires HTTPS redirect URIs for production applications. The SigNoz Docker stack serves HTTP on port 8080 — it does not terminate TLS itself. In production, you must place a reverse proxy in front of SigNoz that handles TLS and forwards traffic to `http://signoz:8080`.

For **local testing**, Entra makes an exception: `http://localhost` redirect URIs are permitted without HTTPS. This means you can test the full SSO flow on your development machine without a certificate.

---

## 3. Azure Entra Configuration

Complete these steps in the [Azure Portal](https://portal.azure.com) before deploying SigNoz.

### 3a. Create Security Groups

Security groups control which SigNoz role each user receives after login.

1. Navigate to **Azure Portal → Microsoft Entra ID → Groups → New group**.
2. Set **Group type** to **Security**.
3. Create at minimum one group for administrators:
   - **Name**: `SigNoz-Admins` (or whatever fits your naming convention)
   - Click **Create**.
4. Optionally create a second group for editors:
   - **Name**: `SigNoz-Editors`
   - Click **Create**.
5. **Record the Object ID** of each group — you will need these GUIDs later. Find them under **Groups → select the group → Overview → Object Id**.

> **Note on viewers**: There is no separate viewer group. Users who authenticate but do not match any group mapping receive the default role, which is VIEWER unless you override it. If you want to restrict who can access SigNoz at all, control that via Enterprise Application assignment (step 3e), not via a viewer group.

### 3b. Register the Application

1. Navigate to **Azure Portal → Microsoft Entra ID → App registrations → New registration**.
2. Fill in:
   - **Name**: `SigNoz` (or any descriptive name)
   - **Supported account types**: **Accounts in this organizational directory only** (single tenant)
   - **Redirect URI**:
     - **Type**: Web
     - **URI**: `https://<your-signoz-host>/api/v1/complete/oidc`
     - For local testing: `http://localhost:8080/api/v1/complete/oidc`
3. Click **Register**.
4. On the app's **Overview** page, record:
   - **Application (client) ID** — this is `SIGNOZ_ENTRA_CLIENT_ID`
   - **Directory (tenant) ID** — this is `SIGNOZ_ENTRA_TENANT_ID`

**Why single tenant?** Multi-tenant would allow any Azure user from any organization to attempt login. Single tenant restricts authentication to users in your directory only.

### 3c. Create a Client Secret

1. In your app registration, go to **Certificates & secrets → Client secrets → New client secret**.
2. Enter a description (e.g., `SigNoz SSO`) and select an expiration period.
3. Click **Add**.
4. **Copy the secret Value immediately** — it is only shown once. This is `SIGNOZ_ENTRA_CLIENT_SECRET`.

> **Warning — secret expiration**: Client secrets have a maximum lifetime of 24 months. When a secret expires, SSO logins will fail. Set a calendar reminder before the expiration date.
>
> **To rotate a secret**: Create a new secret in the Azure Portal, update `SIGNOZ_ENTRA_CLIENT_SECRET` in your `.env` file, and restart the SigNoz container (`docker compose restart signoz`). You can delete the old secret in Azure after confirming the new one works.

### 3d. Configure Token Claims

This step tells Entra to include group membership information in the ID token so SigNoz can map users to roles.

1. In your app registration, go to **Token configuration → Add groups claim**.
2. Select **Security groups**.
3. Click **Add**.

That's all that's needed. The `openid`, `email`, and `profile` scopes are sufficient — no additional API permissions are required, and no admin consent is needed.

> **What this does**: When a user authenticates, Entra includes a `groups` claim in the ID token containing the Object IDs of all security groups the user belongs to. SigNoz matches these IDs against the admin and editor group IDs you configure to determine the user's role.

### 3e. Assign Users and Groups to the Enterprise Application

This step controls **who is allowed to log in** to SigNoz.

1. Navigate to **Azure Portal → Microsoft Entra ID → Enterprise Applications**.
2. Find and select your SigNoz application.
3. Go to **Users and groups → Add user/group**.
4. Assign individual users or groups that should have access to SigNoz.

> **Important — understand the two-layer model**:
>
> - **Enterprise Application assignment** (this step) controls **who can log in**. A user not assigned here will be denied access entirely.
> - **Security group membership** (step 3a) controls **what role they get** after login. A user's group membership determines whether they are an Admin, Editor, or Viewer.
>
> These are independent. A user must be:
> 1. Assigned to the Enterprise Application (to authenticate at all), **AND**
> 2. In a security group (to receive a non-default role)
>
> A user who is assigned to the Enterprise App but not in any security group will log in successfully with the default VIEWER role.

---

## 4. SigNoz Deployment

### 4a. Prepare the Environment File

`deploy/docker/.env.example` is the complete template. The actual `.env` file is gitignored — operators create it once per deployment.

1. From the `deploy/docker/` directory, copy the example environment file:

   ```bash
   cp .env.example .env
   ```

2. Open `.env` and fill in:

   - **`COMPOSE_PROJECT_NAME=signoz`** — leave as the default. This keeps Docker container, volume, and network prefixes deterministic so the names referenced throughout this guide (e.g. `signoz-signoz-1`) line up with what you see in `docker ps`.
   - **`SIGNOZ_USER_ROOT_*`** — required for SSO-first deployments (see [Required: bootstrap admin user](#4b-required-bootstrap-admin-user) below).
   - **`SIGNOZ_ENTRA_*`** — Entra app registration values from section 3 (specifically the tenant ID, client ID, and client secret recorded in subsections 3b and 3c).

   The relevant rows look like:

   ```bash
   # --- Base ---
   COMPOSE_PROJECT_NAME=signoz

   # --- Required for SSO-first deployments ---
   SIGNOZ_USER_ROOT_ENABLED=true
   SIGNOZ_USER_ROOT_EMAIL=admin@corp.com
   SIGNOZ_USER_ROOT_PASSWORD=replace-with-a-strong-password
   SIGNOZ_USER_ROOT_ORG_NAME=default

   # --- Required Entra ---
   SIGNOZ_ENTRA_SSO_ENABLED=true
   SIGNOZ_ENTRA_TENANT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
   SIGNOZ_ENTRA_CLIENT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
   SIGNOZ_ENTRA_CLIENT_SECRET=your-secret-value
   SIGNOZ_ENTRA_DOMAIN=corp.com

   # --- Optional Entra ---
   # SIGNOZ_ENTRA_ADMIN_GROUP_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
   # SIGNOZ_ENTRA_EDITOR_GROUP_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
   # SIGNOZ_ENTRA_DEFAULT_ROLE=VIEWER
   ```

### 4b. Required: bootstrap admin user

Stock SigNoz CE relies on the first browser visit to create an organization via the self-signup form. With SSO enabled that form is replaced by the SSO redirect, so a fresh database has no organization — and SigNoz cannot accept any agent traffic until one exists. The fix is to set `SIGNOZ_USER_ROOT_*`, which tells SigNoz to create the bootstrap organization itself, plus an admin user, on first boot.

| Variable | Required | Purpose |
|---|---|---|
| `SIGNOZ_USER_ROOT_ENABLED` | Yes (set to `true`) | Master switch for the root user reconciler |
| `SIGNOZ_USER_ROOT_EMAIL` | Yes | Login email for the bootstrap admin |
| `SIGNOZ_USER_ROOT_PASSWORD` | Yes | Strong password for the bootstrap admin |
| `SIGNOZ_USER_ROOT_ORG_NAME` | No (defaults to `default`) | Display name for the bootstrap organization |

**The bootstrap admin is also your break-glass login.** If Entra ever becomes unreachable, or your app registration is misconfigured, this account is the only way to get back into SigNoz. Treat its credentials accordingly: store the password in a secret manager, rotate periodically, and consider scoping it to incident response only after SSO is verified working.

### 4c. Environment Variable Reference

| Variable | Required | Default | Where to Find |
|---|---|---|---|
| `SIGNOZ_ENTRA_SSO_ENABLED` | Yes | `false` | Set to `true` |
| `SIGNOZ_ENTRA_TENANT_ID` | Yes | — | Azure Portal → Entra ID → Overview → Tenant ID |
| `SIGNOZ_ENTRA_CLIENT_ID` | Yes | — | Azure Portal → App registrations → your app → Application (client) ID |
| `SIGNOZ_ENTRA_CLIENT_SECRET` | Yes | — | Azure Portal → App registrations → Certificates & secrets |
| `SIGNOZ_ENTRA_DOMAIN` | Yes | — | Your organization's email domain |
| `SIGNOZ_ENTRA_ADMIN_GROUP_ID` | No | — | Azure Portal → Entra ID → Groups → your admin group → Object Id |
| `SIGNOZ_ENTRA_EDITOR_GROUP_ID` | No | — | Azure Portal → Entra ID → Groups → your editor group → Object Id |
| `SIGNOZ_ENTRA_DEFAULT_ROLE` | No | `VIEWER` | One of: `ADMIN`, `EDITOR`, `VIEWER` |
| `COMPOSE_PROJECT_NAME` | Yes | — | Set to `signoz` (provided by `.env.example`); keeps container/volume names stable |
| `SIGNOZ_USER_ROOT_ENABLED` | Yes (SSO-first) | `false` | Set to `true` so the bootstrap org and admin user are created automatically |
| `SIGNOZ_USER_ROOT_EMAIL` | Yes (SSO-first) | — | Login email for the bootstrap admin / break-glass account |
| `SIGNOZ_USER_ROOT_PASSWORD` | Yes (SSO-first) | — | Strong password for the bootstrap admin |
| `SIGNOZ_USER_ROOT_ORG_NAME` | No | `default` | Display name for the bootstrap organization |

### 4d. Role Mapping Behavior

When a user logs in through Entra SSO, SigNoz determines their role as follows:

1. If the user's token `groups` claim contains `SIGNOZ_ENTRA_ADMIN_GROUP_ID` → **ADMIN**
2. If the user's token `groups` claim contains `SIGNOZ_ENTRA_EDITOR_GROUP_ID` → **EDITOR**
3. Otherwise → the value of `SIGNOZ_ENTRA_DEFAULT_ROLE` (defaults to **VIEWER**)

If a user is in both the admin and editor groups, the highest-privilege role wins (ADMIN > EDITOR > VIEWER).

Roles are assigned at **first login** via just-in-time provisioning. On subsequent logins, the existing user record is reused. To change a user's role, update their group membership in Entra — the role mapping is re-evaluated on each login during provisioning.

### 4e. Start SigNoz

From the `deploy/docker/` directory:

**With SQLite (default):**

```bash
docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml up -d
```

**With PostgreSQL** (recommended for production):

```bash
docker compose -f docker-compose.yaml -f docker-compose-entra-sso.yaml -f docker-compose-postgres.yaml up -d
```

When using PostgreSQL, you can optionally set `SIGNOZ_PG_PASSWORD` in your `.env` file. The default password is `signoz`.

SigNoz will be available at `http://localhost:8080` (or your configured host).

---

## 5. Verification

After deployment, verify the SSO flow end-to-end:

1. **Open SigNoz** — navigate to `http://localhost:8080` (or your production URL).
2. **Enter your email** — type an email address matching the `SIGNOZ_ENTRA_DOMAIN` you configured.
3. **Redirect to Entra** — you should be redirected to the Microsoft login page. If you are already signed in to Microsoft, you may be redirected back immediately.
4. **Authenticate** — complete the Entra login (password, MFA, etc.).
5. **Return to SigNoz** — you should be redirected back to SigNoz and landed on the dashboard.
6. **Verify role** — go to **Settings → General** in SigNoz to confirm your role matches your expected group membership:
   - Member of admin group → ADMIN
   - Member of editor group → EDITOR
   - No matching group → VIEWER (or your configured default)

If any step fails, see the [Troubleshooting](#6-troubleshooting) section below.

---

## 6. Troubleshooting

### How to view logs

SigNoz logs OIDC authentication events with structured context. To follow them in real time:

```bash
docker compose logs signoz -f
```

Filter for OIDC-specific messages:

```bash
docker compose logs signoz -f 2>&1 | grep "oidc:"
```

### OIDC discovery fails

**Symptom**: Log message `oidc: failed to create provider`.

**Cause**: SigNoz cannot reach the Entra OIDC discovery endpoint.

**Fix**: Verify that `SIGNOZ_ENTRA_TENANT_ID` is correct. The discovery URL is:
```
https://login.microsoftonline.com/{tenant-id}/v2.0/.well-known/openid-configuration
```
You can test this by opening the URL in a browser (substituting your tenant ID). If it returns a JSON document, the tenant ID is correct. Also check that the container has outbound internet access.

### Token verification fails

**Symptom**: Log message `oidc: failed to verify token`.

**Cause**: The ID token's issuer claim does not match what SigNoz expects, or the token has expired.

**Fix**: This is typically handled automatically — the adapter uses the `IssuerAlias` mechanism to reconcile Entra's issuer URL with its discovery URL. If you see this error, verify that the `SIGNOZ_ENTRA_TENANT_ID` and `SIGNOZ_ENTRA_CLIENT_ID` are correct. An incorrect client ID will cause audience validation to fail.

### Wrong redirect URI

**Symptom**: Entra shows an error like "AADSTS50011: The redirect URI specified in the request does not match" or the browser lands on an error page after authentication.

**Fix**: The redirect URI must match **exactly** between your Entra app registration and what SigNoz generates. Check:
- The path must be `/api/v1/complete/oidc` (not `/api/v1/complete/oidc/` with a trailing slash).
- The scheme must match: `https://` in production, `http://` for localhost.
- The host and port must match: if SigNoz is on port 8080, the redirect URI must include `:8080` (unless your reverse proxy serves on 443).

### Group claims missing

**Symptom**: Users log in successfully but always get the default VIEWER role regardless of group membership.

**Cause**: The ID token does not contain a `groups` claim.

**Fix**: In the Azure Portal, go to **App registrations → your app → Token configuration**. Verify that a **groups claim** is configured with **Security groups** selected. If it is missing, add it (step 3d).

> **Note**: If a user is a member of more than 150 groups, Entra returns an "overage" indicator instead of inline group claims. This is a known Entra limitation. The workaround is to reduce the user's group count or use application-specific group assignments in Entra.

### Role mapping not working

**Symptom**: Group claims are present in the token, but users still get the wrong role.

**Cause**: The group Object IDs in your `.env` file do not match the actual group Object IDs in Entra.

**Fix**: GUIDs must be exact. Copy them directly from **Azure Portal → Entra ID → Groups → select group → Object Id**. GUIDs are case-insensitive, but otherwise must match character-for-character. A common mistake is copying the Group Name instead of the Object ID.

### No one can log in

**Symptom**: Users are redirected to Entra, authenticate, but get an error instead of being redirected back to SigNoz.

**Cause**: Users are not assigned to the Enterprise Application.

**Fix**: Go to **Azure Portal → Enterprise Applications → your app → Users and groups** and verify that the relevant users or groups are assigned (step 3e). Remember: Enterprise Application assignment controls who can authenticate. A user not assigned here will be blocked by Entra before they ever reach SigNoz.

### Everyone gets VIEWER role

**Symptom**: All users log in successfully but everyone is a VIEWER.

**Fix**: Check two things:
1. **Group claims are enabled** — see "Group claims missing" above.
2. **Users are in the correct security groups** — being assigned to the Enterprise Application is not the same as being in a security group. Verify that users are members of the `SigNoz-Admins` or `SigNoz-Editors` groups you created in step 3a.
3. **Group Object IDs match** — verify the GUIDs in `SIGNOZ_ENTRA_ADMIN_GROUP_ID` and `SIGNOZ_ENTRA_EDITOR_GROUP_ID` match exactly.

### Client secret expired

**Symptom**: Log message `oidc: failed to get token` with an error description mentioning an invalid or expired client secret.

**Fix**:
1. In Azure Portal, go to **App registrations → your app → Certificates & secrets**.
2. Create a new client secret.
3. Update `SIGNOZ_ENTRA_CLIENT_SECRET` in your `.env` file with the new value.
4. Restart SigNoz:
   ```bash
   docker compose restart signoz
   ```
5. Delete the old secret from the Azure Portal after confirming the new one works.

### API permissions

No additional API permissions are needed beyond the defaults (`openid`, `email`, `profile`). Do **not** add `GroupMember.Read.All` or other Graph API permissions — SigNoz reads group membership from the ID token's `groups` claim (configured in step 3d), not from the Microsoft Graph API.

---

## 7. Security Considerations

### Protect the client secret

The `.env` file contains `SIGNOZ_ENTRA_CLIENT_SECRET`, which is a sensitive credential. Take care to:

- **Never commit `.env` to version control.** The `.env.example` file contains placeholder values and is safe to commit; `.env` with real values is not.
- Store the secret in a secrets manager (e.g., Azure Key Vault, HashiCorp Vault) in production, injecting it as an environment variable at runtime if your orchestration supports it.

### Use TLS in production

As noted in the prerequisites, the SigNoz Docker stack serves HTTP. In production:

- Place a reverse proxy (nginx, Caddy, Traefik) in front of SigNoz that terminates TLS.
- Update the Entra app registration redirect URI to use `https://`.
- Ensure the reverse proxy forwards the `Host` header so SigNoz generates correct redirect URLs.

HTTP is acceptable only for `localhost` testing.

### Single-tenant only

The Entra app registration must be configured as **single tenant** ("Accounts in this organizational directory only"). Multi-tenant registration would allow users from any Azure AD directory to attempt login, which is almost certainly not what you want.

### Client secret rotation

Establish a rotation schedule before secrets expire:

| Action | When |
|---|---|
| Create new secret | At least 1 week before expiration |
| Update `.env` and restart SigNoz | Same day as creation |
| Delete old secret | After verifying the new secret works |
| Set calendar reminder for next rotation | Immediately after rotation |

Entra allows a maximum secret lifetime of 24 months. Choose a rotation cadence that fits your organization's security policy.
