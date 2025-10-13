package sql

import (
	"reflect"
	"testing"
)

func TestParseSQLFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "Simple queries",
			content:  "SELECT 1; SELECT 2;",
			expected: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:     "Semicolon in string literal",
			content:  "SELECT 'hello; world' as test; SELECT 'another; test';",
			expected: []string{"SELECT 'hello; world' as test", "SELECT 'another; test'"},
		},
		{
			name:     "Escaped quotes in string",
			content:  "SELECT 'It''s a test; with semicolon' as test; SELECT 2;",
			expected: []string{"SELECT 'It''s a test; with semicolon' as test", "SELECT 2"},
		},
		{
			name:     "Line comments with semicolons",
			content:  "SELECT 1; -- comment with ; semicolon\nSELECT 2;",
			expected: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:     "Block comments with semicolons",
			content:  "SELECT 1; /* comment with ; semicolon */ SELECT 2;",
			expected: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:     "Multi-line block comment",
			content:  "SELECT 1; /* multi\nline comment\nwith ; semicolon */ SELECT 2;",
			expected: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:     "Double-quoted identifiers",
			content:  "SELECT \"column;name\" FROM table; SELECT 2;",
			expected: []string{"SELECT \"column;name\" FROM table", "SELECT 2"},
		},
		{
			name: "Complex mixed case",
			content: `-- Comment with ; semicolon
SELECT 'string; with semicolon' as test, 
       "identifier; with semicolon" as col
FROM users; /* block comment ; here */
INSERT INTO test VALUES ('data; here');`,
			expected: []string{
				"SELECT 'string; with semicolon' as test, \n       \"identifier; with semicolon\" as col\nFROM users",
				"INSERT INTO test VALUES ('data; here')",
			},
		},
		{
			name:     "Query without trailing semicolon",
			content:  "SELECT 1; SELECT 2",
			expected: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:     "Empty queries filtered out",
			content:  "SELECT 1;; ; SELECT 2;",
			expected: []string{"SELECT 1", "SELECT 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSQLFile(tt.content)
			if err != nil {
				t.Fatalf("parseSQLFile() error = %v", err)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseSQLFile() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseSQLFileEdgeCases(t *testing.T) {
	// Test the specific edge cases file content
	content := `-- Test file with edge cases for SQL parsing
-- This file contains semicolons in string literals and comments

/* Block comment with ; semicolon inside */
SELECT 'This string contains a ; semicolon' as test_string, 
       'Another string with '' escaped quote ; and semicolon' as test_string2;

-- Line comment with ; semicolon
INSERT INTO users (name, email) VALUES ('Test; User', 'test;user@example.com');

/* Multi-line 
   block comment with 
   ; semicolon inside 
   continues here */
UPDATE users SET name = 'Updated; Name' WHERE email = 'test;user@example.com';

-- Final query without trailing semicolon
DELETE FROM users WHERE email = 'test;user@example.com'`

	expected := []string{
		"SELECT 'This string contains a ; semicolon' as test_string, \n       'Another string with '' escaped quote ; and semicolon' as test_string2",
		"INSERT INTO users (name, email) VALUES ('Test; User', 'test;user@example.com')",
		"UPDATE users SET name = 'Updated; Name' WHERE email = 'test;user@example.com'",
		"DELETE FROM users WHERE email = 'test;user@example.com'",
	}

	result, err := parseSQLFile(content)
	if err != nil {
		t.Fatalf("parseSQLFile() error = %v", err)
	}

	if len(result) != len(expected) {
		t.Fatalf("parseSQLFile() returned %d queries, expected %d", len(result), len(expected))
	}

	for i, query := range result {
		if query != expected[i] {
			t.Errorf("Query %d:\ngot:      %q\nexpected: %q", i, query, expected[i])
		}
	}
}
