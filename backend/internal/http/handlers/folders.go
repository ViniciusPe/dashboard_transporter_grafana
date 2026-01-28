package handlers

import (
	"encoding/json"
	"net/http"

	"dashboard-transporter/internal/config"
	"dashboard-transporter/internal/grafana"
)

type folderOut struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
}

func Folders(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID := r.URL.Query().Get("env")
		if envID == "" {
			http.Error(w, "missing env", http.StatusBadRequest)
			return
		}

		client, err := grafana.NewClientFromEnv(cfg, envID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		folders, err := client.ListFoldersFlat() // <- precisa existir no folders.go do grafana
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		out := make([]folderOut, 0, len(folders))
		for _, f := range folders {
			out = append(out, folderOut{
				UID:   f.UID,
				Title: f.Title,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}
