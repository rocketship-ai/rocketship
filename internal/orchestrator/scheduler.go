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
	ListDueProjectSchedules(ctx context.Context, before time.Time, limit int) ([]persistence.ProjectSchedule, error)
	ClaimDueProjectSchedule(ctx context.Context, scheduleID uuid.UUID, now time.Time) (bool, persistence.ProjectSchedule, error)
	UpdateProjectScheduleLastRun(ctx context.Context, scheduleID uuid.UUID, runID, status string, runAt time.Time) error
	ListActiveSuitesForProjectSchedule(ctx context.Context, projectID, environmentID uuid.UUID) ([]persistence.Suite, error)
	GetProjectWithOrg(ctx context.Context, projectID uuid.UUID) (persistence.ProjectWithOrg, error)
	GetEnvironmentBySlug(ctx context.Context, projectID uuid.UUID, slug string) (persistence.ProjectEnvironment, error)
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

	// Try to acquire leadership via transaction-scoped advisory lock
	// This is safe with connection pooling - lock is bound to the transaction
	acquired, tx, err := s.store.TryAcquireAdvisoryXactLock(ctx, SchedulerAdvisoryLockKey)
	if err != nil {
		s.logger.Error("scheduler: failed to acquire advisory lock", "error", err)
		return
	}
	if !acquired {
		s.logger.Debug("scheduler: another instance holds the lock, skipping tick")
		return
	}
	// Lock is automatically released when transaction ends (commit or rollback)
	defer func() {
		if err := tx.Rollback(); err != nil {
			// Rollback after commit is a no-op, so we ignore the error
			s.logger.Debug("scheduler: tx rollback (expected after commit)", "error", err)
		}
	}()

	s.logger.Debug("scheduler: acquired leadership, checking due schedules")

	// Process project schedules (run creation happens outside the lock transaction)
	if err := s.processProjectSchedules(ctx); err != nil {
		s.logger.Error("scheduler: failed to process project schedules", "error", err)
	}

	// Commit the transaction to release the lock
	if err := tx.Commit(); err != nil {
		s.logger.Error("scheduler: failed to commit lock transaction", "error", err)
	}
}

func (s *Scheduler) processProjectSchedules(ctx context.Context) error {
	now := time.Now().UTC()
	schedules, err := s.store.ListDueProjectSchedules(ctx, now, 100)
	if err != nil {
		return fmt.Errorf("failed to list due schedules: %w", err)
	}

	if len(schedules) == 0 {
		s.logger.Debug("scheduler: no due project schedules")
		return nil
	}

	s.logger.Info("scheduler: found due project schedules", "count", len(schedules))

	for _, schedule := range schedules {
		if err := s.fireProjectSchedule(ctx, schedule, now); err != nil {
			s.logger.Error("scheduler: failed to fire project schedule",
				"schedule_id", schedule.ID,
				"project_id", schedule.ProjectID,
				"environment", schedule.EnvironmentSlug,
				"error", err,
			)
			continue
		}
	}

	return nil
}

func (s *Scheduler) fireProjectSchedule(ctx context.Context, schedule persistence.ProjectSchedule, now time.Time) error {
	s.logger.Info("scheduler: firing project schedule",
		"schedule_id", schedule.ID,
		"schedule_name", schedule.Name,
		"project_id", schedule.ProjectID,
		"environment", schedule.EnvironmentSlug,
	)

	// Atomically claim the schedule if it's still due
	// This is safe in HA - only one instance can successfully claim
	claimed, updatedSchedule, err := s.store.ClaimDueProjectSchedule(ctx, schedule.ID, now)
	if err != nil {
		return fmt.Errorf("failed to claim schedule: %w", err)
	}
	if !claimed {
		s.logger.Debug("scheduler: schedule already claimed by another instance",
			"schedule_id", schedule.ID,
		)
		return nil
	}

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
		runID, status, err := s.fireSuiteRun(ctx, suite, project, schedule, updatedSchedule)
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
	updatedSchedule persistence.ProjectSchedule,
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
			"env":              schedule.EnvironmentSlug,
			"environment":     schedule.EnvironmentSlug,
			"rs_schedule_id":   schedule.ID.String(),
			"rs_schedule_type": "project",
			"rs_environment_id": schedule.EnvironmentID.String(),
		},
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
