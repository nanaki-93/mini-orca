package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type AttachmentImport struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	Name            string `json:"name"`
	MIME            string `json:"mime"`
	Data            []byte `json:"data"`
	ConfirmImport   bool   `json:"confirm_import"`
}

type AttachmentReference struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
	Hash    string `json:"hash"`
}

type AttachmentOverview struct {
	ProjectID       string             `json:"project_id"`
	ProjectRevision string             `json:"project_revision"`
	State           string             `json:"state"`
	Configuration   string             `json:"configuration"`
	SettingsVersion int64              `json:"settings_version"`
	Model           EffectiveModel     `json:"model"`
	Limits          config.AgentLimits `json:"limits"`
	Attachments     []AgentArtifact    `json:"attachments"`
	Truncated       bool               `json:"truncated"`
}

func attachmentSession(projectID string) agentstate.Key {
	return agentstate.Key{App: "mini-orca", User: "local", ID: "attachments_" + projectID}
}

func (s *Service) ImportAttachment(ctx context.Context, request AttachmentImport) (*AgentArtifact, error) {
	if !request.ConfirmImport {
		return nil, fmt.Errorf("explicit local attachment import confirmation is required")
	}
	media := llm.Attachment{Name: request.Name, MIME: request.MIME, Data: slices.Clone(request.Data)}
	if err := media.Validate(); err != nil {
		return nil, err
	}
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	root := s.manager.Root()
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	if err := s.artifactLibraryScope(ctx, query, root); err != nil {
		return nil, err
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, artifactLibraryError(err)
	}
	record, err := store.SaveArtifact(ctx, agentstate.ArtifactWrite{
		Key:  agentstate.ArtifactKey{Session: attachmentSession(query.ProjectID), Name: rand.Text()},
		Kind: "inline", MIME: media.MIME, DisplayName: media.Name, Data: media.Data,
		Metadata:   map[string]string{"kind": "attachment-v1"},
		Provenance: map[string]string{"project": query.ProjectID, "revision": query.ProjectRevision, "policy": policy.Version()},
	})
	if err != nil {
		return nil, artifactLibraryError(err)
	}
	view, _ := artifactLibraryView(store, agentstate.ArtifactInventoryEntry{Artifact: *record}, query)
	return &view, nil
}

func (s *Service) AttachmentOverview(ctx context.Context, query AgentRunQuery) (*AttachmentOverview, error) {
	root := s.manager.Root()
	settings, err := s.AgentSettings(ctx, query)
	if err != nil {
		return nil, err
	}
	store, entries, err := s.artifactInventory(ctx, query, root)
	if err != nil {
		return nil, err
	}
	result := &AttachmentOverview{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, State: "available", Configuration: settings.Configuration, SettingsVersion: settings.Version, Model: s.runtimes.function.effective, Limits: settings.Limits, Attachments: []AgentArtifact{}}
	if !llm.SupportsAttachments(s.runtimes.function.profile) {
		result.State = "unsupported_profile"
	} else if settings.State == "stale" {
		result.State = "stale_settings"
	}
	for _, entry := range entries {
		if !importedAttachment(entry.Artifact, query.ProjectID) {
			continue
		}
		if view, owned := artifactLibraryView(store, entry, query); owned {
			result.Attachments = append(result.Attachments, view)
		}
	}
	slices.SortFunc(result.Attachments, func(a, b AgentArtifact) int {
		if order := b.Created.Compare(a.Created); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	result.Truncated = len(result.Attachments) > 128
	result.Attachments = result.Attachments[:min(len(result.Attachments), 128)]
	return result, s.artifactLibraryScope(ctx, query, root)
}

func importedAttachment(record agentstate.Artifact, projectID string) bool {
	return record.Key.Session == attachmentSession(projectID) && record.Metadata["kind"] == "attachment-v1" && record.Kind == "inline" && !record.Protected
}

func (s *Service) loadAttachments(ctx context.Context, query AgentRunQuery, root, policy string, references []AttachmentReference) ([]llm.Attachment, error) {
	if len(references) < 1 || len(references) > 4 {
		return nil, fmt.Errorf("select between one and four imported attachment versions")
	}
	store, entries, err := s.artifactInventory(ctx, query, root)
	if err != nil {
		return nil, err
	}
	items := make([]llm.Attachment, 0, len(references))
	seen := map[string]bool{}
	for _, ref := range references {
		if seen[ref.ID] {
			return nil, fmt.Errorf("attachment selection repeats a version")
		}
		seen[ref.ID] = true
		media, err := selectAttachment(ctx, store, entries, query, policy, ref)
		if err != nil {
			return nil, err
		}
		items = append(items, media)
	}
	if err := llm.ValidateAttachments(items); err != nil {
		return nil, err
	}
	return items, nil
}

func selectAttachment(ctx context.Context, store *agentstate.Store, entries []agentstate.ArtifactInventoryEntry, query AgentRunQuery, policy string, ref AttachmentReference) (llm.Attachment, error) {
	view, key, err := selectLibraryArtifact(store, entries, query, ref.ID)
	if err != nil {
		return llm.Attachment{}, err
	}
	if view.Version != ref.Version || view.Hash != ref.Hash || view.ProjectRevision != query.ProjectRevision || view.Provenance["policy"] != policy {
		return llm.Attachment{}, ErrArtifactConflict
	}
	record, err := store.ExportArtifact(ctx, key, ref.Version)
	if err != nil {
		return llm.Attachment{}, artifactLibraryError(err)
	}
	if !importedAttachment(*record, query.ProjectID) || record.Hash != ref.Hash {
		return llm.Attachment{}, ErrArtifactConflict
	}
	return llm.Attachment{Name: record.DisplayName, MIME: record.MIME, Data: record.Data}, nil
}
