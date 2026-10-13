package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type workspaceIdentity struct {
	fingerprint string
}

func workspaceFingerprint(ctx context.Context, root string) (string, error) {
	hash := sha256.New()
	buffer := make([]byte, checkCopyBufferSize)
	var files int
	var bytes int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("copy check workspace: %w", err)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if isCheckWorkspaceMetadata(relative) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			if skipCheckWorkspaceDirectory(relative) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isRegularCheckWorkspaceFile(info) {
			return nil
		}
		if files == checkLimits.maxWorkspaceFiles {
			return fmt.Errorf("check workspace exceeds file limit of %d files", checkLimits.maxWorkspaceFiles)
		}
		if info.Size() > checkLimits.maxWorkspaceBytes-bytes {
			return fmt.Errorf("check workspace exceeds byte limit of %d bytes", checkLimits.maxWorkspaceBytes)
		}
		copied, err := hashCheckWorkspaceFile(ctx, hash, path, relative, info, buffer, checkLimits.maxWorkspaceBytes-bytes)
		if err != nil {
			return err
		}
		files++
		bytes += copied
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func fingerprintCheckWorkspace(ctx context.Context, root string) (string, error) {
	hash := sha256.New()
	buffer := make([]byte, checkCopyBufferSize)
	var bytes int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !isRegularCheckWorkspaceFile(info) {
			return fmt.Errorf("check workspace contains a non-regular file")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		copied, err := hashCheckWorkspaceFile(ctx, hash, path, relative, info, buffer, checkLimits.maxWorkspaceBytes-bytes)
		if err != nil {
			return err
		}
		bytes += copied
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func hashCheckWorkspaceFile(ctx context.Context, hash io.Writer, path, relative string, info os.FileInfo, buffer []byte, remaining int64) (int64, error) {
	if _, err := io.WriteString(hash, filepath.ToSlash(relative)+"\x00"+strconv.FormatInt(info.Size(), 10)+"\x00"); err != nil {
		return 0, err
	}
	input, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	if err := validateCheckWorkspaceFile(path, info, input); err != nil {
		_ = input.Close()
		return 0, err
	}
	copied, copyErr := copyCheckWorkspaceFile(ctx, hash, input, buffer, remaining)
	postReadErr := validateCheckWorkspaceFile(path, info, input)
	closeErr := input.Close()
	if copyErr != nil {
		return 0, copyErr
	}
	if postReadErr != nil {
		return 0, postReadErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if copied != info.Size() {
		return 0, fmt.Errorf("check workspace changed while hashing")
	}
	return copied, nil
}
func (fixture workspaceIdentity) verifyCurrent(ctx context.Context, root string) error {
	current, err := workspaceFingerprint(ctx, root)
	if err != nil {
		return err
	}
	if current != fixture.fingerprint {
		return project.ErrRevisionConflict
	}
	return nil
}
