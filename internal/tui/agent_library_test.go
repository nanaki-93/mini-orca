package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func libraryTestState() *State {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}, Path: "/repo", Name: "Library fixture"}
	s.Connected = true
	content := "package main\n"
	hash := sha256.Sum256([]byte(content))
	sourceHash := "sha256:" + hex.EncodeToString(hash[:])
	s.File = &FileInfo{Path: "main.go", ContentHash: sourceHash, Content: content, LineCount: 1}
	s.Index = &ProjectIndex{ProjectIdentity: s.Project.ProjectIdentity, Files: []IndexedFile{{Path: "main.go", ContentHash: sourceHash, LineCount: 1}}}
	v := newLibraryView(s.Project.ProjectIdentity, client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, s.Project.Path)
	v.Loaded[libraryArtifacts], v.Loaded[libraryMemory] = true, true
	payload := "PRIVATE_ARTIFACT_TEXT"
	artifactHash := sha256.Sum256([]byte(payload))
	item := client.AgentArtifact{ProjectIdentity: s.Project.ProjectIdentity, ID: strings.Repeat("a", 64), Name: "report.txt", Version: 1, Hash: hex.EncodeToString(artifactHash[:]), Kind: "text", MIME: "text/plain", Size: len(payload), Owner: client.AgentSessionKey{App: "mini-orca", User: "local", ID: "session"}, Ownership: "project", RevisionStatus: "current", SourceStatus: "unchecked", CanDelete: true}
	v.Artifacts = &client.ArtifactList{ProjectIdentity: s.Project.ProjectIdentity, Snapshot: strings.Repeat("b", 64), Artifacts: []client.AgentArtifact{item}}
	memory := client.AgentMemory{ProjectIdentity: s.Project.ProjectIdentity, ID: externalTestID, Version: 1, Kind: "decision", State: "candidate", Title: "Retained choice", Sources: []client.MemorySource{{Path: "main.go", Hash: sourceHash}}, Provenance: client.MemoryProvenance{Origin: "user"}, SourceStatus: "unchecked", ExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339Nano)}
	v.Memories = []client.AgentMemory{memory}
	s.AgentLibrary = v
	s.Navigate(PageArtifacts)
	return s
}

func discloseLibraryMemory(s *State) {
	item := s.AgentLibrary.Memories[0]
	item.Text, item.SourceStatus = "PRIVATE_ADVICE_TEXT", "current"
	s.AgentLibrary.Memory = &item
	s.Navigate(PageMemory)
}

func libraryPending(t *testing.T, ctx context.Context, s *State, cl *Client) (*BubbleModel, tea.Cmd) {
	t.Helper()
	if len(s.PendingActions) != 1 {
		t.Fatalf("expected one library action: %d %s", len(s.PendingActions), s.Error)
	}
	model := NewBubbleModel(ctx, s, cl, 80, 24)
	cmd, ok := model.prepareAsyncActionCmd("library-test", s.PendingActions[0])
	s.PendingActions = nil
	if !ok || cmd == nil {
		t.Fatalf("library action was not prepared: %s", s.Error)
	}
	return model, cmd
}

func TestLibraryArtifactsRequireExactConfirmationAndPreserveProtection(t *testing.T) {
	for _, action := range []string{"export", "delete"} {
		t.Run(action, func(t *testing.T) {
			s := libraryTestState()
			s.ExecuteCommand(":artifact-" + action)
			if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, s.AgentLibrary.Artifacts.Artifacts[0].Hash) || !strings.Contains(s.Confirmation.Message, "/repo") || len(s.PendingActions) != 0 {
				t.Fatal("artifact action omitted exact review or dispatched before confirmation")
			}
			s.AgentLibrary.Artifacts.Artifacts[0].Version++
			s.HandleKey(KeyEvent{Key: "Enter"})
			if len(s.PendingActions) != 0 {
				t.Fatal("stale artifact confirmation dispatched")
			}
			s.Error = ""
			s.ExecuteCommand(":artifact-" + action)
			s.HandleKey(KeyEvent{Key: "Enter"})
			item, scope := s.AgentLibrary.Artifacts.Artifacts[0], s.AgentLibrary.Scope
			cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				var request client.ArtifactControl
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Version != item.Version || request.Hash != item.Hash || request.ProjectIdentity != scope.ProjectIdentity {
					t.Error("lost reviewed artifact identity")
				}
				value := client.ArtifactReceipt{ProjectIdentity: scope.ProjectIdentity, ID: item.ID, Version: item.Version, Hash: item.Hash, State: "deleted"}
				if action == "export" {
					value.State, value.Path = "exported", "/repo/.mini-orca/exports/artifacts/artifact-copy.bin"
				}
				_ = json.NewEncoder(w).Encode(value)
			})
			model, cmd := libraryPending(t, t.Context(), s, cl)
			model.Update(cmd())
			if s.Error != "" || s.AgentLibrary.Pending != "" {
				t.Fatal("artifact result unavailable", s.Error)
			}
			if action == "export" && (s.AgentLibrary.Export == nil || !strings.Contains(s.CmdOutput, "artifact-copy.bin")) {
				t.Fatal("export omitted actual private path")
			}
			if action == "delete" && len(s.AgentLibrary.Artifacts.Artifacts) != 0 {
				t.Fatal("deleted version remained in the catalog")
			}
		})
	}
	s := libraryTestState()
	s.AgentLibrary.Artifacts.Artifacts[0].Protected, s.AgentLibrary.Artifacts.Artifacts[0].CanDelete = true, false
	s.confirmArtifactControl("delete")
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("protected evidence had a deletion control")
	}
}

func TestLibraryQueuedAndLateActionsRetainRecordAndCheckoutOwnership(t *testing.T) {
	s := libraryTestState()
	s.confirmArtifactControl("export")
	s.HandleKey(KeyEvent{Key: "Enter"})
	action := s.PendingActions[0]
	s.PendingActions = nil
	s.AgentLibrary.Artifacts.Artifacts[0].Hash = strings.Repeat("c", 64)
	model := NewBubbleModel(t.Context(), s, nil, 80, 24)
	if cmd, ok := model.prepareAsyncActionCmd("queued", action); !ok || cmd != nil {
		t.Fatal("queued artifact used changed identity")
	}
	s = libraryTestState()
	s.inspectLibraryRecord()
	request := *s.PendingActions[0].AgentLibrary
	s.PendingActions = nil
	item := client.ArtifactDetail{AgentArtifact: s.AgentLibrary.Artifacts.Artifacts[0], PreviewKind: "text", Text: "PRIVATE_ARTIFACT_TEXT"}
	s.Project.ProjectRevision = "other"
	s.applyLibraryResult(agentLibraryResult{Action: request, Artifact: &item})
	if s.AgentLibrary.Artifact != nil {
		t.Fatal("late payload became evidence for another checkout")
	}
	s = libraryTestState()
	s.inspectLibraryRecord()
	request = *s.PendingActions[0].AgentLibrary
	s.PendingActions = nil
	s.AgentLibrary.Artifacts.Artifacts[0].Hash = strings.Repeat("d", 64)
	s.applyLibraryResult(agentLibraryResult{Action: request, Artifact: &item})
	if s.AgentLibrary.Artifact != nil || s.AgentLibrary.Failures[libraryArtifacts] == "" {
		t.Fatal("late payload replaced changed record")
	}
}

func TestLibraryMemoryInspectionAcceptanceRejectionAndForget(t *testing.T) {
	s := libraryTestState()
	s.Navigate(PageMemory)
	s.confirmMemoryControl("accept")
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("acceptance bypassed private inspection")
	}
	s.Error = ""
	discloseLibraryMemory(s)
	s.confirmMemoryControl("accept")
	if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, "PRIVATE_ADVICE_TEXT") || !strings.Contains(s.Confirmation.Message, s.File.ContentHash) {
		t.Fatal("acceptance omitted advice or source hash")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	before := *s.AgentLibrary.Memory
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var request client.MemoryMutation
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Version != before.Version {
			t.Error("memory action lost exact version")
		}
		if request.Action == "forget" {
			_, _ = w.Write([]byte("null"))
			return
		}
		item := before
		item.Version, item.State = before.Version+1, "accepted"
		_ = json.NewEncoder(w).Encode(item)
	})
	model, cmd := libraryPending(t, t.Context(), s, cl)
	model.Update(cmd())
	if s.AgentLibrary.Memory == nil || s.AgentLibrary.Memory.State != "accepted" || s.AgentLibrary.Memories[0].Text != "" {
		t.Fatal("accepted advice was lost or leaked into metadata")
	}
	s.confirmMemoryControl("reject")
	if s.Confirmation != nil {
		t.Fatal("accepted advice reused candidate rejection")
	}
	s.Error = ""
	before = *s.AgentLibrary.Memory
	s.confirmMemoryControl("forget")
	s.HandleKey(KeyEvent{Key: "Enter"})
	model, cmd = libraryPending(t, t.Context(), s, cl)
	model.Update(cmd())
	if len(s.AgentLibrary.Memories) != 0 || s.AgentLibrary.Memory != nil {
		t.Fatal("forgotten advice remained selectable")
	}
	s = libraryTestState()
	discloseLibraryMemory(s)
	s.confirmMemoryControl("reject")
	s.HandleKey(KeyEvent{Key: "Enter"})
	if len(s.PendingActions) != 1 || s.PendingActions[0].AgentLibrary.Memory.Action != "forget" {
		t.Fatal("candidate rejection did not use guarded forget")
	}
}

func TestLibraryMemoryRefusesStaleExpiredAndChangedConfirmations(t *testing.T) {
	for _, change := range []string{"stale", "expired", "changed-source", "version", "checkout"} {
		t.Run(change, func(t *testing.T) {
			s := libraryTestState()
			discloseLibraryMemory(s)
			if change == "version" || change == "checkout" {
				s.confirmMemoryControl("accept")
				if change == "version" {
					s.AgentLibrary.Memories[0].Version++
				} else {
					s.Project.ProjectID = "other"
				}
				s.HandleKey(KeyEvent{Key: "Enter"})
			} else {
				s.AgentLibrary.Memory.Stale = change == "stale"
				s.AgentLibrary.Memory.Expired = change == "expired"
				if change == "changed-source" {
					s.AgentLibrary.Memory.SourceStatus = "changed"
				}
				s.confirmMemoryControl("accept")
			}
			if s.Confirmation != nil || len(s.PendingActions) != 0 || s.Error == "" {
				t.Fatalf("accepted %s advice", change)
			}
		})
	}
}

func TestLibraryAdviceCapturesSourcesBeforeConfirmationAndRemainsCandidate(t *testing.T) {
	for _, operation := range []string{"create", "edit"} {
		t.Run(operation, func(t *testing.T) {
			s := libraryTestState()
			discloseLibraryMemory(s)
			var reads, writes atomic.Int64
			index, memory := *s.Index, *s.AgentLibrary.Memory
			index.Files = append([]IndexedFile(nil), index.Files...)
			if operation == "edit" {
				index.Files[0].ContentHash = "sha256:" + strings.Repeat("e", 64)
			}
			cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					reads.Add(1)
					_ = json.NewEncoder(w).Encode(index)
					return
				}
				writes.Add(1)
				var request client.MemoryMutation
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Draft == nil || request.Draft.Sources[0].Hash != index.Files[0].ContentHash {
					t.Error("advice did not capture the current hash")
				}
				item := memory
				item.ID, item.Version, item.State = externalTestID, request.Version+1, "candidate"
				item.Text, item.Title, item.Sources = request.Draft.Text, request.Draft.Title, request.Draft.Sources
				_ = json.NewEncoder(w).Encode(item)
			})
			s.ExecuteCommand(":memory-" + operation + " decision 30 Revised choice | Revised advice")
			if s.Confirmation != nil || writes.Load() != 0 {
				t.Fatal("advice retained before source capture")
			}
			model, cmd := libraryPending(t, t.Context(), s, cl)
			model.Update(cmd())
			if s.Confirmation == nil || reads.Load() != 1 || writes.Load() != 0 || !strings.Contains(s.Confirmation.Message, index.Files[0].ContentHash) {
				t.Fatalf("capture did not produce exact review: %s", s.Error)
			}
			s.HandleKey(KeyEvent{Key: "Enter"})
			model, cmd = libraryPending(t, t.Context(), s, cl)
			model.Update(cmd())
			if writes.Load() != 1 || s.AgentLibrary.Memory == nil || s.AgentLibrary.Memory.State != "candidate" || s.Error != "" {
				t.Fatalf("retention result was lost or auto-accepted: %s", s.Error)
			}
		})
	}
}

func TestLibraryAdviceCaptureRejectsChangedContextAndUnavailableSources(t *testing.T) {
	for _, change := range []string{"file", "scope", "sources"} {
		t.Run(change, func(t *testing.T) {
			s := libraryTestState()
			s.captureLibraryAdvice("create", "decision 30 Choice | Advice")
			action := *s.PendingActions[0].AgentLibrary
			s.PendingActions = nil
			index := *s.Index
			if change == "sources" {
				index.Files = nil
			}
			s.applyLibraryResult(agentLibraryResult{Action: action, Index: &index})
			if change == "sources" {
				if s.Confirmation != nil || s.Error == "" {
					t.Fatal("uncaptured advice was offered for retention")
				}
				return
			}
			if s.Confirmation == nil {
				t.Fatal("missing capture confirmation", s.Error)
			}
			if change == "file" {
				s.File.Path = "different.go"
			} else {
				s.Project.ProjectRevision = "new-revision"
			}
			s.HandleKey(KeyEvent{Key: "Enter"})
			if len(s.PendingActions) != 0 || s.Error == "" {
				t.Fatal("stale source capture dispatched")
			}
		})
	}
	s := libraryTestState()
	s.openTaskPlan(&ChangeSeed{Title: "Change", Paths: []string{"main.go"}, Plan: &ChangePlanInput{Problem: "Check", Approach: "Inspect"}})
	s.captureLibraryAdvice("create", "lesson 14 Plan advice | Add a boundary case")
	a := *s.PendingActions[0].AgentLibrary
	s.PendingActions = nil
	s.applyLibraryResult(agentLibraryResult{Action: a, Index: s.Index})
	if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, "Uncaptured planned paths") || !strings.Contains(s.Confirmation.Message, "main_test.go") {
		t.Fatalf("missing planned path was not disclosed: %s", s.Error)
	}
}

func TestLibraryRetainsProposalIdentityAndDerivesReceiptText(t *testing.T) {
	for _, selection := range []string{"outcome", "lesson:1"} {
		t.Run(selection, func(t *testing.T) {
			s := libraryTestState()
			s.Change = &ChangeSession{ProjectIdentity: s.Project.ProjectIdentity, ID: "proposal", Hash: "proposal-hash", Revision: 3, Targets: []ChangeTarget{{Path: "main.go", Hash: s.File.ContentHash, Exists: true}}, Checks: []Check{{Name: "unit", State: "passed"}}, Workflow: &ChangeWorkflow{ID: "workflow", HarnessLessons: []string{"Retained host lesson"}}}
			s.retainLibraryProposal(selection + " 30 Proposal evidence")
			if len(s.PendingActions) != 1 {
				t.Fatal("receipt capture not queued", s.Error)
			}
			action := *s.PendingActions[0].AgentLibrary
			s.PendingActions = nil
			s.applyLibraryResult(agentLibraryResult{Action: action, Index: s.Index})
			if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, s.Change.Hash) {
				t.Fatal("proposal identity missing from retention review", s.Error)
			}
			s.HandleKey(KeyEvent{Key: "Enter"})
			draft := s.PendingActions[0].AgentLibrary.Memory.Draft
			if draft.Text != "" || draft.SessionID != s.Change.ID || draft.Identity == nil || draft.Identity.Hash != s.Change.Hash || draft.Identity.Revision != 3 {
				t.Fatal("retention bypassed host-derived proposal text")
			}
			s.Change.Hash = "revised"
			if s.prepareLibraryAction(s.PendingActions[0].AgentLibrary) {
				t.Fatal("queued retention accepted a changed proposal")
			}
		})
	}
}

func TestLibraryRefreshPreservesHealthyGroupsSelectionAndCatalogSnapshot(t *testing.T) {
	s := libraryTestState()
	discloseLibraryMemory(s)
	item := s.AgentLibrary.Memories[0]
	other := item
	other.ID = "zyxwvutsrqponmlkjihgfedcba"
	s.AgentLibrary.Memories = append(s.AgentLibrary.Memories, other)
	s.SelectedIdx = 1
	s.refreshLibrary()
	action := *s.PendingActions[0].AgentLibrary
	s.PendingActions = nil
	s.applyLibraryResult(agentLibraryResult{Action: action, Memories: []client.AgentMemory{other, item}, Failures: map[string]error{libraryArtifacts: fmt.Errorf("catalog unavailable")}})
	if s.selectedLibraryKey().ID != other.ID || !s.AgentLibrary.Loaded[libraryMemory] || s.AgentLibrary.Failures[libraryArtifacts] == "" || s.AgentLibrary.Memory != nil {
		t.Fatal("partial refresh displaced healthy selection or retained private disclosure")
	}
	s = libraryTestState()
	s.AgentLibrary.Artifacts.NextCursor = "next"
	s.pageLibraryArtifacts(true)
	action = *s.PendingActions[0].AgentLibrary
	s.PendingActions = nil
	page := *s.AgentLibrary.Artifacts
	page.Snapshot = strings.Repeat("c", 64)
	s.applyLibraryResult(agentLibraryResult{Action: action, Artifacts: &page})
	if s.AgentLibrary.Failures[libraryArtifacts] == "" {
		t.Fatal("pagination crossed catalog snapshots")
	}
}

func TestLibraryCancellationStopsTransportAndCannotPublishLateMutation(t *testing.T) {
	s := libraryTestState()
	s.confirmArtifactControl("delete")
	s.HandleKey(KeyEvent{Key: "Enter"})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started, stopped := make(chan struct{}), make(chan struct{})
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(stopped)
	})
	model, cmd := libraryPending(t, ctx, s, cl)
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("library transport did not start")
	}
	s.cancelLibraryRequest()
	s.FlushActions(ctx, cl)
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("library cancellation did not reach transport")
	}
	select {
	case message := <-done:
		model.Update(message)
	case <-ctx.Done():
		t.Fatal("library request owner leaked")
	}
	if len(s.AgentLibrary.Artifacts.Artifacts) != 1 || s.AgentLibrary.Failures[libraryArtifacts] == "" || s.AgentLibrary.Pending != "" {
		t.Fatal("canceled action fabricated deletion or remained actionable")
	}
	s.confirmArtifactControl("delete")
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("canceled mutation was retried before refresh")
	}
}

func TestLibraryViewsRemainBoundedSanitizedAndPassive(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 36}} {
			for _, page := range []Page{PageArtifacts, PageMemory} {
				s := libraryTestState()
				s.Palette = palette
				if page == PageArtifacts {
					s.AgentLibrary.Artifact = &client.ArtifactDetail{AgentArtifact: s.AgentLibrary.Artifacts.Artifacts[0], PreviewKind: "text", Text: "unsafe\x1b[2J\n" + strings.Repeat("read-only retained text ", 50) + "end-of-library"}
				} else {
					discloseLibraryMemory(s)
					s.AgentLibrary.Memory.Text = "unsafe\x1b[2J\n" + strings.Repeat("advisory text ", 50) + "end-of-library"
				}
				s.Navigate(page)
				var visited strings.Builder
				for range 40 {
					frame := s.Render(size[0], size[1])
					visited.WriteString(frame)
					for _, line := range strings.Split(frame, "\n") {
						if theme.VisibleRuneCount(line) != size[0] {
							t.Fatalf("library overflow %s %v: %q", page, size, line)
						}
					}
					s.HandleKey(KeyEvent{Key: "PageDown"})
				}
				for _, token := range []string{"SELECTED RECORD", "Checkout:", "/repo", "end-of-library"} {
					if !strings.Contains(visited.String(), token) {
						t.Fatalf("library detail inaccessible: %s %s %v", token, page, size)
					}
				}
				if strings.Contains(visited.String(), "\x1b[2J") || len(s.PendingActions) != 0 || s.Confirmation != nil {
					t.Fatal("passive library browsing dispatched or emitted terminal controls")
				}
			}
		}
	}
}

func TestLibraryPagesDoNotInheritSourceMutationControls(t *testing.T) {
	for _, page := range []Page{PageArtifacts, PageMemory} {
		s := libraryTestState()
		s.Change = &ChangeSession{ID: "proposal", State: "checked", Hash: "proposal-hash", ProjectIdentity: s.Project.ProjectIdentity}
		s.Navigate(page)
		for _, key := range []string{"c", "u", "g", "ArrowRight", "ArrowLeft"} {
			s.HandleKey(KeyEvent{Key: key})
		}
		if len(s.PendingActions) != 0 || s.Confirmation != nil {
			t.Fatalf("%s inherited proposal mutations", page)
		}
		s.ExecuteCommand(":library-hide")
		if s.AgentLibrary.Memory != nil || s.AgentLibrary.Artifact != nil {
			t.Fatal("hide left private content visible")
		}
	}
}
