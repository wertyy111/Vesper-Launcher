package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"vesper-launcher/pkg/updater"
)

var (
	portFlag     = flag.String("port", "8080", "Port to listen on")
	manifestFlag = flag.String("manifest", "manifest.json", "Path to manifest.json")
	assetsFlag   = flag.String("assets", "assets", "Directory to serve download assets from")
)

type Server struct {
	manifestPath string
	assetsDir    string
	mu           sync.RWMutex
	manifest     *updater.UpdateManifest
}

func NewServer(manifestPath, assetsDir string) *Server {
	s := &Server{
		manifestPath: manifestPath,
		assetsDir:    assetsDir,
	}
	s.reloadManifest()
	return s
}

func (s *Server) reloadManifest() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.manifestPath)
	if err != nil {
		log.Printf("Warning: cannot read manifest %s: %v. Initializing default manifest.", s.manifestPath, err)
		s.manifest = &updater.UpdateManifest{
			SchemaVersion: "1.0",
			UpdatedAt:     time.Now(),
			Latest: updater.ReleaseInfo{
				Version:     "1.0.66",
				Tag:         "v1.0.66",
				ReleaseDate: time.Now(),
				Title:       "Vesper Launcher Go Edition",
				Changelog:   "- 100% Go port with Wails\n- High performance cross-platform updater\n- Minecraft offline multiplayer fix",
				Assets:      make(map[updater.PlatformKey]updater.Asset),
			},
		}
		return
	}

	var m updater.UpdateManifest
	if err := json.Unmarshal(data, &m); err != nil {
		log.Printf("Error parsing manifest: %v", err)
		return
	}
	s.manifest = &m
	log.Printf("Loaded update manifest for latest version: %s", m.Latest.Version)
}

func (s *Server) handleGetManifest(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(s.manifest)
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clientVer := r.URL.Query().Get("version")
	osName := strings.ToLower(r.URL.Query().Get("os"))
	arch := strings.ToLower(r.URL.Query().Get("arch"))

	if osName == "" {
		osName = "linux"
	}
	if arch == "" {
		arch = "amd64"
	}

	platformKey := updater.PlatformKey(fmt.Sprintf("%s-%s", osName, arch))
	latest := s.manifest.Latest
	isNewer := updater.IsNewer(latest.Version, clientVer)

	res := updater.CheckUpdateResult{
		UpdateAvailable: isNewer,
		CurrentVersion:  clientVer,
		LatestVersion:   latest.Version,
		Release:         &latest,
		Mandatory:       latest.Mandatory,
	}

	if asset, ok := latest.Assets[platformKey]; ok {
		res.Asset = &asset
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleNews(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(s.manifest.News)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","service":"vesper-update-server"}`))
}

func main() {
	flag.Parse()

	srv := NewServer(*manifestFlag, *assetsFlag)

	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", srv.handleGetManifest)
	mux.HandleFunc("/api/v1/updates/latest", srv.handleGetManifest)
	mux.HandleFunc("/api/v1/updates/check", srv.handleCheck)
	mux.HandleFunc("/api/v1/updates/news", srv.handleNews)
	mux.HandleFunc("/health", srv.handleHealth)

	if _, err := os.Stat(*assetsFlag); err == nil {
		mux.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir(*assetsFlag))))
	}

	addr := ":" + *portFlag
	log.Printf("Vesper Update Server listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
