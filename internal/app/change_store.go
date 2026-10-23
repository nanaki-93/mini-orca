package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/app/changes"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

var changeIDPattern = regexp.MustCompile(`^change-[a-f0-9]{32}$`)

// changeMetadataPath disallows metadata-directory aliases before either reads
// or writes. Source-write path policy deliberately does not admit metadata.
func changeMetadataPath(root, name string) (string, error) {
	return changes.MetadataPath(root, name)
}

func loadChangeSession(root, id string) (*ChangeSession, error) {
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
	if err := validateStoredPlan(&session); err != nil {
		return err
	}
	if err := validateChangeFindingTargets(&session); err != nil {
		return err
	}
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
	if err := validateStoredFileReview(&session, seen); err != nil {
		return err
	}
	if session.Hash != changeProposalHash(session.Changes) {
		return fmt.Errorf("corrupt proposal identity")
	}
	if err := validatePinnedChangeTests(&session); err != nil {
		return err
	}
	return validateStoredWorkflow(session.Workflow)
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

func storeChangeSession(root string, session *ChangeSession) error {
	refreshChangeReviewDiff(session)
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
	if err := storeChangeArtifacts(root, session); err != nil {
		return err
	}
	return storage.WriteFile(path, data, 0600)
}

func storeChangeArtifacts(root string, session *ChangeSession) error {
	if len(session.Changes) == 0 {
		return nil
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return fmt.Errorf("%w: open change evidence: %w", agentstate.ErrPersistence, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	provenance := map[string]string{"project": session.ProjectID, "revision": session.ProjectRevision, "proposal": session.Hash, "workspace": session.WorkspaceHash, "policy": session.PolicyFingerprint}
	if err := saveChangeArtifact(ctx, store, session.ID, "proposal.json", session.Changes, provenance); err != nil {
		return err
	}
	if len(session.Checks) == 0 {
		return nil
	}
	return saveChangeArtifact(ctx, store, session.ID, "checks.json", struct {
		Options CheckOptions  `json:"options"`
		Checks  []CheckResult `json:"checks"`
	}{session.CheckOptions, session.Checks}, provenance)
}

func saveChangeArtifact(ctx context.Context, store *agentstate.Store, id, name string, value any, provenance map[string]string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode change evidence: %w", err)
	}
	_, err = store.SaveArtifact(ctx, agentstate.ArtifactWrite{
		Key:  agentstate.ArtifactKey{Session: agentstate.Key{App: "mini-orca-changes", User: "local", ID: id}, Name: name},
		Kind: "inline", MIME: "application/json", Data: data, Provenance: provenance,
		Protected: true, Deduplicate: true,
	})
	if err != nil {
		return fmt.Errorf("%w: save change evidence: %w", agentstate.ErrPersistence, err)
	}
	return nil
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
		session, err := loadChangeSession(root, id)
		if err != nil {
			return nil, err
		}
		entry := ChangeHistoryEntry{ID: session.ID, ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Kind: session.Kind, Title: session.Title, Revision: session.Revision, Hash: session.Hash, State: session.State, Freshness: session.Freshness, UpdatedAt: session.UpdatedAt}
		entry.Commit = session.Commit
		entry.FindingTargets = session.FindingTargets
		if session.Workflow != nil {
			entry.WorkflowStatus = session.Workflow.Status
			entry.WorkflowReason = session.Workflow.Reason
		}
		result = append(result, entry)
		if len(result) > 200 {
			return nil, fmt.Errorf("change history exceeds 200 sessions")
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result, nil
}

func validatePinnedChangeTests(session *ChangeSession) error {
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
