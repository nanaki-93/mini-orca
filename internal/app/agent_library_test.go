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
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func libraryFixture(t *testing.T) (*Service, *agentstate.Store, AgentRunQuery, agentstate.ArtifactWrite) {
	t.Helper()
	s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	query := AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}
	request := agentstate.ArtifactWrite{Key: agentstate.ArtifactKey{Session: agentstate.Key{App: "mini-orca", User: "local", ID: "fixture"}, Name: "evidence.txt"}, Kind: "text", MIME: "text/plain", Data: []byte("PRIVATE_ARTIFACT_PAYLOAD"), Metadata: map[string]string{"private": "PRIVATE_CUSTOM_METADATA"}, Provenance: map[string]string{"project": query.ProjectID, "revision": query.ProjectRevision, "run": "fixture-run"}}
	return s, store, query, request
}

func TestArtifactLibraryScopePaginationAndPassiveDisclosure(t *testing.T) {
	s, store, query, request := libraryFixture(t)
	empty, err := s.ListArtifacts(t.Context(), query)
	if err != nil || len(empty.Artifacts) != 0 {
		t.Fatalf("empty catalog=%+v %v", empty, err)
	}
	if _, err := os.Stat(filepath.Join(s.manager.Root(), ".mini-orca", "agents")); !os.IsNotExist(err) {
		t.Fatalf("passive read created storage: %v", err)
	}
	for range 3 {
		if _, err := store.SaveArtifact(t.Context(), request); err != nil {
			t.Fatal(err)
		}
	}
	request.Key.Name, request.Provenance = "foreign", map[string]string{"project": "other"}
	request.Metadata["project_id"] = query.ProjectID
	foreign, err := store.SaveArtifact(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	query.Limit = 2
	first, err := s.ListArtifacts(t.Context(), query)
	if err != nil || len(first.Artifacts) != 2 || first.NextCursor == "" {
		t.Fatalf("first page=%+v %v", first, err)
	}
	data, err := json.Marshal(first)
	if err != nil || bytes.Contains(data, []byte("PRIVATE_")) || first.Artifacts[0].RevisionStatus != "current" || first.Artifacts[0].SourceStatus != "unchecked" {
		t.Fatalf("metadata leaked content or claimed source validation: %s %v", data, err)
	}
	query.Cursor = first.NextCursor
	last, err := s.ListArtifacts(t.Context(), query)
	if err != nil || len(last.Artifacts) != 1 || last.NextCursor != "" || last.Snapshot != first.Snapshot || last.Artifacts[0].ID == first.Artifacts[1].ID {
		t.Fatalf("second page=%+v %v", last, err)
	}
	detail, err := s.ArtifactDetail(t.Context(), query, first.Artifacts[0].ID)
	if err != nil || detail.Text != "PRIVATE_ARTIFACT_PAYLOAD" || detail.PreviewKind != "text" {
		t.Fatalf("detail=%+v %v", detail, err)
	}
	if _, err := s.ArtifactDetail(t.Context(), query, artifactLibraryID(*foreign)); !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("foreign detail exposed: %v", err)
	}
	request.Provenance["project"] = query.ProjectID
	if _, err := store.SaveArtifact(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListArtifacts(t.Context(), query); !errors.Is(err, ErrArtifactConflict) {
		t.Fatalf("changed catalog accepted cursor: %v", err)
	}
	query.ProjectRevision = "stale"
	if _, err := s.ListArtifacts(t.Context(), query); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale project accepted: %v", err)
	}
}

func TestArtifactLibraryExportDeletionAndProtection(t *testing.T) {
	s, store, query, request := libraryFixture(t)
	before, err := os.ReadFile(filepath.Join(s.manager.Root(), "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.SaveArtifact(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	id := artifactLibraryID(*record)
	control := ArtifactControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "export", Version: record.Version, Hash: record.Hash}
	receipt, err := s.ControlArtifact(t.Context(), id, control)
	if err != nil || receipt.State != "exported" || receipt.Path == "" || receipt.Hash != record.Hash {
		t.Fatalf("export=%+v %v", receipt, err)
	}
	exported, err := os.ReadFile(receipt.Path)
	if err != nil || !bytes.Equal(exported, record.Data) {
		t.Fatalf("exported bytes=%q %v", exported, err)
	}
	control.Action, control.Hash = "delete", strings.Repeat("0", 64)
	if _, err := s.ControlArtifact(t.Context(), id, control); !errors.Is(err, ErrArtifactConflict) {
		t.Fatalf("wrong hash deleted: %v", err)
	}
	control.Hash, control.ProjectRevision = record.Hash, "stale"
	if _, err := s.ControlArtifact(t.Context(), id, control); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("wrong revision deleted: %v", err)
	}
	control.ProjectRevision = query.ProjectRevision
	if receipt, err = s.ControlArtifact(t.Context(), id, control); err != nil || receipt.State != "deleted" || receipt.Path != "" {
		t.Fatalf("delete=%+v %v", receipt, err)
	}
	if _, err := s.ArtifactDetail(t.Context(), query, id); !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("deleted version retained: %v", err)
	}
	request.Protected = true
	record, err = store.SaveArtifact(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	id, control.Version = artifactLibraryID(*record), record.Version
	if _, err := s.ControlArtifact(t.Context(), id, control); !errors.Is(err, ErrArtifactProtected) {
		t.Fatalf("protected evidence deleted: %v", err)
	}
	list, err := s.ListArtifacts(t.Context(), query)
	if err != nil || len(list.Artifacts) != 1 || list.Artifacts[0].CanDelete || !list.Artifacts[0].Protected {
		t.Fatalf("protection metadata=%+v %v", list, err)
	}
	after, err := os.ReadFile(filepath.Join(s.manager.Root(), "main.go"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("artifact action changed project source")
	}
}

func TestArtifactLibraryPreviewBoundsAndOwnership(t *testing.T) {
	s, store, query, request := libraryFixture(t)
	request.Data = []byte(strings.Repeat("€", 30000))
	request.Provenance["revision"] = "old"
	record, err := store.SaveArtifact(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := s.ArtifactDetail(t.Context(), query, artifactLibraryID(*record))
	if err != nil || !detail.Truncated || len(detail.Text) > 64<<10 || !utf8.ValidString(detail.Text) || detail.RevisionStatus != "stale" {
		t.Fatalf("preview bound or freshness failed: %v", err)
	}
	request.Kind, request.MIME, request.Data = "inline", "application/octet-stream", []byte{0xff, 0x00}
	request.Provenance["revision"] = ""
	record, err = store.SaveArtifact(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	detail, err = s.ArtifactDetail(t.Context(), query, artifactLibraryID(*record))
	if err != nil || detail.PreviewKind != "binary" || detail.Text != "" || detail.RevisionStatus != "unknown" {
		t.Fatalf("binary preview=%+v %v", detail, err)
	}
	request.Key = researchKey(query.ProjectID, "report")
	request.Kind, request.MIME, request.Data = "inline", "application/json", []byte(`{"answer":"private answer"}`)
	request.Provenance = map[string]string{"policy": "policy"}
	request.Metadata = map[string]string{"kind": "grounded_research_v1", "project_id": query.ProjectID, "project_revision": query.ProjectRevision}
	record, err = store.SaveArtifact(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	detail, err = s.ArtifactDetail(t.Context(), query, artifactLibraryID(*record))
	if err != nil || detail.PreviewKind != "research_report" || detail.Text != "" || detail.ReportID != "report" {
		t.Fatalf("research bypassed report disclosure: %+v %v", detail, err)
	}
	request.Key.Session.ID = "arbitrary-sdk-session"
	record, err = store.SaveArtifact(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ArtifactDetail(t.Context(), query, artifactLibraryID(*record)); !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("untrusted metadata claimed project ownership: %v", err)
	}
}

func TestArtifactLibraryCancellationCorruptionAndInvalidInput(t *testing.T) {
	s, store, query, request := libraryFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.ListArtifacts(ctx, query); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read: %v", err)
	}
	if _, err := s.ArtifactDetail(t.Context(), query, "../escape"); err == nil {
		t.Fatal("invalid ID accepted")
	}
	for _, cursor := range []string{"bad", strings.Repeat("a", 2049)} {
		query.Cursor = cursor
		if _, err := s.ListArtifacts(t.Context(), query); err == nil {
			t.Fatal("invalid cursor accepted")
		}
	}
	query.Cursor, query.Limit = "", 101
	if _, err := s.ListArtifacts(t.Context(), query); err == nil {
		t.Fatal("unbounded page accepted")
	}
	query.Limit = 0
	if _, err := store.SaveArtifact(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(s.manager.Root(), ".mini-orca", "agents", "*", "artifacts.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("catalog paths=%v %v", paths, err)
	}
	if err := os.WriteFile(paths[0], []byte("corrupt catalog"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListArtifacts(t.Context(), query); !errors.Is(err, ErrArtifactUnavailable) {
		t.Fatalf("corruption returned empty success: %v", err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil || string(data) != "corrupt catalog" {
		t.Fatal("read replaced corrupt catalog")
	}
}
