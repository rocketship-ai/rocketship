package persistence

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrGitHubAppNotInstalled is returned when no GitHub App installation exists for an organization
var ErrGitHubAppNotInstalled = errors.New("github app not installed for organization")

// GitHubAppInstallation represents a GitHub App installation linked to an organization
type GitHubAppInstallation struct {
	OrganizationID uuid.UUID
	InstallationID int64
	InstalledBy    uuid.UUID
	AccountLogin   string
	AccountType    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UpsertGitHubAppInstallation creates or updates a GitHub App installation for an organization
func (s *Store) UpsertGitHubAppInstallation(ctx context.Context, orgID uuid.UUID, installationID int64, installedBy uuid.UUID, accountLogin, accountType string) error {
	query := `
		INSERT INTO github_app_installations (organization_id, installation_id, installed_by, account_login, account_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (organization_id) DO UPDATE SET
			installation_id = EXCLUDED.installation_id,
			installed_by = EXCLUDED.installed_by,
			account_login = EXCLUDED.account_login,
			account_type = EXCLUDED.account_type,
			updated_at = NOW()
	`
	_, err := s.db.ExecContext(ctx, query, orgID, installationID, installedBy, accountLogin, accountType)
	return err
}

// GetGitHubAppInstallation retrieves the GitHub App installation for an organization
func (s *Store) GetGitHubAppInstallation(ctx context.Context, orgID uuid.UUID) (installationID int64, accountLogin, accountType string, err error) {
	query := `
		SELECT installation_id, account_login, account_type
		FROM github_app_installations
		WHERE organization_id = $1
	`
	err = s.db.QueryRowContext(ctx, query, orgID).Scan(&installationID, &accountLogin, &accountType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", "", ErrGitHubAppNotInstalled
		}
		return 0, "", "", err
	}
	return installationID, accountLogin, accountType, nil
}

// GetGitHubAppInstallationByID retrieves a GitHub App installation by installation ID
func (s *Store) GetGitHubAppInstallationByID(ctx context.Context, installationID int64) (*GitHubAppInstallation, error) {
	query := `
		SELECT organization_id, installation_id, installed_by, account_login, account_type, created_at, updated_at
		FROM github_app_installations
		WHERE installation_id = $1
	`
	var inst GitHubAppInstallation
	var installedBy sql.NullString
	err := s.db.QueryRowContext(ctx, query, installationID).Scan(
		&inst.OrganizationID,
		&inst.InstallationID,
		&installedBy,
		&inst.AccountLogin,
		&inst.AccountType,
		&inst.CreatedAt,
		&inst.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGitHubAppNotInstalled
		}
		return nil, err
	}
	if installedBy.Valid {
		inst.InstalledBy, _ = uuid.Parse(installedBy.String)
	}
	return &inst, nil
}

// DeleteGitHubAppInstallation removes a GitHub App installation for an organization
func (s *Store) DeleteGitHubAppInstallation(ctx context.Context, orgID uuid.UUID) error {
	query := `DELETE FROM github_app_installations WHERE organization_id = $1`
	_, err := s.db.ExecContext(ctx, query, orgID)
	return err
}

// HasGitHubAppInstallation checks if an organization has a GitHub App installation
func (s *Store) HasGitHubAppInstallation(ctx context.Context, orgID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM github_app_installations WHERE organization_id = $1)`
	var exists bool
	err := s.db.QueryRowContext(ctx, query, orgID).Scan(&exists)
	return exists, err
}

// ListOrgsByInstallationID returns all organization IDs that have a specific GitHub App installation.
// This is used for multi-tenant safety: when processing a webhook, we need to know which orgs
// have this installation so we can scan for each org separately.
func (s *Store) ListOrgsByInstallationID(ctx context.Context, installationID int64) ([]uuid.UUID, error) {
	query := `SELECT organization_id FROM github_app_installations WHERE installation_id = $1`
	var orgIDs []uuid.UUID
	if err := s.db.SelectContext(ctx, &orgIDs, query, installationID); err != nil {
		return nil, err
	}
	return orgIDs, nil
}
