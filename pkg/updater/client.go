package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Client handles checking, downloading, and applying Vesper updates.
type Client struct {
	httpClient *http.Client
	sourceURL  string
	userAgent  string
}

// NewClient creates an updater client.
func NewClient(sourceURL string) *Client {
	if sourceURL == "" {
		sourceURL = "https://raw.githubusercontent.com/wertyy111/Vesper-Launcher/main/manifest.json"
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		sourceURL:  sourceURL,
		userAgent:  "VesperLauncher-GoUpdater/2.0",
	}
}

// CurrentPlatform returns the current platform key.
func CurrentPlatform() PlatformKey {
	switch runtime.GOOS {
	case "windows":
		return PlatformWindowsAmd64
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return PlatformDarwinArm64
		}
		return PlatformDarwinAmd64
	default:
		if runtime.GOARCH == "arm64" {
			return PlatformLinuxArm64
		}
		return PlatformLinuxAmd64
	}
}

// FetchManifest retrieves the update manifest from the configured source.
func (c *Client) FetchManifest(ctx context.Context) (*UpdateManifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status from update server: %s", resp.Status)
	}

	var manifest UpdateManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}

	return &manifest, nil
}

// CheckUpdate checks if a new version is available for currentVersion on current platform.
func (c *Client) CheckUpdate(ctx context.Context, currentVersion string) (*CheckUpdateResult, error) {
	manifest, err := c.FetchManifest(ctx)
	if err != nil {
		return nil, err
	}

	latest := manifest.Latest
	isAvail := IsNewer(latest.Version, currentVersion)
	platform := CurrentPlatform()
	asset, hasAsset := latest.Assets[platform]

	result := &CheckUpdateResult{
		UpdateAvailable: isAvail,
		CurrentVersion:  currentVersion,
		LatestVersion:   latest.Version,
		Release:         &latest,
		Mandatory:       latest.Mandatory,
	}

	if hasAsset {
		result.Asset = &asset
	}

	return result, nil
}

// DownloadProgressCallback reports download progress.
type DownloadProgressCallback func(downloadedBytes, totalBytes int64)

// DownloadFile downloads an asset to targetPath with checksum validation and progress reporting.
func (c *Client) DownloadFile(ctx context.Context, url string, expectedSha256 string, targetPath string, progressCb DownloadProgressCallback) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download server returned: %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	partPath := targetPath + ".part"
	file, err := os.Create(partPath)
	if err != nil {
		return fmt.Errorf("create part file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	totalBytes := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			_ = os.Remove(partPath)
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.Write(buf[:n]); werr != nil {
				_ = os.Remove(partPath)
				return werr
			}
			hasher.Write(buf[:n])
			downloaded += int64(n)
			if progressCb != nil {
				progressCb(downloaded, totalBytes)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			_ = os.Remove(partPath)
			return readErr
		}
	}

	_ = file.Close()

	if expectedSha256 != "" {
		calculated := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(calculated, strings.TrimSpace(expectedSha256)) {
			_ = os.Remove(partPath)
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSha256, calculated)
		}
	}

	if err := os.Rename(partPath, targetPath); err != nil {
		// On Windows rename over existing fails, remove first
		_ = os.Remove(targetPath)
		if err := os.Rename(partPath, targetPath); err != nil {
			return fmt.Errorf("commit download file: %w", err)
		}
	}

	// Ensure executable permissions on Unix
	if runtime.GOOS != "windows" {
		_ = os.Chmod(targetPath, 0755)
	}

	return nil
}

// ApplyBinaryUpdate replaces targetExecutable with newBinary.
func ApplyBinaryUpdate(newBinary string, targetExecutable string) error {
	if runtime.GOOS == "windows" {
		// On Windows running files cannot be replaced directly, rename old to .old first
		oldPath := targetExecutable + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(targetExecutable, oldPath); err != nil {
			return fmt.Errorf("rename running binary: %w", err)
		}
		if err := os.Rename(newBinary, targetExecutable); err != nil {
			// Rollback
			_ = os.Rename(oldPath, targetExecutable)
			return fmt.Errorf("place new binary: %w", err)
		}
		return nil
	}

	// On Linux / macOS, atomic rename works even if binary is currently executing
	if err := os.Chmod(newBinary, 0755); err != nil {
		return err
	}
	if err := os.Rename(newBinary, targetExecutable); err != nil {
		return fmt.Errorf("atomic binary swap: %w", err)
	}
	return nil
}
