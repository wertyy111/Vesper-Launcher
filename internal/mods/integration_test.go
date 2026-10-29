package mods

import (
	"testing"
)

func TestIntegration(t *testing.T) {
	c := NewModrinthClient()

	// 1. Fabric 1.20.1 Mods
	items, err := c.Search("", "Моды", "fabric-1.20.1", "", 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("Expected mods, got 0")
	}
	t.Logf("Found %d Fabric 1.20.1 mods:", len(items))
	for _, it := range items {
		t.Logf(" - [%s] %s (kind: %s, downloads: %d)", it.ProjectId, it.DisplayName, it.ContentKind, it.Downloads)
		if it.ContentKind != "mod" {
			t.Errorf("Expected contentKind 'mod', got %s", it.ContentKind)
		}
	}

	// 2. Resource packs
	rpacks, err := c.Search("", "Ресурспаки", "1.20.1", "", 3)
	if err != nil {
		t.Fatalf("Resourcepack search failed: %v", err)
	}
	if len(rpacks) == 0 {
		t.Fatalf("Expected resourcepacks, got 0")
	}
	t.Logf("Found %d resourcepacks:", len(rpacks))
	for _, it := range rpacks {
		t.Logf(" - [%s] %s (kind: %s)", it.ProjectId, it.DisplayName, it.ContentKind)
		if it.ContentKind != "resourcepack" {
			t.Errorf("Expected contentKind 'resourcepack', got %s", it.ContentKind)
		}
	}

	// 3. Shaders
	shaders, err := c.Search("", "Шейдеры", "1.20.1", "", 3)
	if err != nil {
		t.Fatalf("Shaders search failed: %v", err)
	}
	if len(shaders) == 0 {
		t.Fatalf("Expected shaders, got 0")
	}
	t.Logf("Found %d shaders:", len(shaders))
	for _, it := range shaders {
		t.Logf(" - [%s] %s (kind: %s)", it.ProjectId, it.DisplayName, it.ContentKind)
		if it.ContentKind != "shader" {
			t.Errorf("Expected contentKind 'shader', got %s", it.ContentKind)
		}
	}

	// 4. Modpacks
	mpacks, err := c.Search("", "Сборки", "1.20.1", "", 3)
	if err != nil {
		t.Fatalf("Modpacks search failed: %v", err)
	}
	if len(mpacks) == 0 {
		t.Fatalf("Expected modpacks, got 0")
	}
	t.Logf("Found %d modpacks:", len(mpacks))
	for _, it := range mpacks {
		t.Logf(" - [%s] %s (kind: %s)", it.ProjectId, it.DisplayName, it.ContentKind)
		if it.ContentKind != "modpack" {
			t.Errorf("Expected contentKind 'modpack', got %s", it.ContentKind)
		}
	}

	// 5. Best version resolution for sodium
	fn, dl, err := c.GetProjectBestVersion("sodium", "fabric-1.20.1", "")
	if err != nil {
		t.Fatalf("Best version resolution failed: %v", err)
	}
	if fn == "" || dl == "" {
		t.Fatalf("Expected filename and download URL, got empty")
	}
	t.Logf("Sodium resolved: filename=%s, url=%s", fn, dl)

	// 6. CurseForge Preview
	cfMods := SearchCurseForge("", "Моды")
	if len(cfMods) == 0 {
		t.Fatalf("Expected CurseForge mods")
	}
	t.Logf("Found %d CurseForge mods:", len(cfMods))
}
