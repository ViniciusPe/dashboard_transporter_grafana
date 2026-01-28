package grafana

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type FolderItem struct {
	UID      string `json:"uid"`
	Title    string `json:"title"`
	ParentUID string `json:"parentUid,omitempty"` // pode vir vazio dependendo da versão
}

type FolderOut struct {
	UID   string `json:"uid"`
	Title string `json:"title"` // aqui vai vir "Time A/Projeto X"
}

// listFoldersPage chama:
// GET /api/folders?parentUid=<uid>&page=<n>&limit=<n>
func (c *Client) listFoldersPage(parentUid string, page, limit int) ([]FolderItem, error) {
	qs := url.Values{}
	if strings.TrimSpace(parentUid) != "" {
		qs.Set("parentUid", parentUid)
	}
	qs.Set("page", fmt.Sprintf("%d", page))
	qs.Set("limit", fmt.Sprintf("%d", limit))

	path := "/api/folders?" + qs.Encode()

	// a resposta é um array
	var out []FolderItem
	if err := c.do("GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListFoldersFlat retorna:
// General (uid="")
// Time A
// Time A/Projeto X
func (c *Client) ListFoldersFlat() ([]FolderOut, error) {
	// Sempre inclui General como opção
	result := []FolderOut{
		{UID: "", Title: "General"},
	}

	// Proteções contra loop (caso API volte algo estranho)
	visited := map[string]bool{}
	const limit = 200
	const maxDepth = 10

	var walk func(parentUid, parentPath string, depth int) error
	walk = func(parentUid, parentPath string, depth int) error {
		if depth > maxDepth {
			return nil
		}

		// paginação
		page := 1
		for {
			items, err := c.listFoldersPage(parentUid, page, limit)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				return nil
			}

			for _, f := range items {
				if f.UID == "" {
					continue
				}
				if visited[f.UID] {
					continue
				}
				visited[f.UID] = true

				var fullPath string
				if parentPath == "" {
					fullPath = f.Title
				} else {
					fullPath = parentPath + "/" + f.Title
				}

				result = append(result, FolderOut{
					UID:   f.UID,
					Title: fullPath,
				})

				// desce pros filhos
				if err := walk(f.UID, fullPath, depth+1); err != nil {
					return err
				}
			}

			// se veio menos que o limit, acabou
			if len(items) < limit {
				break
			}
			page++
		}

		return nil
	}

	// raiz
	if err := walk("", "", 0); err != nil {
		return nil, err
	}

	// ordena por título (mantendo General no topo)
	if len(result) > 1 {
		rest := result[1:]
		sort.Slice(rest, func(i, j int) bool {
			return strings.ToLower(rest[i].Title) < strings.ToLower(rest[j].Title)
		})
		result = append([]FolderOut{result[0]}, rest...)
	}

	return result, nil
}

// (opcional) helper pra debug rápido
func (c *Client) DebugFoldersJSON() (string, error) {
	f, err := c.ListFoldersFlat()
	if err != nil {
		return "", err
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	return string(b), nil
}
