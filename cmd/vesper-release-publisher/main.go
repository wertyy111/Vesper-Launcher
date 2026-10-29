package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vesper-launcher/pkg/updater"
)

var (
	versionFlag   = flag.String("version", "1.0.66", "Release version to publish")
	tagFlag       = flag.String("tag", "v1.0.66", "Git tag for release")
	titleFlag     = flag.String("title", "Vesper Launcher 1.0.66", "Release title")
	changelogFlag = flag.String("changelog", "Automated release", "Changelog text or path to markdown file")
	assetsDirFlag = flag.String("dir", "dist", "Directory containing built release binaries")
	baseUrlFlag   = flag.String("base-url", "https://github.com/wertyy111/Vesper-Launcher/releases/download", "Base URL for downloads")
	outputFlag    = flag.String("out", "manifest.json", "Output manifest file path")
)

func calcSha256(filePath string) (string, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	hasher := sha256.New()
	size, err := io.Copy(hasher, f)
	if err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

func detectPlatform(filename string) updater.PlatformKey {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "win") || strings.HasSuffix(lower, ".exe") {
		return updater.PlatformWindowsAmd64
	}
	if strings.Contains(lower, "mac") || strings.Contains(lower, "darwin") || strings.HasSuffix(lower, ".dmg") || strings.HasSuffix(lower, ".pkg") {
		if strings.Contains(lower, "arm64") || strings.Contains(lower, "m1") {
			return updater.PlatformDarwinArm64
		}
		return updater.PlatformDarwinAmd64
	}
	if strings.Contains(lower, "arm64") {
		return updater.PlatformLinuxArm64
	}
	return updater.PlatformLinuxAmd64
}

func main() {
	flag.Parse()

	changelog := *changelogFlag
	if content, err := os.ReadFile(*changelogFlag); err == nil {
		changelog = string(content)
	}

	manifest := updater.UpdateManifest{
		SchemaVersion: "1.0",
		UpdatedAt:     time.Now(),
		Latest: updater.ReleaseInfo{
			Version:     *versionFlag,
			Tag:         *tagFlag,
			ReleaseDate: time.Now(),
			Title:       *titleFlag,
			Changelog:   changelog,
			Assets:      make(map[updater.PlatformKey]updater.Asset),
		},
		News: []updater.NewsItem{
			{
				ID:          fmt.Sprintf("rel-%s", *versionFlag),
				Title:       *titleFlag,
				Summary:     strings.Split(changelog, "\n")[0],
				PublishedAt: time.Now(),
				Badge:       "NEW",
			},
		},
	}

	files, err := os.ReadDir(*assetsDirFlag)
	if err == nil {
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			path := filepath.Join(*assetsDirFlag, file.Name())
			hash, size, err := calcSha256(path)
			if err != nil {
				log.Printf("Skip %s: %v", file.Name(), err)
				continue
			}

			platform := detectPlatform(file.Name())
			downloadUrl := fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(*baseUrlFlag, "/"), *tagFlag, file.Name())

			kind := "portable"
			if strings.HasSuffix(file.Name(), ".exe") {
				kind = "installer"
			} else if strings.HasSuffix(file.Name(), ".AppImage") {
				kind = "appimage"
			} else if strings.HasSuffix(file.Name(), ".deb") {
				kind = "deb"
			} else if strings.HasSuffix(file.Name(), ".dmg") {
				kind = "dmg"
			}

			manifest.Latest.Assets[platform] = updater.Asset{
				Name:   file.Name(),
				URL:    downloadUrl,
				SHA256: hash,
				Size:   size,
				Kind:   kind,
			}
			fmt.Printf("Mapped asset for %s: %s (SHA256: %s, Size: %d bytes)\n", platform, file.Name(), hash[:8]+"...", size)
		}
	} else {
		log.Printf("Assets directory %s not found; writing empty manifest template.", *assetsDirFlag)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		log.Fatalf("JSON marshal failed: %v", err)
	}

	if err := os.WriteFile(*outputFlag, data, 0644); err != nil {
		log.Fatalf("Write manifest failed: %v", err)
	}

	fmt.Printf("Successfully generated update manifest: %s\n", *outputFlag)
}
