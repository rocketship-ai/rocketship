package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
)

// Store represents an in-memory data store
type Store struct {
	data map[string]map[string]interface{}
	mu   sync.RWMutex
}

// TestServer implements the HTTP test server
type TestServer struct {
	store *Store
}

// NewTestServer creates a new instance of TestServer
func NewTestServer() *TestServer {
	return &TestServer{
		store: &Store{
			data: make(map[string]map[string]interface{}),
		},
	}
}

func (s *TestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Log the request
	s.logRequest(r)

	// Special handling for clear state
	if r.URL.Path == "/_clear" && r.Method == http.MethodPost {
		s.store.mu.Lock()
		s.store.data = make(map[string]map[string]interface{})
		s.store.mu.Unlock()
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
		response, err = s.handleGet(resourceType, resourceID)
	case http.MethodPost:
		response, err = s.handlePost(resourceType, r)
	case http.MethodPut:
		response, err = s.handlePut(resourceType, resourceID, r)
	case http.MethodDelete:
		response, err = s.handleDelete(resourceType, resourceID)
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

func (s *TestServer) handleGet(resourceType, resourceID string) (interface{}, error) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	if resourceID == "" {
		// Return all resources of this type
		if resources, ok := s.store.data[resourceType]; ok {
			return resources, nil
		}
		return map[string]interface{}{}, nil
	}

	// Return specific resource
	if resources, ok := s.store.data[resourceType]; ok {
		if resource, ok := resources[resourceID]; ok {
			return resource, nil
		}
	}
	return nil, fmt.Errorf("resource not found")
}

func (s *TestServer) handlePost(resourceType string, r *http.Request) (interface{}, error) {
	var resource map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		return nil, err
	}

	// Generate an ID if not provided
	if _, ok := resource["id"]; !ok {
		resource["id"] = fmt.Sprintf("%s_%d", resourceType, len(s.store.data[resourceType]))
	}
	resourceID := resource["id"].(string)

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	if _, ok := s.store.data[resourceType]; !ok {
		s.store.data[resourceType] = make(map[string]interface{})
	}
	s.store.data[resourceType][resourceID] = resource

	return resource, nil
}

func (s *TestServer) handlePut(resourceType, resourceID string, r *http.Request) (interface{}, error) {
	if resourceID == "" {
		return nil, fmt.Errorf("resource ID required for PUT")
	}

	var resource map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		return nil, err
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	if _, ok := s.store.data[resourceType]; !ok {
		return nil, fmt.Errorf("resource type not found")
	}

	s.store.data[resourceType][resourceID] = resource
	return resource, nil
}

func (s *TestServer) handleDelete(resourceType, resourceID string) (interface{}, error) {
	if resourceID == "" {
		return nil, fmt.Errorf("resource ID required for DELETE")
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	if resources, ok := s.store.data[resourceType]; ok {
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
