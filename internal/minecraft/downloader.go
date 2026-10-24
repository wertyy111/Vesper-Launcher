package minecraft

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

// DownloadBatch downloads multiple tasks in parallel using a worker pool.
func DownloadBatch(tasks []DownloadTask, workerCount int, onProgress ProgressCallback) error {
	if workerCount <= 0 {
		workerCount = 16
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
	client := &http.Client{Timeout: 30 * time.Second}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				if firstErr != nil {
					return
				}

				if err := downloadFile(client, task); err != nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("failed downloading %s: %w", task.URL, err)
					})
					return
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

	resp, err := client.Get(task.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
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
		if hash != task.Sha1 {
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
	return hash == expectedSha1
}
