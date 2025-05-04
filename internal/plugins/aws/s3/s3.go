package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Plugin struct{}

func (c *S3Plugin) Name() string {
	return "aws.s3.get"
}

func (c *S3Plugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	bucket, ok := p["bucket"].(string)
	if !ok {
		return nil, fmt.Errorf("bucket parameter is required")
	}

	key, ok := p["key"].(string)
	if !ok {
		return nil, fmt.Errorf("key parameter is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint := getLocalstackEndpoint(); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return map[string]interface{}{
			"exists": false,
		}, nil
	}

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer resp.Body.Close()

	buf := make([]byte, *resp.ContentLength)
	_, err = resp.Body.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read object content: %w", err)
	}

	return map[string]interface{}{
		"exists":  true,
		"content": string(buf),
	}, nil
}

func getLocalstackEndpoint() string {
	return "http://localstack:4566"
}
