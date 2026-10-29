package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSemVerComparison(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.1.0", "1.0.99", 1},
		{"2.0.0", "1.99.99", 1},
		{"1.0.65", "1.0.65-beta.46", 1},
		{"1.0.65-beta.47", "1.0.65-beta.46", 1},
		{"v1.0.65-beta.46", "1.0.65-beta.46", 0},
		{"1.0.66", "1.0.65", 1},
	}

	for _, tt := range tests {
		got := IsNewer(tt.a, tt.b)
		expectedNewer := tt.want > 0
		if got != expectedNewer {
			t.Errorf("IsNewer(%q, %q) = %v; want %v", tt.a, tt.b, got, expectedNewer)
		}
	}
}

func TestClientCheckUpdate(t *testing.T) {
	manifest := UpdateManifest{
		SchemaVersion: "1.0",
		Latest: ReleaseInfo{
			Version:     "1.0.66",
			Tag:         "v1.0.66",
			ReleaseDate: time.Now(),
			Title:       "Vesper 1.0.66 Update",
			Changelog:   "- Migrated to Go\n- Full cross-platform support",
			Assets: map[PlatformKey]Asset{
				PlatformLinuxAmd64: {
					Name:   "VesperLauncher-linux-amd64.tar.gz",
					URL:    "http://example.com/download/linux",
					SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					Size:   1024,
					Kind:   "portable",
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(manifest)
	}))
	defer ts.Close()

	client := NewClient(ts.URL)
	res, err := client.CheckUpdate(context.Background(), "1.0.65")
	if err != nil {
		t.Fatalf("CheckUpdate failed: %v", err)
	}

	if !res.UpdateAvailable {
		t.Errorf("Expected update to be available")
	}
	if res.LatestVersion != "1.0.66" {
		t.Errorf("Expected latest version 1.0.66, got %s", res.LatestVersion)
	}

	// Now check when already on latest
	res2, err := client.CheckUpdate(context.Background(), "1.0.66")
	if err != nil {
		t.Fatalf("CheckUpdate failed: %v", err)
	}
	if res2.UpdateAvailable {
		t.Errorf("Expected update to NOT be available when on latest")
	}
}
