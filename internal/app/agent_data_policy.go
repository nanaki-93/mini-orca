package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

type CloudSelection struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	TargetID        string `json:"target_id"`
	Kind            string `json:"kind"`
	SourceID        string `json:"source_id"`
}

type CloudPreview struct {
	CloudSelection
	ID             string             `json:"id"`
	Target         config.CloudTarget `json:"target"`
	Configuration  string             `json:"configuration"`
	SourceRevision string             `json:"source_revision"`
	SourceVersion  int64              `json:"source_version"`
	Hash           string             `json:"hash"`
	Size           int                `json:"size"`
}

func cloudHash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func cloudConfiguration(c config.CloudTarget) string {
	data, _ := json.Marshal(c)
	return cloudHash(data)
}

func (s *Service) cloudTarget(id string) (config.CloudTarget, error) {
	for _, target := range s.integrations.config.Cloud {
		if target.ID == id {
			return target, nil
		}
	}
	return config.CloudTarget{}, fmt.Errorf("select a configured cloud destination")
}

func (s *Service) PreviewCloudCopy(ctx context.Context, selection CloudSelection) (*CloudPreview, error) {
	preview, _, _, err := s.prepareCloudCopy(ctx, selection)
	return preview, err
}

func (s *Service) prepareCloudCopy(ctx context.Context, selection CloudSelection) (*CloudPreview, []byte, *agentstate.Store, error) {
	root := s.manager.Root()
	query := AgentRunQuery{ProjectID: selection.ProjectID, ProjectRevision: selection.ProjectRevision}
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, nil, nil, err
	}
	target, err := s.cloudTarget(selection.TargetID)
	if err != nil {
		return nil, nil, nil, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, nil, nil, err
	}
	var data []byte
	var revision string
	var version int64
	switch {
	case selection.Kind == "session" && target.Service == "vertex_session":
		data, revision, err = cloudSessionData(ctx, store, query, selection.SourceID)
	case selection.Kind == "artifact" && target.Service == "gcs_artifact":
		data, revision, version, err = cloudArtifactData(ctx, store, query, selection.SourceID)
	default:
		err = fmt.Errorf("select a completed session for Vertex or an immutable artifact for GCS")
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if len(data) == 0 || len(data) > 4<<20 {
		return nil, nil, nil, fmt.Errorf("cloud copy exceeds the 4 MiB limit")
	}
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, nil, nil, err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, nil, nil, err
	}
	return &CloudPreview{CloudSelection: selection, ID: "mo-" + hex.EncodeToString(id[:]), Target: target, Configuration: cloudConfiguration(target), SourceRevision: revision, SourceVersion: version, Hash: cloudHash(data), Size: len(data)}, data, store, nil
}

func cloudSessionData(ctx context.Context, store *agentstate.Store, query AgentRunQuery, id string) ([]byte, string, error) {
	history, err := store.History(ctx)
	if err != nil {
		return nil, "", err
	}
	i := slices.IndexFunc(history.Runs, func(r agentstate.RunSnapshot) bool { return r.ID == id && r.Binding.Project == query.ProjectID })
	if i < 0 {
		return nil, "", ErrAgentRunNotFound
	}
	run := history.Runs[i]
	if !cloudHistoryComplete(history, run.ID) {
		return nil, "", fmt.Errorf("only completed workflows without approval checkpoints can be copied")
	}
	// Export is an inert archive, not SDK state that can be resumed remotely.
	record, err := store.Session(ctx, run.Session)
	if err != nil {
		return nil, "", err
	}
	data, err := json.Marshal(struct {
		Format  string                 `json:"format"`
		Run     agentstate.RunSnapshot `json:"run"`
		Session *agentstate.Session    `json:"session"`
	}{"mini-orca-session-copy-v1", run, record})
	return data, run.Binding.Revision, err
}

func cloudHistoryComplete(history agentstate.History, id string) bool {
	parents := agentParents(history)
	root := agentRoot(id, parents)
	for _, run := range history.Runs {
		if agentRoot(run.ID, parents) == root && (run.Status != "completed" || run.Input != nil) {
			return false
		}
	}
	return true
}

func cloudArtifactData(ctx context.Context, store *agentstate.Store, query AgentRunQuery, id string) ([]byte, string, int64, error) {
	entries, err := store.ArtifactInventory(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	view, key, err := selectLibraryArtifact(store, entries, query, id)
	if err != nil {
		return nil, "", 0, err
	}
	if view.InUse {
		return nil, "", 0, fmt.Errorf("wait for the artifact owner to finish before copying")
	}
	record, err := store.ExportArtifact(ctx, key, view.Version)
	if err != nil {
		return nil, "", 0, err
	}
	if record.Hash != view.Hash {
		return nil, "", 0, ErrArtifactConflict
	}
	// Copy the selected payload and provenance; private SDK metadata is not needed.
	data, err := json.Marshal(struct {
		Format     string            `json:"format"`
		Name       string            `json:"name"`
		Version    int64             `json:"version"`
		MIME       string            `json:"mime"`
		Hash       string            `json:"hash"`
		Data       []byte            `json:"data"`
		Provenance map[string]string `json:"provenance"`
	}{"mini-orca-artifact-copy-v1", view.Name, view.Version, view.MIME, view.Hash, record.Data, view.Provenance})
	return data, view.ProjectRevision, view.Version, err
}
