package main

import (
	"log"
	"os"

	"nukara/backend/internal/admin"
)

func main() {
	port := os.Getenv("NUKARA_ADMIN_PORT")
	if port == "" {
		port = "9527"
	}

	server := admin.NewServer()
	log.Printf("Admin service starting on port %s", port)
	if err := server.Start(port); err != nil {
		log.Fatalf("Failed to start admin service: %v", err)
	}
}
