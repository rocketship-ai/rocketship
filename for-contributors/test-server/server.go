package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Store represents an in-memory data store
type Store struct {
	data map[string]map[string]interface{}
	mu   sync.RWMutex
}

// TestServer implements the HTTP test server
type TestServer struct {
	stores  map[string]*Store // Map of client IP to store
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

		// Clean up all stores
		s.mu.Lock()
		for clientIP, store := range s.stores {
			store.mu.Lock()
			store.data = make(map[string]map[string]interface{})
			store.mu.Unlock()
			log.Printf("🧹 Cleaned up store for client IP: %s", clientIP)
		}
		s.mu.Unlock()

		log.Printf("🧹 Hourly cleanup completed at %s", time.Now().Format(time.RFC3339))
	}
}

// getStore returns the store for a given client IP
func (s *TestServer) getStore(clientIP string) *Store {
	s.mu.RLock()
	store, exists := s.stores[clientIP]
	s.mu.RUnlock()

	if exists {
		return store
	}

	// Double-checked locking pattern
	s.mu.Lock()
	defer s.mu.Unlock()

	store, exists = s.stores[clientIP]
	if exists {
		return store
	}

	store = &Store{
		data: make(map[string]map[string]interface{}),
	}
	s.stores[clientIP] = store
	return store
}

// getClientIP extracts the client IP from the request
func (s *TestServer) getClientIP(r *http.Request) string {
	// Try to get the real IP from X-Forwarded-For header
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fallback to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if colonIndex := strings.LastIndex(ip, ":"); colonIndex != -1 {
		ip = ip[:colonIndex]
	}
	return ip
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

	// Get client IP and corresponding store
	clientIP := s.getClientIP(r)
	store := s.getStore(clientIP)

	// Special handling for clear state
	if r.URL.Path == "/_clear" && r.Method == http.MethodPost {
		store.mu.Lock()
		store.data = make(map[string]map[string]interface{})
		store.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Parse the path to get resource type and ID
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
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
		// Return all resources of this type
		if resources, ok := store.data[resourceType]; ok {
			return resources, nil
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

	// Generate an ID if not provided
	if _, ok := resource["id"]; !ok {
		resource["id"] = fmt.Sprintf("%s_%d", resourceType, len(store.data[resourceType]))
	}
	resourceID := resource["id"].(string)

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.data[resourceType]; !ok {
		store.data[resourceType] = make(map[string]interface{})
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
