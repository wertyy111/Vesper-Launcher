package updater

import "time"

// PlatformKey identifies OS and architecture (e.g., "linux-amd64", "windows-amd64", "darwin-arm64").
type PlatformKey string

const (
	PlatformLinuxAmd64   PlatformKey = "linux-amd64"
	PlatformLinuxArm64   PlatformKey = "linux-arm64"
	PlatformWindowsAmd64 PlatformKey = "windows-amd64"
	PlatformDarwinArm64  PlatformKey = "darwin-arm64"
	PlatformDarwinAmd64  PlatformKey = "darwin-amd64"
)

// Asset represents a downloadable release artifact with verification metadata.
type Asset struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type,omitempty"`
	Kind        string `json:"kind"` // "installer", "portable", "appimage", "dmg", "deb"
}

// ReleaseInfo holds metadata for a specific release version.
type ReleaseInfo struct {
	Version      string               `json:"version"`
	Tag          string               `json:"tag"`
	ReleaseDate  time.Time            `json:"release_date"`
	Title        string               `json:"title"`
	Changelog    string               `json:"changelog"`
	Prerelease   bool                 `json:"prerelease"`
	Mandatory    bool                 `json:"mandatory"`
	MinSupported string               `json:"min_supported_version,omitempty"`
	Assets       map[PlatformKey]Asset `json:"assets"`
}

// NewsItem represents an announcement or update card shown in the launcher UI.
type NewsItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	ContentURL  string    `json:"content_url,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	Badge       string    `json:"badge,omitempty"`
}

// UpdateManifest is the root payload served by the Vesper update system.
type UpdateManifest struct {
	SchemaVersion string                 `json:"schema_version"`
	Latest        ReleaseInfo            `json:"latest"`
	Releases      []ReleaseInfo          `json:"releases,omitempty"`
	News          []NewsItem             `json:"news,omitempty"`
	Mirrors       []string               `json:"mirrors,omitempty"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// CheckUpdateResult is the response returned when checking for updates.
type CheckUpdateResult struct {
	UpdateAvailable bool         `json:"update_available"`
	CurrentVersion  string       `json:"current_version"`
	LatestVersion   string       `json:"latest_version"`
	Release         *ReleaseInfo `json:"release,omitempty"`
	Asset           *Asset       `json:"asset,omitempty"`
	Mandatory       bool         `json:"mandatory"`
}
