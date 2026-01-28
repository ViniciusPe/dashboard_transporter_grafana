package http

import (
	"net/http"
	"strings"
)

// CORS middleware: permite qualquer Origin (reflete o Origin do browser).
// Isso resolve o plugin rodando em 3000/3001/qualquer host sem precisar rebuildar config.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))

		// Se veio Origin, reflete. Se não veio (curl/server-to-server), não força.
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			// se você usa cookies/sessão em algum momento:
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Accept, Authorization, Content-Type, X-CSRF-Token, X-Grafana-Org-Id, X-Grafana-User, X-Grafana-Role, X-Grafana-Email, X-Grafana-Device-Id")

		// Preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
