package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
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

type updateManifest struct {
	Latest struct {
		Version   string `json:"version"`
		Title     string `json:"title"`
		Changelog string `json:"changelog"`
	} `json:"latest"`
}

// CheckUpdate checks GitHub releases or fallback manifest for an available launcher update.
func (s *UpdateService) CheckUpdate() (*models.UpdatesSnapshot, error) {
	if !s.config.Enabled {
		return &models.UpdatesSnapshot{
			IsAvailable: false,
			Version:     s.currentVersion,
		}, nil
	}

	repoPart := strings.TrimPrefix(s.config.SourceUrl, "https://github.com/")
	repoPart = strings.TrimSuffix(repoPart, "/")

	apiUrl := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoPart)
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err == nil {
		req.Header.Set("User-Agent", "VesperLauncher/1.0")
		if resp, err := s.client.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var rel githubRelease
				if err := json.NewDecoder(resp.Body).Decode(&rel); err == nil {
					latestVer := strings.TrimPrefix(rel.TagName, "v")
					isNewer := isVersionNewer(latestVer, s.currentVersion)
					return &models.UpdatesSnapshot{
						IsAvailable:  isNewer,
						Version:      latestVer,
						Title:        rel.Name,
						ReleaseNotes: rel.Body,
					}, nil
				}
			}
		}
	}

	// Fallback to raw manifest.json from Updates-Vesper repository
	manifestUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/manifest.json", repoPart)
	if mReq, err := http.NewRequest("GET", manifestUrl, nil); err == nil {
		mReq.Header.Set("User-Agent", "VesperLauncher/1.0")
		if mResp, err := s.client.Do(mReq); err == nil {
			defer mResp.Body.Close()
			if mResp.StatusCode == http.StatusOK {
				var manifest updateManifest
				if err := json.NewDecoder(mResp.Body).Decode(&manifest); err == nil {
					latestVer := strings.TrimPrefix(manifest.Latest.Version, "v")
					isNewer := isVersionNewer(latestVer, s.currentVersion)
					return &models.UpdatesSnapshot{
						IsAvailable:  isNewer,
						Version:      latestVer,
						Title:        manifest.Latest.Title,
						ReleaseNotes: manifest.Latest.Changelog,
					}, nil
				}
			}
		}
	}

	return &models.UpdatesSnapshot{
		IsAvailable: false,
		Version:     s.currentVersion,
	}, nil
}

func isVersionNewer(latest, current string) bool {
	latest = strings.TrimSpace(strings.TrimPrefix(latest, "v"))
	current = strings.TrimSpace(strings.TrimPrefix(current, "v"))
	if latest == "" || current == "" {
		return false
	}
	if latest == current {
		return false
	}

	lParts := strings.Split(strings.Split(latest, "-")[0], ".")
	cParts := strings.Split(strings.Split(current, "-")[0], ".")

	for i := 0; i < len(lParts) && i < len(cParts); i++ {
		lNum, err1 := strconv.Atoi(lParts[i])
		cNum, err2 := strconv.Atoi(cParts[i])
		if err1 == nil && err2 == nil {
			if lNum > cNum {
				return true
			}
			if lNum < cNum {
				return false
			}
		}
	}

	return len(lParts) > len(cParts)
}
