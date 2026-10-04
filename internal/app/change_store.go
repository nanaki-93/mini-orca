package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

var changeIDPattern = regexp.MustCompile(`^change-[a-f0-9]{32}$`)

// changeMetadataPath disallows metadata-directory aliases before either reads
// or writes. Source-write path policy deliberately does not admit metadata.
func changeMetadataPath(root, name string) (string, error) {
	current := root
	for _, part := range []string{".mini-orca", "changes", name} {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("change history contains a symlink")
		}
	}
	return current, nil
}

func readChangeSession(root, id string) (*ChangeSession, error) {
	if !changeIDPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid change session id")
	}
	path, err := changeMetadataPath(root, id+".json")
	if err != nil {
		return nil, err
	}
	input, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read change session: %w", err)
	}
	defer input.Close()
	if err := boundedChangeFile(input); err != nil {
		return nil, err
	}
	var session ChangeSession
	decoder := json.NewDecoder(io.LimitReader(input, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&session); err != nil {
		return nil, fmt.Errorf("decode change history: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("change history has trailing data or exceeds its limit")
	}
	if session.SchemaVersion != 1 || session.ID != id || len(session.Targets) == 0 || len(session.Targets) > maxChangePaths || len(session.Messages) > maxChangeMessages {
		return nil, fmt.Errorf("unsupported or corrupt change history")
	}
	if err := validateStoredChange(session); err != nil {
		return nil, err
	}
	return &session, nil
}

func validateStoredChange(session ChangeSession) error {
	seen := map[string]bool{}
	for _, target := range session.Targets {
		if seen[target.Path] || len(target.Content) > maxChangeBytes || target.Hash != contentHash([]byte(target.Content)) && target.Exists {
			return fmt.Errorf("corrupt captured change target")
		}
		seen[target.Path] = true
	}
	if err := validateStoredEdits(session.Changes, seen); err != nil {
		return err
	}
	if session.Hash != changeProposalHash(session.Changes) {
		return fmt.Errorf("corrupt proposal identity")
	}
	for _, pinned := range session.PinnedTests {
		found := false
		for _, edit := range session.Changes {
			if edit.Path == pinned.Path && edit.Hash == pinned.Hash {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("proposal dropped or changed a pinned regression test")
		}
	}
	return nil
}

func validateStoredEdits(edits []ChangeEdit, targets map[string]bool) error {
	seen := map[string]bool{}
	for _, edit := range edits {
		if !targets[edit.Path] || seen[edit.Path] || len(edit.Content) > maxChangeBytes || edit.Hash != contentHash([]byte(edit.Content)) {
			return fmt.Errorf("corrupt stored proposal")
		}
		seen[edit.Path] = true
	}
	return nil
}

func boundedChangeFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 2*1024*1024 {
		return fmt.Errorf("change metadata exceeds 2 MiB")
	}
	return nil
}

func writeChangeSession(root string, session *ChangeSession) error {
	if err := validateStoredChange(*session); err != nil {
		return err
	}
	path, err := changeMetadataPath(root, session.ID+".json")
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 2*1024*1024 {
		return fmt.Errorf("change history exceeds 2 MiB; start another conversation")
	}
	return storage.WriteFile(path, data, 0600)
}

func listChangeSessions(root string) ([]ChangeHistoryEntry, error) {
	path, err := changeMetadataPath(root, "")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return []ChangeHistoryEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := []ChangeHistoryEntry{}
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !changeIDPattern.MatchString(id) {
			continue
		}
		session, err := readChangeSession(root, id)
		if err != nil {
			return nil, err
		}
		result = append(result, ChangeHistoryEntry{ID: session.ID, ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Kind: session.Kind, Title: session.Title, Revision: session.Revision, Hash: session.Hash, State: session.State, Freshness: session.Freshness, UpdatedAt: session.UpdatedAt})
		if len(result) > 200 {
			return nil, fmt.Errorf("change history exceeds 200 sessions")
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result, nil
}
