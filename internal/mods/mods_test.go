package mods

import (
	"testing"
)

func TestCleanGameVersionAndLoader(t *testing.T) {
	tests := []struct {
		input       string
		wantVersion string
		wantLoader  string
	}{
		{"fabric-1.19.2", "1.19.2", "fabric"},
		{"forge-1.20.1", "1.20.1", "forge"},
		{"neoforge-1.21.1", "1.21.1", "neoforge"},
		{"quilt-1.20.1", "1.20.1", "quilt"},
		{"1.21.4", "1.21.4", ""},
		{"1.12.2", "1.12.2", ""},
		{"optifine-1.20.1", "1.20.1", ""},
		{"ForgeOptiFine 1.16.5", "1.16.5", "forge"},
		{"Fabric 1.20.1", "1.20.1", "fabric"},
		{"", "", ""},
	}

	for _, tt := range tests {
		v, l := CleanGameVersionAndLoader(tt.input)
		if v != tt.wantVersion || l != tt.wantLoader {
			t.Errorf("CleanGameVersionAndLoader(%q) = (%q, %q), want (%q, %q)",
				tt.input, v, l, tt.wantVersion, tt.wantLoader)
		}
	}
}

func TestSearchCurseForge(t *testing.T) {
	mods := SearchCurseForge("", "Моды")
	if len(mods) == 0 {
		t.Fatalf("expected non-empty mods in CurseForge preview")
	}

	shaders := SearchCurseForge("", "Шейдеры")
	if len(shaders) == 0 {
		t.Fatalf("expected non-empty shaders in CurseForge preview")
	}

	filtered := SearchCurseForge("JEI", "Моды")
	if len(filtered) != 1 || filtered[0].ProjectId != "cf-jei" {
		t.Fatalf("expected JEI to be found in mods")
	}
}
