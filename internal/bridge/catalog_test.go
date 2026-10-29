package bridge

import (
	"strings"
	"testing"

	"vesper-launcher/internal/state"
)

func TestBuildVersionCatalogIncludesModLoaders(t *testing.T) {
	st := state.NewLauncherState()
	svc := NewBridgeService(st)

	catalog := svc.buildVersionCatalog(nil)

	if len(catalog) == 0 {
		t.Fatalf("catalog should not be empty")
	}

	foundLoaders := map[string]bool{
		"fabric":   false,
		"forge":    false,
		"neoforge": false,
		"optifine": false,
		"release":  false,
	}

	for _, item := range catalog {
		if _, ok := foundLoaders[item.Category]; ok {
			foundLoaders[item.Category] = true
		}
	}

	for cat, found := range foundLoaders {
		if !found {
			t.Errorf("Expected category %q to be present in catalog", cat)
		}
	}

	// Verify specific popular mod versions
	expectedKeys := []string{
		"forge-1.16.5",
		"fabric-1.20.1",
		"forge-1.20.1",
		"optifine-1.20.1",
		"fabric-1.21.4",
		"neoforge-1.21.1",
	}

	catalogKeyMap := make(map[string]string)
	for _, item := range catalog {
		catalogKeyMap[strings.ToLower(item.Key)] = item.DisplayName
	}

	for _, k := range expectedKeys {
		if _, exists := catalogKeyMap[strings.ToLower(k)]; !exists {
			t.Errorf("Expected key %q to be present in version catalog", k)
		}
	}
}
