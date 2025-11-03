package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
	"go.temporal.io/sdk/client"
)

type inMemoryRunStore struct {
	mu   sync.Mutex
	runs map[string]persistence.RunRecord
}

func newInMemoryRunStore() *inMemoryRunStore {
	return &inMemoryRunStore{runs: make(map[string]persistence.RunRecord)}
}

func (s *inMemoryRunStore) InsertRun(ctx context.Context, run persistence.RunRecord) (persistence.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[run.ID]; exists {
		return persistence.RunRecord{}, errors.New("run already exists")
	}

	now := time.Now().UTC()
	run.CreatedAt = now
	run.UpdatedAt = now
	if !run.StartedAt.Valid {
		run.StartedAt = sql.NullTime{Time: now, Valid: true}
	}
	s.runs[run.ID] = run
	return run, nil
}

func (s *inMemoryRunStore) UpdateRun(ctx context.Context, update persistence.RunUpdate) (persistence.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.runs[update.RunID]
	if !ok {
		return persistence.RunRecord{}, sql.ErrNoRows
	}

	if update.Status != nil {
		existing.Status = *update.Status
	}
	if update.StartedAt != nil {
		existing.StartedAt = sql.NullTime{Time: update.StartedAt.UTC(), Valid: true}
	}
	if update.EndedAt != nil {
		existing.EndedAt = sql.NullTime{Time: update.EndedAt.UTC(), Valid: true}
	}
	if update.Totals != nil {
		existing.TotalTests = update.Totals.Total
		existing.PassedTests = update.Totals.Passed
		existing.FailedTests = update.Totals.Failed
		existing.TimeoutTests = update.Totals.Timeout
	}
	if update.CommitSHA != nil {
		existing.CommitSHA = sql.NullString{String: strings.TrimSpace(*update.CommitSHA), Valid: strings.TrimSpace(*update.CommitSHA) != ""}
	}
	if update.BundleSHA != nil {
		existing.BundleSHA = sql.NullString{String: strings.TrimSpace(*update.BundleSHA), Valid: strings.TrimSpace(*update.BundleSHA) != ""}
	}

	existing.UpdatedAt = time.Now().UTC()
	s.runs[existing.ID] = existing
	return existing, nil
}

func (s *inMemoryRunStore) GetRun(ctx context.Context, orgID uuid.UUID, runID string) (persistence.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.runs[runID]
	if !ok || record.OrganizationID != orgID {
		return persistence.RunRecord{}, sql.ErrNoRows
	}
	return record, nil
}

func (s *inMemoryRunStore) ListRuns(ctx context.Context, orgID uuid.UUID, limit int) ([]persistence.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runs := make([]persistence.RunRecord, 0)
	for _, rec := range s.runs {
		if rec.OrganizationID != orgID {
			continue
		}
		runs = append(runs, rec)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})

	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}

	return runs, nil
}

func newTestEngineWithClient(c client.Client) *Engine {
	return NewEngine(c, newInMemoryRunStore())
}
