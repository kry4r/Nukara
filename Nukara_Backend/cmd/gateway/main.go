package main

import (
    "log"
    "net/http"

    "nukara/backend/internal/bootstrap"
)

func main() {
    addr, handler := bootstrap.NewHandler("gateway")
    log.Printf("gateway listening on %s", addr)
    if err := http.ListenAndServe(addr, handler); err != nil {
        log.Fatalf("gateway exited: %v", err)
    }
}
