package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"dashboard-transporter/internal/config"
)

type dashboardOut struct {
	ID    int    `json:"id"`
	UID   string `json:"uid"`
	Title string `json:"title"`
}

type grafanaSearchItem struct {
	ID    int    `json:"id"`
	UID   string `json:"uid"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

func Dashboards(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID := r.URL.Query().Get("env")
		if envID == "" {
			http.Error(w, "missing env", http.StatusBadRequest)
			return
		}

		env := cfg.GetEnvironment(envID)
		if env == nil {
			http.Error(w, "unknown env: "+envID, http.StatusBadRequest)
			return
		}

		base := stringsTrimRightSlash(env.URL)
		endpoint := base + "/api/search?type=dash-db"

		req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
		req.SetBasicAuth(env.User, env.Password)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			http.Error(w, fmt.Sprintf("grafana api error (%d) on search", resp.StatusCode), http.StatusInternalServerError)
			return
		}

		var items []grafanaSearchItem
		if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
			http.Error(w, "failed to decode grafana response: "+err.Error(), http.StatusInternalServerError)
			return
		}

		out := make([]dashboardOut, 0, len(items))
		for _, it := range items {
			if it.Type != "" && it.Type != "dash-db" {
				continue
			}
			out = append(out, dashboardOut{ID: it.ID, UID: it.UID, Title: it.Title})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)

		_ = url.QueryEscape("") // mantém import url usado nesse arquivo sem warnings em alguns linters
	}
}

// util local (sem depender de outros arquivos)
func stringsTrimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
