package java

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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

// EnsureJavaRuntime checks if an Adoptium Temurin JRE/JDK of requiredMajor is available in
// the launcher's runtimes directory, and downloads/extracts it if missing.
func EnsureJavaRuntime(requiredMajor int, onProgress func(pct float64, msg string)) (string, error) {
	osName := "linux"
	archiveExt := ".tar.gz"
	switch runtime.GOOS {
	case "windows":
		osName = "windows"
		archiveExt = ".zip"
	case "darwin":
		osName = "mac"
		archiveExt = ".tar.gz"
	}

	arch := "x64"
	switch runtime.GOARCH {
	case "arm64":
		arch = "aarch64"
	case "386":
		arch = "x32"
	}

	runtimeID := fmt.Sprintf("temurin-%d-%s-%s", requiredMajor, osName, arch)
	mcDir := platform.GetMinecraftDir()
	runtimesRoot := filepath.Join(mcDir, "java-runtimes")
	targetDir := filepath.Join(runtimesRoot, runtimeID)

	binName := "java"
	if runtime.GOOS == "windows" {
		binName = "javaw.exe"
	}
	binPath := filepath.Join(targetDir, "bin", binName)

	if _, err := os.Stat(binPath); err == nil {
		if info, ok := InspectJava(binPath); ok && info.MajorVersion == requiredMajor {
			return binPath, nil
		}
	}

	// Also check fallback "java.exe" on Windows
	if runtime.GOOS == "windows" {
		altBin := filepath.Join(targetDir, "bin", "java.exe")
		if _, err := os.Stat(altBin); err == nil {
			if info, ok := InspectJava(altBin); ok && info.MajorVersion == requiredMajor {
				return altBin, nil
			}
		}
	}

	if onProgress != nil {
		onProgress(10, fmt.Sprintf("Загрузка Java %d (Eclipse Temurin %s)...", requiredMajor, arch))
	}

	_ = os.MkdirAll(runtimesRoot, 0755)
	archivePath := filepath.Join(runtimesRoot, runtimeID+archiveExt)
	extractDir := filepath.Join(runtimesRoot, runtimeID+".extract")

	_ = os.Remove(archivePath)
	_ = os.RemoveAll(extractDir)

	urls := []string{
		fmt.Sprintf("https://api.adoptium.net/v3/binary/latest/%d/ga/%s/%s/jre/hotspot/normal/eclipse", requiredMajor, osName, arch),
		fmt.Sprintf("https://api.adoptium.net/v3/binary/latest/%d/ga/%s/%s/jdk/hotspot/normal/eclipse", requiredMajor, osName, arch),
	}

	var downloadErr error
	client := &http.Client{Timeout: 10 * time.Minute}

	for _, u := range urls {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "VesperLauncher/1.0")

		resp, err := client.Do(req)
		if err != nil {
			downloadErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			downloadErr = fmt.Errorf("status code %d from %s", resp.StatusCode, u)
			continue
		}

		out, err := os.Create(archivePath)
		if err != nil {
			resp.Body.Close()
			return "", err
		}

		total := resp.ContentLength
		var downloaded int64
		buf := make([]byte, 64*1024)
		for {
			n, rErr := resp.Body.Read(buf)
			if n > 0 {
				_, _ = out.Write(buf[:n])
				downloaded += int64(n)
				if onProgress != nil && total > 0 {
					pct := 10.0 + (float64(downloaded)/float64(total))*60.0
					onProgress(pct, fmt.Sprintf("Загрузка Java %d: %.1f / %.1f МБ", requiredMajor, float64(downloaded)/(1024*1024), float64(total)/(1024*1024)))
				}
			}
			if rErr == io.EOF {
				break
			}
			if rErr != nil {
				downloadErr = rErr
				break
			}
		}
		out.Close()
		resp.Body.Close()

		if downloadErr == nil {
			break
		}
		_ = os.Remove(archivePath)
	}

	if downloadErr != nil && !fileExists(archivePath) {
		return "", fmt.Errorf("не удалось скачать Java %d: %w", requiredMajor, downloadErr)
	}

	if onProgress != nil {
		onProgress(75, fmt.Sprintf("Распаковка Java %d...", requiredMajor))
	}

	_ = os.MkdirAll(extractDir, 0755)
	var extractErr error
	if strings.HasSuffix(archivePath, ".zip") {
		extractErr = extractZipStripRoot(archivePath, extractDir)
	} else {
		extractErr = extractTarGzStripRoot(archivePath, extractDir)
	}

	_ = os.Remove(archivePath)

	if extractErr != nil {
		_ = os.RemoveAll(extractDir)
		return "", fmt.Errorf("ошибка распаковки архива Java: %w", extractErr)
	}

	// Move extracted directory to target
	_ = os.RemoveAll(targetDir)
	if err := os.Rename(extractDir, targetDir); err != nil {
		return "", fmt.Errorf("не удалось переместить распакованную Java в %s: %w", targetDir, err)
	}

	// Restore execution permissions on Unix
	if runtime.GOOS != "windows" {
		_ = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.Contains(path, "/bin/") || strings.HasSuffix(path, "jspawnhelper") || strings.HasSuffix(path, "jexec") {
				_ = os.Chmod(path, 0755)
			}
			return nil
		})
	}

	// Locate binary in targetDir
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}
	altBin := filepath.Join(targetDir, "bin", "java")
	if runtime.GOOS == "windows" {
		altBin = filepath.Join(targetDir, "bin", "java.exe")
	}
	if _, err := os.Stat(altBin); err == nil {
		return altBin, nil
	}

	return "", fmt.Errorf("исполняемый файл Java не найден после распаковки в %s", targetDir)
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Size() > 0
}

func extractTarGzStripRoot(tarGzPath, destDir string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		parts := strings.Split(filepath.ToSlash(header.Name), "/")
		if len(parts) <= 1 {
			continue
		}
		relPath := filepath.Join(parts[1:]...)
		if relPath == "" || relPath == "." {
			continue
		}

		target := filepath.Join(destDir, relPath)
		switch header.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0755)
		case tar.TypeReg, tar.TypeRegA:
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			_, _ = io.Copy(outFile, tr)
			outFile.Close()
		case tar.TypeSymlink:
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			_ = os.Remove(target)
			_ = os.Symlink(header.Linkname, target)
		}
	}
	return nil
}

func extractZipStripRoot(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		parts := strings.Split(filepath.ToSlash(f.Name), "/")
		if len(parts) <= 1 {
			continue
		}
		relPath := filepath.Join(parts[1:]...)
		if relPath == "" || relPath == "." {
			continue
		}

		target := filepath.Join(destDir, relPath)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}

		_ = os.MkdirAll(filepath.Dir(target), 0755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, _ = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}
	return nil
}
