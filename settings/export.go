package settings

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var allowedFiles = map[string]bool{
	"settings.json": true,
	"servers.json":  true,
	"groups.json":   true,
}

// ExportSettings creates a ZIP file containing all config files
func ExportSettings(zipPath string) error {
	configDir := ConfigDirPath()

	f, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	for name := range allowedFiles {
		src := filepath.Join(configDir, name)
		data, err := os.ReadFile(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue // skip missing files
			}
			return fmt.Errorf("read %s: %w", name, err)
		}
		fw, err := w.Create(name)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", name, err)
		}
		if _, err := fw.Write(data); err != nil {
			return fmt.Errorf("write zip entry %s: %w", name, err)
		}
	}

	return nil
}

// ImportSettings extracts config files from a ZIP into the config directory
func ImportSettings(zipPath string) error {
	configDir := ConfigDirPath()

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, zf := range r.File {
		name := filepath.Base(zf.Name)
		if !allowedFiles[name] {
			continue // skip unknown files
		}

		rc, err := zf.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", name, err)
		}

		dst := filepath.Join(configDir, name)
		out, err := os.Create(dst)
		if err != nil {
			rc.Close()
			return fmt.Errorf("create %s: %w", name, err)
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}
