package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	validatorErrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
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
	validator       validator.Validator
	schemaValidator schema_validation.SchemaValidator
	operations      []*operationMatcher
	version         string
	versionNumber   float32
	loadedAt        time.Time
	localPath       string
	fileModTime     time.Time
}

type operationMatcher struct {
	method           string
	template         string
	regex            *regexp.Regexp
	operationID      string
	requestSchemas   map[string]*base.Schema
	basePathMatchers []basePathMatcher
}

type basePathMatcher struct {
	segments []string
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

	versionNumber := parseOpenAPIVersion(docRoot.OpenAPI)

	document, docErr := libopenapi.NewDocument(specBytes)
	if docErr != nil {
		return nil, fmt.Errorf("failed to load OpenAPI document for %q: %v", location, docErr)
	}

	docModel, buildErrs := document.BuildV3Model()
	if len(buildErrs) > 0 {
		return nil, fmt.Errorf("failed to build OpenAPI model for %q: %s", location, formatGenericErrors(buildErrs))
	}

	val, valErrs := validator.NewValidator(document)
	if len(valErrs) > 0 {
		return nil, fmt.Errorf("failed to initialise OpenAPI validator for %q: %s", location, formatGenericErrors(valErrs))
	}

	schemaValidator := schema_validation.NewSchemaValidator(
		config.WithOpenAPIMode(),
		config.WithScalarCoercion(),
	)

	operations, err := buildOperationMatchers(docRoot.Paths, docModel)
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAPI operations for %q: %w", location, err)
	}

	entry := &openAPISpecEntry{
		validator:       val,
		schemaValidator: schemaValidator,
		operations:      operations,
		version:         docRoot.OpenAPI,
		versionNumber:   versionNumber,
		loadedAt:        time.Now(),
		localPath:       resolvedPath,
		fileModTime:     modTime,
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

func parseOpenAPIVersion(version string) float32 {
	version = strings.TrimSpace(version)
	if version == "" {
		return 3.1
	}
	parts := strings.Split(version, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 3.1
	}
	minor := 0
	if len(parts) > 1 {
		if m, err := strconv.Atoi(parts[1]); err == nil {
			minor = m
		}
	}
	value, err := strconv.ParseFloat(fmt.Sprintf("%d.%d", major, minor), 32)
	if err != nil {
		return 3.1
	}
	return float32(value)
}

func buildOperationMatchers(paths map[string]map[string]interface{}, docModel *libopenapi.DocumentModel[v3high.Document]) ([]*operationMatcher, error) {
	operations := make([]*operationMatcher, 0)
	for template, methodMap := range paths {
		pattern := convertPathTemplateToRegex(template)
		if pattern == "" {
			continue
		}
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid path template %q: %w", template, err)
		}

		pathItem := docModel.Model.Paths.PathItems.GetOrZero(template)
		for method := range methodMap {
			highOp := extractHighOperation(pathItem, method)
			requestSchemas := extractRequestSchemas(highOp)
			operationID := ""
			if highOp != nil {
				operationID = strings.TrimSpace(highOp.OperationId)
			}
			if operationID == "" {
				operationID = extractOperationIDFromRaw(methodMap[method])
			}
			basePaths := collectBasePaths(highOp, pathItem, &docModel.Model)
			baseMatchers := make([]basePathMatcher, 0, len(basePaths))
			for _, bp := range basePaths {
				baseMatchers = append(baseMatchers, newBasePathMatcher(bp))
			}
			operations = append(operations, &operationMatcher{
				method:           strings.ToUpper(method),
				template:         template,
				regex:            regex,
				operationID:      operationID,
				requestSchemas:   requestSchemas,
				basePathMatchers: baseMatchers,
			})
		}
	}
	return operations, nil
}

func extractHighOperation(pathItem *v3high.PathItem, method string) *v3high.Operation {
	if pathItem == nil {
		return nil
	}
	switch strings.ToUpper(method) {
	case http.MethodGet:
		return pathItem.Get
	case http.MethodPut:
		return pathItem.Put
	case http.MethodPost:
		return pathItem.Post
	case http.MethodDelete:
		return pathItem.Delete
	case http.MethodOptions:
		return pathItem.Options
	case http.MethodHead:
		return pathItem.Head
	case http.MethodPatch:
		return pathItem.Patch
	case http.MethodTrace:
		return pathItem.Trace
	default:
		return nil
	}
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

func extractOperationIDFromRaw(raw interface{}) string {
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

func extractRequestSchemas(op *v3high.Operation) map[string]*base.Schema {
	schemas := make(map[string]*base.Schema)
	if op == nil || op.RequestBody == nil || op.RequestBody.Content == nil {
		return schemas
	}
	for mediaPair := op.RequestBody.Content.First(); mediaPair != nil; mediaPair = mediaPair.Next() {
		mediaType := strings.ToLower(strings.TrimSpace(mediaPair.Key()))
		media := mediaPair.Value()
		if media == nil || media.Schema == nil {
			continue
		}
		schema := media.Schema.Schema()
		if schema != nil {
			schemas[mediaType] = schema
		}
	}
	return schemas
}

func collectBasePaths(op *v3high.Operation, pathItem *v3high.PathItem, doc *v3high.Document) []string {
	seen := make(map[string]struct{})
	basePaths := make([]string, 0)

	addServers := func(servers []*v3high.Server) {
		for _, srv := range servers {
			if srv == nil {
				continue
			}
			base := normalizeBasePath(extractServerBasePath(srv.URL))
			if _, ok := seen[base]; ok {
				continue
			}
			seen[base] = struct{}{}
			basePaths = append(basePaths, base)
		}
	}

	if op != nil && len(op.Servers) > 0 {
		addServers(op.Servers)
	}
	if len(basePaths) == 0 && pathItem != nil && len(pathItem.Servers) > 0 {
		addServers(pathItem.Servers)
	}
	if len(basePaths) == 0 && doc != nil && len(doc.Servers) > 0 {
		addServers(doc.Servers)
	}

	if len(basePaths) == 0 {
		basePaths = append(basePaths, "")
	}
	return basePaths
}

func extractServerBasePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/") {
		return raw
	}
	if strings.Contains(raw, "://") {
		if u, err := url.Parse(raw); err == nil {
			return u.Path
		}
	}
	if idx := strings.Index(raw, "/"); idx != -1 {
		return raw[idx:]
	}
	return ""
}

func normalizeBasePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

func newBasePathMatcher(path string) basePathMatcher {
	return basePathMatcher{segments: splitTemplateSegments(path)}
}

func (m basePathMatcher) trim(path string) (string, bool) {
	reqSegments := splitActualPath(path)
	if len(reqSegments) < len(m.segments) {
		return "", false
	}
	for i, seg := range m.segments {
		if isTemplateSegment(seg) {
			if reqSegments[i] == "" {
				return "", false
			}
			continue
		}
		if seg != reqSegments[i] {
			return "", false
		}
	}
	remainder := reqSegments[len(m.segments):]
	if len(remainder) == 0 {
		return "/", true
	}
	return "/" + strings.Join(remainder, "/"), true
}

func splitTemplateSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

func splitActualPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

func isTemplateSegment(seg string) bool {
	return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
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
		if len(op.basePathMatchers) == 0 {
			if op.regex.MatchString(path) {
				return op
			}
			continue
		}
		for _, matcher := range op.basePathMatchers {
			trimmed, ok := matcher.trim(path)
			if !ok {
				continue
			}
			if trimmed == "" {
				trimmed = "/"
			}
			if op.regex.MatchString(trimmed) {
				return op
			}
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
	if !ok {
		return formatValidationErrors("openapi request validation failed", errs)
	}
	if err := v.performAdditionalRequestValidation(); err != nil {
		return err
	}
	return nil
}

func (v *openAPIValidator) performAdditionalRequestValidation() error {
	if v.entry.schemaValidator == nil || v.matchedOp == nil || len(v.matchedOp.requestSchemas) == 0 {
		return nil
	}

	mediaType := normalizeMediaType(v.requestClone.Header.Get("Content-Type"))
	if mediaType == "" {
		return nil
	}

	schema, ok := v.matchedOp.requestSchemas[mediaType]
	if !ok {
		return nil
	}

	if !isFormMediaType(mediaType) {
		return nil
	}

	payload, err := parseURLEncodedPayload(v.requestBodyCopy)
	if err != nil {
		return fmt.Errorf("openapi request validation failed: %w", err)
	}

	valid, errs := v.entry.schemaValidator.ValidateSchemaObjectWithVersion(schema, payload, v.entry.versionNumber)
	if valid {
		return nil
	}
	return formatValidationErrors("openapi request validation failed", errs)
}

func normalizeMediaType(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	media, _, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(media)
}

func isFormMediaType(mediaType string) bool {
	return mediaType == "application/x-www-form-urlencoded"
}

func parseURLEncodedPayload(body []byte) (map[string]any, error) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse form payload: %w", err)
	}
	result := make(map[string]any)
	for key, vals := range values {
		if len(vals) == 1 {
			result[key] = vals[0]
			continue
		}
		arr := make([]any, len(vals))
		for i, val := range vals {
			arr[i] = val
		}
		result[key] = arr
	}
	return result, nil
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
