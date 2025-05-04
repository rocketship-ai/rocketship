package sqs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type SQSPlugin struct{}

func (c *SQSPlugin) Name() string {
	return "aws.sqs.send"
}

func (c *SQSPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	queue, ok := p["queue"].(string)
	if !ok {
		return nil, fmt.Errorf("queue parameter is required")
	}

	message, ok := p["message"].(string)
	if !ok {
		return nil, fmt.Errorf("message parameter is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		if endpoint := getLocalstackEndpoint(); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	queueURL := fmt.Sprintf("http://localstack:4566/000000000000/%s", queue)

	resp, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(message),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	return map[string]interface{}{
		"messageId": *resp.MessageId,
	}, nil
}

func getLocalstackEndpoint() string {
	return "http://localstack:4566"
}
