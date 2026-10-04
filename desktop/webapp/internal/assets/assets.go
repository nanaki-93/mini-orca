package assets

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:files
var bundled embed.FS

func Files() (fs.FS, error) {
	files, err := fs.Sub(bundled, "files")
	if err != nil {
		return nil, err
	}
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return nil, fmt.Errorf("web assets are missing; run make web-build: %w", err)
	}
	return files, nil
}
