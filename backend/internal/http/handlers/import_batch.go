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
	RequestedBy string   `json:"requestedBy"` // pode ser lista: "a,b;c\n d"
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
	ID      int    `json:"id"`
	Version int    `json:"version"`
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

// aceita: "a,b; c \n d" => ["a","b","c","d"]
func parseRequestedByList(in string) []string {
	in = strings.TrimSpace(in)
	if in == "" {
		return nil
	}

	// normaliza separadores para vírgula
	repl := strings.NewReplacer(";", ",", "\n", ",", "\r", ",", "\t", ",")
	in = repl.Replace(in)

	parts := strings.Split(in, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key := strings.ToLower(p)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}

	return out
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

		// ✅ usa a função que já existe no package (definida em dashboards.go)
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

			// pega título (fallback de search)
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

			// 3) RBAC: Editor (2) pro(s) requestedBy
			requesters := parseRequestedByList(req.RequestedBy)
			if len(requesters) == 0 {
				res.Status = "warning"
				res.Message = "import ok; rbac skipped (requestedBy vazio)"
				results = append(results, res)
				continue
			}

			// resolve dashID
			dashID, warn := resolveDashboardIDAfterImport(dstBase, dst.User, dst.Password, orgID, targetUID, impOut.ID, title)
			if warn != "" {
				res.Status = "warning"
				res.Message = "import ok; rbac failed (" + warn + ")"
				results = append(results, res)
				continue
			}

			// ✅ aplica todos os usuários em UM POST só
			warn = applyDashboardPermissionsByIDMulti(dstBase, dst.User, dst.Password, orgID, dashID, requesters, 2)
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
	if impID > 0 {
		return impID, ""
	}

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

// ✅ aplica permissão no dashboard por ID para VÁRIOS usuários,
// preservando tudo que já existe e garantindo userId com permission desejada.
// Faz 1 GET + 1 POST (não tem sobrescrita por chamada).
func applyDashboardPermissionsByIDMulti(dstBase, adminUser, adminPass, orgID string, dashID int, loginOrEmails []string, permission int) string {
	// 1) resolve todos os userIds
	userIDs := make([]int, 0, len(loginOrEmails))
	failed := make([]string, 0)

	seenIDs := map[int]struct{}{}
	for _, who := range loginOrEmails {
		who = strings.TrimSpace(who)
		if who == "" {
			continue
		}

		lookupURL := dstBase + "/api/users/lookup?loginOrEmail=" + url.QueryEscape(who)
		lreq, _ := http.NewRequest(http.MethodGet, lookupURL, nil)
		lreq.SetBasicAuth(adminUser, adminPass)
		lreq.Header.Set("Accept", "application/json")
		lreq.Header.Set("X-Grafana-Org-Id", orgID)

		lresp, err := http.DefaultClient.Do(lreq)
		if err != nil {
			failed = append(failed, who+" (lookup err: "+err.Error()+")")
			continue
		}
		lbody, _ := io.ReadAll(lresp.Body)
		_ = lresp.Body.Close()

		if lresp.StatusCode >= 300 {
			failed = append(failed, fmt.Sprintf("%s (lookup api %d)", who, lresp.StatusCode))
			continue
		}

		var u grafanaUserLookupResp
		if err := json.Unmarshal(lbody, &u); err != nil {
			failed = append(failed, who+" (lookup decode err)")
			continue
		}
		if u.ID == 0 {
			failed = append(failed, who+" (userId=0)")
			continue
		}

		if _, ok := seenIDs[u.ID]; ok {
			continue
		}
		seenIDs[u.ID] = struct{}{}
		userIDs = append(userIDs, u.ID)
	}

	if len(userIDs) == 0 {
		if len(failed) > 0 {
			return "no valid users; failed: " + strings.Join(failed, "; ")
		}
		return "no valid users"
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

	var current grafanaDashPermGetResp
	_ = json.Unmarshal(pgbody, &current)

	// 3) monta payload preservando entradas + garante TODOS userIds
	itemsOut := make([]map[string]any, 0, len(current.Permissions)+len(userIDs))

	// set dos users que já existem no current
	existingUser := map[int]int{}
	for _, p := range current.Permissions {
		if p.UserID != 0 {
			existingUser[p.UserID] = p.Permission
		}
	}

	// preserva tudo que já existe (users/teams/roles)
	for _, p := range current.Permissions {
		if p.UserID != 0 {
			// se for um dos users alvo, força permission desejada
			if _, ok := seenIDs[p.UserID]; ok {
				itemsOut = append(itemsOut, map[string]any{
					"userId":     p.UserID,
					"permission": permission,
				})
			} else {
				itemsOut = append(itemsOut, map[string]any{
					"userId":     p.UserID,
					"permission": p.Permission,
				})
			}
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

	// adiciona os users alvo que ainda não existiam
	for _, id := range userIDs {
		if _, ok := existingUser[id]; ok {
			continue
		}
		itemsOut = append(itemsOut, map[string]any{
			"userId":     id,
			"permission": permission,
		})
	}

	payload := map[string]any{"items": itemsOut}
	b, _ := json.Marshal(payload)

	// 4) POST permissions (1 vez)
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

	// se alguns falharam no lookup, retorna warning (mas não falha tudo)
	if len(failed) > 0 {
		return "some users failed lookup: " + strings.Join(failed, "; ")
	}

	return ""
}
