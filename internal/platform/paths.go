package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetMinecraftDir returns the standard Minecraft data directory for the current OS.
func GetMinecraftDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, ".minecraft")
		}
		return filepath.Join(home, "AppData", "Roaming", ".minecraft")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "minecraft")
	default: // linux, bsd, etc.
		return filepath.Join(home, ".minecraft")
	}
}

// GetVesperConfigDir returns the directory where launcher configuration files are stored.
func GetVesperConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "VesperLauncher")
		}
		return filepath.Join(home, "AppData", "Roaming", "VesperLauncher")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "VesperLauncher")
	default:
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome != "" {
			return filepath.Join(configHome, "VesperLauncher")
		}
		return filepath.Join(home, ".config", "VesperLauncher")
	}
}
