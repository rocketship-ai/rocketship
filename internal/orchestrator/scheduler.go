package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

const (
	// DefaultSchedulerPollInterval is how often the scheduler checks for due schedules
	DefaultSchedulerPollInterval = 10 * time.Second
	// SchedulerAdvisoryLockKey is the Postgres advisory lock key for scheduler leadership
	SchedulerAdvisoryLockKey int64 = 7700001
)

// Scheduler runs scheduled test suites in the background
type Scheduler struct {
	engine       *Engine
	store        SchedulerStore
	pollInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	logger       *slog.Logger
}

// SchedulerStore defines the database interface required by the scheduler
type SchedulerStore interface {
	// Advisory lock for leader election (transaction-scoped for HA safety)
	TryAcquireAdvisoryXactLock(ctx context.Context, lockKey int64) (bool, persistence.SchedulerTx, error)

	// Project schedule operations
	ListDueProjectScheduleIDs(ctx context.Context, before time.Time, limit int) ([]uuid.UUID, error)
	ClaimDueProjectSchedule(ctx context.Context, scheduleID uuid.UUID, now time.Time) (bool, persistence.ProjectSchedule, error)
	UpdateProjectScheduleLastRun(ctx context.Context, scheduleID uuid.UUID, runID, status string, runAt time.Time) error
	ListActiveSuitesForProjectSchedule(ctx context.Context, projectID, environmentID uuid.UUID) ([]persistence.Suite, error)
	GetProjectWithOrg(ctx context.Context, projectID uuid.UUID) (persistence.ProjectWithOrg, error)
	GetEnvironmentBySlug(ctx context.Context, projectID uuid.UUID, slug string) (persistence.ProjectEnvironment, error)

	// Suite schedule operations (overrides)
	ListDueSuiteScheduleIDs(ctx context.Context, before time.Time, limit int) ([]uuid.UUID, error)
	ClaimDueSuiteSchedule(ctx context.Context, scheduleID uuid.UUID, now time.Time) (bool, persistence.SuiteScheduleWithEnv, error)
	UpdateSuiteScheduleLastRun(ctx context.Context, scheduleID uuid.UUID, runID, status string, runAt time.Time) error
	GetSuiteWithProjectAndEnv(ctx context.Context, suiteID, environmentID uuid.UUID) (persistence.SuiteWithProjectAndEnv, error)
}

// NewScheduler creates a new scheduler
func NewScheduler(engine *Engine, store SchedulerStore, logger *slog.Logger) *Scheduler {
	pollInterval := DefaultSchedulerPollInterval
	if envInterval := os.Getenv("ROCKETSHIP_SCHEDULER_POLL_INTERVAL"); envInterval != "" {
		if secs, err := strconv.Atoi(envInterval); err == nil && secs > 0 {
			pollInterval = time.Duration(secs) * time.Second
		}
	}

	return &Scheduler{
		engine:       engine,
		store:        store,
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
		logger:       logger,
	}
}

// Start begins the scheduler loop
func (s *Scheduler) Start() {
	s.wg.Add(1)
	go s.run()
	s.logger.Info("scheduler started", "poll_interval", s.pollInterval)
}

// Stop gracefully shuts down the scheduler
func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("scheduler stopped")
}

func (s *Scheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	now := time.Now().UTC()

	// Phase 1: Acquire leadership and discover due schedules (fast, holds lock briefly)
	projectScheduleIDs, suiteScheduleIDs, err := s.discoverDueSchedules(ctx, now)
	if err != nil {
		if err != errLockNotAcquired {
			s.logger.Error("scheduler: failed to discover due schedules", "error", err)
		}
		return
	}

	if len(projectScheduleIDs) == 0 && len(suiteScheduleIDs) == 0 {
		s.logger.Debug("scheduler: no due schedules")
		return
	}

	if len(projectScheduleIDs) > 0 {
		s.logger.Info("scheduler: found due project schedules", "count", len(projectScheduleIDs))
	}
	if len(suiteScheduleIDs) > 0 {
		s.logger.Info("scheduler: found due suite schedules", "count", len(suiteScheduleIDs))
	}

	// Phase 2: Process schedules outside the lock (lock already released)
	// Each schedule is claimed atomically

	// Process project schedules
	for _, scheduleID := range projectScheduleIDs {
		if err := s.fireProjectScheduleByID(ctx, scheduleID, now); err != nil {
			s.logger.Error("scheduler: failed to fire project schedule",
				"schedule_id", scheduleID,
				"error", err,
			)
			continue
		}
	}

	// Process suite schedules (overrides)
	for _, scheduleID := range suiteScheduleIDs {
		if err := s.fireSuiteScheduleByID(ctx, scheduleID, now); err != nil {
			s.logger.Error("scheduler: failed to fire suite schedule",
				"schedule_id", scheduleID,
				"error", err,
			)
			continue
		}
	}
}

// errLockNotAcquired is returned when another instance holds the scheduler lock
var errLockNotAcquired = fmt.Errorf("lock not acquired")

// discoverDueSchedules acquires the advisory lock, fetches due schedule IDs, and releases
// the lock immediately. This minimizes lock hold time to milliseconds instead of minutes.
// Returns (projectScheduleIDs, suiteScheduleIDs, error)
func (s *Scheduler) discoverDueSchedules(ctx context.Context, now time.Time) ([]uuid.UUID, []uuid.UUID, error) {
	// Try to acquire leadership via transaction-scoped advisory lock
	// This is safe with connection pooling - lock is bound to the transaction
	acquired, tx, err := s.store.TryAcquireAdvisoryXactLock(ctx, SchedulerAdvisoryLockKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to acquire advisory lock: %w", err)
	}
	if !acquired {
		s.logger.Debug("scheduler: another instance holds the lock, skipping tick")
		return nil, nil, errLockNotAcquired
	}
	defer func() {
		_ = tx.Rollback()
	}()

	s.logger.Debug("scheduler: acquired leadership, checking due schedules")

	// Fetch due project schedule IDs (fast query, no row locks)
	projectScheduleIDs, err := s.store.ListDueProjectScheduleIDs(ctx, now, 100)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list due project schedules: %w", err)
	}

	// Fetch due suite schedule IDs (fast query, no row locks)
	suiteScheduleIDs, err := s.store.ListDueSuiteScheduleIDs(ctx, now, 100)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list due suite schedules: %w", err)
	}

	// Commit immediately to release the advisory lock
	// The lock is now held for only milliseconds (discovery phase)
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("failed to commit lock transaction: %w", err)
	}

	return projectScheduleIDs, suiteScheduleIDs, nil
}

// fireProjectScheduleByID attempts to claim and fire a schedule by its ID.
// This is called outside the advisory lock, relying on ClaimDueProjectSchedule for atomicity.
func (s *Scheduler) fireProjectScheduleByID(ctx context.Context, scheduleID uuid.UUID, now time.Time) error {
	s.logger.Debug("scheduler: attempting to claim schedule", "schedule_id", scheduleID)

	// Atomically claim the schedule if it's still due
	// This is safe in HA - only one instance can successfully claim
	claimed, schedule, err := s.store.ClaimDueProjectSchedule(ctx, scheduleID, now)
	if err != nil {
		return fmt.Errorf("failed to claim schedule: %w", err)
	}
	if !claimed {
		s.logger.Debug("scheduler: schedule already claimed by another instance",
			"schedule_id", scheduleID,
		)
		return nil
	}

	s.logger.Info("scheduler: firing project schedule",
		"schedule_id", schedule.ID,
		"schedule_name", schedule.Name,
		"project_id", schedule.ProjectID,
		"environment", schedule.EnvironmentSlug,
	)

	// Get project with org ID
	project, err := s.store.GetProjectWithOrg(ctx, schedule.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Get active suites for this project's default branch (excluding suites with overrides)
	suites, err := s.store.ListActiveSuitesForProjectSchedule(ctx, schedule.ProjectID, schedule.EnvironmentID)
	if err != nil {
		return fmt.Errorf("failed to list suites: %w", err)
	}

	if len(suites) == 0 {
		s.logger.Warn("scheduler: no active suites found for project schedule",
			"schedule_id", schedule.ID,
			"project_id", schedule.ProjectID,
		)
		return nil
	}

	s.logger.Info("scheduler: running suites for project schedule",
		"schedule_id", schedule.ID,
		"suite_count", len(suites),
	)

	var firstRunID string
	var lastStatus string

	// Fire a run for each suite
	for _, suite := range suites {
		runID, status, err := s.fireSuiteRun(ctx, suite, project, schedule)
		if err != nil {
			s.logger.Error("scheduler: failed to fire suite run",
				"schedule_id", schedule.ID,
				"suite_id", suite.ID,
				"suite_name", suite.Name,
				"error", err,
			)
			lastStatus = "FAILED"
			continue
		}
		if firstRunID == "" {
			firstRunID = runID
		}
		lastStatus = status
	}

	// Update schedule's last run info
	if firstRunID != "" {
		if err := s.store.UpdateProjectScheduleLastRun(ctx, schedule.ID, firstRunID, lastStatus, time.Now().UTC()); err != nil {
			s.logger.Error("scheduler: failed to update schedule last run",
				"schedule_id", schedule.ID,
				"error", err,
			)
		}
	}

	return nil
}

func (s *Scheduler) fireSuiteRun(
	ctx context.Context,
	suite persistence.Suite,
	project persistence.ProjectWithOrg,
	schedule persistence.ProjectSchedule,
) (string, string, error) {
	if suite.YamlPayload == "" {
		return "", "", fmt.Errorf("suite %s has no yaml_payload", suite.Name)
	}

	// Build run context
	runContext := &generated.RunContext{
		ProjectId:    project.ID.String(),
		Branch:       project.DefaultBranch,
		Trigger:      "schedule",
		Source:       "scheduler",
		ScheduleName: schedule.Name,
		Metadata: map[string]string{
			"env":               schedule.EnvironmentSlug,
			"environment":       schedule.EnvironmentSlug,
			"rs_schedule_id":    schedule.ID.String(),
			"rs_schedule_type":  "project",
			"rs_environment_id": schedule.EnvironmentID.String(),
		},
	}

	// Add commit metadata from project's default branch HEAD (if available)
	if project.DefaultBranchHeadSHA != "" {
		runContext.CommitSha = project.DefaultBranchHeadSHA
	}
	if project.DefaultBranchHeadMessage != "" {
		runContext.Metadata["rs_commit_message"] = project.DefaultBranchHeadMessage
	}

	// Create the run request
	req := &generated.CreateRunRequest{
		YamlPayload: []byte(suite.YamlPayload),
		Context:     runContext,
	}

	// Create run through the engine (bypasses gRPC, calls internal method)
	resp, err := s.engine.createRunInternal(ctx, project.OrganizationID, "schedule:"+schedule.ID.String(), req)
	if err != nil {
		return "", "", fmt.Errorf("failed to create run: %w", err)
	}

	s.logger.Info("scheduler: created run for suite",
		"run_id", resp.RunId,
		"suite_name", suite.Name,
		"schedule_id", schedule.ID,
		"environment", schedule.EnvironmentSlug,
	)

	return resp.RunId, "RUNNING", nil
}

// fireSuiteScheduleByID attempts to claim and fire a suite schedule override by its ID.
// This is called outside the advisory lock, relying on ClaimDueSuiteSchedule for atomicity.
func (s *Scheduler) fireSuiteScheduleByID(ctx context.Context, scheduleID uuid.UUID, now time.Time) error {
	s.logger.Debug("scheduler: attempting to claim suite schedule", "schedule_id", scheduleID)

	// Atomically claim the schedule if it's still due
	// This is safe in HA - only one instance can successfully claim
	claimed, schedule, err := s.store.ClaimDueSuiteSchedule(ctx, scheduleID, now)
	if err != nil {
		return fmt.Errorf("failed to claim suite schedule: %w", err)
	}
	if !claimed {
		s.logger.Debug("scheduler: suite schedule already claimed by another instance",
			"schedule_id", scheduleID,
		)
		return nil
	}

	s.logger.Info("scheduler: firing suite schedule",
		"schedule_id", schedule.ID,
		"schedule_name", schedule.Name,
		"suite_id", schedule.SuiteID,
		"environment", schedule.EnvironmentSlug,
	)

	// Get suite with project and environment info
	suiteWithEnv, err := s.store.GetSuiteWithProjectAndEnv(ctx, schedule.SuiteID, schedule.EnvironmentID.UUID)
	if err != nil {
		return fmt.Errorf("failed to get suite with project/env: %w", err)
	}

	// Verify the suite is on the default branch
	if suiteWithEnv.SourceRef != suiteWithEnv.ProjectDefaultBranch {
		s.logger.Warn("scheduler: skipping suite schedule - suite not on default branch",
			"schedule_id", schedule.ID,
			"suite_id", schedule.SuiteID,
			"suite_source_ref", suiteWithEnv.SourceRef,
			"project_default_branch", suiteWithEnv.ProjectDefaultBranch,
		)
		return nil
	}

	if suiteWithEnv.YamlPayload == "" {
		s.logger.Warn("scheduler: skipping suite schedule - no yaml_payload",
			"schedule_id", schedule.ID,
			"suite_id", schedule.SuiteID,
		)
		return nil
	}

	// Fire the run for this single suite
	runID, _, err := s.fireSuiteRunForSuiteSchedule(ctx, suiteWithEnv, schedule)
	if err != nil {
		s.logger.Error("scheduler: failed to fire suite run for suite schedule",
			"schedule_id", schedule.ID,
			"suite_id", schedule.SuiteID,
			"error", err,
		)
		// Update last run status to FAILED
		_ = s.store.UpdateSuiteScheduleLastRun(ctx, schedule.ID, "", "FAILED", time.Now().UTC())
		return err
	}

	// Update schedule's last run info
	if err := s.store.UpdateSuiteScheduleLastRun(ctx, schedule.ID, runID, "RUNNING", time.Now().UTC()); err != nil {
		s.logger.Error("scheduler: failed to update suite schedule last run",
			"schedule_id", schedule.ID,
			"error", err,
		)
	}

	return nil
}

func (s *Scheduler) fireSuiteRunForSuiteSchedule(
	ctx context.Context,
	suiteWithEnv persistence.SuiteWithProjectAndEnv,
	schedule persistence.SuiteScheduleWithEnv,
) (string, string, error) {
	// Build run context
	runContext := &generated.RunContext{
		ProjectId:    suiteWithEnv.ProjectID.String(),
		Branch:       suiteWithEnv.ProjectDefaultBranch,
		Trigger:      "schedule",
		Source:       "scheduler",
		ScheduleName: schedule.Name,
		Metadata: map[string]string{
			"env":               schedule.EnvironmentSlug,
			"environment":       schedule.EnvironmentSlug,
			"rs_schedule_id":    schedule.ID.String(),
			"rs_schedule_type":  "suite",
			"rs_environment_id": schedule.EnvironmentID.UUID.String(),
		},
	}

	// Add commit metadata from project's default branch HEAD (if available)
	if suiteWithEnv.ProjectDefaultBranchHeadSHA != "" {
		runContext.CommitSha = suiteWithEnv.ProjectDefaultBranchHeadSHA
	}
	if suiteWithEnv.ProjectDefaultBranchHeadMsg != "" {
		runContext.Metadata["rs_commit_message"] = suiteWithEnv.ProjectDefaultBranchHeadMsg
	}

	// Create the run request
	req := &generated.CreateRunRequest{
		YamlPayload: []byte(suiteWithEnv.YamlPayload),
		Context:     runContext,
	}

	// Create run through the engine (bypasses gRPC, calls internal method)
	resp, err := s.engine.createRunInternal(ctx, suiteWithEnv.ProjectOrganizationID, "schedule:"+schedule.ID.String(), req)
	if err != nil {
		return "", "", fmt.Errorf("failed to create run: %w", err)
	}

	s.logger.Info("scheduler: created run for suite schedule",
		"run_id", resp.RunId,
		"suite_name", suiteWithEnv.Name,
		"schedule_id", schedule.ID,
		"schedule_type", "suite",
		"environment", schedule.EnvironmentSlug,
	)

	return resp.RunId, "RUNNING", nil
}
