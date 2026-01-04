package persistence

import (
	"context"
	"fmt"
)

// TryAcquireAdvisoryLock attempts to acquire a Postgres advisory lock.
// Returns true if the lock was acquired, false if another session holds it.
// Uses pg_try_advisory_lock which is non-blocking.
//
// DEPRECATED: Use TryAcquireAdvisoryXactLock for HA-safe locking with connection pooling.
func (s *Store) TryAcquireAdvisoryLock(ctx context.Context, lockKey int64) (bool, error) {
	var acquired bool
	const query = `SELECT pg_try_advisory_lock($1)`
	if err := s.db.GetContext(ctx, &acquired, query, lockKey); err != nil {
		return false, fmt.Errorf("failed to try advisory lock: %w", err)
	}
	return acquired, nil
}

// ReleaseAdvisoryLock releases a Postgres advisory lock.
// Returns nil if the lock was released successfully.
//
// DEPRECATED: Use TryAcquireAdvisoryXactLock for HA-safe locking with connection pooling.
func (s *Store) ReleaseAdvisoryLock(ctx context.Context, lockKey int64) error {
	var released bool
	const query = `SELECT pg_advisory_unlock($1)`
	if err := s.db.GetContext(ctx, &released, query, lockKey); err != nil {
		return fmt.Errorf("failed to release advisory lock: %w", err)
	}
	if !released {
		return fmt.Errorf("lock %d was not held by this session", lockKey)
	}
	return nil
}

// SchedulerTx is a transaction interface for scheduler operations
type SchedulerTx interface {
	Commit() error
	Rollback() error
}

// TryAcquireAdvisoryXactLock attempts to acquire a transaction-scoped advisory lock.
// Returns (acquired, tx, nil) if successful. The lock is automatically released when the
// transaction is committed or rolled back. This is safe with connection pooling because
// the lock is bound to the transaction, not the session.
//
// The caller MUST commit or rollback the returned transaction.
func (s *Store) TryAcquireAdvisoryXactLock(ctx context.Context, lockKey int64) (bool, SchedulerTx, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	var acquired bool
	const query = `SELECT pg_try_advisory_xact_lock($1)`
	if err := tx.GetContext(ctx, &acquired, query, lockKey); err != nil {
		_ = tx.Rollback()
		return false, nil, fmt.Errorf("failed to try advisory xact lock: %w", err)
	}

	if !acquired {
		_ = tx.Rollback()
		return false, nil, nil
	}

	return true, tx, nil
}
