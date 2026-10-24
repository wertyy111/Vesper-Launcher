package minecraft

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractNatives unpacks native binaries (.so, .dll, .dylib) from jar files to the natives directory.
func ExtractNatives(jarPath, destDir string) error {
	reader, err := zip.OpenReader(jarPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	_ = os.MkdirAll(destDir, 0755)

	for _, file := range reader.File {
		name := file.Name
		if strings.HasPrefix(name, "META-INF/") || file.FileInfo().IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".so" && ext != ".dll" && ext != ".dylib" && !strings.Contains(name, ".so.") {
			continue
		}

		targetPath := filepath.Join(destDir, filepath.Base(name))
		rc, err := file.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
