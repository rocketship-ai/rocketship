package kafka

import "time"

// KafkaPlugin represents a Kafka test step
type KafkaPlugin struct {
	Name       string            `json:"name" yaml:"name"`
	Plugin     string            `json:"plugin" yaml:"plugin"`
	Config     KafkaConfig       `json:"config" yaml:"config"`
	Assertions []KafkaAssertion  `json:"assertions" yaml:"assertions"`
	Save       []SaveConfig      `json:"save" yaml:"save,omitempty"`
}

// KafkaConfig contains the Kafka operation configuration
type KafkaConfig struct {
	// Connection settings
	Brokers           []string          `json:"brokers" yaml:"brokers"`
	SecurityProtocol  string            `json:"security_protocol" yaml:"security_protocol,omitempty"`  // PLAINTEXT, SASL_PLAINTEXT, SASL_SSL, SSL
	SASLMechanism     string            `json:"sasl_mechanism" yaml:"sasl_mechanism,omitempty"`        // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	Username          string            `json:"username" yaml:"username,omitempty"`
	Password          string            `json:"password" yaml:"password,omitempty"`
	TLSEnabled        bool              `json:"tls_enabled" yaml:"tls_enabled,omitempty"`
	TLSSkipVerify     bool              `json:"tls_skip_verify" yaml:"tls_skip_verify,omitempty"`
	
	// Operation type - one of: produce, consume, topic_info
	Operation         string            `json:"operation" yaml:"operation"`
	
	// Topic settings
	Topic             string            `json:"topic" yaml:"topic"`
	
	// Producer-specific settings
	Message           string            `json:"message" yaml:"message,omitempty"`
	Key               string            `json:"key" yaml:"key,omitempty"`
	Headers           map[string]string `json:"headers" yaml:"headers,omitempty"`
	Partition         *int32            `json:"partition" yaml:"partition,omitempty"`   // specific partition, nil for auto
	RequiredAcks      int16             `json:"required_acks" yaml:"required_acks,omitempty"` // 0=NoResponse, 1=WaitForLocal, -1=WaitForAll
	
	// Consumer-specific settings
	ConsumerGroup     string            `json:"consumer_group" yaml:"consumer_group,omitempty"`
	Offset            string            `json:"offset" yaml:"offset,omitempty"`         // oldest, newest, or specific offset
	TimeoutMs         int               `json:"timeout_ms" yaml:"timeout_ms,omitempty"` // consumer timeout in milliseconds
	MaxMessages       int               `json:"max_messages" yaml:"max_messages,omitempty"` // max messages to consume
	MessageFilter     MessageFilter     `json:"message_filter" yaml:"message_filter,omitempty"`
}

// MessageFilter defines filtering criteria for consumed messages
type MessageFilter struct {
	KeyContains     string            `json:"key_contains" yaml:"key_contains,omitempty"`
	ValueContains   string            `json:"value_contains" yaml:"value_contains,omitempty"`
	Headers         map[string]string `json:"headers" yaml:"headers,omitempty"`
	MinOffset       *int64            `json:"min_offset" yaml:"min_offset,omitempty"`
	MaxOffset       *int64            `json:"max_offset" yaml:"max_offset,omitempty"`
}

// KafkaAssertion represents a test assertion for Kafka operations
type KafkaAssertion struct {
	Type     string      `json:"type" yaml:"type"`           // assertion type
	Expected interface{} `json:"expected" yaml:"expected"`   // expected value
	Field    string      `json:"field" yaml:"field,omitempty"` // field to check (for message content assertions)
	Count    *int        `json:"count" yaml:"count,omitempty"` // expected count (for message_count assertions)
}

// SaveConfig represents a configuration for saving Kafka response data
type SaveConfig struct {
	MessageField string `json:"message_field" yaml:"message_field,omitempty"` // message field to extract (key, value, partition, offset, timestamp)
	MessageIndex int    `json:"message_index" yaml:"message_index,omitempty"` // index of message in consumed messages (0-based)
	TopicInfo    string `json:"topic_info" yaml:"topic_info,omitempty"`      // topic info field (partitions, replicas, config)
	As           string `json:"as" yaml:"as"`                                 // variable name to save as
	Required     *bool  `json:"required" yaml:"required,omitempty"`          // whether the value is required (defaults to true)
}

// Common assertion types
const (
	AssertionTypeMessageReceived  = "message_received"    // assert that at least one message was received
	AssertionTypeMessageCount     = "message_count"       // assert specific number of messages received
	AssertionTypeMessageKey       = "message_key"         // assert message key equals expected
	AssertionTypeMessageValue     = "message_value"       // assert message value equals expected
	AssertionTypeMessageHeader    = "message_header"      // assert message header equals expected
	AssertionTypePartition        = "partition"           // assert message was sent to expected partition
	AssertionTypeTopicExists      = "topic_exists"        // assert topic exists
	AssertionTypePartitionCount   = "partition_count"     // assert topic has expected number of partitions
)

// Common operations
const (
	OperationProduce   = "produce"
	OperationConsume   = "consume"  
	OperationTopicInfo = "topic_info"
)

// KafkaResponse represents the response from a Kafka operation
type KafkaResponse struct {
	Operation    string           `json:"operation"`
	Topic        string           `json:"topic"`
	Messages     []ConsumedMessage `json:"messages,omitempty"`      // for consume operations
	ProducedMessage *ProducedMessage `json:"produced_message,omitempty"` // for produce operations
	TopicInfo    *TopicInfo       `json:"topic_info,omitempty"`    // for topic_info operations
	Error        string           `json:"error,omitempty"`
}

// ConsumedMessage represents a message consumed from Kafka
type ConsumedMessage struct {
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Headers   map[string]string `json:"headers"`
	Partition int32             `json:"partition"`
	Offset    int64             `json:"offset"`
	Timestamp time.Time         `json:"timestamp"`
}

// ProducedMessage represents a message that was produced to Kafka
type ProducedMessage struct {
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Headers   map[string]string `json:"headers"`
	Partition int32             `json:"partition"`
	Offset    int64             `json:"offset"`
	Timestamp time.Time         `json:"timestamp"`
}

// TopicInfo represents information about a Kafka topic
type TopicInfo struct {
	Name       string                    `json:"name"`
	Partitions []PartitionInfo           `json:"partitions"`
	Config     map[string]string         `json:"config,omitempty"`
}

// PartitionInfo represents information about a topic partition
type PartitionInfo struct {
	ID       int32   `json:"id"`
	Leader   int32   `json:"leader"`
	Replicas []int32 `json:"replicas"`
	ISR      []int32 `json:"isr"` // In-Sync Replicas
}

// ActivityResponse represents the response from the Kafka activity
type ActivityResponse struct {
	Response *KafkaResponse    `json:"response"`
	Saved    map[string]string `json:"saved"`
}