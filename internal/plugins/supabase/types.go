package supabase

// SupabasePlugin represents the Supabase plugin configuration
type SupabasePlugin struct {
	Name   string          `json:"name" yaml:"name"`
	Plugin string          `json:"plugin" yaml:"plugin"`
	Config SupabaseConfig  `json:"config" yaml:"config"`
}

// SupabaseConfig defines the configuration for Supabase operations
type SupabaseConfig struct {
	URL       string            `json:"url" yaml:"url"`                     // Supabase project URL
	Key       string            `json:"key" yaml:"key"`                     // Supabase API key (anon or service)
	Operation string            `json:"operation" yaml:"operation"`         // Operation type
	Table     string            `json:"table,omitempty" yaml:"table,omitempty"` // Table name for DB operations
	Select    *SelectConfig     `json:"select,omitempty" yaml:"select,omitempty"`
	Insert    *InsertConfig     `json:"insert,omitempty" yaml:"insert,omitempty"`
	Update    *UpdateConfig     `json:"update,omitempty" yaml:"update,omitempty"`
	Delete    *DeleteConfig     `json:"delete,omitempty" yaml:"delete,omitempty"`
	RPC       *RPCConfig        `json:"rpc,omitempty" yaml:"rpc,omitempty"`
	Auth      *AuthConfig       `json:"auth,omitempty" yaml:"auth,omitempty"`
	Storage   *StorageConfig    `json:"storage,omitempty" yaml:"storage,omitempty"`
	Timeout   string            `json:"timeout,omitempty" yaml:"timeout,omitempty"` // Operation timeout
}

// SelectConfig defines configuration for SELECT operations
type SelectConfig struct {
	Columns []string      `json:"columns,omitempty" yaml:"columns,omitempty"` // Columns to select
	Filters []FilterConfig `json:"filters,omitempty" yaml:"filters,omitempty"` // Filters to apply
	Order   []OrderConfig `json:"order,omitempty" yaml:"order,omitempty"`     // Ordering
	Limit   *int          `json:"limit,omitempty" yaml:"limit,omitempty"`     // Result limit
	Offset  *int          `json:"offset,omitempty" yaml:"offset,omitempty"`   // Result offset
	Count   string        `json:"count,omitempty" yaml:"count,omitempty"`     // Count type (exact, planned, estimated)
}

// InsertConfig defines configuration for INSERT operations
type InsertConfig struct {
	Data       interface{} `json:"data" yaml:"data"`                                 // Data to insert
	Upsert     bool        `json:"upsert,omitempty" yaml:"upsert,omitempty"`         // Use upsert
	OnConflict string      `json:"on_conflict,omitempty" yaml:"on_conflict,omitempty"` // Conflict resolution column
}

// UpdateConfig defines configuration for UPDATE operations
type UpdateConfig struct {
	Data    map[string]interface{} `json:"data" yaml:"data"`                         // Data to update
	Filters []FilterConfig         `json:"filters,omitempty" yaml:"filters,omitempty"` // Filters for update
}

// DeleteConfig defines configuration for DELETE operations
type DeleteConfig struct {
	Filters []FilterConfig `json:"filters" yaml:"filters"` // Filters for delete (required for safety)
}

// FilterConfig defines a filter condition
type FilterConfig struct {
	Column   string      `json:"column" yaml:"column"`     // Column name
	Operator string      `json:"operator" yaml:"operator"` // Filter operator
	Value    interface{} `json:"value" yaml:"value"`       // Filter value
}

// OrderConfig defines ordering configuration
type OrderConfig struct {
	Column    string `json:"column" yaml:"column"`                     // Column to order by
	Ascending bool   `json:"ascending,omitempty" yaml:"ascending,omitempty"` // Ascending order (default true)
}

// RPCConfig defines configuration for RPC function calls
type RPCConfig struct {
	Function string                 `json:"function" yaml:"function"`           // Function name
	Params   map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"` // Function parameters
}

// AuthConfig defines configuration for auth operations
type AuthConfig struct {
	Email         string                 `json:"email,omitempty" yaml:"email,omitempty"`                   // User email
	Password      string                 `json:"password,omitempty" yaml:"password,omitempty"`             // User password
	UserID        string                 `json:"user_id,omitempty" yaml:"user_id,omitempty"`               // User ID for admin operations
	EmailConfirm  bool                   `json:"email_confirm,omitempty" yaml:"email_confirm,omitempty"`   // Auto-confirm email (admin only)
	UserMetadata  map[string]interface{} `json:"user_metadata,omitempty" yaml:"user_metadata,omitempty"`   // User metadata
	AppMetadata   map[string]interface{} `json:"app_metadata,omitempty" yaml:"app_metadata,omitempty"`     // App metadata
}

// StorageConfig defines configuration for storage operations
type StorageConfig struct {
	Bucket       string `json:"bucket,omitempty" yaml:"bucket,omitempty"`             // Bucket name
	Path         string `json:"path,omitempty" yaml:"path,omitempty"`                 // File path
	FileContent  string `json:"file_content,omitempty" yaml:"file_content,omitempty"` // File content (base64 or text)
	FilePath     string `json:"file_path,omitempty" yaml:"file_path,omitempty"`       // Local file path
	Public       bool   `json:"public,omitempty" yaml:"public,omitempty"`             // Public access
	CacheControl string `json:"cache_control,omitempty" yaml:"cache_control,omitempty"` // Cache control header
	ContentType  string `json:"content_type,omitempty" yaml:"content_type,omitempty"` // Content type
}

// SupabaseResponse represents the response from Supabase operations
type SupabaseResponse struct {
	Data      interface{}        `json:"data,omitempty"`      // Response data
	Count     *int               `json:"count,omitempty"`     // Row count for queries
	Error     *SupabaseError     `json:"error,omitempty"`     // Error details
	Metadata  *ResponseMetadata  `json:"metadata,omitempty"`  // Operation metadata
}

// SupabaseError represents a Supabase API error
type SupabaseError struct {
	Code    string `json:"code,omitempty"`    // Error code
	Message string `json:"message,omitempty"` // Error message
	Details string `json:"details,omitempty"` // Error details
	Hint    string `json:"hint,omitempty"`    // Error hint
}

// ResponseMetadata provides operation metadata
type ResponseMetadata struct {
	Operation   string `json:"operation"`           // Operation performed
	Table       string `json:"table,omitempty"`     // Table involved
	Duration    string `json:"duration"`            // Operation duration
	StatusCode  int    `json:"status_code"`         // HTTP status code
	Headers     map[string]string `json:"headers,omitempty"` // Response headers
}

// ActivityResponse represents the complete activity response
type ActivityResponse struct {
	Response *SupabaseResponse  `json:"response"`
	Saved    map[string]string  `json:"saved"`
}

// Operation constants
const (
	OpSelect           = "select"
	OpInsert           = "insert"
	OpUpdate           = "update"
	OpDelete           = "delete"
	OpRPC              = "rpc"
	OpAuthCreateUser   = "auth_create_user"
	OpAuthDeleteUser   = "auth_delete_user"
	OpAuthSignUp       = "auth_sign_up"
	OpAuthSignIn       = "auth_sign_in"
	OpStorageCreateBucket = "storage_create_bucket"
	OpStorageDeleteBucket = "storage_delete_bucket"
	OpStorageUpload    = "storage_upload"
	OpStorageDownload  = "storage_download"
	OpStorageDelete    = "storage_delete"
)

// Filter operators
const (
	OpEq          = "eq"
	OpNeq         = "neq"
	OpGt          = "gt"
	OpGte         = "gte"
	OpLt          = "lt"
	OpLte         = "lte"
	OpLike        = "like"
	OpILike       = "ilike"
	OpIs          = "is"
	OpIn          = "in"
	OpContains    = "contains"
	OpContainedBy = "contained_by"
	OpRangeGt     = "range_gt"
	OpRangeGte    = "range_gte"
	OpRangeLt     = "range_lt"
	OpRangeLte    = "range_lte"
)