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