package mods

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"vesper-launcher/internal/models"
)

type ModrinthClient struct {
	client *http.Client
}

func NewModrinthClient() *ModrinthClient {
	return &ModrinthClient{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type modrinthSearchResult struct {
	Hits []struct {
		ProjectID    string   `json:"project_id"`
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		Author       string   `json:"author"`
		IconURL      string   `json:"icon_url"`
		Downloads    int64    `json:"downloads"`
		Followers    int64    `json:"follows"`
		Categories   []string `json:"categories"`
		GameVersions []string `json:"versions"`
		Loaders      []string `json:"loaders"`
		ProjectType  string   `json:"project_type"`
	} `json:"hits"`
	TotalHits int `json:"total_hits"`
}

// Search queries the Modrinth v2 API for projects.
func (c *ModrinthClient) Search(query, category, gameVersion, loader string, limit int) ([]models.ModItem, error) {
	if limit <= 0 {
		limit = 20
	}

	endpoint := "https://api.modrinth.com/v2/search"
	params := url.Values{}
	if query != "" {
		params.Set("query", query)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("index", "downloads")

	// Map UI category to Modrinth project_type
	projectType := "mod"
	switch category {
	case "Ресурспаки", "resourcepack":
		projectType = "resourcepack"
	case "Сборки", "modpack":
		projectType = "modpack"
	case "Шейдеры", "shader":
		projectType = "shader"
	}

	facets := [][]string{
		{fmt.Sprintf("project_type:%s", projectType)},
	}
	if gameVersion != "" {
		facets = append(facets, []string{fmt.Sprintf("versions:%s", gameVersion)})
	}
	if loader != "" && projectType == "mod" {
		facets = append(facets, []string{fmt.Sprintf("categories:%s", loader)})
	}

	facetsJson, _ := json.Marshal(facets)
	params.Set("facets", string(facetsJson))

	reqUrl := fmt.Sprintf("%s?%s", endpoint, params.Encode())
	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "VesperLauncher/1.0 (contact@vesperlauncher.org)")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("modrinth search failed with status %s", resp.Status)
	}

	var res modrinthSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	var items []models.ModItem
	for _, hit := range res.Hits {
		items = append(items, models.ModItem{
			ProjectId:    hit.ProjectID,
			DisplayName:  hit.Title,
			Description:  hit.Description,
			Author:       hit.Author,
			IconUrl:      hit.IconURL,
			Downloads:    hit.Downloads,
			Followers:    hit.Followers,
			ContentKind:  hit.ProjectType,
			Categories:   hit.Categories,
			GameVersions: hit.GameVersions,
			Loaders:      hit.Loaders,
			IsInstalled:  false,
			CanInstall:   true,
			ActionText:   "Установить",
		})
	}

	return items, nil
}
