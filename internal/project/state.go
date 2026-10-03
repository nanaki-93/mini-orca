package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrNoActiveProject          = errors.New("active project has not been analyzed")
	ErrRevisionConflict         = errors.New("project or file state changed")
	ErrExcludedFile             = errors.New("file is excluded by context policy")
	ErrUnsupportedFile          = errors.New("file does not support symbol extraction")
	ErrSecurityRulesUnavailable = errors.New("security rules are unavailable for this file")
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
	files, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{
		Root: root, IgnoredDirectories: ignoredProjectDirectories, IncludeSymlinkFiles: true, MaxFiles: maxProjectFiles,
	})
	if err != nil {
		return "", err
	}
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
	return hashFileContext(context.Background(), path, make([]byte, 32*1024), 0)
}

func hashFileContext(ctx context.Context, path string, buffer []byte, limit int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("source is not a regular file")
	}
	if limit > 0 && info.Size() > limit {
		return "", ErrRevisionConflict
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := file.Read(buffer)
		size += int64(n)
		if limit > 0 && size > limit {
			return "", ErrRevisionConflict
		}
		_, _ = hash.Write(buffer[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// VerifyIndexedSources checks every captured source hash with one reusable read
// buffer. It does not materialize source text or trust size/mtime as identity.
func VerifyIndexedSources(ctx context.Context, root string, files []IndexFile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return err
	}
	buffer := make([]byte, 32*1024)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if file.Binary || file.SizeBytes > maxFileViewBytes {
			return ErrRevisionConflict
		}
		path, err := resolveFile(canonical, file.Path)
		if err != nil {
			return err
		}
		hash, err := hashFileContext(ctx, path, buffer, maxFileViewBytes)
		if err != nil {
			return err
		}
		if hash != file.ContentHash {
			return ErrRevisionConflict
		}
	}
	return ctx.Err()
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
