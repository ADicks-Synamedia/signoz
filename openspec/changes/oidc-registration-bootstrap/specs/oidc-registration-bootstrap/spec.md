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

### Requirement: Bootstrap skips when no org exists
If no organization exists yet (first startup before any user has signed up), the bootstrap SHALL skip with an info-level log and no error.

#### Scenario: First startup with no org
- **WHEN** `SIGNOZ_ENTRA_SSO_ENABLED=true` but no org exists in the database
- **THEN** bootstrap logs an info message and returns nil (no error)
