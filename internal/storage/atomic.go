// Package storage provides small durable primitives for metadata owned by the
// application. Domain stores keep their own validation and serialization.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteFile replaces path with data through a same-directory temporary file.
// Metadata directories and files are private to the current user. The source
// file is synced and closed before it replaces an existing destination.
func WriteFile(path string, data []byte, mode os.FileMode) error {
	return writeFile(path, data, mode, defaultAtomicFileOperations())
}

type atomicFileOperations struct {
	write   func(*os.File, []byte) (int, error)
	chmod   func(*os.File, os.FileMode) error
	sync    func(*os.File) error
	close   func(*os.File) error
	replace func(string, string) error
}

func defaultAtomicFileOperations() atomicFileOperations {
	return atomicFileOperations{
		write:   func(file *os.File, data []byte) (int, error) { return file.Write(data) },
		chmod:   func(file *os.File, mode os.FileMode) error { return file.Chmod(mode) },
		sync:    func(file *os.File) error { return file.Sync() },
		close:   func(file *os.File) error { return file.Close() },
		replace: os.Rename,
	}
}

func writeFile(path string, data []byte, mode os.FileMode, operations atomicFileOperations) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create metadata directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".metadata-*.tmp")
	if err != nil {
		return fmt.Errorf("create metadata temp file: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		_ = os.Remove(temporaryPath)
	}()
	if _, err := operations.write(temporary, data); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}
	if err := operations.chmod(temporary, mode); err != nil {
		return fmt.Errorf("set metadata permissions: %w", err)
	}
	if err := operations.sync(temporary); err != nil {
		return fmt.Errorf("sync metadata: %w", err)
	}
	if err := operations.close(temporary); err != nil {
		closed = true
		return fmt.Errorf("close metadata: %w", err)
	}
	closed = true
	if err := operations.replace(temporaryPath, path); err != nil {
		return fmt.Errorf("replace metadata: %w", err)
	}
	return nil
}

// RecoverCorrupt preserves an unreadable metadata file using the established
// .corrupt-<UTC timestamp> suffix. Stores opt into recovery explicitly.
func RecoverCorrupt(path string) error {
	corrupt := path + ".corrupt-" + time.Now().UTC().Format("20060102T150405.000000000")
	if err := os.Rename(path, corrupt); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("recover corrupt metadata: %w", err)
	}
	return nil
}
