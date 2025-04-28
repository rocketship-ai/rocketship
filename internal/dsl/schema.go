package dsl

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
	yaml "gopkg.in/yaml.v3"
)

func GetJSONSchema() string {
	return `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"required": ["version", "tests"],
		"properties": {
			"version": {
				"type": "integer",
				"enum": [1]
			},
			"tests": {
				"type": "array",
				"items": {
					"$ref": "#/definitions/test"
				},
				"minItems": 1
			}
		},
		"definitions": {
			"test": {
				"type": "object",
				"required": ["name", "steps"],
				"properties": {
					"name": {
						"type": "string",
						"minLength": 1
					},
					"steps": {
						"type": "array",
						"items": {
							"$ref": "#/definitions/step"
						},
						"minItems": 1
					}
				}
			},
			"step": {
				"type": "object",
				"required": ["op"],
				"properties": {
					"op": {
						"type": "string",
						"enum": ["http.send", "http.expect", "aws.s3.get", "aws.s3.exists", "aws.ddb.query", "aws.sqs.send", "sleep"]
					},
					"params": {
						"type": "object"
					},
					"expect": {
						"type": "object"
					},
					"save": {
						"type": "object",
						"required": ["jsonPath", "as"],
						"properties": {
							"jsonPath": {
								"type": "string"
							},
							"as": {
								"type": "string",
								"pattern": "^[a-zA-Z][a-zA-Z0-9_]*$"
							}
						}
					},
					"duration": {
						"type": "string",
						"pattern": "^[0-9]+(ns|us|µs|ms|s|m|h)$"
					}
				},
				"allOf": [
					{
						"if": {
							"properties": {
								"op": {
									"enum": ["sleep"]
								}
							}
						},
						"then": {
							"required": ["duration"]
						}
					},
					{
						"if": {
							"properties": {
								"op": {
									"enum": ["http.send"]
								}
							}
						},
						"then": {
							"required": ["params"],
							"properties": {
								"params": {
									"required": ["method", "url"]
								}
							}
						}
					},
					{
						"if": {
							"properties": {
								"op": {
									"enum": ["aws.s3.get", "aws.s3.exists"]
								}
							}
						},
						"then": {
							"required": ["params"],
							"properties": {
								"params": {
									"required": ["bucket", "key"]
								}
							}
						}
					},
					{
						"if": {
							"properties": {
								"op": {
									"enum": ["aws.ddb.query"]
								}
							}
						},
						"then": {
							"required": ["params"],
							"properties": {
								"params": {
									"required": ["table", "key"]
								}
							}
						}
					},
					{
						"if": {
							"properties": {
								"op": {
									"enum": ["aws.sqs.send"]
								}
							}
						},
						"then": {
							"required": ["params"],
							"properties": {
								"params": {
									"required": ["queue", "message"]
								}
							}
						}
					}
				]
			}
		}
	}`
}

func ValidateYAMLWithSchema(yamlPayload []byte) error {
	var data interface{}
	if err := yaml.Unmarshal(yamlPayload, &data); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	schemaLoader := gojsonschema.NewStringLoader(GetJSONSchema())
	documentLoader := gojsonschema.NewBytesLoader(jsonData)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("failed to validate schema: %w", err)
	}

	if !result.Valid() {
		var errMsg string
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return fmt.Errorf("schema validation failed:\n%s", errMsg)
	}

	return nil
}
