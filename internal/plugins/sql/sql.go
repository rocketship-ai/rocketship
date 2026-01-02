package sql

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.temporal.io/sdk/activity"

	// Database drivers
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&SQLPlugin{})
}

// GetType returns the plugin type identifier
func (sp *SQLPlugin) GetType() string {
	return "sql"
}

// Activity executes SQL operations and returns results
func (sp *SQLPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)

	// Parse configuration from parameters
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	config := &SQLConfig{}
	if err := parseConfig(configData, config); err != nil {
		return nil, fmt.Errorf("failed to parse SQL config: %w", err)
	}

	// Validate required fields
	if config.Driver == "" {
		return nil, fmt.Errorf("driver is required")
	}
	if config.DSN == "" {
		return nil, fmt.Errorf("dsn is required")
	}
	if len(config.Commands) == 0 && config.File == "" {
		return nil, fmt.Errorf("either commands or file must be specified")
	}

	logger.Info("Executing SQL plugin", "driver", config.Driver, "queries", len(config.Commands))

	// Extract state and env from parameters
	state, _ := p["state"].(map[string]interface{})

	// Extract env secrets from params (for {{ .env.* }} template resolution)
	env := make(map[string]string)
	if envData, ok := p["env"].(map[string]interface{}); ok {
		for k, v := range envData {
			if strVal, ok := v.(string); ok {
				env[k] = strVal
			}
		}
	} else if envData, ok := p["env"].(map[string]string); ok {
		env = envData
	}

	// Apply variable replacement to DSN and commands
	if err := applyVariableReplacement(config, state, env); err != nil {
		return nil, fmt.Errorf("variable replacement failed: %w", err)
	}

	// Get queries to execute
	queries, err := getQueries(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get queries: %w", err)
	}

	// Execute SQL operations
	response, err := executeQueries(ctx, config, queries)
	if err != nil {
		return nil, fmt.Errorf("SQL execution failed: %w", err)
	}

	// Process assertions
	if assertions, ok := p["assertions"].([]interface{}); ok {
		vars, _ := p["vars"].(map[string]interface{})
		if err := applyVariableReplacementToAssertions(assertions, state, env, vars); err != nil {
			return nil, fmt.Errorf("assertion variable replacement failed: %w", err)
		}
		if err := processAssertions(response, assertions); err != nil {
			return nil, fmt.Errorf("assertion failed: %w", err)
		}
	}

	// Process save configuration
	savedValues := make(map[string]string)
	if saveConfig, ok := p["save"].([]interface{}); ok {
		savedValues = processSaveConfig(response, saveConfig)
	}

	logger.Info("SQL execution completed", "queries", response.Stats.TotalQueries, "saved_vars", len(savedValues))

	return &ActivityResponse{
		Response: response,
		Saved:    savedValues,
	}, nil
}

// parseConfig converts map[string]interface{} to SQLConfig
func parseConfig(configData map[string]interface{}, config *SQLConfig) error {
	if driver, ok := configData["driver"].(string); ok {
		config.Driver = driver
	}
	if dsn, ok := configData["dsn"].(string); ok {
		config.DSN = dsn
	}
	if file, ok := configData["file"].(string); ok {
		config.File = file
	}
	if timeout, ok := configData["timeout"].(string); ok {
		config.Timeout = timeout
	}

	// Parse commands array
	if commandsInterface, ok := configData["commands"]; ok {
		if commandsSlice, ok := commandsInterface.([]interface{}); ok {
			for _, cmd := range commandsSlice {
				if cmdStr, ok := cmd.(string); ok {
					config.Commands = append(config.Commands, cmdStr)
				}
			}
		}
	}

	return nil
}

// applyVariableReplacement replaces variables in DSN and commands using DSL template processing
func applyVariableReplacement(config *SQLConfig, state map[string]interface{}, env map[string]string) error {
	// Create template context with runtime variables and env secrets
	// (config vars already processed by CLI)
	context := dsl.TemplateContext{
		Runtime: state,
		Env:     env,
	}

	// Process DSN
	if config.DSN != "" {
		processedDSN, err := dsl.ProcessTemplate(config.DSN, context)
		if err != nil {
			return fmt.Errorf("failed to process DSN template: %w", err)
		}
		config.DSN = processedDSN
	}

	// Process commands
	for i, cmd := range config.Commands {
		processedCmd, err := dsl.ProcessTemplate(cmd, context)
		if err != nil {
			return fmt.Errorf("failed to process command template at index %d: %w", i, err)
		}
		config.Commands[i] = processedCmd
	}

	return nil
}

func applyVariableReplacementToAssertions(assertions []interface{}, state map[string]interface{}, env map[string]string, vars map[string]interface{}) error {
	// We intentionally avoid Go template execution for assertions because users may
	// legitimately assert on literal handlebars strings (e.g. "{{ placeholder }}").
	// We do targeted substitution for .vars.*, .env.*, and known runtime vars only,
	// while leaving unknown handlebars untouched.

	envRegex := regexp.MustCompile(`\{\{\s*\.env\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)
	runtimeRegex := regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z0-9_]+)*)\s*\}\}`)

	replaceUnescaped := func(input string, re *regexp.Regexp, replacer func(groups []string, match string) string) string {
		locs := re.FindAllStringSubmatchIndex(input, -1)
		if len(locs) == 0 {
			return input
		}

		var out strings.Builder
		out.Grow(len(input))
		last := 0

		for _, loc := range locs {
			start, end := loc[0], loc[1]
			// If the handlebars are escaped like \{{ ... }}, leave untouched.
			if start > 0 && input[start-1] == '\\' {
				continue
			}

			out.WriteString(input[last:start])

			groups := make([]string, 0, (len(loc)-2)/2)
			for i := 2; i < len(loc); i += 2 {
				si, ei := loc[i], loc[i+1]
				if si == -1 || ei == -1 {
					groups = append(groups, "")
					continue
				}
				groups = append(groups, input[si:ei])
			}

			out.WriteString(replacer(groups, input[start:end]))
			last = end
		}

		out.WriteString(input[last:])
		return out.String()
	}

	for _, assertionInterface := range assertions {
		assertion, ok := assertionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		expected, ok := assertion["expected"].(string)
		if !ok || expected == "" {
			continue
		}

		if strings.Contains(expected, ".vars.") && vars != nil {
			processed, err := dsl.ProcessConfigVariablesOnly(expected, vars)
			if err != nil {
				return fmt.Errorf("failed to process config vars in assertion expected: %w", err)
			}
			expected = processed
		}

		// Replace .env.* with precedence: OS env > env secrets map (DB).
		expected = replaceUnescaped(expected, envRegex, func(groups []string, match string) string {
			if len(groups) < 1 {
				return match
			}
			key := groups[0]
			if value, ok := os.LookupEnv(key); ok {
				return value
			}
			if value, ok := env[key]; ok {
				return value
			}
			return "<no value>"
		})

		// Replace runtime vars only when they exist in the state map.
		expected = replaceUnescaped(expected, runtimeRegex, func(groups []string, match string) string {
			if len(groups) < 1 {
				return match
			}
			key := groups[0]
			// Avoid consuming .env.* and .vars.* which may remain as literals.
			if strings.HasPrefix(key, ".env.") || strings.HasPrefix(key, ".vars.") {
				return match
			}
			if value, ok := state[key]; ok {
				return fmt.Sprintf("%v", value)
			}
			return match
		})

		assertion["expected"] = expected
	}

	return nil
}

// getQueries returns the list of SQL queries to execute
func getQueries(config *SQLConfig) ([]string, error) {
	if len(config.Commands) > 0 {
		return config.Commands, nil
	}

	if config.File != "" {
		content, err := os.ReadFile(config.File)
		if err != nil {
			return nil, fmt.Errorf("failed to read SQL file %s: %w", config.File, err)
		}

		// Parse SQL file content with proper delimiter handling
		queries, err := parseSQLFile(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse SQL file %s: %w", config.File, err)
		}

		return queries, nil
	}

	return nil, fmt.Errorf("no queries specified")
}

// parseSQLFile parses SQL file content and splits queries while respecting string literals and comments
func parseSQLFile(content string) ([]string, error) {
	var queries []string
	var currentQuery strings.Builder

	runes := []rune(content)
	length := len(runes)

	for i := 0; i < length; i++ {
		char := runes[i]

		switch char {
		case '-':
			// Handle SQL line comments (-- comment)
			if i+1 < length && runes[i+1] == '-' {
				// Skip until end of line
				for i < length && runes[i] != '\n' {
					i++
				}
				if i < length {
					currentQuery.WriteRune('\n') // Preserve newline
				}
				continue
			}
			currentQuery.WriteRune(char)

		case '/':
			// Handle SQL block comments (/* comment */)
			if i+1 < length && runes[i+1] == '*' {
				i += 2 // Skip /*
				// Skip until */
				for i+1 < length {
					if runes[i] == '*' && runes[i+1] == '/' {
						i += 2 // Skip */
						break
					}
					i++
				}
				currentQuery.WriteRune(' ') // Replace comment with space
				continue
			}
			currentQuery.WriteRune(char)

		case '\'':
			// Handle single-quoted strings
			currentQuery.WriteRune(char)
			i++
			for i < length {
				char = runes[i]
				currentQuery.WriteRune(char)
				if char == '\'' {
					// Check for escaped quote ('')
					if i+1 < length && runes[i+1] == '\'' {
						i++ // Skip escaped quote
						currentQuery.WriteRune('\'')
					} else {
						break // End of string
					}
				}
				i++
			}

		case '"':
			// Handle double-quoted identifiers
			currentQuery.WriteRune(char)
			i++
			for i < length {
				char = runes[i]
				currentQuery.WriteRune(char)
				if char == '"' {
					// Check for escaped quote ("")
					if i+1 < length && runes[i+1] == '"' {
						i++ // Skip escaped quote
						currentQuery.WriteRune('"')
					} else {
						break // End of identifier
					}
				}
				i++
			}

		case ';':
			// Found statement delimiter outside of strings/comments
			currentQuery.WriteRune(char)
			query := strings.TrimSpace(currentQuery.String())
			if query != "" && query != ";" {
				// Remove trailing semicolon and add clean query
				query = strings.TrimSuffix(query, ";")
				query = strings.TrimSpace(query)
				if query != "" {
					queries = append(queries, query)
				}
			}
			currentQuery.Reset()

		default:
			currentQuery.WriteRune(char)
		}
	}

	// Handle any remaining content as the last query
	lastQuery := strings.TrimSpace(currentQuery.String())
	if lastQuery != "" {
		// Remove trailing semicolon if present
		lastQuery = strings.TrimSuffix(lastQuery, ";")
		lastQuery = strings.TrimSpace(lastQuery)
		if lastQuery != "" {
			queries = append(queries, lastQuery)
		}
	}

	return queries, nil
}

// executeQueries executes SQL queries and returns results
func executeQueries(ctx context.Context, config *SQLConfig, queries []string) (*SQLResponse, error) {
	logger := activity.GetLogger(ctx)
	startTime := time.Now()

	// Establish database connection
	db, err := sqlx.Connect(config.Driver, config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("Failed to close database connection", "error", err)
		}
	}()

	connectionTime := time.Since(startTime)

	// Set connection timeout
	if config.Timeout != "" {
		timeout, err := time.ParseDuration(config.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout format: %w", err)
		}
		db.SetConnMaxLifetime(timeout)
	}

	// Configure connection pool for integration testing
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(5 * time.Minute)

	response := &SQLResponse{
		Queries: make([]QueryResult, 0, len(queries)),
		Stats: ExecutionStats{
			TotalQueries:   len(queries),
			ConnectionTime: connectionTime.String(),
		},
	}

	// Execute each query
	var queryErrors []string
	for i, query := range queries {
		queryResult := executeQuery(ctx, db, query)
		response.Queries = append(response.Queries, queryResult)

		if queryResult.Error == "" {
			response.Stats.SuccessCount++
		} else {
			response.Stats.ErrorCount++
			// Collect detailed error information
			queryErrors = append(queryErrors, fmt.Sprintf("Query %d failed: %s. Query: %s", i+1, queryResult.Error, query))
		}
	}

	response.Stats.TotalDuration = time.Since(startTime).String()

	// If any queries failed, return an error with detailed information
	if len(queryErrors) > 0 {
		return response, fmt.Errorf("SQL execution failed with %d error(s):\n%s", len(queryErrors), strings.Join(queryErrors, "\n"))
	}

	return response, nil
}

// executeQuery executes a single SQL query
func executeQuery(ctx context.Context, db *sqlx.DB, query string) QueryResult {
	startTime := time.Now()

	result := QueryResult{
		Query: query,
		Rows:  make([]map[string]interface{}, 0),
	}

	// Determine if this is a SELECT query or a modification query
	trimmedQuery := strings.TrimSpace(strings.ToUpper(query))
	isSelect := strings.HasPrefix(trimmedQuery, "SELECT") ||
		strings.HasPrefix(trimmedQuery, "WITH") ||
		strings.Contains(trimmedQuery, "RETURNING")

	if isSelect {
		// Execute SELECT query
		rows, err := db.QueryxContext(ctx, query)
		if err != nil {
			result.Error = err.Error()
			result.Duration = time.Since(startTime).String()
			return result
		}
		defer func() {
			if err := rows.Close(); err != nil {
				activity.GetLogger(ctx).Warn("Failed to close query rows", "error", err)
			}
		}()

		// Process rows
		for rows.Next() {
			row := make(map[string]interface{})
			if err := rows.MapScan(row); err != nil {
				result.Error = err.Error()
				break
			}
			result.Rows = append(result.Rows, row)
		}

		if err := rows.Err(); err != nil && result.Error == "" {
			result.Error = err.Error()
		}

	} else {
		// Execute modification query (INSERT, UPDATE, DELETE)
		execResult, err := db.ExecContext(ctx, query)
		if err != nil {
			result.Error = err.Error()
			result.Duration = time.Since(startTime).String()
			return result
		}

		if rowsAffected, err := execResult.RowsAffected(); err == nil {
			result.RowsAffected = rowsAffected
		}
	}

	result.Duration = time.Since(startTime).String()
	return result
}

// processSaveConfig processes save configuration to extract values from results
func processSaveConfig(response *SQLResponse, saveConfig []interface{}) map[string]string {
	savedValues := make(map[string]string)

	for _, saveItem := range saveConfig {
		saveMap, ok := saveItem.(map[string]interface{})
		if !ok {
			continue
		}

		asValue, ok := saveMap["as"].(string)
		if !ok {
			continue
		}

		// Extract value based on sql_result configuration
		if sqlResult, ok := saveMap["sql_result"].(string); ok {
			value := extractSQLValue(response, sqlResult)
			if value != "" {
				savedValues[asValue] = value
			}
		}
	}

	return savedValues
}

// extractSQLValue extracts values from SQL results using path notation
func extractSQLValue(response *SQLResponse, path string) string {
	// Support patterns like:
	// ".queries[0].rows[0].id" - first query, first row, id column
	// ".queries[0].rows_affected" - first query rows affected count
	// ".stats.success_count" - overall success count

	if len(response.Queries) == 0 {
		return ""
	}

	// Simple path parsing for common cases
	if strings.HasPrefix(path, ".queries[0].rows[0].") {
		if len(response.Queries[0].Rows) > 0 {
			columnName := strings.TrimPrefix(path, ".queries[0].rows[0].")
			if value, ok := response.Queries[0].Rows[0][columnName]; ok {
				return fmt.Sprintf("%v", value)
			}
		}
	} else if path == ".queries[0].rows_affected" {
		return fmt.Sprintf("%d", response.Queries[0].RowsAffected)
	} else if path == ".stats.success_count" {
		return fmt.Sprintf("%d", response.Stats.SuccessCount)
	}

	return ""
}

// processAssertions validates SQL response against assertions
func processAssertions(response *SQLResponse, assertions []interface{}) error {
	for _, assertionInterface := range assertions {
		assertion, ok := assertionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		assertionType, ok := assertion["type"].(string)
		if !ok {
			return fmt.Errorf("assertion type is required")
		}

		expected := assertion["expected"]

		switch assertionType {
		case "query_count":
			expectedCount, ok := expected.(float64) // JSON numbers are float64
			if !ok {
				return fmt.Errorf("query_count assertion expected must be a number")
			}
			if float64(response.Stats.TotalQueries) != expectedCount {
				return fmt.Errorf("query count assertion failed: expected %v, got %d", expectedCount, response.Stats.TotalQueries)
			}

		case "success_count":
			expectedCount, ok := expected.(float64)
			if !ok {
				return fmt.Errorf("success_count assertion expected must be a number")
			}
			if float64(response.Stats.SuccessCount) != expectedCount {
				return fmt.Errorf("success count assertion failed: expected %v, got %d", expectedCount, response.Stats.SuccessCount)
			}

		case "row_count":
			queryIndex, ok := assertion["query_index"].(float64)
			if !ok {
				return fmt.Errorf("row_count assertion requires query_index")
			}
			expectedCount, ok := expected.(float64)
			if !ok {
				return fmt.Errorf("row_count assertion expected must be a number")
			}

			queryIdx := int(queryIndex)
			if queryIdx >= len(response.Queries) || queryIdx < 0 {
				return fmt.Errorf("query_index %d is out of range", queryIdx)
			}

			actualCount := len(response.Queries[queryIdx].Rows)
			if float64(actualCount) != expectedCount {
				return fmt.Errorf("row count assertion failed for query %d: expected %v, got %d", queryIdx, expectedCount, actualCount)
			}

		case "column_value":
			queryIndex, ok := assertion["query_index"].(float64)
			if !ok {
				return fmt.Errorf("column_value assertion requires query_index")
			}
			rowIndex, ok := assertion["row_index"].(float64)
			if !ok {
				return fmt.Errorf("column_value assertion requires row_index")
			}
			column, ok := assertion["column"].(string)
			if !ok {
				return fmt.Errorf("column_value assertion requires column")
			}

			queryIdx := int(queryIndex)
			rowIdx := int(rowIndex)

			if queryIdx >= len(response.Queries) || queryIdx < 0 {
				return fmt.Errorf("query_index %d is out of range", queryIdx)
			}
			if rowIdx >= len(response.Queries[queryIdx].Rows) || rowIdx < 0 {
				return fmt.Errorf("row_index %d is out of range for query %d", rowIdx, queryIdx)
			}

			actualValue, exists := response.Queries[queryIdx].Rows[rowIdx][column]
			if !exists {
				return fmt.Errorf("column '%s' not found in query %d, row %d", column, queryIdx, rowIdx)
			}

			// Convert both values to strings for comparison
			expectedStr := fmt.Sprintf("%v", expected)
			actualStr := fmt.Sprintf("%v", actualValue)

			if actualStr != expectedStr {
				return fmt.Errorf("column value assertion failed for query %d, row %d, column '%s': expected %v, got %v", queryIdx, rowIdx, column, expected, actualValue)
			}

		default:
			return fmt.Errorf("unsupported assertion type: %s", assertionType)
		}
	}

	return nil
}
