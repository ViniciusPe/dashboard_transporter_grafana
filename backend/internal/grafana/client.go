package grafana

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Client representa o cliente para API do Grafana
type Client struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

// NewClient cria um novo cliente Grafana
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		client:   &http.Client{},
	}
}

// do executa uma requisição HTTP para a API do Grafana
func (c *Client) do(method, path string, body interface{}, out interface{}) error {
	var reqBodyReader *strings.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBodyReader = strings.NewReader(string(b))
	} else {
		reqBodyReader = strings.NewReader("")
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBodyReader)
	if err != nil {
		return err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.username, c.password)))

	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	log.Printf("[GRAFANA API] %s %s", method, c.baseURL+path)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Ler o corpo da resposta para debug
	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var e map[string]interface{}
		_ = json.Unmarshal(bodyBytes, &e)
		return fmt.Errorf("grafana api error (%d): %v", resp.StatusCode, e)
	}

	// Re-criar o reader para decodificar
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	return nil
}

// DashboardSearchItem representa um item na lista de dashboards
type DashboardSearchItem struct {
	ID    int    `json:"id"`
	UID   string `json:"uid"`
	Title string `json:"title"`
}

// ListDashboards lista todos os dashboards
func (c *Client) ListDashboards() ([]DashboardSearchItem, error) {
	var out []DashboardSearchItem
	err := c.do("GET", "/api/search?type=dash-db", nil, &out)
	return out, err
}

// DashboardFullResponse representa a resposta completa da API do Grafana
type DashboardFullResponse struct {
	Dashboard map[string]interface{} `json:"dashboard"`
	Meta      map[string]interface{} `json:"meta"`
}

// GetDashboardByUID obtém um dashboard pelo UID
func (c *Client) GetDashboardByUID(uid string) (map[string]interface{}, error) {
	var response DashboardFullResponse
	err := c.do("GET", "/api/dashboards/uid/"+uid, nil, &response)
	if err != nil {
		return nil, err
	}

	log.Printf("[GRAFANA API] Dashboard %s found, title: %v", uid, response.Dashboard["title"])
	return response.Dashboard, nil
}

// sanitizeDashboardForImport evita conflitos de ID (folder x dashboard) no destino.
// IMPORTANTE: NUNCA enviar `id` do ambiente de origem.
func sanitizeDashboardForImport(src map[string]interface{}) map[string]interface{} {
	// cópia rasa (já resolve aqui)
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}

	// Essas chaves causam dor no import se vierem do DEV
	// - id: conflita com folders/dashboards no destino
	// - version: em import normalmente deve ser 0
	// - uid: mantém (é a “identidade” que você quer transportar)
	dst["id"] = nil
	dst["version"] = 0

	// Se existir (às vezes aparece em dashboards mais novos)
	delete(dst, "meta")
	delete(dst, "folderId")
	delete(dst, "folderUid")
	delete(dst, "folderTitle")

	return dst
}

// ImportDashboard importa um dashboard
func (c *Client) ImportDashboard(dashboard map[string]interface{}, folderUID string) (string, error) {
	safeDash := sanitizeDashboardForImport(dashboard)

	payload := map[string]interface{}{
		"dashboard": safeDash,
		"overwrite": true,
		"message":   "Imported by Dashboard Transporter",
	}

	if folderUID != "" {
		payload["folderUid"] = folderUID
	}

	log.Printf("[GRAFANA API] Importing dashboard, title: %v, folder: %s",
		safeDash["title"], folderUID)

	var resp struct {
		UID    string `json:"uid"`
		Status string `json:"status"`
		Title  string `json:"title"`
	}

	err := c.do("POST", "/api/dashboards/db", payload, &resp)
	if err != nil {
		log.Printf("[GRAFANA API] Import failed: %v", err)
		return "", err
	}

	log.Printf("[GRAFANA API] Import successful: %s (title: %s)", resp.UID, resp.Title)
	return resp.UID, nil
}

// GetUserID obtém o ID de um usuário pelo login/email
func (c *Client) GetUserID(login string) (int, error) {
	var resp struct {
		ID int `json:"id"`
	}

	err := c.do("GET", "/api/users/lookup?loginOrEmail="+login, nil, &resp)
	return resp.ID, err
}

// SetDashboardPermissions define permissões para um dashboard
func (c *Client) SetDashboardPermissions(dashboardUID string, userID int) error {
	payload := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"userId":     userID,
				"permission": 2, // Editor = 2, View = 1, Admin = 4
			},
		},
	}

	log.Printf("[GRAFANA API] Setting permissions for dashboard %s, user ID: %d", dashboardUID, userID)

	return c.do(
		"POST",
		"/api/dashboards/uid/"+dashboardUID+"/permissions",
		payload,
		nil,
	)
}
