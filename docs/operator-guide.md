# Operator Deployment Guide — SigNoz with Azure Entra ID SSO

> **Audience**: Platform engineers and IT administrators deploying SigNoz with Entra ID single sign-on  
> **Last updated**: 2026-05-06

This guide walks through deploying SigNoz Community Edition with Azure Entra ID (formerly Azure AD) as the identity provider. By the end, users in your organization will log in to SigNoz via Entra, with their SigNoz role determined by Entra security group membership.

---

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Azure Entra Configuration](#2-azure-entra-configuration)
3. [SigNoz Deployment](#3-signoz-deployment)
4. [Verification](#4-verification)
5. [Troubleshooting](#5-troubleshooting)
6. [Security Considerations](#6-security-considerations)

---

## 1. Prerequisites

Before you begin, make sure you have:

- **Docker and Docker Compose** installed (Compose V2 recommended).
- **Azure Entra ID tenant** with administrator access (you will need to create app registrations and security groups).
- **A domain or hostname for SigNoz** — either a real domain (e.g., `signoz.corp.com`) or `localhost` for local testing.
- **TLS termination** (production only) — a reverse proxy such as nginx, Caddy, or Traefik that terminates HTTPS in front of SigNoz.
- **Credentials for a bootstrap admin user** — for SSO-first deployments you must set `SIGNOZ_USER_ROOT_*` so the server creates the first organization on its own (see [3a. Prepare the Environment File](#3a-prepare-the-environment-file)). This admin is also a break-glass login if Entra is misconfigured or unreachable.

### Why TLS is required for production

Azure Entra ID requires HTTPS redirect URIs for production applications. The SigNoz Docker stack serves HTTP on port 8080 — it does not terminate TLS itself. In production, you must place a reverse proxy in front of SigNoz that handles TLS and forwards traffic to `http://signoz:8080`.

For **local testing**, Entra makes an exception: `http://localhost` redirect URIs are permitted without HTTPS. This means you can test the full SSO flow on your development machine without a certificate.

---

## 2. Azure Entra Configuration

Complete these steps in the [Azure Portal](https://portal.azure.com) before deploying SigNoz.

### 2a. Create Security Groups

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

> **Note on viewers**: There is no separate viewer group. Users who authenticate but do not match any group mapping receive the default role, which is VIEWER unless you override it. If you want to restrict who can access SigNoz at all, control that via Enterprise Application assignment (step 2e), not via a viewer group.

### 2b. Register the Application

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

### 2c. Create a Client Secret

1. In your app registration, go to **Certificates & secrets → Client secrets → New client secret**.
2. Enter a description (e.g., `SigNoz SSO`) and select an expiration period.
3. Click **Add**.
4. **Copy the secret Value immediately** — it is only shown once. This is `SIGNOZ_ENTRA_CLIENT_SECRET`.

> **Warning — secret expiration**: Client secrets have a maximum lifetime of 24 months. When a secret expires, SSO logins will fail. Set a calendar reminder before the expiration date.
>
> **To rotate a secret**: Create a new secret in the Azure Portal, update `SIGNOZ_ENTRA_CLIENT_SECRET` in your `.env` file, and restart the SigNoz container (`docker compose restart signoz`). You can delete the old secret in Azure after confirming the new one works.

### 2d. Configure Token Claims

This step tells Entra to include group membership information in the ID token so SigNoz can map users to roles.

1. In your app registration, go to **Token configuration → Add groups claim**.
2. Select **Security groups**.
3. Click **Add**.

That's all that's needed. The `openid`, `email`, and `profile` scopes are sufficient — no additional API permissions are required, and no admin consent is needed.

> **What this does**: When a user authenticates, Entra includes a `groups` claim in the ID token containing the Object IDs of all security groups the user belongs to. SigNoz matches these IDs against the admin and editor group IDs you configure to determine the user's role.

### 2e. Assign Users and Groups to the Enterprise Application

This step controls **who is allowed to log in** to SigNoz.

1. Navigate to **Azure Portal → Microsoft Entra ID → Enterprise Applications**.
2. Find and select your SigNoz application.
3. Go to **Users and groups → Add user/group**.
4. Assign individual users or groups that should have access to SigNoz.

> **Important — understand the two-layer model**:
>
> - **Enterprise Application assignment** (this step) controls **who can log in**. A user not assigned here will be denied access entirely.
> - **Security group membership** (step 2a) controls **what role they get** after login. A user's group membership determines whether they are an Admin, Editor, or Viewer.
>
> These are independent. A user must be:
> 1. Assigned to the Enterprise Application (to authenticate at all), **AND**
> 2. In a security group (to receive a non-default role)
>
> A user who is assigned to the Enterprise App but not in any security group will log in successfully with the default VIEWER role.

---

## 3. SigNoz Deployment

### 3a. Prepare the Environment File

`deploy/docker/.env.example` is the complete template. The actual `.env` file is gitignored — operators create it once per deployment.

1. From the `deploy/docker/` directory, copy the example environment file:

   ```bash
   cp .env.example .env
   ```

2. Open `.env` and fill in:

   - **`COMPOSE_PROJECT_NAME=signoz`** — leave as the default. This keeps Docker container, volume, and network prefixes deterministic so the names referenced throughout this guide (e.g. `signoz-signoz-1`) line up with what you see in `docker ps`.
   - **`SIGNOZ_USER_ROOT_*`** — required for SSO-first deployments (see [Required: bootstrap admin user](#3b-required-bootstrap-admin-user) below).
   - **`SIGNOZ_ENTRA_*`** — Entra app registration values from step 2.

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

### 3b. Required: bootstrap admin user

Stock SigNoz CE relies on the first browser visit to create an organization via the self-signup form. With SSO enabled that form is replaced by the SSO redirect, so a fresh database has no organization — and SigNoz cannot accept any agent traffic until one exists. The fix is to set `SIGNOZ_USER_ROOT_*`, which tells SigNoz to create the bootstrap organization itself, plus an admin user, on first boot.

| Variable | Required | Purpose |
|---|---|---|
| `SIGNOZ_USER_ROOT_ENABLED` | Yes (set to `true`) | Master switch for the root user reconciler |
| `SIGNOZ_USER_ROOT_EMAIL` | Yes | Login email for the bootstrap admin |
| `SIGNOZ_USER_ROOT_PASSWORD` | Yes | Strong password for the bootstrap admin |
| `SIGNOZ_USER_ROOT_ORG_NAME` | No (defaults to `default`) | Display name for the bootstrap organization |

**The bootstrap admin is also your break-glass login.** If Entra ever becomes unreachable, or your app registration is misconfigured, this account is the only way to get back into SigNoz. Treat its credentials accordingly: store the password in a secret manager, rotate periodically, and consider scoping it to incident response only after SSO is verified working.

### 3c. Environment Variable Reference

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

### 3d. Role Mapping Behavior

When a user logs in through Entra SSO, SigNoz determines their role as follows:

1. If the user's token `groups` claim contains `SIGNOZ_ENTRA_ADMIN_GROUP_ID` → **ADMIN**
2. If the user's token `groups` claim contains `SIGNOZ_ENTRA_EDITOR_GROUP_ID` → **EDITOR**
3. Otherwise → the value of `SIGNOZ_ENTRA_DEFAULT_ROLE` (defaults to **VIEWER**)

If a user is in both the admin and editor groups, the highest-privilege role wins (ADMIN > EDITOR > VIEWER).

Roles are assigned at **first login** via just-in-time provisioning. On subsequent logins, the existing user record is reused. To change a user's role, update their group membership in Entra — the role mapping is re-evaluated on each login during provisioning.

### 3e. Start SigNoz

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

## 4. Verification

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

If any step fails, see the [Troubleshooting](#5-troubleshooting) section below.

---

## 5. Troubleshooting

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

**Fix**: In the Azure Portal, go to **App registrations → your app → Token configuration**. Verify that a **groups claim** is configured with **Security groups** selected. If it is missing, add it (step 2d).

> **Note**: If a user is a member of more than 150 groups, Entra returns an "overage" indicator instead of inline group claims. This is a known Entra limitation. The workaround is to reduce the user's group count or use application-specific group assignments in Entra.

### Role mapping not working

**Symptom**: Group claims are present in the token, but users still get the wrong role.

**Cause**: The group Object IDs in your `.env` file do not match the actual group Object IDs in Entra.

**Fix**: GUIDs must be exact. Copy them directly from **Azure Portal → Entra ID → Groups → select group → Object Id**. GUIDs are case-insensitive, but otherwise must match character-for-character. A common mistake is copying the Group Name instead of the Object ID.

### No one can log in

**Symptom**: Users are redirected to Entra, authenticate, but get an error instead of being redirected back to SigNoz.

**Cause**: Users are not assigned to the Enterprise Application.

**Fix**: Go to **Azure Portal → Enterprise Applications → your app → Users and groups** and verify that the relevant users or groups are assigned (step 2e). Remember: Enterprise Application assignment controls who can authenticate. A user not assigned here will be blocked by Entra before they ever reach SigNoz.

### Everyone gets VIEWER role

**Symptom**: All users log in successfully but everyone is a VIEWER.

**Fix**: Check two things:
1. **Group claims are enabled** — see "Group claims missing" above.
2. **Users are in the correct security groups** — being assigned to the Enterprise Application is not the same as being in a security group. Verify that users are members of the `SigNoz-Admins` or `SigNoz-Editors` groups you created in step 2a.
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

No additional API permissions are needed beyond the defaults (`openid`, `email`, `profile`). Do **not** add `GroupMember.Read.All` or other Graph API permissions — SigNoz reads group membership from the ID token's `groups` claim (configured in step 2d), not from the Microsoft Graph API.

---

## 6. Security Considerations

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
