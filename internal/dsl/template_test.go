package dsl

import (
	"testing"
)

func TestEscapedHandlebars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		runtime  map[string]interface{}
		expected string
	}{
		{
			name:     "escaped handlebars with config var",
			input:    `API endpoint: {{ .vars.base_url }}, literal handlebars: \{{ not_a_variable }}`,
			vars:     map[string]interface{}{"base_url": "https://api.example.com"},
			runtime:  map[string]interface{}{},
			expected: `API endpoint: https://api.example.com, literal handlebars: {{ not_a_variable }}`,
		},
		{
			name:     "escaped handlebars with runtime var",
			input:    `User ID: {{ user_id }}, literal template: \{{ .vars.secret }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"user_id": "12345"},
			expected: `User ID: 12345, literal template: {{ .vars.secret }}`,
		},
		{
			name:     "multiple escaped handlebars",
			input:    `\{{ first }} and \{{ second }} are literals, but {{ .vars.real }} is processed`,
			vars:     map[string]interface{}{"real": "actual_value"},
			runtime:  map[string]interface{}{},
			expected: `{{ first }} and {{ second }} are literals, but actual_value is processed`,
		},
		{
			name:     "config variables only with escaped",
			input:    `Config: {{ .vars.env }}, escaped: \{{ .vars.secret }}`,
			vars:     map[string]interface{}{"env": "production"},
			runtime:  map[string]interface{}{},
			expected: `Config: production, escaped: {{ .vars.secret }}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := TemplateContext{
				Vars:    tt.vars,
				Runtime: tt.runtime,
			}
			
			result, err := ProcessTemplate(tt.input, context)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			
			if result != tt.expected {
				t.Errorf("ProcessTemplate result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}

func TestEscapedHandlebarsConfigOnly(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		expected string
	}{
		{
			name:     "config only with escaped",
			input:    `Config: {{ .vars.env }}, escaped: \{{ .vars.secret }}, runtime: {{ user_id }}`,
			vars:     map[string]interface{}{"env": "production"},
			expected: `Config: production, escaped: {_{ .vars.secret }_}, runtime: {{ user_id }}`,
		},
		{
			name:     "only escaped handlebars",
			input:    `All escaped: \{{ .vars.secret }} and \{{ user_id }}`,
			vars:     map[string]interface{}{},
			expected: `All escaped: {_{ .vars.secret }_} and {_{ user_id }_}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProcessConfigVariablesOnly(tt.input, tt.vars)
			if err != nil {
				t.Fatalf("ProcessConfigVariablesOnly failed: %v", err)
			}
			
			if result != tt.expected {
				t.Errorf("ProcessConfigVariablesOnly result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}

func TestMultiLevelEscapedHandlebars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		runtime  map[string]interface{}
		expected string
	}{
		{
			name:     "single escape (1 backslash)",
			input:    `Escaped: \{{ var }} and processed: {{ .vars.value }}`,
			vars:     map[string]interface{}{"value": "processed_value"},
			runtime:  map[string]interface{}{},
			expected: `Escaped: {{ var }} and processed: processed_value`,
		},
		{
			name:     "double escape (2 backslashes)",
			input:    `Literal backslash: \\{{ var }} and processed: {{ .vars.value }}`,
			vars:     map[string]interface{}{"value": "processed_value"},
			runtime:  map[string]interface{}{"var": "runtime_value"},
			expected: `Literal backslash: \runtime_value and processed: processed_value`,
		},
		{
			name:     "triple escape (3 backslashes)",
			input:    `Triple: \\\{{ var }} and processed: {{ .vars.value }}`,
			vars:     map[string]interface{}{"value": "processed_value"},
			runtime:  map[string]interface{}{},
			expected: `Triple: \{{ var }} and processed: processed_value`,
		},
		{
			name:     "quadruple escape (4 backslashes)",
			input:    `Quad: \\\\{{ var }} and processed: {{ .vars.value }}`,
			vars:     map[string]interface{}{"value": "processed_value"},
			runtime:  map[string]interface{}{"var": "runtime_value"},
			expected: `Quad: \\runtime_value and processed: processed_value`,
		},
		{
			name:     "five escapes (5 backslashes)",
			input:    `Five: \\\\\{{ var }} and processed: {{ .vars.value }}`,
			vars:     map[string]interface{}{"value": "processed_value"},
			runtime:  map[string]interface{}{},
			expected: `Five: \\{{ var }} and processed: processed_value`,
		},
		{
			name:     "mixed levels with runtime",
			input:    `One: \{{ var }}, Two: \\{{ var }}, Three: \\\{{ var }}, Runtime: {{ user_id }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"user_id": "12345", "var": "test_value"},
			expected: `One: {{ var }}, Two: \test_value, Three: \{{ var }}, Runtime: 12345`,
		},
		{
			name:     "complex mixed scenario",
			input:    `API: {{ .vars.api_url }}, Escaped: \{{ template }}, Double: \\{{ literal }}, User: {{ user }}`,
			vars:     map[string]interface{}{"api_url": "https://api.com"},
			runtime:  map[string]interface{}{"user": "alice", "literal": "test_literal"},
			expected: `API: https://api.com, Escaped: {{ template }}, Double: \test_literal, User: alice`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := TemplateContext{
				Vars:    tt.vars,
				Runtime: tt.runtime,
			}
			
			result, err := ProcessTemplate(tt.input, context)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			
			if result != tt.expected {
				t.Errorf("ProcessTemplate result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}

func TestMultiLevelEscapedHandlebarsConfigOnly(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		expected string
	}{
		{
			name:     "config only with multiple escape levels",
			input:    `Config: {{ .vars.env }}, Single: \{{ .vars.secret }}, Double: \\{{ .vars.literal }}, Triple: \\\{{ .vars.triple }}, Runtime: {{ user_id }}`,
			vars:     map[string]interface{}{"env": "production"},
			expected: `Config: production, Single: {_{ .vars.secret }_}, Double: \{{ .vars.literal }}, Triple: \{_{ .vars.triple }_}, Runtime: {{ user_id }}`,
		},
		{
			name:     "various escape levels",
			input:    `One: \{{ .vars.a }}, Two: \\{{ .vars.b }}, Three: \\\{{ .vars.c }}, Four: \\\\{{ .vars.d }}`,
			vars:     map[string]interface{}{"b": "value_b", "d": "value_d"},
			expected: `One: {_{ .vars.a }_}, Two: \value_b, Three: \{_{ .vars.c }_}, Four: \\value_d`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProcessConfigVariablesOnly(tt.input, tt.vars)
			if err != nil {
				t.Fatalf("ProcessConfigVariablesOnly failed: %v", err)
			}
			
			if result != tt.expected {
				t.Errorf("ProcessConfigVariablesOnly result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}