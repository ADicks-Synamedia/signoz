package signoz

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SigNoz/signoz/pkg/errors"
	"github.com/SigNoz/signoz/pkg/types"
	"github.com/SigNoz/signoz/pkg/types/authtypes"
	"github.com/SigNoz/signoz/pkg/valuer"
)

// --- Mock Organization Getter ---

type mockOrgGetter struct {
	orgs []*types.Organization
	// appearAfterCalls: if > 0, the first N calls to ListByOwnedKeyRange return
	// nil; subsequent calls return orgs. Used to simulate the user reconciler
	// creating the first org partway through bootstrap's wait.
	appearAfterCalls int
	calls            int
}

func (m *mockOrgGetter) ListByOwnedKeyRange(_ context.Context) ([]*types.Organization, error) {
	m.calls++
	if m.appearAfterCalls > 0 && m.calls <= m.appearAfterCalls {
		return nil, nil
	}
	return m.orgs, nil
}

func (m *mockOrgGetter) GetByName(_ context.Context, _ string) (*types.Organization, error) {
	return nil, nil
}

func (m *mockOrgGetter) GetByIDOrName(_ context.Context, _ valuer.UUID, _ string) (*types.Organization, bool, error) {
	return nil, false, nil
}

func (m *mockOrgGetter) Get(_ context.Context, _ valuer.UUID) (*types.Organization, error) {
	return nil, nil
}

// --- Mock AuthDomain Store ---

type mockAuthDomainStore struct {
	domains map[string]*authtypes.AuthDomain
}

func newMockAuthDomainStore() *mockAuthDomainStore {
	return &mockAuthDomainStore{
		domains: make(map[string]*authtypes.AuthDomain),
	}
}

func (m *mockAuthDomainStore) storeKey(name string, orgID valuer.UUID) string {
	return name + ":" + orgID.String()
}

func (m *mockAuthDomainStore) Create(_ context.Context, domain *authtypes.AuthDomain) error {
	k := m.storeKey(domain.StorableAuthDomain().Name, domain.StorableAuthDomain().OrgID)
	if _, exists := m.domains[k]; exists {
		return errors.Newf(errors.TypeAlreadyExists, authtypes.ErrCodeAuthDomainAlreadyExists, "already exists")
	}
	m.domains[k] = domain
	return nil
}

func (m *mockAuthDomainStore) Get(_ context.Context, id valuer.UUID) (*authtypes.AuthDomain, error) {
	for _, d := range m.domains {
		if d.StorableAuthDomain().ID == id {
			return d, nil
		}
	}
	return nil, errors.Newf(errors.TypeNotFound, authtypes.ErrCodeAuthDomainNotFound, "not found")
}

func (m *mockAuthDomainStore) GetByOrgIDAndID(_ context.Context, orgID valuer.UUID, id valuer.UUID) (*authtypes.AuthDomain, error) {
	for _, d := range m.domains {
		if d.StorableAuthDomain().OrgID == orgID && d.StorableAuthDomain().ID == id {
			return d, nil
		}
	}
	return nil, errors.Newf(errors.TypeNotFound, authtypes.ErrCodeAuthDomainNotFound, "not found")
}

func (m *mockAuthDomainStore) GetByName(_ context.Context, name string) (*authtypes.AuthDomain, error) {
	for _, d := range m.domains {
		if d.StorableAuthDomain().Name == name {
			return d, nil
		}
	}
	return nil, errors.Newf(errors.TypeNotFound, authtypes.ErrCodeAuthDomainNotFound, "not found")
}

func (m *mockAuthDomainStore) GetByNameAndOrgID(_ context.Context, name string, orgID valuer.UUID) (*authtypes.AuthDomain, error) {
	k := m.storeKey(name, orgID)
	if d, exists := m.domains[k]; exists {
		return d, nil
	}
	return nil, errors.Newf(errors.TypeNotFound, authtypes.ErrCodeAuthDomainNotFound, "not found")
}

func (m *mockAuthDomainStore) ListByOrgID(_ context.Context, orgID valuer.UUID) ([]*authtypes.AuthDomain, error) {
	var result []*authtypes.AuthDomain
	for _, d := range m.domains {
		if d.StorableAuthDomain().OrgID == orgID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *mockAuthDomainStore) Update(_ context.Context, domain *authtypes.AuthDomain) error {
	k := m.storeKey(domain.StorableAuthDomain().Name, domain.StorableAuthDomain().OrgID)
	m.domains[k] = domain
	return nil
}

func (m *mockAuthDomainStore) Delete(_ context.Context, orgID valuer.UUID, id valuer.UUID) error {
	for k, d := range m.domains {
		if d.StorableAuthDomain().OrgID == orgID && d.StorableAuthDomain().ID == id {
			delete(m.domains, k)
			return nil
		}
	}
	return nil
}

// --- Helpers ---

func setEntraEnvVars(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func allRequiredEnvVars() map[string]string {
	return map[string]string{
		"SIGNOZ_ENTRA_SSO_ENABLED":   "true",
		"SIGNOZ_ENTRA_TENANT_ID":     "test-tenant-id",
		"SIGNOZ_ENTRA_CLIENT_ID":     "test-client-id",
		"SIGNOZ_ENTRA_CLIENT_SECRET": "test-client-secret",
		"SIGNOZ_ENTRA_DOMAIN":        "corp.com",
	}
}

// --- Tests ---

func TestBootstrap_SkipsWhenDisabled(t *testing.T) {
	store := newMockAuthDomainStore()
	orgGetter := &mockOrgGetter{}

	err := BootstrapEntraSSO(context.Background(), slog.Default(), store, orgGetter)
	require.NoError(t, err)
	assert.Empty(t, store.domains)
}

func TestBootstrap_SkipsWhenExplicitlyDisabled(t *testing.T) {
	t.Setenv("SIGNOZ_ENTRA_SSO_ENABLED", "false")

	store := newMockAuthDomainStore()
	orgGetter := &mockOrgGetter{}

	err := BootstrapEntraSSO(context.Background(), slog.Default(), store, orgGetter)
	require.NoError(t, err)
	assert.Empty(t, store.domains)
}

func TestBootstrap_ErrorOnMissingRequiredEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		missing string
	}{
		{"missing tenant ID", "SIGNOZ_ENTRA_TENANT_ID"},
		{"missing client ID", "SIGNOZ_ENTRA_CLIENT_ID"},
		{"missing client secret", "SIGNOZ_ENTRA_CLIENT_SECRET"},
		{"missing domain", "SIGNOZ_ENTRA_DOMAIN"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vars := allRequiredEnvVars()
			delete(vars, tc.missing)
			setEntraEnvVars(t, vars)

			store := newMockAuthDomainStore()
			orgGetter := &mockOrgGetter{
				orgs: []*types.Organization{{Identifiable: types.Identifiable{ID: valuer.GenerateUUID()}, Name: "test-org"}},
			}

			err := BootstrapEntraSSO(context.Background(), slog.Default(), store, orgGetter)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.missing)
		})
	}
}

func TestBootstrap_TimesOutWhenNoOrgAppears(t *testing.T) {
	setEntraEnvVars(t, allRequiredEnvVars())

	store := newMockAuthDomainStore()
	orgGetter := &mockOrgGetter{orgs: nil}

	// Use a tiny timeout so the test finishes in ~100ms.
	err := bootstrapEntraSSO(context.Background(), slog.Default(), store, orgGetter, 100*time.Millisecond, 20*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SIGNOZ_USER_ROOT_EMAIL")
	assert.Contains(t, err.Error(), "SIGNOZ_USER_ROOT_PASSWORD")
	assert.Empty(t, store.domains)
}

func TestBootstrap_WaitsForOrgAndSucceeds(t *testing.T) {
	setEntraEnvVars(t, allRequiredEnvVars())

	orgID := valuer.GenerateUUID()
	store := newMockAuthDomainStore()
	orgGetter := &mockOrgGetter{
		orgs:             []*types.Organization{{Identifiable: types.Identifiable{ID: orgID}, Name: "test-org"}},
		appearAfterCalls: 2, // empty for first 2 calls, populated thereafter
	}

	err := bootstrapEntraSSO(context.Background(), slog.Default(), store, orgGetter, 1*time.Second, 20*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, store.domains, 1)
	require.NotNil(t, store.domains["corp.com:"+orgID.String()])
	assert.GreaterOrEqual(t, orgGetter.calls, 3)
}

func TestBootstrap_CreatesAuthDomain(t *testing.T) {
	vars := allRequiredEnvVars()
	vars["SIGNOZ_ENTRA_ADMIN_GROUP_ID"] = "admin-guid"
	vars["SIGNOZ_ENTRA_EDITOR_GROUP_ID"] = "editor-guid"
	vars["SIGNOZ_ENTRA_DEFAULT_ROLE"] = "VIEWER"
	setEntraEnvVars(t, vars)

	orgID := valuer.GenerateUUID()
	store := newMockAuthDomainStore()
	orgGetter := &mockOrgGetter{
		orgs: []*types.Organization{{Identifiable: types.Identifiable{ID: orgID}, Name: "test-org"}},
	}

	err := BootstrapEntraSSO(context.Background(), slog.Default(), store, orgGetter)
	require.NoError(t, err)
	assert.Len(t, store.domains, 1)

	domain := store.domains["corp.com:"+orgID.String()]
	require.NotNil(t, domain)
	assert.Equal(t, "corp.com", domain.StorableAuthDomain().Name)
	assert.Equal(t, orgID, domain.StorableAuthDomain().OrgID)
	assert.True(t, domain.AuthDomainConfig().SSOEnabled)
	assert.Equal(t, authtypes.AuthNProviderOIDC, domain.AuthDomainConfig().AuthNProvider)

	oidcConfig := domain.AuthDomainConfig().OIDC
	require.NotNil(t, oidcConfig)
	assert.Equal(t, "https://login.microsoftonline.com/test-tenant-id/v2.0", oidcConfig.Issuer)
	assert.Equal(t, "test-client-id", oidcConfig.ClientID)
	assert.Equal(t, "test-client-secret", oidcConfig.ClientSecret)
	assert.Equal(t, "https://sts.windows.net/test-tenant-id/", oidcConfig.IssuerAlias)
	assert.Equal(t, "email", oidcConfig.ClaimMapping.Email)
	assert.Equal(t, "name", oidcConfig.ClaimMapping.Name)
	assert.Equal(t, "groups", oidcConfig.ClaimMapping.Groups)
	assert.Equal(t, "role", oidcConfig.ClaimMapping.Role)

	roleMapping := domain.AuthDomainConfig().RoleMapping
	require.NotNil(t, roleMapping)
	assert.Equal(t, "VIEWER", roleMapping.DefaultRole)
	assert.Equal(t, map[string]string{"admin-guid": "ADMIN", "editor-guid": "EDITOR"}, roleMapping.GroupMappings)
}

func TestBootstrap_UpdatesExistingAuthDomain(t *testing.T) {
	orgID := valuer.GenerateUUID()
	store := newMockAuthDomainStore()
	orgGetter := &mockOrgGetter{
		orgs: []*types.Organization{{Identifiable: types.Identifiable{ID: orgID}, Name: "test-org"}},
	}

	// First bootstrap
	setEntraEnvVars(t, allRequiredEnvVars())
	err := BootstrapEntraSSO(context.Background(), slog.Default(), store, orgGetter)
	require.NoError(t, err)
	assert.Len(t, store.domains, 1)

	originalDomain := store.domains["corp.com:"+orgID.String()]
	originalID := originalDomain.StorableAuthDomain().ID

	// Second bootstrap with changed secret
	t.Setenv("SIGNOZ_ENTRA_CLIENT_SECRET", "new-secret")
	err = BootstrapEntraSSO(context.Background(), slog.Default(), store, orgGetter)
	require.NoError(t, err)
	assert.Len(t, store.domains, 1)

	updatedDomain := store.domains["corp.com:"+orgID.String()]
	assert.Equal(t, originalID, updatedDomain.StorableAuthDomain().ID)
	assert.Equal(t, "new-secret", updatedDomain.AuthDomainConfig().OIDC.ClientSecret)
}

func TestBootstrap_DefaultRoleWhenNoGroups(t *testing.T) {
	setEntraEnvVars(t, allRequiredEnvVars())

	orgID := valuer.GenerateUUID()
	store := newMockAuthDomainStore()
	orgGetter := &mockOrgGetter{
		orgs: []*types.Organization{{Identifiable: types.Identifiable{ID: orgID}, Name: "test-org"}},
	}

	err := BootstrapEntraSSO(context.Background(), slog.Default(), store, orgGetter)
	require.NoError(t, err)

	domain := store.domains["corp.com:"+orgID.String()]
	require.NotNil(t, domain)

	roleMapping := domain.AuthDomainConfig().RoleMapping
	require.NotNil(t, roleMapping)
	assert.Equal(t, "VIEWER", roleMapping.DefaultRole)
	assert.Empty(t, roleMapping.GroupMappings)
}
