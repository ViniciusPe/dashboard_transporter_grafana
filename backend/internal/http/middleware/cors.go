package middleware

import (
	"net/http"
	"strings"
)

// CORSOptions controla como vamos liberar CORS.
// - AllowedOrigins vazio => permite qualquer Origin (reflete o origin do request)
// - Se preencher, valida contra a lista
type CORSOptions struct {
	AllowedOrigins []string
}

func CORS(opts CORSOptions) func(http.Handler) http.Handler {
	allowedAll := len(opts.AllowedOrigins) == 0

	allowed := make(map[string]struct{}, len(opts.AllowedOrigins))
	for _, o := range opts.AllowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Se não tem Origin, é chamada server-to-server / curl — segue normal
			if origin != "" {
				if allowedAll {
					// DEV/TEST: reflete qualquer origin
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else {
					// PROD: só libera whitelisted
					if _, ok := allowed[origin]; ok {
						w.Header().Set("Access-Control-Allow-Origin", origin)
					} else {
						// Origin não permitido -> não seta header (browser bloqueia)
						// mas ainda deixa o backend responder pra calls internas
					}
				}

				// importante pra variar por Origin (cache proxy etc.)
				w.Header().Add("Vary", "Origin")

				// como seu frontend pode usar cookies/auth do Grafana no futuro,
				// já deixa habilitado.
				w.Header().Set("Access-Control-Allow-Credentials", "true")

				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers",
					"Accept, Authorization, Content-Type, X-CSRF-Token, X-Grafana-Org-Id, X-Grafana-User, X-Grafana-Role, X-Grafana-Device-Id",
				)
			}

			// Preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
