package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// CopyWorkspace intentionally keeps its own traversal: a temporary Go
// check workspace needs every regular project file (except metadata and known
// caches), whereas project.WalkProjectFiles returns only source-policy
// candidates. destination must be a temporary workspace owned by the caller.
func CopyWorkspace(ctx context.Context, source, destination string) error {
	copier := checkWorkspaceCopier{
		ctx: ctx, source: source, destination: destination,
		buffer: make([]byte, checkCopyBufferSize),
	}
	if err := copier.copy(); err != nil {
		if cleanupErr := os.RemoveAll(destination); cleanupErr != nil {
			return fmt.Errorf("%w; clean failed check workspace: %v", err, cleanupErr)
		}
		return err
	}
	return nil
}

type checkWorkspaceCopier struct {
	ctx         context.Context
	source      string
	destination string
	files       int
	bytes       int64
	buffer      []byte
}

func (copier *checkWorkspaceCopier) copy() error {
	return filepath.WalkDir(copier.source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := copier.ctx.Err(); err != nil {
			return fmt.Errorf("copy check workspace: %w", err)
		}
		relative, err := filepath.Rel(copier.source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		return copier.copyEntry(path, relative, entry)
	})
}

func (copier *checkWorkspaceCopier) copyEntry(path, relative string, entry os.DirEntry) error {
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
		return os.MkdirAll(filepath.Join(copier.destination, relative), 0700)
	}
	if !isRegularCheckWorkspaceFile(info) {
		return nil
	}
	if copier.files == checkLimits.maxWorkspaceFiles {
		return fmt.Errorf("check workspace exceeds file limit of %d files", checkLimits.maxWorkspaceFiles)
	}
	if info.Size() > checkLimits.maxWorkspaceBytes-copier.bytes {
		return fmt.Errorf("check workspace exceeds byte limit of %d bytes", checkLimits.maxWorkspaceBytes)
	}
	return copier.copyFile(path, relative, info)
}

func (copier *checkWorkspaceCopier) copyFile(path, relative string, info os.FileInfo) error {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := validateCheckWorkspaceFile(path, info, input); err != nil {
		_ = input.Close()
		return err
	}
	outputPath := filepath.Join(copier.destination, relative)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		_ = input.Close()
		return err
	}
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		_ = input.Close()
		return err
	}
	copied, copyErr := copyCheckWorkspaceFile(copier.ctx, output, input, copier.buffer, checkLimits.maxWorkspaceBytes-copier.bytes)
	closeErr := output.Close()
	inputErr := input.Close()
	if copyErr != nil {
		_ = os.Remove(outputPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(outputPath)
		return closeErr
	}
	if inputErr != nil {
		_ = os.Remove(outputPath)
		return inputErr
	}
	copier.files++
	copier.bytes += copied
	return nil
}

func isRegularCheckWorkspaceFile(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}

func validateCheckWorkspaceFile(path string, expected os.FileInfo, input *os.File) error {
	if !isRegularCheckWorkspaceFile(expected) {
		return fmt.Errorf("check workspace source %q is not a regular file", path)
	}
	opened, err := input.Stat()
	if err != nil {
		return fmt.Errorf("stat check workspace source %q: %w", path, err)
	}
	if !isRegularCheckWorkspaceFile(opened) || !os.SameFile(expected, opened) {
		return fmt.Errorf("check workspace source %q changed while opening", path)
	}
	current, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("recheck workspace source %q: %w", path, err)
	}
	if !isRegularCheckWorkspaceFile(current) || !os.SameFile(expected, current) {
		return fmt.Errorf("check workspace source %q changed while opening", path)
	}
	return nil
}

func copyCheckWorkspaceFile(ctx context.Context, output io.Writer, input io.Reader, buffer []byte, remaining int64) (int64, error) {
	var copied int64
	for {
		if err := ctx.Err(); err != nil {
			return copied, fmt.Errorf("copy check workspace: %w", err)
		}
		read, readErr := input.Read(buffer)
		if read > 0 {
			if int64(read) > remaining-copied {
				return copied, fmt.Errorf("check workspace exceeds byte limit of %d bytes", checkLimits.maxWorkspaceBytes)
			}
			written, writeErr := output.Write(buffer[:read])
			copied += int64(written)
			if writeErr != nil {
				return copied, writeErr
			}
			if written != read {
				return copied, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return copied, nil
		}
		if readErr != nil {
			return copied, readErr
		}
	}
}

func skipCheckWorkspaceDirectory(relative string) bool {
	switch filepath.Base(relative) {
	case "node_modules", ".gradle", ".kotlin", "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache", ".tox", "build", "dist", "target":
		return !checkWorkspaceFixturePath(relative)
	default:
		return false
	}
}

func isCheckWorkspaceMetadata(relative string) bool {
	switch filepath.Base(relative) {
	case ".mini-orca", ".worktrees", ".git", ".hg", ".svn":
		return true
	default:
		return false
	}
}

func checkWorkspaceFixturePath(relative string) bool {
	for _, part := range strings.FieldsFunc(filepath.ToSlash(relative), func(r rune) bool { return r == '/' }) {
		if part == "testdata" {
			return true
		}
	}
	return false
}

func WorkspaceFingerprint(ctx context.Context, root string) (string, error) {
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

func FingerprintWorkspace(ctx context.Context, root string) (string, error) {
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

func VerifyWorkspaceFingerprint(ctx context.Context, root, expectedFingerprint string) error {
	current, err := WorkspaceFingerprint(ctx, root)
	if err != nil {
		return err
	}
	if current != expectedFingerprint {
		return project.ErrRevisionConflict
	}
	return nil
}
