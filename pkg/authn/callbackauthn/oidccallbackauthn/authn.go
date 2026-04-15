package oidccallbackauthn

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/SigNoz/signoz/pkg/authn"
	"github.com/SigNoz/signoz/pkg/errors"
	"github.com/SigNoz/signoz/pkg/factory"
	"github.com/SigNoz/signoz/pkg/http/client"
	"github.com/SigNoz/signoz/pkg/types/authtypes"
	"github.com/SigNoz/signoz/pkg/valuer"
)

const (
	redirectPath string = "/api/v1/complete/oidc"
)

var scopes = []string{oidc.ScopeOpenID, "email", "profile"}

var _ authn.CallbackAuthN = (*AuthN)(nil)

type AuthN struct {
	store      authtypes.AuthNStore
	settings   factory.ScopedProviderSettings
	httpClient *client.Client
}

func New(ctx context.Context, store authtypes.AuthNStore, providerSettings factory.ProviderSettings) (*AuthN, error) {
	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/SigNoz/signoz/pkg/authn/callbackauthn/oidccallbackauthn")

	httpClient, err := client.New(settings.Logger(), providerSettings.TracerProvider, providerSettings.MeterProvider)
	if err != nil {
		return nil, err
	}

	return &AuthN{
		store:      store,
		settings:   settings,
		httpClient: httpClient,
	}, nil
}

func (a *AuthN) LoginURL(ctx context.Context, siteURL *url.URL, authDomain *authtypes.AuthDomain) (string, error) {
	if authDomain.AuthDomainConfig().AuthNProvider != authtypes.AuthNProviderOIDC {
		return "", errors.Newf(errors.TypeInternal, authtypes.ErrCodeAuthDomainMismatch, "domain type is not oidc")
	}

	oidcConfig := authDomain.AuthDomainConfig().OIDC

	oidcProvider, err := a.newOIDCProvider(ctx, oidcConfig)
	if err != nil {
		return "", err
	}

	oauth2Config := a.oauth2Config(siteURL, oidcConfig, oidcProvider)

	return oauth2Config.AuthCodeURL(
		authtypes.NewState(siteURL, authDomain.StorableAuthDomain().ID).URL.String(),
	), nil
}

func (a *AuthN) HandleCallback(ctx context.Context, query url.Values) (*authtypes.CallbackIdentity, error) {
	if errMsg := query.Get("error"); errMsg != "" {
		a.settings.Logger().ErrorContext(ctx, "oidc: error while authenticating", slog.String("error", errMsg), slog.String("error_description", query.Get("error_description")))
		return nil, errors.Newf(errors.TypeInternal, errors.CodeInternal, "oidc: error while authenticating").WithAdditional(query.Get("error_description"))
	}

	state, err := authtypes.NewStateFromString(query.Get("state"))
	if err != nil {
		a.settings.Logger().ErrorContext(ctx, "oidc: invalid state", errors.Attr(err))
		return nil, errors.Newf(errors.TypeInvalidInput, authtypes.ErrCodeInvalidState, "oidc: invalid state").WithAdditional(err.Error())
	}

	authDomain, err := a.store.GetAuthDomainFromID(ctx, state.DomainID)
	if err != nil {
		return nil, err
	}

	oidcConfig := authDomain.AuthDomainConfig().OIDC

	oidcProvider, err := a.newOIDCProvider(ctx, oidcConfig)
	if err != nil {
		return nil, err
	}

	oauth2Config := a.oauth2Config(state.URL, oidcConfig, oidcProvider)

	exchangeCtx := context.WithValue(ctx, oauth2.HTTPClient, a.httpClient.Client())
	token, err := oauth2Config.Exchange(exchangeCtx, query.Get("code"))
	if err != nil {
		var retrieveError *oauth2.RetrieveError
		if errors.As(err, &retrieveError) {
			a.settings.Logger().ErrorContext(ctx, "oidc: failed to get token", errors.Attr(err), slog.String("error_description", retrieveError.ErrorDescription), slog.String("body", string(retrieveError.Body)))
			return nil, errors.Newf(errors.TypeForbidden, errors.CodeForbidden, "oidc: failed to get token").WithAdditional(retrieveError.ErrorDescription)
		}

		a.settings.Logger().ErrorContext(ctx, "oidc: failed to get token", errors.Attr(err))
		return nil, errors.Newf(errors.TypeInternal, errors.CodeInternal, "oidc: failed to get token")
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New(errors.TypeInvalidInput, errors.CodeInvalidInput, "oidc: no id_token in token response")
	}

	verifier := oidcProvider.Verifier(&oidc.Config{ClientID: oidcConfig.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		a.settings.Logger().ErrorContext(ctx, "oidc: failed to verify token", errors.Attr(err))
		return nil, errors.Newf(errors.TypeForbidden, errors.CodeForbidden, "oidc: failed to verify token")
	}

	var allClaims map[string]interface{}
	if err := idToken.Claims(&allClaims); err != nil {
		a.settings.Logger().ErrorContext(ctx, "oidc: missing or invalid claims", errors.Attr(err))
		return nil, errors.Newf(errors.TypeForbidden, errors.CodeForbidden, "oidc: missing or invalid claims").WithAdditional(err.Error())
	}

	claimMapping := oidcConfig.ClaimMapping

	emailStr, _ := allClaims[claimMapping.Email].(string)
	if emailStr == "" {
		a.settings.Logger().ErrorContext(ctx, "oidc: missing or invalid email claim", slog.String("claim_key", claimMapping.Email))
		return nil, errors.Newf(errors.TypeForbidden, errors.CodeForbidden, "oidc: missing or invalid claims")
	}

	if !oidcConfig.InsecureSkipEmailVerified {
		emailVerified, exists := allClaims["email_verified"]
		if !exists {
			a.settings.Logger().ErrorContext(ctx, "oidc: email_verified claim is absent; set insecureSkipEmailVerified to true or configure the claim in your identity provider", slog.String("email", emailStr))
			return nil, errors.Newf(errors.TypeForbidden, errors.CodeForbidden, "oidc: email is not verified")
		}
		if verified, isBool := emailVerified.(bool); isBool && !verified {
			a.settings.Logger().ErrorContext(ctx, "oidc: email is not verified", slog.String("email", emailStr))
			return nil, errors.Newf(errors.TypeForbidden, errors.CodeForbidden, "oidc: email is not verified")
		}
	}

	email, err := valuer.NewEmail(emailStr)
	if err != nil {
		return nil, errors.Newf(errors.TypeInvalidInput, errors.CodeInvalidInput, "oidc: failed to parse email").WithAdditional(err.Error())
	}

	name, _ := allClaims[claimMapping.Name].(string)
	groups := extractGroups(allClaims, claimMapping.Groups)
	role, _ := allClaims[claimMapping.Role].(string)

	return authtypes.NewCallbackIdentity(name, email, authDomain.StorableAuthDomain().OrgID, state, groups, role), nil
}

func (a *AuthN) ProviderInfo(ctx context.Context, authDomain *authtypes.AuthDomain) *authtypes.AuthNProviderInfo {
	return &authtypes.AuthNProviderInfo{
		RelayStatePath: nil,
	}
}

func (a *AuthN) newOIDCProvider(ctx context.Context, oidcConfig *authtypes.OIDCConfig) (*oidc.Provider, error) {
	discoveryCtx := context.WithValue(ctx, oauth2.HTTPClient, a.httpClient.Client())
	if oidcConfig.IssuerAlias != "" {
		discoveryCtx = oidc.InsecureIssuerURLContext(discoveryCtx, oidcConfig.IssuerAlias)
	}

	provider, err := oidc.NewProvider(discoveryCtx, oidcConfig.Issuer)
	if err != nil {
		a.settings.Logger().ErrorContext(ctx, "oidc: failed to create provider", errors.Attr(err), slog.String("issuer", oidcConfig.Issuer))
		return nil, errors.Newf(errors.TypeInternal, errors.CodeInternal, "oidc: failed to create provider")
	}

	return provider, nil
}

func (a *AuthN) oauth2Config(siteURL *url.URL, oidcConfig *authtypes.OIDCConfig, provider *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     oidcConfig.ClientID,
		ClientSecret: oidcConfig.ClientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
		RedirectURL: (&url.URL{
			Scheme: siteURL.Scheme,
			Host:   siteURL.Host,
			Path:   redirectPath,
		}).String(),
	}
}

func extractGroups(claims map[string]interface{}, groupsKey string) []string {
	raw, ok := claims[groupsKey]
	if !ok {
		return nil
	}

	switch v := raw.(type) {
	case []interface{}:
		groups := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				groups = append(groups, s)
			} else {
				groups = append(groups, fmt.Sprintf("%v", item))
			}
		}
		return groups
	case []string:
		return v
	default:
		return nil
	}
}
