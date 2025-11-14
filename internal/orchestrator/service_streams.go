package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
)

// StreamLogs streams logs for a test run in real-time
func (e *Engine) StreamLogs(req *generated.LogStreamRequest, stream generated.Engine_StreamLogsServer) error {
	runID := req.RunId

	_, orgID, err := e.resolvePrincipalAndOrg(stream.Context())
	if err != nil {
		return err
	}

	e.mu.RLock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.RUnlock()
		return fmt.Errorf("run not found: %s", runID)
	}
	if orgID != uuid.Nil && runInfo.OrganizationID != uuid.Nil && runInfo.OrganizationID != orgID {
		e.mu.RUnlock()
		return fmt.Errorf("run not found: %s", runID)
	}

	logs := make([]LogLine, len(runInfo.Logs))
	copy(logs, runInfo.Logs)
	e.mu.RUnlock()

	for _, logMsg := range logs {
		if err := stream.Send(&generated.LogLine{
			Ts:       time.Now().Format(time.RFC3339),
			Msg:      logMsg.Msg,
			Color:    logMsg.Color,
			Bold:     logMsg.Bold,
			TestName: logMsg.TestName,
			StepName: logMsg.StepName,
		}); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastLogIndex := len(logs)

	for {
		select {
		case <-ticker.C:
			e.mu.RLock()
			runInfo, exists := e.runs[runID]
			if !exists {
				e.mu.RUnlock()
				return fmt.Errorf("run not found: %s", runID)
			}
			if orgID != uuid.Nil && runInfo.OrganizationID != uuid.Nil && runInfo.OrganizationID != orgID {
				e.mu.RUnlock()
				return fmt.Errorf("run not found: %s", runID)
			}

			var newLogs []LogLine
			status := runInfo.Status

			if len(runInfo.Logs) > lastLogIndex {
				newLogs = make([]LogLine, len(runInfo.Logs)-lastLogIndex)
				copy(newLogs, runInfo.Logs[lastLogIndex:])
				lastLogIndex = len(runInfo.Logs)
			}
			e.mu.RUnlock()

			for _, logMsg := range newLogs {
				if err := stream.Send(&generated.LogLine{
					Ts:       time.Now().Format(time.RFC3339),
					Msg:      logMsg.Msg,
					Color:    logMsg.Color,
					Bold:     logMsg.Bold,
					TestName: logMsg.TestName,
					StepName: logMsg.StepName,
				}); err != nil {
					return err
				}
			}

			if status == "PASSED" || status == "FAILED" {
				return nil
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

// AddLog adds a log entry to a test run
func (e *Engine) AddLog(ctx context.Context, req *generated.AddLogRequest) (*generated.AddLogResponse, error) {
	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}
	if req.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	_, orgID, err := e.resolvePrincipalAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	runInfo, exists := e.runs[req.RunId]
	if !exists {
		e.mu.RUnlock()
		return nil, fmt.Errorf("run not found: %s", req.RunId)
	}
	if orgID != uuid.Nil && runInfo.OrganizationID != uuid.Nil && runInfo.OrganizationID != orgID {
		e.mu.RUnlock()
		return nil, fmt.Errorf("run not found: %s", req.RunId)
	}
	e.mu.RUnlock()

	e.addLogWithContext(req.RunId, req.Message, req.Color, req.Bold, req.TestName, req.StepName)
	return &generated.AddLogResponse{}, nil
}
