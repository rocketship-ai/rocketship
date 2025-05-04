package ddb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoDBPlugin struct{}

func (c *DynamoDBPlugin) Name() string {
	return "aws.ddb.query"
}

func (c *DynamoDBPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	table, ok := p["table"].(string)
	if !ok {
		return nil, fmt.Errorf("table parameter is required")
	}

	key, ok := p["key"].(string)
	if !ok {
		return nil, fmt.Errorf("key parameter is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		if endpoint := getLocalstackEndpoint(); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	keyAttr := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: key},
	}

	resp, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key:       keyAttr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query DynamoDB: %w", err)
	}

	result := make(map[string]interface{})
	if resp.Item == nil {
		return result, nil
	}

	for k, v := range resp.Item {
		switch attr := v.(type) {
		case *types.AttributeValueMemberS:
			result[k] = attr.Value
		case *types.AttributeValueMemberN:
			result[k] = attr.Value
		case *types.AttributeValueMemberBOOL:
			result[k] = attr.Value
		}
	}

	return result, nil
}

func getLocalstackEndpoint() string {
	return "http://localstack:4566"
}
