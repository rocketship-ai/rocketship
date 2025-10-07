package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// Store represents an in-memory data store
type Store struct {
	data     map[string]map[string]interface{}
	mu       sync.RWMutex
	counters map[string]int64 // Per-resource-type counters (simplified)
}

// TestServer implements the HTTP test server
type TestServer struct {
	stores  map[string]*Store // Map of session ID to store
	mu      sync.RWMutex      // Mutex for stores map
	limiter *rate.Limiter
}

// NewTestServer creates a new instance of TestServer
func NewTestServer() *TestServer {
	server := &TestServer{
		stores: make(map[string]*Store),
		// Allow 100 requests per minute
		limiter: rate.NewLimiter(rate.Every(time.Second), 100),
	}

	// Start the cleanup goroutine
	go server.startCleanupScheduler()

	return server
}

// startCleanupScheduler starts a goroutine that cleans up all stores at the top of every hour
func (s *TestServer) startCleanupScheduler() {
	for {
		// Calculate time until next hour
		now := time.Now()
		nextHour := now.Truncate(time.Hour).Add(time.Hour)
		timeUntilNextHour := nextHour.Sub(now)

		// Sleep until next hour
		time.Sleep(timeUntilNextHour)

		// First collect all stores
		s.mu.RLock()
		stores := make([]*Store, 0, len(s.stores))
		for _, store := range s.stores {
			stores = append(stores, store)
		}
		s.mu.RUnlock()

		// Then clean up each store
		for _, store := range stores {
			store.mu.Lock()
			store.data = make(map[string]map[string]interface{})
			store.counters = make(map[string]int64)
			store.mu.Unlock()
		}

		log.Printf("🧹 Hourly cleanup completed at %s", time.Now().Format(time.RFC3339))
	}
}

// getStore returns the store for a given session ID
func (s *TestServer) getStore(sessionID string) *Store {
	s.mu.RLock()
	store, exists := s.stores[sessionID]
	s.mu.RUnlock()

	if exists {
		return store
	}

	// Double-checked locking pattern
	s.mu.Lock()
	defer s.mu.Unlock()

	store, exists = s.stores[sessionID]
	if exists {
		return store
	}

	store = &Store{
		data:     make(map[string]map[string]interface{}),
		counters: make(map[string]int64),
	}
	s.stores[sessionID] = store
	return store
}

// getSessionID extracts session ID from request for store isolation
func (s *TestServer) getSessionID(r *http.Request) string {
	// Check for test session header first
	if sessionID := r.Header.Get("X-Test-Session"); sessionID != "" {
		return "session_" + sessionID
	}

	// Fallback to IP-based isolation for requests without session header
	ip := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			ip = strings.TrimSpace(ips[0])
		}
	}

	// Remove port if present
	if colonIndex := strings.LastIndex(ip, ":"); colonIndex != -1 {
		ip = ip[:colonIndex]
	}

	return "ip_" + ip
}

// corsMiddleware adds CORS headers to all responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware applies rate limiting to requests
func (s *TestServer) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *TestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Apply middleware
	handler := corsMiddleware(s.rateLimitMiddleware(http.HandlerFunc(s.handleRequest)))
	handler.ServeHTTP(w, r)
}

func (s *TestServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Log the request
	s.logRequest(r)

	// Special handling for health check
	if r.URL.Path == "/_health" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse the path for special endpoints
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) > 0 {
		// Handle special utility endpoints (session-agnostic)
		switch parts[0] {
		case "status":
			s.handleStatus(w, r, parts)
			return
		case "delay":
			s.handleDelay(w, r, parts)
			return
		case "echo":
			s.handleEcho(w, r)
			return
		case "uuid":
			s.handleUUID(w, r)
			return
		case "json":
			s.handleJSON(w, r)
			return
		}
	}

	// Get session ID and corresponding store for CRUD operations
	sessionID := s.getSessionID(r)
	store := s.getStore(sessionID)

	// Special handling for clear state
	if r.URL.Path == "/_clear" && r.Method == http.MethodPost {
		store.mu.Lock()
		store.data = make(map[string]map[string]interface{})
		store.counters = make(map[string]int64)
		store.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Parse the path to get resource type and ID
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	resourceType := parts[0]
	var resourceID string
	if len(parts) > 1 {
		resourceID = parts[1]
	}

	var response interface{}
	var err error

	switch r.Method {
	case http.MethodGet:
		response, err = s.handleGet(store, resourceType, resourceID)
	case http.MethodPost:
		response, err = s.handlePost(store, resourceType, r)
	case http.MethodPut:
		response, err = s.handlePut(store, resourceType, resourceID, r)
	case http.MethodDelete:
		response, err = s.handleDelete(store, resourceType, resourceID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err != nil {
		if err.Error() == "resource not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Write response
	if response != nil {
		json.NewEncoder(w).Encode(response)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}

	// Log the response
	s.logResponse(w, response)
}

func (s *TestServer) handleGet(store *Store, resourceType, resourceID string) (interface{}, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	if resourceID == "" {
		// Return all resources of this type in consistent indexed format
		if resources, ok := store.data[resourceType]; ok {
			// Sort resources by ID to ensure consistent ordering
			var sortedKeys []string
			for key := range resources {
				sortedKeys = append(sortedKeys, key)
			}
			sort.Strings(sortedKeys)

			// Convert to indexed format for client expectations
			indexedResources := make(map[string]interface{})
			for index, key := range sortedKeys {
				indexKey := fmt.Sprintf("%s_%d", resourceType, index)
				indexedResources[indexKey] = resources[key]
			}
			return indexedResources, nil
		}
		return map[string]interface{}{}, nil
	}

	// Return specific resource
	if resources, ok := store.data[resourceType]; ok {
		if resource, ok := resources[resourceID]; ok {
			return resource, nil
		}
	}
	return nil, fmt.Errorf("resource not found")
}

func (s *TestServer) handlePost(store *Store, resourceType string, r *http.Request) (interface{}, error) {
	var resource map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		return nil, err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	// Initialize the resource type map if it doesn't exist
	if _, ok := store.data[resourceType]; !ok {
		store.data[resourceType] = make(map[string]interface{})
	}

	// Generate an ID if not provided
	if _, ok := resource["id"]; !ok {
		// Use simple per-resource-type counter within lock for sequential IDs
		currentCount := store.counters[resourceType]
		store.counters[resourceType] = currentCount + 1
		resource["id"] = fmt.Sprintf("%s_%d", resourceType, currentCount)
	}
	resourceID := resource["id"].(string)

	// Add request headers that start with X- to the response for testing purposes
	for name, values := range r.Header {
		if strings.HasPrefix(strings.ToUpper(name), "X-") && len(values) > 0 {
			resource[strings.ToLower(name)] = values[0]
		}
	}

	store.data[resourceType][resourceID] = resource

	return resource, nil
}

func (s *TestServer) handlePut(store *Store, resourceType, resourceID string, r *http.Request) (interface{}, error) {
	if resourceID == "" {
		return nil, fmt.Errorf("resource ID required for PUT")
	}

	var resource map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		return nil, err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.data[resourceType]; !ok {
		return nil, fmt.Errorf("resource type not found")
	}

	store.data[resourceType][resourceID] = resource
	return resource, nil
}

func (s *TestServer) handleDelete(store *Store, resourceType, resourceID string) (interface{}, error) {
	if resourceID == "" {
		return nil, fmt.Errorf("resource ID required for DELETE")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if resources, ok := store.data[resourceType]; ok {
		if _, ok := resources[resourceID]; ok {
			delete(resources, resourceID)
			return nil, nil
		}
	}
	return nil, fmt.Errorf("resource not found")
}

// handleStatus returns the specified HTTP status code
func (s *TestServer) handleStatus(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 2 {
		http.Error(w, "Status code required: /status/{code}", http.StatusBadRequest)
		return
	}

	code, err := strconv.Atoi(parts[1])
	if err != nil || code < 100 || code > 599 {
		http.Error(w, "Invalid status code", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    code,
		"message": http.StatusText(code),
	})
}

// handleDelay delays the response by the specified number of seconds (max 10s)
func (s *TestServer) handleDelay(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 2 {
		http.Error(w, "Delay seconds required: /delay/{seconds}", http.StatusBadRequest)
		return
	}

	seconds, err := strconv.Atoi(parts[1])
	if err != nil || seconds < 0 || seconds > 10 {
		http.Error(w, "Invalid delay: must be 0-10 seconds", http.StatusBadRequest)
		return
	}

	time.Sleep(time.Duration(seconds) * time.Second)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"delayed": seconds,
		"message": fmt.Sprintf("Delayed for %d seconds", seconds),
	})
}

// handleEcho echoes back request details (headers, body, query params)
func (s *TestServer) handleEcho(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			query[key] = values[0]
		}
	}

	// Parse headers
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	response := map[string]interface{}{
		"method":  r.Method,
		"url":     r.URL.String(),
		"headers": headers,
		"query":   query,
	}

	// For POST requests, parse body and form data
	if r.Method == http.MethodPost {
		contentType := r.Header.Get("Content-Type")

		if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			if err := r.ParseForm(); err == nil {
				form := make(map[string]string)
				for key, values := range r.Form {
					if len(values) > 0 {
						form[key] = values[0]
					}
				}
				response["form"] = form
			}
		} else if strings.Contains(contentType, "application/json") {
			var body interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				response["json"] = body
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleUUID generates and returns a new UUID v4
func (s *TestServer) handleUUID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"uuid": uuid.New().String(),
	})
}

// handleJSON returns sample JSON data for testing
func (s *TestServer) handleJSON(w http.ResponseWriter, r *http.Request) {
	sampleJSON := map[string]interface{}{
		"slideshow": map[string]interface{}{
			"title":  "Sample Slide Show",
			"author": "Yours Truly",
			"date":   "date of publication",
			"slides": []map[string]interface{}{
				{
					"title": "Wake up to WonderWidgets!",
					"type":  "all",
				},
				{
					"title": "Overview",
					"type":  "all",
					"items": []string{
						"Why WonderWidgets are great",
						"Who buys WonderWidgets",
					},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(sampleJSON)
}

func (s *TestServer) logRequest(r *http.Request) {
	// Log request details
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Printf("❌ Error dumping request: %v", err)
		return
	}

	log.Printf("📥 Incoming Request:\n%s\n", string(dump))
}

func (s *TestServer) logResponse(w http.ResponseWriter, response interface{}) {
	log.Printf("📤 Response:\nStatus: %d\nBody: %+v\n", http.StatusOK, response)
}
