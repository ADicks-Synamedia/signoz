## ADDED Requirements

### Requirement: Interface compliance
The `oidccallbackauthn.AuthN` type SHALL satisfy the `authn.CallbackAuthN` interface at compile time. The compile-time check `var _ authn.CallbackAuthN = (*AuthN)(nil)` MUST be present in the package.

#### Scenario: Compile-time interface check
- **WHEN** the package is compiled
- **THEN** `var _ authn.CallbackAuthN = (*AuthN)(nil)` compiles without error

### Requirement: Constructor creates provider from dependencies
The `New` function SHALL accept a context, an `authtypes.AuthNStore`, and `factory.ProviderSettings`, and return an `*AuthN` and error. It SHALL create a scoped logger, an HTTP client, and store references for use by the interface methods.

#### Scenario: Successful construction
- **WHEN** `New(ctx, store, providerSettings)` is called with valid arguments
- **THEN** a non-nil `*AuthN` is returned with no error

### Requirement: LoginURL generates OIDC authorization URL
`LoginURL` SHALL use OIDC discovery from the `OIDCConfig.Issuer` to obtain the authorization endpoint, then construct an authorization URL with the correct `client_id`, `redirect_uri`, `scope`, `response_type`, and `state` parameters. The redirect URI SHALL use the path `/api/v1/complete/oidc` combined with the scheme and host from the provided site URL. The scopes SHALL be `openid email profile`. The state SHALL encode the auth domain ID and site URL.

#### Scenario: Authorization URL for standard OIDC provider
- **WHEN** `LoginURL` is called with a site URL `https://signoz.example.com` and an AuthDomain with OIDC config (issuer `https://idp.example.com`)
- **THEN** the returned URL starts with the authorization endpoint from OIDC discovery
- **THEN** the URL contains query parameter `client_id` matching `OIDCConfig.ClientID`
- **THEN** the URL contains query parameter `redirect_uri` equal to `https://signoz.example.com/api/v1/complete/oidc`
- **THEN** the URL contains query parameter `scope` including `openid email profile`
- **THEN** the URL contains a `state` parameter encoding the domain ID

#### Scenario: LoginURL rejects non-OIDC auth domain
- **WHEN** `LoginURL` is called with an AuthDomain whose provider is not `AuthNProviderOIDC`
- **THEN** an error with code `auth_domain_mismatch` is returned

### Requirement: LoginURL handles issuer alias for OIDC discovery
When `OIDCConfig.IssuerAlias` is non-empty, `LoginURL` SHALL use `OIDCConfig.Issuer` as the fetch URL for OIDC discovery but accept `IssuerAlias` as the issuer value in the discovery document (via `oidc.InsecureIssuerURLContext`).

#### Scenario: Discovery with issuer alias
- **WHEN** `LoginURL` is called with `OIDCConfig.Issuer` set to `https://login.microsoftonline.com/tenant/v2.0` and `IssuerAlias` set to `https://sts.windows.net/tenant/`
- **THEN** discovery is fetched from `https://login.microsoftonline.com/tenant/v2.0/.well-known/openid-configuration`
- **THEN** the discovery document's `issuer` field matching `https://sts.windows.net/tenant/` is accepted without error

### Requirement: HandleCallback exchanges code and verifies ID token
`HandleCallback` SHALL parse the state parameter, look up the AuthDomain from the store, perform OIDC discovery, exchange the authorization code for tokens via `oauth2.Config.Exchange`, extract the `id_token` from the token response, and verify it using the OIDC provider's verifier with the configured `ClientID` as the expected audience.

#### Scenario: Successful callback with valid code
- **WHEN** `HandleCallback` is called with query parameters containing a valid `code` and `state`
- **THEN** the authorization code is exchanged at the token endpoint
- **THEN** the returned ID token is verified against the JWKS
- **THEN** a non-nil `CallbackIdentity` is returned with no error

#### Scenario: Callback with error from provider
- **WHEN** `HandleCallback` is called with query parameters containing an `error` field
- **THEN** an error is returned and the error description is logged

#### Scenario: Callback with invalid state
- **WHEN** `HandleCallback` is called with an invalid or missing `state` parameter
- **THEN** an error with code `invalid_state` is returned

### Requirement: HandleCallback extracts claims via ClaimMapping
`HandleCallback` SHALL extract user claims from the verified ID token using the keys defined in `OIDCConfig.ClaimMapping`. The email claim (from `ClaimMapping.Email`, default `"email"`) SHALL be parsed into the `CallbackIdentity.Email` field. The name claim (from `ClaimMapping.Name`, default `"name"`) SHALL populate `CallbackIdentity.Name`. The groups claim (from `ClaimMapping.Groups`, default `"groups"`) SHALL populate `CallbackIdentity.Groups` as a string slice. The role claim (from `ClaimMapping.Role`, default `"role"`) SHALL populate `CallbackIdentity.Role`.

#### Scenario: Claims extracted with default mapping
- **WHEN** the ID token contains claims `{"email": "user@corp.com", "name": "Test User", "groups": ["group-a", "group-b"]}`
- **THEN** `CallbackIdentity.Email` is `user@corp.com`
- **THEN** `CallbackIdentity.Name` is `Test User`
- **THEN** `CallbackIdentity.Groups` is `["group-a", "group-b"]`

#### Scenario: Claims extracted with custom mapping
- **WHEN** the OIDCConfig has `ClaimMapping.Email` set to `"preferred_email"` and the token contains `{"preferred_email": "user@corp.com"}`
- **THEN** `CallbackIdentity.Email` is `user@corp.com`

#### Scenario: Missing email claim
- **WHEN** the ID token does not contain the mapped email claim
- **THEN** an error is returned indicating missing or invalid claims

### Requirement: HandleCallback checks email verification
When `OIDCConfig.InsecureSkipEmailVerified` is false (default), `HandleCallback` SHALL check the `email_verified` claim in the ID token. If the email is not verified, an error SHALL be returned.

#### Scenario: Unverified email rejected
- **WHEN** `InsecureSkipEmailVerified` is false and the ID token has `email_verified: false`
- **THEN** an error is returned indicating the email is not verified

#### Scenario: Unverified email allowed when skip enabled
- **WHEN** `InsecureSkipEmailVerified` is true and the ID token has `email_verified: false`
- **THEN** authentication proceeds normally

### Requirement: HandleCallback handles issuer alias for token verification
When `OIDCConfig.IssuerAlias` is non-empty, `HandleCallback` SHALL configure the token verifier to accept the aliased issuer (using `SkipIssuerCheck: true` in the verifier config, since issuer validation was performed during discovery).

#### Scenario: Token verification with issuer alias
- **WHEN** `HandleCallback` processes a token whose `iss` claim is `https://sts.windows.net/tenant/` but discovery was done against `https://login.microsoftonline.com/tenant/v2.0`
- **THEN** the token is verified successfully without issuer mismatch error

### Requirement: ProviderInfo returns nil relay state
`ProviderInfo` SHALL return an `AuthNProviderInfo` with `RelayStatePath` set to `nil`, indicating standard OIDC redirect behavior with no relay state.

#### Scenario: Provider info for OIDC
- **WHEN** `ProviderInfo` is called with any AuthDomain
- **THEN** the returned `AuthNProviderInfo` has `RelayStatePath` equal to `nil`
