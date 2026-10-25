package app

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

type CloudDestination struct {
	Target        config.CloudTarget `json:"target"`
	Configuration string             `json:"configuration"`
	State         string             `json:"state"`
}
type CloudCatalog struct {
	ProjectID       string                 `json:"project_id"`
	ProjectRevision string                 `json:"project_revision"`
	Destinations    []CloudDestination     `json:"destinations"`
	Copies          []agentstate.CloudCopy `json:"copies"`
}
type CloudControl struct {
	ProjectID            string        `json:"project_id"`
	ProjectRevision      string        `json:"project_revision"`
	Action               string        `json:"action"`
	ID                   string        `json:"id"`
	Version              int64         `json:"version"`
	Preview              *CloudPreview `json:"preview,omitempty"`
	Confirm              bool          `json:"confirm"`
	ConfirmSourceSharing bool          `json:"confirm_source_sharing"`
}

func (s *Service) CloudCopies(ctx context.Context, query AgentRunQuery) (*CloudCatalog, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, err
	}
	rows, err := store.CloudCopies(ctx, query.ProjectID)
	if err != nil {
		return nil, err
	}
	result := &CloudCatalog{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Copies: rows, Destinations: []CloudDestination{}}
	for _, target := range s.integrations.config.Cloud {
		result.Destinations = append(result.Destinations, CloudDestination{Target: target, Configuration: cloudConfiguration(target), State: "configured"})
	}
	return result, s.integrationScope(ctx, query, root)
}

func (s *Service) ControlCloudCopy(ctx context.Context, request CloudControl) (*agentstate.CloudCopy, error) {
	if len(request.ID) != 35 || !strings.HasPrefix(request.ID, "mo-") {
		return nil, fmt.Errorf("cloud control requires the exact preview or receipt ID")
	}
	if _, err := hex.DecodeString(request.ID[3:]); err != nil {
		return nil, fmt.Errorf("cloud copy ID must contain lowercase hexadecimal digits")
	}
	if strings.ToLower(request.ID) != request.ID {
		return nil, fmt.Errorf("cloud copy ID must contain lowercase hexadecimal digits")
	}
	if !request.Confirm || !slices.Contains([]string{"copy", "check", "delete"}, request.Action) {
		return nil, fmt.Errorf("cloud controls require an explicit copy, check or delete confirmation")
	}
	root := s.manager.Root()
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	authorize := func() error { return s.integrationScope(ctx, query, root) }
	if err := authorize(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	var record agentstate.CloudCopy
	var data []byte
	var store *agentstate.Store
	var err error
	if request.Action == "copy" {
		record, data, store, err = s.claimCloudCopy(ctx, request, authorize)
	} else {
		record, store, err = s.claimCloudControl(ctx, request, authorize)
	}
	if err != nil {
		return nil, err
	}
	open := s.openCloud
	if open == nil {
		open = agentruntime.OpenCloudServices
	}
	backend, remoteErr := open(ctx, record.Target)
	if remoteErr == nil {
		if err := authorize(); err != nil {
			remoteErr = err
		} else {
			record.State, remoteErr = runCloudControl(ctx, backend, request.Action, record, data)
		}
		remoteErr = errors.Join(remoteErr, backend.Close())
	}
	if remoteErr != nil {
		record.State = "unknown"
	}
	record.LeaseUntil = time.Time{}
	// The receipt must survive request cancellation. This bounded final write can
	// update only the admitted checkout/record; it cannot authorize further I/O.
	finish, done := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer done()
	result, saveErr := store.SaveCloudCopy(finish, record, record.Version, func() error { return s.agentHistoryScope(query, root) })
	if saveErr != nil {
		return nil, fmt.Errorf("cloud outcome may be retained; refresh its receipt before recovery: %w", saveErr)
	}
	return &result, nil
}

func runCloudControl(ctx context.Context, backend agentruntime.CloudServices, action string, r agentstate.CloudCopy, data []byte) (string, error) {
	switch action {
	case "copy":
		if err := backend.Copy(ctx, r, data); err != nil {
			return "unknown", err
		}
	case "delete":
		if err := backend.Delete(ctx, r); err != nil {
			return "unknown", err
		}
	}
	found, err := backend.Inspect(ctx, r)
	if err != nil {
		return "unknown", err
	}
	if action == "delete" {
		if found {
			return "unknown", agentruntime.ErrCloudUnavailable
		}
		return "deleted", nil
	}
	if found {
		return "completed", nil
	}
	return "missing", nil
}

func (s *Service) claimCloudCopy(ctx context.Context, r CloudControl, authorize func() error) (agentstate.CloudCopy, []byte, *agentstate.Store, error) {
	if !r.ConfirmSourceSharing || r.Preview == nil || r.Version != 0 || r.ID != r.Preview.ID || r.ProjectID != r.Preview.ProjectID || r.ProjectRevision != r.Preview.ProjectRevision {
		return agentstate.CloudCopy{}, nil, nil, fmt.Errorf("copy requires the exact preview and source-sharing confirmation")
	}
	current, data, store, err := s.prepareCloudCopy(ctx, r.Preview.CloudSelection)
	if err != nil {
		return agentstate.CloudCopy{}, nil, nil, err
	}
	current.ID = r.Preview.ID
	if *current != *r.Preview {
		return agentstate.CloudCopy{}, nil, nil, agentstate.ErrCloudConflict
	}
	now := time.Now().UTC()
	record := agentstate.CloudCopy{ID: r.ID, ProjectID: r.ProjectID, ProjectRevision: r.ProjectRevision, Target: current.Target, Configuration: current.Configuration, Kind: current.Kind, SourceID: current.SourceID, SourceVersion: current.SourceVersion, SourceRevision: current.SourceRevision, Hash: current.Hash, Size: current.Size, State: "copying", CreatedAt: now, CleanupAt: now.Add(time.Duration(current.Target.RetentionDays) * 24 * time.Hour), LeaseUntil: now.Add(90 * time.Second)}
	record, err = store.SaveCloudCopy(ctx, record, 0, authorize)
	return record, data, store, err
}

func (s *Service) claimCloudControl(ctx context.Context, r CloudControl, authorize func() error) (agentstate.CloudCopy, *agentstate.Store, error) {
	if r.Preview != nil || r.ConfirmSourceSharing || r.Version < 1 {
		return agentstate.CloudCopy{}, nil, fmt.Errorf("check/delete requires only a retained copy ID and current version")
	}
	store, err := agentstate.Open(s.manager.Root())
	if err != nil {
		return agentstate.CloudCopy{}, nil, err
	}
	rows, err := store.CloudCopies(ctx, r.ProjectID)
	if err != nil {
		return agentstate.CloudCopy{}, nil, err
	}
	i := slices.IndexFunc(rows, func(row agentstate.CloudCopy) bool { return row.ID == r.ID })
	if i < 0 {
		return agentstate.CloudCopy{}, nil, agentstate.ErrNotFound
	}
	record := rows[i]
	target, err := s.cloudTarget(record.Target.ID)
	if err != nil {
		return record, nil, err
	}
	if record.Version != r.Version || record.LeaseUntil.After(time.Now()) || record.Target != target || record.Configuration != cloudConfiguration(target) || record.State == "deleted" {
		return record, nil, agentstate.ErrCloudConflict
	}
	record.State = "checking"
	if r.Action == "delete" {
		record.State = "deleting"
	}
	record.LeaseUntil = time.Now().UTC().Add(90 * time.Second)
	record, err = store.SaveCloudCopy(ctx, record, r.Version, authorize)
	return record, store, err
}
