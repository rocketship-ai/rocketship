package persistence

import (
	"reflect"
	"testing"
)

func TestParsePluginsFromStepSummaries(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "compact JSON format",
			input:    `[{"name":"x","plugin":"http","step_index":1},{"name":"y","plugin":"log","step_index":2}]`,
			expected: []string{"http", "log"},
		},
		{
			name:     "JSONB spacing format (as returned by postgres)",
			input:    `[{"name": "x", "plugin": "http", "step_index": 1}, {"name": "y", "plugin": "log", "step_index": 2}]`,
			expected: []string{"http", "log"},
		},
		{
			name:     "duplicate plugins returns unique list",
			input:    `[{"plugin":"http"},{"plugin":"log"},{"plugin":"http"},{"plugin":"delay"},{"plugin":"http"}]`,
			expected: []string{"http", "log", "delay"},
		},
		{
			name:     "preserves first-seen order",
			input:    `[{"plugin":"supabase"},{"plugin":"http"},{"plugin":"delay"},{"plugin":"http"}]`,
			expected: []string{"supabase", "http", "delay"},
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: []string{},
		},
		{
			name:     "null string",
			input:    `null`,
			expected: []string{},
		},
		{
			name:     "empty string",
			input:    ``,
			expected: []string{},
		},
		{
			name:     "invalid JSON returns empty",
			input:    `not valid json`,
			expected: []string{},
		},
		{
			name:     "missing plugin field skipped",
			input:    `[{"name":"step1"},{"plugin":"http"},{"name":"step3"}]`,
			expected: []string{"http"},
		},
		{
			name:     "empty plugin value skipped",
			input:    `[{"plugin":""},{"plugin":"http"},{"plugin":""}]`,
			expected: []string{"http"},
		},
		{
			name:     "real-world example with mixed fields",
			input:    `[{"step_index": 0, "plugin": "http", "name": "Create user"}, {"step_index": 1, "plugin": "delay", "name": "Wait"}, {"step_index": 2, "plugin": "http", "name": "Verify user"}]`,
			expected: []string{"http", "delay"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePluginsFromStepSummaries(tt.input)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parsePluginsFromStepSummaries(%q) = %v, want %v",
					tt.input, result, tt.expected)
			}
		})
	}
}


func TestHasAnyPlugin(t *testing.T) {
	tests := []struct {
		name       string
		testPlugins   []string
		reqPlugins []string
		expected   bool
	}{
		{
			name:       "has matching plugin",
			testPlugins:   []string{"http", "delay", "log"},
			reqPlugins: []string{"http"},
			expected:   true,
		},
		{
			name:       "no matching plugin",
			testPlugins:   []string{"http", "delay"},
			reqPlugins: []string{"supabase"},
			expected:   false,
		},
		{
			name:       "case insensitive match",
			testPlugins:   []string{"HTTP", "DELAY"},
			reqPlugins: []string{"http"},
			expected:   true,
		},
		{
			name:       "multiple requested, one matches",
			testPlugins:   []string{"http"},
			reqPlugins: []string{"supabase", "delay", "http"},
			expected:   true,
		},
		{
			name:       "empty test plugins",
			testPlugins:   []string{},
			reqPlugins: []string{"http"},
			expected:   false,
		},
		{
			name:       "empty requested plugins",
			testPlugins:   []string{"http"},
			reqPlugins: []string{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAnyPlugin(tt.testPlugins, tt.reqPlugins)

			if result != tt.expected {
				t.Errorf("hasAnyPlugin(%v, %v) = %v, want %v",
					tt.testPlugins, tt.reqPlugins, result, tt.expected)
			}
		})
	}
}
