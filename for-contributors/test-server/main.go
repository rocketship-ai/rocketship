package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the server on")
	flag.Parse()

	server := NewTestServer()

	log.Printf("🚀 Starting test server on port %d...", *port)
	log.Printf("📝 Request logging is enabled")
	log.Printf("💾 Using in-memory store")

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), server); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}
