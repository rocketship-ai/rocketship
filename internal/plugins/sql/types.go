package sql

// SQLPlugin represents the SQL plugin configuration
type SQLPlugin struct {
	Name   string    `json:"name" yaml:"name"`
	Plugin string    `json:"plugin" yaml:"plugin"`
	Config SQLConfig `json:"config" yaml:"config"`
}

// SQLConfig defines the configuration for SQL operations
type SQLConfig struct {
	Driver   string   `json:"driver" yaml:"driver"`                         // postgres, mysql, sqlite, sqlserver
	DSN      string   `json:"dsn" yaml:"dsn"`                               // Database connection string
	Commands []string `json:"commands,omitempty" yaml:"commands,omitempty"` // Inline SQL commands
	File     string   `json:"file,omitempty" yaml:"file,omitempty"`         // External SQL file
	Timeout  string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`   // Query timeout (e.g., "30s")
}

// SQLResponse represents the response from SQL operations
type SQLResponse struct {
	Queries []QueryResult  `json:"queries"`
	Stats   ExecutionStats `json:"stats"`
}

// QueryResult represents the result of a single SQL query
type QueryResult struct {
	Query        string                   `json:"query"`
	RowsAffected int64                    `json:"rows_affected"`
	Rows         []map[string]interface{} `json:"rows"`
	Error        string                   `json:"error,omitempty"`
	Duration     string                   `json:"duration"`
}

// ExecutionStats provides overall execution statistics
type ExecutionStats struct {
	TotalQueries   int    `json:"total_queries"`
	SuccessCount   int    `json:"success_count"`
	ErrorCount     int    `json:"error_count"`
	TotalDuration  string `json:"total_duration"`
	ConnectionTime string `json:"connection_time"`
}

// ActivityResponse represents the complete activity response
type ActivityResponse struct {
	Response *SQLResponse      `json:"response"`
	Saved    map[string]string `json:"saved"`
}
