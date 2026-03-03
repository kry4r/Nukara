package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"nukara/backend/internal/analysis"
	"nukara/backend/internal/store"
)

func main() {
	// Initialize store
	var dataStore store.DataStore
	postgresDSN := getEnv("NUKARA_POSTGRES_DSN", "")
	redisAddr := getEnv("NUKARA_REDIS_ADDR", "localhost:6379")

	if strings.TrimSpace(postgresDSN) != "" {
		persistentStore, err := store.NewPostgresStore(postgresDSN, redisAddr)
		if err != nil {
			log.Printf("⚠️  [analysis] WARNING: PostgreSQL connection failed, using in-memory store: %v", err)
			dataStore = store.NewStore()
		} else {
			dataStore = persistentStore
			log.Printf("[analysis] Persistent storage enabled: postgres + redis")
		}
	} else {
		log.Printf("⚠️  [analysis] WARNING: NUKARA_POSTGRES_DSN not set, using in-memory store")
		dataStore = store.NewStore()
	}

	// Create server
	srv := analysis.NewServer(dataStore)
	handler := srv.Handler()

	// Get port from environment
	port := getEnv("NUKARA_ANALYSIS_PORT", "8007")
	addr := fmt.Sprintf(":%s", port)

	log.Printf("[analysis] service listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("[analysis] service exited: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
