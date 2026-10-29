package minecraft

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vesper-launcher/internal/java"
	"vesper-launcher/internal/platform"
)

// forgeInstallProfile is a subset of install_profile.json (modern and legacy formats).
type forgeInstallProfile struct {
	Libraries  []LibraryEntry    `json:"libraries"`
	Processors []json.RawMessage `json:"processors"`
	// Legacy (<=1.12) format
	Install *struct {
		Path     string `json:"path"`
		FilePath string `json:"filePath"`
	} `json:"install"`
	VersionInfo json.RawMessage `json:"versionInfo"`
}

func forgeInstallerPath(versionID string) string {
	return filepath.Join(platform.GetMinecraftDir(), "versions", versionID, versionID+"-installer.jar")
}

func forgeProcessedMarker(versionID string) string {
	return filepath.Join(platform.GetMinecraftDir(), "versions", versionID, ".vesper-forge-processed")
}

func readZipEntry(zr *zip.Reader, name string) []byte {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			defer rc.Close()
			b, _ := io.ReadAll(rc)
			return b
		}
	}
	return nil
}

// extractForgeVersionJSON returns version JSON from an installer jar (modern version.json or legacy versionInfo),
// and performs legacy universal jar extraction if required.
func extractForgeVersionJSON(jarBytes []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(jarBytes), int64(len(jarBytes)))
	if err != nil {
		return nil, err
	}

	if v := readZipEntry(zr, "version.json"); len(v) > 0 {
		return v, nil
	}

	profileBytes := readZipEntry(zr, "install_profile.json")
	if len(profileBytes) == 0 {
		return nil, fmt.Errorf("installer jar has neither version.json nor install_profile.json")
	}
	var profile forgeInstallProfile
	if err := json.Unmarshal(profileBytes, &profile); err != nil {
		return nil, err
	}
	if len(profile.VersionInfo) == 0 {
		return nil, fmt.Errorf("install_profile.json has no versionInfo")
	}

	// Legacy: extract embedded universal jar to the library path
	if profile.Install != nil && profile.Install.Path != "" && profile.Install.FilePath != "" {
		if rel, err := MavenCoordinatesToPath(profile.Install.Path); err == nil {
			dest := filepath.Join(platform.GetMinecraftDir(), "libraries", rel)
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				if data := readZipEntry(zr, profile.Install.FilePath); len(data) > 0 {
					_ = os.MkdirAll(filepath.Dir(dest), 0755)
					_ = os.WriteFile(dest, data, 0644)
				}
			}
		}
	}
	return profile.VersionInfo, nil
}

func ensureLauncherProfilesStub(mcDir string) {
	p := filepath.Join(mcDir, "launcher_profiles.json")
	if _, err := os.Stat(p); err == nil {
		return
	}
	stub := map[string]any{
		"profiles": map[string]any{
			"VesperLauncher": map[string]any{"name": "VesperLauncher", "type": "custom"},
		},
		"settings":        map[string]any{},
		"selectedProfile": "VesperLauncher",
		"launcherVersion": map[string]any{"name": "VesperLauncher", "format": 21},
	}
	data, _ := json.MarshalIndent(stub, "", "  ")
	_ = os.WriteFile(p, data, 0644)
}

// ensureForgeProcessed runs the official Forge/NeoForge installer (--installClient) for modern
// versions that require install processors (binary patching, jar splitting, etc).
func ensureForgeProcessed(versionID, mcVersion string, onProgress func(float64, string)) error {
	marker := forgeProcessedMarker(versionID)
	if _, err := os.Stat(marker); err == nil {
		return nil
	}

	installerPath := forgeInstallerPath(versionID)
	jarBytes, err := os.ReadFile(installerPath)
	if err != nil {
		return fmt.Errorf("forge installer not found: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(jarBytes), int64(len(jarBytes)))
	if err != nil {
		return err
	}

	var profile forgeInstallProfile
	if pb := readZipEntry(zr, "install_profile.json"); len(pb) > 0 {
		_ = json.Unmarshal(pb, &profile)
	}

	if len(profile.Processors) == 0 {
		// Nothing to process (legacy or simple installer)
		_ = os.WriteFile(marker, []byte("ok"), 0644)
		return nil
	}

	mcDir := platform.GetMinecraftDir()
	librariesDir := filepath.Join(mcDir, "libraries")

	// 1. Pre-download installer libraries through our mirrors so the installer finds them locally
	if onProgress != nil {
		onProgress(82, "Загрузка библиотек установщика...")
	}
	var tasks []DownloadTask
	for _, lib := range profile.Libraries {
		if lib.Downloads.Artifact != nil && lib.Downloads.Artifact.URL != "" && lib.Downloads.Artifact.Path != "" {
			tasks = append(tasks, DownloadTask{
				URL:  lib.Downloads.Artifact.URL,
				Path: filepath.Join(librariesDir, lib.Downloads.Artifact.Path),
				Sha1: lib.Downloads.Artifact.Sha1,
				Size: lib.Downloads.Artifact.Size,
			})
		}
	}
	if len(tasks) > 0 {
		if err := DownloadBatch(tasks, 16, func(done, total int64, file string) {
			if onProgress != nil {
				onProgress(82+float64(done)/float64(total)*3, fmt.Sprintf("[%d/%d] %s", done, total, file))
			}
		}); err != nil {
			fmt.Printf("[Forge] Warning: some installer libraries failed: %v\n", err)
		}
	}

	// 2. Run installer
	if onProgress != nil {
		onProgress(86, "Установка Forge (патчинг клиента, это может занять 1-2 минуты)...")
	}
	ensureLauncherProfilesStub(mcDir)

	reqMajor := java.GetRequiredJavaMajor(mcVersion)
	javaPath, _, err := java.SelectBestJava(mcVersion, java.DetectRuntimes())
	if err != nil || javaPath == "" {
		if dlJava, dlErr := java.EnsureJavaRuntime(reqMajor, onProgress); dlErr == nil {
			javaPath = dlJava
		} else if javaPath == "" {
			javaPath = "java"
		}
	}

	cmd := exec.Command(javaPath, "-jar", installerPath, "--installClient", mcDir)
	cmd.Dir = filepath.Dir(installerPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()

	_ = os.WriteFile(filepath.Join(filepath.Dir(installerPath), "installer-output.log"), out.Bytes(), 0644)

	if runErr != nil {
		tail := out.String()
		if len(tail) > 1500 {
			tail = tail[len(tail)-1500:]
		}
		return fmt.Errorf("forge installer failed: %v\n%s", runErr, strings.TrimSpace(tail))
	}

	_ = os.WriteFile(marker, []byte("ok"), 0644)
	return nil
}
