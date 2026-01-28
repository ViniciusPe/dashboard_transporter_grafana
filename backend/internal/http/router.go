package http

import (
	"net/http"

	"dashboard-transporter/internal/config"
	"dashboard-transporter/internal/http/handlers"

	"github.com/go-chi/chi/v5"
)

func NewRouter(cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// ✅ middleware novo (sem options)
	r.Use(CORS)

	r.Get("/health", handlers.Health)
	r.Get("/environments", handlers.Environments(cfg))
	r.Get("/dashboards", handlers.Dashboards(cfg))
	r.Get("/folders", handlers.Folders(cfg))
	r.Get("/debug/user/{env}/{username}", handlers.DebugUser(cfg))
	r.Post("/dashboards/import/batch", handlers.ImportDashboardsBatch(cfg))

	return r
}
