package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"go.temporal.io/sdk/activity"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&KafkaPlugin{})
}

// replaceVariables replaces {{ variable }} patterns in the input string with values from the state
// Uses DSL template functions to properly handle escaped handlebars
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

// extractMissingVars extracts variable names from template execution errors
func extractMissingVars(err error) []string {
	// For now, just return the error string
	// TODO: Parse Go template errors more intelligently
	return []string{err.Error()}
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

func (kp *KafkaPlugin) GetType() string {
	return "kafka"
}

func (kp *KafkaPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("[DEBUG] Kafka Activity input parameters:", "params", fmt.Sprintf("%+v", p))

	// Get state from input parameters with improved type handling
	state := make(map[string]string)
	if stateData, ok := p["state"].(map[string]interface{}); ok {
		for k, v := range stateData {
			switch val := v.(type) {
			case string:
				state[k] = val
			case float64:
				state[k] = fmt.Sprintf("%.0f", val) // Remove decimal for whole numbers
			case bool:
				state[k] = fmt.Sprintf("%t", val)
			case nil:
				state[k] = ""
			default:
				// For complex types, use JSON marshaling
				bytes, err := json.Marshal(val)
				if err != nil {
					return nil, fmt.Errorf("failed to convert state value for %s: %w", k, err)
				}
				state[k] = string(bytes)
			}
			logger.Info(fmt.Sprintf("[DEBUG] Loaded state[%s] = %s (type: %T)", k, state[k], v))
		}
	}
	logger.Info(fmt.Sprintf("[DEBUG] Loaded state: %v", state))

	// Parse the plugin configuration
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format: got type %T", p["config"])
	}

	// Parse and validate the configuration
	config, err := kp.parseConfig(configData, state)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	logger.Info("[DEBUG] Parsed Kafka config:", "config", fmt.Sprintf("%+v", config))

	// Execute the Kafka operation
	response, err := kp.executeOperation(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Kafka operation: %w", err)
	}

	// Process assertions
	if err := kp.processAssertions(p, response, state); err != nil {
		return nil, err
	}

	// Process saves
	saved := make(map[string]string)
	if err := kp.processSaves(p, response, saved); err != nil {
		return nil, err
	}

	return &ActivityResponse{
		Response: response,
		Saved:    saved,
	}, nil
}

func (kp *KafkaPlugin) parseConfig(configData map[string]interface{}, state map[string]string) (*KafkaConfig, error) {
	config := &KafkaConfig{}
	
	// Parse brokers (required)
	brokersRaw, ok := configData["brokers"]
	if !ok {
		return nil, fmt.Errorf("brokers are required")
	}
	
	switch brokers := brokersRaw.(type) {
	case []interface{}:
		config.Brokers = make([]string, len(brokers))
		for i, broker := range brokers {
			if brokerStr, ok := broker.(string); ok {
				processedBroker, err := replaceVariables(brokerStr, state)
				if err != nil {
					return nil, fmt.Errorf("failed to replace variables in broker %s: %w", brokerStr, err)
				}
				config.Brokers[i] = processedBroker
			} else {
				return nil, fmt.Errorf("broker must be a string, got %T", broker)
			}
		}
	case []string:
		config.Brokers = make([]string, len(brokers))
		for i, broker := range brokers {
			processedBroker, err := replaceVariables(broker, state)
			if err != nil {
				return nil, fmt.Errorf("failed to replace variables in broker %s: %w", broker, err)
			}
			config.Brokers[i] = processedBroker
		}
	default:
		return nil, fmt.Errorf("brokers must be an array of strings, got %T", brokersRaw)
	}

	// Parse operation (required)
	operationRaw, ok := configData["operation"]
	if !ok {
		return nil, fmt.Errorf("operation is required")
	}
	if operationStr, ok := operationRaw.(string); ok {
		config.Operation = operationStr
	} else {
		return nil, fmt.Errorf("operation must be a string, got %T", operationRaw)
	}

	// Validate operation
	switch config.Operation {
	case OperationProduce, OperationConsume, OperationTopicInfo:
		// valid operations
	default:
		return nil, fmt.Errorf("invalid operation: %s. Must be one of: %s, %s, %s", 
			config.Operation, OperationProduce, OperationConsume, OperationTopicInfo)
	}

	// Parse topic (required)
	topicRaw, ok := configData["topic"]
	if !ok {
		return nil, fmt.Errorf("topic is required")
	}
	if topicStr, ok := topicRaw.(string); ok {
		processedTopic, err := replaceVariables(topicStr, state)
		if err != nil {
			return nil, fmt.Errorf("failed to replace variables in topic: %w", err)
		}
		config.Topic = processedTopic
	} else {
		return nil, fmt.Errorf("topic must be a string, got %T", topicRaw)
	}

	// Parse optional string fields with variable replacement
	stringFields := map[string]*string{
		"security_protocol": &config.SecurityProtocol,
		"sasl_mechanism":    &config.SASLMechanism,
		"username":          &config.Username,
		"password":          &config.Password,
		"message":           &config.Message,
		"key":               &config.Key,
		"consumer_group":    &config.ConsumerGroup,
		"offset":            &config.Offset,
	}

	for fieldName, fieldPtr := range stringFields {
		if raw, exists := configData[fieldName]; exists {
			if str, ok := raw.(string); ok {
				processed, err := replaceVariables(str, state)
				if err != nil {
					return nil, fmt.Errorf("failed to replace variables in %s: %w", fieldName, err)
				}
				*fieldPtr = processed
			} else {
				return nil, fmt.Errorf("%s must be a string, got %T", fieldName, raw)
			}
		}
	}

	// Parse boolean fields
	if raw, exists := configData["tls_enabled"]; exists {
		if b, ok := raw.(bool); ok {
			config.TLSEnabled = b
		} else {
			return nil, fmt.Errorf("tls_enabled must be a boolean, got %T", raw)
		}
	}

	if raw, exists := configData["tls_skip_verify"]; exists {
		if b, ok := raw.(bool); ok {
			config.TLSSkipVerify = b
		} else {
			return nil, fmt.Errorf("tls_skip_verify must be a boolean, got %T", raw)
		}
	}

	// Parse numeric fields
	if raw, exists := configData["partition"]; exists {
		if f, ok := raw.(float64); ok {
			partition := int32(f)
			config.Partition = &partition
		} else {
			return nil, fmt.Errorf("partition must be a number, got %T", raw)
		}
	}

	if raw, exists := configData["required_acks"]; exists {
		if f, ok := raw.(float64); ok {
			config.RequiredAcks = int16(f)
		} else {
			return nil, fmt.Errorf("required_acks must be a number, got %T", raw)
		}
	}

	if raw, exists := configData["timeout_ms"]; exists {
		if f, ok := raw.(float64); ok {
			config.TimeoutMs = int(f)
		} else {
			return nil, fmt.Errorf("timeout_ms must be a number, got %T", raw)
		}
	}

	if raw, exists := configData["max_messages"]; exists {
		if f, ok := raw.(float64); ok {
			config.MaxMessages = int(f)
		} else {
			return nil, fmt.Errorf("max_messages must be a number, got %T", raw)
		}
	}

	// Parse headers map with variable replacement
	if raw, exists := configData["headers"]; exists {
		if headersMap, ok := raw.(map[string]interface{}); ok {
			config.Headers = make(map[string]string)
			for key, value := range headersMap {
				if valueStr, ok := value.(string); ok {
					processedValue, err := replaceVariables(valueStr, state)
					if err != nil {
						return nil, fmt.Errorf("failed to replace variables in header %s: %w", key, err)
					}
					config.Headers[key] = processedValue
				} else {
					return nil, fmt.Errorf("header %s must be a string, got %T", key, value)
				}
			}
		} else {
			return nil, fmt.Errorf("headers must be a map, got %T", raw)
		}
	}

	// Parse message filter
	if raw, exists := configData["message_filter"]; exists {
		if filterMap, ok := raw.(map[string]interface{}); ok {
			filter := MessageFilter{}
			
			// Parse filter string fields
			filterStringFields := map[string]*string{
				"key_contains":   &filter.KeyContains,
				"value_contains": &filter.ValueContains,
			}
			
			for fieldName, fieldPtr := range filterStringFields {
				if fraw, exists := filterMap[fieldName]; exists {
					if str, ok := fraw.(string); ok {
						processed, err := replaceVariables(str, state)
						if err != nil {
							return nil, fmt.Errorf("failed to replace variables in message_filter.%s: %w", fieldName, err)
						}
						*fieldPtr = processed
					} else {
						return nil, fmt.Errorf("message_filter.%s must be a string, got %T", fieldName, fraw)
					}
				}
			}
			
			// Parse filter headers
			if raw, exists := filterMap["headers"]; exists {
				if headersMap, ok := raw.(map[string]interface{}); ok {
					filter.Headers = make(map[string]string)
					for key, value := range headersMap {
						if valueStr, ok := value.(string); ok {
							processedValue, err := replaceVariables(valueStr, state)
							if err != nil {
								return nil, fmt.Errorf("failed to replace variables in message_filter.headers.%s: %w", key, err)
							}
							filter.Headers[key] = processedValue
						} else {
							return nil, fmt.Errorf("message_filter.headers.%s must be a string, got %T", key, value)
						}
					}
				} else {
					return nil, fmt.Errorf("message_filter.headers must be a map, got %T", raw)
				}
			}
			
			// Parse offset fields
			if raw, exists := filterMap["min_offset"]; exists {
				if f, ok := raw.(float64); ok {
					offset := int64(f)
					filter.MinOffset = &offset
				} else {
					return nil, fmt.Errorf("message_filter.min_offset must be a number, got %T", raw)
				}
			}
			
			if raw, exists := filterMap["max_offset"]; exists {
				if f, ok := raw.(float64); ok {
					offset := int64(f)
					filter.MaxOffset = &offset
				} else {
					return nil, fmt.Errorf("message_filter.max_offset must be a number, got %T", raw)
				}
			}
			
			config.MessageFilter = filter
		} else {
			return nil, fmt.Errorf("message_filter must be a map, got %T", raw)
		}
	}

	return config, nil
}

func (kp *KafkaPlugin) executeOperation(ctx context.Context, config *KafkaConfig) (*KafkaResponse, error) {
	// Create Sarama config
	saramaConfig := sarama.NewConfig()
	
	// Configure authentication
	if config.SecurityProtocol != "" {
		switch strings.ToUpper(config.SecurityProtocol) {
		case "SASL_PLAINTEXT":
			saramaConfig.Net.SASL.Enable = true
			if config.Username != "" {
				saramaConfig.Net.SASL.User = config.Username
			}
			if config.Password != "" {
				saramaConfig.Net.SASL.Password = config.Password
			}
			
			switch strings.ToUpper(config.SASLMechanism) {
			case "PLAIN", "":
				saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
			case "SCRAM-SHA-256":
				saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			case "SCRAM-SHA-512":
				saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			default:
				return nil, fmt.Errorf("unsupported SASL mechanism: %s", config.SASLMechanism)
			}
			
		case "SASL_SSL":
			saramaConfig.Net.SASL.Enable = true
			saramaConfig.Net.TLS.Enable = true
			if config.Username != "" {
				saramaConfig.Net.SASL.User = config.Username
			}
			if config.Password != "" {
				saramaConfig.Net.SASL.Password = config.Password
			}
			
			switch strings.ToUpper(config.SASLMechanism) {
			case "PLAIN", "":
				saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
			case "SCRAM-SHA-256":
				saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			case "SCRAM-SHA-512":
				saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			default:
				return nil, fmt.Errorf("unsupported SASL mechanism: %s", config.SASLMechanism)
			}
			
			if config.TLSSkipVerify {
				saramaConfig.Net.TLS.Config = &tls.Config{InsecureSkipVerify: true}
			}
			
		case "SSL":
			saramaConfig.Net.TLS.Enable = true
			if config.TLSSkipVerify {
				saramaConfig.Net.TLS.Config = &tls.Config{InsecureSkipVerify: true}
			}
		}
	} else if config.TLSEnabled {
		saramaConfig.Net.TLS.Enable = true
		if config.TLSSkipVerify {
			saramaConfig.Net.TLS.Config = &tls.Config{InsecureSkipVerify: true}
		}
	}

	// Set consumer return errors
	saramaConfig.Consumer.Return.Errors = true

	switch config.Operation {
	case OperationProduce:
		return kp.executeProduceOperation(ctx, config, saramaConfig)
	case OperationConsume:
		return kp.executeConsumeOperation(ctx, config, saramaConfig)
	case OperationTopicInfo:
		return kp.executeTopicInfoOperation(ctx, config, saramaConfig)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", config.Operation)
	}
}

func (kp *KafkaPlugin) executeProduceOperation(ctx context.Context, config *KafkaConfig, saramaConfig *sarama.Config) (*KafkaResponse, error) {
	// Configure producer
	saramaConfig.Producer.RequiredAcks = sarama.RequiredAcks(config.RequiredAcks)
	saramaConfig.Producer.Retry.Max = 3
	saramaConfig.Producer.Return.Successes = true

	// Create producer
	producer, err := sarama.NewSyncProducer(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}
	defer func() { _ = producer.Close() }()

	// Prepare message
	message := &sarama.ProducerMessage{
		Topic: config.Topic,
		Value: sarama.StringEncoder(config.Message),
	}

	if config.Key != "" {
		message.Key = sarama.StringEncoder(config.Key)
	}

	if config.Partition != nil {
		message.Partition = *config.Partition
	}

	// Add headers
	if len(config.Headers) > 0 {
		headers := make([]sarama.RecordHeader, 0, len(config.Headers))
		for key, value := range config.Headers {
			headers = append(headers, sarama.RecordHeader{
				Key:   []byte(key),
				Value: []byte(value),
			})
		}
		message.Headers = headers
	}

	// Send message
	partition, offset, err := producer.SendMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	response := &KafkaResponse{
		Operation: OperationProduce,
		Topic:     config.Topic,
		ProducedMessage: &ProducedMessage{
			Key:       config.Key,
			Value:     config.Message,
			Headers:   config.Headers,
			Partition: partition,
			Offset:    offset,
			Timestamp: time.Now(),
		},
	}

	return response, nil
}

func (kp *KafkaPlugin) executeConsumeOperation(ctx context.Context, config *KafkaConfig, saramaConfig *sarama.Config) (*KafkaResponse, error) {
	// Configure consumer
	saramaConfig.Consumer.Return.Errors = true
	
	// Set consumer offset
	switch strings.ToLower(config.Offset) {
	case "oldest", "earliest":
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	case "newest", "latest", "":
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	default:
		// Try to parse as specific offset
		if offset, err := strconv.ParseInt(config.Offset, 10, 64); err == nil {
			saramaConfig.Consumer.Offsets.Initial = offset
		} else {
			return nil, fmt.Errorf("invalid offset value: %s", config.Offset)
		}
	}

	// Create consumer
	consumer, err := sarama.NewConsumer(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}
	defer func() { _ = consumer.Close() }()

	// Get topic partitions
	partitions, err := consumer.Partitions(config.Topic)
	if err != nil {
		return nil, fmt.Errorf("failed to get partitions for topic %s: %w", config.Topic, err)
	}

	// Set defaults
	maxMessages := config.MaxMessages
	if maxMessages <= 0 {
		maxMessages = 10 // default
	}
	
	timeout := time.Duration(config.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second // default
	}

	var messages []ConsumedMessage
	messageChan := make(chan ConsumedMessage, maxMessages)
	errorChan := make(chan error, len(partitions))

	// Start consumers for all partitions
	for _, partition := range partitions {
		go func(partition int32) {
			pc, err := consumer.ConsumePartition(config.Topic, partition, saramaConfig.Consumer.Offsets.Initial)
			if err != nil {
				errorChan <- fmt.Errorf("failed to start consumer for partition %d: %w", partition, err)
				return
			}
			defer func() { _ = pc.Close() }()

			for {
				select {
				case msg := <-pc.Messages():
					if msg != nil {
						consumedMsg := ConsumedMessage{
							Key:       string(msg.Key),
							Value:     string(msg.Value),
							Headers:   make(map[string]string),
							Partition: msg.Partition,
							Offset:    msg.Offset,
							Timestamp: msg.Timestamp,
						}

						// Extract headers
						for _, header := range msg.Headers {
							consumedMsg.Headers[string(header.Key)] = string(header.Value)
						}

						// Apply filter
						if kp.messageMatchesFilter(consumedMsg, config.MessageFilter) {
							select {
							case messageChan <- consumedMsg:
							case <-ctx.Done():
								return
							}
						}
					}
				case err := <-pc.Errors():
					if err != nil {
						errorChan <- fmt.Errorf("consumer error on partition %d: %w", partition, err)
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}(partition)
	}

	// Collect messages with timeout
	timeoutChan := time.After(timeout)
	
collectLoop:
	for len(messages) < maxMessages {
		select {
		case msg := <-messageChan:
			messages = append(messages, msg)
		case err := <-errorChan:
			return nil, err
		case <-timeoutChan:
			break collectLoop
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	response := &KafkaResponse{
		Operation: OperationConsume,
		Topic:     config.Topic,
		Messages:  messages,
	}

	return response, nil
}

func (kp *KafkaPlugin) messageMatchesFilter(msg ConsumedMessage, filter MessageFilter) bool {
	// Check key contains
	if filter.KeyContains != "" && !strings.Contains(msg.Key, filter.KeyContains) {
		return false
	}

	// Check value contains
	if filter.ValueContains != "" && !strings.Contains(msg.Value, filter.ValueContains) {
		return false
	}

	// Check headers
	for filterKey, filterValue := range filter.Headers {
		if msgValue, exists := msg.Headers[filterKey]; !exists || msgValue != filterValue {
			return false
		}
	}

	// Check offset range
	if filter.MinOffset != nil && msg.Offset < *filter.MinOffset {
		return false
	}
	
	if filter.MaxOffset != nil && msg.Offset > *filter.MaxOffset {
		return false
	}

	return true
}

func (kp *KafkaPlugin) executeTopicInfoOperation(ctx context.Context, config *KafkaConfig, saramaConfig *sarama.Config) (*KafkaResponse, error) {
	// Create cluster admin client
	admin, err := sarama.NewClusterAdmin(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer func() { _ = admin.Close() }()

	// Get topic metadata
	metadata, err := admin.DescribeTopics([]string{config.Topic})
	if err != nil {
		return nil, fmt.Errorf("failed to describe topic: %w", err)
	}

	if len(metadata) == 0 {
		return nil, fmt.Errorf("topic %s does not exist", config.Topic)
	}
	
	topicMetadata := metadata[0]

	// Build topic info
	topicInfo := &TopicInfo{
		Name:       config.Topic,
		Partitions: make([]PartitionInfo, len(topicMetadata.Partitions)),
	}

	for i, partition := range topicMetadata.Partitions {
		topicInfo.Partitions[i] = PartitionInfo{
			ID:       partition.ID,
			Leader:   partition.Leader,
			Replicas: partition.Replicas,
			ISR:      partition.Isr,
		}
	}

	// Get topic configuration
	configResource := sarama.ConfigResource{
		Type: sarama.TopicResource,
		Name: config.Topic,
	}
	configEntries, err := admin.DescribeConfig(configResource)
	if err == nil && len(configEntries) > 0 {
		topicInfo.Config = make(map[string]string)
		for _, entry := range configEntries {
			topicInfo.Config[entry.Name] = entry.Value
		}
	}

	response := &KafkaResponse{
		Operation: OperationTopicInfo,
		Topic:     config.Topic,
		TopicInfo: topicInfo,
	}

	return response, nil
}

func (kp *KafkaPlugin) processAssertions(p map[string]interface{}, response *KafkaResponse, state map[string]string) error {
	assertions, ok := p["assertions"].([]interface{})
	if !ok {
		return nil
	}

	for _, assertion := range assertions {
		assertionMap, ok := assertion.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid assertion format: got type %T", assertion)
		}

		assertionType, ok := assertionMap["type"].(string)
		if !ok {
			return fmt.Errorf("assertion type is required")
		}

		// Replace variables in expected value if it's a string
		if expectedStr, ok := assertionMap["expected"].(string); ok {
			expectedStr, err := replaceVariables(expectedStr, state)
			if err != nil {
				return fmt.Errorf("failed to replace variables in expected value: %w", err)
			}
			assertionMap["expected"] = expectedStr
		}

		switch assertionType {
		case AssertionTypeMessageReceived:
			if response.Operation != OperationConsume {
				return fmt.Errorf("message_received assertion only valid for consume operations")
			}
			if len(response.Messages) == 0 {
				return fmt.Errorf("message_received assertion failed: no messages received")
			}

		case AssertionTypeMessageCount:
			if response.Operation != OperationConsume {
				return fmt.Errorf("message_count assertion only valid for consume operations")
			}
			expected, ok := assertionMap["expected"].(float64)
			if !ok {
				return fmt.Errorf("message_count assertion expected value must be a number: got type %T", assertionMap["expected"])
			}
			if len(response.Messages) != int(expected) {
				return fmt.Errorf("message_count assertion failed: expected %d messages, got %d", int(expected), len(response.Messages))
			}

		case AssertionTypeMessageKey:
			if response.Operation != OperationConsume {
				return fmt.Errorf("message_key assertion only valid for consume operations")
			}
			if len(response.Messages) == 0 {
				return fmt.Errorf("message_key assertion failed: no messages received")
			}
			expected, ok := assertionMap["expected"].(string)
			if !ok {
				return fmt.Errorf("message_key assertion expected value must be a string")
			}
			// Check first message by default, or use specific index
			msgIndex := 0
			if idx, exists := assertionMap["message_index"]; exists {
				if f, ok := idx.(float64); ok {
					msgIndex = int(f)
				}
			}
			if msgIndex >= len(response.Messages) {
				return fmt.Errorf("message_key assertion failed: message index %d out of range (have %d messages)", msgIndex, len(response.Messages))
			}
			if response.Messages[msgIndex].Key != expected {
				return fmt.Errorf("message_key assertion failed: expected %q, got %q", expected, response.Messages[msgIndex].Key)
			}

		case AssertionTypeMessageValue:
			if response.Operation != OperationConsume {
				return fmt.Errorf("message_value assertion only valid for consume operations")
			}
			if len(response.Messages) == 0 {
				return fmt.Errorf("message_value assertion failed: no messages received")
			}
			expected, ok := assertionMap["expected"].(string)
			if !ok {
				return fmt.Errorf("message_value assertion expected value must be a string")
			}
			// Check first message by default, or use specific index
			msgIndex := 0
			if idx, exists := assertionMap["message_index"]; exists {
				if f, ok := idx.(float64); ok {
					msgIndex = int(f)
				}
			}
			if msgIndex >= len(response.Messages) {
				return fmt.Errorf("message_value assertion failed: message index %d out of range (have %d messages)", msgIndex, len(response.Messages))
			}
			if response.Messages[msgIndex].Value != expected {
				return fmt.Errorf("message_value assertion failed: expected %q, got %q", expected, response.Messages[msgIndex].Value)
			}

		case AssertionTypeMessageHeader:
			if response.Operation != OperationConsume {
				return fmt.Errorf("message_header assertion only valid for consume operations")
			}
			if len(response.Messages) == 0 {
				return fmt.Errorf("message_header assertion failed: no messages received")
			}
			headerName, ok := assertionMap["field"].(string)
			if !ok {
				return fmt.Errorf("message_header assertion requires field name")
			}
			expected, ok := assertionMap["expected"].(string)
			if !ok {
				return fmt.Errorf("message_header assertion expected value must be a string")
			}
			// Check first message by default, or use specific index
			msgIndex := 0
			if idx, exists := assertionMap["message_index"]; exists {
				if f, ok := idx.(float64); ok {
					msgIndex = int(f)
				}
			}
			if msgIndex >= len(response.Messages) {
				return fmt.Errorf("message_header assertion failed: message index %d out of range (have %d messages)", msgIndex, len(response.Messages))
			}
			actual, exists := response.Messages[msgIndex].Headers[headerName]
			if !exists {
				return fmt.Errorf("message_header assertion failed: header %q not found", headerName)
			}
			if actual != expected {
				return fmt.Errorf("message_header assertion failed for %q: expected %q, got %q", headerName, expected, actual)
			}

		case AssertionTypePartition:
			expected, ok := assertionMap["expected"].(float64)
			if !ok {
				return fmt.Errorf("partition assertion expected value must be a number")
			}
			expectedPartition := int32(expected)
			
			switch response.Operation {
			case OperationProduce:
				if response.ProducedMessage.Partition != expectedPartition {
					return fmt.Errorf("partition assertion failed: expected partition %d, got %d", expectedPartition, response.ProducedMessage.Partition)
				}
			case OperationConsume:
				if len(response.Messages) == 0 {
					return fmt.Errorf("partition assertion failed: no messages received")
				}
				// Check first message by default, or use specific index
				msgIndex := 0
				if idx, exists := assertionMap["message_index"]; exists {
					if f, ok := idx.(float64); ok {
						msgIndex = int(f)
					}
				}
				if msgIndex >= len(response.Messages) {
					return fmt.Errorf("partition assertion failed: message index %d out of range (have %d messages)", msgIndex, len(response.Messages))
				}
				if response.Messages[msgIndex].Partition != expectedPartition {
					return fmt.Errorf("partition assertion failed: expected partition %d, got %d", expectedPartition, response.Messages[msgIndex].Partition)
				}
			default:
				return fmt.Errorf("partition assertion not valid for operation %s", response.Operation)
			}

		case AssertionTypeTopicExists:
			if response.Operation != OperationTopicInfo {
				return fmt.Errorf("topic_exists assertion only valid for topic_info operations")
			}
			if response.TopicInfo == nil {
				return fmt.Errorf("topic_exists assertion failed: topic does not exist")
			}

		case AssertionTypePartitionCount:
			if response.Operation != OperationTopicInfo {
				return fmt.Errorf("partition_count assertion only valid for topic_info operations")
			}
			expected, ok := assertionMap["expected"].(float64)
			if !ok {
				return fmt.Errorf("partition_count assertion expected value must be a number")
			}
			if response.TopicInfo == nil {
				return fmt.Errorf("partition_count assertion failed: topic info not available")
			}
			if len(response.TopicInfo.Partitions) != int(expected) {
				return fmt.Errorf("partition_count assertion failed: expected %d partitions, got %d", int(expected), len(response.TopicInfo.Partitions))
			}

		default:
			return fmt.Errorf("unknown assertion type: %s", assertionType)
		}
	}

	return nil
}

func (kp *KafkaPlugin) processSaves(p map[string]interface{}, response *KafkaResponse, saved map[string]string) error {
	saves, ok := p["save"].([]interface{})
	if !ok {
		log.Printf("[DEBUG] No saves configured")
		return nil
	}

	log.Printf("[DEBUG] Processing %d saves", len(saves))
	for _, save := range saves {
		saveMap, ok := save.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid save format: got type %T", save)
		}

		as, ok := saveMap["as"].(string)
		if !ok {
			return fmt.Errorf("'as' field is required for save")
		}

		// Check if required is explicitly set to false
		required := true
		if req, ok := saveMap["required"].(bool); ok {
			required = req
		}

		// Handle message field save
		if messageField, ok := saveMap["message_field"].(string); ok {
			log.Printf("[DEBUG] Processing message field save: '%s' as %s", messageField, as)
			
			if response.Operation != OperationConsume && response.Operation != OperationProduce {
				if required {
					return fmt.Errorf("message_field save only valid for produce/consume operations")
				}
				continue
			}

			// Get message index (default to 0)
			msgIndex := 0
			if idx, exists := saveMap["message_index"]; exists {
				if f, ok := idx.(float64); ok {
					msgIndex = int(f)
				}
			}

			var value string
			var found bool

			if response.Operation == OperationProduce && response.ProducedMessage != nil {
				switch messageField {
				case "key":
					value = response.ProducedMessage.Key
					found = true
				case "value":
					value = response.ProducedMessage.Value
					found = true
				case "partition":
					value = fmt.Sprintf("%d", response.ProducedMessage.Partition)
					found = true
				case "offset":
					value = fmt.Sprintf("%d", response.ProducedMessage.Offset)
					found = true
				case "timestamp":
					value = response.ProducedMessage.Timestamp.Format(time.RFC3339)
					found = true
				}
			} else if response.Operation == OperationConsume && len(response.Messages) > msgIndex {
				msg := response.Messages[msgIndex]
				switch messageField {
				case "key":
					value = msg.Key
					found = true
				case "value":
					value = msg.Value
					found = true
				case "partition":
					value = fmt.Sprintf("%d", msg.Partition)
					found = true
				case "offset":
					value = fmt.Sprintf("%d", msg.Offset)
					found = true
				case "timestamp":
					value = msg.Timestamp.Format(time.RFC3339)
					found = true
				default:
					// Check if it's a header
					if headerValue, exists := msg.Headers[messageField]; exists {
						value = headerValue
						found = true
					}
				}
			}

			if !found {
				if required {
					return fmt.Errorf("required message field %s not found", messageField)
				}
				continue
			}

			saved[as] = value
			log.Printf("[DEBUG] Saved message field %s as %s: %s", messageField, as, value)
			continue
		}

		// Handle topic info save
		if topicInfoField, ok := saveMap["topic_info"].(string); ok {
			log.Printf("[DEBUG] Processing topic info save: '%s' as %s", topicInfoField, as)
			
			if response.Operation != OperationTopicInfo {
				if required {
					return fmt.Errorf("topic_info save only valid for topic_info operations")
				}
				continue
			}

			if response.TopicInfo == nil {
				if required {
					return fmt.Errorf("topic info not available")
				}
				continue
			}

			var value string
			var found bool

			switch topicInfoField {
			case "name":
				value = response.TopicInfo.Name
				found = true
			case "partition_count":
				value = fmt.Sprintf("%d", len(response.TopicInfo.Partitions))
				found = true
			case "partitions":
				bytes, err := json.Marshal(response.TopicInfo.Partitions)
				if err != nil {
					return fmt.Errorf("failed to marshal partitions: %w", err)
				}
				value = string(bytes)
				found = true
			case "config":
				bytes, err := json.Marshal(response.TopicInfo.Config)
				if err != nil {
					return fmt.Errorf("failed to marshal config: %w", err)
				}
				value = string(bytes)
				found = true
			default:
				// Check if it's a specific config key
				if configValue, exists := response.TopicInfo.Config[topicInfoField]; exists {
					value = configValue
					found = true
				}
			}

			if !found {
				if required {
					return fmt.Errorf("required topic info field %s not found", topicInfoField)
				}
				continue
			}

			saved[as] = value
			log.Printf("[DEBUG] Saved topic info field %s as %s: %s", topicInfoField, as, value)
			continue
		}

		return fmt.Errorf("save configuration must specify either message_field or topic_info")
	}

	log.Printf("[DEBUG] Final saved values: %v", saved)
	return nil
}