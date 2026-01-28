package handlers

import (
	"net/http"
	"strings"
)

func grafanaLoggedUserFromHeaders(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Grafana-User")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-Grafana-Email")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-WEBAUTH-USER")); v != "" {
		return v
	}
	return ""
}
