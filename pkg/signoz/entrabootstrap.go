package signoz

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/SigNoz/signoz/pkg/errors"
	"github.com/SigNoz/signoz/pkg/modules/organization"
	"github.com/SigNoz/signoz/pkg/types/authtypes"
	"github.com/SigNoz/signoz/pkg/valuer"
)

// bootstrapOrgWaitTimeout is the maximum time BootstrapEntraSSO will wait for
// the first organization to appear in the database. The user reconciler at
// pkg/modules/user/impluser/service.go:58 ticks every 10 seconds; 90s gives
// roughly 9 reconciler ticks of headroom for slow first-boot databases. If
// the reconciler interval changes, this budget should change with it.
const bootstrapOrgWaitTimeout = 90 * time.Second

// bootstrapOrgPollInterval is how often BootstrapEntraSSO polls for the first
// organization while waiting. It is intentionally smaller than the reconciler
// tick so the worst-case latency between org creation and AuthDomain creation
// stays small.
const bootstrapOrgPollInterval = 2 * time.Second

// BootstrapEntraSSO reads SIGNOZ_ENTRA_* environment variables and creates or
// updates an AuthDomain entry in the database for Entra ID SSO. This runs at
// startup and is idempotent — safe to call on every server start.
//
// On a fresh database with no organization yet, this function waits up to
// bootstrapOrgWaitTimeout for the user reconciler (driven by SIGNOZ_USER_ROOT_*)
// to create the first organization, then proceeds. If the timeout elapses with
// no organization, an error is returned naming the env vars the operator must
// set so the caller can fail startup loudly.
func BootstrapEntraSSO(ctx context.Context, logger *slog.Logger, authDomainStore authtypes.AuthDomainStore, orgGetter organization.Getter) error {
	return bootstrapEntraSSO(ctx, logger, authDomainStore, orgGetter, bootstrapOrgWaitTimeout, bootstrapOrgPollInterval)
}

// bootstrapEntraSSO is the testable form of BootstrapEntraSSO with injectable
// timeout and poll interval.
func bootstrapEntraSSO(ctx context.Context, logger *slog.Logger, authDomainStore authtypes.AuthDomainStore, orgGetter organization.Getter, waitTimeout time.Duration, pollInterval time.Duration) error {
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

	orgID, err := waitForFirstOrg(ctx, logger, orgGetter, waitTimeout, pollInterval)
	if err != nil {
		return err
	}

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

// waitForFirstOrg returns the ID of the first organization in the database,
// polling on pollInterval until either an organization appears or waitTimeout
// elapses. Cancellation of the parent context is honored.
func waitForFirstOrg(ctx context.Context, logger *slog.Logger, orgGetter organization.Getter, waitTimeout time.Duration, pollInterval time.Duration) (valuer.UUID, error) {
	// First attempt immediately so a populated database doesn't pay the poll latency.
	orgs, err := orgGetter.ListByOwnedKeyRange(ctx)
	if err != nil {
		return valuer.UUID{}, fmt.Errorf("entra bootstrap: failed to list organizations: %w", err)
	}
	if len(orgs) > 0 {
		return orgs[0].ID, nil
	}

	logger.InfoContext(ctx, "entra bootstrap: no organization yet, waiting for user reconciler to create one",
		slog.Duration("timeout", waitTimeout),
		slog.Duration("poll_interval", pollInterval),
	)

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return valuer.UUID{}, fmt.Errorf("entra bootstrap: timed out after %s waiting for first organization to be created; for SSO-first deployments set SIGNOZ_USER_ROOT_EMAIL and SIGNOZ_USER_ROOT_PASSWORD so the root user reconciler creates the bootstrap org", waitTimeout)
		case <-ticker.C:
			orgs, err := orgGetter.ListByOwnedKeyRange(ctx)
			if err != nil {
				return valuer.UUID{}, fmt.Errorf("entra bootstrap: failed to list organizations: %w", err)
			}
			if len(orgs) > 0 {
				logger.InfoContext(ctx, "entra bootstrap: organization detected, proceeding", slog.String("org_id", orgs[0].ID.String()))
				return orgs[0].ID, nil
			}
		}
	}
}

// BootstrapEntraSSO is a method form of the package-level BootstrapEntraSSO that
// uses the dependencies captured during signoz.New. Callers in cmd/community
// invoke it after the registry has started so the user reconciler has had time
// to create the first organization on a fresh database.
func (s *SigNoz) BootstrapEntraSSO(ctx context.Context) error {
	return BootstrapEntraSSO(ctx, s.entrabootstrapLogger, s.entrabootstrapAuthDomainStore, s.entrabootstrapOrgGetter)
}
