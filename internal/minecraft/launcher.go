package minecraft

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vesper-launcher/internal/auth"
	"vesper-launcher/internal/java"
	"vesper-launcher/internal/platform"
)

type LaunchOptions struct {
	VersionID       string
	Nickname        string
	MemoryMb        int
	JavaRuntimeMode string
	CustomJava      string
	CustomJvm       string
	OnProgress      func(percent float64, message string)
	OnGameStart     func()
	OnGameFinish    func(exitCode int)
}

// LaunchGame orchestrates the full sequence of verifying files, building arguments, and spawning the Minecraft process.
func LaunchGame(opts LaunchOptions) (*exec.Cmd, error) {
	mcDir := platform.GetMinecraftDir()
	versionsDir := filepath.Join(mcDir, "versions", opts.VersionID)

	// 1. Ensure version and all dependencies (libraries, assets, client jar) are installed
	pkg, err := EnsureVersionInstalled(opts.VersionID, opts.OnProgress)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare version %s: %w", opts.VersionID, err)
	}

	var parentPkg *VersionPackage
	if pkg.InheritsFrom != "" {
		parentPkg, _ = LoadVersionPackage(pkg.InheritsFrom, "")
	}

	// 2. Locate Java runtime based on JavaRuntimeMode (auto, system, custom)
	var javaPath string
	switch opts.JavaRuntimeMode {
	case "custom":
		javaPath = strings.TrimSpace(opts.CustomJava)
		if javaPath == "" {
			return nil, fmt.Errorf("в настройках выбран режим 'Свой', но путь к Java не указан. Укажите путь к исполняемому файлу java")
		}
		if _, err := os.Stat(javaPath); err != nil {
			return nil, fmt.Errorf("указанный файл Java не найден: %s", javaPath)
		}
	case "system":
		sysJava, err := exec.LookPath("java")
		if err != nil {
			return nil, fmt.Errorf("системная Java не найдена в PATH (переменных окружения)")
		}
		javaPath = sysJava
	case "auto", "":
		if opts.CustomJava != "" && opts.JavaRuntimeMode == "" {
			javaPath = opts.CustomJava
		} else {
			runtimes := java.DetectRuntimes()
			bestPath, reqMajor, err := java.SelectBestJava(opts.VersionID, runtimes)
			if err != nil {
				if opts.OnProgress != nil {
					opts.OnProgress(10, fmt.Sprintf("Подготовка Java %d для версии %s...", reqMajor, opts.VersionID))
				}
				dlJava, dlErr := java.EnsureJavaRuntime(reqMajor, opts.OnProgress)
				if dlErr != nil {
					return nil, fmt.Errorf("для версии %s требуется Java %d, но не удалось автоматически установить её: %w", opts.VersionID, reqMajor, dlErr)
				}
				bestPath = dlJava
			}
			javaPath = bestPath
		}
	}

	if opts.OnProgress != nil {
		opts.OnProgress(85, "Сборка ClassPath и окружения...")
	}

	// 3. Collect libraries and natives
	var libPaths []string
	librariesDir := filepath.Join(mcDir, "libraries")
	nativesDir := filepath.Join(versionsDir, "natives")

	seenLibs := make(map[string]bool)
	collectLibraries := func(libs []LibraryEntry) {
		for _, lib := range libs {
			if !CheckRules(lib.Rules) {
				continue
			}

			if lib.Downloads.Artifact != nil && lib.Downloads.Artifact.Path != "" {
				fullPath := filepath.Join(librariesDir, lib.Downloads.Artifact.Path)
				if _, err := os.Stat(fullPath); err == nil && !seenLibs[fullPath] {
					seenLibs[fullPath] = true
					libPaths = append(libPaths, fullPath)
				}
			} else if lib.Name != "" {
				if relPath, err := MavenCoordinatesToPath(lib.Name); err == nil {
					fullPath := filepath.Join(librariesDir, relPath)
					if _, errS := os.Stat(fullPath); errS == nil && !seenLibs[fullPath] {
						seenLibs[fullPath] = true
						libPaths = append(libPaths, fullPath)
					}
				}
			}
		}
	}

	if parentPkg != nil {
		collectLibraries(MergeLibraries(parentPkg.Libraries, pkg.Libraries))
	} else {
		collectLibraries(pkg.Libraries)
	}

	// 4. Client JAR
	clientJar := filepath.Join(versionsDir, opts.VersionID+".jar")
	if _, err := os.Stat(clientJar); os.IsNotExist(err) && parentPkg != nil {
		parentJar := filepath.Join(mcDir, "versions", parentPkg.ID, parentPkg.ID+".jar")
		if _, errP := os.Stat(parentJar); errP == nil {
			clientJar = parentJar
		}
	}

	// 5. Classpath
	classpath := BuildClasspath(libPaths, clientJar)

	if opts.OnProgress != nil {
		opts.OnProgress(90, "Формирование аргументов запуска...")
	}

	// 6. Auth profile
	profile := auth.CreateOfflineProfile(opts.Nickname)

	userUUID := profile.UUID
	userToken := profile.AccessToken
	userType := "mojang"
	if isLegacyVersion(opts.VersionID) {
		raw := strings.ReplaceAll(profile.UUID, "-", "")
		userUUID = raw
		userToken = raw
	}

	// 7. Assets resolution
	assetsDir := filepath.Join(mcDir, "assets")
	assetIndex := pkg.Assets
	if assetIndex == "" && pkg.AssetIndex != nil {
		assetIndex = pkg.AssetIndex.ID
	}
	if assetIndex == "" && parentPkg != nil {
		assetIndex = parentPkg.Assets
		if assetIndex == "" && parentPkg.AssetIndex != nil {
			assetIndex = parentPkg.AssetIndex.ID
		}
	}
	if assetIndex == "" {
		assetIndex = "legacy"
	}

	mainClass := pkg.MainClass
	if mainClass == "" && parentPkg != nil {
		mainClass = parentPkg.MainClass
	}
	if mainClass == "" {
		mainClass = "net.minecraft.client.main.Main"
	}

	var gameArgsList []any
	var jvmArgsList []any
	if parentPkg != nil && parentPkg.Arguments != nil {
		gameArgsList = append(gameArgsList, parentPkg.Arguments.Game...)
		jvmArgsList = append(jvmArgsList, parentPkg.Arguments.Jvm...)
	}
	if pkg.Arguments != nil {
		gameArgsList = append(gameArgsList, pkg.Arguments.Game...)
		jvmArgsList = append(jvmArgsList, pkg.Arguments.Jvm...)
	}

	parentID := ""
	if parentPkg != nil {
		parentID = parentPkg.ID
	}

	ctx := LaunchContext{
		MinecraftDir:    mcDir,
		VersionID:       opts.VersionID,
		ParentVersionID: parentID,
		NativesDir:      nativesDir,
		Classpath:    classpath,
		MainClass:    mainClass,
		Username:     profile.Username,
		UUID:         userUUID,
		AccessToken:  userToken,
		UserType:     userType,
		VersionType:  "release",
		AssetsDir:    assetsDir,
		AssetIndex:   assetIndex,
		MemoryMb:     opts.MemoryMb,
		CustomJvm:    opts.CustomJvm,
		GameArgs:     gameArgsList,
		JvmArgs:      jvmArgsList,
	}

	jvmArgs := BuildJvmArguments(ctx)
	mcArgs := pkg.MinecraftArguments
	if mcArgs == "" && parentPkg != nil {
		mcArgs = parentPkg.MinecraftArguments
	}
	gameArgs := BuildGameArguments(ctx, mcArgs)

	var finalArgs []string
	finalArgs = append(finalArgs, jvmArgs...)
	finalArgs = append(finalArgs, ctx.MainClass)
	finalArgs = append(finalArgs, gameArgs...)

	if opts.OnProgress != nil {
		opts.OnProgress(100, "Запуск процесса...")
	}

	cmd := exec.Command(javaPath, finalArgs...)
	cmd.Dir = mcDir

	// Setup logging
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Minecraft process: %w", err)
	}

	if opts.OnGameStart != nil {
		opts.OnGameStart()
	}

	// Stream stdout & stderr
	go func() {
		logFile, _ := os.Create(filepath.Join(mcDir, "game-latest.log"))
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if logFile != nil {
				_, _ = logFile.WriteString(line + "\n")
			}
		}
		if logFile != nil {
			_ = logFile.Close()
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "ERROR") || strings.Contains(line, "Exception") {
				fmt.Println("[MC Error]", line)
			}
		}
	}()

	// Wait for exit
	go func() {
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		if opts.OnGameFinish != nil {
			opts.OnGameFinish(exitCode)
		}
	}()

	return cmd, nil
}

func isLegacyVersion(versionID string) bool {
	v := strings.ToLower(versionID)
	legacyMarkers := []string{"1.16", "1.15", "1.14", "1.13", "1.12", "1.11", "1.10", "1.9", "1.8", "1.7", "1.6", "1.5"}
	for _, marker := range legacyMarkers {
		if strings.Contains(v, marker) {
			return true
		}
	}
	return false
}
