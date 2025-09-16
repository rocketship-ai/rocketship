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
	"time"

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
	Version          string
	CacheTTL         time.Duration
}

type openAPIValidator struct {
	config       *openAPIValidationConfig
	entry        *openAPISpecEntry
	requestInput *openapi3filter.RequestValidationInput
	requestBody  []byte
}

type openAPISpecEntry struct {
	doc         *openapi3.T
	router      routers.Router
	loadedAt    time.Time
	localPath   string
	fileModTime time.Time
}

var openAPISpecCache = struct {
	mu    sync.RWMutex
	specs map[string]*openAPISpecEntry
}{
	specs: make(map[string]*openAPISpecEntry),
}

const defaultOpenAPICacheTTL = 30 * time.Minute

type openAPICacheOptions struct {
	Version string
	TTL     time.Duration
}

func cacheKeyForOpenAPI(location, version string) string {
	if version == "" {
		return location
	}
	return fmt.Sprintf("%s::%s", location, version)
}

func newOpenAPIValidator(ctx context.Context, configData map[string]interface{}, suiteData interface{}, state map[string]string) (*openAPIValidator, error) {
	cfg, err := parseOpenAPIValidationConfig(configData, suiteData, state)
	if err != nil || cfg == nil {
		return nil, err
	}

	entry, err := loadOpenAPISpec(ctx, cfg.SpecLocation, openAPICacheOptions{
		Version: cfg.Version,
		TTL:     cfg.CacheTTL,
	})
	if err != nil {
		return nil, err
	}

	return &openAPIValidator{config: cfg, entry: entry}, nil
}

func parseOpenAPIValidationConfig(configData map[string]interface{}, suiteData interface{}, state map[string]string) (*openAPIValidationConfig, error) {
	var (
		suiteMap map[string]interface{}
		ok       bool
	)

	if suiteData != nil {
		suiteMap, ok = suiteData.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("suite openapi config must be an object")
		}
	}

	var stepOverride map[string]interface{}
	if raw, exists := configData["openapi"]; exists {
		var ok bool
		stepOverride, ok = raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("openapi config must be an object")
		}
	}

	if suiteMap == nil && stepOverride == nil {
		return nil, nil
	}

	cfg := &openAPIValidationConfig{
		ValidateRequest:  true,
		ValidateResponse: true,
		CacheTTL:         defaultOpenAPICacheTTL,
	}

	if suiteMap != nil {
		if spec, exists, err := getStringField(suiteMap, "spec"); err != nil {
			return nil, err
		} else if exists {
			if spec == "" {
				return nil, fmt.Errorf("openapi.spec must be a non-empty string")
			}
			processed, err := replaceVariables(spec, state)
			if err != nil {
				return nil, fmt.Errorf("failed to process openapi.spec template: %w", err)
			}
			cfg.SpecLocation = processed
		} else {
			return nil, fmt.Errorf("openapi.spec is required in suite-level configuration")
		}

		if val, exists, err := getBoolField(suiteMap, "validate_request"); err != nil {
			return nil, err
		} else if exists {
			cfg.ValidateRequest = val
		}

		if val, exists, err := getBoolField(suiteMap, "validate_response"); err != nil {
			return nil, err
		} else if exists {
			cfg.ValidateResponse = val
		}

		if version, exists, err := getStringField(suiteMap, "version"); err != nil {
			return nil, err
		} else if exists {
			cfg.Version = version
		}

		if ttlStr, exists, err := getStringField(suiteMap, "cache_ttl"); err != nil {
			return nil, err
		} else if exists && ttlStr != "" {
			dur, err := time.ParseDuration(ttlStr)
			if err != nil {
				return nil, fmt.Errorf("openapi.cache_ttl must be a valid duration: %w", err)
			}
			if dur > 0 {
				cfg.CacheTTL = dur
			}
		}
	}

	if stepOverride != nil {
		if spec, exists, err := getStringField(stepOverride, "spec"); err != nil {
			return nil, err
		} else if exists {
			if spec == "" {
				return nil, fmt.Errorf("openapi.spec override must be a non-empty string")
			}
			processed, err := replaceVariables(spec, state)
			if err != nil {
				return nil, fmt.Errorf("failed to process openapi.spec template: %w", err)
			}
			cfg.SpecLocation = processed
		}

		if opID, exists, err := getStringField(stepOverride, "operation_id"); err != nil {
			return nil, err
		} else if exists {
			cfg.OperationID = opID
		}

		if val, exists, err := getBoolField(stepOverride, "validate_request"); err != nil {
			return nil, err
		} else if exists {
			cfg.ValidateRequest = val
		}

		if val, exists, err := getBoolField(stepOverride, "validate_response"); err != nil {
			return nil, err
		} else if exists {
			cfg.ValidateResponse = val
		}

		if version, exists, err := getStringField(stepOverride, "version"); err != nil {
			return nil, err
		} else if exists {
			cfg.Version = version
		}
	}

	if !cfg.ValidateRequest && !cfg.ValidateResponse {
		return nil, nil
	}

	if strings.TrimSpace(cfg.SpecLocation) == "" {
		return nil, fmt.Errorf("openapi.spec must be provided via suite-level defaults or step override")
	}

	return cfg, nil
}

func getStringField(data map[string]interface{}, key string) (string, bool, error) {
	raw, exists := data[key]
	if !exists {
		return "", false, nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", true, fmt.Errorf("openapi.%s must be a string", key)
	}
	return strings.TrimSpace(value), true, nil
}

func getBoolField(data map[string]interface{}, key string) (bool, bool, error) {
	raw, exists := data[key]
	if !exists {
		return false, false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, true, fmt.Errorf("openapi.%s must be a boolean", key)
	}
	return value, true, nil
}

func loadOpenAPISpec(ctx context.Context, location string, opts openAPICacheOptions) (*openAPISpecEntry, error) {
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = defaultOpenAPICacheTTL
	}

	cacheKey := cacheKeyForOpenAPI(location, opts.Version)

	openAPISpecCache.mu.RLock()
	if entry, ok := openAPISpecCache.specs[cacheKey]; ok {
		openAPISpecCache.mu.RUnlock()
		if !entry.needsReload(ttl) {
			return entry, nil
		}
	} else {
		openAPISpecCache.mu.RUnlock()
	}

	openAPISpecCache.mu.Lock()
	defer openAPISpecCache.mu.Unlock()

	if entry, ok := openAPISpecCache.specs[cacheKey]; ok {
		if entry.needsReload(ttl) {
			delete(openAPISpecCache.specs, cacheKey)
		} else {
			return entry, nil
		}
	}

	loader := &openapi3.Loader{Context: ctx}
	loader.IsExternalRefsAllowed = true

	var (
		doc          *openapi3.T
		err          error
		parsedURL    *url.URL
		resolvedPath string
		modTime      time.Time
	)

	if u, parseErr := url.Parse(location); parseErr == nil && u.Scheme != "" {
		parsedURL = u
		doc, err = loader.LoadFromURI(u)
		if err != nil {
			switch strings.ToLower(u.Scheme) {
			case "http", "https":
				return nil, fmt.Errorf("failed to load OpenAPI spec from %q: %w", location, err)
			case "file":
				// fall back to local file handling below
				doc = nil
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
			} else if parsedURL.Path != "" && parsedURL.Scheme == "" {
				path = parsedURL.Path
			}
		}
		if !filepath.IsAbs(path) {
			if wd, err := os.Getwd(); err == nil {
				path = filepath.Join(wd, path)
			}
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			return nil, fmt.Errorf("failed to stat OpenAPI spec %q: %w", location, statErr)
		}
		modTime = info.ModTime()
		resolvedPath = path
		doc, err = loader.LoadFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load OpenAPI spec from %q: %w", location, err)
		}
	}

	router, err := legacy.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAPI router for %q: %w", location, err)
	}

	entry := &openAPISpecEntry{
		doc:         doc,
		router:      router,
		loadedAt:    time.Now(),
		localPath:   resolvedPath,
		fileModTime: modTime,
	}
	openAPISpecCache.specs[cacheKey] = entry
	return entry, nil
}

func (entry *openAPISpecEntry) needsReload(ttl time.Duration) bool {
	if entry == nil {
		return true
	}
	if ttl > 0 && time.Since(entry.loadedAt) > ttl {
		return true
	}
	if entry.localPath != "" {
		if info, err := os.Stat(entry.localPath); err == nil {
			if info.ModTime().After(entry.fileModTime) {
				return true
			}
		}
	}
	return false
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
