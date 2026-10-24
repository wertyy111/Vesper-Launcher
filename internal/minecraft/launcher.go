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
	VersionID    string
	Nickname     string
	MemoryMb     int
	CustomJava   string
	CustomJvm    string
	OnProgress   func(percent float64, message string)
	OnGameStart  func()
	OnGameFinish func(exitCode int)
}

// LaunchGame orchestrates the full sequence of verifying files, building arguments, and spawning the Minecraft process.
func LaunchGame(opts LaunchOptions) (*exec.Cmd, error) {
	mcDir := platform.GetMinecraftDir()
	versionsDir := filepath.Join(mcDir, "versions", opts.VersionID)
	versionJsonPath := filepath.Join(versionsDir, opts.VersionID+".json")

	if opts.OnProgress != nil {
		opts.OnProgress(10, "Загрузка конфигурации версии...")
	}

	pkg, err := LoadVersionPackage(opts.VersionID, "")
	if err != nil {
		// Fallback: check if it exists in bundled
		bundledPath := filepath.Join("bundled", "BundledVersions", opts.VersionID, opts.VersionID+".json")
		if b, errRead := os.ReadFile(bundledPath); errRead == nil {
			_ = os.MkdirAll(versionsDir, 0755)
			_ = os.WriteFile(versionJsonPath, b, 0644)
			pkg, err = LoadVersionPackage(opts.VersionID, "")
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load version package for %s: %w", opts.VersionID, err)
	}

	// 1. Locate Java runtime
	javaPath := opts.CustomJava
	if javaPath == "" {
		runtimes := java.DetectRuntimes()
		javaPath = java.SelectBestJava(opts.VersionID, runtimes)
	}

	if opts.OnProgress != nil {
		opts.OnProgress(30, "Проверка библиотек и natives...")
	}

	// 2. Collect libraries and natives
	var libPaths []string
	librariesDir := filepath.Join(mcDir, "libraries")
	nativesDir := filepath.Join(versionsDir, "natives")
	_ = os.MkdirAll(nativesDir, 0755)

	currentOS := getCurrentOSName()

	for _, lib := range pkg.Libraries {
		if !CheckRules(lib.Rules) {
			continue
		}

		if lib.Downloads.Artifact != nil && lib.Downloads.Artifact.Path != "" {
			fullPath := filepath.Join(librariesDir, lib.Downloads.Artifact.Path)
			if _, err := os.Stat(fullPath); err == nil {
				libPaths = append(libPaths, fullPath)
			}
		}

		// Check natives classifier
		if lib.Natives != nil {
			classifierKey := lib.Natives[currentOS]
			if classifierKey != "" && lib.Downloads.Classifiers != nil {
				if artifact, ok := lib.Downloads.Classifiers[classifierKey]; ok && artifact.Path != "" {
					nativeJarPath := filepath.Join(librariesDir, artifact.Path)
					if _, err := os.Stat(nativeJarPath); err == nil {
						_ = ExtractNatives(nativeJarPath, nativesDir)
					}
				}
			}
		}
	}

	// 3. Client JAR
	clientJar := filepath.Join(versionsDir, opts.VersionID+".jar")

	// 4. Classpath
	classpath := BuildClasspath(libPaths, clientJar)

	if opts.OnProgress != nil {
		opts.OnProgress(80, "Формирование аргументов запуска...")
	}

	// 5. Auth profile
	profile := auth.CreateOfflineProfile(opts.Nickname)

	// 6. Build Context
	assetsDir := filepath.Join(mcDir, "assets")
	assetIndex := pkg.Assets
	if assetIndex == "" && pkg.AssetIndex != nil {
		assetIndex = pkg.AssetIndex.ID
	}
	if assetIndex == "" {
		assetIndex = "legacy"
	}

	ctx := LaunchContext{
		MinecraftDir: mcDir,
		VersionID:    opts.VersionID,
		NativesDir:   nativesDir,
		Classpath:    classpath,
		MainClass:    pkg.MainClass,
		Username:     profile.Username,
		UUID:         profile.UUID,
		AccessToken:  profile.AccessToken,
		UserType:     profile.UserType,
		VersionType:  "release",
		AssetsDir:    assetsDir,
		AssetIndex:   assetIndex,
		MemoryMb:     opts.MemoryMb,
		CustomJvm:    opts.CustomJvm,
	}

	if ctx.MainClass == "" {
		ctx.MainClass = "net.minecraft.client.main.Main"
	}

	jvmArgs := BuildJvmArguments(ctx)
	gameArgs := BuildGameArguments(ctx, pkg.MinecraftArguments)

	var finalArgs []string
	finalArgs = append(finalArgs, jvmArgs...)
	finalArgs = append(finalArgs, "-cp", classpath)
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
