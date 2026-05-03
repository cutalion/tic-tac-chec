package router

import (
	"net/http"
	"tic-tac-chec/internal/web/config"
)

func corsMiddleware(cfg config.Server) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return corsHandler(next, cfg)
	}
}

func corsHandler(next http.Handler, cfg config.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(cfg.AllowedOrigins) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		for _, origin := range cfg.AllowedOrigins {
			if r.Header.Get("Origin") == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				break
			}
		}

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	}
}
