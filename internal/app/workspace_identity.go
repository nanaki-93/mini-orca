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
	workspace, err := os.MkdirTemp("", "mini-orca-workspace-fingerprint-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workspace)
	if err := copyCheckWorkspace(ctx, root, workspace); err != nil {
		return "", err
	}
	return fingerprintCheckWorkspace(ctx, workspace)
}
func fingerprintCheckWorkspace(ctx context.Context, root string) (string, error) {
	hash := sha256.New()
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
		if _, err := io.WriteString(hash, filepath.ToSlash(relative)+"\x00"+strconv.FormatInt(info.Size(), 10)+"\x00"); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := validateCheckWorkspaceFile(path, info, input); err != nil {
			_ = input.Close()
			return err
		}
		copied, copyErr := copyCheckWorkspaceFile(ctx, hash, input, make([]byte, checkCopyBufferSize), checkLimits.maxWorkspaceBytes-bytes)
		postReadErr := validateCheckWorkspaceFile(path, info, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if postReadErr != nil {
			return postReadErr
		}
		if closeErr != nil {
			return closeErr
		}
		if copied != info.Size() {
			return fmt.Errorf("check workspace changed while hashing")
		}
		bytes += copied
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
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
