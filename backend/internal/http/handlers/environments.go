package handlers

import (
	"encoding/json"
	"net/http"

	"dashboard-transporter/internal/config"
)

func Environments(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg.Environments)
	}
}
