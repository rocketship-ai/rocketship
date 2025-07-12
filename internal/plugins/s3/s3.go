package s3

import (
	"context"
	"fmt"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"sort"
)

func init() {
	plugins.RegisterPlugin(&S3Plugin{})
}

func (p *S3Plugin) GetType() string {
	return "S3"
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



func (p *S3Plugin) Activity(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	fmt.Println("Name of S3 activity", params["name"])
	fmt.Println("Plugin Name", params["plugin"])
	fmt.Println("Test Config", params["config"])
	fmt.Println("State", params["state"])

	// so i want to populate config with states we have earlier
	state := params["state"]
	// replaceVariables()
	// check type of operation
	config := params["config"]
	if config == nil{
		return nil, fmt.Errorf("config is required for S3 activity")
	}

	configData, ok := config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config type for S3 activity")
	}

	operation, ok := configData["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation is required and must be a string")
	}

	// get aws-access-key-id

	// get aws-secret-key-id

	// get aws-region

	// bucket name, folder-name
	// create url  

	// we also have to check assertions for download, upload, delete

	switch operation {
	case "upload":
		return p.handleUpload(configData)
	case "download":
		return p.handleDownload(configData)
	case "delete":
		return p.handleDelete(configData)
	default:
		return nil, fmt.Errorf("unsupported S3 operation: %s", operation)
	}

	saved := make(map[string]string)


	// return nil, fmt.Errorf("assertion failed: %w", fmt.Errorf("S3 activity not implemented yet"))
}

//  also pass aws instance to all the function
func (p *S3Plugin) handleUpload(config map[string]interface{}) (interface{}, error) {
	// get file-path from which we have to upload file
	filePath := config["file-path"]
	// extract file name after getting
	
	// or i should return saved data, error 
	return nil, nil
}

func (p *S3Plugin) handleDownload(config map[string]interface{}) (interface{}, error) {
	
	return nil, nil
}

func (p *S3Plugin) handleDelete(config map[string]interface{}) (interface{}, error) {
	return nil, nil
}

