# Overview

## Elevator Pitch

This project adds an Azure Entra ID SSO adapter to the SigNoz community edition (MIT-licensed) observability platform. It enables organisations to authenticate users via Microsoft Entra ID using OIDC/MSAL, with group-based role assignment: Entra group memberships determine whether a user gets admin or general-user access inside SigNoz. Users are just-in-time provisioned on first SSO login. The solution is packaged as a full-stack Docker Compose deployment with PostgreSQL backing, configured entirely through environment variables, and does not depend on or reference any SigNoz enterprise-edition code.

## Problem Statement

Organisations that use Microsoft Entra ID as their identity provider currently have no supported path to single sign-on with SigNoz's community edition. Teams must maintain separate local credentials, manually provision accounts, and assign roles by hand. This creates friction for end users (yet another password), risk for security teams (no centralised credential revocation), and toil for platform administrators (manual onboarding/offboarding). The enterprise edition of SigNoz offers SSO capabilities, but many teams need a community-edition-compatible solution that integrates with their existing Entra ID tenant.

## Audiences

This project serves three distinct audiences:

| Persona | Summary |
|---|---|
| **SSO End User** | Logs in to SigNoz via Entra ID SSO. Sees dashboards, traces, and logs based on their assigned role. Does not configure auth. |
| **Platform Admin** | Configures the SSO integration, manages Entra group-to-SigNoz-role mappings, and monitors auth health. May also be an end user. |
| **Deployer / Operator** | Sets up the Docker Compose stack, provides Entra configuration via environment variables, manages PostgreSQL, handles upgrades and troubleshooting. |

See `personas/` for detailed persona descriptions.
