## ADDED Requirements

### Requirement: OIDC provider is registered in the auth provider map
The `NewAuthNs` function in `pkg/signoz/authn.go` SHALL include `authtypes.AuthNProviderOIDC` mapped to an `oidccallbackauthn.AuthN` instance in the returned provider map.

#### Scenario: OIDC provider available after initialization
- **WHEN** `NewAuthNs` is called
- **THEN** the returned map contains a key `authtypes.AuthNProviderOIDC` with a non-nil `CallbackAuthN` implementation

### Requirement: Startup bootstrap creates AuthDomain from env vars
When `SIGNOZ_ENTRA_SSO_ENABLED` is set to `"true"` and required env vars (`SIGNOZ_ENTRA_TENANT_ID`, `SIGNOZ_ENTRA_CLIENT_ID`, `SIGNOZ_ENTRA_CLIENT_SECRET`, `SIGNOZ_ENTRA_DOMAIN`) are present, the bootstrap SHALL create an `AuthDomain` entry in the database with the OIDC provider type, correct `OIDCConfig`, and `RoleMapping`.

#### Scenario: First startup with all env vars set
- **WHEN** the server starts with `SIGNOZ_ENTRA_SSO_ENABLED=true` and all required env vars
- **THEN** an `AuthDomain` is created for the configured domain with `AuthNProviderOIDC`, correct issuer URL, client ID, client secret, and claim mapping defaults

#### Scenario: Startup with SSO disabled
- **WHEN** `SIGNOZ_ENTRA_SSO_ENABLED` is not set or set to `"false"`
- **THEN** no AuthDomain is created and no error occurs

#### Scenario: Startup with missing required env vars
- **WHEN** `SIGNOZ_ENTRA_SSO_ENABLED=true` but a required env var is missing
- **THEN** an error is returned indicating which variable is missing

### Requirement: Bootstrap is idempotent
If an `AuthDomain` already exists for the configured domain and org, the bootstrap SHALL update it with the current env var values rather than creating a duplicate.

#### Scenario: Subsequent startup with same config
- **WHEN** the server restarts with the same env vars and an AuthDomain already exists
- **THEN** the existing AuthDomain is updated (not duplicated)

#### Scenario: Subsequent startup with changed config
- **WHEN** the server restarts with a different client secret
- **THEN** the existing AuthDomain is updated with the new client secret

### Requirement: Group-to-role mapping from env vars
The bootstrap SHALL construct a `RoleMapping` from `SIGNOZ_ENTRA_ADMIN_GROUP_ID`, `SIGNOZ_ENTRA_EDITOR_GROUP_ID`, and `SIGNOZ_ENTRA_DEFAULT_ROLE` env vars.

#### Scenario: Admin and editor groups configured
- **WHEN** `SIGNOZ_ENTRA_ADMIN_GROUP_ID=aaa` and `SIGNOZ_ENTRA_EDITOR_GROUP_ID=bbb` are set
- **THEN** the `RoleMapping.GroupMappings` contains `{"aaa": "ADMIN", "bbb": "EDITOR"}`

#### Scenario: Default role configured
- **WHEN** `SIGNOZ_ENTRA_DEFAULT_ROLE=VIEWER` is set
- **THEN** `RoleMapping.DefaultRole` is `"VIEWER"`

#### Scenario: No group mappings configured
- **WHEN** neither admin nor editor group env vars are set
- **THEN** `RoleMapping.GroupMappings` is empty and `DefaultRole` is `"VIEWER"`

### Requirement: Bootstrap waits for first org instead of skipping
When `SIGNOZ_ENTRA_SSO_ENABLED=true`, all required env vars are set, and no organization exists yet, `BootstrapEntraSSO` SHALL poll for the first organization on a bounded retry loop rather than skipping. The poll SHALL succeed once an organization appears (e.g. created by the `SIGNOZ_USER_ROOT_*` reconciler) and SHALL return an error if the timeout elapses without an organization appearing.

#### Scenario: First startup, org appears mid-poll
- **GIVEN** `SIGNOZ_ENTRA_SSO_ENABLED=true` and all required env vars are set
- **AND** no organization exists when `BootstrapEntraSSO` is first called
- **WHEN** an organization appears in the database before the timeout elapses
- **THEN** `BootstrapEntraSSO` proceeds to create the AuthDomain for that organization and returns nil

#### Scenario: First startup, no org appears within timeout
- **GIVEN** `SIGNOZ_ENTRA_SSO_ENABLED=true` and all required env vars are set
- **AND** no organization is ever created during the wait window
- **WHEN** the timeout elapses
- **THEN** `BootstrapEntraSSO` returns a non-nil error explaining that no organization was found and pointing operators at `SIGNOZ_USER_ROOT_*`

#### Scenario: Org exists immediately at startup
- **GIVEN** `SIGNOZ_ENTRA_SSO_ENABLED=true` and an organization already exists
- **WHEN** `BootstrapEntraSSO` is called
- **THEN** the AuthDomain is created/updated on the first attempt without polling
