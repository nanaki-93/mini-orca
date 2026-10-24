package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

var ErrArtifactNotFound = agentstate.ErrNotFound
var ErrArtifactConflict = agentstate.ErrArtifactConflict
var ErrArtifactProtected = agentstate.ErrArtifactInUse
var ErrArtifactUnavailable = errors.New("artifact library unavailable")

// AgentArtifact exposes host provenance, never arbitrary SDK custom metadata or
// payload text. Revision freshness is distinct from source revalidation.
type AgentArtifact struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Version         int64             `json:"version"`
	Hash            string            `json:"hash"`
	Kind            string            `json:"kind"`
	MIME            string            `json:"mime"`
	DisplayName     string            `json:"display_name,omitempty"`
	Size            int               `json:"size"`
	Created         time.Time         `json:"created"`
	ProjectID       string            `json:"project_id"`
	ProjectRevision string            `json:"project_revision"`
	Owner           agentstate.Key    `json:"owner"`
	Ownership       string            `json:"ownership"`
	Provenance      map[string]string `json:"provenance"`
	RevisionStatus  string            `json:"revision_status"`
	SourceStatus    string            `json:"source_status"`
	Protected       bool              `json:"protected"`
	InUse           bool              `json:"in_use"`
	CanDelete       bool              `json:"can_delete"`
	URI             string            `json:"uri"`
}

type ArtifactList struct {
	ProjectID       string          `json:"project_id"`
	ProjectRevision string          `json:"project_revision"`
	Snapshot        string          `json:"snapshot"`
	Artifacts       []AgentArtifact `json:"artifacts"`
	NextCursor      string          `json:"next_cursor,omitempty"`
}

type ArtifactDetail struct {
	AgentArtifact
	PreviewKind string `json:"preview_kind"`
	Text        string `json:"text,omitempty"`
	Truncated   bool   `json:"truncated"`
	ReportID    string `json:"report_id,omitempty"`
}

type ArtifactControl struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	Action          string `json:"action"`
	Version         int64  `json:"version"`
	Hash            string `json:"hash"`
}

type ArtifactReceipt struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	ID              string `json:"id"`
	Version         int64  `json:"version"`
	Hash            string `json:"hash"`
	State           string `json:"state"`
	Path            string `json:"path,omitempty"`
}

func artifactLibraryError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, project.ErrRevisionConflict) {
		return err
	}
	if errors.Is(err, ErrArtifactNotFound) || errors.Is(err, ErrArtifactConflict) || errors.Is(err, ErrArtifactProtected) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrArtifactUnavailable, err)
}

func (s *Service) artifactLibraryScope(ctx context.Context, query AgentRunQuery, root string) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return s.agentHistoryScope(query, root)
}

func (s *Service) artifactInventory(ctx context.Context, query AgentRunQuery, root string) (*agentstate.Store, []agentstate.ArtifactInventoryEntry, error) {
	if err := s.artifactLibraryScope(ctx, query, root); err != nil {
		return nil, nil, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, nil, artifactLibraryError(err)
	}
	entries, err := store.ArtifactInventory(ctx)
	if err != nil {
		return nil, nil, artifactLibraryError(err)
	}
	return store, entries, s.artifactLibraryScope(ctx, query, root)
}

func artifactLibraryID(record agentstate.Artifact) string {
	key := record.Key
	digest := sha256.Sum256([]byte(strings.Join([]string{key.Session.App, key.Session.User, key.Session.ID, key.Name, strconv.FormatInt(record.Version, 10)}, "\x00")))
	return hex.EncodeToString(digest[:])
}

func artifactOwnership(record agentstate.Artifact, projectID string) (string, string) {
	if recorded := record.Provenance["project"]; recorded != "" {
		if recorded == projectID {
			return "project", record.Provenance["revision"]
		}
		return "", ""
	}
	key := record.Key.Session
	if key.App == "mini-orca" && key.User == "local" && key.ID == "research_"+projectID && record.Metadata["kind"] == "grounded_research_v1" && record.Metadata["project_id"] == projectID {
		return "research", record.Metadata["project_revision"]
	}
	return "", ""
}

func artifactLibraryView(store *agentstate.Store, entry agentstate.ArtifactInventoryEntry, query AgentRunQuery) (AgentArtifact, bool) {
	record := entry.Artifact
	owner, revision := artifactOwnership(record, query.ProjectID)
	if owner == "" {
		return AgentArtifact{}, false
	}
	status := "stale"
	if revision == "" {
		status = "unknown"
	} else if revision == query.ProjectRevision {
		status = "current"
	}
	provenance := map[string]string{}
	for _, key := range []string{"project", "revision", "policy", "configuration", "run", "proposal", "workspace"} {
		if value := record.Provenance[key]; value != "" {
			provenance[key] = value
		}
	}
	return AgentArtifact{ID: artifactLibraryID(record), Name: record.Key.Name, Version: record.Version, Hash: record.Hash, Kind: record.Kind, MIME: record.MIME, DisplayName: record.DisplayName, Size: record.Size, Created: record.Created, ProjectID: query.ProjectID, ProjectRevision: revision, Owner: record.Key.Session, Ownership: owner, Provenance: provenance, RevisionStatus: status, SourceStatus: "unchecked", Protected: record.Protected, InUse: entry.InUse, CanDelete: !record.Protected && !entry.InUse, URI: store.ArtifactURI(record)}, true
}

type artifactListCursor struct {
	Project, Revision, Snapshot, After string
}

func artifactCursorOffset(query AgentRunQuery, snapshot string, records []AgentArtifact) (int, error) {
	if query.Cursor == "" {
		return 0, nil
	}
	if len(query.Cursor) > 2048 {
		return 0, fmt.Errorf("invalid artifact catalog cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(query.Cursor)
	var cursor artifactListCursor
	if err != nil || json.Unmarshal(data, &cursor) != nil || cursor.After == "" {
		return 0, fmt.Errorf("invalid artifact catalog cursor")
	}
	if cursor.Project != query.ProjectID || cursor.Revision != query.ProjectRevision || cursor.Snapshot != snapshot {
		return 0, ErrArtifactConflict
	}
	for i, record := range records {
		if record.ID == cursor.After {
			return i + 1, nil
		}
	}
	return 0, ErrArtifactConflict
}

func (s *Service) ListArtifacts(ctx context.Context, query AgentRunQuery) (*ArtifactList, error) {
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 {
		return nil, fmt.Errorf("artifact limit must be between 1 and 100")
	}
	root := s.manager.Root()
	store, entries, err := s.artifactInventory(ctx, query, root)
	if err != nil {
		return nil, err
	}
	records := []AgentArtifact{}
	for _, entry := range entries {
		if record, owned := artifactLibraryView(store, entry, query); owned {
			records = append(records, record)
		}
	}
	slices.SortFunc(records, func(a, b AgentArtifact) int { return strings.Compare(a.ID, b.ID) })
	data, err := json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("encode artifact snapshot: %w", err)
	}
	digest := sha256.Sum256(data)
	snapshot := hex.EncodeToString(digest[:])
	offset, err := artifactCursorOffset(query, snapshot, records)
	if err != nil {
		return nil, err
	}
	end := min(offset+query.Limit, len(records))
	result := &ArtifactList{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Snapshot: snapshot, Artifacts: records[offset:end]}
	if end < len(records) {
		data, err := json.Marshal(artifactListCursor{Project: query.ProjectID, Revision: query.ProjectRevision, Snapshot: snapshot, After: records[end-1].ID})
		if err != nil {
			return nil, fmt.Errorf("encode artifact cursor: %w", err)
		}
		result.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	return result, s.artifactLibraryScope(ctx, query, root)
}

func selectLibraryArtifact(store *agentstate.Store, entries []agentstate.ArtifactInventoryEntry, query AgentRunQuery, id string) (AgentArtifact, agentstate.ArtifactKey, error) {
	if data, err := hex.DecodeString(id); err != nil || len(data) != sha256.Size {
		return AgentArtifact{}, agentstate.ArtifactKey{}, fmt.Errorf("invalid artifact ID")
	}
	for _, entry := range entries {
		if artifactLibraryID(entry.Artifact) != id {
			continue
		}
		if value, owned := artifactLibraryView(store, entry, query); owned {
			return value, entry.Key, nil
		}
	}
	return AgentArtifact{}, agentstate.ArtifactKey{}, ErrArtifactNotFound
}

func (s *Service) ArtifactDetail(ctx context.Context, query AgentRunQuery, id string) (*ArtifactDetail, error) {
	root := s.manager.Root()
	store, entries, err := s.artifactInventory(ctx, query, root)
	if err != nil {
		return nil, err
	}
	view, key, err := selectLibraryArtifact(store, entries, query, id)
	if err != nil {
		return nil, err
	}
	record, err := store.ExportArtifact(ctx, key, view.Version)
	if err != nil {
		return nil, artifactLibraryError(err)
	}
	if record.Hash != view.Hash {
		return nil, ErrArtifactConflict
	}
	result := artifactPreview(view, record.Data)
	return &result, s.artifactLibraryScope(ctx, query, root)
}

func artifactPreview(view AgentArtifact, data []byte) ArtifactDetail {
	result := ArtifactDetail{AgentArtifact: view, PreviewKind: "binary"}
	if view.Ownership == "research" {
		result.PreviewKind, result.ReportID = "research_report", strings.TrimSuffix(view.Name, ".json")
		return result
	}
	media, _, _ := mime.ParseMediaType(view.MIME) // The store has already validated the media type.
	if !utf8.Valid(data) || view.Kind != "text" && !strings.HasPrefix(media, "text/") && media != "application/json" {
		return result
	}
	result.PreviewKind = "text"
	if len(data) > 64<<10 {
		data, result.Truncated = data[:64<<10], true
		for !utf8.Valid(data) {
			data = data[:len(data)-1]
		}
	}
	result.Text = string(data)
	return result
}

func (s *Service) ControlArtifact(ctx context.Context, id string, request ArtifactControl) (*ArtifactReceipt, error) {
	if request.Action != "export" && request.Action != "delete" || request.Version < 1 || len(request.Hash) != 64 {
		return nil, fmt.Errorf("export/delete require exact artifact version and hash")
	}
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	root := s.manager.Root()
	store, entries, err := s.artifactInventory(ctx, query, root)
	if err != nil {
		return nil, err
	}
	view, key, err := selectLibraryArtifact(store, entries, query, id)
	if err != nil {
		return nil, err
	}
	if view.Version != request.Version || view.Hash != request.Hash {
		return nil, ErrArtifactConflict
	}
	guard := func() error { return s.artifactLibraryScope(ctx, query, root) }
	result := &ArtifactReceipt{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, ID: id, Version: view.Version, Hash: view.Hash, State: "deleted"}
	if request.Action == "export" {
		result.State = "exported"
		result.Path, err = store.ExportArtifactFile(ctx, key, view.Version, view.Hash, guard)
	} else {
		err = store.DeleteArtifactVersion(ctx, key, view.Version, view.Hash, guard)
	}
	if err != nil {
		return nil, artifactLibraryError(err)
	}
	return result, nil
}
