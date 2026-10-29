package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"vesper-launcher/internal/platform"
	"vesper-launcher/internal/state"
)

func TestModDownloadAndDelete(t *testing.T) {
	s := state.NewLauncherState()
	b := NewBridgeService(s)

	mcDir := platform.GetMinecraftDir()
	modsDir := filepath.Join(mcDir, "mods")
	_ = os.MkdirAll(modsDir, 0755)

	testFileName := "test-dummy-mod-1.0.jar"
	testFilePath := filepath.Join(modsDir, testFileName)

	if err := os.WriteFile(testFilePath, []byte("dummy mod content"), 0644); err != nil {
		t.Fatalf("Failed to write dummy mod: %v", err)
	}

	files := s.ScanInstalledFiles()
	found := false
	for _, f := range files {
		if f == testFileName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ScanInstalledFiles failed to locate %s", testFileName)
	}

	b.deleteModFile("test-dummy-mod", testFileName)

	if _, err := os.Stat(testFilePath); !os.IsNotExist(err) {
		t.Fatalf("Expected file to be deleted, but it still exists")
	}

	_ = os.Remove(testFilePath)
}

func TestDeleteMinecraftVersion(t *testing.T) {
	s := state.NewLauncherState()
	b := NewBridgeService(s)

	mcDir := platform.GetMinecraftDir()
	versionsDir := filepath.Join(mcDir, "versions")
	testVerDir := filepath.Join(versionsDir, "test-version-dummy")
	_ = os.MkdirAll(testVerDir, 0755)
	_ = os.WriteFile(filepath.Join(testVerDir, "test-version-dummy.json"), []byte("{}"), 0644)

	// Call deleteMinecraftVersion
	b.deleteMinecraftVersion("test-version-dummy")

	// Verify folder is removed
	if _, err := os.Stat(testVerDir); !os.IsNotExist(err) {
		t.Fatalf("Expected version folder to be removed, but it still exists")
	}
}
