package minecraft

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type LaunchContext struct {
	MinecraftDir    string
	VersionID       string
	ParentVersionID string
	NativesDir      string
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
	GameArgs     []any
	JvmArgs      []any
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

// BuildJvmArguments generates standard JVM arguments for Minecraft, including online fixes and compatibility flags.
func BuildJvmArguments(ctx LaunchContext) []string {
	var args []string

	// Memory limits
	mem := ctx.MemoryMb
	if mem < 1024 {
		mem = 2048
	}
	args = append(args, fmt.Sprintf("-Xmx%dM", mem))
	args = append(args, "-Xms512M")

	// Aikar's high-performance garbage collection flags (from C# JvmArgBuilder.cs)
	args = append(args,
		"-XX:+UseG1GC",
		"-XX:+ParallelRefProcEnabled",
		"-XX:MaxGCPauseMillis=50",
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:G1NewSizePercent=20",
		"-XX:G1ReservePercent=20",
		"-XX:G1MixedGCCountTarget=4",
		"-XX:G1HeapWastePercent=5",
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:G1RSetUpdatingPauseTimePercent=5",
		"-XX:SurvivorRatio=32",
		"-XX:+PerfDisableSharedMem",
		"-XX:MaxTenuringThreshold=1",
	)

	// Natives path is added below (only if metadata doesn't provide it)

	// Encoding
	args = append(args, "-Dfile.encoding=UTF-8")
	args = append(args, "-Dsun.stdout.encoding=UTF-8")
	args = append(args, "-Dsun.stderr.encoding=UTF-8")

	// Network / DNS fixes for multiplayer (prevents IPv6 connection hang and breaks)
	args = append(args, "-Djava.net.preferIPv4Stack=true")
	args = append(args, "-Djava.net.useSystemProxies=true")
	args = append(args, "-Dlog4j2.formatMsgNoLookups=true")

	// Java 9+ compatibility & socket / reflection opens (from C#)
	args = append(args, "-XX:+IgnoreUnrecognizedVMOptions")
	args = append(args, "--add-exports=java.base/sun.security.util=ALL-UNNAMED")
	args = append(args, "--add-exports=jdk.naming.dns/com.sun.jndi.dns=java.naming")
	args = append(args, "--add-opens=java.base/java.util.jar=ALL-UNNAMED")
	args = append(args, "--add-opens=java.base/java.lang=ALL-UNNAMED")
	args = append(args, "--add-opens=java.base/java.lang.reflect=ALL-UNNAMED")
	args = append(args, "--add-opens=java.base/java.net=ALL-UNNAMED")
	args = append(args, "--add-opens=java.base/java.util=ALL-UNNAMED")

	// Force multiplayer button and chat enabled in offline mode (from C# line 8917)
	args = append(args, "-Dcom.mojang.minecraft.multiplayer=true")
	args = append(args, "-Dcom.mojang.minecraft.onlineChat=true")
	args = append(args, "-Dcom.mojang.minecraft.chat=true")

	// TLauncher-style 1.16.5 / 1.16.x offline multiplayer fix:
	// In Minecraft 1.16.4/1.16.5, Mojang added Microsoft account safety privilege checks.
	// When launched without a Microsoft account, contacting api.minecraftservices.com returns 401 Unauthorized,
	// which locks the "Multiplayer" button ("Сетевая игра отключена. Проверьте настройки учетной записи Microsoft").
	// Setting api services hosts to nope.invalid forces Minecraft to catch UnknownHostException,
	// causing it to fall back to enabling multiplayer in offline mode!
	if strings.Contains(ctx.VersionID, "1.16") || strings.Contains(ctx.ParentVersionID, "1.16") {
		args = append(args, "-Dminecraft.api.auth.host=https://nope.invalid")
		args = append(args, "-Dminecraft.api.account.host=https://nope.invalid")
		args = append(args, "-Dminecraft.api.session.host=https://nope.invalid")
		args = append(args, "-Dminecraft.api.services.host=https://nope.invalid")
	}

	hasClasspath := false
	hasNativePath := false

	// Metadata JVM arguments if any (e.g., from Fabric/Forge JSON)
	if len(ctx.JvmArgs) > 0 {
		replacements := getReplacements(ctx)
		parsedJvm := parseArgumentEntries(ctx.JvmArgs, replacements, ctx.ParentVersionID)
		for _, arg := range parsedJvm {
			if arg == "-cp" || arg == "-classpath" {
				hasClasspath = true
			}
			if strings.HasPrefix(arg, "-Djava.library.path=") {
				hasNativePath = true
			}
			args = append(args, arg)
		}
	}

	if !hasNativePath {
		args = append(args, fmt.Sprintf("-Djava.library.path=%s", ctx.NativesDir))
		args = append(args, fmt.Sprintf("-Dorg.lwjgl.librarypath=%s", ctx.NativesDir))
	}

	if !hasClasspath {
		args = append(args, "-cp", ctx.Classpath)
	}

	// Custom user JVM args
	if ctx.CustomJvm != "" {
		custom := strings.Fields(ctx.CustomJvm)
		args = append(args, custom...)
	}

	return args
}

func getReplacements(ctx LaunchContext) map[string]string {
	cpSep := ":"
	if runtime.GOOS == "windows" {
		cpSep = ";"
	}

	return map[string]string{
		"${auth_player_name}":    ctx.Username,
		"${version_name}":        ctx.VersionID,
		"${game_directory}":      ctx.MinecraftDir,
		"${assets_root}":         ctx.AssetsDir,
		"${assets_index_name}":   ctx.AssetIndex,
		"${auth_uuid}":           ctx.UUID,
		"${auth_access_token}":   ctx.AccessToken,
		"${user_type}":           ctx.UserType,
		"${version_type}":        ctx.VersionType,
		"${user_properties}":     "{}",
		"${clientid}":            "offline-client",
		"${auth_xuid}":           "0",
		"${classpath}":           ctx.Classpath,
		"${natives_directory}":   ctx.NativesDir,
		"${library_directory}":   filepath.Join(ctx.MinecraftDir, "libraries"),
		"${classpath_separator}": cpSep,
		"${resolution_width}":    "1280",
		"${resolution_height}":   "720",
		"${launcher_name}":       "VesperLauncher",
		"${launcher_version}":    "1.0.0",
		"${version_type_name}":   ctx.VersionType,
	}
}

func appendParentJarToIgnoreList(arg, parentVersion string) string {
	if !strings.HasPrefix(arg, "-DignoreList=") || parentVersion == "" {
		return arg
	}
	target := parentVersion + ".jar"
	tokens := strings.Split(arg, ",")
	for _, t := range tokens {
		if t == target {
			return arg
		}
	}
	return arg + "," + target
}

func parseArgumentEntries(entries []any, replacements map[string]string, parentVersion string) []string {
	var result []string
	for _, entry := range entries {
		switch v := entry.(type) {
		case string:
			expanded := replaceTokens(v, replacements)
			expanded = appendParentJarToIgnoreList(expanded, parentVersion)
			result = append(result, expanded)
		case map[string]any:
			// check rules if any
			if rulesAny, ok := v["rules"].([]any); ok {
				if !evalRulesAny(rulesAny) {
					continue
				}
			}
			valAny := v["value"]
			switch val := valAny.(type) {
			case string:
				expanded := replaceTokens(val, replacements)
				expanded = appendParentJarToIgnoreList(expanded, parentVersion)
				result = append(result, expanded)
			case []any:
				for _, part := range val {
					if s, ok := part.(string); ok {
						expanded := replaceTokens(s, replacements)
						expanded = appendParentJarToIgnoreList(expanded, parentVersion)
						result = append(result, expanded)
					}
				}
			}
		}
	}
	return result
}

func evalRulesAny(rules []any) bool {
	if len(rules) == 0 {
		return true
	}
	currentOS := getCurrentOSName()
	allowed := false

	for _, r := range rules {
		ruleMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		action, _ := ruleMap["action"].(string)
		match := true

		if osMap, ok := ruleMap["os"].(map[string]any); ok {
			if name, ok := osMap["name"].(string); ok && name != "" {
				if name != currentOS {
					match = false
				}
			}
		}

		if featuresMap, ok := ruleMap["features"].(map[string]any); ok {
			if isDemo, ok := featuresMap["is_demo_user"].(bool); ok && isDemo {
				match = false
			}
			if hasQuickPlay, ok := featuresMap["has_quick_plays_support"].(bool); ok && hasQuickPlay {
				match = false
			}
		}

		if match {
			if action == "allow" {
				allowed = true
			} else if action == "disallow" {
				allowed = false
			}
		}
	}
	return allowed
}

func replaceTokens(str string, replacements map[string]string) string {
	res := str
	for k, v := range replacements {
		res = strings.ReplaceAll(res, k, v)
	}
	return res
}

// BuildGameArguments formats the command line flags passed to the Minecraft main class.
func BuildGameArguments(ctx LaunchContext, rawTemplate string) []string {
	replacements := getReplacements(ctx)
	var result []string

	if len(ctx.GameArgs) > 0 {
		result = parseArgumentEntries(ctx.GameArgs, replacements, ctx.ParentVersionID)
	} else if rawTemplate != "" {
		tokens := strings.Fields(rawTemplate)
		for _, tok := range tokens {
			result = append(result, replaceTokens(tok, replacements))
		}
	} else {
		result = []string{
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

	// Ensure --userProperties is passed if not present
	hasUserProps := false
	for _, arg := range result {
		if strings.EqualFold(arg, "--userProperties") {
			hasUserProps = true
			break
		}
	}
	if !hasUserProps {
		result = append(result, "--userProperties", "{}")
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
