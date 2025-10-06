package supabase

// parseConfig parses configuration from map to struct
func parseConfig(configData map[string]interface{}, config *SupabaseConfig) error {
	// Simple field mapping
	if url, ok := configData["url"].(string); ok {
		config.URL = url
	}
	if key, ok := configData["key"].(string); ok {
		config.Key = key
	}
	if operation, ok := configData["operation"].(string); ok {
		config.Operation = operation
	}
	if table, ok := configData["table"].(string); ok {
		config.Table = table
	}
	if timeout, ok := configData["timeout"].(string); ok {
		config.Timeout = timeout
	}

	// Parse operation-specific configs
	if selectData, ok := configData["select"].(map[string]interface{}); ok {
		config.Select = &SelectConfig{}
		parseSelectConfig(selectData, config.Select)
	}

	if insertData, ok := configData["insert"].(map[string]interface{}); ok {
		config.Insert = &InsertConfig{}
		parseInsertConfig(insertData, config.Insert)
	}

	if updateData, ok := configData["update"].(map[string]interface{}); ok {
		config.Update = &UpdateConfig{}
		parseUpdateConfig(updateData, config.Update)
	}

	if deleteData, ok := configData["delete"].(map[string]interface{}); ok {
		config.Delete = &DeleteConfig{}
		parseDeleteConfig(deleteData, config.Delete)
	}

	if rpcData, ok := configData["rpc"].(map[string]interface{}); ok {
		config.RPC = &RPCConfig{}
		parseRPCConfig(rpcData, config.RPC)
	}

	if authData, ok := configData["auth"].(map[string]interface{}); ok {
		config.Auth = &AuthConfig{}
		parseAuthConfig(authData, config.Auth)
	}

	if storageData, ok := configData["storage"].(map[string]interface{}); ok {
		config.Storage = &StorageConfig{}
		parseStorageConfig(storageData, config.Storage)
	}

	return nil
}

// parseSelectConfig parses select operation configuration
func parseSelectConfig(data map[string]interface{}, config *SelectConfig) {
	if columns, ok := data["columns"].([]interface{}); ok {
		config.Columns = make([]string, len(columns))
		for i, col := range columns {
			if colStr, ok := col.(string); ok {
				config.Columns[i] = colStr
			}
		}
	}

	if filters, ok := data["filters"].([]interface{}); ok {
		config.Filters = parseFilters(filters)
	}

	if order, ok := data["order"].([]interface{}); ok {
		config.Order = parseOrder(order)
	}

	if limit, ok := data["limit"].(float64); ok {
		limitInt := int(limit)
		config.Limit = &limitInt
	}

	if offset, ok := data["offset"].(float64); ok {
		offsetInt := int(offset)
		config.Offset = &offsetInt
	}

	if count, ok := data["count"].(string); ok {
		config.Count = count
	}
}

// parseInsertConfig parses insert operation configuration
func parseInsertConfig(data map[string]interface{}, config *InsertConfig) {
	if dataField, ok := data["data"]; ok {
		config.Data = dataField
	}
	if upsert, ok := data["upsert"].(bool); ok {
		config.Upsert = upsert
	}
	if onConflict, ok := data["on_conflict"].(string); ok {
		config.OnConflict = onConflict
	}
}

// parseUpdateConfig parses update operation configuration
func parseUpdateConfig(data map[string]interface{}, config *UpdateConfig) {
	if dataField, ok := data["data"].(map[string]interface{}); ok {
		config.Data = dataField
	}
	if filters, ok := data["filters"].([]interface{}); ok {
		config.Filters = parseFilters(filters)
	}
}

// parseDeleteConfig parses delete operation configuration
func parseDeleteConfig(data map[string]interface{}, config *DeleteConfig) {
	if filters, ok := data["filters"].([]interface{}); ok {
		config.Filters = parseFilters(filters)
	}
}

// parseRPCConfig parses RPC operation configuration
func parseRPCConfig(data map[string]interface{}, config *RPCConfig) {
	if function, ok := data["function"].(string); ok {
		config.Function = function
	}
	if params, ok := data["params"].(map[string]interface{}); ok {
		config.Params = params
	}
}

// parseAuthConfig parses auth operation configuration
func parseAuthConfig(data map[string]interface{}, config *AuthConfig) {
	if email, ok := data["email"].(string); ok {
		config.Email = email
	}
	if password, ok := data["password"].(string); ok {
		config.Password = password
	}
	if userID, ok := data["user_id"].(string); ok {
		config.UserID = userID
	}
	if emailConfirm, ok := data["email_confirm"].(bool); ok {
		config.EmailConfirm = emailConfirm
	}
	if userMetadata, ok := data["user_metadata"].(map[string]interface{}); ok {
		config.UserMetadata = userMetadata
	}
	if appMetadata, ok := data["app_metadata"].(map[string]interface{}); ok {
		config.AppMetadata = appMetadata
	}
}

// parseStorageConfig parses storage operation configuration
func parseStorageConfig(data map[string]interface{}, config *StorageConfig) {
	if bucket, ok := data["bucket"].(string); ok {
		config.Bucket = bucket
	}
	if path, ok := data["path"].(string); ok {
		config.Path = path
	}
	if fileContent, ok := data["file_content"].(string); ok {
		config.FileContent = fileContent
	}
	if filePath, ok := data["file_path"].(string); ok {
		config.FilePath = filePath
	}
	if public, ok := data["public"].(bool); ok {
		config.Public = public
	}
	if cacheControl, ok := data["cache_control"].(string); ok {
		config.CacheControl = cacheControl
	}
	if contentType, ok := data["content_type"].(string); ok {
		config.ContentType = contentType
	}
}

// parseFilters parses filter configurations
func parseFilters(filters []interface{}) []FilterConfig {
	result := make([]FilterConfig, 0, len(filters))
	for _, filterInterface := range filters {
		if filterMap, ok := filterInterface.(map[string]interface{}); ok {
			filter := FilterConfig{}
			if column, ok := filterMap["column"].(string); ok {
				filter.Column = column
			}
			if operator, ok := filterMap["operator"].(string); ok {
				filter.Operator = operator
			}
			if value, ok := filterMap["value"]; ok {
				filter.Value = value
			}
			result = append(result, filter)
		}
	}
	return result
}

// parseOrder parses order configurations
func parseOrder(order []interface{}) []OrderConfig {
	result := make([]OrderConfig, 0, len(order))
	for _, orderInterface := range order {
		if orderMap, ok := orderInterface.(map[string]interface{}); ok {
			orderConfig := OrderConfig{Ascending: true} // default
			if column, ok := orderMap["column"].(string); ok {
				orderConfig.Column = column
			}
			if ascending, ok := orderMap["ascending"].(bool); ok {
				orderConfig.Ascending = ascending
			}
			result = append(result, orderConfig)
		}
	}
	return result
}
