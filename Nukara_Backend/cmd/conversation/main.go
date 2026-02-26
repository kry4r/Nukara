package main

import (
    "log"
    "net/http"

    "nukara/backend/internal/bootstrap"
)

func main() {
    addr, handler := bootstrap.NewHandler("conversation")
    log.Printf("conversation service listening on %s", addr)
    if err := http.ListenAndServe(addr, handler); err != nil {
        log.Fatalf("conversation service exited: %v", err)
    }
}
