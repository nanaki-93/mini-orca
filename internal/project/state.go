package project

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
)

var (
	ErrNoActiveProject  = errors.New("active project has not been analyzed")
	ErrRevisionConflict = errors.New("project or file state changed")
	ErrExcludedFile     = errors.New("file is excluded by context policy")
	ErrUnsupportedFile  = errors.New("file does not support symbol extraction")
)

func projectID(root string) string {
	sum := sha256.Sum256([]byte(root))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func projectRevision(root string) (string, error) {
	policy, err := NewContextPolicy(root)
	if err != nil {
		return "", err
	}
	files, err := listFiles(root)
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	hash := sha256.New()
	_, _ = io.WriteString(hash, root+"\n")
	for _, relative := range files {
		if !policy.Decide(relative).Include {
			continue
		}
		fullPath, err := ResolveFile(root, relative)
		if err != nil {
			continue
		}
		fileHash, err := hashFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("hash %s: %w", relative, err)
		}
		_, _ = io.WriteString(hash, relative+"\x00"+fileHash+"\n")
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
