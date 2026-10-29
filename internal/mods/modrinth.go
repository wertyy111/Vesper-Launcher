package mods

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"vesper-launcher/internal/models"
)

var versionRegex = regexp.MustCompile(`\b\d+\.\d+(?:\.\d+)?\b`)

// CleanGameVersionAndLoader extracts a clean Minecraft version (e.g. "1.19.2") and loader ("fabric", "forge", "neoforge", "quilt").
func CleanGameVersionAndLoader(raw string) (cleanVersion string, loader string) {
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "neoforge") {
		loader = "neoforge"
	} else if strings.Contains(lower, "fabric") {
		loader = "fabric"
	} else if strings.Contains(lower, "forge") {
		loader = "forge"
	} else if strings.Contains(lower, "quilt") {
		loader = "quilt"
	}

	if m := versionRegex.FindString(raw); m != "" {
		cleanVersion = m
	} else {
		cleanVersion = raw
	}
	return cleanVersion, loader
}

type ModrinthClient struct {
	client *http.Client
}

func NewModrinthClient() *ModrinthClient {
	return &ModrinthClient{
		client: &http.Client{Timeout: 12 * time.Second},
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

type modrinthVersion struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"project_id"`
	Name          string   `json:"name"`
	VersionNumber string   `json:"version_number"`
	GameVersions  []string `json:"game_versions"`
	Loaders       []string `json:"loaders"`
	Files         []struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Primary  bool   `json:"primary"`
		Size     int64  `json:"size"`
	} `json:"files"`
}

// Search queries the Modrinth v2 API for projects.
func (c *ModrinthClient) Search(query, category, rawVersion, rawLoader string, limit int) ([]models.ModItem, error) {
	if limit <= 0 {
		limit = 30
	}

	gameVersion, detectedLoader := CleanGameVersionAndLoader(rawVersion)
	loader := rawLoader
	if loader == "" {
		loader = detectedLoader
	}

	// Map UI category to Modrinth project_type
	projectType := "mod"
	switch category {
	case "Ресурспаки", "Ресурс паки", "resourcepack":
		projectType = "resourcepack"
	case "Сборки", "modpack":
		projectType = "modpack"
	case "Шейдеры", "shader":
		projectType = "shader"
	default:
		projectType = "mod"
	}

	// 1. Initial attempt: with version and loader
	items, err := c.doSearch(query, projectType, gameVersion, loader, limit)
	if err == nil && len(items) > 0 {
		return items, nil
	}

	// 2. If 0 hits and we had a loader constraint for mods, retry without loader
	if loader != "" && projectType == "mod" {
		items, err = c.doSearch(query, projectType, gameVersion, "", limit)
		if err == nil && len(items) > 0 {
			return items, nil
		}
	}

	// 3. If still 0 hits and we had a version constraint, retry without version constraint
	if gameVersion != "" {
		items, err = c.doSearch(query, projectType, "", "", limit)
		if err == nil && len(items) > 0 {
			return items, nil
		}
	}

	return items, err
}

func (c *ModrinthClient) doSearch(query, projectType, gameVersion, loader string, limit int) ([]models.ModItem, error) {
	endpoint := "https://api.modrinth.com/v2/search"
	params := url.Values{}
	if query != "" {
		params.Set("query", query)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("index", "downloads")

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

// GetProjectBestVersion retrieves the primary downloadable file for a project matching version and loader.
func (c *ModrinthClient) GetProjectBestVersion(projectId, rawVersion, rawLoader string) (filename string, downloadUrl string, err error) {
	gameVersion, detectedLoader := CleanGameVersionAndLoader(rawVersion)
	loader := rawLoader
	if loader == "" {
		loader = detectedLoader
	}

	params := url.Values{}
	if loader != "" {
		lJson, _ := json.Marshal([]string{loader})
		params.Set("loaders", string(lJson))
	}
	if gameVersion != "" {
		vJson, _ := json.Marshal([]string{gameVersion})
		params.Set("game_versions", string(vJson))
	}

	endpoint := fmt.Sprintf("https://api.modrinth.com/v2/project/%s/version", url.PathEscape(projectId))
	reqUrl := endpoint
	if len(params) > 0 {
		reqUrl = fmt.Sprintf("%s?%s", endpoint, params.Encode())
	}

	filename, downloadUrl, err = c.fetchPrimaryFileVersion(reqUrl)
	if err == nil && filename != "" && downloadUrl != "" {
		return filename, downloadUrl, nil
	}

	// Fallback without filters
	return c.fetchPrimaryFileVersion(endpoint)
}

func (c *ModrinthClient) fetchPrimaryFileVersion(reqUrl string) (filename string, downloadUrl string, err error) {
	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "VesperLauncher/1.0 (contact@vesperlauncher.org)")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("modrinth version lookup failed with status %s", resp.Status)
	}

	var versions []modrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return "", "", err
	}

	if len(versions) == 0 {
		return "", "", fmt.Errorf("no versions found")
	}

	for _, ver := range versions {
		if len(ver.Files) == 0 {
			continue
		}
		for _, f := range ver.Files {
			if f.Primary && f.URL != "" && f.Filename != "" {
				return f.Filename, f.URL, nil
			}
		}
		if ver.Files[0].URL != "" && ver.Files[0].Filename != "" {
			return ver.Files[0].Filename, ver.Files[0].URL, nil
		}
	}

	return "", "", fmt.Errorf("no valid downloadable files found")
}
