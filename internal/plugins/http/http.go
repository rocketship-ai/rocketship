package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type HTTPPlugin struct{}

func (c *HTTPPlugin) Name() string {
	return "http.send"
}

func (c *HTTPPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	method, ok := p["method"].(string)
	if !ok {
		return nil, fmt.Errorf("method parameter is required")
	}

	url, ok := p["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter is required")
	}

	var body io.Reader
	if bodyStr, ok := p["body"].(string); ok {
		body = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if headers, ok := p["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if strVal, ok := v.(string); ok {
				req.Header.Add(k, strVal)
			}
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return map[string]interface{}{
		"status":  resp.StatusCode,
		"headers": resp.Header,
		"body":    string(respBody),
	}, nil
}
