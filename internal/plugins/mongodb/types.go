package mongodb

// MongoDBPlugin represents the MongoDB plugin configuration
type MongoDBPlugin struct {
	Name   string        `json:"name" yaml:"name"`
	Plugin string        `json:"plugin" yaml:"plugin"`
	Config MongoDBConfig `json:"config" yaml:"config"`
}

// MongoDBConfig defines the configuration for MongoDB operations
type MongoDBConfig struct {
	URI        string          `json:"uri" yaml:"uri"`                               // MongoDB connection URI
	Database   string          `json:"database" yaml:"database"`                     // Database name
	Collection string          `json:"collection,omitempty" yaml:"collection,omitempty"` // Collection name for operations
	Operation  string          `json:"operation" yaml:"operation"`                   // Operation type
	Insert     *InsertConfig   `json:"insert,omitempty" yaml:"insert,omitempty"`
	Find       *FindConfig     `json:"find,omitempty" yaml:"find,omitempty"`
	Update     *UpdateConfig   `json:"update,omitempty" yaml:"update,omitempty"`
	Delete     *DeleteConfig   `json:"delete,omitempty" yaml:"delete,omitempty"`
	Count      *CountConfig    `json:"count,omitempty" yaml:"count,omitempty"`
	Aggregate  *AggregateConfig `json:"aggregate,omitempty" yaml:"aggregate,omitempty"`
	Index      *IndexConfig    `json:"index,omitempty" yaml:"index,omitempty"`
	Admin      *AdminConfig    `json:"admin,omitempty" yaml:"admin,omitempty"`
	Timeout    string          `json:"timeout,omitempty" yaml:"timeout,omitempty"`   // Operation timeout
}

// InsertConfig defines configuration for INSERT operations
type InsertConfig struct {
	Document  interface{}   `json:"document,omitempty" yaml:"document,omitempty"`     // Single document to insert
	Documents []interface{} `json:"documents,omitempty" yaml:"documents,omitempty"`   // Multiple documents to insert
	Ordered   bool          `json:"ordered,omitempty" yaml:"ordered,omitempty"`       // Whether to perform ordered insertion
}

// FindConfig defines configuration for FIND operations
type FindConfig struct {
	Filter     map[string]interface{} `json:"filter,omitempty" yaml:"filter,omitempty"`         // Query filter
	Projection map[string]interface{} `json:"projection,omitempty" yaml:"projection,omitempty"` // Fields to include/exclude
	Sort       map[string]interface{} `json:"sort,omitempty" yaml:"sort,omitempty"`             // Sort specification
	Limit      *int64                 `json:"limit,omitempty" yaml:"limit,omitempty"`           // Limit number of results
	Skip       *int64                 `json:"skip,omitempty" yaml:"skip,omitempty"`             // Number of documents to skip
}

// UpdateConfig defines configuration for UPDATE operations
type UpdateConfig struct {
	Filter    map[string]interface{} `json:"filter" yaml:"filter"`                           // Query filter
	Update    map[string]interface{} `json:"update" yaml:"update"`                           // Update document
	Upsert    bool                   `json:"upsert,omitempty" yaml:"upsert,omitempty"`       // Create if not exists
	Multiple  bool                   `json:"multiple,omitempty" yaml:"multiple,omitempty"`   // Update multiple documents
}

// DeleteConfig defines configuration for DELETE operations
type DeleteConfig struct {
	Filter   map[string]interface{} `json:"filter" yaml:"filter"`                         // Query filter
	Multiple bool                   `json:"multiple,omitempty" yaml:"multiple,omitempty"` // Delete multiple documents
}

// CountConfig defines configuration for COUNT operations
type CountConfig struct {
	Filter map[string]interface{} `json:"filter,omitempty" yaml:"filter,omitempty"` // Query filter
}

// AggregateConfig defines configuration for AGGREGATE operations
type AggregateConfig struct {
	Pipeline []map[string]interface{} `json:"pipeline" yaml:"pipeline"` // Aggregation pipeline
}

// IndexConfig defines configuration for INDEX operations
type IndexConfig struct {
	Keys    map[string]interface{} `json:"keys" yaml:"keys"`                             // Index keys specification
	Options map[string]interface{} `json:"options,omitempty" yaml:"options,omitempty"`   // Index options
	Action  string                 `json:"action" yaml:"action"`                         // create, drop, list
	Name    string                 `json:"name,omitempty" yaml:"name,omitempty"`         // Index name for drop operation
}

// AdminConfig defines configuration for ADMIN operations
type AdminConfig struct {
	Action     string `json:"action" yaml:"action"`                             // create_collection, drop_collection, list_collections
	Collection string `json:"collection,omitempty" yaml:"collection,omitempty"` // Collection name
}

// MongoDBResponse represents the response from MongoDB operations
type MongoDBResponse struct {
	Data         interface{}       `json:"data,omitempty"`         // Response data
	InsertedIDs  []interface{}     `json:"inserted_ids,omitempty"` // IDs of inserted documents
	ModifiedCount int64            `json:"modified_count,omitempty"` // Number of modified documents
	DeletedCount int64             `json:"deleted_count,omitempty"` // Number of deleted documents
	UpsertedID   interface{}       `json:"upserted_id,omitempty"`  // ID of upserted document
	Count        int64             `json:"count,omitempty"`        // Document count
	MatchedCount int64             `json:"matched_count,omitempty"` // Number of matched documents
	Error        *MongoDBError     `json:"error,omitempty"`        // Error details
	Metadata     *ResponseMetadata `json:"metadata,omitempty"`     // Operation metadata
}

// MongoDBError represents a MongoDB operation error
type MongoDBError struct {
	Code    int    `json:"code,omitempty"`    // Error code
	Message string `json:"message,omitempty"` // Error message
	Details string `json:"details,omitempty"` // Error details
}

// ResponseMetadata provides operation metadata
type ResponseMetadata struct {
	Operation   string `json:"operation"`           // Operation performed
	Database    string `json:"database,omitempty"`  // Database involved
	Collection  string `json:"collection,omitempty"` // Collection involved
	Duration    string `json:"duration"`            // Operation duration
}

// ActivityResponse represents the complete activity response
type ActivityResponse struct {
	Response *MongoDBResponse   `json:"response"`
	Saved    map[string]string  `json:"saved"`
}

// Operation constants
const (
	OpInsert         = "insert"
	OpInsertMany     = "insert_many"
	OpFind           = "find"
	OpFindOne        = "find_one"
	OpUpdate         = "update"
	OpUpdateMany     = "update_many"
	OpDelete         = "delete"
	OpDeleteMany     = "delete_many"
	OpCount          = "count"
	OpAggregate      = "aggregate"
	OpCreateIndex    = "create_index"
	OpDropIndex      = "drop_index"
	OpListIndexes    = "list_indexes"
	OpCreateCollection = "create_collection"
	OpDropCollection   = "drop_collection"
	OpListCollections  = "list_collections"
)