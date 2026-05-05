# Persona: SSO End User

## Who they are

A developer, SRE, or engineering manager in an organisation that uses Microsoft Entra ID for identity. They use SigNoz daily to view dashboards, query traces, search logs, and investigate incidents. They are not responsible for configuring authentication or managing the SigNoz deployment.

## What they want

- **Seamless login**: Click a single button or be redirected to their familiar Entra ID login page. No separate SigNoz credentials to create, remember, or rotate.
- **Immediate access**: On first login, their account exists automatically with the correct permissions. No waiting for an admin to create their account.
- **Correct permissions**: Their access level (admin features vs read-only dashboards) matches their team's expectations, derived from their Entra group membership.

## What they do NOT want

- To manage or understand the SSO configuration.
- To maintain a separate password for SigNoz.
- To file a ticket and wait for account provisioning before they can use the platform.

## Key interactions

1. Navigates to the SigNoz URL.
2. Is redirected to the Entra ID login page (or is already signed in via browser SSO).
3. Authenticates with Entra ID (password, MFA, etc. — managed by Entra policy, not SigNoz).
4. Lands on the SigNoz dashboard with role-appropriate access.

## Success looks like

The user does not think about authentication. They open SigNoz and they are in — with the right level of access, every time.
