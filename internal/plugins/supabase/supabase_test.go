package supabase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSupabasePlugin_GetType(t *testing.T) {
	plugin := &SupabasePlugin{}
	assert.Equal(t, "supabase", plugin.GetType())
}

func TestBuildFilterParam(t *testing.T) {
	tests := []struct {
		name     string
		filter   FilterConfig
		expected string
	}{
		{
			name:     "eq filter",
			filter:   FilterConfig{Column: "status", Operator: "eq", Value: "active"},
			expected: "eq.active",
		},
		{
			name:     "gt filter",
			filter:   FilterConfig{Column: "age", Operator: "gt", Value: 18},
			expected: "gt.18",
		},
		{
			name:     "in filter with array",
			filter:   FilterConfig{Column: "status", Operator: "in", Value: []interface{}{"active", "pending"}},
			expected: "in.(active,pending)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFilterParam(tt.filter)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseContentRange(t *testing.T) {
	tests := []struct {
		name          string
		contentRange  string
		expected      int
	}{
		{
			name:         "normal range",
			contentRange: "0-24/573",
			expected:     573,
		},
		{
			name:         "wildcard range",
			contentRange: "*/100",
			expected:     100,
		},
		{
			name:         "invalid format",
			contentRange: "invalid",
			expected:     -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseContentRange(tt.contentRange)
			assert.Equal(t, tt.expected, result)
		})
	}
}