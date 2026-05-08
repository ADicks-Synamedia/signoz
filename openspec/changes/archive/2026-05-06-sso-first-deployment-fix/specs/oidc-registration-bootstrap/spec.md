## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Bootstrap skips when no org exists
**Reason**: This behavior caused a permanent SSO mis-configuration in SSO-first deployments — the bootstrap returned `nil` and was never re-invoked, so the AuthDomain was never created. Replaced by the bounded retry loop described in `Bootstrap waits for first org instead of skipping`.

**Migration**: Operators of SSO-first deployments must now set the `SIGNOZ_USER_ROOT_*` environment variables so the user reconciler creates the first organization within the bootstrap's wait window. This is documented in the updated operator guide.
