package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// ValidationService handles repository validation and CODEOWNERS management
type ValidationService struct {
	githubClient *Client
	rbacRepo     *rbac.Repository
}

// NewValidationService creates a new validation service
func NewValidationService(githubClient *Client, rbacRepo *rbac.Repository) *ValidationService {
	return &ValidationService{
		githubClient: githubClient,
		rbacRepo:     rbacRepo,
	}
}

// ValidateAndCreateRepository validates a GitHub repository URL and creates/updates it in the database
func (s *ValidationService) ValidateAndCreateRepository(ctx context.Context, repoURL string, enforceCodeowners bool) (*rbac.RepositoryEntity, error) {
	// Parse and validate repository URL
	owner, repo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Check if repository exists and is accessible
	githubRepo, err := s.githubClient.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to access repository: %w", err)
	}

	// Normalize the repository URL to the standard GitHub format
	normalizedURL := githubRepo.GetHTMLURL()

	// Check if repository already exists in database
	existingRepo, err := s.rbacRepo.GetRepository(ctx, normalizedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing repository: %w", err)
	}

	if existingRepo != nil {
		// Repository already exists, update it if needed
		if existingRepo.EnforceCodeowners != enforceCodeowners {
			existingRepo.EnforceCodeowners = enforceCodeowners
			if err := s.rbacRepo.UpdateRepository(ctx, existingRepo); err != nil {
				return nil, fmt.Errorf("failed to update repository: %w", err)
			}
		}

		// Refresh CODEOWNERS cache if enforcement is enabled
		if enforceCodeowners {
			if err := s.RefreshCodeownersCache(ctx, existingRepo.ID); err != nil {
				// Log warning but don't fail - CODEOWNERS refresh can be retried later
				fmt.Printf("Warning: failed to refresh CODEOWNERS cache for repository %s: %v\n", normalizedURL, err)
			}
		}

		return existingRepo, nil
	}

	// Create new repository entity
	repoEntity := &rbac.RepositoryEntity{
		ID:                uuid.New().String(),
		URL:               normalizedURL,
		EnforceCodeowners: enforceCodeowners,
		CreatedAt:         time.Now(),
	}

	// Create repository in database
	if err := s.rbacRepo.CreateRepository(ctx, repoEntity); err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	// Fetch and cache CODEOWNERS if enforcement is enabled
	if enforceCodeowners {
		if err := s.RefreshCodeownersCache(ctx, repoEntity.ID); err != nil {
			// Log warning but don't fail - CODEOWNERS can be cached later
			fmt.Printf("Warning: failed to cache CODEOWNERS for new repository %s: %v\n", normalizedURL, err)
		}
	}

	return repoEntity, nil
}

// RefreshCodeownersCache fetches and caches the CODEOWNERS file for a repository
func (s *ValidationService) RefreshCodeownersCache(ctx context.Context, repoID string) error {
	// Get repository from database
	repo, err := s.rbacRepo.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	if repo == nil {
		return fmt.Errorf("repository not found: %s", repoID)
	}

	// Parse repository URL to get owner/repo
	owner, repoName, err := ParseRepositoryURL(repo.URL)
	if err != nil {
		return fmt.Errorf("failed to parse repository URL: %w", err)
	}

	// Fetch CODEOWNERS from GitHub
	codeownersData, err := s.githubClient.GetCodeowners(ctx, owner, repoName, "")
	if err != nil {
		return fmt.Errorf("failed to fetch CODEOWNERS: %w", err)
	}

	// Serialize CODEOWNERS data for caching
	cacheData, err := json.Marshal(codeownersData)
	if err != nil {
		return fmt.Errorf("failed to serialize CODEOWNERS data: %w", err)
	}

	// Update repository with cached CODEOWNERS
	if err := s.rbacRepo.UpdateRepositoryCodeowners(ctx, repoID, cacheData); err != nil {
		return fmt.Errorf("failed to update CODEOWNERS cache: %w", err)
	}

	return nil
}

// ValidateRepositoryAccess checks if a repository URL is valid and accessible
func (s *ValidationService) ValidateRepositoryAccess(ctx context.Context, repoURL string) error {
	owner, repo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}

	_, err = s.githubClient.GetRepository(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("repository not accessible: %w", err)
	}

	return nil
}

// CheckPathPermissions checks if teams have permission to access a specific file path
func (s *ValidationService) CheckPathPermissions(ctx context.Context, repoURL, filePath string, teamNames []string) (bool, error) {
	// Get repository from database
	repo, err := s.rbacRepo.GetRepository(ctx, repoURL)
	if err != nil {
		return false, fmt.Errorf("failed to get repository: %w", err)
	}

	if repo == nil {
		return false, fmt.Errorf("repository not registered: %s", repoURL)
	}

	// If CODEOWNERS enforcement is disabled, allow access
	if !repo.EnforceCodeowners {
		return true, nil
	}

	// Parse cached CODEOWNERS data
	if repo.CodeownersCache == nil {
		// No CODEOWNERS cache - try to refresh it
		if err := s.RefreshCodeownersCache(ctx, repo.ID); err != nil {
			return false, fmt.Errorf("failed to load CODEOWNERS: %w", err)
		}

		// Re-fetch repository with updated cache
		repo, err = s.rbacRepo.GetRepositoryByID(ctx, repo.ID)
		if err != nil {
			return false, fmt.Errorf("failed to re-fetch repository: %w", err)
		}
	}

	var codeownersData rbac.CodeownersData
	if err := json.Unmarshal(repo.CodeownersCache, &codeownersData); err != nil {
		return false, fmt.Errorf("failed to parse CODEOWNERS cache: %w", err)
	}

	// Check path ownership using GitHub client
	return s.githubClient.CheckPathOwnership(&codeownersData, filePath, teamNames), nil
}

// GetRepositoryMetadata returns metadata about a repository
func (s *ValidationService) GetRepositoryMetadata(ctx context.Context, repoURL string) (*RepositoryMetadata, error) {
	return s.githubClient.GetRepositoryMetadata(ctx, repoURL)
}

// RefreshAllCodeownersCache refreshes CODEOWNERS cache for all repositories with enforcement enabled
func (s *ValidationService) RefreshAllCodeownersCache(ctx context.Context) error {
	// Get all repositories
	repositories, err := s.rbacRepo.ListRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	var errors []string
	for _, repo := range repositories {
		if repo.EnforceCodeowners {
			if err := s.RefreshCodeownersCache(ctx, repo.ID); err != nil {
				errors = append(errors, fmt.Sprintf("repository %s: %v", repo.URL, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to refresh some CODEOWNERS caches: %v", errors)
	}

	return nil
}

// GetCodeownersInfo returns information about a repository's CODEOWNERS file
func (s *ValidationService) GetCodeownersInfo(ctx context.Context, repoURL string) (*CodeownersInfo, error) {
	owner, repo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Try to fetch CODEOWNERS from GitHub
	codeownersData, err := s.githubClient.GetCodeowners(ctx, owner, repo, "")
	if err != nil {
		return &CodeownersInfo{
			HasCodeowners: false,
			Error:         err.Error(),
		}, nil
	}

	info := &CodeownersInfo{
		HasCodeowners: true,
		RuleCount:     len(codeownersData.Rules),
		LastUpdated:   codeownersData.Updated,
	}

	// Extract unique owners
	ownerSet := make(map[string]bool)
	for _, rule := range codeownersData.Rules {
		for _, owner := range rule.Owners {
			ownerSet[owner] = true
		}
	}

	for owner := range ownerSet {
		info.Owners = append(info.Owners, owner)
	}

	return info, nil
}

// CodeownersInfo contains information about a repository's CODEOWNERS file
type CodeownersInfo struct {
	HasCodeowners bool      `json:"has_codeowners"`
	RuleCount     int       `json:"rule_count"`
	Owners        []string  `json:"owners"`
	LastUpdated   time.Time `json:"last_updated"`
	Error         string    `json:"error,omitempty"`
}