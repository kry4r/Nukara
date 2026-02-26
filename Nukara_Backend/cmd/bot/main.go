package main

import (
    "log"
    "net/http"

    "nukara/backend/internal/bootstrap"
)

func main() {
    addr, handler := bootstrap.NewHandler("bot")
    log.Printf("bot service listening on %s", addr)
    if err := http.ListenAndServe(addr, handler); err != nil {
        log.Fatalf("bot service exited: %v", err)
    }
}
