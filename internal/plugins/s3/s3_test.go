package s3

import (
	"context"
	// "encoding/json"
	// "errors"
	"os"
	"strings"
	"testing"
)

func TestS3Plugin_GetType(t *testing.T) {
	plugin := &S3Plugin{}
	if plugin.GetType() != "s3" {
		t.Errorf("expected plugin type 's3', got '%s'", plugin.GetType())
	}
}

func TestReplaceVariables(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		state    map[string]string
		expected string
		wantErr  bool
	}{
		{"no vars", "hello", nil, "hello", false},
		{"basic replace", "{{ name }}", map[string]string{"name": "world"}, "world", false},
		{"missing var", "{{ missing }}", map[string]string{"other": "value"}, "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := replaceVariables(tc.input, tc.state)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error mismatch: got %v, wantErr=%v", err, tc.wantErr)
			}
			if res != tc.expected && !tc.wantErr {
				t.Errorf("unexpected result: got %s, want %s", res, tc.expected)
			}
		})
	}
}

func TestGetByJsonPath(t *testing.T) {
	input := map[string]interface{}{
		"s3_file_name": "file.txt",
		"status":       "success",
	}

	res, err := getByJsonPath(input, ".s3_file_name")
	if err != nil || res != "file.txt" {
		t.Errorf("unexpected result: got %v (err: %v)", res, err)
	}
}

func TestProcessAssertions(t *testing.T) {
	result := map[string]interface{}{
		"status": "success",
		"url":    "https://example.com/uploads/file.txt",
	}

	params := map[string]interface{}{
		"assertions": []interface{}{
			map[string]interface{}{
				"type":     "json_path",
				"path":     ".status",
				"expected": "success",
			},
		},
	}

	err := processAssertions(params, result, nil)
	if err != nil {
		t.Errorf("expected assertions to pass, got error: %v", err)
	}
}

func TestProcessSaves(t *testing.T) {
	result := map[string]interface{}{
		"s3_file_name": "file1.txt",
	}

	params := map[string]interface{}{
		"save": []interface{}{
			map[string]interface{}{
				"json_path": ".s3_file_name",
				"as":        "saved_name",
			},
		},
	}

	saved := make(map[string]string)
	err := processSaves(params, result, saved)
	if err != nil {
		t.Errorf("processSaves failed: %v", err)
	}
	if saved["saved_name"] != "file1.txt" {
		t.Errorf("unexpected saved value: got %s", saved["saved_name"])
	}
}

func TestS3Plugin_Activity_InvalidConfig(t *testing.T) {
	plugin := &S3Plugin{}
	ctx := context.Background()
	_, err := plugin.Activity(ctx, map[string]interface{}{
		"config": "not_a_map",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid config format") {
		t.Errorf("expected invalid config error, got %v", err)
	}
}

func TestS3Plugin_Activity_UploadDownload(t *testing.T) {
	// Skipping actual upload/download test due to S3 dependencies
	t.Skip("This test requires valid AWS credentials and S3 access")

	plugin := &S3Plugin{}
	ctx := context.Background()
	params := map[string]interface{}{
		"config": map[string]interface{}{
			"operation":      "PUT",
			"aws_access_key": os.Getenv("AWS_ACCESS_KEY_ID"),
			"aws_secret_key": os.Getenv("AWS_SECRET_ACCESS_KEY"),
			"aws_region":     "ap-south-1",
			"bucket":         "your-bucket-name",
			"file_path":      "testdata/testfile.txt",
			"upload_path":    "test/testfile.txt",
		},
	}

	_, err := plugin.Activity(ctx, params)
	if err != nil {
		t.Fatalf("Activity failed: %v", err)
	}
}

func TestGetStateKeys(t *testing.T) {
	input := map[string]string{"z": "1", "a": "2", "m": "3"}
	keys := getStateKeys(input)
	if len(keys) != 3 || keys[0] != "a" || keys[2] != "z" {
		t.Errorf("unexpected sorted keys: %v", keys)
	}
}
