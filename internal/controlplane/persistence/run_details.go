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
            request_data, response_data, assertions_passed, assertions_failed,
            started_at, ended_at, duration_ms, created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10, $11, $12, $13, $14, NOW())
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

	requestJSON := []byte("null")
	if step.RequestData != nil {
		encoded, err := json.Marshal(step.RequestData)
		if err != nil {
			return RunStep{}, fmt.Errorf("failed to encode request data: %w", err)
		}
		requestJSON = encoded
	}

	responseJSON := []byte("null")
	if step.ResponseData != nil {
		encoded, err := json.Marshal(step.ResponseData)
		if err != nil {
			return RunStep{}, fmt.Errorf("failed to encode response data: %w", err)
		}
		responseJSON = encoded
	}

	if err := s.db.GetContext(ctx, &step.CreatedAt, query,
		step.ID, step.RunTestID, step.StepIndex, step.Name, step.Plugin, step.Status, errMsg,
		string(requestJSON), string(responseJSON), step.AssertionsPassed, step.AssertionsFailed,
		startedAt, endedAt, durationMs); err != nil {
		return RunStep{}, fmt.Errorf("failed to insert run step: %w", err)
	}

	return step, nil
}

// ListRunSteps returns all steps for a run test
func (s *Store) ListRunSteps(ctx context.Context, runTestID uuid.UUID) ([]RunStep, error) {
	const query = `
        SELECT id, run_test_id, step_index, name, plugin, status, error_message,
               request_data, response_data, assertions_data, variables_data, step_config,
               assertions_passed, assertions_failed,
               started_at, ended_at, duration_ms, created_at
        FROM run_steps
        WHERE run_test_id = $1
        ORDER BY step_index ASC
    `

	rows := []struct {
		RunStep
		RequestDataRaw    []byte `db:"request_data"`
		ResponseDataRaw   []byte `db:"response_data"`
		AssertionsDataRaw []byte `db:"assertions_data"`
		VariablesDataRaw  []byte `db:"variables_data"`
		StepConfigRaw     []byte `db:"step_config"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, query, runTestID); err != nil {
		return nil, fmt.Errorf("failed to list run steps: %w", err)
	}

	steps := make([]RunStep, 0, len(rows))
	for _, row := range rows {
		step := row.RunStep
		if len(row.RequestDataRaw) > 0 && string(row.RequestDataRaw) != "null" {
			var data map[string]interface{}
			if err := json.Unmarshal(row.RequestDataRaw, &data); err != nil {
				return nil, fmt.Errorf("failed to parse request data: %w", err)
			}
			step.RequestData = data
		}
		if len(row.ResponseDataRaw) > 0 && string(row.ResponseDataRaw) != "null" {
			var data map[string]interface{}
			if err := json.Unmarshal(row.ResponseDataRaw, &data); err != nil {
				return nil, fmt.Errorf("failed to parse response data: %w", err)
			}
			step.ResponseData = data
		}
		if len(row.AssertionsDataRaw) > 0 && string(row.AssertionsDataRaw) != "null" {
			var data []AssertionResult
			if err := json.Unmarshal(row.AssertionsDataRaw, &data); err != nil {
				return nil, fmt.Errorf("failed to parse assertions data: %w", err)
			}
			step.AssertionsData = data
		}
		if len(row.VariablesDataRaw) > 0 && string(row.VariablesDataRaw) != "null" {
			var data []SavedVariable
			if err := json.Unmarshal(row.VariablesDataRaw, &data); err != nil {
				return nil, fmt.Errorf("failed to parse variables data: %w", err)
			}
			step.VariablesData = data
		}
		if len(row.StepConfigRaw) > 0 && string(row.StepConfigRaw) != "null" {
			var data map[string]interface{}
			if err := json.Unmarshal(row.StepConfigRaw, &data); err != nil {
				return nil, fmt.Errorf("failed to parse step config: %w", err)
			}
			step.StepConfig = data
		}
		steps = append(steps, step)
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

// RunTestWithRun contains a run test with its parent run summary
type RunTestWithRun struct {
	RunTest
	Run RunRecord
}

// GetRunTestWithRun retrieves a run test by ID along with its parent run
// Validates that the run belongs to the specified organization
func (s *Store) GetRunTestWithRun(ctx context.Context, orgID uuid.UUID, runTestID uuid.UUID) (RunTestWithRun, error) {
	const query = `
        SELECT
            rt.id, rt.run_id, rt.test_id, rt.workflow_id, rt.name, rt.status, rt.error_message,
            rt.started_at, rt.ended_at, rt.duration_ms, rt.step_count, rt.passed_steps, rt.failed_steps, rt.created_at,
            r.id AS "run.id", r.organization_id AS "run.organization_id", r.project_id AS "run.project_id",
            r.status AS "run.status", r.suite_name AS "run.suite_name", r.initiator AS "run.initiator",
            r.trigger AS "run.trigger", r.schedule_name AS "run.schedule_name", r.config_source AS "run.config_source",
            r.source AS "run.source", r.branch AS "run.branch", r.environment AS "run.environment",
            r.commit_sha AS "run.commit_sha", r.bundle_sha AS "run.bundle_sha",
            r.total_tests AS "run.total_tests", r.passed_tests AS "run.passed_tests",
            r.failed_tests AS "run.failed_tests", r.timeout_tests AS "run.timeout_tests",
            r.skipped_tests AS "run.skipped_tests", r.environment_id AS "run.environment_id",
            r.schedule_id AS "run.schedule_id", r.commit_message AS "run.commit_message",
            r.created_at AS "run.created_at", r.updated_at AS "run.updated_at",
            r.started_at AS "run.started_at", r.ended_at AS "run.ended_at"
        FROM run_tests rt
        JOIN runs r ON rt.run_id = r.id
        WHERE rt.id = $1 AND r.organization_id = $2
    `

	// We need to scan into separate structs then combine
	type row struct {
		RunTest
		RunID            string         `db:"run.id"`
		RunOrgID         uuid.UUID      `db:"run.organization_id"`
		RunProjectID     uuid.NullUUID  `db:"run.project_id"`
		RunStatus        string         `db:"run.status"`
		RunSuiteName     string         `db:"run.suite_name"`
		RunInitiator     string         `db:"run.initiator"`
		RunTrigger       string         `db:"run.trigger"`
		RunScheduleName  string         `db:"run.schedule_name"`
		RunConfigSource  string         `db:"run.config_source"`
		RunSource        string         `db:"run.source"`
		RunBranch        string         `db:"run.branch"`
		RunEnvironment   string         `db:"run.environment"`
		RunCommitSHA     sql.NullString `db:"run.commit_sha"`
		RunBundleSHA     sql.NullString `db:"run.bundle_sha"`
		RunTotalTests    int            `db:"run.total_tests"`
		RunPassedTests   int            `db:"run.passed_tests"`
		RunFailedTests   int            `db:"run.failed_tests"`
		RunTimeoutTests  int            `db:"run.timeout_tests"`
		RunSkippedTests  int            `db:"run.skipped_tests"`
		RunEnvironmentID uuid.NullUUID  `db:"run.environment_id"`
		RunScheduleID    uuid.NullUUID  `db:"run.schedule_id"`
		RunCommitMessage sql.NullString `db:"run.commit_message"`
		RunCreatedAt     time.Time      `db:"run.created_at"`
		RunUpdatedAt     time.Time      `db:"run.updated_at"`
		RunStartedAt     sql.NullTime   `db:"run.started_at"`
		RunEndedAt       sql.NullTime   `db:"run.ended_at"`
	}

	var r row
	if err := s.db.GetContext(ctx, &r, query, runTestID, orgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunTestWithRun{}, sql.ErrNoRows
		}
		return RunTestWithRun{}, fmt.Errorf("failed to get run test with run: %w", err)
	}

	return RunTestWithRun{
		RunTest: r.RunTest,
		Run: RunRecord{
			ID:             r.RunID,
			OrganizationID: r.RunOrgID,
			ProjectID:      r.RunProjectID,
			Status:         r.RunStatus,
			SuiteName:      r.RunSuiteName,
			Initiator:      r.RunInitiator,
			Trigger:        r.RunTrigger,
			ScheduleName:   r.RunScheduleName,
			ConfigSource:   r.RunConfigSource,
			Source:         r.RunSource,
			Branch:         r.RunBranch,
			Environment:    r.RunEnvironment,
			CommitSHA:      r.RunCommitSHA,
			BundleSHA:      r.RunBundleSHA,
			TotalTests:     r.RunTotalTests,
			PassedTests:    r.RunPassedTests,
			FailedTests:    r.RunFailedTests,
			TimeoutTests:   r.RunTimeoutTests,
			SkippedTests:   r.RunSkippedTests,
			EnvironmentID:  r.RunEnvironmentID,
			ScheduleID:     r.RunScheduleID,
			CommitMessage:  r.RunCommitMessage,
			CreatedAt:      r.RunCreatedAt,
			UpdatedAt:      r.RunUpdatedAt,
			StartedAt:      r.RunStartedAt,
			EndedAt:        r.RunEndedAt,
		},
	}, nil
}

// UpsertRunStep creates or updates a run step record
// Uses ON CONFLICT with the (run_test_id, step_index) unique constraint
func (s *Store) UpsertRunStep(ctx context.Context, step RunStep) (RunStep, error) {
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
            request_data, response_data, assertions_data, variables_data, step_config,
            assertions_passed, assertions_failed,
            started_at, ended_at, duration_ms, created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::jsonb, $11::jsonb, $12::jsonb, $13, $14, $15, $16, $17, NOW())
        ON CONFLICT (run_test_id, step_index) DO UPDATE SET
            name = EXCLUDED.name,
            plugin = EXCLUDED.plugin,
            status = EXCLUDED.status,
            error_message = EXCLUDED.error_message,
            request_data = EXCLUDED.request_data,
            response_data = EXCLUDED.response_data,
            assertions_data = EXCLUDED.assertions_data,
            variables_data = EXCLUDED.variables_data,
            step_config = EXCLUDED.step_config,
            assertions_passed = EXCLUDED.assertions_passed,
            assertions_failed = EXCLUDED.assertions_failed,
            started_at = COALESCE(run_steps.started_at, EXCLUDED.started_at),
            ended_at = EXCLUDED.ended_at,
            duration_ms = EXCLUDED.duration_ms
        RETURNING id, created_at
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

	requestJSON := []byte("null")
	if step.RequestData != nil {
		encoded, err := json.Marshal(step.RequestData)
		if err != nil {
			return RunStep{}, fmt.Errorf("failed to encode request data: %w", err)
		}
		requestJSON = encoded
	}

	responseJSON := []byte("null")
	if step.ResponseData != nil {
		encoded, err := json.Marshal(step.ResponseData)
		if err != nil {
			return RunStep{}, fmt.Errorf("failed to encode response data: %w", err)
		}
		responseJSON = encoded
	}

	assertionsJSON := []byte("null")
	if step.AssertionsData != nil {
		encoded, err := json.Marshal(step.AssertionsData)
		if err != nil {
			return RunStep{}, fmt.Errorf("failed to encode assertions data: %w", err)
		}
		assertionsJSON = encoded
	}

	variablesJSON := []byte("null")
	if step.VariablesData != nil {
		encoded, err := json.Marshal(step.VariablesData)
		if err != nil {
			return RunStep{}, fmt.Errorf("failed to encode variables data: %w", err)
		}
		variablesJSON = encoded
	}

	stepConfigJSON := []byte("null")
	if step.StepConfig != nil {
		encoded, err := json.Marshal(step.StepConfig)
		if err != nil {
			return RunStep{}, fmt.Errorf("failed to encode step config: %w", err)
		}
		stepConfigJSON = encoded
	}

	row := s.db.QueryRowxContext(ctx, query,
		step.ID, step.RunTestID, step.StepIndex, step.Name, step.Plugin, step.Status, errMsg,
		string(requestJSON), string(responseJSON), string(assertionsJSON), string(variablesJSON), string(stepConfigJSON),
		step.AssertionsPassed, step.AssertionsFailed,
		startedAt, endedAt, durationMs)

	if err := row.Scan(&step.ID, &step.CreatedAt); err != nil {
		return RunStep{}, fmt.Errorf("failed to upsert run step: %w", err)
	}

	return step, nil
}

// SetRunTestRunning updates a run_test status to RUNNING if it's currently PENDING.
// This is called when the first step of a test starts executing.
func (s *Store) SetRunTestRunning(ctx context.Context, runTestID uuid.UUID) error {
	if runTestID == uuid.Nil {
		return errors.New("run test id required")
	}

	const query = `
        UPDATE run_tests
        SET status = 'RUNNING'
        WHERE id = $1 AND status = 'PENDING'
    `

	// Note: We don't check rows affected because it's fine if the status
	// is already RUNNING (subsequent steps can trigger this call)
	_, err := s.db.ExecContext(ctx, query, runTestID)
	if err != nil {
		return fmt.Errorf("failed to set run test running: %w", err)
	}

	return nil
}

// UpdateRunTestStepCounts recomputes and updates passed/failed step counts for a run test based on run_steps.
// Note: step_count is NOT updated here - it represents the expected total steps from YAML and is set at insert time.
func (s *Store) UpdateRunTestStepCounts(ctx context.Context, runTestID uuid.UUID) error {
	if runTestID == uuid.Nil {
		return errors.New("run test id required")
	}

	const query = `
        UPDATE run_tests SET
            passed_steps = (SELECT COUNT(*) FROM run_steps WHERE run_test_id = $1 AND status = 'PASSED'),
            failed_steps = (SELECT COUNT(*) FROM run_steps WHERE run_test_id = $1 AND status = 'FAILED')
        WHERE id = $1
    `

	res, err := s.db.ExecContext(ctx, query, runTestID)
	if err != nil {
		return fmt.Errorf("failed to update run test step counts: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetRunTestByWorkflowID retrieves a run test by workflow ID (exposed for engine use)
// Note: This is already implemented above, but adding an alias for interface compliance
func (s *Store) LookupRunTestByWorkflowID(ctx context.Context, workflowID string) (RunTest, error) {
	return s.GetRunTestByWorkflowID(ctx, workflowID)
}
