package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"nukara/backend/internal/neo4jadapter"
)

func main() {
	boltURL := envOr("NUKARA_NEO4J_BOLT_URL", "bolt://127.0.0.1:7687")
	username := envOr("NUKARA_NEO4J_USER", "neo4j")
	password := strings.TrimSpace(os.Getenv("NUKARA_NEO4J_PASSWORD"))
	database := envOr("NUKARA_NEO4J_DATABASE", "neo4j")
	host := envOr("NUKARA_NEO4J_ADAPTER_HOST", "127.0.0.1")
	port := envOr("NUKARA_NEO4J_ADAPTER_PORT", "17687")
	addr := host + ":" + port

	store, err := neo4jadapter.NewBoltStore(boltURL, username, password, database)
	if err != nil {
		log.Fatalf("create neo4j adapter store: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := store.Close(ctx); err != nil {
			log.Printf("close neo4j adapter store: %v", err)
		}
	}()

	srv := neo4jadapter.NewServer(store, username, password)
	log.Printf("neo4j adapter listening on http://%s (bolt=%s db=%s)", addr, boltURL, database)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("neo4j adapter listen: %v", err)
	}
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
