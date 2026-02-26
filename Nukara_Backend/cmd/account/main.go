package main

import (
    "log"
    "net/http"

    "nukara/backend/internal/bootstrap"
)

func main() {
    addr, handler := bootstrap.NewHandler("account")
    log.Printf("account service listening on %s", addr)
    if err := http.ListenAndServe(addr, handler); err != nil {
        log.Fatalf("account service exited: %v", err)
    }
}
