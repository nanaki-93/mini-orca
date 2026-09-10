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

func TestWriteFileAuthorizedRejectsAfterTempSyncWithoutReplacing(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "record.json")
	if err := os.WriteFile(path, []byte("prior"), 0600); err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("authorization rejected")
	called := false
	err := WriteFileAuthorized(path, []byte("next"), 0600, func() error {
		called = true
		temporaries, globErr := filepath.Glob(filepath.Join(directory, ".metadata-*.tmp"))
		if globErr != nil || len(temporaries) != 1 {
			t.Fatalf("temporary metadata = %v, %v", temporaries, globErr)
		}
		data, readErr := os.ReadFile(temporaries[0])
		if readErr != nil || string(data) != "next" {
			t.Fatalf("temporary content = %q, %v", data, readErr)
		}
		return rejected
	})
	if !called || !errors.Is(err, rejected) {
		t.Fatalf("authorization result = %v, called = %t", err, called)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "prior" {
		t.Fatalf("metadata after rejected authorization = %q, %v", data, readErr)
	}
	temporaries, globErr := filepath.Glob(filepath.Join(directory, ".metadata-*.tmp"))
	if globErr != nil || len(temporaries) != 0 {
		t.Fatalf("temporary metadata after rejection = %v, %v", temporaries, globErr)
	}
}

func TestWriteFileRetainsPriorDataAndClosesTempOnOperationFailures(t *testing.T) {
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
			name: "chmod",
			configure: func(operations *atomicFileOperations, failure error) {
				operations.chmod = func(*os.File, os.FileMode) error { return failure }
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
			path := filepath.Join(directory, "record.json")
			if err := os.WriteFile(path, []byte("prior"), 0600); err != nil {
				t.Fatal(err)
			}
			failure := errors.New(test.name + " failed")
			operations := defaultAtomicFileOperations()
			test.configure(&operations, failure)
			var temporary *os.File
			write := operations.write
			operations.write = func(file *os.File, data []byte) (int, error) {
				temporary = file
				return write(file, data)
			}
			err := writeFile(path, []byte("next"), 0600, operations)
			if !errors.Is(err, failure) {
				t.Fatalf("write error = %v", err)
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil || string(data) != "prior" {
				t.Fatalf("prior metadata = %q, %v", data, readErr)
			}
			if temporary == nil {
				t.Fatal("no temporary file was opened")
			}
			if _, err := temporary.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("temporary file was not closed: %v", err)
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
