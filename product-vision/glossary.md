# Glossary

User-facing vocabulary used throughout the project documentation.

| Term | Definition |
|---|---|
| **Azure Entra ID** | Microsoft's cloud-based identity and access management service (formerly Azure Active Directory / Azure AD). The identity provider this project integrates with. |
| **OIDC (OpenID Connect)** | An authentication protocol built on top of OAuth 2.0. Provides identity verification via ID tokens issued by an identity provider. The only protocol used by this project. |
| **MSAL (Microsoft Authentication Library)** | Microsoft's official library for integrating applications with Entra ID via OIDC/OAuth 2.0. Used on the backend to handle token acquisition and validation. |
| **SSO (Single Sign-On)** | An authentication pattern where a user logs in once with a central identity provider and gains access to multiple applications without re-entering credentials. |
| **JIT Provisioning (Just-in-Time Provisioning)** | Automatically creating a user account in SigNoz the first time that user authenticates via SSO. No pre-registration or manual account creation required. |
| **Enterprise Application** | An Entra ID concept: a service principal that represents an external application (in this case SigNoz) within an Entra tenant. Users must be assigned to the Enterprise Application to authenticate. |
| **Group Claim** | A claim included in the OIDC token that lists the Entra security groups a user belongs to. Used to determine the user's role in SigNoz. |
| **Tenant** | An Entra ID tenant represents an organisation's directory. Identified by a tenant ID (GUID). Each deployment of this project connects to exactly one tenant. |
| **Client ID** | The application (client) ID assigned to the SigNoz Enterprise Application registration in Entra ID. Used to identify the application during OIDC flows. |
| **Client Secret** | A credential associated with the Entra application registration, used by the backend to authenticate itself to Entra during token exchange. Must be kept secret. |
| **ID Token** | A JWT issued by Entra ID after successful authentication. Contains claims about the user (name, email, group memberships) that SigNoz uses for provisioning and role assignment. |
| **Role Mapping** | The process of translating Entra group memberships into SigNoz roles (admin or general user). Configured via environment variables that associate Entra group object IDs with SigNoz role names. |
| **Query Service** | The SigNoz backend service that handles API requests, authentication, and data queries. The SSO adapter integrates here. |
| **SigNoz Community Edition** | The MIT-licensed, open-source edition of SigNoz. All project work targets this edition exclusively. |
