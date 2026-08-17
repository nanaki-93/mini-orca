package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	ErrNoActiveProject  = errors.New("active project has not been analyzed")
	ErrRevisionConflict = errors.New("project or file state changed")
	ErrExcludedFile     = errors.New("file is excluded by context policy")
	ErrUnsupportedFile  = errors.New("file does not support symbol extraction")
)

// Activity is durable project-scoped metadata. It intentionally excludes model
// output and source content.
type Activity struct {
	Role            string    `json:"role"`
	Content         string    `json:"content"`
	Phase           string    `json:"phase"`
	Timestamp       time.Time `json:"timestamp"`
	ProjectID       string    `json:"project_id"`
	ProjectRevision string    `json:"project_revision"`
	TargetFile      string    `json:"target_file,omitempty"`
	TargetSymbol    string    `json:"target_symbol,omitempty"`
}

type sessionStore struct {
	mu         sync.Mutex
	path       string
	projectID  string
	revision   string
	activities []Activity
}

func newSessionStore(root, projectID, revision string) (*sessionStore, error) {
	store := &sessionStore{
		path:      filepath.Join(root, ".mini-orca", "sessions", "activity.json"),
		projectID: projectID,
		revision:  revision,
	}
	data, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project activity: %w", err)
	}
	if err := json.Unmarshal(data, &store.activities); err != nil {
		corrupt := store.path + ".corrupt-" + time.Now().UTC().Format("20060102T150405.000000000")
		if renameErr := os.Rename(store.path, corrupt); renameErr != nil {
			return nil, fmt.Errorf("recover corrupt project activity: %w", renameErr)
		}
		store.activities = nil
	}
	return store, nil
}

func (s *sessionStore) append(activity Activity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	activity.ProjectID = s.projectID
	activity.ProjectRevision = s.revision
	if activity.Timestamp.IsZero() {
		activity.Timestamp = time.Now().UTC()
	} else {
		activity.Timestamp = activity.Timestamp.UTC()
	}
	s.activities = append(s.activities, activity)
	return s.writeLocked()
}

func (s *sessionStore) history() []Activity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Activity(nil), s.activities...)
}

func (s *sessionStore) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}
	data, err := json.MarshalIndent(s.activities, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project activity: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".activity-*.tmp")
	if err != nil {
		return fmt.Errorf("create activity temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write activity temp file: %w", err)
	}
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("set activity permissions: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync activity temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close activity temp file: %w", err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return fmt.Errorf("replace project activity: %w", err)
	}
	return nil
}

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
