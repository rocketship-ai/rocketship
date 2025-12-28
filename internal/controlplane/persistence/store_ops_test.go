package persistence

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestRoleSummary_AggregatedRoles(t *testing.T) {
	tests := []struct {
		name     string
		summary  RoleSummary
		expected []string
	}{
		{
			name: "organization admin has owner role",
			summary: RoleSummary{
				Organizations: []OrganizationMembership{
					{OrganizationID: uuid.New(), IsAdmin: true},
				},
				Projects: nil,
			},
			expected: []string{"owner"},
		},
		{
			name: "multiple organizations still one owner role",
			summary: RoleSummary{
				Organizations: []OrganizationMembership{
					{OrganizationID: uuid.New(), IsAdmin: true},
					{OrganizationID: uuid.New(), IsAdmin: true},
				},
				Projects: nil,
			},
			expected: []string{"owner"},
		},
		{
			name: "project write role gives editor",
			summary: RoleSummary{
				Organizations: nil,
				Projects: []ProjectMembership{
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "write"},
				},
			},
			expected: []string{"editor"},
		},
		{
			name: "project read role gives viewer",
			summary: RoleSummary{
				Organizations: nil,
				Projects: []ProjectMembership{
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "read"},
				},
			},
			expected: []string{"viewer"},
		},
		{
			name: "both write and read projects gives editor and viewer",
			summary: RoleSummary{
				Organizations: nil,
				Projects: []ProjectMembership{
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "write"},
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "read"},
				},
			},
			expected: []string{"editor", "viewer"},
		},
		{
			name: "case insensitive role matching",
			summary: RoleSummary{
				Organizations: nil,
				Projects: []ProjectMembership{
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "WRITE"},
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "Read"},
				},
			},
			expected: []string{"editor", "viewer"},
		},
		{
			name: "owner takes precedence over project roles",
			summary: RoleSummary{
				Organizations: []OrganizationMembership{
					{OrganizationID: uuid.New(), IsAdmin: true},
				},
				Projects: []ProjectMembership{
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "write"},
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "read"},
				},
			},
			expected: []string{"owner", "editor", "viewer"},
		},
		{
			name: "no organizations or projects gives pending",
			summary: RoleSummary{
				Organizations: nil,
				Projects:      nil,
			},
			expected: []string{"pending"},
		},
		{
			name: "empty organizations slice gives pending",
			summary: RoleSummary{
				Organizations: []OrganizationMembership{},
				Projects:      []ProjectMembership{},
			},
			expected: []string{"pending"},
		},
		{
			name: "unknown project role ignored",
			summary: RoleSummary{
				Organizations: nil,
				Projects: []ProjectMembership{
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "unknown"},
				},
			},
			expected: []string{"pending"},
		},
		{
			name: "deduplicate duplicate write roles",
			summary: RoleSummary{
				Organizations: nil,
				Projects: []ProjectMembership{
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "write"},
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "write"},
					{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "write"},
				},
			},
			expected: []string{"editor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roles := tt.summary.AggregatedRoles()

			if len(roles) != len(tt.expected) {
				t.Errorf("length mismatch: got %d roles %v, want %d roles %v",
					len(roles), roles, len(tt.expected), tt.expected)
				return
			}

			for i, expected := range tt.expected {
				if i >= len(roles) {
					t.Errorf("missing role at index %d: want %q", i, expected)
					continue
				}
				if roles[i] != expected {
					t.Errorf("role mismatch at index %d: got %q, want %q", i, roles[i], expected)
				}
			}
		})
	}
}

func TestRoleSummary_AggregatedRoles_Order(t *testing.T) {
	// Test that the order is consistent: owner, editor, viewer, pending
	summary := RoleSummary{
		Organizations: []OrganizationMembership{
			{OrganizationID: uuid.New(), IsAdmin: true},
		},
		Projects: []ProjectMembership{
			{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "read"},
			{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "write"},
		},
	}

	roles := summary.AggregatedRoles()

	expected := []string{"owner", "editor", "viewer"}
	if len(roles) != len(expected) {
		t.Fatalf("length mismatch: got %v, want %v", roles, expected)
	}

	for i, want := range expected {
		if roles[i] != want {
			t.Errorf("order mismatch at index %d: got %q, want %q", i, roles[i], want)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "user@example.com",
			expected: "user@example.com",
		},
		{
			input:    "USER@EXAMPLE.COM",
			expected: "user@example.com",
		},
		{
			input:    "  user@example.com  ",
			expected: "user@example.com",
		},
		{
			input:    "  USER@EXAMPLE.COM  ",
			expected: "user@example.com",
		},
		{
			input:    "User.Name+Tag@Example.Com",
			expected: "user.name+tag@example.com",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "   ",
			expected: "",
		},
		{
			input:    "\t\nuser@example.com\n\t",
			expected: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeEmail(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeEmail(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestOrganizationMembership(t *testing.T) {
	// Test that OrganizationMembership struct works as expected
	orgID := uuid.New()
	membership := OrganizationMembership{
		OrganizationID: orgID,
		IsAdmin:        true,
	}

	if membership.OrganizationID != orgID {
		t.Error("organization ID mismatch")
	}
	if !membership.IsAdmin {
		t.Error("expected admin to be true")
	}
}

func TestProjectMembership(t *testing.T) {
	// Test that ProjectMembership struct works as expected
	projectID := uuid.New()
	orgID := uuid.New()
	membership := ProjectMembership{
		ProjectID:      projectID,
		OrganizationID: orgID,
		Role:           "write",
	}

	if membership.ProjectID != projectID {
		t.Error("project ID mismatch")
	}
	if membership.OrganizationID != orgID {
		t.Error("organization ID mismatch")
	}
	if membership.Role != "write" {
		t.Errorf("role mismatch: got %q, want %q", membership.Role, "write")
	}
}

func TestRoleSummary_EmptyInitialization(t *testing.T) {
	// Test that a zero-value RoleSummary gives pending role
	summary := RoleSummary{}
	roles := summary.AggregatedRoles()

	if len(roles) != 1 {
		t.Fatalf("expected 1 role, got %d: %v", len(roles), roles)
	}
	if roles[0] != "pending" {
		t.Errorf("expected pending role, got %q", roles[0])
	}
}

func TestRoleSummary_NilSlices(t *testing.T) {
	// Test that nil slices are handled correctly
	summary := RoleSummary{
		Organizations: nil,
		Projects:      nil,
	}
	roles := summary.AggregatedRoles()

	if len(roles) != 1 {
		t.Fatalf("expected 1 role, got %d: %v", len(roles), roles)
	}
	if roles[0] != "pending" {
		t.Errorf("expected pending role, got %q", roles[0])
	}
}

func TestOrganizationRegistration_EmailFieldExists(t *testing.T) {
	// Verify that the OrganizationRegistration struct has the email field
	// This test ensures the email field addition is preserved
	reg := OrganizationRegistration{
		ID:      uuid.New(),
		UserID:  uuid.New(),
		Email:   "test@example.com",
		OrgName: "test-org",
	}

	if reg.Email != "test@example.com" {
		t.Errorf("email field mismatch: got %q, want %q", reg.Email, "test@example.com")
	}

	// Verify email can be set and retrieved
	reg.Email = "updated@example.com"
	if reg.Email != "updated@example.com" {
		t.Errorf("email update failed: got %q, want %q", reg.Email, "updated@example.com")
	}
}

func TestOrganizationInvite_EmailFieldExists(t *testing.T) {
	// Verify that the OrganizationInvite struct has the email field
	invite := OrganizationInvite{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          "invite@example.com",
		Role:           "member",
	}

	if invite.Email != "invite@example.com" {
		t.Errorf("email field mismatch: got %q, want %q", invite.Email, "invite@example.com")
	}

	// Verify email can be set and retrieved
	invite.Email = "newemail@example.com"
	if invite.Email != "newemail@example.com" {
		t.Errorf("email update failed: got %q, want %q", invite.Email, "newemail@example.com")
	}
}

func TestRoleSummary_CaseInsensitiveRoles(t *testing.T) {
	// Ensure that role matching is case-insensitive
	summary := RoleSummary{
		Organizations: nil,
		Projects: []ProjectMembership{
			{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "WRITE"},
			{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "wRiTe"},
			{ProjectID: uuid.New(), OrganizationID: uuid.New(), Role: "ReAd"},
		},
	}

	roles := summary.AggregatedRoles()

	// Should deduplicate to just "editor" and "viewer"
	expected := []string{"editor", "viewer"}
	if len(roles) != len(expected) {
		t.Fatalf("length mismatch: got %v, want %v", roles, expected)
	}

	for i, want := range expected {
		if roles[i] != want {
			t.Errorf("role mismatch at index %d: got %q, want %q", i, roles[i], want)
		}
	}
}

func TestProjectInput_Validation(t *testing.T) {
	// Test ProjectInput struct
	input := ProjectInput{
		Name:          "test-project",
		RepoURL:       "https://github.com/test/repo",
		DefaultBranch: "main",
		PathScope:     []string{"src/**", "tests/**"},
	}

	if input.Name == "" {
		t.Error("name should not be empty")
	}
	if input.RepoURL == "" {
		t.Error("repoURL should not be empty")
	}
	if input.DefaultBranch == "" {
		t.Error("defaultBranch should not be empty")
	}
	if len(input.PathScope) != 2 {
		t.Errorf("expected 2 path scopes, got %d", len(input.PathScope))
	}
}

func TestCreateOrgInput_Validation(t *testing.T) {
	// Test CreateOrgInput struct
	input := CreateOrgInput{
		Name: "test-org",
		Slug: "test-org",
		Project: ProjectInput{
			Name:          "initial-project",
			RepoURL:       "https://github.com/test/repo",
			DefaultBranch: "main",
			PathScope:     []string{"**"},
		},
	}

	if input.Name == "" {
		t.Error("name should not be empty")
	}
	if input.Slug == "" {
		t.Error("slug should not be empty")
	}
	if input.Project.Name == "" {
		t.Error("project name should not be empty")
	}
}

func TestErrors_Constants(t *testing.T) {
	// Test that error constants are defined
	if ErrRefreshTokenNotFound == nil {
		t.Error("ErrRefreshTokenNotFound should be defined")
	}
	if ErrOrganizationSlugUsed == nil {
		t.Error("ErrOrganizationSlugUsed should be defined")
	}

	// Test error messages
	if !strings.Contains(ErrRefreshTokenNotFound.Error(), "refresh token") {
		t.Errorf("unexpected error message: %v", ErrRefreshTokenNotFound)
	}
	if !strings.Contains(ErrOrganizationSlugUsed.Error(), "slug") {
		t.Errorf("unexpected error message: %v", ErrOrganizationSlugUsed)
	}
}
