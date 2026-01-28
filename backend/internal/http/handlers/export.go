package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"dashboard-transporter/internal/config"
	"dashboard-transporter/internal/grafana"
)

func ExportDashboard(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID := r.URL.Query().Get("env")
		uid := r.URL.Query().Get("uid")

		if envID == "" || uid == "" {
			http.Error(w, "env and uid are required", http.StatusBadRequest)
			return
		}

		env, ok := cfg.FindEnvironment(envID)
		if !ok {
			http.Error(w, "environment not found", http.StatusNotFound)
			return
		}

		log.Printf("[HANDLER] Export - Environment: %s, UID: %s", env.ID, uid)

		// ✅ Usar credenciais do config
		client := grafana.NewClient(env.URL, env.User, env.Password)

		dashboard, err := client.GetDashboardByUID(uid)
		if err != nil {
			log.Printf("[HANDLER] Error getting dashboard: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dashboard)
	}
}