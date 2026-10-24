package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"vesper-launcher/internal/models"
)

type UpdateConfig struct {
	Enabled           bool   `json:"Enabled"`
	CheckOnStartup    bool   `json:"CheckOnStartup"`
	SourceType        string `json:"SourceType"`
	SourceUrl         string `json:"SourceUrl"`
	IncludePrerelease bool   `json:"IncludePrerelease"`
}

type UpdateService struct {
	client         *http.Client
	currentVersion string
	config         UpdateConfig
}

func NewUpdateService(currentVer string) *UpdateService {
	s := &UpdateService{
		client:         &http.Client{Timeout: 10 * time.Second},
		currentVersion: currentVer,
	}
	s.loadConfig()
	return s
}

func (s *UpdateService) loadConfig() {
	b, err := os.ReadFile("update-config.json")
	if err == nil {
		_ = json.Unmarshal(b, &s.config)
	}
	if s.config.SourceUrl == "" {
		s.config.SourceUrl = "https://github.com/wertyy111/Updates-Vesper"
		s.config.Enabled = true
	}
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// CheckUpdate checks GitHub releases for an available launcher update.
func (s *UpdateService) CheckUpdate() (*models.UpdatesSnapshot, error) {
	if !s.config.Enabled {
		return &models.UpdatesSnapshot{
			IsAvailable: false,
			Version:     s.currentVersion,
		}, nil
	}

	// Extract repo path: e.g. "wertyy111/Updates-Vesper"
	repoPart := strings.TrimPrefix(s.config.SourceUrl, "https://github.com/")
	repoPart = strings.TrimSuffix(repoPart, "/")

	apiUrl := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoPart)
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "VesperLauncher/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &models.UpdatesSnapshot{
			IsAvailable: false,
			Version:     s.currentVersion,
		}, nil
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	latestVer := strings.TrimPrefix(rel.TagName, "v")
	isNewer := isVersionNewer(latestVer, s.currentVersion)

	return &models.UpdatesSnapshot{
		IsAvailable:  isNewer,
		Version:      latestVer,
		Title:        rel.Name,
		ReleaseNotes: rel.Body,
	}, nil
}

func isVersionNewer(latest, current string) bool {
	latest = strings.TrimSpace(latest)
	current = strings.TrimSpace(current)
	if latest == "" || current == "" {
		return false
	}
	return latest != current
}
