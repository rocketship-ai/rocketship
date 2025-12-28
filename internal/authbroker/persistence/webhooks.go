package persistence

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// InsertWebhookDelivery records a received GitHub webhook delivery.
// On conflict (duplicate delivery_id), it does nothing (idempotent).
func (s *Store) InsertWebhookDelivery(ctx context.Context, deliveryID, event, repoFullName, ref, action string) error {
	query := `
		INSERT INTO github_webhook_deliveries (delivery_id, event, repository_full_name, ref, action, received_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (delivery_id) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query,
		deliveryID,
		event,
		nullString(repoFullName),
		nullString(ref),
		nullString(action),
	)
	return err
}

// nullString converts an empty string to sql.NullString for nullable columns
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// ScanAttemptStatus represents the status of a scan attempt
type ScanAttemptStatus string

const (
	ScanAttemptSuccess ScanAttemptStatus = "success"
	ScanAttemptError   ScanAttemptStatus = "error"
	ScanAttemptSkipped ScanAttemptStatus = "skipped"
)

// ScanAttempt represents a record of a webhook-triggered scan
type ScanAttempt struct {
	ID                 uuid.UUID         `db:"id"`
	DeliveryID         string            `db:"delivery_id"`
	OrganizationID     uuid.UUID         `db:"organization_id"`
	RepositoryFullName string            `db:"repository_full_name"`
	SourceRef          string            `db:"source_ref"`
	HeadSHA            string            `db:"head_sha"`
	Status             ScanAttemptStatus `db:"status"`
	ErrorMessage       string            `db:"error_message"`
	SuitesFound        int               `db:"suites_found"`
	TestsFound         int               `db:"tests_found"`
	CreatedAt          time.Time         `db:"created_at"`
}

// InsertScanAttempt records a scan attempt for a webhook delivery
func (s *Store) InsertScanAttempt(ctx context.Context, attempt ScanAttempt) error {
	if attempt.ID == uuid.Nil {
		attempt.ID = uuid.New()
	}

	query := `
		INSERT INTO github_scan_attempts (id, delivery_id, organization_id, repository_full_name, source_ref, head_sha, status, error_message, suites_found, tests_found, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`
	_, err := s.db.ExecContext(ctx, query,
		attempt.ID,
		attempt.DeliveryID,
		attempt.OrganizationID,
		attempt.RepositoryFullName,
		attempt.SourceRef,
		nullString(attempt.HeadSHA),
		string(attempt.Status),
		nullString(attempt.ErrorMessage),
		attempt.SuitesFound,
		attempt.TestsFound,
	)
	return err
}

// ListRecentScanAttempts returns the most recent scan attempts for debugging
func (s *Store) ListRecentScanAttempts(ctx context.Context, limit int) ([]ScanAttempt, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}

	query := `
		SELECT id, delivery_id, organization_id, repository_full_name, source_ref,
		       COALESCE(head_sha, '') as head_sha, status, COALESCE(error_message, '') as error_message,
		       suites_found, tests_found, created_at
		FROM github_scan_attempts
		ORDER BY created_at DESC
		LIMIT $1
	`
	var attempts []ScanAttempt
	if err := s.db.SelectContext(ctx, &attempts, query, limit); err != nil {
		return nil, err
	}
	return attempts, nil
}
