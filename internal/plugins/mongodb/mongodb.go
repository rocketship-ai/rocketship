package mongodb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/itchyny/gojq"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.temporal.io/sdk/activity"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&MongoDBPlugin{})
}

// GetType returns the plugin type identifier
func (mp *MongoDBPlugin) GetType() string {
	return "mongodb"
}

// Activity executes MongoDB operations and returns results
func (mp *MongoDBPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)
	
	// Parse configuration from parameters
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	// Get state for variable replacement
	state := make(map[string]string)
	if stateInterface, ok := p["state"]; ok {
		if stateMap, ok := stateInterface.(map[string]interface{}); ok {
			for k, v := range stateMap {
				state[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	config := &MongoDBConfig{}
	if err := parseConfig(configData, config); err != nil {
		return nil, fmt.Errorf("failed to parse MongoDB config: %w", err)
	}
	
	// Process runtime variables in string fields using central DSL template system
	config.URI = replaceVariables(config.URI, state)
	config.Database = replaceVariables(config.Database, state)
	if config.Collection != "" {
		config.Collection = replaceVariables(config.Collection, state)
	}
	
	// Process runtime variables in operation configurations
	if config.Insert != nil {
		config.Insert = processInsertConfig(config.Insert, state)
	}
	if config.Find != nil {
		config.Find = processFindConfig(config.Find, state)
	}
	if config.Update != nil {
		config.Update = processUpdateConfig(config.Update, state)
	}
	if config.Delete != nil {
		config.Delete = processDeleteConfig(config.Delete, state)
	}
	if config.Count != nil {
		config.Count = processCountConfig(config.Count, state)
	}
	if config.Aggregate != nil {
		config.Aggregate = processAggregateConfig(config.Aggregate, state)
	}
	if config.Index != nil {
		config.Index = processIndexConfig(config.Index, state)
	}
	if config.Admin != nil {
		config.Admin = processAdminConfig(config.Admin, state)
	}

	// Validate required fields
	if config.URI == "" {
		return nil, fmt.Errorf("uri is required")
	}
	if config.Database == "" {
		return nil, fmt.Errorf("database is required")
	}
	if config.Operation == "" {
		return nil, fmt.Errorf("operation is required")
	}

	logger.Info("Executing MongoDB operation", "operation", config.Operation, "database", config.Database, "collection", config.Collection)

	// Set default timeout
	timeout := 30 * time.Second
	if config.Timeout != "" {
		if parsedTimeout, err := time.ParseDuration(config.Timeout); err == nil {
			timeout = parsedTimeout
		}
	}

	// Create context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	startTime := time.Now()
	response, err := executeMongoDBOperation(ctxWithTimeout, config)
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("MongoDB operation failed", "error", err, "duration", duration)
		return nil, err
	}

	// Add metadata
	if response.Metadata == nil {
		response.Metadata = &ResponseMetadata{}
	}
	response.Metadata.Operation = config.Operation
	response.Metadata.Database = config.Database
	response.Metadata.Collection = config.Collection
	response.Metadata.Duration = duration.String()

	logger.Info("MongoDB operation completed", "operation", config.Operation, "duration", duration)

	// Process assertions
	if assertions, ok := p["assertions"].([]interface{}); ok {
		if err := processAssertions(response, assertions, p); err != nil {
			logger.Error("Assertion failed", "error", err)
			return nil, fmt.Errorf("assertion failed: %w", err)
		}
	}

	// Handle save operations
	saved := make(map[string]string)
	if saveConfigs, ok := p["save"].([]interface{}); ok {
		for _, saveConfigInterface := range saveConfigs {
			if saveConfig, ok := saveConfigInterface.(map[string]interface{}); ok {
				if err := processSave(response, saveConfig, saved); err != nil {
					logger.Warn("Failed to save value", "error", err)
				}
			}
		}
	}

	return &ActivityResponse{
		Response: response,
		Saved:    saved,
	}, nil
}

// executeMongoDBOperation performs the actual MongoDB operation
func executeMongoDBOperation(ctx context.Context, config *MongoDBConfig) (*MongoDBResponse, error) {
	// Connect to MongoDB
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.URI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			activity.GetLogger(ctx).Warn("Failed to disconnect from MongoDB", "error", err)
		}
	}()

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	// Get database
	db := client.Database(config.Database)

	switch config.Operation {
	case OpInsert:
		return executeInsert(ctx, db, config)
	case OpInsertMany:
		return executeInsertMany(ctx, db, config)
	case OpFind:
		return executeFind(ctx, db, config)
	case OpFindOne:
		return executeFindOne(ctx, db, config)
	case OpUpdate:
		return executeUpdate(ctx, db, config)
	case OpUpdateMany:
		return executeUpdateMany(ctx, db, config)
	case OpDelete:
		return executeDelete(ctx, db, config)
	case OpDeleteMany:
		return executeDeleteMany(ctx, db, config)
	case OpCount:
		return executeCount(ctx, db, config)
	case OpAggregate:
		return executeAggregate(ctx, db, config)
	case OpCreateIndex:
		return executeCreateIndex(ctx, db, config)
	case OpDropIndex:
		return executeDropIndex(ctx, db, config)
	case OpListIndexes:
		return executeListIndexes(ctx, db, config)
	case OpCreateCollection:
		return executeCreateCollection(ctx, db, config)
	case OpDropCollection:
		return executeDropCollection(ctx, db, config)
	case OpListCollections:
		return executeListCollections(ctx, db, config)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", config.Operation)
	}
}

// executeInsert handles INSERT operations
func executeInsert(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for insert operation")
	}
	if config.Insert == nil || config.Insert.Document == nil {
		return nil, fmt.Errorf("document is required for insert operation")
	}

	collection := db.Collection(config.Collection)
	
	result, err := collection.InsertOne(ctx, config.Insert.Document)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		InsertedIDs: []interface{}{result.InsertedID},
	}, nil
}

// executeInsertMany handles INSERT_MANY operations
func executeInsertMany(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for insert_many operation")
	}
	if config.Insert == nil || len(config.Insert.Documents) == 0 {
		return nil, fmt.Errorf("documents are required for insert_many operation")
	}

	collection := db.Collection(config.Collection)
	
	opts := options.InsertMany()
	if config.Insert.Ordered {
		opts.SetOrdered(true)
	}
	
	result, err := collection.InsertMany(ctx, config.Insert.Documents, opts)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		InsertedIDs: result.InsertedIDs,
	}, nil
}

// executeFind handles FIND operations
func executeFind(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for find operation")
	}

	collection := db.Collection(config.Collection)
	
	filter := bson.M{}
	if config.Find != nil && config.Find.Filter != nil {
		filter = config.Find.Filter
	}
	
	opts := options.Find()
	if config.Find != nil {
		if config.Find.Projection != nil {
			opts.SetProjection(config.Find.Projection)
		}
		if config.Find.Sort != nil {
			opts.SetSort(config.Find.Sort)
		}
		if config.Find.Limit != nil {
			opts.SetLimit(*config.Find.Limit)
		}
		if config.Find.Skip != nil {
			opts.SetSkip(*config.Find.Skip)
		}
	}
	
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			activity.GetLogger(ctx).Warn("Failed to close cursor", "error", err)
		}
	}()
	
	var results []interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		Data: results,
	}, nil
}

// executeFindOne handles FIND_ONE operations
func executeFindOne(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for find_one operation")
	}

	collection := db.Collection(config.Collection)
	
	filter := bson.M{}
	if config.Find != nil && config.Find.Filter != nil {
		filter = config.Find.Filter
	}
	
	opts := options.FindOne()
	if config.Find != nil {
		if config.Find.Projection != nil {
			opts.SetProjection(config.Find.Projection)
		}
		if config.Find.Sort != nil {
			opts.SetSort(config.Find.Sort)
		}
		if config.Find.Skip != nil {
			opts.SetSkip(*config.Find.Skip)
		}
	}
	
	var result bson.M
	err := collection.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &MongoDBResponse{
				Data: nil,
			}, nil
		}
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		Data: result,
	}, nil
}

// executeUpdate handles UPDATE operations
func executeUpdate(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for update operation")
	}
	if config.Update == nil || config.Update.Filter == nil || config.Update.Update == nil {
		return nil, fmt.Errorf("filter and update are required for update operation")
	}

	collection := db.Collection(config.Collection)
	
	opts := options.Update()
	if config.Update.Upsert {
		opts.SetUpsert(true)
	}
	
	result, err := collection.UpdateOne(ctx, config.Update.Filter, config.Update.Update, opts)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	response := &MongoDBResponse{
		MatchedCount:  result.MatchedCount,
		ModifiedCount: result.ModifiedCount,
	}
	
	if result.UpsertedID != nil {
		response.UpsertedID = result.UpsertedID
	}

	return response, nil
}

// executeUpdateMany handles UPDATE_MANY operations
func executeUpdateMany(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for update_many operation")
	}
	if config.Update == nil || config.Update.Filter == nil || config.Update.Update == nil {
		return nil, fmt.Errorf("filter and update are required for update_many operation")
	}

	collection := db.Collection(config.Collection)
	
	opts := options.Update()
	if config.Update.Upsert {
		opts.SetUpsert(true)
	}
	
	result, err := collection.UpdateMany(ctx, config.Update.Filter, config.Update.Update, opts)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	response := &MongoDBResponse{
		MatchedCount:  result.MatchedCount,
		ModifiedCount: result.ModifiedCount,
	}
	
	if result.UpsertedID != nil {
		response.UpsertedID = result.UpsertedID
	}

	return response, nil
}

// executeDelete handles DELETE operations
func executeDelete(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for delete operation")
	}
	if config.Delete == nil || config.Delete.Filter == nil {
		return nil, fmt.Errorf("filter is required for delete operation")
	}

	collection := db.Collection(config.Collection)
	
	result, err := collection.DeleteOne(ctx, config.Delete.Filter)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		DeletedCount: result.DeletedCount,
	}, nil
}

// executeDeleteMany handles DELETE_MANY operations
func executeDeleteMany(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for delete_many operation")
	}
	if config.Delete == nil || config.Delete.Filter == nil {
		return nil, fmt.Errorf("filter is required for delete_many operation")
	}

	collection := db.Collection(config.Collection)
	
	result, err := collection.DeleteMany(ctx, config.Delete.Filter)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		DeletedCount: result.DeletedCount,
	}, nil
}

// executeCount handles COUNT operations
func executeCount(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for count operation")
	}

	collection := db.Collection(config.Collection)
	
	filter := bson.M{}
	if config.Count != nil && config.Count.Filter != nil {
		filter = config.Count.Filter
	}
	
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		Count: count,
	}, nil
}

// executeAggregate handles AGGREGATE operations
func executeAggregate(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for aggregate operation")
	}
	if config.Aggregate == nil || len(config.Aggregate.Pipeline) == 0 {
		return nil, fmt.Errorf("pipeline is required for aggregate operation")
	}

	collection := db.Collection(config.Collection)
	
	// Convert pipeline to proper bson format
	var pipeline []interface{}
	for _, stage := range config.Aggregate.Pipeline {
		pipeline = append(pipeline, stage)
	}
	
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			activity.GetLogger(ctx).Warn("Failed to close cursor", "error", err)
		}
	}()
	
	var results []interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		Data: results,
	}, nil
}

// executeCreateIndex handles CREATE_INDEX operations
func executeCreateIndex(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for create_index operation")
	}
	if config.Index == nil || config.Index.Keys == nil {
		return nil, fmt.Errorf("keys are required for create_index operation")
	}

	collection := db.Collection(config.Collection)
	
	indexModel := mongo.IndexModel{
		Keys: config.Index.Keys,
	}
	
	if config.Index.Options != nil {
		opts := options.Index()
		// Process common index options
		if name, ok := config.Index.Options["name"].(string); ok {
			opts.SetName(name)
		}
		if unique, ok := config.Index.Options["unique"].(bool); ok {
			opts.SetUnique(unique)
		}
		if sparse, ok := config.Index.Options["sparse"].(bool); ok {
			opts.SetSparse(sparse)
		}
		// Note: background option is deprecated in MongoDB 4.2+
		indexModel.Options = opts
	}
	
	indexName, err := collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		Data: map[string]interface{}{
			"index_name": indexName,
		},
	}, nil
}

// executeDropIndex handles DROP_INDEX operations
func executeDropIndex(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for drop_index operation")
	}
	if config.Index == nil || config.Index.Name == "" {
		return nil, fmt.Errorf("name is required for drop_index operation")
	}

	collection := db.Collection(config.Collection)
	
	_, err := collection.Indexes().DropOne(ctx, config.Index.Name)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		Data: map[string]interface{}{
			"dropped_index": config.Index.Name,
		},
	}, nil
}

// executeListIndexes handles LIST_INDEXES operations
func executeListIndexes(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Collection == "" {
		return nil, fmt.Errorf("collection is required for list_indexes operation")
	}

	collection := db.Collection(config.Collection)
	
	cursor, err := collection.Indexes().List(ctx)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			activity.GetLogger(ctx).Warn("Failed to close cursor", "error", err)
		}
	}()
	
	var indexes []interface{}
	if err := cursor.All(ctx, &indexes); err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		Data: indexes,
	}, nil
}

// executeCreateCollection handles CREATE_COLLECTION operations
func executeCreateCollection(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Admin == nil || config.Admin.Collection == "" {
		return nil, fmt.Errorf("collection name is required for create_collection operation")
	}

	err := db.CreateCollection(ctx, config.Admin.Collection)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		Data: map[string]interface{}{
			"created_collection": config.Admin.Collection,
		},
	}, nil
}

// executeDropCollection handles DROP_COLLECTION operations
func executeDropCollection(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	if config.Admin == nil || config.Admin.Collection == "" {
		return nil, fmt.Errorf("collection name is required for drop_collection operation")
	}

	collection := db.Collection(config.Admin.Collection)
	err := collection.Drop(ctx)
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	return &MongoDBResponse{
		Data: map[string]interface{}{
			"dropped_collection": config.Admin.Collection,
		},
	}, nil
}

// executeListCollections handles LIST_COLLECTIONS operations
func executeListCollections(ctx context.Context, db *mongo.Database, config *MongoDBConfig) (*MongoDBResponse, error) {
	names, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return &MongoDBResponse{
			Error: &MongoDBError{
				Message: err.Error(),
			},
		}, nil
	}

	var collections []interface{}
	for _, name := range names {
		collections = append(collections, map[string]interface{}{
			"name": name,
		})
	}

	return &MongoDBResponse{
		Data: collections,
	}, nil
}

// Helper functions for variable replacement using central DSL template system

// replaceVariables replaces template variables using the central DSL template system
func replaceVariables(input string, state map[string]string) string {
	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}
	
	// Create template context with runtime variables
	context := dsl.TemplateContext{
		Runtime: runtime,
	}
	
	// Use centralized template processing for consistent variable handling
	result, err := dsl.ProcessTemplate(input, context)
	if err != nil {
		// If template processing fails, return original input to maintain backward compatibility
		return input
	}
	
	return result
}

// processVariablesInMap recursively processes variables in a map structure using DSL template system
func processVariablesInMap(data map[string]interface{}, state map[string]string) map[string]interface{} {
	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}
	
	// Create template context with runtime variables
	context := dsl.TemplateContext{
		Runtime: runtime,
	}
	
	// Use DSL recursive processing which handles all data types consistently
	result := processRuntimeVariablesRecursive(data, context)
	if resultMap, ok := result.(map[string]interface{}); ok {
		return resultMap
	}
	
	// Fallback to original data if processing fails
	return data
}

// processRuntimeVariablesRecursive processes runtime variables in any nested data structure
func processRuntimeVariablesRecursive(data interface{}, context dsl.TemplateContext) interface{} {
	switch v := data.(type) {
	case string:
		result, err := dsl.ProcessTemplate(v, context)
		if err != nil {
			return v // Return original on error
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			// Process the key itself (in case it contains variables)
			processedKey, err := dsl.ProcessTemplate(key, context)
			if err != nil {
				processedKey = key // Use original key on error
			}
			// Process the value recursively
			result[processedKey] = processRuntimeVariablesRecursive(value, context)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = processRuntimeVariablesRecursive(item, context)
		}
		return result
	default:
		// For non-string types (numbers, booleans, etc.), return as-is
		return data
	}
}

// Configuration processing functions
func processInsertConfig(config *InsertConfig, state map[string]string) *InsertConfig {
	if config == nil {
		return nil
	}
	
	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}
	
	context := dsl.TemplateContext{
		Runtime: runtime,
	}
	
	result := &InsertConfig{
		Ordered: config.Ordered,
	}
	
	if config.Document != nil {
		result.Document = processRuntimeVariablesRecursive(config.Document, context)
	}
	
	if len(config.Documents) > 0 {
		result.Documents = make([]interface{}, len(config.Documents))
		for i, doc := range config.Documents {
			result.Documents[i] = processRuntimeVariablesRecursive(doc, context)
		}
	}
	
	return result
}

func processFindConfig(config *FindConfig, state map[string]string) *FindConfig {
	if config == nil {
		return nil
	}
	
	result := &FindConfig{
		Limit: config.Limit,
		Skip:  config.Skip,
	}
	
	if config.Filter != nil {
		result.Filter = processVariablesInMap(config.Filter, state)
	}
	if config.Projection != nil {
		result.Projection = processVariablesInMap(config.Projection, state)
	}
	if config.Sort != nil {
		result.Sort = processVariablesInMap(config.Sort, state)
	}
	
	return result
}

func processUpdateConfig(config *UpdateConfig, state map[string]string) *UpdateConfig {
	if config == nil {
		return nil
	}
	
	result := &UpdateConfig{
		Upsert:   config.Upsert,
		Multiple: config.Multiple,
	}
	
	if config.Filter != nil {
		result.Filter = processVariablesInMap(config.Filter, state)
	}
	if config.Update != nil {
		result.Update = processVariablesInMap(config.Update, state)
	}
	
	return result
}

func processDeleteConfig(config *DeleteConfig, state map[string]string) *DeleteConfig {
	if config == nil {
		return nil
	}
	
	result := &DeleteConfig{
		Multiple: config.Multiple,
	}
	
	if config.Filter != nil {
		result.Filter = processVariablesInMap(config.Filter, state)
	}
	
	return result
}

func processCountConfig(config *CountConfig, state map[string]string) *CountConfig {
	if config == nil {
		return nil
	}
	
	result := &CountConfig{}
	
	if config.Filter != nil {
		result.Filter = processVariablesInMap(config.Filter, state)
	}
	
	return result
}

func processAggregateConfig(config *AggregateConfig, state map[string]string) *AggregateConfig {
	if config == nil {
		return nil
	}
	
	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}
	
	context := dsl.TemplateContext{
		Runtime: runtime,
	}
	
	result := &AggregateConfig{
		Pipeline: make([]map[string]interface{}, len(config.Pipeline)),
	}
	
	for i, stage := range config.Pipeline {
		result.Pipeline[i] = processRuntimeVariablesRecursive(stage, context).(map[string]interface{})
	}
	
	return result
}

func processIndexConfig(config *IndexConfig, state map[string]string) *IndexConfig {
	if config == nil {
		return nil
	}
	
	result := &IndexConfig{
		Action: replaceVariables(config.Action, state),
		Name:   replaceVariables(config.Name, state),
	}
	
	if config.Keys != nil {
		result.Keys = processVariablesInMap(config.Keys, state)
	}
	if config.Options != nil {
		result.Options = processVariablesInMap(config.Options, state)
	}
	
	return result
}

func processAdminConfig(config *AdminConfig, state map[string]string) *AdminConfig {
	if config == nil {
		return nil
	}
	
	return &AdminConfig{
		Action:     replaceVariables(config.Action, state),
		Collection: replaceVariables(config.Collection, state),
	}
}

// parseConfig converts map[string]interface{} to MongoDBConfig
func parseConfig(configData map[string]interface{}, config *MongoDBConfig) error {
	if uri, ok := configData["uri"].(string); ok {
		config.URI = uri
	}
	if database, ok := configData["database"].(string); ok {
		config.Database = database
	}
	if collection, ok := configData["collection"].(string); ok {
		config.Collection = collection
	}
	if operation, ok := configData["operation"].(string); ok {
		config.Operation = operation
	}
	if timeout, ok := configData["timeout"].(string); ok {
		config.Timeout = timeout
	}

	// Parse operation-specific configs
	if insertData, ok := configData["insert"].(map[string]interface{}); ok {
		config.Insert = &InsertConfig{}
		parseInsertConfig(insertData, config.Insert)
	}
	
	if findData, ok := configData["find"].(map[string]interface{}); ok {
		config.Find = &FindConfig{}
		parseFindConfig(findData, config.Find)
	}
	
	if updateData, ok := configData["update"].(map[string]interface{}); ok {
		config.Update = &UpdateConfig{}
		parseUpdateConfig(updateData, config.Update)
	}
	
	if deleteData, ok := configData["delete"].(map[string]interface{}); ok {
		config.Delete = &DeleteConfig{}
		parseDeleteConfig(deleteData, config.Delete)
	}
	
	if countData, ok := configData["count"].(map[string]interface{}); ok {
		config.Count = &CountConfig{}
		parseCountConfig(countData, config.Count)
	}
	
	if aggregateData, ok := configData["aggregate"].(map[string]interface{}); ok {
		config.Aggregate = &AggregateConfig{}
		parseAggregateConfig(aggregateData, config.Aggregate)
	}
	
	if indexData, ok := configData["index"].(map[string]interface{}); ok {
		config.Index = &IndexConfig{}
		parseIndexConfig(indexData, config.Index)
	}
	
	if adminData, ok := configData["admin"].(map[string]interface{}); ok {
		config.Admin = &AdminConfig{}
		parseAdminConfig(adminData, config.Admin)
	}

	return nil
}

// Helper parsing functions for each config type
func parseInsertConfig(data map[string]interface{}, config *InsertConfig) {
	if document, ok := data["document"]; ok {
		config.Document = document
	}
	if documents, ok := data["documents"].([]interface{}); ok {
		config.Documents = documents
	}
	if ordered, ok := data["ordered"].(bool); ok {
		config.Ordered = ordered
	}
}

func parseFindConfig(data map[string]interface{}, config *FindConfig) {
	if filter, ok := data["filter"].(map[string]interface{}); ok {
		config.Filter = filter
	}
	if projection, ok := data["projection"].(map[string]interface{}); ok {
		config.Projection = projection
	}
	if sort, ok := data["sort"].(map[string]interface{}); ok {
		config.Sort = sort
	}
	if limit, ok := data["limit"].(float64); ok {
		limitInt := int64(limit)
		config.Limit = &limitInt
	}
	if skip, ok := data["skip"].(float64); ok {
		skipInt := int64(skip)
		config.Skip = &skipInt
	}
}

func parseUpdateConfig(data map[string]interface{}, config *UpdateConfig) {
	if filter, ok := data["filter"].(map[string]interface{}); ok {
		config.Filter = filter
	}
	if update, ok := data["update"].(map[string]interface{}); ok {
		config.Update = update
	}
	if upsert, ok := data["upsert"].(bool); ok {
		config.Upsert = upsert
	}
	if multiple, ok := data["multiple"].(bool); ok {
		config.Multiple = multiple
	}
}

func parseDeleteConfig(data map[string]interface{}, config *DeleteConfig) {
	if filter, ok := data["filter"].(map[string]interface{}); ok {
		config.Filter = filter
	}
	if multiple, ok := data["multiple"].(bool); ok {
		config.Multiple = multiple
	}
}

func parseCountConfig(data map[string]interface{}, config *CountConfig) {
	if filter, ok := data["filter"].(map[string]interface{}); ok {
		config.Filter = filter
	}
}

func parseAggregateConfig(data map[string]interface{}, config *AggregateConfig) {
	if pipeline, ok := data["pipeline"].([]interface{}); ok {
		config.Pipeline = make([]map[string]interface{}, len(pipeline))
		for i, stage := range pipeline {
			if stageMap, ok := stage.(map[string]interface{}); ok {
				config.Pipeline[i] = stageMap
			}
		}
	}
}

func parseIndexConfig(data map[string]interface{}, config *IndexConfig) {
	if keys, ok := data["keys"].(map[string]interface{}); ok {
		config.Keys = keys
	}
	if options, ok := data["options"].(map[string]interface{}); ok {
		config.Options = options
	}
	if action, ok := data["action"].(string); ok {
		config.Action = action
	}
	if name, ok := data["name"].(string); ok {
		config.Name = name
	}
}

func parseAdminConfig(data map[string]interface{}, config *AdminConfig) {
	if action, ok := data["action"].(string); ok {
		config.Action = action
	}
	if collection, ok := data["collection"].(string); ok {
		config.Collection = collection
	}
}

// processSave handles saving values from response using JSON path extraction
func processSave(response *MongoDBResponse, saveConfig map[string]interface{}, saved map[string]string) error {
	asName, ok := saveConfig["as"].(string)
	if !ok {
		return fmt.Errorf("'as' field is required for save config")
	}

	var value interface{}

	// Check for JSON path extraction
	if jsonPath, ok := saveConfig["json_path"].(string); ok {
		// Parse the JSON path using gojq
		query, err := gojq.Parse(jsonPath)
		if err != nil {
			return fmt.Errorf("failed to parse JSON path %s: %w", jsonPath, err)
		}

		// Run the query on the response
		iter := query.Run(response)
		v, ok := iter.Next()
		if !ok {
			return fmt.Errorf("no results from JSON path %s", jsonPath)
		}
		if err, ok := v.(error); ok {
			return fmt.Errorf("error evaluating JSON path %s: %w", jsonPath, err)
		}
		value = v
	} else {
		return fmt.Errorf("'json_path' must be specified for save config")
	}

	// Convert value to string
	if value != nil {
		saved[asName] = fmt.Sprintf("%v", value)
	}

	return nil
}

// processAssertions validates assertions against the MongoDB response
func processAssertions(response *MongoDBResponse, assertions []interface{}, params map[string]interface{}) error {
	// Rebuild state from parameters for variable replacement
	state := make(map[string]string)
	if stateInterface, ok := params["state"]; ok {
		if stateMap, ok := stateInterface.(map[string]interface{}); ok {
			for k, v := range stateMap {
				state[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	for _, assertionInterface := range assertions {
		assertion, ok := assertionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		assertionType, ok := assertion["type"].(string)
		if !ok {
			return fmt.Errorf("assertion type is required")
		}

		switch assertionType {
		case "document_exists":
			if err := processDocumentExistsAssertion(response, assertion, state); err != nil {
				return err
			}
		case "count_equals":
			if err := processCountEqualsAssertion(response, assertion, state); err != nil {
				return err
			}
		case "field_matches":
			if err := processFieldMatchesAssertion(response, assertion, state); err != nil {
				return err
			}
		case "result_contains":
			if err := processResultContainsAssertion(response, assertion, state); err != nil {
				return err
			}
		case "json_path":
			if err := processJSONPathAssertion(response, assertion, state); err != nil {
				return err
			}
		case "error_exists":
			if err := processErrorExistsAssertion(response, assertion, state); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported assertion type: %s", assertionType)
		}
	}

	return nil
}

// processDocumentExistsAssertion validates document existence
func processDocumentExistsAssertion(response *MongoDBResponse, assertion map[string]interface{}, state map[string]string) error {
	expected, ok := assertion["expected"].(bool)
	if !ok {
		return fmt.Errorf("expected boolean value is required for document_exists assertion")
	}

	exists := false
	if response.Data != nil {
		// Check if data is a single document or an array
		if dataArray, ok := response.Data.([]interface{}); ok {
			exists = len(dataArray) > 0
		} else {
			exists = true
		}
	}

	if exists != expected {
		return fmt.Errorf("document_exists assertion failed: expected %t, got %t", expected, exists)
	}

	return nil
}

// processCountEqualsAssertion validates count values
func processCountEqualsAssertion(response *MongoDBResponse, assertion map[string]interface{}, state map[string]string) error {
	expected, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for count_equals assertion")
	}

	// Replace variables in expected value
	expectedStr := replaceVariables(fmt.Sprintf("%v", expected), state)
	expectedCount, err := strconv.ParseInt(expectedStr, 10, 64)
	if err != nil {
		return fmt.Errorf("expected count must be an integer: %s", expectedStr)
	}

	var actualCount int64
	if response.Count > 0 {
		actualCount = response.Count
	} else if response.Data != nil {
		if dataArray, ok := response.Data.([]interface{}); ok {
			actualCount = int64(len(dataArray))
		} else {
			actualCount = 1
		}
	}

	if actualCount != expectedCount {
		return fmt.Errorf("count_equals assertion failed: expected %d, got %d", expectedCount, actualCount)
	}

	return nil
}

// processFieldMatchesAssertion validates field values in response data
func processFieldMatchesAssertion(response *MongoDBResponse, assertion map[string]interface{}, state map[string]string) error {
	field, ok := assertion["field"].(string)
	if !ok {
		return fmt.Errorf("field is required for field_matches assertion")
	}

	expected, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for field_matches assertion")
	}

	// Replace variables in expected value
	expectedStr := replaceVariables(fmt.Sprintf("%v", expected), state)

	// Get actual value from response
	var actualValue interface{}
	if response.Data != nil {
		if dataArray, ok := response.Data.([]interface{}); ok {
			if len(dataArray) > 0 {
				if doc, ok := dataArray[0].(map[string]interface{}); ok {
					actualValue = doc[field]
				} else if doc, ok := dataArray[0].(bson.M); ok {
					actualValue = doc[field]
				}
			}
		} else if doc, ok := response.Data.(map[string]interface{}); ok {
			actualValue = doc[field]
		} else if doc, ok := response.Data.(bson.M); ok {
			actualValue = doc[field]
		}
	}

	if actualValue == nil {
		return fmt.Errorf("field '%s' not found in response data", field)
	}

	actualStr := fmt.Sprintf("%v", actualValue)
	if actualStr != expectedStr {
		return fmt.Errorf("field_matches assertion failed for field '%s': expected %s, got %s", field, expectedStr, actualStr)
	}

	return nil
}

// processResultContainsAssertion validates that response contains expected data
func processResultContainsAssertion(response *MongoDBResponse, assertion map[string]interface{}, state map[string]string) error {
	expected, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for result_contains assertion")
	}

	// Replace variables in expected value
	expectedStr := replaceVariables(fmt.Sprintf("%v", expected), state)

	// Convert response data to string for search
	responseStr := fmt.Sprintf("%v", response.Data)

	if !contains(responseStr, expectedStr) {
		return fmt.Errorf("result_contains assertion failed: response does not contain %s", expectedStr)
	}

	return nil
}

// processJSONPathAssertion validates JSON path expressions
func processJSONPathAssertion(response *MongoDBResponse, assertion map[string]interface{}, state map[string]string) error {
	path, ok := assertion["path"].(string)
	if !ok {
		return fmt.Errorf("path is required for json_path assertion")
	}

	// Parse the JSON path using gojq
	query, err := gojq.Parse(path)
	if err != nil {
		return fmt.Errorf("failed to parse JSON path %s: %w", path, err)
	}

	// Run the query on the response
	iter := query.Run(response)
	actualValue, ok := iter.Next()
	if !ok {
		return fmt.Errorf("no results from JSON path %s", path)
	}
	if err, ok := actualValue.(error); ok {
		return fmt.Errorf("error evaluating JSON path %s: %w", path, err)
	}

	// Check if we just need to verify existence
	if _, hasExpected := assertion["expected"]; !hasExpected {
		// Just checking for existence - if we got here, it exists
		return nil
	}

	// Compare with expected value
	expected := assertion["expected"]
	expectedStr := replaceVariables(fmt.Sprintf("%v", expected), state)

	// Convert actual value to string for comparison
	actualStr := fmt.Sprintf("%v", actualValue)

	if actualStr != expectedStr {
		return fmt.Errorf("json_path assertion failed at %s: expected %s, got %s", path, expectedStr, actualStr)
	}

	return nil
}

// processErrorExistsAssertion validates error presence
func processErrorExistsAssertion(response *MongoDBResponse, assertion map[string]interface{}, state map[string]string) error {
	expected, ok := assertion["expected"].(bool)
	if !ok {
		return fmt.Errorf("expected boolean value is required for error_exists assertion")
	}

	hasError := response.Error != nil
	if hasError != expected {
		return fmt.Errorf("error_exists assertion failed: expected %t, got %t", expected, hasError)
	}

	return nil
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		len(s) > 0 && (s[0:len(substr)] == substr || 
		len(s) > len(substr) && contains(s[1:], substr)))
}