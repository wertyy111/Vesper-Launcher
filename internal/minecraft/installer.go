package minecraft

import (
	"archive/zip"
	"bytes"
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
		lowerID := strings.ToLower(versionID)
		if strings.HasPrefix(lowerID, "fabric-") {
			mcVer := strings.TrimPrefix(lowerID, "fabric-")
			if onProgress != nil {
				onProgress(8, fmt.Sprintf("Загрузка профиля Fabric для Minecraft %s...", mcVer))
			}
			fPkg, errF := fetchFabricProfile(mcVer, versionID)
			if errF == nil && fPkg != nil {
				pkg = fPkg
				err = nil
			} else {
				fmt.Printf("[Installer] Fabric fetch error: %v\n", errF)
			}
		} else if strings.HasPrefix(lowerID, "forge-") {
			mcVer := strings.TrimPrefix(lowerID, "forge-")
			if onProgress != nil {
				onProgress(8, fmt.Sprintf("Загрузка профиля Forge для Minecraft %s...", mcVer))
			}
			forgePkg, errForge := fetchForgeProfile(mcVer, versionID)
			if errForge == nil && forgePkg != nil {
				pkg = forgePkg
				err = nil
			} else {
				fmt.Printf("[Installer] Forge fetch error: %v\n", errForge)
			}
		} else if strings.HasPrefix(lowerID, "neoforge-") {
			mcVer := strings.TrimPrefix(lowerID, "neoforge-")
			if onProgress != nil {
				onProgress(8, fmt.Sprintf("Загрузка профиля NeoForge для Minecraft %s...", mcVer))
			}
			neoPkg, errNeo := fetchNeoForgeProfile(mcVer, versionID)
			if errNeo == nil && neoPkg != nil {
				pkg = neoPkg
				err = nil
			} else {
				fmt.Printf("[Installer] NeoForge fetch error: %v\n", errNeo)
			}
		}

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
			} else if lib.Name != "" {
				relPath, err := MavenCoordinatesToPath(lib.Name)
				if err == nil {
					destPath := filepath.Join(librariesDir, relPath)
					if _, errS := os.Stat(destPath); os.IsNotExist(errS) {
						downloadUrl := resolveMavenDownloadUrl(lib, relPath)
						if downloadUrl != "" {
							tasks = append(tasks, DownloadTask{
								URL:  downloadUrl,
								Path: destPath,
							})
						}
					}
				}
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

func fetchFabricProfile(mcVersion, versionID string) (*VersionPackage, error) {
	loaderListURL := fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s", mcVersion)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(loaderListURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fabric meta returned status: %d", resp.StatusCode)
	}

	var loaders []struct {
		Loader struct {
			Version string `json:"version"`
			Stable  bool   `json:"stable"`
		} `json:"loader"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loaders); err != nil {
		return nil, err
	}
	if len(loaders) == 0 {
		return nil, fmt.Errorf("no fabric loader available for Minecraft %s", mcVersion)
	}

	loaderVer := loaders[0].Loader.Version
	for _, l := range loaders {
		if l.Loader.Stable {
			loaderVer = l.Loader.Version
			break
		}
	}

	profileURL := fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s/%s/profile/json", mcVersion, loaderVer)
	resp2, err := client.Get(profileURL)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fabric profile returned status: %d", resp2.StatusCode)
	}

	data, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, err
	}

	var rawMap map[string]any
	if err := json.Unmarshal(data, &rawMap); err == nil {
		rawMap["id"] = versionID
		rawMap["inheritsFrom"] = mcVersion
		data, _ = json.MarshalIndent(rawMap, "", "  ")
	}

	mcDir := platform.GetMinecraftDir()
	savePath := filepath.Join(mcDir, "versions", versionID, versionID+".json")
	_ = os.MkdirAll(filepath.Dir(savePath), 0755)
	_ = os.WriteFile(savePath, data, 0644)

	var pkg VersionPackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func fetchForgeProfile(mcVersion, versionID string) (*VersionPackage, error) {
	buildsURL := fmt.Sprintf("https://bmclapi2.bangbang93.com/forge/minecraft/%s", mcVersion)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(buildsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bmclapi forge returned status: %d", resp.StatusCode)
	}

	var builds []struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&builds); err != nil {
		return nil, err
	}
	if len(builds) == 0 {
		return nil, fmt.Errorf("no forge build available for Minecraft %s", mcVersion)
	}

	forgeVer := builds[len(builds)-1].Version
	installerURL := fmt.Sprintf("https://maven.minecraftforge.net/net/minecraftforge/forge/%s-%s/forge-%s-%s-installer.jar", mcVersion, forgeVer, mcVersion, forgeVer)
	mirrorURL := fmt.Sprintf("https://bmclapi2.bangbang93.com/maven/net/minecraftforge/forge/%s-%s/forge-%s-%s-installer.jar", mcVersion, forgeVer, mcVersion, forgeVer)

	jarBytes, err := downloadWithFallback(installerURL, mirrorURL)
	if err != nil {
		return nil, err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(jarBytes), int64(len(jarBytes)))
	if err != nil {
		return nil, err
	}

	var versionJsonBytes []byte
	for _, f := range zipReader.File {
		if f.Name == "version.json" {
			rc, err := f.Open()
			if err == nil {
				versionJsonBytes, _ = io.ReadAll(rc)
				_ = rc.Close()
			}
			break
		}
	}

	if len(versionJsonBytes) == 0 {
		return nil, fmt.Errorf("version.json not found in forge installer jar")
	}

	var rawMap map[string]any
	if err := json.Unmarshal(versionJsonBytes, &rawMap); err == nil {
		rawMap["id"] = versionID
		rawMap["inheritsFrom"] = mcVersion
		versionJsonBytes, _ = json.MarshalIndent(rawMap, "", "  ")
	}

	mcDir := platform.GetMinecraftDir()
	savePath := filepath.Join(mcDir, "versions", versionID, versionID+".json")
	_ = os.MkdirAll(filepath.Dir(savePath), 0755)
	_ = os.WriteFile(savePath, versionJsonBytes, 0644)

	var pkg VersionPackage
	if err := json.Unmarshal(versionJsonBytes, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func fetchNeoForgeProfile(mcVersion, versionID string) (*VersionPackage, error) {
	listURL := fmt.Sprintf("https://bmclapi2.bangbang93.com/neoforge/list/%s", mcVersion)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(listURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bmclapi neoforge returned status: %d", resp.StatusCode)
	}

	var list []struct {
		Version       string `json:"version"`
		InstallerPath string `json:"installerPath"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no neoforge build available for Minecraft %s", mcVersion)
	}

	neoVer := list[0].Version
	installerURL := fmt.Sprintf("https://maven.neoforged.net/releases/net/neoforged/neoforge/%s/neoforge-%s-installer.jar", neoVer, neoVer)
	mirrorURL := fmt.Sprintf("https://bmclapi2.bangbang93.com/maven/net/neoforged/neoforge/%s/neoforge-%s-installer.jar", neoVer, neoVer)

	jarBytes, err := downloadWithFallback(installerURL, mirrorURL)
	if err != nil {
		return nil, err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(jarBytes), int64(len(jarBytes)))
	if err != nil {
		return nil, err
	}

	var versionJsonBytes []byte
	for _, f := range zipReader.File {
		if f.Name == "version.json" {
			rc, err := f.Open()
			if err == nil {
				versionJsonBytes, _ = io.ReadAll(rc)
				_ = rc.Close()
			}
			break
		}
	}

	if len(versionJsonBytes) == 0 {
		return nil, fmt.Errorf("version.json not found in neoforge installer jar")
	}

	var rawMap map[string]any
	if err := json.Unmarshal(versionJsonBytes, &rawMap); err == nil {
		rawMap["id"] = versionID
		rawMap["inheritsFrom"] = mcVersion
		versionJsonBytes, _ = json.MarshalIndent(rawMap, "", "  ")
	}

	mcDir := platform.GetMinecraftDir()
	savePath := filepath.Join(mcDir, "versions", versionID, versionID+".json")
	_ = os.MkdirAll(filepath.Dir(savePath), 0755)
	_ = os.WriteFile(savePath, versionJsonBytes, 0644)

	var pkg VersionPackage
	if err := json.Unmarshal(versionJsonBytes, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func downloadWithFallback(primaryUrl, fallbackUrl string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(primaryUrl)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	if fallbackUrl != "" {
		respF, errF := client.Get(fallbackUrl)
		if errF == nil && respF.StatusCode == http.StatusOK {
			defer respF.Body.Close()
			return io.ReadAll(respF.Body)
		}
		if respF != nil {
			_ = respF.Body.Close()
		}
	}
	return nil, fmt.Errorf("failed to download from primary and fallback URLs: %s", primaryUrl)
}

func resolveMavenDownloadUrl(lib LibraryEntry, relPath string) string {
	slashRelPath := filepath.ToSlash(relPath)
	if lib.URL != "" && (strings.HasPrefix(lib.URL, "http://") || strings.HasPrefix(lib.URL, "https://")) {
		return strings.TrimRight(lib.URL, "/") + "/" + slashRelPath
	}
	if strings.Contains(lib.Name, "fabricmc") {
		return "https://maven.fabricmc.net/" + slashRelPath
	}
	if strings.Contains(lib.Name, "minecraftforge") {
		return "https://maven.minecraftforge.net/" + slashRelPath
	}
	if strings.Contains(lib.Name, "neoforged") {
		return "https://maven.neoforged.net/releases/" + slashRelPath
	}
	return "https://libraries.minecraft.net/" + slashRelPath
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
