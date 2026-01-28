package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"dashboard-transporter/internal/config"
)

type importBatchRequest struct {
	SourceEnv   string   `json:"sourceEnv"`
	TargetEnv   string   `json:"targetEnv"`
	FolderUID   string   `json:"folderUid"`
	RequestedBy string   `json:"requestedBy"` // login/email no Grafana DESTINO
	UIDs        []string `json:"uids"`
}

type importBatchResult struct {
	SourceUID string `json:"sourceUid"`
	TargetUID string `json:"targetUid,omitempty"`
	Status    string `json:"status"`            // ok | warning | error
	Message   string `json:"message,omitempty"` // detalhes
}

type grafanaDashboardGetResp struct {
	Meta struct {
		ID  int    `json:"id"`
		UID string `json:"uid"`
	} `json:"meta"`
	Dashboard map[string]any `json:"dashboard"`
}

type grafanaImportReq struct {
	Dashboard map[string]any `json:"dashboard"`
	FolderUID string         `json:"folderUid"`
	Overwrite bool           `json:"overwrite"`
}

type grafanaImportResp struct {
	Status  string `json:"status"`
	UID     string `json:"uid"`
	Slug    string `json:"slug"`
	ID      int    `json:"id"`      // <-- IMPORTANTISSIMO (muitas versões retornam)
	Version int    `json:"version"` // opcional
}

type grafanaUserLookupResp struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

type grafanaDashPermGetResp struct {
	DashboardID int `json:"dashboardId"`
	Permissions []struct {
		ID         int    `json:"id"`
		UserID     int    `json:"userId"`
		UserLogin  string `json:"userLogin"`
		UserEmail  string `json:"userEmail"`
		TeamID     int    `json:"teamId"`
		Role       string `json:"role"`
		Permission int    `json:"permission"`
	} `json:"permissions"`
}

func getOrgIDFromRequest(r *http.Request) string {
	v := strings.TrimSpace(r.Header.Get("X-Grafana-Org-Id"))
	if v == "" {
		return "1"
	}
	return v
}

func ImportDashboardsBatch(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := getOrgIDFromRequest(r)

		var req importBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
		if req.SourceEnv == "" || req.TargetEnv == "" {
			http.Error(w, "sourceEnv and targetEnv are required", http.StatusBadRequest)
			return
		}
		if len(req.UIDs) == 0 {
			http.Error(w, "uids is required", http.StatusBadRequest)
			return
		}

		src := cfg.GetEnvironment(req.SourceEnv)
		dst := cfg.GetEnvironment(req.TargetEnv)
		if src == nil || dst == nil {
			http.Error(w, "unknown sourceEnv or targetEnv", http.StatusBadRequest)
			return
		}

		srcBase := stringsTrimRightSlash(src.URL)
		dstBase := stringsTrimRightSlash(dst.URL)

		results := make([]importBatchResult, 0, len(req.UIDs))

		for _, uid := range req.UIDs {
			res := importBatchResult{SourceUID: uid}

			// 1) GET dashboard do SOURCE
			getURL := srcBase + "/api/dashboards/uid/" + url.PathEscape(uid)
			getReq, _ := http.NewRequest(http.MethodGet, getURL, nil)
			getReq.SetBasicAuth(src.User, src.Password)
			getReq.Header.Set("Accept", "application/json")
			getReq.Header.Set("X-Grafana-Org-Id", orgID)

			getResp, err := http.DefaultClient.Do(getReq)
			if err != nil {
				res.Status = "error"
				res.Message = "source get failed: " + err.Error()
				results = append(results, res)
				continue
			}
			bodyBytes, _ := io.ReadAll(getResp.Body)
			_ = getResp.Body.Close()

			if getResp.StatusCode >= 300 {
				res.Status = "error"
				res.Message = fmt.Sprintf("source get failed: grafana api %d: %s", getResp.StatusCode, string(bodyBytes))
				results = append(results, res)
				continue
			}

			var dashGet grafanaDashboardGetResp
			if err := json.Unmarshal(bodyBytes, &dashGet); err != nil {
				res.Status = "error"
				res.Message = "source decode failed: " + err.Error()
				results = append(results, res)
				continue
			}
			if dashGet.Dashboard == nil {
				res.Status = "error"
				res.Message = "source returned empty dashboard"
				results = append(results, res)
				continue
			}

			// pega título (vamos usar no fallback de search)
			title, _ := dashGet.Dashboard["title"].(string)

			// Sanitização p/ import
			dashGet.Dashboard["id"] = nil
			dashGet.Dashboard["version"] = 0

			// 2) IMPORT no TARGET
			importPayload := grafanaImportReq{
				Dashboard: dashGet.Dashboard,
				FolderUID: req.FolderUID, // "" = General
				Overwrite: true,
			}

			b, _ := json.Marshal(importPayload)
			importURL := dstBase + "/api/dashboards/db"
			impReq, _ := http.NewRequest(http.MethodPost, importURL, bytes.NewReader(b))
			impReq.Header.Set("Content-Type", "application/json")
			impReq.Header.Set("Accept", "application/json")
			impReq.Header.Set("X-Grafana-Org-Id", orgID)
			impReq.SetBasicAuth(dst.User, dst.Password)

			impResp, err := http.DefaultClient.Do(impReq)
			if err != nil {
				res.Status = "error"
				res.Message = "target import failed: " + err.Error()
				results = append(results, res)
				continue
			}
			impBody, _ := io.ReadAll(impResp.Body)
			_ = impResp.Body.Close()

			if impResp.StatusCode >= 300 {
				res.Status = "error"
				res.Message = fmt.Sprintf("target import failed: grafana api %d: %s", impResp.StatusCode, string(impBody))
				results = append(results, res)
				continue
			}

			var impOut grafanaImportResp
			_ = json.Unmarshal(impBody, &impOut)

			targetUID := impOut.UID
			if targetUID == "" {
				targetUID = uid
			}
			res.TargetUID = targetUID

			// 3) RBAC: SEMPRE EDITOR (2) no DASHBOARD pro requestedBy
			requester := strings.TrimSpace(req.RequestedBy)
			if requester == "" {
				res.Status = "warning"
				res.Message = "import ok; rbac skipped (requestedBy vazio)"
				results = append(results, res)
				continue
			}

			// resolve dashID (prioridade: importResp.id -> GET uid meta.id -> search por title)
			dashID, warn := resolveDashboardIDAfterImport(dstBase, dst.User, dst.Password, orgID, targetUID, impOut.ID, title)
			if warn != "" {
				res.Status = "warning"
				res.Message = "import ok; rbac failed (" + warn + ")"
				results = append(results, res)
				continue
			}

			warn = applyDashboardPermissionsByID(dstBase, dst.User, dst.Password, orgID, dashID, requester, 2)
			if warn != "" {
				res.Status = "warning"
				res.Message = "import ok; rbac failed (" + warn + ")"
				results = append(results, res)
				continue
			}

			res.Status = "ok"
			results = append(results, res)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(results)
	}
}

// resolveDashboardIDAfterImport tenta descobrir o dashID do destino usando:
// 1) impID (resposta do POST /api/dashboards/db)
// 2) GET /api/dashboards/uid/<uid> -> meta.id
// 3) /api/search?type=dash-db&query=<title> e casa uid
func resolveDashboardIDAfterImport(dstBase, adminUser, adminPass, orgID, dashboardUID string, impID int, title string) (int, string) {
	// 1) Melhor caso: Grafana retornou o ID direto no import
	if impID > 0 {
		return impID, ""
	}

	// 2) Tenta via GET /api/dashboards/uid/<uid>
	getURL := dstBase + "/api/dashboards/uid/" + url.PathEscape(dashboardUID)
	greq, _ := http.NewRequest(http.MethodGet, getURL, nil)
	greq.SetBasicAuth(adminUser, adminPass)
	greq.Header.Set("Accept", "application/json")
	greq.Header.Set("X-Grafana-Org-Id", orgID)

	gresp, err := http.DefaultClient.Do(greq)
	if err != nil {
		return 0, "get dash by uid: " + err.Error()
	}
	gbody, _ := io.ReadAll(gresp.Body)
	_ = gresp.Body.Close()

	if gresp.StatusCode >= 300 {
		return 0, fmt.Sprintf("get dash by uid grafana api %d: %s", gresp.StatusCode, string(gbody))
	}

	var dashGet grafanaDashboardGetResp
	if err := json.Unmarshal(gbody, &dashGet); err != nil {
		return 0, "decode dash by uid: " + err.Error()
	}

	if dashGet.Meta.ID > 0 {
		return dashGet.Meta.ID, ""
	}

	// 3) Fallback final: search por title (porque search NÃO acha por UID)
	title = strings.TrimSpace(title)
	if title == "" {
		return 0, "dash id not found (meta.id=0 and title empty)"
	}

	type searchItem struct {
		ID  int    `json:"id"`
		UID string `json:"uid"`
	}
	searchURL := dstBase + "/api/search?type=dash-db&query=" + url.QueryEscape(title)
	sreq, _ := http.NewRequest(http.MethodGet, searchURL, nil)
	sreq.SetBasicAuth(adminUser, adminPass)
	sreq.Header.Set("Accept", "application/json")
	sreq.Header.Set("X-Grafana-Org-Id", orgID)

	sresp, err := http.DefaultClient.Do(sreq)
	if err != nil {
		return 0, "search by title: " + err.Error()
	}
	sbody, _ := io.ReadAll(sresp.Body)
	_ = sresp.Body.Close()

	if sresp.StatusCode >= 300 {
		return 0, fmt.Sprintf("search by title grafana api %d: %s", sresp.StatusCode, string(sbody))
	}

	var items []searchItem
	if err := json.Unmarshal(sbody, &items); err != nil {
		return 0, "decode search by title: " + err.Error()
	}

	for _, it := range items {
		if it.UID == dashboardUID && it.ID > 0 {
			return it.ID, ""
		}
	}

	return 0, "dash id not found (meta.id=0 and search-by-title did not match uid)"
}

// aplica permissão no dashboard por ID, preservando o que já existe, e garantindo userId com permission desejada
func applyDashboardPermissionsByID(dstBase, adminUser, adminPass, orgID string, dashID int, loginOrEmail string, permission int) string {
	// 1) lookup user id
	lookupURL := dstBase + "/api/users/lookup?loginOrEmail=" + url.QueryEscape(loginOrEmail)
	lreq, _ := http.NewRequest(http.MethodGet, lookupURL, nil)
	lreq.SetBasicAuth(adminUser, adminPass)
	lreq.Header.Set("Accept", "application/json")
	lreq.Header.Set("X-Grafana-Org-Id", orgID)

	lresp, err := http.DefaultClient.Do(lreq)
	if err != nil {
		return "lookup: " + err.Error()
	}
	lbody, _ := io.ReadAll(lresp.Body)
	_ = lresp.Body.Close()

	if lresp.StatusCode >= 300 {
		return fmt.Sprintf("lookup grafana api %d: %s", lresp.StatusCode, string(lbody))
	}

	var u grafanaUserLookupResp
	if err := json.Unmarshal(lbody, &u); err != nil {
		return "decode lookup: " + err.Error()
	}
	if u.ID == 0 {
		return "lookup returned userId=0"
	}

	// 2) GET current permissions
	permURL := fmt.Sprintf("%s/api/dashboards/id/%d/permissions", dstBase, dashID)
	pgreq, _ := http.NewRequest(http.MethodGet, permURL, nil)
	pgreq.SetBasicAuth(adminUser, adminPass)
	pgreq.Header.Set("Accept", "application/json")
	pgreq.Header.Set("X-Grafana-Org-Id", orgID)

	pgresp, err := http.DefaultClient.Do(pgreq)
	if err != nil {
		return "get perms: " + err.Error()
	}
	pgbody, _ := io.ReadAll(pgresp.Body)
	_ = pgresp.Body.Close()

	if pgresp.StatusCode >= 300 {
		return fmt.Sprintf("get perms grafana api %d: %s", pgresp.StatusCode, string(pgbody))
	}

	// 3) monta payload preservando entradas + garante userId
	var current grafanaDashPermGetResp
	_ = json.Unmarshal(pgbody, &current)

	itemsOut := make([]map[string]any, 0, len(current.Permissions)+1)

	found := false
	for _, p := range current.Permissions {
		if p.UserID == u.ID {
			itemsOut = append(itemsOut, map[string]any{
				"userId":     u.ID,
				"permission": permission,
			})
			found = true
			continue
		}

		if p.UserID != 0 {
			itemsOut = append(itemsOut, map[string]any{
				"userId":     p.UserID,
				"permission": p.Permission,
			})
			continue
		}
		if p.TeamID != 0 {
			itemsOut = append(itemsOut, map[string]any{
				"teamId":     p.TeamID,
				"permission": p.Permission,
			})
			continue
		}
		if p.Role != "" {
			itemsOut = append(itemsOut, map[string]any{
				"role":       p.Role,
				"permission": p.Permission,
			})
			continue
		}
	}

	if !found {
		itemsOut = append(itemsOut, map[string]any{
			"userId":     u.ID,
			"permission": permission,
		})
	}

	payload := map[string]any{"items": itemsOut}
	b, _ := json.Marshal(payload)

	// 4) POST permissions
	ppreq, _ := http.NewRequest(http.MethodPost, permURL, bytes.NewReader(b))
	ppreq.SetBasicAuth(adminUser, adminPass)
	ppreq.Header.Set("Content-Type", "application/json")
	ppreq.Header.Set("Accept", "application/json")
	ppreq.Header.Set("X-Grafana-Org-Id", orgID)

	ppresp, err := http.DefaultClient.Do(ppreq)
	if err != nil {
		return "post perms: " + err.Error()
	}
	ppbody, _ := io.ReadAll(ppresp.Body)
	_ = ppresp.Body.Close()

	if ppresp.StatusCode >= 300 {
		return fmt.Sprintf("post perms grafana api %d: %s", ppresp.StatusCode, string(ppbody))
	}

	return ""
}
