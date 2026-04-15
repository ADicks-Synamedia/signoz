package signoz

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/SigNoz/signoz/pkg/authn"
	"github.com/SigNoz/signoz/pkg/authn/callbackauthn/oidccallbackauthn"
	"github.com/SigNoz/signoz/pkg/factory"
	"github.com/SigNoz/signoz/pkg/modules/authdomain"
	"github.com/SigNoz/signoz/pkg/modules/session/implsession"
	"github.com/SigNoz/signoz/pkg/modules/user"
	"github.com/SigNoz/signoz/pkg/tokenizer"
	"github.com/SigNoz/signoz/pkg/types"
	"github.com/SigNoz/signoz/pkg/types/authtypes"
	"github.com/SigNoz/signoz/pkg/valuer"
)

// --- Mock OIDC Server for integration test ---

type integrationMockOIDCServer struct {
	Server     *httptest.Server
	PrivateKey *rsa.PrivateKey
	Issuer     string
}

func newIntegrationMockOIDCServer(t *testing.T, idTokenClaims map[string]interface{}) *integrationMockOIDCServer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	mock := &integrationMockOIDCServer{
		PrivateKey: privateKey,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", mock.handleDiscovery)
	mux.HandleFunc("/jwks", mock.handleJWKS)
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		idToken := mock.signIDToken(t, idTokenClaims)
		resp := map[string]interface{}{
			"access_token": "mock-access-token",
			"token_type":   "Bearer",
			"id_token":     idToken,
			"expires_in":   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mock.Server = httptest.NewServer(mux)
	mock.Issuer = mock.Server.URL
	return mock
}

func (m *integrationMockOIDCServer) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	discovery := map[string]interface{}{
		"issuer":                 m.Issuer,
		"authorization_endpoint": m.Server.URL + "/authorize",
		"token_endpoint":         m.Server.URL + "/token",
		"jwks_uri":               m.Server.URL + "/jwks",
		"response_types_supported": []string{"code"},
		"subject_types_supported":  []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(discovery)
}

func (m *integrationMockOIDCServer) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	pub := &m.PrivateKey.PublicKey
	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": "test-key-1",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwks)
}

func (m *integrationMockOIDCServer) signIDToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()

	now := time.Now()
	jwtClaims := jwt.MapClaims{
		"iss": m.Issuer,
		"aud": "test-client-id",
		"sub": "user-123",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
	for k, v := range claims {
		jwtClaims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)
	token.Header["kid"] = "test-key-1"

	signed, err := token.SignedString(m.PrivateKey)
	require.NoError(t, err)
	return signed
}

// --- Mock AuthDomain Module ---

type mockAuthDomainModule struct {
	domain *authtypes.AuthDomain
}

func (m *mockAuthDomainModule) ListByOrgID(_ context.Context, _ valuer.UUID) ([]*authtypes.AuthDomain, error) {
	return []*authtypes.AuthDomain{m.domain}, nil
}

func (m *mockAuthDomainModule) Get(_ context.Context, _ valuer.UUID) (*authtypes.AuthDomain, error) {
	return m.domain, nil
}

func (m *mockAuthDomainModule) GetByOrgIDAndID(_ context.Context, _ valuer.UUID, _ valuer.UUID) (*authtypes.AuthDomain, error) {
	return m.domain, nil
}

func (m *mockAuthDomainModule) GetByNameAndOrgID(_ context.Context, _ string, _ valuer.UUID) (*authtypes.AuthDomain, error) {
	return m.domain, nil
}

func (m *mockAuthDomainModule) Create(_ context.Context, _ *authtypes.AuthDomain) error {
	return nil
}

func (m *mockAuthDomainModule) Update(_ context.Context, _ *authtypes.AuthDomain) error {
	return nil
}

func (m *mockAuthDomainModule) Delete(_ context.Context, _ valuer.UUID, _ valuer.UUID) error {
	return nil
}

func (m *mockAuthDomainModule) GetAuthNProviderInfo(_ context.Context, _ *authtypes.AuthDomain) *authtypes.AuthNProviderInfo {
	return nil
}

func (m *mockAuthDomainModule) Collect(_ context.Context, _ valuer.UUID) (map[string]any, error) {
	return nil, nil
}

var _ authdomain.Module = (*mockAuthDomainModule)(nil)

// --- Mock User Setter ---

type mockUserSetter struct {
	createdUser *types.User
}

func (m *mockUserSetter) CreateFirstUser(_ context.Context, _ *types.Organization, _ string, _ valuer.Email, _ string) (*types.User, error) {
	return nil, nil
}

func (m *mockUserSetter) CreateUser(_ context.Context, _ *types.User, _ ...user.CreateUserOption) error {
	return nil
}

func (m *mockUserSetter) GetOrCreateUser(_ context.Context, u *types.User, _ ...user.CreateUserOption) (*types.User, error) {
	m.createdUser = u
	return u, nil
}

func (m *mockUserSetter) GetOrCreateResetPasswordToken(_ context.Context, _ valuer.UUID) (*types.ResetPasswordToken, error) {
	return nil, nil
}

func (m *mockUserSetter) UpdatePasswordByResetPasswordToken(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockUserSetter) UpdatePassword(_ context.Context, _ valuer.UUID, _ string, _ string) error {
	return nil
}

func (m *mockUserSetter) ForgotPassword(_ context.Context, _ valuer.UUID, _ valuer.Email, _ string) error {
	return nil
}

func (m *mockUserSetter) UpdateUserDeprecated(_ context.Context, _ valuer.UUID, _ string, _ *types.DeprecatedUser) (*types.DeprecatedUser, error) {
	return nil, nil
}

func (m *mockUserSetter) UpdateUser(_ context.Context, _ valuer.UUID, _ valuer.UUID, _ *types.UpdatableUser) (*types.User, error) {
	return nil, nil
}

func (m *mockUserSetter) UpdateAnyUserDeprecated(_ context.Context, _ valuer.UUID, _ *types.DeprecatedUser) error {
	return nil
}

func (m *mockUserSetter) UpdateAnyUser(_ context.Context, _ valuer.UUID, _ *types.User) error {
	return nil
}

func (m *mockUserSetter) DeleteUser(_ context.Context, _ valuer.UUID, _ string, _ string) error {
	return nil
}

func (m *mockUserSetter) CreateBulkInvite(_ context.Context, _ valuer.UUID, _ valuer.UUID, _ valuer.Email, _ *types.PostableBulkInviteRequest) ([]*types.Invite, error) {
	return nil, nil
}

func (m *mockUserSetter) UpdateUserRoles(_ context.Context, _, _ valuer.UUID, _ []string) error {
	return nil
}

func (m *mockUserSetter) AddUserRole(_ context.Context, _, _ valuer.UUID, _ string) error {
	return nil
}

func (m *mockUserSetter) RemoveUserRole(_ context.Context, _, _ valuer.UUID, _ valuer.UUID) error {
	return nil
}

func (m *mockUserSetter) Collect(_ context.Context, _ valuer.UUID) (map[string]any, error) {
	return nil, nil
}

var _ user.Setter = (*mockUserSetter)(nil)

// --- Mock User Getter ---

type mockUserGetter struct{}

func (m *mockUserGetter) GetRootUserByOrgID(_ context.Context, _ valuer.UUID) (*types.User, []*authtypes.UserRole, error) {
	return nil, nil, nil
}

func (m *mockUserGetter) ListDeprecatedUsersByOrgID(_ context.Context, _ valuer.UUID) ([]*types.DeprecatedUser, error) {
	return nil, nil
}

func (m *mockUserGetter) ListUsersByOrgID(_ context.Context, _ valuer.UUID) ([]*types.User, error) {
	return nil, nil
}

func (m *mockUserGetter) GetDeprecatedUserByOrgIDAndID(_ context.Context, _ valuer.UUID, _ valuer.UUID) (*types.DeprecatedUser, error) {
	return nil, nil
}

func (m *mockUserGetter) GetUserByOrgIDAndID(_ context.Context, _ valuer.UUID, _ valuer.UUID) (*types.User, error) {
	return nil, nil
}

func (m *mockUserGetter) Get(_ context.Context, _ valuer.UUID) (*types.DeprecatedUser, error) {
	return nil, nil
}

func (m *mockUserGetter) ListUsersByEmailAndOrgIDs(_ context.Context, _ valuer.Email, _ []valuer.UUID) ([]*types.User, error) {
	return nil, nil
}

func (m *mockUserGetter) CountByOrgID(_ context.Context, _ valuer.UUID) (int64, error) {
	return 0, nil
}

func (m *mockUserGetter) CountByOrgIDAndStatuses(_ context.Context, _ valuer.UUID, _ []string) (map[valuer.String]int64, error) {
	return nil, nil
}

func (m *mockUserGetter) GetFactorPasswordByUserID(_ context.Context, _ valuer.UUID) (*types.FactorPassword, error) {
	return nil, nil
}

func (m *mockUserGetter) GetNonDeletedUserByEmailAndOrgID(_ context.Context, _ valuer.Email, _ valuer.UUID) (*types.User, error) {
	return nil, nil
}

func (m *mockUserGetter) GetRolesByUserID(_ context.Context, _ valuer.UUID) ([]*authtypes.UserRole, error) {
	return nil, nil
}

func (m *mockUserGetter) GetUsersByOrgIDAndRoleID(_ context.Context, _ valuer.UUID, _ valuer.UUID) ([]*types.User, error) {
	return nil, nil
}

var _ user.Getter = (*mockUserGetter)(nil)

// --- Mock Tokenizer ---

type mockTokenizer struct {
	lastIdentity *authtypes.Identity
}

func (m *mockTokenizer) Start(_ context.Context) error { return nil }
func (m *mockTokenizer) Stop(_ context.Context) error  { return nil }

func (m *mockTokenizer) CreateToken(_ context.Context, identity *authtypes.Identity, _ map[string]string) (*authtypes.Token, error) {
	m.lastIdentity = identity
	return &authtypes.Token{
		ID:           valuer.GenerateUUID(),
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Meta:         map[string]string{},
	}, nil
}

func (m *mockTokenizer) GetIdentity(_ context.Context, _ string) (*authtypes.Identity, error) {
	return nil, nil
}

func (m *mockTokenizer) RotateToken(_ context.Context, _ string, _ string) (*authtypes.Token, error) {
	return nil, nil
}

func (m *mockTokenizer) DeleteToken(_ context.Context, _ string) error   { return nil }
func (m *mockTokenizer) DeleteTokensByUserID(_ context.Context, _ valuer.UUID) error { return nil }
func (m *mockTokenizer) DeleteIdentity(_ context.Context, _ valuer.UUID) error       { return nil }
func (m *mockTokenizer) SetLastObservedAt(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (m *mockTokenizer) ListMaxLastObservedAtByOrgID(_ context.Context, _ valuer.UUID) (map[valuer.UUID]time.Time, error) {
	return nil, nil
}

func (m *mockTokenizer) Config() tokenizer.Config {
	return tokenizer.Config{
		Rotation: tokenizer.RotationConfig{
			Interval: 30 * time.Minute,
		},
	}
}

func (m *mockTokenizer) Collect(_ context.Context, _ valuer.UUID) (map[string]any, error) {
	return nil, nil
}

var _ tokenizer.Tokenizer = (*mockTokenizer)(nil)

// --- Mock AuthN Store (for OIDC provider) ---

type integrationMockAuthNStore struct {
	authDomain *authtypes.AuthDomain
}

func (m *integrationMockAuthNStore) GetActiveUserAndFactorPasswordByEmailAndOrgID(_ context.Context, _ string, _ valuer.UUID) (*types.User, *types.FactorPassword, []*authtypes.UserRole, error) {
	return nil, nil, nil, nil
}

func (m *integrationMockAuthNStore) GetAuthDomainFromID(_ context.Context, _ valuer.UUID) (*authtypes.AuthDomain, error) {
	return m.authDomain, nil
}

// --- Integration Test ---

// TestCreateCallbackAuthNSession_FullOIDCFlow exercises the complete
// CreateCallbackAuthNSession path: mock OIDC provider → code exchange →
// token verification → group-to-role mapping → JIT user creation →
// session token returned.
func TestCreateCallbackAuthNSession_FullOIDCFlow(t *testing.T) {
	ctx := context.Background()
	orgID := valuer.GenerateUUID()

	// 1. Set up mock OIDC server with claims including groups
	adminGroupID := "entra-admin-group-guid"
	mockOIDC := newIntegrationMockOIDCServer(t, map[string]interface{}{
		"email":          "alice@corp.com",
		"email_verified": true,
		"name":           "Alice Smith",
		"groups":         []interface{}{adminGroupID, "some-other-group"},
	})
	defer mockOIDC.Server.Close()

	// 2. Create AuthDomain with OIDC config + group-to-role mapping
	oidcConfig := &authtypes.OIDCConfig{
		Issuer:       mockOIDC.Server.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		ClaimMapping: authtypes.AttributeMapping{
			Email:  "email",
			Name:   "name",
			Groups: "groups",
			Role:   "role",
		},
	}

	domainConfig := &authtypes.AuthDomainConfig{
		SSOEnabled:    true,
		AuthNProvider: authtypes.AuthNProviderOIDC,
		OIDC:          oidcConfig,
		RoleMapping: &authtypes.RoleMapping{
			DefaultRole: "VIEWER",
			GroupMappings: map[string]string{
				adminGroupID: "ADMIN",
			},
		},
	}

	configJSON, err := json.Marshal(domainConfig)
	require.NoError(t, err)

	authDomain, err := authtypes.NewAuthDomain("corp.com", string(configJSON), orgID)
	require.NoError(t, err)

	// 3. Build OIDC CallbackAuthN provider with mock store
	authNStore := &integrationMockAuthNStore{authDomain: authDomain}
	providerSettings := factory.ProviderSettings{
		Logger:         slog.Default(),
		MeterProvider:  noop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
	}

	oidcProvider, err := oidccallbackauthn.New(ctx, authNStore, providerSettings)
	require.NoError(t, err)

	authNs := map[authtypes.AuthNProvider]authn.AuthN{
		authtypes.AuthNProviderOIDC: oidcProvider,
	}

	// 4. Build session module with all mock dependencies
	mockUserSet := &mockUserSetter{}
	mockTok := &mockTokenizer{}
	mockAuthDomainMod := &mockAuthDomainModule{domain: authDomain}

	sessionModule := implsession.NewModule(
		providerSettings,
		authNs,
		mockUserSet,
		&mockUserGetter{},
		mockAuthDomainMod,
		mockTok,
		&mockOrgGetter{orgs: []*types.Organization{{Identifiable: types.Identifiable{ID: orgID}, Name: "test-org"}}},
	)

	// 5. Simulate the OIDC callback: build state + query params as if the
	//    browser redirected back from the OIDC provider
	siteURL, _ := url.Parse("https://signoz.example.com")
	state := authtypes.NewState(siteURL, authDomain.StorableAuthDomain().ID)

	queryValues := url.Values{
		"code":  {"mock-authorization-code"},
		"state": {state.URL.String()},
	}

	// 6. Call CreateCallbackAuthNSession — this exercises the full path
	redirectURL, err := sessionModule.CreateCallbackAuthNSession(ctx, authtypes.AuthNProviderOIDC, queryValues)
	require.NoError(t, err)
	require.NotEmpty(t, redirectURL)

	// 7. Verify the redirect URL contains session tokens
	parsed, err := url.Parse(redirectURL)
	require.NoError(t, err)

	assert.Equal(t, "signoz.example.com", parsed.Host)
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "test-access-token", parsed.Query().Get("accessToken"))
	assert.Equal(t, "test-refresh-token", parsed.Query().Get("refreshToken"))
	assert.Equal(t, "bearer", parsed.Query().Get("tokenType"))
	assert.NotEmpty(t, parsed.Query().Get("expiresIn"))

	// 8. Verify JIT user creation was called with correct identity
	require.NotNil(t, mockUserSet.createdUser, "GetOrCreateUser should have been called")
	assert.Equal(t, "alice@corp.com", mockUserSet.createdUser.Email.StringValue())
	assert.Equal(t, "Alice Smith", mockUserSet.createdUser.DisplayName)
	assert.Equal(t, orgID, mockUserSet.createdUser.OrgID)
	assert.False(t, mockUserSet.createdUser.IsRoot)

	// 9. Verify tokenizer received the correct identity
	require.NotNil(t, mockTok.lastIdentity, "CreateToken should have been called")
	assert.Equal(t, "alice@corp.com", mockTok.lastIdentity.Email.StringValue())
	assert.Equal(t, orgID, mockTok.lastIdentity.OrgID)
}
