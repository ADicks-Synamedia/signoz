## 1. Provider Registration

- [x] 1.1 Modify `pkg/signoz/authn.go` to import `oidccallbackauthn` and add `authtypes.AuthNProviderOIDC` to the provider map

## 2. Bootstrap Implementation

- [x] 2.1 Create `pkg/signoz/entrabootstrap.go` with `BootstrapEntraSSO` function that reads `SIGNOZ_ENTRA_*` env vars
- [x] 2.2 Implement env var validation: require `TENANT_ID`, `CLIENT_ID`, `CLIENT_SECRET`, `DOMAIN` when `SSO_ENABLED=true`
- [x] 2.3 Construct `OIDCConfig` from env vars: issuer URL from tenant ID, client ID, client secret, default claim mapping
- [x] 2.4 Construct `RoleMapping` from env vars: admin/editor group GUIDs, default role
- [x] 2.5 Implement upsert logic: `GetByNameAndOrgID` → create or update
- [x] 2.6 Handle no-org case: skip bootstrap with info log when no org exists yet

## 3. Wiring

- [x] 3.1 Modify `pkg/signoz/signoz.go` to call `BootstrapEntraSSO` after auth initialization

## 4. Tests

- [x] 4.1 Test: `NewAuthNs` returns map with OIDC provider (verified via `go build`)
- [x] 4.2 Test: bootstrap creates AuthDomain with correct OIDCConfig when all env vars set
- [x] 4.3 Test: bootstrap skips when SSO_ENABLED is not true
- [x] 4.4 Test: bootstrap returns error when required env var is missing
- [x] 4.5 Test: bootstrap updates existing AuthDomain (idempotent)
- [x] 4.6 Test: bootstrap constructs correct RoleMapping from group env vars
- [x] 4.7 Test: bootstrap skips when no org exists
- [x] 4.8 Run tests and verify all pass
