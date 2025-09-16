package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

type openAPIValidationConfig struct {
	SpecLocation     string
	OperationID      string
	ValidateRequest  bool
	ValidateResponse bool
}

type openAPIValidator struct {
	config       *openAPIValidationConfig
	entry        *openAPISpecEntry
	requestInput *openapi3filter.RequestValidationInput
	requestBody  []byte
}

type openAPISpecEntry struct {
	doc    *openapi3.T
	router routers.Router
}

var openAPISpecCache = struct {
	mu    sync.RWMutex
	specs map[string]*openAPISpecEntry
}{
	specs: make(map[string]*openAPISpecEntry),
}

func newOpenAPIValidator(ctx context.Context, configData map[string]interface{}, state map[string]string) (*openAPIValidator, error) {
	cfg, err := parseOpenAPIValidationConfig(configData, state)
	if err != nil || cfg == nil {
		return nil, err
	}

	entry, err := loadOpenAPISpec(ctx, cfg.SpecLocation)
	if err != nil {
		return nil, err
	}

	return &openAPIValidator{config: cfg, entry: entry}, nil
}

func parseOpenAPIValidationConfig(configData map[string]interface{}, state map[string]string) (*openAPIValidationConfig, error) {
	raw, ok := configData["openapi"]
	if !ok {
		return nil, nil
	}

	openapiMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("openapi config must be an object")
	}

	rawSpec, ok := openapiMap["spec"]
	if !ok {
		return nil, fmt.Errorf("openapi.spec is required when OpenAPI validation is enabled")
	}

	specStr, ok := rawSpec.(string)
	if !ok {
		return nil, fmt.Errorf("openapi.spec must be a string")
	}
	specStr = strings.TrimSpace(specStr)
	if specStr == "" {
		return nil, fmt.Errorf("openapi.spec must be a non-empty string")
	}

	if processed, err := replaceVariables(specStr, state); err == nil {
		specStr = processed
	} else {
		return nil, fmt.Errorf("failed to process openapi.spec template: %w", err)
	}

	cfg := &openAPIValidationConfig{
		SpecLocation:     specStr,
		ValidateRequest:  true,
		ValidateResponse: true,
	}

	if rawOperationID, ok := openapiMap["operation_id"]; ok {
		operationID, ok := rawOperationID.(string)
		if !ok {
			return nil, fmt.Errorf("openapi.operation_id must be a string")
		}
		cfg.OperationID = strings.TrimSpace(operationID)
	}

	if rawValidateRequest, ok := openapiMap["validate_request"]; ok {
		validateRequest, ok := rawValidateRequest.(bool)
		if !ok {
			return nil, fmt.Errorf("openapi.validate_request must be a boolean")
		}
		cfg.ValidateRequest = validateRequest
	}

	if rawValidateResponse, ok := openapiMap["validate_response"]; ok {
		validateResponse, ok := rawValidateResponse.(bool)
		if !ok {
			return nil, fmt.Errorf("openapi.validate_response must be a boolean")
		}
		cfg.ValidateResponse = validateResponse
	}

	return cfg, nil
}

func loadOpenAPISpec(ctx context.Context, location string) (*openAPISpecEntry, error) {
	openAPISpecCache.mu.RLock()
	if entry, ok := openAPISpecCache.specs[location]; ok {
		openAPISpecCache.mu.RUnlock()
		return entry, nil
	}
	openAPISpecCache.mu.RUnlock()

	openAPISpecCache.mu.Lock()
	defer openAPISpecCache.mu.Unlock()

	if entry, ok := openAPISpecCache.specs[location]; ok {
		return entry, nil
	}

	loader := &openapi3.Loader{Context: ctx}
	loader.IsExternalRefsAllowed = true

	var (
		doc       *openapi3.T
		err       error
		parsedURL *url.URL
	)

	if u, parseErr := url.Parse(location); parseErr == nil && u.Scheme != "" {
		parsedURL = u
		doc, err = loader.LoadFromURI(u)
		if err != nil {
			switch strings.ToLower(u.Scheme) {
			case "http", "https":
				return nil, fmt.Errorf("failed to load OpenAPI spec from %q: %w", location, err)
			default:
				doc = nil
			}
		}
	}

	if doc == nil {
		path := location
		if parsedURL != nil {
			if parsedURL.Scheme == "file" && parsedURL.Path != "" {
				path = parsedURL.Path
			} else if parsedURL.Path != "" {
				path = parsedURL.Path
			}
		}
		if !filepath.IsAbs(path) {
			if wd, err := os.Getwd(); err == nil {
				path = filepath.Join(wd, path)
			}
		}
		doc, err = loader.LoadFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load OpenAPI spec from %q: %w", location, err)
		}
	}

	router, err := legacy.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAPI router for %q: %w", location, err)
	}

	entry := &openAPISpecEntry{doc: doc, router: router}
	openAPISpecCache.specs[location] = entry
	return entry, nil
}

func (v *openAPIValidator) shouldValidateRequest() bool {
	return v != nil && v.config.ValidateRequest
}

func (v *openAPIValidator) shouldValidateResponse() bool {
	return v != nil && v.config.ValidateResponse
}

func (v *openAPIValidator) prepareRequestValidation(ctx context.Context, req *http.Request, body []byte) error {
	clone := req.Clone(ctx)
	clone.Header = req.Header.Clone()
	setRequestBody(clone, body)

	route, pathParams, err := v.entry.router.FindRoute(clone)
	if err != nil {
		return fmt.Errorf("failed to find OpenAPI operation for %s %s in %q: %w", req.Method, req.URL.Path, v.config.SpecLocation, err)
	}

	if v.config.OperationID != "" {
		actualOperationID := ""
		if route.Operation != nil {
			actualOperationID = route.Operation.OperationID
		}
		if actualOperationID != v.config.OperationID {
			return fmt.Errorf("OpenAPI operation_id mismatch: expected %q but matched %q", v.config.OperationID, actualOperationID)
		}
	}

	v.requestInput = &openapi3filter.RequestValidationInput{
		Request:    clone,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			MultiError:         true,
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
	}

	if len(body) > 0 {
		v.requestBody = append([]byte(nil), body...)
	} else {
		v.requestBody = nil
	}

	return nil
}

func (v *openAPIValidator) validateRequest(ctx context.Context) error {
	if v.requestInput == nil {
		return fmt.Errorf("internal error: request validation input is not prepared")
	}

	if err := openapi3filter.ValidateRequest(ctx, v.requestInput); err != nil {
		return fmt.Errorf("openapi request validation failed: %w", err)
	}
	if len(v.requestBody) == 0 {
		setRequestBody(v.requestInput.Request, nil)
	} else {
		setRequestBody(v.requestInput.Request, v.requestBody)
	}
	return nil
}

func (v *openAPIValidator) validateResponse(ctx context.Context, resp *http.Response, body []byte) error {
	if v.requestInput == nil {
		return fmt.Errorf("internal error: request validation input is required for response validation")
	}

	if len(v.requestBody) == 0 {
		setRequestBody(v.requestInput.Request, nil)
	} else {
		setRequestBody(v.requestInput.Request, v.requestBody)
	}

	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: v.requestInput,
		Status:                 resp.StatusCode,
		Header:                 resp.Header.Clone(),
		Body:                   io.NopCloser(bytes.NewReader(body)),
		Options: &openapi3filter.Options{
			MultiError:            true,
			IncludeResponseStatus: true,
			AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
		},
	}

	if err := openapi3filter.ValidateResponse(ctx, responseInput); err != nil {
		return fmt.Errorf("openapi response validation failed: %w", err)
	}

	return nil
}

func setRequestBody(req *http.Request, body []byte) {
	if len(body) == 0 {
		req.Body = http.NoBody
		req.ContentLength = 0
		req.GetBody = func() (io.ReadCloser, error) { return http.NoBody, nil }
		return
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
}
