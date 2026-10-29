package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"vesper-launcher/pkg/updater"
)

var (
	sourceFlag     = flag.String("source", "", "Update manifest URL or server endpoint")
	currentVerFlag = flag.String("version", "1.0.65", "Current launcher version")
	checkOnlyFlag  = flag.Bool("check", false, "Check for update and exit")
	targetBinFlag  = flag.String("target", "", "Path to launcher binary to update")
	restartFlag    = flag.Bool("restart", false, "Relaunch target after updating")
)

func main() {
	flag.Parse()

	client := updater.NewClient(*sourceFlag)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Printf("[VesperUpdater] Checking for updates (current version: %s)...\n", *currentVerFlag)
	res, err := client.CheckUpdate(ctx, *currentVerFlag)
	if err != nil {
		log.Fatalf("[VesperUpdater] Error checking updates: %v", err)
	}

	if !res.UpdateAvailable {
		fmt.Printf("[VesperUpdater] Up to date! Version %s is the latest.\n", *currentVerFlag)
		os.Exit(0)
	}

	fmt.Printf("[VesperUpdater] New version available: %s (Current: %s)\n", res.LatestVersion, *currentVerFlag)
	fmt.Printf("[VesperUpdater] Release title: %s\n", res.Release.Title)
	if res.Release.Changelog != "" {
		fmt.Printf("[VesperUpdater] Changelog:\n%s\n", res.Release.Changelog)
	}

	if *checkOnlyFlag {
		os.Exit(0)
	}

	if res.Asset == nil {
		fmt.Println("[VesperUpdater] No compatible asset found for your platform in this release.")
		os.Exit(1)
	}

	targetBin := *targetBinFlag
	if targetBin == "" {
		execPath, err := os.Executable()
		if err == nil {
			targetBin = execPath
		}
	}

	tempDownload := filepath.Join(os.TempDir(), res.Asset.Name)
	fmt.Printf("[VesperUpdater] Downloading %s (%d bytes)...\n", res.Asset.Name, res.Asset.Size)

	lastPct := -1
	err = client.DownloadFile(ctx, res.Asset.URL, res.Asset.SHA256, tempDownload, func(downloaded, total int64) {
		if total > 0 {
			pct := int((downloaded * 100) / total)
			if pct != lastPct && pct%10 == 0 {
				fmt.Printf("[VesperUpdater] Downloading... %d%%\n", pct)
				lastPct = pct
			}
		}
	})
	if err != nil {
		log.Fatalf("[VesperUpdater] Download failed: %v", err)
	}

	fmt.Println("[VesperUpdater] Download verified successfully!")
	if targetBin != "" {
		fmt.Printf("[VesperUpdater] Applying update to %s...\n", targetBin)
		if err := updater.ApplyBinaryUpdate(tempDownload, targetBin); err != nil {
			log.Fatalf("[VesperUpdater] Failed to apply update: %v", err)
		}
		fmt.Println("[VesperUpdater] Update applied successfully!")

		if *restartFlag {
			fmt.Printf("[VesperUpdater] Restarting %s...\n", targetBin)
			cmd := exec.Command(targetBin)
			_ = cmd.Start()
		}
	}
}
