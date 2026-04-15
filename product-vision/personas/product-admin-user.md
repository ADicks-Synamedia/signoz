# Persona: Platform Admin

## Who they are

A senior engineer, tech lead, or platform team member responsible for configuring and maintaining the SSO integration between Entra ID and SigNoz. They have access to both the Entra ID admin portal and the SigNoz deployment configuration. They may also use SigNoz day-to-day as an end user.

## What they want

- **Clear group-to-role mapping**: A straightforward way to say "members of Entra group X get SigNoz role Y" using environment variables.
- **Confidence in auth health**: Visibility into whether SSO is working — users are logging in successfully, role mappings are being applied correctly.
- **Minimal ongoing maintenance**: Once configured, the integration should not require frequent intervention. Group membership changes in Entra should be reflected automatically at next login.

## What they do NOT want

- To manually provision or deprovision individual SigNoz user accounts.
- To debug opaque auth failures with no logging or error messages.
- To modify application source code to change role mappings.

## Key interactions

1. Registers the SigNoz application in Entra ID (Enterprise Application, app registration).
2. Configures group claims in the Entra app registration.
3. Sets environment variables for tenant ID, client ID, client secret, and group-to-role mappings.
4. Verifies that a test user can log in and receives the expected role.
5. Monitors auth-related logs for errors or unexpected behaviour.

## Success looks like

The admin configures SSO once via environment variables, verifies it works with a test login, and then rarely needs to touch it again. New users get the right access automatically based on their Entra group membership.
