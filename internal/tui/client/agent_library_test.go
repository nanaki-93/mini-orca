package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func libraryClientFixture() (AgentSettingsScope, ArtifactDetail, AgentMemory, FileInfo) {
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}, WorktreePath: "/tmp/task space"}
	text := "retained payload"
	hash := sha256.Sum256([]byte(text))
	artifact := ArtifactDetail{AgentArtifact: AgentArtifact{ProjectIdentity: scope.ProjectIdentity, ID: strings.Repeat("a", 64), Name: "report.txt", Version: 1, Hash: hex.EncodeToString(hash[:]), Kind: "text", MIME: "text/plain", Size: len(text), Owner: AgentSessionKey{App: "mini-orca", User: "local", ID: "session"}, Ownership: "project", RevisionStatus: "current", SourceStatus: "unchecked", CanDelete: true}, PreviewKind: "text", Text: text}
	content := "package main\n"
	sourceHash := sha256.Sum256([]byte(content))
	file := FileInfo{Path: "main.go", ContentHash: "sha256:" + hex.EncodeToString(sourceHash[:]), Content: content, LineCount: 1}
	memory := AgentMemory{ProjectIdentity: scope.ProjectIdentity, ID: externalFixtureID, Version: 1, Kind: "decision", State: "candidate", Title: "Choice", Text: "Private retained advice", Sources: []MemorySource{{Path: file.Path, Hash: file.ContentHash}}, Provenance: MemoryProvenance{Origin: "user"}, SourceStatus: "current", ExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339Nano)}
	return scope, artifact, memory, file
}

func TestLibraryClientPreservesWireScopesAndExactReceipts(t *testing.T) {
	scope, artifact, memory, file := libraryClientFixture()
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		q := r.URL.Query()
		if q.Get("project_id") != scope.ProjectID || q.Get("project_revision") != scope.ProjectRevision || q.Get("worktree_path") != scope.WorktreePath {
			t.Error("lost selected worktree identity")
		}
		if r.Method == "GET" {
			switch r.URL.Path {
			case "/api/projects/current/artifacts":
				if q.Has("cursor") || q.Get("limit") != "100" {
					t.Error("first page must omit an empty cursor and bound its limit")
				}
				_ = json.NewEncoder(w).Encode(ArtifactList{ProjectIdentity: scope.ProjectIdentity, Snapshot: strings.Repeat("b", 64), Artifacts: []AgentArtifact{artifact.AgentArtifact}})
			case "/api/projects/current/artifacts/" + artifact.ID:
				_ = json.NewEncoder(w).Encode(artifact)
			case "/api/projects/current/memory":
				item := memory
				item.Text, item.SourceStatus = "", "unchecked"
				_ = json.NewEncoder(w).Encode([]AgentMemory{item})
			case "/api/projects/current/memory/" + memory.ID:
				_ = json.NewEncoder(w).Encode(memory)
			case "/api/projects/current/index":
				_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: scope.ProjectIdentity, Files: []IndexedFile{{Path: file.Path, ContentHash: file.ContentHash, LineCount: file.LineCount}}})
			case "/api/projects/current/files/info":
				if q.Get("path") != file.Path {
					t.Error("source link lost its selected path")
				}
				_ = json.NewEncoder(w).Encode(file)
			default:
				t.Errorf("unexpected read: %s", r.URL)
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/projects/current/artifacts/") {
			var control ArtifactControl
			if err := json.NewDecoder(r.Body).Decode(&control); err != nil || control.ProjectIdentity != scope.ProjectIdentity || control.Version != artifact.Version || control.Hash != artifact.Hash {
				t.Error("lost exact artifact request")
			}
			receipt := ArtifactReceipt{ProjectIdentity: scope.ProjectIdentity, ID: artifact.ID, Version: control.Version, Hash: control.Hash, State: "deleted"}
			if control.Action == "export" {
				receipt.State, receipt.Path = "exported", "/tmp/task space/.mini-orca/exports/artifacts/artifact-copy.bin"
			}
			_ = json.NewEncoder(w).Encode(receipt)
			return
		}
		var request MemoryMutation
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.ProjectIdentity != scope.ProjectIdentity {
			t.Error("lost memory request identity")
		}
		if request.Action == "forget" {
			_, _ = w.Write([]byte("null"))
			return
		}
		item := memory
		item.Version = request.Version + 1
		if request.Action == "accept" {
			item.State = "accepted"
		} else if request.Draft == nil || request.Draft.Sources[0].Hash != file.ContentHash {
			t.Error("missing captured memory sources")
		}
		_ = json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	if _, err := cl.ReadArtifacts(t.Context(), "artifacts", scope, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.ArtifactDetail(t.Context(), "preview", scope, artifact.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.ReadMemories(t.Context(), "memory", scope); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.MemoryDetail(t.Context(), "advice", scope, memory.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.LibrarySource(t.Context(), "source", scope, memory.Sources[0]); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"export", "delete"} {
		if _, err := cl.ControlArtifact(t.Context(), action, scope, artifact.ID, ArtifactControl{ProjectIdentity: scope.ProjectIdentity, Version: artifact.Version, Hash: artifact.Hash, Action: action}); err != nil {
			t.Fatal(err)
		}
	}
	for _, action := range []string{"create", "edit", "accept", "forget"} {
		id := memory.ID
		request := MemoryMutation{ProjectIdentity: scope.ProjectIdentity, Action: action, Version: memory.Version}
		if action == "create" {
			id, request.Version = "", 0
		}
		if action == "create" || action == "edit" {
			request.Draft = &MemoryDraft{Kind: "decision", Title: memory.Title, Text: memory.Text, Sources: memory.Sources, RetentionDays: 30}
		}
		if _, err := cl.MutateMemory(t.Context(), action, scope, id, request); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	if requests.Load() != 13 {
		t.Fatalf("unexpected extra transport or missing scope check: %d", requests.Load())
	}
}

func TestLibraryClientRejectsForeignOversizedAndInconsistentResponses(t *testing.T) {
	for _, kind := range []string{"foreign", "hash", "oversized", "duplicate", "text-in-list", "false-acceptance", "missing-export-path"} {
		t.Run(kind, func(t *testing.T) {
			scope, artifact, memory, _ := libraryClientFixture()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "foreign":
					artifact.ProjectID = "other"
					_ = json.NewEncoder(w).Encode(artifact)
				case "hash":
					artifact.Text = "different bytes"
					_ = json.NewEncoder(w).Encode(artifact)
				case "oversized":
					artifact.Text = strings.Repeat("x", (64<<10)+1)
					artifact.Truncated = true
					_ = json.NewEncoder(w).Encode(artifact)
				case "duplicate":
					_ = json.NewEncoder(w).Encode(ArtifactList{ProjectIdentity: scope.ProjectIdentity, Snapshot: strings.Repeat("b", 64), Artifacts: []AgentArtifact{artifact.AgentArtifact, artifact.AgentArtifact}})
				case "text-in-list":
					_ = json.NewEncoder(w).Encode([]AgentMemory{memory})
				case "false-acceptance":
					memory.Version++
					_ = json.NewEncoder(w).Encode(memory)
				case "missing-export-path":
					_ = json.NewEncoder(w).Encode(ArtifactReceipt{ProjectIdentity: scope.ProjectIdentity, ID: artifact.ID, Version: artifact.Version, Hash: artifact.Hash, State: "exported", Path: "/tmp/source.go"})
				}
			}))
			defer server.Close()
			cl, _ := NewClient(server.URL)
			defer cl.Close()
			var err error
			switch kind {
			case "duplicate":
				_, err = cl.ReadArtifacts(t.Context(), "read", scope, "")
			case "text-in-list":
				_, err = cl.ReadMemories(t.Context(), "read", scope)
			case "false-acceptance":
				_, err = cl.MutateMemory(t.Context(), "accept", scope, memory.ID, MemoryMutation{ProjectIdentity: scope.ProjectIdentity, Action: "accept", Version: 1})
			case "missing-export-path":
				_, err = cl.ControlArtifact(t.Context(), "export", scope, artifact.ID, ArtifactControl{ProjectIdentity: scope.ProjectIdentity, Action: "export", Version: 1, Hash: artifact.Hash})
			default:
				_, err = cl.ArtifactDetail(t.Context(), "read", scope, artifact.ID)
			}
			if err == nil {
				t.Fatalf("accepted %s", kind)
			}
		})
	}
}

func TestLibraryClientSourceLinksCheckIdentityAndBoundText(t *testing.T) {
	for _, kind := range []string{"changed-hash", "changed-checkout", "removed-before", "removed-after", "changed-index-hash", "changed-range", "bounded"} {
		t.Run(kind, func(t *testing.T) {
			scope, _, memory, file := libraryClientFixture()
			if kind == "bounded" {
				file.Content = strings.Repeat("€", 30000)
				hash := sha256.Sum256([]byte(file.Content))
				file.ContentHash = "sha256:" + hex.EncodeToString(hash[:])
				memory.Sources[0].Hash = file.ContentHash
			}
			var indexes atomic.Int64
			var sourceReads atomic.Int64
			if kind == "changed-range" {
				memory.Sources[0].StartLine, memory.Sources[0].EndLine = 1, 2
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/projects/current/index" {
					identity := scope.ProjectIdentity
					read := indexes.Add(1)
					if read == 2 && kind == "changed-checkout" {
						identity.ProjectID = "other"
					}
					files := []IndexedFile{{Path: file.Path, ContentHash: file.ContentHash, LineCount: file.LineCount}}
					if kind == "removed-before" || kind == "removed-after" && read == 2 {
						files = nil
					} else if kind == "changed-index-hash" {
						files[0].ContentHash = "sha256:" + strings.Repeat("0", 64)
					}
					_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: identity, Files: files})
					return
				}
				sourceReads.Add(1)
				copy := file
				if kind == "changed-hash" {
					copy.ContentHash = "sha256:" + strings.Repeat("0", 64)
				}
				_ = json.NewEncoder(w).Encode(copy)
			}))
			defer server.Close()
			cl, _ := NewClient(server.URL)
			defer cl.Close()
			value, err := cl.LibrarySource(t.Context(), "source", scope, memory.Sources[0])
			if kind == "bounded" {
				if err != nil || value == nil || !value.Truncated || len(value.Content) > 64<<10 {
					t.Fatalf("source bound=%+v %v", value, err)
				}
			} else if err == nil {
				t.Fatalf("source preview ignored %s", kind)
			}
			if (kind == "removed-before" || kind == "changed-index-hash" || kind == "changed-range") && sourceReads.Load() != 0 {
				t.Fatal("read an ineligible memory source")
			}
		})
	}
}

func TestLibraryClientActualArtifactHandlerContract(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "library fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(config.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}}
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveArtifact(t.Context(), agentstate.ArtifactWrite{Key: agentstate.ArtifactKey{Session: agentstate.Key{App: "mini-orca", User: "local", ID: "session"}, Name: "evidence.txt"}, Kind: "text", MIME: "text/plain", Data: []byte("actual retained content"), Provenance: map[string]string{"project": index.ProjectID, "revision": index.ProjectRevision}})
	if err != nil {
		t.Fatal(err)
	}
	h := handlers.NewAgentLibraryHandler(service)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects/current/artifacts", h.List)
	mux.HandleFunc("GET /api/projects/current/artifacts/{artifactID}", h.Detail)
	mux.HandleFunc("POST /api/projects/current/artifacts/{artifactID}/control", h.Control)
	server := httptest.NewServer(mux)
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	list, err := cl.ReadArtifacts(t.Context(), "list", scope, "")
	if err != nil || len(list.Artifacts) != 1 {
		t.Fatalf("actual list contract=%+v %v", list, err)
	}
	item := list.Artifacts[0]
	if _, err := cl.ArtifactDetail(t.Context(), "detail", scope, item.ID); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"export", "delete"} {
		if _, err := cl.ControlArtifact(t.Context(), action, scope, item.ID, ArtifactControl{ProjectIdentity: scope.ProjectIdentity, Action: action, Version: item.Version, Hash: item.Hash}); err != nil {
			t.Fatalf("actual %s contract: %v", action, err)
		}
	}
}
