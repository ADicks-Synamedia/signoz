package signoz

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/SigNoz/signoz/pkg/errors"
	"github.com/SigNoz/signoz/pkg/modules/organization"
	"github.com/SigNoz/signoz/pkg/types/authtypes"
)

// BootstrapEntraSSO reads SIGNOZ_ENTRA_* environment variables and creates or
// updates an AuthDomain entry in the database for Entra ID SSO. This runs at
// startup and is idempotent — safe to call on every server start.
func BootstrapEntraSSO(ctx context.Context, logger *slog.Logger, authDomainStore authtypes.AuthDomainStore, orgGetter organization.Getter) error {
	if os.Getenv("SIGNOZ_ENTRA_SSO_ENABLED") != "true" {
		return nil
	}

	tenantID := os.Getenv("SIGNOZ_ENTRA_TENANT_ID")
	clientID := os.Getenv("SIGNOZ_ENTRA_CLIENT_ID")
	clientSecret := os.Getenv("SIGNOZ_ENTRA_CLIENT_SECRET")
	domain := os.Getenv("SIGNOZ_ENTRA_DOMAIN")

	if tenantID == "" {
		return fmt.Errorf("SIGNOZ_ENTRA_SSO_ENABLED is true but SIGNOZ_ENTRA_TENANT_ID is not set")
	}
	if clientID == "" {
		return fmt.Errorf("SIGNOZ_ENTRA_SSO_ENABLED is true but SIGNOZ_ENTRA_CLIENT_ID is not set")
	}
	if clientSecret == "" {
		return fmt.Errorf("SIGNOZ_ENTRA_SSO_ENABLED is true but SIGNOZ_ENTRA_CLIENT_SECRET is not set")
	}
	if domain == "" {
		return fmt.Errorf("SIGNOZ_ENTRA_SSO_ENABLED is true but SIGNOZ_ENTRA_DOMAIN is not set")
	}

	orgs, err := orgGetter.ListByOwnedKeyRange(ctx)
	if err != nil {
		return fmt.Errorf("entra bootstrap: failed to list organizations: %w", err)
	}

	if len(orgs) == 0 {
		logger.WarnContext(ctx, "entra bootstrap: no organization found, skipping — SSO will be configured on next restart after org creation")
		return nil
	}

	orgID := orgs[0].ID

	issuer := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenantID)
	issuerAlias := fmt.Sprintf("https://sts.windows.net/%s/", tenantID)

	groupMappings := make(map[string]string)
	if adminGroupID := os.Getenv("SIGNOZ_ENTRA_ADMIN_GROUP_ID"); adminGroupID != "" {
		groupMappings[adminGroupID] = "ADMIN"
	}
	if editorGroupID := os.Getenv("SIGNOZ_ENTRA_EDITOR_GROUP_ID"); editorGroupID != "" {
		groupMappings[editorGroupID] = "EDITOR"
	}

	defaultRole := os.Getenv("SIGNOZ_ENTRA_DEFAULT_ROLE")
	if defaultRole == "" {
		defaultRole = "VIEWER"
	}

	config := &authtypes.AuthDomainConfig{
		SSOEnabled:    true,
		AuthNProvider: authtypes.AuthNProviderOIDC,
		OIDC: &authtypes.OIDCConfig{
			Issuer:       issuer,
			IssuerAlias:  issuerAlias,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			ClaimMapping: authtypes.AttributeMapping{
				Email:  "email",
				Name:   "name",
				Groups: "groups",
				Role:   "role",
			},
		},
		RoleMapping: &authtypes.RoleMapping{
			DefaultRole:   defaultRole,
			GroupMappings: groupMappings,
		},
	}

	existing, err := authDomainStore.GetByNameAndOrgID(ctx, domain, orgID)
	if err != nil && !errors.Ast(err, errors.TypeNotFound) {
		return fmt.Errorf("entra bootstrap: failed to look up auth domain: %w", err)
	}

	if existing != nil {
		if err := existing.Update(config); err != nil {
			return fmt.Errorf("entra bootstrap: failed to prepare auth domain update: %w", err)
		}
		if err := authDomainStore.Update(ctx, existing); err != nil {
			return fmt.Errorf("entra bootstrap: failed to update auth domain: %w", err)
		}
		logger.InfoContext(ctx, "entra bootstrap: updated AuthDomain for Entra SSO", slog.String("domain", domain), slog.String("org_id", orgID.String()))
		return nil
	}

	authDomain, err := authtypes.NewAuthDomainFromConfig(domain, config, orgID)
	if err != nil {
		return fmt.Errorf("entra bootstrap: failed to create auth domain: %w", err)
	}

	if err := authDomainStore.Create(ctx, authDomain); err != nil {
		return fmt.Errorf("entra bootstrap: failed to save auth domain: %w", err)
	}

	logger.InfoContext(ctx, "entra bootstrap: created AuthDomain for Entra SSO", slog.String("domain", domain), slog.String("org_id", orgID.String()))
	return nil
}
