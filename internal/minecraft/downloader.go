package minecraft

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type DownloadTask struct {
	URL  string
	Path string
	Sha1 string
	Size int64
}

type ProgressCallback func(completed, total int64, currentFile string)

// GetDownloadFallbackURLs resolves all available mirrors for a given download URL.
// Primary official URL is tried first, followed by fast, reliable BMCLAPI mirrors and alternates.
func GetDownloadFallbackURLs(rawURL string) []string {
	if rawURL == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return []string{rawURL}
	}

	seen := make(map[string]bool)
	var result []string
	add := func(candidate string) {
		if candidate != "" && !seen[candidate] {
			seen[candidate] = true
			result = append(result, candidate)
		}
	}

	add(rawURL)

	host := strings.ToLower(u.Host)
	path := u.Path

	switch host {
	case "resources.download.minecraft.net":
		// Asset object e.g. /27/27132102cc62dbd42716cd0bf5906420265bb32d
		add("https://bmclapi2.bangbang93.com/assets" + path)
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 2 && len(parts[1]) >= 20 {
			add("https://piston-data.mojang.com/v1/objects/" + parts[1])
		}

	case "libraries.minecraft.net":
		add("https://bmclapi2.bangbang93.com/maven" + path)

	case "maven.fabricmc.net":
		add("https://bmclapi2.bangbang93.com/maven" + path)

	case "maven.minecraftforge.net":
		add("https://bmclapi2.bangbang93.com/maven" + path)

	case "files.minecraftforge.net":
		if strings.HasPrefix(path, "/maven/") {
			add("https://bmclapi2.bangbang93.com" + path)
			add("https://maven.minecraftforge.net" + strings.TrimPrefix(path, "/maven"))
		}

	case "maven.neoforged.net":
		subPath := strings.TrimPrefix(path, "/releases")
		add("https://bmclapi2.bangbang93.com/maven" + subPath)
		add("https://bmclapi2.bangbang93.com/maven" + path)

	case "meta.fabricmc.net":
		add("https://bmclapi2.bangbang93.com/fabric-meta" + path)

	case "piston-meta.mojang.com":
		add("https://bmclapi2.bangbang93.com" + path)
		add("https://launchermeta.mojang.com" + path)

	case "launchermeta.mojang.com":
		add("https://bmclapi2.bangbang93.com" + path)
		add("https://piston-meta.mojang.com" + path)

	case "piston-data.mojang.com", "launcher.mojang.com":
		add("https://bmclapi2.bangbang93.com" + path)
		parts := strings.Split(strings.Trim(path, "/"), "/")
		for i, p := range parts {
			if p == "objects" && i+1 < len(parts) {
				hash := parts[i+1]
				if len(hash) >= 2 {
					add("https://bmclapi2.bangbang93.com/assets/" + hash[:2] + "/" + hash)
					add("https://resources.download.minecraft.net/" + hash[:2] + "/" + hash)
				}
				break
			}
		}
	}

	return result
}

// DownloadBatch downloads multiple tasks in parallel using a worker pool.
func DownloadBatch(tasks []DownloadTask, workerCount int, onProgress ProgressCallback) error {
	if workerCount <= 0 {
		workerCount = 24
	}

	// Filter tasks that are already up to date
	var pending []DownloadTask
	for _, t := range tasks {
		if !isUpToDate(t.Path, t.Sha1, t.Size) {
			pending = append(pending, t)
		}
	}

	total := int64(len(pending))
	if total == 0 {
		if onProgress != nil {
			onProgress(1, 1, "Все файлы актуальны")
		}
		return nil
	}

	taskChan := make(chan DownloadTask, len(pending))
	for _, t := range pending {
		taskChan <- t
	}
	close(taskChan)

	var completed int64
	var errOnce sync.Once
	var firstErr error

	var wg sync.WaitGroup
	client := &http.Client{Timeout: 35 * time.Second}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				if firstErr != nil {
					return
				}

				if err := downloadFile(client, task); err != nil {
					// Check if this task is an asset object (sound/texture)
					isAssetObject := strings.Contains(task.Path, filepath.Join("assets", "objects"))
					if isAssetObject {
						// Don't kill the entire install for an individual non-critical sound asset
						fmt.Printf("[Downloader] Warning: asset skipped after all mirrors failed (%s): %v\n", task.URL, err)
					} else {
						errOnce.Do(func() {
							firstErr = fmt.Errorf("failed downloading %s: %w", task.URL, err)
						})
						return
					}
				}

				done := atomic.AddInt64(&completed, 1)
				if onProgress != nil {
					onProgress(done, total, filepath.Base(task.Path))
				}
			}
		}()
	}

	wg.Wait()
	return firstErr
}

func downloadFile(client *http.Client, task DownloadTask) error {
	_ = os.MkdirAll(filepath.Dir(task.Path), 0755)

	candidates := GetDownloadFallbackURLs(task.URL)
	var lastErr error

	for _, candidateURL := range candidates {
		// Retry up to 2 times for each candidate mirror
		for attempt := 0; attempt < 2; attempt++ {
			err := tryDownloadSingleURL(client, candidateURL, task)
			if err == nil {
				return nil
			}
			lastErr = err
			time.Sleep(120 * time.Millisecond)
		}
	}

	return lastErr
}

func tryDownloadSingleURL(client *http.Client, targetURL string, task DownloadTask) error {
	resp, err := client.Get(targetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status %d from %s", resp.StatusCode, targetURL)
	}

	tempFile := task.Path + ".tmp"
	out, err := os.Create(tempFile)
	if err != nil {
		return err
	}

	hasher := sha1.New()
	writer := io.MultiWriter(out, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		out.Close()
		_ = os.Remove(tempFile)
		return err
	}
	out.Close()

	if task.Sha1 != "" {
		hash := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(hash, task.Sha1) {
			_ = os.Remove(tempFile)
			return fmt.Errorf("hash mismatch for %s: got %s, expected %s", task.Path, hash, task.Sha1)
		}
	}

	return os.Rename(tempFile, task.Path)
}

func isUpToDate(path, expectedSha1 string, expectedSize int64) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}

	if expectedSize > 0 && info.Size() != expectedSize {
		return false
	}

	if expectedSha1 == "" {
		return true
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	return strings.EqualFold(hash, expectedSha1)
}
