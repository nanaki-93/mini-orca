package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicallyReplacesPrivateMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "record.json")
	if err := WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "second" {
		t.Fatalf("stored metadata = %q, %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("metadata permissions = %v, %v", info.Mode().Perm(), err)
	}
}

func TestWriteFileKeepsPriorDataAndCleansTempWhenReplacementFails(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "record.json")
	if err := os.WriteFile(path, []byte("prior"), 0600); err != nil {
		t.Fatal(err)
	}
	replaceErr := errors.New("replace failed")
	operations := defaultAtomicFileOperations()
	operations.replace = func(string, string) error { return replaceErr }
	err := writeFile(path, []byte("next"), 0600, operations)
	if !errors.Is(err, replaceErr) {
		t.Fatalf("replacement error = %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "prior" {
		t.Fatalf("prior metadata = %q, %v", data, readErr)
	}
	temporaries, globErr := filepath.Glob(filepath.Join(directory, ".metadata-*.tmp"))
	if globErr != nil || len(temporaries) != 0 {
		t.Fatalf("temporary metadata = %v, %v", temporaries, globErr)
	}
}

func TestWriteFileCleansTempOnWriteSyncAndCloseFailures(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*atomicFileOperations, error)
	}{
		{
			name: "write",
			configure: func(operations *atomicFileOperations, failure error) {
				operations.write = func(*os.File, []byte) (int, error) { return 0, failure }
			},
		},
		{
			name: "sync",
			configure: func(operations *atomicFileOperations, failure error) {
				operations.sync = func(*os.File) error { return failure }
			},
		},
		{
			name: "close",
			configure: func(operations *atomicFileOperations, failure error) {
				operations.close = func(file *os.File) error {
					_ = file.Close()
					return failure
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			failure := errors.New(test.name + " failed")
			operations := defaultAtomicFileOperations()
			test.configure(&operations, failure)
			err := writeFile(filepath.Join(directory, "record.json"), []byte("next"), 0600, operations)
			if !errors.Is(err, failure) {
				t.Fatalf("write error = %v", err)
			}
			temporaries, globErr := filepath.Glob(filepath.Join(directory, ".metadata-*.tmp"))
			if globErr != nil || len(temporaries) != 0 {
				t.Fatalf("temporary metadata = %v, %v", temporaries, globErr)
			}
		})
	}
}

func TestRecoverCorruptUsesEstablishedSuffix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record.json")
	if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RecoverCorrupt(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("corrupt metadata still at original path: %v", err)
	}
	backups, err := filepath.Glob(path + ".corrupt-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("corrupt backup = %v, %v", backups, err)
	}
}
