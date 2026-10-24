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

type AssetIndexFile struct {
	Objects map[string]AssetObject `json:"objects"`
}

type AssetObject struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// EnsureVersionInstalled ensures the version package, parent version (if inherited),
// client jar, all libraries, and all assets are downloaded and extracted.
func EnsureVersionInstalled(versionID string, onProgress func(percent float64, message string)) (*VersionPackage, error) {
	mcDir := platform.GetMinecraftDir()
	versionsDir := filepath.Join(mcDir, "versions", versionID)
	versionJsonPath := filepath.Join(versionsDir, versionID+".json")

	// 1. Check if bundled version exists and copy if not on disk
	bundledDir := filepath.Join("bundled", "BundledVersions", versionID)
	bundledJsonPath := filepath.Join(bundledDir, versionID+".json")
	if _, err := os.Stat(versionJsonPath); os.IsNotExist(err) {
		if _, errB := os.Stat(bundledJsonPath); errB == nil {
			_ = copyDir(bundledDir, versionsDir)
		}
	}

	// Also copy bundled libraries if available
	bundledLibs := filepath.Join("bundled", "BundledLibraries")
	if _, err := os.Stat(bundledLibs); err == nil {
		destLibs := filepath.Join(mcDir, "libraries")
		_ = copyDir(bundledLibs, destLibs)
	}

	if onProgress != nil {
		onProgress(5, fmt.Sprintf("Загрузка спецификации %s...", versionID))
	}

	pkg, err := LoadVersionPackage(versionID, "")
	if err != nil {
		// Try resolving from Mojang manifest
		manifest, errM := FetchMojangManifest()
		if errM != nil {
			return nil, fmt.Errorf("failed to fetch manifest: %w", errM)
		}
		var foundUrl string
		for _, v := range manifest.Versions {
			if strings.EqualFold(v.ID, versionID) {
				foundUrl = v.URL
				break
			}
		}
		if foundUrl == "" {
			return nil, fmt.Errorf("version %s not found in Mojang manifest or local storage", versionID)
		}
		pkg, err = LoadVersionPackage(versionID, foundUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to download version JSON: %w", err)
		}
	}

	// Handle inheritance (e.g., Forge/Fabric inheriting from vanilla)
	var parentPkg *VersionPackage
	if pkg.InheritsFrom != "" {
		if onProgress != nil {
			onProgress(10, fmt.Sprintf("Загрузка родительской версии %s...", pkg.InheritsFrom))
		}
		parentPkg, err = EnsureVersionInstalled(pkg.InheritsFrom, onProgress)
		if err != nil {
			return nil, fmt.Errorf("failed to install parent version %s: %w", pkg.InheritsFrom, err)
		}
	}

	// 2. Prepare download tasks for libraries and client jar
	var tasks []DownloadTask
	librariesDir := filepath.Join(mcDir, "libraries")
	currentOS := getCurrentOSName()

	// Helper to collect libraries
	addLibraryTasks := func(libs []LibraryEntry) {
		for _, lib := range libs {
			if !CheckRules(lib.Rules) {
				continue
			}

			if lib.Downloads.Artifact != nil && lib.Downloads.Artifact.URL != "" {
				destPath := filepath.Join(librariesDir, lib.Downloads.Artifact.Path)
				tasks = append(tasks, DownloadTask{
					URL:  lib.Downloads.Artifact.URL,
					Path: destPath,
					Sha1: lib.Downloads.Artifact.Sha1,
					Size: lib.Downloads.Artifact.Size,
				})
			}

			if lib.Natives != nil && lib.Downloads.Classifiers != nil {
				classifierKey := lib.Natives[currentOS]
				if classifierKey != "" {
					if artifact, ok := lib.Downloads.Classifiers[classifierKey]; ok && artifact.URL != "" {
						destPath := filepath.Join(librariesDir, artifact.Path)
						tasks = append(tasks, DownloadTask{
							URL:  artifact.URL,
							Path: destPath,
							Sha1: artifact.Sha1,
							Size: artifact.Size,
						})
					}
				}
			}
		}
	}

	addLibraryTasks(pkg.Libraries)
	if parentPkg != nil {
		addLibraryTasks(parentPkg.Libraries)
	}

	// Client JAR task
	effectiveClientPkg := pkg
	if parentPkg != nil && (len(pkg.Downloads) == 0 || pkg.Downloads["client"].URL == "") {
		effectiveClientPkg = parentPkg
	}

	targetJarVersionID := effectiveClientPkg.ID
	clientJarPath := filepath.Join(mcDir, "versions", targetJarVersionID, targetJarVersionID+".jar")

	if clientArtifact, ok := effectiveClientPkg.Downloads["client"]; ok && clientArtifact.URL != "" {
		tasks = append(tasks, DownloadTask{
			URL:  clientArtifact.URL,
			Path: clientJarPath,
			Sha1: clientArtifact.Sha1,
			Size: clientArtifact.Size,
		})
	}

	// 3. Asset Index
	effectiveAssetIndex := pkg.AssetIndex
	if effectiveAssetIndex == nil && parentPkg != nil {
		effectiveAssetIndex = parentPkg.AssetIndex
	}

	assetsDir := filepath.Join(mcDir, "assets")
	if effectiveAssetIndex != nil && effectiveAssetIndex.URL != "" {
		indexPath := filepath.Join(assetsDir, "indexes", effectiveAssetIndex.ID+".json")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			if onProgress != nil {
				onProgress(20, "Загрузка индекса ресурсов...")
			}
			err := downloadSingleFile(effectiveAssetIndex.URL, indexPath)
			if err != nil {
				fmt.Println("[Warning] Failed to download asset index:", err)
			}
		}

		// Parse index and add missing asset objects
		if indexBytes, err := os.ReadFile(indexPath); err == nil {
			var idx AssetIndexFile
			if err := json.Unmarshal(indexBytes, &idx); err == nil {
				for _, obj := range idx.Objects {
					if len(obj.Hash) < 2 {
						continue
					}
					prefix := obj.Hash[:2]
					objPath := filepath.Join(assetsDir, "objects", prefix, obj.Hash)
					if _, err := os.Stat(objPath); os.IsNotExist(err) {
						objUrl := fmt.Sprintf("https://resources.download.minecraft.net/%s/%s", prefix, obj.Hash)
						tasks = append(tasks, DownloadTask{
							URL:  objUrl,
							Path: objPath,
							Sha1: obj.Hash,
							Size: obj.Size,
						})
					}
				}
			}
		}
	}

	// 4. Execute batch downloads
	if len(tasks) > 0 {
		if onProgress != nil {
			onProgress(30, fmt.Sprintf("Загрузка файлов (%d шт.)...", len(tasks)))
		}

		err := DownloadBatch(tasks, 32, func(completed, total int64, currentFile string) {
			pct := 30.0 + (float64(completed)/float64(total))*50.0
			if onProgress != nil {
				onProgress(pct, fmt.Sprintf("[%d/%d] %s", completed, total, currentFile))
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed during downloads: %w", err)
		}
	}

	// 5. Extract natives
	if onProgress != nil {
		onProgress(85, "Распаковка системных библиотек (natives)...")
	}

	nativesDir := filepath.Join(versionsDir, "natives")
	_ = os.MkdirAll(nativesDir, 0755)

	extractNativesFromLibs := func(libs []LibraryEntry) {
		for _, lib := range libs {
			if !CheckRules(lib.Rules) || lib.Natives == nil || lib.Downloads.Classifiers == nil {
				continue
			}
			classifierKey := lib.Natives[currentOS]
			if classifierKey != "" {
				if artifact, ok := lib.Downloads.Classifiers[classifierKey]; ok && artifact.Path != "" {
					jarPath := filepath.Join(librariesDir, artifact.Path)
					if _, err := os.Stat(jarPath); err == nil {
						_ = ExtractNatives(jarPath, nativesDir)
					}
				}
			}
		}
	}

	extractNativesFromLibs(pkg.Libraries)
	if parentPkg != nil {
		extractNativesFromLibs(parentPkg.Libraries)
	}

	// If bundled natives folder exists, copy files from it
	bundledNativesDir := filepath.Join(bundledDir, "natives")
	if _, err := os.Stat(bundledNativesDir); err == nil {
		_ = copyDir(bundledNativesDir, nativesDir)
	}

	return pkg, nil
}

func downloadSingleFile(url, destPath string) error {
	_ = os.MkdirAll(filepath.Dir(destPath), 0755)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(dst, 0755)

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			_ = copyDir(srcPath, dstPath)
		} else {
			if _, err := os.Stat(dstPath); os.IsNotExist(err) {
				_ = copyFile(srcPath, dstPath)
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
