package minecraft

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vesper-launcher/internal/platform"
)

const MojangManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

type VersionManifestV2 struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []VersionEntry `json:"versions"`
}

type VersionEntry struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	URL         string    `json:"url"`
	Time        time.Time `json:"time"`
	ReleaseTime time.Time `json:"releaseTime"`
	Sha1        string    `json:"sha1"`
}

type VersionPackage struct {
	ID                 string                      `json:"id"`
	MainClass          string                      `json:"mainClass"`
	InheritsFrom       string                      `json:"inheritsFrom"`
	MinecraftArguments string                      `json:"minecraftArguments"`
	Arguments          *ArgumentRules              `json:"arguments"`
	AssetIndex         *AssetIndexRef              `json:"assetIndex"`
	Assets             string                      `json:"assets"`
	Downloads          map[string]DownloadArtifact `json:"downloads"`
	Libraries          []LibraryEntry              `json:"libraries"`
	JavaVersion        *JavaVersionInfo            `json:"javaVersion"`
}

type ArgumentRules struct {
	Game []any `json:"game"`
	Jvm  []any `json:"jvm"`
}

type AssetIndexRef struct {
	ID        string `json:"id"`
	Sha1      string `json:"sha1"`
	Size      int64  `json:"size"`
	TotalSize int64  `json:"totalSize"`
	URL       string `json:"url"`
}

type DownloadArtifact struct {
	Sha1 string `json:"sha1"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
	Path string `json:"path"`
}

type LibraryEntry struct {
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Downloads LibraryDownloads  `json:"downloads"`
	Natives   map[string]string `json:"natives"`
	Rules     []RuleEntry       `json:"rules"`
}

// MavenCoordinatesToPath converts Maven coordinates (e.g., "group:artifact:version[:classifier]") to a relative file path.
func MavenCoordinatesToPath(mavenName string) (string, error) {
	parts := strings.Split(mavenName, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid maven coordinates: %s", mavenName)
	}

	groupPath := strings.ReplaceAll(parts[0], ".", "/")
	artifact := parts[1]
	version := parts[2]
	ext := "jar"
	classifier := ""

	if len(parts) > 3 {
		classifier = parts[3]
		if atIdx := strings.Index(classifier, "@"); atIdx != -1 {
			ext = classifier[atIdx+1:]
			classifier = classifier[:atIdx]
		}
	} else if atIdx := strings.Index(version, "@"); atIdx != -1 {
		ext = version[atIdx+1:]
		version = version[:atIdx]
	}

	fileName := fmt.Sprintf("%s-%s.%s", artifact, version, ext)
	if classifier != "" {
		fileName = fmt.Sprintf("%s-%s-%s.%s", artifact, version, classifier, ext)
	}

	return filepath.Join(filepath.FromSlash(groupPath), artifact, version, fileName), nil
}

type LibraryDownloads struct {
	Artifact    *DownloadArtifact           `json:"artifact"`
	Classifiers map[string]DownloadArtifact `json:"classifiers"`
}

type RuleEntry struct {
	Action string `json:"action"`
	OS     struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Arch    string `json:"arch"`
	} `json:"os"`
	Features map[string]bool `json:"features"`
}

type JavaVersionInfo struct {
	Component    string `json:"component"`
	MajorVersion int    `json:"majorVersion"`
}

// FetchMojangManifest retrieves or loads the cached Mojang version manifest.
func FetchMojangManifest() (*VersionManifestV2, error) {
	cachePath := filepath.Join(platform.GetMinecraftDir(), "version_manifest_v2.json")

	// Try network first
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(MojangManifestURL)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		bytes, err := io.ReadAll(resp.Body)
		if err == nil {
			var manifest VersionManifestV2
			if err := json.Unmarshal(bytes, &manifest); err == nil {
				_ = os.WriteFile(cachePath, bytes, 0644)
				return &manifest, nil
			}
		}
	}

	// Fallback to cache
	bytes, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest and no cache available")
	}

	var manifest VersionManifestV2
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// LoadVersionPackage loads a specific version's package JSON from disk or Mojang.
func LoadVersionPackage(versionId, versionUrl string) (*VersionPackage, error) {
	mcDir := platform.GetMinecraftDir()
	versionJsonPath := filepath.Join(mcDir, "versions", versionId, versionId+".json")

	if bytes, err := os.ReadFile(versionJsonPath); err == nil {
		var pkg VersionPackage
		if err := json.Unmarshal(bytes, &pkg); err == nil {
			return &pkg, nil
		}
	}

	if versionUrl == "" {
		return nil, fmt.Errorf("version JSON not found locally and no URL provided")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(versionUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	_ = os.MkdirAll(filepath.Dir(versionJsonPath), 0755)
	_ = os.WriteFile(versionJsonPath, bytes, 0644)

	var pkg VersionPackage
	if err := json.Unmarshal(bytes, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}
