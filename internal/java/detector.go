package java

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type JavaInfo struct {
	Path         string `json:"path"`
	MajorVersion int    `json:"majorVersion"`
	FullVersion  string `json:"fullVersion"`
	Is64Bit      bool   `json:"is64Bit"`
}

// DetectRuntimes scans the OS for installed Java environments.
func DetectRuntimes() []JavaInfo {
	var candidates []string

	// 1. Check system PATH
	if p, err := exec.LookPath("java"); err == nil {
		if realPath, err := filepath.EvalSymlinks(p); err == nil {
			candidates = append(candidates, realPath)
		} else {
			candidates = append(candidates, p)
		}
	}

	// 2. OS-specific standard directories
	switch runtime.GOOS {
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		if programFiles != "" {
			scanJavaDir(filepath.Join(programFiles, "Java"), &candidates)
			scanJavaDir(filepath.Join(programFiles, "Eclipse Adoptium"), &candidates)
			scanJavaDir(filepath.Join(programFiles, "BellSoft"), &candidates)
			scanJavaDir(filepath.Join(programFiles, "Zulu"), &candidates)
		}
	case "darwin":
		scanJavaDir("/Library/Java/JavaVirtualMachines", &candidates)
		scanJavaDir("/System/Library/Java/JavaVirtualMachines", &candidates)
	default: // Linux
		scanJavaDir("/usr/lib/jvm", &candidates)
		scanJavaDir("/usr/java", &candidates)
		scanJavaDir("/opt/java", &candidates)
	}

	// Remove duplicates and inspect
	seen := make(map[string]bool)
	var runtimes []JavaInfo

	for _, path := range candidates {
		if seen[path] {
			continue
		}
		seen[path] = true

		if info, ok := InspectJava(path); ok {
			runtimes = append(runtimes, info)
		}
	}

	return runtimes
}

func scanJavaDir(dir string, candidates *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		subDir := filepath.Join(dir, entry.Name())
		var binJava string
		if runtime.GOOS == "windows" {
			binJava = filepath.Join(subDir, "bin", "java.exe")
		} else {
			binJava = filepath.Join(subDir, "bin", "java")
			if _, err := os.Stat(binJava); os.IsNotExist(err) {
				binJava = filepath.Join(subDir, "jre", "bin", "java")
			}
		}

		if _, err := os.Stat(binJava); err == nil {
			*candidates = append(*candidates, binJava)
		}
	}
}

// InspectJava executes `java -version` and extracts version info.
func InspectJava(javaPath string) (JavaInfo, bool) {
	cmd := exec.Command(javaPath, "-version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return JavaInfo{}, false
	}

	output := out.String()
	versionRegex := regexp.MustCompile(`version "([^"]+)"`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) < 2 {
		return JavaInfo{}, false
	}

	fullVer := matches[1]
	majorVer := parseMajorVersion(fullVer)
	is64Bit := strings.Contains(output, "64-Bit") || strings.Contains(output, "x86_64") || strings.Contains(output, "amd64") || strings.Contains(output, "arm64")

	return JavaInfo{
		Path:         javaPath,
		MajorVersion: majorVer,
		FullVersion:  fullVer,
		Is64Bit:      is64Bit,
	}, true
}

func parseMajorVersion(v string) int {
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return 8
	}

	if parts[0] == "1" && len(parts) > 1 {
		if val, err := strconv.Atoi(parts[1]); err == nil {
			return val
		}
	}

	// Java 9+ (e.g. "17.0.2", "21")
	dashParts := strings.Split(parts[0], "-")
	if val, err := strconv.Atoi(dashParts[0]); err == nil {
		return val
	}

	return 8
}

// SelectBestJava chooses the appropriate Java version for the given Minecraft release.
func SelectBestJava(mcVersion string, runtimes []JavaInfo) string {
	requiredMajor := 21
	if strings.HasPrefix(mcVersion, "1.16") || strings.HasPrefix(mcVersion, "1.12") || strings.HasPrefix(mcVersion, "1.8") {
		requiredMajor = 8
	} else if strings.HasPrefix(mcVersion, "1.17") || strings.HasPrefix(mcVersion, "1.18") || strings.HasPrefix(mcVersion, "1.19") || strings.HasPrefix(mcVersion, "1.20.0") || strings.HasPrefix(mcVersion, "1.20.1") || strings.HasPrefix(mcVersion, "1.20.2") || strings.HasPrefix(mcVersion, "1.20.3") || strings.HasPrefix(mcVersion, "1.20.4") {
		requiredMajor = 17
	}

	// 1. Exact match
	for _, r := range runtimes {
		if r.MajorVersion == requiredMajor {
			return r.Path
		}
	}

	// 2. Compatible higher version (except Java 8 which needs 8)
	if requiredMajor > 8 {
		for _, r := range runtimes {
			if r.MajorVersion >= requiredMajor {
				return r.Path
			}
		}
	}

	// 3. Fallback to first available
	if len(runtimes) > 0 {
		return runtimes[0].Path
	}

	return "java"
}
