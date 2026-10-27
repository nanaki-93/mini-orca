package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

// writeToken is this launch's credential for requests that change state. The
// Web UI receives it in its document; other local clients read the private
// file, which only the daemon's user can open, and send it in a header. A web
// page in another origin can neither read the document nor the file.
type writeToken struct {
	value, file string
}

// newWriteToken creates the launch's token and saves it with mode 0600. When
// the file cannot be written, the token still guards the daemon and only the
// Web UI can send it.
func newWriteToken() (writeToken, error) {
	token := writeToken{value: rand.Text()}
	path, err := writeTokenPath()
	if err != nil {
		return token, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return token, fmt.Errorf("create the write token directory: %w", err)
	}
	if err := storage.WriteFileAuthorized(path, []byte(token.value+"\n"), 0o600, nil); err != nil {
		return token, fmt.Errorf("save the write token: %w", err)
	}
	token.file = path
	return token, nil
}

func writeTokenPath() (string, error) {
	if path := os.Getenv("MINI_ORCA_TOKEN_FILE"); path != "" {
		if !filepath.IsAbs(path) {
			return "", fmt.Errorf("MINI_ORCA_TOKEN_FILE must be an absolute path")
		}
		return path, nil
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the write token directory: %w", err)
	}
	return filepath.Join(directory, "mini-orca", "daemon-token"), nil
}
