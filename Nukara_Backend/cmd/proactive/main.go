package main

import (
    "log"
    "net/http"

    "nukara/backend/internal/bootstrap"
)

func main() {
    addr, handler := bootstrap.NewHandler("proactive")
    log.Printf("proactive service listening on %s", addr)
    if err := http.ListenAndServe(addr, handler); err != nil {
        log.Fatalf("proactive service exited: %v", err)
    }
}
