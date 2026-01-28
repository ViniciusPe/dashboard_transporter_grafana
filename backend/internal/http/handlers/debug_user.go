package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"dashboard-transporter/internal/config"
)

type grafanaUserLookup struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

func DebugUser(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID := chi.URLParam(r, "env")
		username := chi.URLParam(r, "username")

		if envID == "" || username == "" {
			http.Error(w, "missing env or username", http.StatusBadRequest)
			return
		}

		env := cfg.GetEnvironment(envID)
		if env == nil {
			http.Error(w, "unknown env: "+envID, http.StatusBadRequest)
			return
		}

		base := stringsTrimRightSlash(env.URL)
		endpoint := base + "/api/users/lookup?loginOrEmail=" + url.QueryEscape(username)

		req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
		req.SetBasicAuth(env.User, env.Password)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if resp.StatusCode >= 300 {
			http.Error(w, fmt.Sprintf("grafana api error (%d) on user lookup", resp.StatusCode), http.StatusInternalServerError)
			return
		}

		var u grafanaUserLookup
		if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
			http.Error(w, "failed to decode grafana response: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(u)
	}
}
