package orchestrator

import (
	"context"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/handlers/auth"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/handlers/repositories"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/handlers/runs"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/handlers/teams"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/handlers/tokens"
	"github.com/rocketship-ai/rocketship/internal/rbac"
	"go.temporal.io/sdk/client"
)

// Engine is the main orchestrator that handles all gRPC endpoints
type Engine struct {
	generated.UnimplementedEngineServer
	
	// Handlers for different domains
	runsHandler         *runs.Handler
	teamsHandler        *teams.Handler
	repositoriesHandler *repositories.Handler
	tokensHandler       *tokens.Handler
	authHandler         *auth.Handler
}

// NewEngine creates a new engine with modular handlers
func NewEngine(c client.Client, rbacRepo *rbac.Repository) *Engine {
	return &Engine{
		runsHandler:         runs.NewHandler(c),
		teamsHandler:        teams.NewHandler(rbacRepo),
		repositoriesHandler: repositories.NewHandler(rbacRepo),
		tokensHandler:       tokens.NewHandler(rbacRepo),
		authHandler:         auth.NewHandler(rbacRepo),
	}
}

// ==================== RUN MANAGEMENT ====================

func (e *Engine) CreateRun(ctx context.Context, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
	return e.runsHandler.CreateRun(ctx, req)
}

func (e *Engine) StreamLogs(req *generated.LogStreamRequest, stream generated.Engine_StreamLogsServer) error {
	return e.runsHandler.StreamLogs(req, stream)
}

func (e *Engine) AddLog(ctx context.Context, req *generated.AddLogRequest) (*generated.AddLogResponse, error) {
	return e.runsHandler.AddLog(ctx, req)
}

func (e *Engine) ListRuns(ctx context.Context, req *generated.ListRunsRequest) (*generated.ListRunsResponse, error) {
	return e.runsHandler.ListRuns(ctx, req)
}

func (e *Engine) GetRun(ctx context.Context, req *generated.GetRunRequest) (*generated.GetRunResponse, error) {
	return e.runsHandler.GetRun(ctx, req)
}

func (e *Engine) CancelRun(ctx context.Context, req *generated.CancelRunRequest) (*generated.CancelRunResponse, error) {
	return e.runsHandler.CancelRun(ctx, req)
}

// ==================== TEAM MANAGEMENT ====================

func (e *Engine) CreateTeam(ctx context.Context, req *generated.CreateTeamRequest) (*generated.CreateTeamResponse, error) {
	return e.teamsHandler.CreateTeam(ctx, req)
}

func (e *Engine) ListTeams(ctx context.Context, req *generated.ListTeamsRequest) (*generated.ListTeamsResponse, error) {
	return e.teamsHandler.ListTeams(ctx, req)
}

func (e *Engine) AddTeamMember(ctx context.Context, req *generated.AddTeamMemberRequest) (*generated.AddTeamMemberResponse, error) {
	return e.teamsHandler.AddTeamMember(ctx, req)
}

func (e *Engine) GetTeam(ctx context.Context, req *generated.GetTeamRequest) (*generated.GetTeamResponse, error) {
	return e.teamsHandler.GetTeam(ctx, req)
}

func (e *Engine) RemoveTeamMember(ctx context.Context, req *generated.RemoveTeamMemberRequest) (*generated.RemoveTeamMemberResponse, error) {
	return e.teamsHandler.RemoveTeamMember(ctx, req)
}

func (e *Engine) AddTeamRepository(ctx context.Context, req *generated.AddTeamRepositoryRequest) (*generated.AddTeamRepositoryResponse, error) {
	return e.teamsHandler.AddTeamRepository(ctx, req)
}

func (e *Engine) RemoveTeamRepository(ctx context.Context, req *generated.RemoveTeamRepositoryRequest) (*generated.RemoveTeamRepositoryResponse, error) {
	return e.teamsHandler.RemoveTeamRepository(ctx, req)
}

// ==================== REPOSITORY MANAGEMENT ====================

func (e *Engine) AddRepository(ctx context.Context, req *generated.AddRepositoryRequest) (*generated.AddRepositoryResponse, error) {
	return e.repositoriesHandler.AddRepository(ctx, req)
}

func (e *Engine) ListRepositories(ctx context.Context, req *generated.ListRepositoriesRequest) (*generated.ListRepositoriesResponse, error) {
	return e.repositoriesHandler.ListRepositories(ctx, req)
}

func (e *Engine) GetRepository(ctx context.Context, req *generated.GetRepositoryRequest) (*generated.GetRepositoryResponse, error) {
	return e.repositoriesHandler.GetRepository(ctx, req)
}

func (e *Engine) RemoveRepository(ctx context.Context, req *generated.RemoveRepositoryRequest) (*generated.RemoveRepositoryResponse, error) {
	return e.repositoriesHandler.RemoveRepository(ctx, req)
}

func (e *Engine) AssignTeamToRepository(ctx context.Context, req *generated.AssignTeamToRepositoryRequest) (*generated.AssignTeamToRepositoryResponse, error) {
	return e.repositoriesHandler.AssignTeamToRepository(ctx, req)
}

func (e *Engine) UnassignTeamFromRepository(ctx context.Context, req *generated.UnassignTeamFromRepositoryRequest) (*generated.UnassignTeamFromRepositoryResponse, error) {
	return e.repositoriesHandler.UnassignTeamFromRepository(ctx, req)
}

// ==================== TOKEN MANAGEMENT ====================

func (e *Engine) CreateToken(ctx context.Context, req *generated.CreateTokenRequest) (*generated.CreateTokenResponse, error) {
	return e.tokensHandler.CreateToken(ctx, req)
}

func (e *Engine) ListTokens(ctx context.Context, req *generated.ListTokensRequest) (*generated.ListTokensResponse, error) {
	return e.tokensHandler.ListTokens(ctx, req)
}

func (e *Engine) RevokeToken(ctx context.Context, req *generated.RevokeTokenRequest) (*generated.RevokeTokenResponse, error) {
	return e.tokensHandler.RevokeToken(ctx, req)
}

// ==================== AUTH MANAGEMENT ====================

func (e *Engine) GetAuthConfig(ctx context.Context, req *generated.GetAuthConfigRequest) (*generated.GetAuthConfigResponse, error) {
	return e.authHandler.GetAuthConfig(ctx, req)
}

func (e *Engine) GetCurrentUser(ctx context.Context, req *generated.GetCurrentUserRequest) (*generated.GetCurrentUserResponse, error) {
	return e.authHandler.GetCurrentUser(ctx, req)
}

// ==================== HEALTH CHECK ====================

func (e *Engine) Health(ctx context.Context, req *generated.HealthRequest) (*generated.HealthResponse, error) {
	return &generated.HealthResponse{
		Status: "ok",
	}, nil
}