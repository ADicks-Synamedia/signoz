package oidccallbackauthn

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
	"github.com/SigNoz/signoz/pkg/factory"
	"github.com/SigNoz/signoz/pkg/types"
	"github.com/SigNoz/signoz/pkg/types/authtypes"
	"github.com/SigNoz/signoz/pkg/valuer"
)

// Compile-time interface check
var _ authn.CallbackAuthN = (*AuthN)(nil)

// --- Mock AuthNStore ---

type mockAuthNStore struct {
	authDomain *authtypes.AuthDomain
}

func (m *mockAuthNStore) GetActiveUserAndFactorPasswordByEmailAndOrgID(_ context.Context, _ string, _ valuer.UUID) (*types.User, *types.FactorPassword, []*authtypes.UserRole, error) {
	return nil, nil, nil, nil
}

func (m *mockAuthNStore) GetAuthDomainFromID(_ context.Context, _ valuer.UUID) (*authtypes.AuthDomain, error) {
	return m.authDomain, nil
}

// --- Mock OIDC Server ---

type mockOIDCServer struct {
	Server     *httptest.Server
	PrivateKey *rsa.PrivateKey
	Issuer     string
}

func newMockOIDCServer(t *testing.T) *mockOIDCServer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	mock := &mockOIDCServer{
		PrivateKey: privateKey,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", mock.handleDiscovery)
	mux.HandleFunc("/jwks", mock.handleJWKS)
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mock.Server = httptest.NewServer(mux)
	mock.Issuer = mock.Server.URL

	return mock
}

func newMockOIDCServerWithIssuerAlias(t *testing.T, aliasIssuer string) *mockOIDCServer {
	t.Helper()

	mock := newMockOIDCServer(t)
	mock.Issuer = aliasIssuer
	return mock
}

func (m *mockOIDCServer) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
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

func (m *mockOIDCServer) handleJWKS(w http.ResponseWriter, _ *http.Request) {
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

func (m *mockOIDCServer) signIDToken(t *testing.T, claims map[string]interface{}) string {
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

// withTokenEndpoint adds a token endpoint to the mock server that returns a signed ID token.
func (m *mockOIDCServer) withTokenEndpoint(t *testing.T, idTokenClaims map[string]interface{}) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", m.handleDiscovery)
	mux.HandleFunc("/jwks", m.handleJWKS)
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		idToken := m.signIDToken(t, idTokenClaims)
		resp := map[string]interface{}{
			"access_token": "mock-access-token",
			"token_type":   "Bearer",
			"id_token":     idToken,
			"expires_in":   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	m.Server.Config.Handler = mux
}

func (m *mockOIDCServer) close() {
	m.Server.Close()
}

// --- Test Helpers ---

func newTestProviderSettings() factory.ProviderSettings {
	return factory.ProviderSettings{
		Logger:         slog.Default(),
		MeterProvider:  noop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
	}
}

func newTestAuthDomain(t *testing.T, issuer, clientID, clientSecret string) *authtypes.AuthDomain {
	t.Helper()
	return newTestAuthDomainWithOIDCConfig(t, &authtypes.OIDCConfig{
		Issuer:       issuer,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
}

func newTestAuthDomainWithOIDCConfig(t *testing.T, oidcConfig *authtypes.OIDCConfig) *authtypes.AuthDomain {
	t.Helper()

	config := &authtypes.AuthDomainConfig{
		SSOEnabled:    true,
		AuthNProvider: authtypes.AuthNProviderOIDC,
		OIDC:          oidcConfig,
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	authDomain, err := authtypes.NewAuthDomain("example.com", string(data), valuer.GenerateUUID())
	require.NoError(t, err)

	return authDomain
}

func newTestGoogleAuthDomain(t *testing.T) *authtypes.AuthDomain {
	t.Helper()

	config := &authtypes.AuthDomainConfig{
		SSOEnabled:    true,
		AuthNProvider: authtypes.AuthNProviderGoogleAuth,
		Google: &authtypes.GoogleConfig{
			ClientID:     "google-client-id",
			ClientSecret: "google-client-secret",
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	authDomain, err := authtypes.NewAuthDomain("example.com", string(data), valuer.GenerateUUID())
	require.NoError(t, err)

	return authDomain
}

// --- Tests ---

func TestInterfaceCompliance(t *testing.T) {
	// This is a compile-time check, but we include a test for explicitness.
	var provider authn.CallbackAuthN = &AuthN{}
	assert.NotNil(t, provider)
}

func TestNew(t *testing.T) {
	store := &mockAuthNStore{}
	settings := newTestProviderSettings()

	provider, err := New(context.Background(), store, settings)
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestLoginURL_ReturnsCorrectAuthorizationURL(t *testing.T) {
	mockServer := newMockOIDCServer(t)
	defer mockServer.close()

	authDomain := newTestAuthDomain(t, mockServer.Server.URL, "test-client-id", "test-client-secret")
	store := &mockAuthNStore{authDomain: authDomain}

	provider, err := New(context.Background(), store, newTestProviderSettings())
	require.NoError(t, err)

	siteURL, _ := url.Parse("https://signoz.example.com")
	loginURL, err := provider.LoginURL(context.Background(), siteURL, authDomain)
	require.NoError(t, err)

	parsed, err := url.Parse(loginURL)
	require.NoError(t, err)

	// Should point to the mock server's authorize endpoint
	assert.Equal(t, mockServer.Server.URL+"/authorize", parsed.Scheme+"://"+parsed.Host+parsed.Path)

	// Check query parameters
	query := parsed.Query()
	assert.Equal(t, "test-client-id", query.Get("client_id"))
	assert.Equal(t, "https://signoz.example.com/api/v1/complete/oidc", query.Get("redirect_uri"))
	assert.Contains(t, query.Get("scope"), "openid")
	assert.Contains(t, query.Get("scope"), "email")
	assert.Contains(t, query.Get("scope"), "profile")
	assert.Equal(t, "code", query.Get("response_type"))
	assert.NotEmpty(t, query.Get("state"))

	// Verify state encodes the domain ID
	state, err := authtypes.NewStateFromString(query.Get("state"))
	require.NoError(t, err)
	assert.Equal(t, authDomain.StorableAuthDomain().ID, state.DomainID)
}

func TestLoginURL_RejectsNonOIDCAuthDomain(t *testing.T) {
	authDomain := newTestGoogleAuthDomain(t)
	store := &mockAuthNStore{authDomain: authDomain}

	provider, err := New(context.Background(), store, newTestProviderSettings())
	require.NoError(t, err)

	siteURL, _ := url.Parse("https://signoz.example.com")
	_, err = provider.LoginURL(context.Background(), siteURL, authDomain)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not oidc")
}

func TestHandleCallback_Success(t *testing.T) {
	mockServer := newMockOIDCServer(t)
	defer mockServer.close()

	mockServer.withTokenEndpoint(t, map[string]interface{}{
		"email":          "user@example.com",
		"email_verified": true,
		"name":           "Test User",
		"groups":         []interface{}{"group-a", "group-b"},
	})

	authDomain := newTestAuthDomain(t, mockServer.Server.URL, "test-client-id", "test-client-secret")
	store := &mockAuthNStore{authDomain: authDomain}

	provider, err := New(context.Background(), store, newTestProviderSettings())
	require.NoError(t, err)

	siteURL, _ := url.Parse("https://signoz.example.com")
	state := authtypes.NewState(siteURL, authDomain.StorableAuthDomain().ID)

	query := url.Values{
		"code":  {"mock-auth-code"},
		"state": {state.URL.String()},
	}

	identity, err := provider.HandleCallback(context.Background(), query)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, "user@example.com", identity.Email.StringValue())
	assert.Equal(t, "Test User", identity.Name)
	assert.Equal(t, []string{"group-a", "group-b"}, identity.Groups)
	assert.Equal(t, authDomain.StorableAuthDomain().OrgID, identity.OrgID)
}

func TestHandleCallback_ErrorFromProvider(t *testing.T) {
	mockServer := newMockOIDCServer(t)
	defer mockServer.close()

	authDomain := newTestAuthDomain(t, mockServer.Server.URL, "test-client-id", "test-client-secret")
	store := &mockAuthNStore{authDomain: authDomain}

	provider, err := New(context.Background(), store, newTestProviderSettings())
	require.NoError(t, err)

	query := url.Values{
		"error":             {"access_denied"},
		"error_description": {"user denied access"},
	}

	identity, err := provider.HandleCallback(context.Background(), query)
	assert.Nil(t, identity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error while authenticating")
}

func TestHandleCallback_InvalidState(t *testing.T) {
	mockServer := newMockOIDCServer(t)
	defer mockServer.close()

	authDomain := newTestAuthDomain(t, mockServer.Server.URL, "test-client-id", "test-client-secret")
	store := &mockAuthNStore{authDomain: authDomain}

	provider, err := New(context.Background(), store, newTestProviderSettings())
	require.NoError(t, err)

	query := url.Values{
		"code":  {"mock-auth-code"},
		"state": {"not-a-valid-url-\x00"},
	}

	identity, err := provider.HandleCallback(context.Background(), query)
	assert.Nil(t, identity)
	assert.Error(t, err)
}

func TestHandleCallback_UnverifiedEmailRejected(t *testing.T) {
	mockServer := newMockOIDCServer(t)
	defer mockServer.close()

	mockServer.withTokenEndpoint(t, map[string]interface{}{
		"email":          "user@example.com",
		"email_verified": false,
		"name":           "Test User",
	})

	oidcConfig := &authtypes.OIDCConfig{
		Issuer:                    mockServer.Server.URL,
		ClientID:                  "test-client-id",
		ClientSecret:              "test-client-secret",
		InsecureSkipEmailVerified: false,
	}
	authDomain := newTestAuthDomainWithOIDCConfig(t, oidcConfig)
	store := &mockAuthNStore{authDomain: authDomain}

	provider, err := New(context.Background(), store, newTestProviderSettings())
	require.NoError(t, err)

	siteURL, _ := url.Parse("https://signoz.example.com")
	state := authtypes.NewState(siteURL, authDomain.StorableAuthDomain().ID)

	query := url.Values{
		"code":  {"mock-auth-code"},
		"state": {state.URL.String()},
	}

	identity, err := provider.HandleCallback(context.Background(), query)
	assert.Nil(t, identity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email is not verified")
}

func TestHandleCallback_UnverifiedEmailAllowedWhenSkipped(t *testing.T) {
	mockServer := newMockOIDCServer(t)
	defer mockServer.close()

	mockServer.withTokenEndpoint(t, map[string]interface{}{
		"email":          "user@example.com",
		"email_verified": false,
		"name":           "Test User",
	})

	oidcConfig := &authtypes.OIDCConfig{
		Issuer:                    mockServer.Server.URL,
		ClientID:                  "test-client-id",
		ClientSecret:              "test-client-secret",
		InsecureSkipEmailVerified: true,
	}
	authDomain := newTestAuthDomainWithOIDCConfig(t, oidcConfig)
	store := &mockAuthNStore{authDomain: authDomain}

	provider, err := New(context.Background(), store, newTestProviderSettings())
	require.NoError(t, err)

	siteURL, _ := url.Parse("https://signoz.example.com")
	state := authtypes.NewState(siteURL, authDomain.StorableAuthDomain().ID)

	query := url.Values{
		"code":  {"mock-auth-code"},
		"state": {state.URL.String()},
	}

	identity, err := provider.HandleCallback(context.Background(), query)
	require.NoError(t, err)
	require.NotNil(t, identity)
	assert.Equal(t, "user@example.com", identity.Email.StringValue())
}

func TestHandleCallback_MissingEmailVerifiedClaimRejected(t *testing.T) {
	mockServer := newMockOIDCServer(t)
	defer mockServer.close()

	mockServer.withTokenEndpoint(t, map[string]interface{}{
		"email": "user@example.com",
		"name":  "Test User",
		// email_verified claim intentionally absent
	})

	oidcConfig := &authtypes.OIDCConfig{
		Issuer:                    mockServer.Server.URL,
		ClientID:                  "test-client-id",
		ClientSecret:              "test-client-secret",
		InsecureSkipEmailVerified: false,
	}
	authDomain := newTestAuthDomainWithOIDCConfig(t, oidcConfig)
	store := &mockAuthNStore{authDomain: authDomain}

	provider, err := New(context.Background(), store, newTestProviderSettings())
	require.NoError(t, err)

	siteURL, _ := url.Parse("https://signoz.example.com")
	state := authtypes.NewState(siteURL, authDomain.StorableAuthDomain().ID)

	query := url.Values{
		"code":  {"mock-auth-code"},
		"state": {state.URL.String()},
	}

	identity, err := provider.HandleCallback(context.Background(), query)
	assert.Nil(t, identity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email is not verified")
}

func TestHandleCallback_IssuerAlias(t *testing.T) {
	aliasIssuer := "https://sts.windows.net/test-tenant/"

	mockServer := newMockOIDCServerWithIssuerAlias(t, aliasIssuer)
	defer mockServer.close()

	mockServer.withTokenEndpoint(t, map[string]interface{}{
		"email":          "user@corp.com",
		"email_verified": true,
		"name":           "Entra User",
		"groups":         []interface{}{"admin-group-guid"},
	})

	oidcConfig := &authtypes.OIDCConfig{
		Issuer:       mockServer.Server.URL,
		IssuerAlias:  aliasIssuer,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	authDomain := newTestAuthDomainWithOIDCConfig(t, oidcConfig)
	store := &mockAuthNStore{authDomain: authDomain}

	provider, err := New(context.Background(), store, newTestProviderSettings())
	require.NoError(t, err)

	siteURL, _ := url.Parse("https://signoz.example.com")
	state := authtypes.NewState(siteURL, authDomain.StorableAuthDomain().ID)

	query := url.Values{
		"code":  {"mock-auth-code"},
		"state": {state.URL.String()},
	}

	identity, err := provider.HandleCallback(context.Background(), query)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, "user@corp.com", identity.Email.StringValue())
	assert.Equal(t, "Entra User", identity.Name)
	assert.Equal(t, []string{"admin-group-guid"}, identity.Groups)
}

func TestProviderInfo_ReturnsNilRelayState(t *testing.T) {
	provider := &AuthN{}
	authDomain := newTestAuthDomain(t, "https://idp.example.com", "client-id", "secret")

	info := provider.ProviderInfo(context.Background(), authDomain)
	assert.NotNil(t, info)
	assert.Nil(t, info.RelayStatePath)
}

func TestExtractGroups(t *testing.T) {
	tests := []struct {
		name      string
		claims    map[string]interface{}
		groupsKey string
		expected  []string
	}{
		{
			name:      "string slice via interface",
			claims:    map[string]interface{}{"groups": []interface{}{"a", "b", "c"}},
			groupsKey: "groups",
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "no groups key",
			claims:    map[string]interface{}{"email": "test@example.com"},
			groupsKey: "groups",
			expected:  nil,
		},
		{
			name:      "custom groups key",
			claims:    map[string]interface{}{"mygroups": []interface{}{"x"}},
			groupsKey: "mygroups",
			expected:  []string{"x"},
		},
		{
			name:      "non-array value",
			claims:    map[string]interface{}{"groups": "single-value"},
			groupsKey: "groups",
			expected:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractGroups(tc.claims, tc.groupsKey)
			assert.Equal(t, tc.expected, result)
		})
	}
}
