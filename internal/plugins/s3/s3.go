package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func init() {
	plugins.RegisterPlugin(&S3Plugin{})
}

func (p *S3Plugin) GetType() string {
	return "s3"
}

// getStateKeys returns a sorted list of keys from the state map
func getStateKeys(state map[string]string) []string {
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// extractMissingVars extracts variable names from template execution errors
func extractMissingVars(err error) []string {
	// For now, just return the error string
	// TODO: Parse Go template errors more intelligently
	return []string{err.Error()}
}


// replaceVariables replaces {{ variable }} patterns in the input string with values from the state
// Now uses DSL template functions to properly handle escaped handlebars
func replaceVariables(input string, state map[string]string) (string, error) {
	// Convert state to interface{} map for DSL functions
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}

	// Create template context with only runtime variables (config vars already processed by CLI)
	context := dsl.TemplateContext{
		Runtime: runtime,
	}

	// Use DSL template processing which handles escaped handlebars
	result, err := dsl.ProcessTemplate(input, context)
	if err != nil {
		availableVars := getStateKeys(state)
		return "", fmt.Errorf("undefined variables: %v. Available runtime variables: %v", extractMissingVars(err), availableVars)
	}

	return result, nil
}

func getRequiredString(config map[string]interface{}, key string) (string, error) {
    val, ok := config[key]
    if !ok {
        return "", fmt.Errorf("%s is required", key)
    }
    strVal, ok := val.(string)
    if !ok || strVal == "" {
        return "", fmt.Errorf("%s must be a non-empty string", key)
    }
    return strVal, nil
}


func (p *S3Plugin) Activity(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	cfg, ok := params["config"].(map[string]interface{})
	if !ok{
		return nil, fmt.Errorf("invalid config format: got type %T", params["config"])
	}

	state := make(map[string]string)
	for k, v := range params["state"].(map[string]interface{}) {
		if strVal, ok := v.(string); ok {
			state[k] = strVal
		}
	}
	
	// Replace variables in config with values from state
	for key, value := range cfg {
		if strValue, ok := value.(string); ok {
			if newValue, err := replaceVariables(strValue, state); err == nil {
				cfg[key] = newValue
			}
		}
	}	

	operation, ok := cfg["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation is required and must be a string")
	}

	awsAccessKey, err := getRequiredString(cfg, "aws_access_key")
	if err != nil {
		return nil, err
	}
	awsSecretKey, err := getRequiredString(cfg, "aws_secret_key")
	if err != nil {
		return nil, err
	}
	awsRegion, err := getRequiredString(cfg, "aws_region")
	if err != nil {
		return nil, err
	}
	_, err = getRequiredString(cfg, "bucket")
	if err != nil {
		return nil, err
	}

	// Setup AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(awsRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsAccessKey, awsSecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	switch operation {
		case "PUT":
			return handlePut(ctx, cfg, params, client, state)
		case "GET":
			return handleGet(ctx, cfg, client)
		case "DELETE":
			return handleDelete(ctx, cfg, client)
		default:
			return nil, fmt.Errorf("operation %s is not supported", operation)
	}
}

//  also pass aws instance to all the function
func handlePut(ctx context.Context, cfg map[string]interface{}, params map[string]interface{}, client *s3.Client, state map[string]string) (interface{}, error) {
	// get file-path from which we have to upload file
	filePath, err := getRequiredString(cfg, "file_path")
	if err != nil {
		return nil, err
	}

	// get file from a path provided and check wether the file exists or not
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	bucket := cfg["bucket"].(string)

	keyPrefix := ""
	if val, ok := cfg["key_prefix"]; ok {
		if folderStr, ok := val.(string); ok && folderStr != "" {
			keyPrefix = folderStr
		}
	}
	
	fileName := filepath.Base(filePath)
	trimmedFileName := strings.TrimSpace(fileName)
	cleanedFileName := strings.ReplaceAll(trimmedFileName, " ", "_")
	
	var key string
	if keyPrefix != "" {
		key = filepath.Join(keyPrefix, cleanedFileName)
	} else {
		key = cleanedFileName
	}

	awsRegion := cfg["aws_region"]
	output, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(file),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	etag := ""
	if output.ETag != nil {
		etag = strings.Trim(*output.ETag, `"`)
	}
	versionID := ""
	if output.VersionId != nil {
		versionID = *output.VersionId
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, awsRegion, key)

	result := map[string]interface{}{
		"status": 200,
		"key": key,
		"url": url,
		"s3_file_name": cleanedFileName,
		"etag":        etag,
		"version_id":  versionID, // empty string if versioning is disabled
	}

	// get assertions from params
	if err := processAssertions(params, result, state); err != nil {
		return nil, err
	}
	
	saved := make(map[string]string)

	if err := processSaves(params, result, saved); err != nil {
		return nil, err
	}
	
	return &ActivityResponse{
		Response: &S3Response{
			Result: result,
		},
		Saved: saved,
	}, nil
}

func handleGet(ctx context.Context, cfg map[string]interface{}, client *s3.Client) (interface{}, error) {
	fileName, err := getRequiredString(cfg, "s3_file_name")
	if err != nil {
		return nil, err
	}

	bucket := cfg["bucket"].(string)

	keyPrefix := ""
	if val, ok := cfg["key_prefix"]; ok {
		if folderStr, ok := val.(string); ok && folderStr != "" {
			keyPrefix = folderStr
		}
	}
	
	trimmedFileName := strings.TrimSpace(fileName)
	cleanedFileName := strings.ReplaceAll(trimmedFileName, " ", "_")

	var key string
	if keyPrefix != "" {
		key = filepath.Join(keyPrefix, cleanedFileName)
	} else {
		key = cleanedFileName
	}

	// Download the file from S3
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer resp.Body.Close()

	if file_path, ok := cfg["file_path"].(string); ok && file_path != "" {
		// Save the file locally if file_path is provided
		outFile, err := os.Create(file_path)
		if err != nil {
			return nil, fmt.Errorf("failed to create local file %s: %w", file_path, err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, resp.Body); err != nil {
			return nil, fmt.Errorf("failed to save file %s: %w", file_path, err)
		}
		
	}

	etag := ""
	if resp.ETag != nil {
		etag = strings.Trim(*resp.ETag, `"`)
	}
	versionID := ""
	if resp.VersionId != nil {
		versionID = *resp.VersionId
	}

	result := map[string]interface{}{
		"status": 	   200,
		"etag":        etag,
		"version_id":  versionID, // empty string if versioning is disabled
	}

	return &ActivityResponse{
		Response: &S3Response{
			Result: result,
		},
	}, nil
}

func handleDelete(ctx context.Context, cfg map[string]interface{}, client *s3.Client) (interface{}, error) {
	fileName, err := getRequiredString(cfg, "s3_file_name")
	if err != nil {
		return nil, err
	}
	bucket := cfg["bucket"].(string)
	
	keyPrefix := ""
	if val, ok := cfg["key_prefix"]; ok {
		if folderStr, ok := val.(string); ok && folderStr != "" {
			keyPrefix = folderStr
		}
	}

	trimmedFileName := strings.TrimSpace(fileName)
	cleanedFileName := strings.ReplaceAll(trimmedFileName, " ", "_")

	var key string
	if keyPrefix != "" {
		key = filepath.Join(keyPrefix, cleanedFileName)
	} else {
		key = cleanedFileName
	}

	// Delete the file from S3
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete from S3: %w", err)
	}
	result := map[string]interface{}{
		"status": 204,
	}
	return &ActivityResponse{
		Response: &S3Response{
			Result: result,
		},
	}, nil
}

func processAssertions(params map[string]interface{}, result map[string]interface{}, state map[string]string) error {
	assertions, ok := params["assertions"].([]interface{})
	if !ok {
		return nil
	}

	for i, a := range assertions {
		assertion, ok := a.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid assertion format at index %d", i)
		}

		assertType := assertion["type"].(string)
		if assertType != "json_path" {
			return fmt.Errorf("unsupported assertion type: %s", assertType)
		}

		path, _ := assertion["path"].(string)
		expected := assertion["expected"]

		actual, err := getByJsonPath(result, path)
		if err != nil {
			return fmt.Errorf("assertion failed at %s: %v", path, err)
		}

		actualStr := fmt.Sprintf("%v", actual)
		expectedStr := fmt.Sprintf("%v", expected)

		newActualValue, err := replaceVariables(actualStr, state)
		if err != nil {
			return fmt.Errorf("failed to replace variables in assertion at %s: %v", path, err)
		}
		if newActualValue != expectedStr {
			return fmt.Errorf("assertion failed at %s: expected %v, got %v", path, expectedStr, actualStr)
		}
	}
	return nil
}

func processSaves(params map[string]interface{}, result map[string]interface{}, saved map[string]string) error {
	saves, ok := params["save"].([]interface{})
	if !ok {
		return nil
	}

	for _, s := range saves {
		save, ok := s.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid save format: %v", s)
		}

		jsonPath, _ := save["json_path"].(string)
		varName, _ := save["as"].(string)

		value, err := getByJsonPath(result, jsonPath)
		if err != nil {
			return fmt.Errorf("failed to get value by json path %s: %v", jsonPath, err)
		}

		saved[varName] = fmt.Sprintf("%v", value)
	}
	return nil
}

func getByJsonPath(data map[string]interface{}, path string) (interface{}, error) {
	path = strings.TrimPrefix(path, ".")
	parts := strings.Split(path, ".")

	var current interface{} = data

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil, fmt.Errorf("path %s not found", path)
		}
	}
	return current, nil
}
