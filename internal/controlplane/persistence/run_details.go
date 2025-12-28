package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// InsertRunTest creates a new run test record
func (s *Store) InsertRunTest(ctx context.Context, rt RunTest) (RunTest, error) {
	if rt.RunID == "" {
		return RunTest{}, errors.New("run id required")
	}
	if rt.WorkflowID == "" {
		return RunTest{}, errors.New("workflow id required")
	}
	if rt.Name == "" {
		return RunTest{}, errors.New("test name required")
	}

	if rt.ID == uuid.Nil {
		rt.ID = uuid.New()
	}
	if rt.Status == "" {
		rt.Status = "PENDING"
	}

	const query = `
        INSERT INTO run_tests (
            id, run_id, test_id, workflow_id, name, status, error_message,
            started_at, ended_at, duration_ms, step_count, passed_steps, failed_steps, created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW())
        RETURNING created_at
    `

	var testID, errMsg, startedAt, endedAt, durationMs interface{}
	if rt.TestID.Valid {
		testID = rt.TestID.UUID
	}
	if rt.ErrorMessage.Valid {
		errMsg = rt.ErrorMessage.String
	}
	if rt.StartedAt.Valid {
		startedAt = rt.StartedAt.Time
	}
	if rt.EndedAt.Valid {
		endedAt = rt.EndedAt.Time
	}
	if rt.DurationMs.Valid {
		durationMs = rt.DurationMs.Int64
	}

	if err := s.db.GetContext(ctx, &rt.CreatedAt, query,
		rt.ID, rt.RunID, testID, rt.WorkflowID, rt.Name, rt.Status, errMsg,
		startedAt, endedAt, durationMs, rt.StepCount, rt.PassedSteps, rt.FailedSteps); err != nil {
		return RunTest{}, fmt.Errorf("failed to insert run test: %w", err)
	}

	return rt, nil
}

// GetRunTest retrieves a run test by ID
func (s *Store) GetRunTest(ctx context.Context, id uuid.UUID) (RunTest, error) {
	const query = `
        SELECT id, run_id, test_id, workflow_id, name, status, error_message,
               started_at, ended_at, duration_ms, step_count, passed_steps, failed_steps, created_at
        FROM run_tests
        WHERE id = $1
    `

	var rt RunTest
	if err := s.db.GetContext(ctx, &rt, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunTest{}, sql.ErrNoRows
		}
		return RunTest{}, fmt.Errorf("failed to get run test: %w", err)
	}

	return rt, nil
}

// GetRunTestByWorkflowID retrieves a run test by workflow ID
func (s *Store) GetRunTestByWorkflowID(ctx context.Context, workflowID string) (RunTest, error) {
	const query = `
        SELECT id, run_id, test_id, workflow_id, name, status, error_message,
               started_at, ended_at, duration_ms, step_count, passed_steps, failed_steps, created_at
        FROM run_tests
        WHERE workflow_id = $1
    `

	var rt RunTest
	if err := s.db.GetContext(ctx, &rt, query, workflowID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunTest{}, sql.ErrNoRows
		}
		return RunTest{}, fmt.Errorf("failed to get run test by workflow id: %w", err)
	}

	return rt, nil
}

// ListRunTests returns all tests for a run
func (s *Store) ListRunTests(ctx context.Context, runID string) ([]RunTest, error) {
	const query = `
        SELECT id, run_id, test_id, workflow_id, name, status, error_message,
               started_at, ended_at, duration_ms, step_count, passed_steps, failed_steps, created_at
        FROM run_tests
        WHERE run_id = $1
        ORDER BY created_at ASC
    `

	var tests []RunTest
	if err := s.db.SelectContext(ctx, &tests, query, runID); err != nil {
		return nil, fmt.Errorf("failed to list run tests: %w", err)
	}
	if tests == nil {
		tests = []RunTest{}
	}

	return tests, nil
}

// UpdateRunTest updates a run test with completion status
func (s *Store) UpdateRunTest(ctx context.Context, rt RunTest) error {
	if rt.ID == uuid.Nil {
		return errors.New("run test id required")
	}

	const query = `
        UPDATE run_tests
        SET status = $2, error_message = $3, started_at = $4, ended_at = $5,
            duration_ms = $6, step_count = $7, passed_steps = $8, failed_steps = $9
        WHERE id = $1
    `

	var errMsg, startedAt, endedAt, durationMs interface{}
	if rt.ErrorMessage.Valid {
		errMsg = rt.ErrorMessage.String
	}
	if rt.StartedAt.Valid {
		startedAt = rt.StartedAt.Time
	}
	if rt.EndedAt.Valid {
		endedAt = rt.EndedAt.Time
	}
	if rt.DurationMs.Valid {
		durationMs = rt.DurationMs.Int64
	}

	res, err := s.db.ExecContext(ctx, query,
		rt.ID, rt.Status, errMsg, startedAt, endedAt, durationMs,
		rt.StepCount, rt.PassedSteps, rt.FailedSteps)
	if err != nil {
		return fmt.Errorf("failed to update run test: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// UpdateRunTestByWorkflowID updates a run test by workflow ID
func (s *Store) UpdateRunTestByWorkflowID(ctx context.Context, workflowID, status string, errorMsg *string, endedAt time.Time, durationMs int64) error {
	if workflowID == "" {
		return errors.New("workflow id required")
	}

	const query = `
        UPDATE run_tests
        SET status = $2, error_message = $3, ended_at = $4, duration_ms = $5
        WHERE workflow_id = $1
    `

	res, err := s.db.ExecContext(ctx, query, workflowID, status, errorMsg, endedAt, durationMs)
	if err != nil {
		return fmt.Errorf("failed to update run test: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// InsertRunStep creates a new run step record
func (s *Store) InsertRunStep(ctx context.Context, step RunStep) (RunStep, error) {
	if step.RunTestID == uuid.Nil {
		return RunStep{}, errors.New("run test id required")
	}
	if step.Name == "" {
		return RunStep{}, errors.New("step name required")
	}
	if step.Plugin == "" {
		return RunStep{}, errors.New("plugin required")
	}

	if step.ID == uuid.Nil {
		step.ID = uuid.New()
	}
	if step.Status == "" {
		step.Status = "PENDING"
	}

	const query = `
        INSERT INTO run_steps (
            id, run_test_id, step_index, name, plugin, status, error_message,
            assertions_passed, assertions_failed, started_at, ended_at, duration_ms, created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
        RETURNING created_at
    `

	var errMsg, startedAt, endedAt, durationMs interface{}
	if step.ErrorMessage.Valid {
		errMsg = step.ErrorMessage.String
	}
	if step.StartedAt.Valid {
		startedAt = step.StartedAt.Time
	}
	if step.EndedAt.Valid {
		endedAt = step.EndedAt.Time
	}
	if step.DurationMs.Valid {
		durationMs = step.DurationMs.Int64
	}

	if err := s.db.GetContext(ctx, &step.CreatedAt, query,
		step.ID, step.RunTestID, step.StepIndex, step.Name, step.Plugin, step.Status, errMsg,
		step.AssertionsPassed, step.AssertionsFailed, startedAt, endedAt, durationMs); err != nil {
		return RunStep{}, fmt.Errorf("failed to insert run step: %w", err)
	}

	return step, nil
}

// ListRunSteps returns all steps for a run test
func (s *Store) ListRunSteps(ctx context.Context, runTestID uuid.UUID) ([]RunStep, error) {
	const query = `
        SELECT id, run_test_id, step_index, name, plugin, status, error_message,
               assertions_passed, assertions_failed, started_at, ended_at, duration_ms, created_at
        FROM run_steps
        WHERE run_test_id = $1
        ORDER BY step_index ASC
    `

	var steps []RunStep
	if err := s.db.SelectContext(ctx, &steps, query, runTestID); err != nil {
		return nil, fmt.Errorf("failed to list run steps: %w", err)
	}
	if steps == nil {
		steps = []RunStep{}
	}

	return steps, nil
}

// InsertRunLog creates a new run log entry
func (s *Store) InsertRunLog(ctx context.Context, log RunLog) (RunLog, error) {
	if log.RunID == "" {
		return RunLog{}, errors.New("run id required")
	}
	if log.Message == "" {
		return RunLog{}, errors.New("message required")
	}

	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.Level == "" {
		log.Level = "INFO"
	}
	if log.LoggedAt.IsZero() {
		log.LoggedAt = time.Now().UTC()
	}

	const query = `
        INSERT INTO run_logs (id, run_id, run_test_id, run_step_id, level, message, metadata, logged_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
    `

	var runTestID, runStepID interface{}
	if log.RunTestID.Valid {
		runTestID = log.RunTestID.UUID
	}
	if log.RunStepID.Valid {
		runStepID = log.RunStepID.UUID
	}

	metadataJSON := []byte("null")
	if log.Metadata != nil {
		encoded, err := json.Marshal(log.Metadata)
		if err != nil {
			return RunLog{}, fmt.Errorf("failed to encode run log metadata: %w", err)
		}
		metadataJSON = encoded
	}

	if _, err := s.db.ExecContext(ctx, query,
		log.ID, log.RunID, runTestID, runStepID, log.Level, log.Message, string(metadataJSON), log.LoggedAt); err != nil {
		return RunLog{}, fmt.Errorf("failed to insert run log: %w", err)
	}

	return log, nil
}

// ListRunLogs returns logs for a run
func (s *Store) ListRunLogs(ctx context.Context, runID string, limit int) ([]RunLog, error) {
	if limit <= 0 {
		limit = 1000
	}
	if limit > 10000 {
		limit = 10000
	}

	const query = `
        SELECT id, run_id, run_test_id, run_step_id, level, message, metadata, logged_at
        FROM run_logs
        WHERE run_id = $1
        ORDER BY logged_at ASC
        LIMIT $2
    `

	rows := []struct {
		RunLog
		MetadataRaw []byte `db:"metadata"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, query, runID, limit); err != nil {
		return nil, fmt.Errorf("failed to list run logs: %w", err)
	}

	logs := make([]RunLog, 0, len(rows))
	for _, row := range rows {
		log := row.RunLog
		if len(row.MetadataRaw) > 0 {
			var meta map[string]interface{}
			if err := json.Unmarshal(row.MetadataRaw, &meta); err != nil {
				return nil, fmt.Errorf("failed to parse run log metadata: %w", err)
			}
			log.Metadata = meta
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// ListRunLogsByTest returns logs for a specific test within a run
func (s *Store) ListRunLogsByTest(ctx context.Context, runTestID uuid.UUID, limit int) ([]RunLog, error) {
	if limit <= 0 {
		limit = 1000
	}
	if limit > 10000 {
		limit = 10000
	}

	const query = `
        SELECT id, run_id, run_test_id, run_step_id, level, message, metadata, logged_at
        FROM run_logs
        WHERE run_test_id = $1
        ORDER BY logged_at ASC
        LIMIT $2
    `

	rows := []struct {
		RunLog
		MetadataRaw []byte `db:"metadata"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, query, runTestID, limit); err != nil {
		return nil, fmt.Errorf("failed to list run logs by test: %w", err)
	}

	logs := make([]RunLog, 0, len(rows))
	for _, row := range rows {
		log := row.RunLog
		if len(row.MetadataRaw) > 0 {
			var meta map[string]interface{}
			if err := json.Unmarshal(row.MetadataRaw, &meta); err != nil {
				return nil, fmt.Errorf("failed to parse run log metadata: %w", err)
			}
			log.Metadata = meta
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// InsertRunArtifact creates a new run artifact record
func (s *Store) InsertRunArtifact(ctx context.Context, artifact RunArtifact) (RunArtifact, error) {
	if artifact.RunID == "" {
		return RunArtifact{}, errors.New("run id required")
	}
	if artifact.Name == "" {
		return RunArtifact{}, errors.New("artifact name required")
	}
	if artifact.ArtifactType == "" {
		return RunArtifact{}, errors.New("artifact type required")
	}
	if artifact.StoragePath == "" {
		return RunArtifact{}, errors.New("storage path required")
	}

	if artifact.ID == uuid.Nil {
		artifact.ID = uuid.New()
	}

	const query = `
        INSERT INTO run_artifacts (
            id, run_id, run_test_id, run_step_id, name, artifact_type,
            mime_type, size_bytes, storage_path, created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
        RETURNING created_at
    `

	var runTestID, runStepID, mimeType, sizeBytes interface{}
	if artifact.RunTestID.Valid {
		runTestID = artifact.RunTestID.UUID
	}
	if artifact.RunStepID.Valid {
		runStepID = artifact.RunStepID.UUID
	}
	if artifact.MimeType.Valid {
		mimeType = artifact.MimeType.String
	}
	if artifact.SizeBytes.Valid {
		sizeBytes = artifact.SizeBytes.Int64
	}

	if err := s.db.GetContext(ctx, &artifact.CreatedAt, query,
		artifact.ID, artifact.RunID, runTestID, runStepID, artifact.Name, artifact.ArtifactType,
		mimeType, sizeBytes, artifact.StoragePath); err != nil {
		return RunArtifact{}, fmt.Errorf("failed to insert run artifact: %w", err)
	}

	return artifact, nil
}

// ListRunArtifacts returns all artifacts for a run
func (s *Store) ListRunArtifacts(ctx context.Context, runID string) ([]RunArtifact, error) {
	const query = `
        SELECT id, run_id, run_test_id, run_step_id, name, artifact_type,
               mime_type, size_bytes, storage_path, created_at
        FROM run_artifacts
        WHERE run_id = $1
        ORDER BY created_at ASC
    `

	var artifacts []RunArtifact
	if err := s.db.SelectContext(ctx, &artifacts, query, runID); err != nil {
		return nil, fmt.Errorf("failed to list run artifacts: %w", err)
	}
	if artifacts == nil {
		artifacts = []RunArtifact{}
	}

	return artifacts, nil
}

// GetRunArtifact retrieves an artifact by ID
func (s *Store) GetRunArtifact(ctx context.Context, id uuid.UUID) (RunArtifact, error) {
	const query = `
        SELECT id, run_id, run_test_id, run_step_id, name, artifact_type,
               mime_type, size_bytes, storage_path, created_at
        FROM run_artifacts
        WHERE id = $1
    `

	var artifact RunArtifact
	if err := s.db.GetContext(ctx, &artifact, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunArtifact{}, sql.ErrNoRows
		}
		return RunArtifact{}, fmt.Errorf("failed to get run artifact: %w", err)
	}

	return artifact, nil
}
