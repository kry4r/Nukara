package admin

import (
	"encoding/base64"
	"net/http"
	"os"
	"strings"
)

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := os.Getenv("NUKARA_ADMIN_USERNAME")
		password := os.Getenv("NUKARA_ADMIN_PASSWORD")

		if username == "" || password == "" {
			http.Error(w, "Admin credentials not configured", http.StatusInternalServerError)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Admin Portal"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(auth, "Basic ") {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		payload, err := base64.StdEncoding.DecodeString(auth[6:])
		if err != nil {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[0] != username || pair[1] != password {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
