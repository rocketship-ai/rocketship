package orchestrator

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
)

type memoryRunStore struct {
	mu   sync.Mutex
	runs map[string]persistence.RunRecord
}

func NewMemoryRunStore() RunStore {
	return &memoryRunStore{runs: make(map[string]persistence.RunRecord)}
}

func (s *memoryRunStore) InsertRun(ctx context.Context, run persistence.RunRecord) (persistence.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.runs[run.ID]; ok {
		return persistence.RunRecord{}, sql.ErrNoRows
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

func (s *memoryRunStore) UpdateRun(ctx context.Context, update persistence.RunUpdate) (persistence.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.runs[update.RunID]
	if !ok {
		return persistence.RunRecord{}, sql.ErrNoRows
	}

	if update.Status != nil {
		rec.Status = *update.Status
	}
	if update.StartedAt != nil {
		rec.StartedAt = sql.NullTime{Time: update.StartedAt.UTC(), Valid: true}
	}
	if update.EndedAt != nil {
		rec.EndedAt = sql.NullTime{Time: update.EndedAt.UTC(), Valid: true}
	}
	if update.Totals != nil {
		rec.TotalTests = update.Totals.Total
		rec.PassedTests = update.Totals.Passed
		rec.FailedTests = update.Totals.Failed
		rec.TimeoutTests = update.Totals.Timeout
	}
	if update.CommitSHA != nil {
		trimmed := strings.TrimSpace(*update.CommitSHA)
		rec.CommitSHA = sql.NullString{String: trimmed, Valid: trimmed != ""}
	}
	if update.BundleSHA != nil {
		trimmed := strings.TrimSpace(*update.BundleSHA)
		rec.BundleSHA = sql.NullString{String: trimmed, Valid: trimmed != ""}
	}

	rec.UpdatedAt = time.Now().UTC()
	s.runs[rec.ID] = rec
	return rec, nil
}

func (s *memoryRunStore) GetRun(ctx context.Context, orgID uuid.UUID, runID string) (persistence.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.runs[runID]
	if !ok {
		return persistence.RunRecord{}, sql.ErrNoRows
	}
	if orgID != uuid.Nil && rec.OrganizationID != orgID {
		return persistence.RunRecord{}, sql.ErrNoRows
	}
	return rec, nil
}

func (s *memoryRunStore) ListRuns(ctx context.Context, orgID uuid.UUID, limit int) ([]persistence.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runs := make([]persistence.RunRecord, 0)
	for _, rec := range s.runs {
		if orgID != uuid.Nil && rec.OrganizationID != orgID {
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

// Run details methods - no-op for memory store (tests are tracked in-memory via Engine.runs)

func (s *memoryRunStore) InsertRunTest(_ context.Context, rt persistence.RunTest) (persistence.RunTest, error) {
	// No-op for memory store - tests are tracked via Engine.runs
	rt.CreatedAt = time.Now().UTC()
	return rt, nil
}

func (s *memoryRunStore) UpdateRunTestByWorkflowID(_ context.Context, _, _ string, _ *string, _ time.Time, _ int64) error {
	// No-op for memory store - tests are tracked via Engine.runs
	return nil
}

func (s *memoryRunStore) ListRunTests(_ context.Context, _ string) ([]persistence.RunTest, error) {
	// No-op for memory store - tests are tracked via Engine.runs
	return []persistence.RunTest{}, nil
}

func (s *memoryRunStore) InsertRunLog(_ context.Context, log persistence.RunLog) (persistence.RunLog, error) {
	// No-op for memory store - logs are tracked via Engine.runs
	log.LoggedAt = time.Now().UTC()
	return log, nil
}

func (s *memoryRunStore) ListRunLogs(_ context.Context, _ string, _ int) ([]persistence.RunLog, error) {
	// No-op for memory store - logs are tracked via Engine.runs
	return []persistence.RunLog{}, nil
}
