package minecraft

import (
	"fmt"
	"runtime"
	"strings"
)

type LaunchContext struct {
	MinecraftDir string
	VersionID    string
	NativesDir   string
	Classpath    string
	MainClass    string
	Username     string
	UUID         string
	AccessToken  string
	UserType     string
	VersionType  string
	AssetsDir    string
	AssetIndex   string
	MemoryMb     int
	CustomJvm    string
}

// CheckRules evaluates whether a library or argument rule matches the current OS.
func CheckRules(rules []RuleEntry) bool {
	if len(rules) == 0 {
		return true
	}

	currentOS := getCurrentOSName()
	allowed := false

	for _, rule := range rules {
		match := true

		if rule.OS.Name != "" {
			if rule.OS.Name != currentOS {
				match = false
			}
		}

		if match {
			if rule.Action == "allow" {
				allowed = true
			} else if rule.Action == "disallow" {
				allowed = false
			}
		}
	}

	return allowed
}

func getCurrentOSName() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "osx"
	default:
		return "linux"
	}
}

// BuildJvmArguments generates standard JVM arguments for Minecraft.
func BuildJvmArguments(ctx LaunchContext) []string {
	var args []string

	// Memory limits
	mem := ctx.MemoryMb
	if mem < 1024 {
		mem = 2048
	}
	args = append(args, fmt.Sprintf("-Xmx%dM", mem))
	args = append(args, fmt.Sprintf("-Xms%dM", 512))

	// Natives path
	args = append(args, fmt.Sprintf("-Djava.library.path=%s", ctx.NativesDir))
	args = append(args, fmt.Sprintf("-Dorg.lwjgl.librarypath=%s", ctx.NativesDir))

	// Encoding
	args = append(args, "-Dfile.encoding=UTF-8")

	// Custom user JVM args
	if ctx.CustomJvm != "" {
		custom := strings.Fields(ctx.CustomJvm)
		args = append(args, custom...)
	}

	// Classpath separator
	cpSeparator := ":"
	if runtime.GOOS == "windows" {
		cpSeparator = ";"
	}
	_ = cpSeparator

	return args
}

// BuildGameArguments formats the command line flags passed to the Minecraft main class.
func BuildGameArguments(ctx LaunchContext, rawTemplate string) []string {
	if rawTemplate == "" {
		return []string{
			"--username", ctx.Username,
			"--version", ctx.VersionID,
			"--gameDir", ctx.MinecraftDir,
			"--assetsDir", ctx.AssetsDir,
			"--assetIndex", ctx.AssetIndex,
			"--uuid", ctx.UUID,
			"--accessToken", ctx.AccessToken,
			"--userType", ctx.UserType,
			"--versionType", ctx.VersionType,
		}
	}

	replacements := map[string]string{
		"${auth_player_name}": ctx.Username,
		"${version_name}":     ctx.VersionID,
		"${game_directory}":   ctx.MinecraftDir,
		"${assets_root}":      ctx.AssetsDir,
		"${assets_index_name}": ctx.AssetIndex,
		"${auth_uuid}":        ctx.UUID,
		"${auth_access_token}": ctx.AccessToken,
		"${user_type}":        ctx.UserType,
		"${version_type}":     ctx.VersionType,
	}

	tokens := strings.Fields(rawTemplate)
	var result []string
	for _, tok := range tokens {
		val := tok
		for k, v := range replacements {
			val = strings.ReplaceAll(val, k, v)
		}
		result = append(result, val)
	}

	return result
}

// BuildClasspath joins all library JARs and the client JAR into a classpath string.
func BuildClasspath(libraries []string, clientJar string) string {
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}

	all := append(libraries, clientJar)
	return strings.Join(all, sep)
}
