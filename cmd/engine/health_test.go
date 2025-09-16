package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	server := httptest.NewServer(newHealthMux())
	defer server.Close()

	paths := []string{"/", "/healthz"}

	for _, path := range paths {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s failed: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 for %s, got %d", path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected content-type application/json for %s, got %s", path, ct)
		}

		var payload healthPayload
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode response for %s: %v", path, err)
		}
		_ = resp.Body.Close()

		if payload.Status != "ok" {
			t.Fatalf("expected status ok for %s, got %s", path, payload.Status)
		}
	}
}
