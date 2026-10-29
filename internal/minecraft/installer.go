package minecraft

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

	if lid := strings.ToLower(versionID); strings.HasPrefix(lid, "forge-") || strings.HasPrefix(lid, "neoforge-") {
		_, errMarker := os.Stat(forgeProcessedMarker(versionID))
		_, errInst := os.Stat(forgeInstallerPath(versionID))
		if errMarker != nil && errInst != nil {
			_ = os.Remove(versionJsonPath)
		}
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
		} else if strings.HasPrefix(lowerID, "optifine-") || strings.HasPrefix(lowerID, "optifine ") {
			mcVer := strings.TrimPrefix(lowerID, "optifine-")
			mcVer = strings.TrimPrefix(mcVer, "optifine ")
			if strings.Contains(mcVer, " ") {
				mcVer = strings.Fields(mcVer)[0]
			}
			if onProgress != nil {
				onProgress(8, fmt.Sprintf("Загрузка профиля OptiFine для Minecraft %s...", mcVer))
			}
			optiPkg, errOpti := fetchOptiFineProfile(mcVer, versionID)
			if errOpti == nil && optiPkg != nil {
				pkg = optiPkg
				err = nil
			} else {
				fmt.Printf("[Installer] OptiFine fetch error: %v\n", errOpti)
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

			// 1. Main artifact (if specified in downloads)
			if lib.Downloads.Artifact != nil && lib.Downloads.Artifact.URL != "" {
				destPath := filepath.Join(librariesDir, lib.Downloads.Artifact.Path)
				tasks = append(tasks, DownloadTask{
					URL:  lib.Downloads.Artifact.URL,
					Path: destPath,
					Sha1: lib.Downloads.Artifact.Sha1,
					Size: lib.Downloads.Artifact.Size,
				})
			} else if lib.Downloads.Artifact == nil && len(lib.Downloads.Classifiers) == 0 && len(lib.Natives) == 0 && lib.Name != "" {
				// Only synthesize maven artifact if library has no natives/classifiers block
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

			// 2. Platform natives classifier
			if lib.Natives != nil && lib.Downloads.Classifiers != nil {
				classifierKey := lib.Natives[currentOS]
				if classifierKey == "" && currentOS == "osx" {
					classifierKey = lib.Natives["macos"]
				}
				if classifierKey != "" {
					arch := "64"
					if runtime.GOARCH == "386" || runtime.GOARCH == "arm" {
						arch = "32"
					}
					classifierKey = strings.ReplaceAll(classifierKey, "${arch}", arch)

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

	if parentPkg != nil {
		addLibraryTasks(MergeLibraries(parentPkg.Libraries, pkg.Libraries))
	} else {
		addLibraryTasks(pkg.Libraries)
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

		err := DownloadBatch(tasks, 24, func(completed, total int64, currentFile string) {
			pct := 30.0 + (float64(completed)/float64(total))*50.0
			if onProgress != nil {
				onProgress(pct, fmt.Sprintf("[%d/%d] %s", completed, total, currentFile))
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed during downloads: %w", err)
		}
	}

	// 4b. Modern Forge/NeoForge: run installer processors (patch client jar)
	if lid := strings.ToLower(versionID); (strings.HasPrefix(lid, "forge-") || strings.HasPrefix(lid, "neoforge-")) && pkg.InheritsFrom != "" {
		if err := ensureForgeProcessed(versionID, pkg.InheritsFrom, onProgress); err != nil {
			return nil, err
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

	if onProgress != nil {
		onProgress(100, "Версия готова!")
	}

	return pkg, nil
}

func fetchFabricProfile(mcVersion, versionID string) (*VersionPackage, error) {
	urls := []string{
		fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s", mcVersion),
		fmt.Sprintf("https://bmclapi2.bangbang93.com/fabric-meta/v2/versions/loader/%s", mcVersion),
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var loaders []struct {
		Loader struct {
			Version string `json:"version"`
			Stable  bool   `json:"stable"`
		} `json:"loader"`
	}

	var fetchErr error
	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			fetchErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fetchErr = fmt.Errorf("status %d from %s", resp.StatusCode, u)
			continue
		}
		err = json.NewDecoder(resp.Body).Decode(&loaders)
		resp.Body.Close()
		if err == nil && len(loaders) > 0 {
			fetchErr = nil
			break
		}
	}

	if len(loaders) == 0 {
		return nil, fmt.Errorf("no fabric loader available for Minecraft %s: %v", mcVersion, fetchErr)
	}

	loaderVer := loaders[0].Loader.Version
	for _, l := range loaders {
		if l.Loader.Stable {
			loaderVer = l.Loader.Version
			break
		}
	}

	profileURLs := []string{
		fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s/%s/profile/json", mcVersion, loaderVer),
		fmt.Sprintf("https://bmclapi2.bangbang93.com/fabric-meta/v2/versions/loader/%s/%s/profile/json", mcVersion, loaderVer),
	}

	var data []byte
	var profErr error
	for _, pu := range profileURLs {
		resp, err := client.Get(pu)
		if err != nil {
			profErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			profErr = fmt.Errorf("status %d from %s", resp.StatusCode, pu)
			continue
		}
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil && len(b) > 0 {
			data = b
			profErr = nil
			break
		}
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("failed to fetch fabric profile for Minecraft %s: %v", mcVersion, profErr)
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

	installerPath := forgeInstallerPath(versionID)
	_ = os.MkdirAll(filepath.Dir(installerPath), 0755)
	_ = os.WriteFile(installerPath, jarBytes, 0644)
	_ = os.Remove(forgeProcessedMarker(versionID))

	versionJsonBytes, err := extractForgeVersionJSON(jarBytes)
	if err != nil {
		return nil, err
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

	neoVer := list[len(list)-1].Version
	installerURL := fmt.Sprintf("https://maven.neoforged.net/releases/net/neoforged/neoforge/%s/neoforge-%s-installer.jar", neoVer, neoVer)
	mirrorURL := fmt.Sprintf("https://bmclapi2.bangbang93.com/maven/net/neoforged/neoforge/%s/neoforge-%s-installer.jar", neoVer, neoVer)

	jarBytes, err := downloadWithFallback(installerURL, mirrorURL)
	if err != nil {
		return nil, err
	}

	installerPath := forgeInstallerPath(versionID)
	_ = os.MkdirAll(filepath.Dir(installerPath), 0755)
	_ = os.WriteFile(installerPath, jarBytes, 0644)
	_ = os.Remove(forgeProcessedMarker(versionID))

	versionJsonBytes, err := extractForgeVersionJSON(jarBytes)
	if err != nil {
		return nil, err
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

func fetchOptiFineProfile(mcVersion, versionID string) (*VersionPackage, error) {
	listURL := fmt.Sprintf("https://bmclapi2.bangbang93.com/optifine/%s", mcVersion)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(listURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bmclapi optifine returned status: %d", resp.StatusCode)
	}

	var builds []struct {
		McVersion string `json:"mcversion"`
		Patch     string `json:"patch"`
		Type      string `json:"type"`
		Filename  string `json:"filename"`
		Forge     string `json:"forge"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&builds); err != nil {
		return nil, err
	}
	if len(builds) == 0 {
		return nil, fmt.Errorf("no optifine build available for Minecraft %s", mcVersion)
	}

	// Pick preferred build: prefer stable (non-preview)
	selected := builds[0]
	for _, b := range builds {
		isPreview := strings.HasPrefix(strings.ToLower(b.Filename), "preview_") ||
			strings.HasPrefix(strings.ToLower(b.Patch), "pre")
		if !isPreview {
			selected = b
			break
		}
	}

	coordinateVersion := fmt.Sprintf("%s_%s_%s", mcVersion, selected.Type, selected.Patch)
	optifineJarRelPath := filepath.Join("optifine", "OptiFine", coordinateVersion, fmt.Sprintf("OptiFine-%s.jar", coordinateVersion))

	mcDir := platform.GetMinecraftDir()
	librariesDir := filepath.Join(mcDir, "libraries")
	optifineJarPath := filepath.Join(librariesDir, optifineJarRelPath)

	optifineDownloadURL := fmt.Sprintf("https://bmclapi2.bangbang93.com/optifine/%s/%s/%s", mcVersion, selected.Type, selected.Patch)

	// Download OptiFine jar
	if _, err := os.Stat(optifineJarPath); os.IsNotExist(err) {
		err := downloadSingleFile(optifineDownloadURL, optifineJarPath)
		if err != nil {
			return nil, fmt.Errorf("failed to download OptiFine jar: %w", err)
		}
	}

	// Inspect OptiFine jar for launchwrapper-of
	launchwrapperLibName := "net.minecraft:launchwrapper:1.12"
	var launchwrapperRelPath string

	if jarFile, err := zip.OpenReader(optifineJarPath); err == nil {
		for _, f := range jarFile.File {
			base := filepath.Base(f.Name)
			if strings.HasPrefix(base, "launchwrapper-of-") && strings.HasSuffix(base, ".jar") {
				ver := strings.TrimSuffix(strings.TrimPrefix(base, "launchwrapper-of-"), ".jar")
				launchwrapperLibName = fmt.Sprintf("optifine:launchwrapper-of:%s", ver)
				launchwrapperRelPath = filepath.Join("optifine", "launchwrapper-of", ver, base)
				destLaunchwrapper := filepath.Join(librariesDir, launchwrapperRelPath)
				if _, errStat := os.Stat(destLaunchwrapper); os.IsNotExist(errStat) {
					if rc, errOpen := f.Open(); errOpen == nil {
						_ = os.MkdirAll(filepath.Dir(destLaunchwrapper), 0755)
						if outF, errOut := os.Create(destLaunchwrapper); errOut == nil {
							_, _ = io.Copy(outF, rc)
							outF.Close()
						}
						rc.Close()
					}
				}
				break
			}
		}
		jarFile.Close()
	}

	libs := []LibraryEntry{
		{
			Name: fmt.Sprintf("optifine:OptiFine:%s", coordinateVersion),
			Downloads: LibraryDownloads{
				Artifact: &DownloadArtifact{
					Path: filepath.ToSlash(optifineJarRelPath),
					URL:  optifineDownloadURL,
				},
			},
		},
	}

	if launchwrapperRelPath != "" {
		libs = append(libs, LibraryEntry{
			Name: launchwrapperLibName,
			Downloads: LibraryDownloads{
				Artifact: &DownloadArtifact{
					Path: filepath.ToSlash(launchwrapperRelPath),
				},
			},
		})
	} else {
		libs = append(libs, LibraryEntry{
			Name: launchwrapperLibName,
		})
	}

	pkg := VersionPackage{
		ID:           versionID,
		InheritsFrom: mcVersion,
		MainClass:    "net.minecraft.launchwrapper.Launch",
		Arguments: &ArgumentRules{
			Game: []any{"--tweakClass", "optifine.OptiFineTweaker"},
			Jvm:  []any{},
		},
		MinecraftArguments: "--username ${auth_player_name} --version ${version_name} --gameDir ${game_directory} --assetsDir ${assets_root} --assetIndex ${assets_index_name} --uuid ${auth_uuid} --accessToken ${auth_access_token} --userType ${user_type} --versionType ${version_type} --tweakClass optifine.OptiFineTweaker",
		Libraries:          libs,
	}

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return nil, err
	}

	versionJsonPath := filepath.Join(mcDir, "versions", versionID, versionID+".json")
	_ = os.MkdirAll(filepath.Dir(versionJsonPath), 0755)
	_ = os.WriteFile(versionJsonPath, data, 0644)

	return &pkg, nil
}

func downloadWithFallback(primaryUrl, fallbackUrl string) ([]byte, error) {
	candidates := []string{primaryUrl}
	if fallbackUrl != "" && fallbackUrl != primaryUrl {
		candidates = append(candidates, fallbackUrl)
	}
	client := &http.Client{Timeout: 60 * time.Second}

	var lastErr error
	for _, u := range candidates {
		subCandidates := GetDownloadFallbackURLs(u)
		for _, sub := range subCandidates {
			resp, err := client.Get(sub)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				return io.ReadAll(resp.Body)
			}
			if resp != nil {
				_ = resp.Body.Close()
			}
			lastErr = err
		}
	}
	return nil, fmt.Errorf("failed to download from candidate URLs: %v", lastErr)
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

func downloadSingleFile(targetURL, destPath string) error {
	_ = os.MkdirAll(filepath.Dir(destPath), 0755)
	client := &http.Client{Timeout: 35 * time.Second}
	candidates := GetDownloadFallbackURLs(targetURL)

	var lastErr error
	for _, cURL := range candidates {
		for attempt := 0; attempt < 2; attempt++ {
			resp, err := client.Get(cURL)
			if err != nil {
				lastErr = err
				time.Sleep(150 * time.Millisecond)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				lastErr = fmt.Errorf("status %d from %s", resp.StatusCode, cURL)
				time.Sleep(150 * time.Millisecond)
				continue
			}

			tempPath := destPath + ".tmp"
			out, err := os.Create(tempPath)
			if err != nil {
				resp.Body.Close()
				return err
			}

			_, err = io.Copy(out, resp.Body)
			out.Close()
			resp.Body.Close()
			if err != nil {
				_ = os.Remove(tempPath)
				lastErr = err
				time.Sleep(150 * time.Millisecond)
				continue
			}

			return os.Rename(tempPath, destPath)
		}
	}

	return fmt.Errorf("failed downloading %s after trying mirrors: %v", targetURL, lastErr)
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
