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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	validatorErrors "github.com/pb33f/libopenapi-validator/errors"
	"gopkg.in/yaml.v3"
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
	config          *openAPIValidationConfig
	entry           *openAPISpecEntry
	requestClone    *http.Request
	requestBodyCopy []byte
	matchedOp       *operationMatcher
}

type openAPISpecEntry struct {
	validator   validator.Validator
	operations  []*operationMatcher
	version     string
	loadedAt    time.Time
	localPath   string
	fileModTime time.Time
}

type operationMatcher struct {
	method      string
	template    string
	regex       *regexp.Regexp
	operationID string
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

	specBytes, resolvedPath, modTime, err := fetchSpecBytes(location)
	if err != nil {
		return nil, err
	}

	docRoot, err := parseOpenAPIRoot(specBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec %q: %w", location, err)
	}

	document, docErrs := libopenapi.NewDocument(specBytes)
	if docErrs != nil {
		return nil, fmt.Errorf("failed to load OpenAPI document for %q: %v", location, docErrs)
	}

	val, valErrs := validator.NewValidator(document)
	if len(valErrs) > 0 {
		return nil, fmt.Errorf("failed to initialise OpenAPI validator for %q: %s", location, formatGenericErrors(valErrs))
	}

	operations, err := buildOperationMatchers(docRoot.Paths)
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAPI operations for %q: %w", location, err)
	}

	entry := &openAPISpecEntry{
		validator:   val,
		operations:  operations,
		version:     docRoot.OpenAPI,
		loadedAt:    time.Now(),
		localPath:   resolvedPath,
		fileModTime: modTime,
	}
	openAPISpecCache.specs[cacheKey] = entry
	return entry, nil
}

func fetchSpecBytes(location string) ([]byte, string, time.Time, error) {
	if parsed, err := url.Parse(location); err == nil && parsed.Scheme != "" && parsed.Scheme != "file" {
		resp, err := http.Get(location)
		if err != nil {
			return nil, "", time.Time{}, fmt.Errorf("failed to download OpenAPI spec from %q: %w", location, err)
		}
		defer func() {
			if cerr := resp.Body.Close(); cerr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to close OpenAPI spec response body: %v\n", cerr)
			}
		}()

		if resp.StatusCode != http.StatusOK {
			return nil, "", time.Time{}, fmt.Errorf("failed to download OpenAPI spec from %q: HTTP %d", location, resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", time.Time{}, fmt.Errorf("failed to read OpenAPI spec from %q: %w", location, err)
		}
		return data, "", time.Now(), nil
	}

	path := location
	if parsed, err := url.Parse(location); err == nil && parsed.Scheme == "file" {
		path = parsed.Path
	}

	if !filepath.IsAbs(path) {
		if wd, err := os.Getwd(); err == nil {
			path = filepath.Join(wd, path)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("failed to read OpenAPI spec %q: %w", location, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("failed to stat OpenAPI spec %q: %w", location, err)
	}

	return data, path, info.ModTime(), nil
}

type openAPIRoot struct {
	OpenAPI string                            `yaml:"openapi" json:"openapi"`
	Paths   map[string]map[string]interface{} `yaml:"paths" json:"paths"`
}

func parseOpenAPIRoot(data []byte) (*openAPIRoot, error) {
	var root openAPIRoot
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root.Paths == nil {
		root.Paths = make(map[string]map[string]interface{})
	}
	return &root, nil
}

func buildOperationMatchers(paths map[string]map[string]interface{}) ([]*operationMatcher, error) {
	httpMethods := map[string]struct{}{
		"GET": {}, "PUT": {}, "POST": {}, "DELETE": {}, "OPTIONS": {}, "HEAD": {}, "PATCH": {}, "TRACE": {},
	}

	operations := make([]*operationMatcher, 0)
	for template, methodMap := range paths {
		if methodMap == nil {
			continue
		}
		pattern := convertPathTemplateToRegex(template)
		if pattern == "" {
			continue
		}
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid path template %q: %w", template, err)
		}

		for method, raw := range methodMap {
			if _, ok := httpMethods[strings.ToUpper(method)]; !ok {
				continue
			}
			operationID := extractOperationID(raw)
			operations = append(operations, &operationMatcher{
				method:      strings.ToUpper(method),
				template:    template,
				regex:       regex,
				operationID: operationID,
			})
		}
	}

	return operations, nil
}

func convertPathTemplateToRegex(template string) string {
	if template == "" {
		return ""
	}
	escaped := regexp.QuoteMeta(template)
	re := regexp.MustCompile(`\\\{[^/]+\\\}`)
	replaced := re.ReplaceAllString(escaped, "[^/]+")
	return "^" + replaced + "$"
}

func extractOperationID(raw interface{}) string {
	if raw == nil {
		return ""
	}
	if m, ok := raw.(map[string]interface{}); ok {
		if val, exists := m["operationId"]; exists {
			if str, ok := val.(string); ok {
				return strings.TrimSpace(str)
			}
		}
	}
	return ""
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

func (entry *openAPISpecEntry) matchOperation(method, path string) *operationMatcher {
	method = strings.ToUpper(method)
	for _, op := range entry.operations {
		if op.method != method {
			continue
		}
		if op.regex.MatchString(path) {
			return op
		}
	}
	return nil
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

	matched := v.entry.matchOperation(clone.Method, clone.URL.Path)
	if matched == nil {
		return fmt.Errorf("failed to find OpenAPI operation for %s %s in %q", req.Method, req.URL.Path, v.config.SpecLocation)
	}

	if v.config.OperationID != "" {
		actual := matched.operationID
		if actual != v.config.OperationID {
			return fmt.Errorf("OpenAPI operation_id mismatch: expected %q but matched %q", v.config.OperationID, actual)
		}
	}

	if len(body) > 0 {
		v.requestBodyCopy = append([]byte(nil), body...)
	} else {
		v.requestBodyCopy = nil
	}

	v.requestClone = clone
	v.matchedOp = matched

	return nil
}

func (v *openAPIValidator) validateRequest(ctx context.Context) error {
	if v.requestClone == nil {
		return fmt.Errorf("internal error: request validation input is not prepared")
	}
	setRequestBody(v.requestClone, v.requestBodyCopy)
	ok, errs := v.entry.validator.ValidateHttpRequest(v.requestClone)
	if ok {
		return nil
	}
	return formatValidationErrors("openapi request validation failed", errs)
}

func (v *openAPIValidator) validateResponse(ctx context.Context, resp *http.Response, body []byte) error {
	if v.requestClone == nil {
		return fmt.Errorf("internal error: request validation input is required for response validation")
	}

	setRequestBody(v.requestClone, v.requestBodyCopy)
	respClone := cloneHTTPResponse(resp, body)
	respClone.Request = v.requestClone
	ok, errs := v.entry.validator.ValidateHttpRequestResponse(v.requestClone, respClone)
	if ok {
		return nil
	}
	return formatValidationErrors("openapi response validation failed", errs)
}

func cloneHTTPResponse(resp *http.Response, body []byte) *http.Response {
	clone := &http.Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Proto:         resp.Proto,
		ProtoMajor:    resp.ProtoMajor,
		ProtoMinor:    resp.ProtoMinor,
		Header:        resp.Header.Clone(),
		Trailer:       resp.Trailer.Clone(),
		ContentLength: int64(len(body)),
		Body:          io.NopCloser(bytes.NewReader(body)),
	}
	return clone
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

func formatValidationErrors(prefix string, errs []*validatorErrors.ValidationError) error {
	if len(errs) == 0 {
		return fmt.Errorf("%s: unknown validation error", prefix)
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		msg := strings.TrimSpace(e.Message)
		if msg == "" {
			msg = strings.TrimSpace(e.Reason)
		}
		if msg == "" {
			msg = fmt.Sprintf("%s validation failed", e.ValidationType)
		}

		if len(e.SchemaValidationErrors) > 0 {
			schemaParts := make([]string, 0, len(e.SchemaValidationErrors))
			for _, se := range e.SchemaValidationErrors {
				detail := strings.TrimSpace(se.Reason)
				if loc := strings.TrimSpace(se.Location); loc != "" {
					detail = fmt.Sprintf("%s: %s", loc, detail)
				}
				schemaParts = append(schemaParts, detail)
			}
			msg = fmt.Sprintf("%s (%s)", msg, strings.Join(schemaParts, "; "))
		}

		if fix := strings.TrimSpace(e.HowToFix); fix != "" {
			msg = fmt.Sprintf("%s. Fix: %s", msg, fix)
		}
		parts = append(parts, msg)
	}
	return fmt.Errorf("%s: %s", prefix, strings.Join(parts, "; "))
}

func formatGenericErrors(errs []error) string {
	if len(errs) == 0 {
		return "unknown error"
	}
	msgs := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		msgs = append(msgs, strings.TrimSpace(err.Error()))
	}
	if len(msgs) == 0 {
		return "unknown error"
	}
	return strings.Join(msgs, "; ")
}
