package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var cloudCopyID = regexp.MustCompile(`^mo-[a-f0-9]{32}$`)

func (c *Client) cloudJSON(ctx context.Context, id string, scope AgentSettingsScope, action string, payload, out any) error {
	target, err := externalTarget(scope, "cloud-copies", "", action)
	if err != nil {
		return err
	}
	method, body := http.MethodGet, ""
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode cloud request: %w", err)
		}
		method, body = http.MethodPost, string(data)
	}
	response, err := c.Request(ctx, id, method, target, body)
	if err != nil {
		return err
	}
	if response.Status < 200 || response.Status >= 300 {
		return fmt.Errorf("%s", DecodeDaemonError(response.Status, response.Body))
	}
	decoder := json.NewDecoder(bytes.NewBufferString(response.Body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode cloud receipt: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("cloud response contains trailing data")
	}
	return nil
}

func cloudIdentity(value string) bool {
	return value != "" && len(value) <= 128 && value != "." && value != ".." && !strings.ContainsAny(value, "/\\") && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
}

func cloudScope(scope AgentSettingsScope) error {
	if scope.WorktreePath != "" || !cloudIdentity(scope.ProjectID) || !cloudIdentity(scope.ProjectRevision) {
		return fmt.Errorf("cloud copies require the current main checkout")
	}
	return nil
}

func validateCloudTarget(target CloudTarget, configuration string) error {
	fields := [][2]string{
		{target.ID, `^[a-z][a-z0-9_]{0,23}$`},
		{target.Project, `^[1-9][0-9]{5,19}$`},
		{target.Region, `^[a-z]{2,16}-[a-z]{2,16}[0-9]{1,2}$`},
		{target.Principal, `^[a-z][a-z0-9-]{4,28}[a-z0-9]@[a-z][a-z0-9-]{4,28}[a-z0-9]\.iam\.gserviceaccount\.com$`},
		{target.CredentialFileEnv, `^[A-Z][A-Z0-9_]{0,63}$`},
	}
	for _, field := range fields {
		if matched, _ := regexp.MatchString(field[1], field[0]); !matched {
			return fmt.Errorf("cloud destination identity is invalid")
		}
	}
	pattern := `^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`
	if target.Service == "vertex_session" {
		pattern = `^[1-9][0-9]{0,30}$`
	}
	matched, _ := regexp.MatchString(pattern, target.Resource)
	if !matched || !slices.Contains([]string{"vertex_session", "gcs_artifact"}, target.Service) || target.RetentionDays < 1 || target.RetentionDays > 365 {
		return fmt.Errorf("cloud service, resource or cleanup policy is invalid")
	}
	data, err := json.Marshal(target)
	if err != nil {
		return fmt.Errorf("encode cloud destination: %w", err)
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != configuration {
		return fmt.Errorf("cloud destination differs from its configuration fingerprint")
	}
	return nil
}

func validateCloudContent(kind, sourceID, revision, hash string, version int64, size int, target CloudTarget) error {
	if !cloudIdentity(sourceID) || !cloudIdentity(revision) || !libraryHash.MatchString(hash) || version < 0 || version > 1<<53 || size < 1 || size > 4<<20 {
		return fmt.Errorf("cloud source identity, hash, version or size is invalid")
	}
	if !slices.Contains([]string{"session", "artifact"}, kind) || (kind == "session") != (target.Service == "vertex_session") {
		return fmt.Errorf("cloud source kind does not match the destination service")
	}
	return nil
}

func validateCloudPreview(scope AgentSettingsScope, p CloudPreview) error {
	if p.ProjectIdentity != scope.ProjectIdentity || !cloudCopyID.MatchString(p.ID) || p.TargetID != p.Target.ID {
		return fmt.Errorf("cloud preview belongs to another checkout or destination")
	}
	if err := validateCloudTarget(p.Target, p.Configuration); err != nil {
		return err
	}
	return validateCloudContent(p.Kind, p.SourceID, p.SourceRevision, p.Hash, p.SourceVersion, p.Size, p.Target)
}

func validateCloudCopy(scope AgentSettingsScope, r CloudCopy) error {
	// Receipts retain their original revision after the main checkout changes.
	if r.ProjectID != scope.ProjectID || !cloudIdentity(r.ProjectRevision) || !cloudCopyID.MatchString(r.ID) || r.Version < 1 || r.Version > 1<<53 {
		return fmt.Errorf("cloud receipt belongs to another project or has an invalid version")
	}
	if err := validateCloudTarget(r.Target, r.Configuration); err != nil {
		return err
	}
	if err := validateCloudContent(r.Kind, r.SourceID, r.SourceRevision, r.Hash, r.SourceVersion, r.Size, r.Target); err != nil {
		return err
	}
	return validateCloudLifecycle(r)
}

func validateCloudLifecycle(r CloudCopy) error {
	if !slices.Contains([]string{"copying", "checking", "deleting", "completed", "unknown", "missing", "deleted"}, r.State) || r.CreatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) || !r.CleanupAt.Equal(r.CreatedAt.Add(time.Duration(r.Target.RetentionDays)*24*time.Hour)) {
		return fmt.Errorf("cloud receipt state or cleanup deadline is invalid")
	}
	busy := slices.Contains([]string{"copying", "checking", "deleting"}, r.State)
	if busy != !r.LeaseUntil.IsZero() || busy && (!r.LeaseUntil.After(r.UpdatedAt) || r.LeaseUntil.Sub(r.UpdatedAt) > 2*time.Minute) {
		return fmt.Errorf("cloud receipt has an invalid operation lease")
	}
	return nil
}

func (c *Client) ReadCloudCopies(ctx context.Context, id string, scope AgentSettingsScope) (*CloudCatalog, error) {
	if err := cloudScope(scope); err != nil {
		return nil, err
	}
	var result CloudCatalog
	if err := c.cloudJSON(ctx, id, scope, "", nil, &result); err != nil {
		return nil, err
	}
	if result.ProjectIdentity != scope.ProjectIdentity || len(result.Destinations) > 4 || len(result.Copies) > 128 {
		return nil, fmt.Errorf("cloud catalog exceeds its bounds or belongs to another checkout")
	}
	seen := map[string]bool{}
	for _, destination := range result.Destinations {
		if err := validateCloudTarget(destination.Target, destination.Configuration); err != nil {
			return nil, err
		}
		if destination.State != "configured" || seen[destination.Target.ID] {
			return nil, fmt.Errorf("cloud catalog repeats a destination or invents availability")
		}
		seen[destination.Target.ID] = true
	}
	for _, receipt := range result.Copies {
		if err := validateCloudCopy(scope, receipt); err != nil {
			return nil, err
		}
		if seen[receipt.ID] {
			return nil, fmt.Errorf("cloud catalog repeats a receipt")
		}
		seen[receipt.ID] = true
	}
	return &result, nil
}

func (c *Client) RefreshCloudCopies(ctx context.Context, id string, scope AgentSettingsScope) (*CloudCatalog, *ProjectIndex, error) {
	if err := cloudScope(scope); err != nil {
		return nil, nil, err
	}
	var index ProjectIndex
	if err := c.DoJSON(ctx, id, http.MethodGet, "/api/projects/current/index", nil, &index); err != nil {
		return nil, nil, err
	}
	if index.ProjectID != scope.ProjectID || !cloudIdentity(index.ProjectRevision) {
		return nil, nil, fmt.Errorf("cloud index belongs to another project")
	}
	scope.ProjectIdentity = index.ProjectIdentity
	catalog, err := c.ReadCloudCopies(ctx, id, scope)
	if err != nil {
		return nil, nil, err
	}
	return catalog, &index, nil
}

func (c *Client) PreviewCloudCopy(ctx context.Context, id string, scope AgentSettingsScope, selection CloudSelection) (*CloudPreview, error) {
	if err := cloudScope(scope); err != nil {
		return nil, err
	}
	if selection.ProjectIdentity != scope.ProjectIdentity || !connectionIDPattern.MatchString(selection.TargetID) || !cloudIdentity(selection.SourceID) || !slices.Contains([]string{"artifact", "session"}, selection.Kind) {
		return nil, fmt.Errorf("select a destination and exact local artifact or completed invocation")
	}
	var result CloudPreview
	if err := c.cloudJSON(ctx, id, scope, "preview", selection, &result); err != nil {
		return nil, err
	}
	if err := validateCloudPreview(scope, result); err != nil {
		return nil, err
	}
	if result.CloudSelection != selection {
		return nil, fmt.Errorf("cloud preview differs from the selected source")
	}
	return &result, nil
}

func (c *Client) ControlCloudCopy(ctx context.Context, id string, scope AgentSettingsScope, request CloudControl, before *CloudCopy) (*CloudCopy, error) {
	if err := validateCloudControl(scope, request, before); err != nil {
		return nil, err
	}
	var result CloudCopy
	if err := c.cloudJSON(ctx, id, scope, "control", request, &result); err != nil {
		return nil, err
	}
	if err := validateCloudCopy(scope, result); err != nil {
		return nil, err
	}
	if err := validateCloudOutcome(request, before, result); err != nil {
		return nil, err
	}
	return &result, nil
}

func validateCloudControl(scope AgentSettingsScope, r CloudControl, before *CloudCopy) error {
	if err := cloudScope(scope); err != nil {
		return err
	}
	if r.ProjectIdentity != scope.ProjectIdentity || !r.Confirm || !cloudCopyID.MatchString(r.ID) || !slices.Contains([]string{"copy", "check", "delete"}, r.Action) {
		return fmt.Errorf("cloud control requires exact checkout identity and confirmation")
	}
	if r.Action == "copy" {
		if before != nil || r.Preview == nil || !r.ConfirmSourceSharing || r.Version != 0 || r.ID != r.Preview.ID {
			return fmt.Errorf("copy requires the exact preview and explicit source sharing")
		}
		return validateCloudPreview(scope, *r.Preview)
	}
	return validateCloudRecovery(scope, r, before)
}

func validateCloudRecovery(scope AgentSettingsScope, r CloudControl, before *CloudCopy) error {
	if r.Preview != nil || r.ConfirmSourceSharing || before == nil || before.ID != r.ID || before.Version != r.Version || before.State == "deleted" || before.LeaseUntil.After(time.Now()) {
		return fmt.Errorf("recovery requires a current idle receipt and cannot repeat an upload")
	}
	return validateCloudCopy(scope, *before)
}

func cloudCopyIdentity(r CloudCopy) CloudCopy {
	r.Version, r.State, r.UpdatedAt, r.LeaseUntil = 0, "", time.Time{}, time.Time{}
	return r
}

func validateCloudOutcome(request CloudControl, before *CloudCopy, result CloudCopy) error {
	if result.ID != request.ID || result.Version != request.Version+2 || !result.LeaseUntil.IsZero() {
		return fmt.Errorf("cloud control receipt differs from the admitted operation; refresh before recovery")
	}
	states := []string{"completed", "missing", "unknown"}
	if request.Action == "delete" {
		states = []string{"deleted", "unknown"}
	}
	if !slices.Contains(states, result.State) {
		return fmt.Errorf("cloud control returned an inconsistent outcome")
	}
	if before != nil {
		if cloudCopyIdentity(*before) != cloudCopyIdentity(result) || result.UpdatedAt.Before(before.UpdatedAt) {
			return fmt.Errorf("cloud recovery changed the retained source or destination")
		}
		return nil
	}
	preview := CloudPreview{CloudSelection: CloudSelection{ProjectIdentity: result.ProjectIdentity, TargetID: result.Target.ID, Kind: result.Kind, SourceID: result.SourceID}, ID: result.ID, Target: result.Target, Configuration: result.Configuration, SourceVersion: result.SourceVersion, SourceRevision: result.SourceRevision, Hash: result.Hash, Size: result.Size}
	if preview != *request.Preview {
		return fmt.Errorf("cloud copy receipt differs from the confirmed preview")
	}
	return nil
}
