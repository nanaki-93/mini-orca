package agentstate

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestJournalReadRetriesAtomicReplacement(t *testing.T) {
	root := t.TempDir()
	path, replacement := filepath.Join(root, "journal.json"), filepath.Join(root, "replacement.json")
	if err := os.WriteFile(path, []byte(`{"snapshot":"old"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte(`{"snapshot":"new"}`), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	data, err := readJournalFileOpening(path, func(name string) (*os.File, error) {
		calls++
		if calls == 1 {
			// Atomic writers may replace the inode after Lstat and before Open.
			if err := os.Rename(replacement, path); err != nil {
				return nil, err
			}
		}
		return os.Open(name)
	})
	if err != nil || string(data) != `{"snapshot":"new"}` || calls != 2 {
		t.Fatalf("valid atomic replacement was rejected: data=%q opens=%d error=%v", data, calls, err)
	}
}

func TestJournalReadReplacementRetainsPrivateFileChecks(t *testing.T) {
	for _, kind := range []string{"symlink", "public mode"} {
		t.Run(kind, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("Unix permissions and unprivileged symlinks")
			}
			root := t.TempDir()
			path, replacement := filepath.Join(root, "journal.json"), filepath.Join(root, "replacement.json")
			if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(replacement, []byte(`{"private":"outside"}`), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			data, err := readJournalFileOpening(path, func(name string) (*os.File, error) {
				calls++
				if calls == 1 {
					if kind == "symlink" {
						if err := os.Remove(path); err != nil {
							return nil, err
						}
						if err := os.Symlink(replacement, path); err != nil {
							return nil, err
						}
					} else {
						if err := os.Chmod(replacement, 0644); err != nil {
							return nil, err
						}
						if err := os.Rename(replacement, path); err != nil {
							return nil, err
						}
					}
				}
				return os.Open(name)
			})
			if !errors.Is(err, ErrCorrupt) || len(data) != 0 {
				t.Fatalf("unsafe replacement was read: %q %v", data, err)
			}
		})
	}
}

func TestJournalReadBoundsRepeatedReplacement(t *testing.T) {
	root := t.TempDir()
	path, replacement := filepath.Join(root, "journal.json"), filepath.Join(root, "replacement.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	data, err := readJournalFileOpening(path, func(name string) (*os.File, error) {
		calls++
		if err := os.WriteFile(replacement, []byte(`{}`), 0600); err != nil {
			return nil, err
		}
		if err := os.Rename(replacement, path); err != nil {
			return nil, err
		}
		return os.Open(name)
	})
	if err == nil || calls != 3 || len(data) != 0 {
		t.Fatalf("unstable read was unbounded or successful: opens=%d data=%q error=%v", calls, data, err)
	}
}
