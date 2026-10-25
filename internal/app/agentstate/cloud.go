package agentstate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

var ErrCloudConflict = errors.New("cloud copy changed; refresh the retained receipt")
var cloudCopyID = regexp.MustCompile(`^mo-[a-f0-9]{32}$`)

// CloudCopy is source-free and durable before remote effects. An expired lease
// permits explicit reconciliation or deletion, never replay of an upload.
type CloudCopy struct {
	ID              string             `json:"id"`
	Version         int64              `json:"version"`
	ProjectID       string             `json:"project_id"`
	ProjectRevision string             `json:"project_revision"`
	Target          config.CloudTarget `json:"target"`
	Configuration   string             `json:"configuration"`
	Kind            string             `json:"kind"`
	SourceID        string             `json:"source_id"`
	SourceVersion   int64              `json:"source_version"`
	SourceRevision  string             `json:"source_revision"`
	Hash            string             `json:"hash"`
	Size            int                `json:"size"`
	State           string             `json:"state"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	CleanupAt       time.Time          `json:"cleanup_at"`
	LeaseUntil      time.Time          `json:"lease_until"`
}

func (r CloudCopy) validate() error {
	if !cloudCopyID.MatchString(r.ID) || r.Version < 1 || r.Version > 1<<53 || !validID(r.ProjectID) || !validID(r.ProjectRevision) || r.Target.Validate() != nil {
		return ErrCorrupt
	}
	if err := r.validateContent(); err != nil {
		return err
	}
	return r.validateLifecycle()
}

func (r CloudCopy) validateContent() error {
	if !validHash(r.Configuration) || !validHash(r.Hash) || !validID(r.SourceID) || !validID(r.SourceRevision) || r.SourceVersion < 0 || r.SourceVersion > 1<<53 || r.Size < 1 || r.Size > 4<<20 {
		return ErrCorrupt
	}
	if r.Kind != "session" && r.Kind != "artifact" || (r.Kind == "session") != (r.Target.Service == "vertex_session") {
		return ErrCorrupt
	}
	return nil
}

func (r CloudCopy) validateLifecycle() error {
	if !slices.Contains([]string{"copying", "checking", "deleting", "completed", "unknown", "missing", "deleted"}, r.State) || r.CreatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) || r.CleanupAt != r.CreatedAt.Add(time.Duration(r.Target.RetentionDays)*24*time.Hour) {
		return ErrCorrupt
	}
	busy := slices.Contains([]string{"copying", "checking", "deleting"}, r.State)
	if busy != !r.LeaseUntil.IsZero() || busy && (!r.LeaseUntil.After(r.UpdatedAt) || r.LeaseUntil.Sub(r.UpdatedAt) > 2*time.Minute) {
		return ErrCorrupt
	}
	return nil
}

type cloudCatalog struct {
	Schema    int         `json:"schema"`
	Workspace string      `json:"workspace"`
	Copies    []CloudCopy `json:"copies"`
}

func (s *Store) CloudCopies(ctx context.Context, projectID string) ([]CloudCopy, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	location, err := s.path("cloud.json")
	if err != nil {
		return nil, err
	}
	data, err := readJournalFile(location)
	if errors.Is(err, os.ErrNotExist) {
		return []CloudCopy{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read cloud receipts: %w", ErrPersistence, err)
	}
	var catalog cloudCatalog
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if len(data) > 256<<10 || !validJSON(data) || dec.Decode(&catalog) != nil || catalog.Schema != 1 || catalog.Workspace != s.workspace || len(catalog.Copies) > 128 {
		return nil, ErrCorrupt
	}
	seen := map[string]bool{}
	for _, r := range catalog.Copies {
		if r.validate() != nil || r.ProjectID != projectID || seen[r.ID] {
			return nil, ErrCorrupt
		}
		seen[r.ID] = true
	}
	return catalog.Copies, context.Cause(ctx)
}

func cloudCopyIdentity(r CloudCopy) CloudCopy {
	r.Version, r.State, r.UpdatedAt, r.LeaseUntil = 0, "", time.Time{}, time.Time{}
	return r
}

func (s *Store) SaveCloudCopy(ctx context.Context, r CloudCopy, expected int64, authorize func() error) (result CloudCopy, err error) {
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	lock, err := s.lock(ctx)
	if err != nil {
		return result, persistenceOperationError(parent, "lock cloud receipts", err)
	}
	defer func() {
		if e := lock.Close(); e != nil {
			err = errors.Join(err, fmt.Errorf("%w: unlock cloud receipts: %w", ErrPersistence, e))
		}
	}()
	rows, err := s.CloudCopies(ctx, r.ProjectID)
	if err != nil {
		return result, err
	}
	i := slices.IndexFunc(rows, func(old CloudCopy) bool { return old.ID == r.ID })
	if i < 0 {
		if expected != 0 || r.State != "copying" || len(rows) >= 128 {
			return result, ErrCloudConflict
		}
	} else if rows[i].Version != expected || cloudCopyIdentity(rows[i]) != cloudCopyIdentity(r) || r.State == "copying" {
		return result, ErrCloudConflict
	}
	r.Version, r.UpdatedAt = expected+1, time.Now().UTC()
	if r.validate() != nil {
		return result, ErrCorrupt
	}
	if i < 0 {
		rows = append(rows, r)
	} else {
		rows[i] = r
	}
	if err := s.writeCloudCopies(ctx, rows, authorize); err != nil {
		return result, persistenceOperationError(parent, "save cloud receipt", err)
	}
	return r, nil
}

func (s *Store) writeCloudCopies(ctx context.Context, rows []CloudCopy, authorize func() error) error {
	data, err := json.Marshal(cloudCatalog{Schema: 1, Workspace: s.workspace, Copies: rows})
	if err != nil {
		return err
	}
	location, err := s.path("cloud.json")
	if err != nil {
		return err
	}
	if len(data) > 256<<10 {
		return fmt.Errorf("cloud receipt capacity reached")
	}
	return s.write(location, data, 0600, func() error {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		if authorize == nil {
			return fmt.Errorf("cloud receipt requires scope authorization")
		}
		return authorize()
	})
}
