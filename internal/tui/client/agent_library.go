package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var libraryHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validateLibraryArtifact(scope AgentSettingsScope, item *AgentArtifact) error {
	if !libraryHash.MatchString(item.ID) || !libraryHash.MatchString(item.Hash) || item.Version < 1 || item.Version > 1<<53 || item.ProjectID != scope.ProjectID {
		return fmt.Errorf("artifact identity or content hash is unavailable")
	}
	if err := validateArtifactMetadata(*item); err != nil {
		return err
	}
	status := "stale"
	if item.ProjectRevision == "" {
		status = "unknown"
	} else if item.ProjectRevision == scope.ProjectRevision {
		status = "current"
	}
	if item.RevisionStatus != status || item.SourceStatus != "unchecked" || !slices.Contains([]string{"text", "inline"}, item.Kind) {
		return fmt.Errorf("artifact freshness metadata is inconsistent")
	}
	item.CanDelete = item.CanDelete && !item.Protected && !item.InUse
	return nil
}

func validateArtifactMetadata(item AgentArtifact) error {
	if item.Size < 0 || item.Size > 4<<20 || item.Name == "" || item.Owner.App == "" || item.Owner.User == "" || item.Owner.ID == "" || !slices.Contains([]string{"project", "research"}, item.Ownership) {
		return fmt.Errorf("artifact ownership or size is unavailable")
	}
	return nil
}

func (c *Client) ReadArtifacts(ctx context.Context, requestID string, scope AgentSettingsScope, cursor string) (*ArtifactList, error) {
	target, err := externalTarget(scope, "artifacts", "", "")
	if err != nil {
		return nil, err
	}
	if len(cursor) > 2048 {
		return nil, fmt.Errorf("artifact continuation cursor exceeds its bound")
	}
	target += "&limit=100"
	if cursor != "" {
		target += "&cursor=" + url.QueryEscape(cursor)
	}
	var result ArtifactList
	if err := c.DoJSON(ctx, requestID, http.MethodGet, target, nil, &result); err != nil {
		return nil, err
	}
	if result.ProjectIdentity != scope.ProjectIdentity || !libraryHash.MatchString(result.Snapshot) || len(result.Artifacts) > 100 || len(result.NextCursor) > 2048 {
		return nil, fmt.Errorf("artifact page is unavailable or belongs to another snapshot")
	}
	seen := map[string]bool{}
	for i := range result.Artifacts {
		if err := validateLibraryArtifact(scope, &result.Artifacts[i]); err != nil {
			return nil, err
		}
		if seen[result.Artifacts[i].ID] {
			return nil, fmt.Errorf("artifact page repeats a version")
		}
		seen[result.Artifacts[i].ID] = true
	}
	return &result, nil
}

func (c *Client) ArtifactDetail(ctx context.Context, requestID string, scope AgentSettingsScope, id string) (*ArtifactDetail, error) {
	if !libraryHash.MatchString(id) {
		return nil, fmt.Errorf("select an exact artifact version")
	}
	var result ArtifactDetail
	if err := c.externalJSON(ctx, requestID, scope, "artifacts", id, "", nil, &result); err != nil {
		return nil, err
	}
	if err := validateLibraryArtifact(scope, &result.AgentArtifact); err != nil {
		return nil, err
	}
	if result.ID != id {
		return nil, fmt.Errorf("artifact detail belongs to another version")
	}
	if err := validateArtifactPreview(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func validateArtifactPreview(result ArtifactDetail) error {
	if len(result.Text) > 64<<10 || !utf8.ValidString(result.Text) {
		return fmt.Errorf("artifact text exceeds its preview bound")
	}
	switch result.PreviewKind {
	case "text":
		if result.Ownership == "research" || result.ReportID != "" {
			return fmt.Errorf("research requires its cited report")
		}
		if !result.Truncated {
			hash := sha256.Sum256([]byte(result.Text))
			if hex.EncodeToString(hash[:]) != result.Hash || len(result.Text) != result.Size {
				return fmt.Errorf("artifact preview does not match its retained hash")
			}
		}
	case "binary", "research_report":
		if result.Text != "" || result.Truncated || (result.PreviewKind == "research_report") != (result.Ownership == "research" && externalRecordID.MatchString(result.ReportID)) {
			return fmt.Errorf("artifact disclosure metadata is inconsistent")
		}
	default:
		return fmt.Errorf("unsupported artifact preview")
	}
	return nil
}

func (c *Client) ControlArtifact(ctx context.Context, requestID string, scope AgentSettingsScope, id string, request ArtifactControl) (*ArtifactReceipt, error) {
	if request.ProjectIdentity != scope.ProjectIdentity || !libraryHash.MatchString(id) || !libraryHash.MatchString(request.Hash) || request.Version < 1 || !slices.Contains([]string{"export", "delete"}, request.Action) {
		return nil, fmt.Errorf("artifact control requires current scope, action, version and hash")
	}
	var result ArtifactReceipt
	if err := c.externalJSON(ctx, requestID, scope, "artifacts", id, "control", request, &result); err != nil {
		return nil, err
	}
	if err := validateArtifactReceipt(scope, id, request, result); err != nil {
		return nil, err
	}
	return &result, nil
}

func validateArtifactReceipt(scope AgentSettingsScope, id string, request ArtifactControl, result ArtifactReceipt) error {
	if result.ProjectIdentity != scope.ProjectIdentity || result.ID != id || result.Version != request.Version || result.Hash != request.Hash {
		return fmt.Errorf("artifact receipt differs from the reviewed version")
	}
	if request.Action == "delete" {
		if result.State != "deleted" || result.Path != "" {
			return fmt.Errorf("artifact deletion outcome is unavailable")
		}
	} else if result.State != "exported" || !validArtifactExportPath(result.Path) {
		return fmt.Errorf("artifact export receipt has no private destination")
	}
	return nil
}

func validArtifactExportPath(value string) bool {
	clean := filepath.ToSlash(filepath.Clean(value))
	return filepath.IsAbs(value) && strings.HasSuffix(path.Dir(clean), "/.mini-orca/exports/artifacts") && strings.HasPrefix(path.Base(clean), "artifact-")
}

func validateLibraryMemory(scope AgentSettingsScope, item *AgentMemory) error {
	if err := externalIdentity(scope, item.ProjectIdentity, item.ID); err != nil {
		return err
	}
	if err := validateMemoryRecordBounds(*item); err != nil {
		return err
	}
	for _, source := range item.Sources {
		if err := source.Validate(); err != nil {
			return err
		}
	}
	expires, err := time.Parse(time.RFC3339Nano, item.ExpiresAt)
	if err != nil {
		return fmt.Errorf("memory retention is unavailable")
	}
	item.Stale = item.Stale || item.ProjectIdentity != scope.ProjectIdentity || item.SourceStatus == "changed"
	item.Expired = item.Expired || !expires.After(time.Now())
	return nil
}

func validateMemoryRecordBounds(item AgentMemory) error {
	if item.Version < 1 || item.Version > 1<<53 || len(item.Title) > 120 || item.Title == "" || len(item.Text) > 2048 || len(item.Sources) < 1 || len(item.Sources) > 64 || !utf8.ValidString(item.Title) || !utf8.ValidString(item.Text) {
		return fmt.Errorf("memory record exceeds its retained bounds")
	}
	if !slices.Contains([]string{"candidate", "accepted"}, item.State) || !slices.Contains([]string{"decision", "lesson", "verified_outcome"}, item.Kind) || !slices.Contains([]string{"user", "proposal_checks", "harness_lesson"}, item.Provenance.Origin) {
		return fmt.Errorf("memory state or provenance is unavailable")
	}
	return nil
}

func (source MemorySource) Validate() error {
	if err := validateLibrarySourcePath(source.Path); err != nil {
		return err
	}
	if !strings.HasPrefix(source.Hash, "sha256:") || !libraryHash.MatchString(strings.TrimPrefix(source.Hash, "sha256:")) || source.StartLine < 0 || source.EndLine < source.StartLine || source.EndLine > 1000000 || (source.StartLine == 0) != (source.EndLine == 0) {
		return fmt.Errorf("memory source requires its exact hash and line range")
	}
	return nil
}

func validateLibrarySourcePath(value string) error {
	if value == "" || len(value) > 512 || path.IsAbs(value) || path.Clean(value) != value || value == "." || value == ".." || strings.HasPrefix(value, "../") || strings.Contains(value, "\\") || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("memory source requires a bounded relative path")
	}
	return nil
}

func (c *Client) ReadMemories(ctx context.Context, requestID string, scope AgentSettingsScope) ([]AgentMemory, error) {
	var result []AgentMemory
	if err := c.externalJSON(ctx, requestID, scope, "memory", "", "", nil, &result); err != nil {
		return nil, err
	}
	if len(result) > 512 {
		return nil, fmt.Errorf("memory inventory exceeds its retained bound")
	}
	seen := map[string]bool{}
	for i := range result {
		item := &result[i]
		if err := validateLibraryMemory(scope, item); err != nil {
			return nil, err
		}
		if item.Text != "" || item.SourceStatus != "unchecked" || seen[item.ID] {
			return nil, fmt.Errorf("memory inventory disclosed text or repeated a record")
		}
		seen[item.ID] = true
	}
	return result, nil
}

func (c *Client) MemoryDetail(ctx context.Context, requestID string, scope AgentSettingsScope, id string) (*AgentMemory, error) {
	var result AgentMemory
	if id == "" {
		return nil, fmt.Errorf("select a retained memory")
	}
	if err := c.externalJSON(ctx, requestID, scope, "memory", id, "", nil, &result); err != nil {
		return nil, err
	}
	if err := validateLibraryMemory(scope, &result); err != nil {
		return nil, err
	}
	if result.ID != id || !slices.Contains([]string{"current", "changed"}, result.SourceStatus) || result.Text == "" {
		return nil, fmt.Errorf("memory detail has no matching private disclosure")
	}
	return &result, nil
}

func (c *Client) MutateMemory(ctx context.Context, requestID string, scope AgentSettingsScope, id string, request MemoryMutation) (*AgentMemory, error) {
	if err := validateMemoryMutationRequest(scope, id, request); err != nil {
		return nil, err
	}
	var result *AgentMemory
	if err := c.externalJSON(ctx, requestID, scope, "memory", id, "", request, &result); err != nil {
		return nil, err
	}
	if request.Action == "forget" {
		if result != nil {
			return nil, fmt.Errorf("memory removal returned an unexpected record")
		}
		return nil, nil
	}
	if err := validateMemoryReceipt(scope, id, request, result); err != nil {
		return nil, err
	}
	return result, nil
}

func validateMemoryMutationRequest(scope AgentSettingsScope, id string, request MemoryMutation) error {
	if request.ProjectIdentity != scope.ProjectIdentity || !slices.Contains([]string{"create", "edit", "accept", "forget"}, request.Action) {
		return fmt.Errorf("memory action or scope changed")
	}
	if request.Action == "create" {
		if id != "" || request.Version != 0 {
			return fmt.Errorf("new memory requires no retained identity")
		}
	} else if !externalRecordID.MatchString(id) || request.Version < 1 {
		return fmt.Errorf("memory action requires its exact version")
	}
	needsDraft := request.Action == "create" || request.Action == "edit"
	if needsDraft != (request.Draft != nil) {
		return fmt.Errorf("create/edit require a draft; accept/forget require only identity")
	}
	if request.Draft != nil {
		return validateMemoryDraftRequest(*request.Draft)
	}
	return nil
}

func validateMemoryDraftRequest(draft MemoryDraft) error {
	if draft.Title == "" || len(draft.Title) > 120 || len(draft.Text) > 2048 || draft.RetentionDays < 0 || draft.RetentionDays > 365 || len(draft.Sources) < 1 || len(draft.Sources) > 64 {
		return fmt.Errorf("memory draft exceeds title, text, retention or source bounds")
	}
	for _, source := range draft.Sources {
		if err := source.Validate(); err != nil {
			return err
		}
	}
	return validateMemoryDraftOrigin(draft)
}

func validateMemoryDraftOrigin(draft MemoryDraft) error {
	if !slices.Contains([]string{"decision", "lesson", "verified_outcome"}, draft.Kind) {
		return fmt.Errorf("memory kind is unavailable")
	}
	if draft.SessionID != "" {
		if draft.Identity == nil || draft.Text != "" {
			return fmt.Errorf("retained proposal advice derives its text from an exact proposal identity")
		}
	} else if draft.Text == "" || draft.Identity != nil || draft.LessonIndex != nil || draft.Kind == "verified_outcome" {
		return fmt.Errorf("manual advice requires text and cannot claim proposal check authority")
	}
	return nil
}

func validateMemoryReceipt(scope AgentSettingsScope, id string, request MemoryMutation, result *AgentMemory) error {
	if result == nil {
		return fmt.Errorf("memory action returned no receipt")
	}
	if err := validateLibraryMemory(scope, result); err != nil {
		return err
	}
	if result.ProjectIdentity != scope.ProjectIdentity || id != "" && result.ID != id || result.Version <= request.Version || result.SourceStatus != "current" {
		return fmt.Errorf("memory receipt differs from the reviewed identity")
	}
	state := "candidate"
	if request.Action == "accept" {
		state = "accepted"
	}
	if result.State != state || result.Text == "" {
		return fmt.Errorf("memory action returned an unexpected state")
	}
	return nil
}

// LibrarySource brackets the legacy read-only file endpoint with scoped index
// checks and exact content identity. It never opens a different source as evidence.
func (c *Client) LibrarySource(ctx context.Context, requestID string, scope AgentSettingsScope, source MemorySource) (*LibrarySource, error) {
	if err := source.Validate(); err != nil {
		return nil, err
	}
	if err := c.requireLibrarySource(ctx, requestID+"-before", scope, source); err != nil {
		return nil, err
	}
	target, err := externalTarget(scope, "files/info", "", "")
	if err != nil {
		return nil, err
	}
	var result FileInfo
	if err := c.DoJSON(ctx, requestID, http.MethodGet, target+"&path="+url.QueryEscape(source.Path), nil, &result); err != nil {
		return nil, err
	}
	if err := c.requireLibrarySource(ctx, requestID+"-after", scope, source); err != nil {
		return nil, err
	}
	if result.Path != source.Path || result.ContentHash != source.Hash {
		return nil, fmt.Errorf("memory source changed; inspect the current file separately")
	}
	preview := &LibrarySource{FileInfo: result}
	if !result.Binary {
		hash := sha256.Sum256([]byte(result.Content))
		if "sha256:"+hex.EncodeToString(hash[:]) != source.Hash || !utf8.ValidString(result.Content) {
			return nil, fmt.Errorf("source preview does not match its retained content hash")
		}
		if len(preview.Content) > 64<<10 {
			preview.Content, preview.Truncated = preview.Content[:64<<10], true
			for !utf8.ValidString(preview.Content) {
				preview.Content = preview.Content[:len(preview.Content)-1]
			}
		}
	}
	return preview, nil
}

func (c *Client) requireLibrarySource(ctx context.Context, requestID string, scope AgentSettingsScope, source MemorySource) error {
	index, err := c.ExternalIndex(ctx, requestID, scope)
	if err != nil {
		return err
	}
	for _, file := range index.Files {
		if file.Path == source.Path && file.ContentHash == source.Hash && source.EndLine <= file.LineCount {
			return nil
		}
	}
	return fmt.Errorf("memory source is no longer in the current index with its retained hash and range")
}
