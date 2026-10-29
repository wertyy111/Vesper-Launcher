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

// GetLibraryIdentity returns the unique groupId:artifactId key for deduplication and overriding.
func GetLibraryIdentity(name string) string {
	ext := "jar"
	norm := strings.TrimSpace(name)
	if atIdx := strings.Index(norm, "@"); atIdx != -1 {
		ext = norm[atIdx+1:]
		norm = norm[:atIdx]
	}
	parts := strings.Split(norm, ":")
	if len(parts) < 3 {
		return name
	}
	groupId := parts[0]
	artifactId := parts[1]
	classifier := ""
	if len(parts) >= 4 {
		classifier = parts[3]
	}
	if classifier != "" {
		return fmt.Sprintf("%s:%s:%s@%s", groupId, artifactId, classifier, ext)
	}
	return fmt.Sprintf("%s:%s@%s", groupId, artifactId, ext)
}

// MergeLibraries merges parent and child libraries, letting the child override matching (groupId:artifactId) libraries.
func MergeLibraries(parentLibs, childLibs []LibraryEntry) []LibraryEntry {
	indexByKey := make(map[string]int)
	var merged []LibraryEntry

	addOrOverride := func(lib LibraryEntry) {
		key := GetLibraryIdentity(lib.Name)
		if idx, exists := indexByKey[key]; exists {
			merged[idx] = lib
		} else {
			indexByKey[key] = len(merged)
			merged = append(merged, lib)
		}
	}

	for _, l := range parentLibs {
		addOrOverride(l)
	}
	for _, l := range childLibs {
		addOrOverride(l)
	}

	return merged
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

// FetchMojangManifest retrieves or loads the cached Mojang version manifest, with automatic BMCLAPI fallback.
func FetchMojangManifest() (*VersionManifestV2, error) {
	cachePath := filepath.Join(platform.GetMinecraftDir(), "version_manifest_v2.json")

	urls := []string{
		"https://piston-meta.mojang.com/mc/game/version_manifest_v2.json",
		"https://bmclapi2.bangbang93.com/mc/game/version_manifest_v2.json",
		"https://launchermeta.mojang.com/mc/game/version_manifest_v2.json",
	}

	client := &http.Client{Timeout: 15 * time.Second}
	for _, u := range urls {
		resp, err := client.Get(u)
		if err == nil && resp.StatusCode == 200 {
			data, errR := io.ReadAll(resp.Body)
			resp.Body.Close()
			if errR == nil {
				var manifest VersionManifestV2
				if err := json.Unmarshal(data, &manifest); err == nil && len(manifest.Versions) > 0 {
					_ = os.WriteFile(cachePath, data, 0644)
					return &manifest, nil
				}
			}
		} else if resp != nil {
			resp.Body.Close()
		}
	}

	// Fallback to cache
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest and no cache available")
	}

	var manifest VersionManifestV2
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// LoadVersionPackage loads a specific version's package JSON from disk or Mojang/mirrors.
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

	candidates := GetDownloadFallbackURLs(versionUrl)
	client := &http.Client{Timeout: 20 * time.Second}

	var data []byte
	var fetchErr error
	for _, u := range candidates {
		resp, err := client.Get(u)
		if err == nil && resp.StatusCode == 200 {
			b, errR := io.ReadAll(resp.Body)
			resp.Body.Close()
			if errR == nil && len(b) > 0 {
				data = b
				fetchErr = nil
				break
			}
		} else {
			if resp != nil {
				resp.Body.Close()
			}
			fetchErr = err
		}
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("failed to download version package from %s: %v", versionUrl, fetchErr)
	}

	_ = os.MkdirAll(filepath.Dir(versionJsonPath), 0755)
	_ = os.WriteFile(versionJsonPath, data, 0644)

	var pkg VersionPackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}
