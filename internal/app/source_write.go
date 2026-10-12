package app

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

func atomicWrite(path string, content []byte) error {
	info, err := os.Stat(path)
	mode := os.FileMode(0600)
	if err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".mini-orca-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err == nil {
		defer directory.Close()
		if err := directory.Sync(); err != nil {
			return &sourceWriteError{err}
		}
	} else {
		return &sourceWriteError{err}
	}
	return nil
}

type sourceWriteError struct{ cause error }

func (err *sourceWriteError) Error() string {
	return "source replaced; directory sync failed: " + err.cause.Error()
}
func (err *sourceWriteError) Unwrap() error { return err.cause }

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}
