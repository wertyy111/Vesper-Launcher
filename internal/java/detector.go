package java

import (
	"bytes"
	"fmt"
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

	// 2. Check JAVA_HOME & JDK_HOME
	for _, envKey := range []string{"JAVA_HOME", "JDK_HOME"} {
		if envVal := os.Getenv(envKey); envVal != "" {
			binJava := filepath.Join(envVal, "bin", "java")
			if runtime.GOOS == "windows" {
				binJava = filepath.Join(envVal, "bin", "java.exe")
			}
			if _, err := os.Stat(binJava); err == nil {
				candidates = append(candidates, binJava)
			}
		}
	}

	// 3. User home directories (~/.jdks, ~/.sdkman/candidates/java)
	if homeDir, err := os.UserHomeDir(); err == nil {
		scanJavaDir(filepath.Join(homeDir, ".jdks"), &candidates)
		scanJavaDir(filepath.Join(homeDir, ".sdkman", "candidates", "java"), &candidates)
		scanJavaDir(filepath.Join(homeDir, ".config", "VesperLauncher", "runtimes"), &candidates)

		// Bundled runtimes of other launchers (Mojang official, TLauncher, etc.)
		for _, pattern := range []string{
			filepath.Join(homeDir, ".tlauncher", "mojang_jre", "*", "*", "*", "bin", "java"),
			filepath.Join(homeDir, ".minecraft", "runtime", "*", "*", "*", "bin", "java"),
			filepath.Join(homeDir, ".minecraft", "runtime", "*", "bin", "java"),
			filepath.Join(homeDir, ".minecraft", "java-runtimes", "*", "bin", "java"),
			filepath.Join(homeDir, ".local", "share", "PrismLauncher", "java", "*", "bin", "java"),
		} {
			if matches, err := filepath.Glob(pattern); err == nil {
				candidates = append(candidates, matches...)
			}
		}
	}

	// 4. OS-specific standard directories
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
		scanJavaDir("/opt/jdk", &candidates)
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

var baseVersionRegex = regexp.MustCompile(`\b(1\.\d+(\.\d+)?)\b`)

// ExtractMinecraftBaseVersion extracts the vanilla release version from names like "forge-1.19.2", "fabric-1.20.1", "1.16.5"
func ExtractMinecraftBaseVersion(v string) string {
	matches := baseVersionRegex.FindStringSubmatch(v)
	if len(matches) > 1 {
		return matches[1]
	}
	return v
}

// GetRequiredJavaMajor returns the target Java major version for a given Minecraft release.
func GetRequiredJavaMajor(mcVersion string) int {
	base := ExtractMinecraftBaseVersion(mcVersion)
	if strings.HasPrefix(base, "1.16") ||
		strings.HasPrefix(base, "1.15") ||
		strings.HasPrefix(base, "1.14") ||
		strings.HasPrefix(base, "1.13") ||
		strings.HasPrefix(base, "1.12") ||
		strings.HasPrefix(base, "1.11") ||
		strings.HasPrefix(base, "1.10") ||
		strings.HasPrefix(base, "1.9") ||
		strings.HasPrefix(base, "1.8") ||
		strings.HasPrefix(base, "1.7") {
		return 8
	}
	if strings.HasPrefix(base, "1.17") {
		return 16
	}
	if strings.HasPrefix(base, "1.18") ||
		strings.HasPrefix(base, "1.19") ||
		base == "1.20" ||
		base == "1.20.1" ||
		base == "1.20.2" ||
		base == "1.20.3" ||
		base == "1.20.4" {
		return 17
	}
	return 21
}

// SelectBestJava chooses the appropriate Java version for the given Minecraft release.
func SelectBestJava(mcVersion string, runtimes []JavaInfo) (string, int, error) {
	requiredMajor := GetRequiredJavaMajor(mcVersion)

	// 1. Exact match
	for _, r := range runtimes {
		if r.MajorVersion == requiredMajor {
			return r.Path, requiredMajor, nil
		}
	}

	// 2. Compatible higher version (for Java 17+, except Java 8 which strictly needs 8)
	if requiredMajor > 8 {
		for _, r := range runtimes {
			if r.MajorVersion >= requiredMajor && r.MajorVersion <= 25 {
				return r.Path, requiredMajor, nil
			}
		}
	}

	// 3. Fallback to first available with an error if incompatible
	if len(runtimes) > 0 {
		return runtimes[0].Path, requiredMajor, fmt.Errorf("для версии %s требуется Java %d, но найдена только Java %d (%s)", mcVersion, requiredMajor, runtimes[0].MajorVersion, runtimes[0].Path)
	}

	return "java", requiredMajor, fmt.Errorf("для версии %s требуется Java %d, но Java не обнаружена на компьютере", mcVersion, requiredMajor)
}
