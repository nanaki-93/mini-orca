package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func attachmentImportFixture(t *testing.T, endpoint string) (*Service, AttachmentImport) {
	t.Helper()
	s, request := researchService(t, endpoint)
	return s, AttachmentImport{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Name: "brief.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.7\n1 0 obj\nPRIVATE_ATTACHMENT\nendobj\n%%EOF\n"), ConfirmImport: true}
}

func attachmentQuery(request AttachmentImport) AgentRunQuery {
	return AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
}

func TestAttachmentImportIsPrivateImmutableAndPassivelyInspectable(t *testing.T) {
	s, request := attachmentImportFixture(t, "http://127.0.0.1:1")
	query := attachmentQuery(request)
	first, err := s.ImportAttachment(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ImportAttachment(t.Context(), request)
	if err != nil || second.ID == first.ID || first.Hash != second.Hash || first.Version != 1 {
		t.Fatalf("immutable import: %+v %v", second, err)
	}
	metadata, _ := json.Marshal(first)
	if bytes.Contains(metadata, []byte("PRIVATE_ATTACHMENT")) || bytes.Contains(metadata, []byte(`"data"`)) {
		t.Fatal("payload escaped import receipt")
	}
	view, err := s.AttachmentOverview(t.Context(), query)
	if err != nil || view.State != "available" || len(view.Attachments) != 2 || !view.Model.Capabilities.Attachments || view.Configuration == "" {
		t.Fatalf("overview: %+v %v", view, err)
	}
	detail, err := s.ArtifactDetail(t.Context(), query, first.ID)
	if err != nil || detail.PreviewKind != "binary" || detail.Text != "" {
		t.Fatalf("binary disclosure: %+v %v", detail, err)
	}
	receipt, err := s.ControlArtifact(t.Context(), first.ID, ArtifactControl{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Action: "export", Version: first.Version, Hash: first.Hash})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(receipt.Path)
	if err != nil || !bytes.Equal(data, request.Data) {
		t.Fatalf("private export changed: %v", err)
	}
	info, err := os.Stat(receipt.Path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("export permission: %v", err)
	}
	_, err = s.ControlArtifact(t.Context(), first.ID, ArtifactControl{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Action: "delete", Version: first.Version, Hash: first.Hash})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ArtifactDetail(t.Context(), query, first.ID); !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("deleted attachment: %v", err)
	}
	view, err = s.AttachmentOverview(t.Context(), query)
	if err != nil || len(view.Attachments) != 1 || view.Attachments[0].ID != second.ID {
		t.Fatalf("delete affected another import: %+v %v", view, err)
	}
}

func TestAttachmentImportRejectsInvalidScopeContentAndConsentWithoutWrites(t *testing.T) {
	for _, kind := range []string{"consent", "revision", "project", "MIME", "bytes", "name", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			s, request := attachmentImportFixture(t, "http://127.0.0.1:1")
			query := attachmentQuery(request)
			ctx := t.Context()
			switch kind {
			case "consent":
				request.ConfirmImport = false
			case "revision":
				request.ProjectRevision = "old"
			case "project":
				request.ProjectID = "another"
			case "MIME":
				request.MIME = "text/html"
			case "bytes":
				request.Data = []byte("malformed")
			case "name":
				request.Name = "https://example.invalid/file"
			case "canceled":
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			if value, err := s.ImportAttachment(ctx, request); err == nil || value != nil {
				t.Fatalf("invalid import: %+v %v", value, err)
			}
			view, err := s.AttachmentOverview(t.Context(), query)
			if err != nil || len(view.Attachments) != 0 {
				t.Fatalf("rejected import wrote content: %+v %v", view, err)
			}
		})
	}
}

func TestAttachmentOverviewBoundsInventoryAndPreservesStaleOwnership(t *testing.T) {
	s, request := attachmentImportFixture(t, "http://127.0.0.1:1")
	query := attachmentQuery(request)
	first, err := s.ImportAttachment(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	store, err := agentstate.Open(s.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 128; i++ {
		_, err := store.SaveArtifact(t.Context(), agentstate.ArtifactWrite{Key: agentstate.ArtifactKey{Session: attachmentSession(query.ProjectID), Name: "same"}, Kind: "inline", MIME: request.MIME, DisplayName: request.Name, Data: request.Data, Metadata: map[string]string{"kind": "attachment-v1"}, Provenance: map[string]string{"project": query.ProjectID, "revision": query.ProjectRevision}})
		if err != nil {
			t.Fatal(err)
		}
	}
	view, err := s.AttachmentOverview(t.Context(), query)
	if err != nil || !view.Truncated || len(view.Attachments) != 128 {
		t.Fatalf("inventory bound: %+v %v", view, err)
	}
	if err := os.WriteFile(filepath.Join(s.manager.Root(), "main.go"), []byte("package main\n// current revision\n"), 0600); err != nil {
		t.Fatal(err)
	}
	index, err := s.Reindex()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AttachmentOverview(t.Context(), query); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale query accepted: %v", err)
	}
	query.ProjectRevision = index.ProjectRevision
	detail, err := s.ArtifactDetail(t.Context(), query, first.ID)
	if err != nil || detail.RevisionStatus != "stale" {
		t.Fatalf("historical import lost: %+v %v", detail, err)
	}
}

func TestAttachmentStorageFailureRemainsUnavailableAndPreservesCatalog(t *testing.T) {
	s, request := attachmentImportFixture(t, "http://127.0.0.1:1")
	if _, err := s.ImportAttachment(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(s.manager.Root(), ".mini-orca", "agents", "*", "artifacts.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("catalog: %v %v", paths, err)
	}
	if err := os.WriteFile(paths[0], []byte("corrupt preserved media"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportAttachment(t.Context(), request); !errors.Is(err, ErrArtifactUnavailable) {
		t.Fatalf("write failure hidden: %v", err)
	}
	if _, err := s.AttachmentOverview(t.Context(), attachmentQuery(request)); !errors.Is(err, ErrArtifactUnavailable) {
		t.Fatalf("read failure hidden: %v", err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil || string(data) != "corrupt preserved media" {
		t.Fatal("corruption overwritten")
	}
	request.ProjectRevision = strings.Repeat("f", 64)
	if _, err := s.ImportAttachment(t.Context(), request); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("scope did not precede storage: %v", err)
	}
}
