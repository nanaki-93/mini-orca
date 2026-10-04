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
	for _, edit := range session.Changes {
		if !seen[edit.Path] || edit.Hash != contentHash([]byte(edit.Content)) {
			return fmt.Errorf("corrupt stored proposal")
		}
	}
	if session.Hash != changeProposalHash(session.Changes) {
		return fmt.Errorf("corrupt proposal identity")
	}
	return nil
}

func writeChangeSession(root string, session *ChangeSession) error {
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

func listChangeSessions(root string) ([]ChangeSession, error) {
	path, err := changeMetadataPath(root, "")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return []ChangeSession{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) > 200 {
		return nil, fmt.Errorf("change history exceeds 200 entries")
	}
	result := []ChangeSession{}
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !changeIDPattern.MatchString(id) {
			continue
		}
		session, err := readChangeSession(root, id)
		if err != nil {
			return nil, err
		}
		result = append(result, *session)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result, nil
}
