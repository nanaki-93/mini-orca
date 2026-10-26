package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

const attachmentAnswer = `{"modelVersion":"gemini-2.5-flash","candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"PRIVATE_ADVISORY_ANSWER"}]}}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":3,"totalTokenCount":11}}`

func attachmentAdviceFixture(t *testing.T, endpoint string) (*Service, AttachmentAdviceRequest, *AgentArtifact) {
	t.Helper()
	s, upload := attachmentImportFixture(t, endpoint)
	item, err := s.ImportAttachment(t.Context(), upload)
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.AttachmentOverview(t.Context(), attachmentQuery(upload))
	if err != nil {
		t.Fatal(err)
	}
	return s, AttachmentAdviceRequest{ProjectID: upload.ProjectID, ProjectRevision: upload.ProjectRevision, Configuration: view.Configuration, SettingsVersion: view.SettingsVersion, Model: view.Model.Model, ProviderOrigin: view.Model.ProviderOrigin, Question: "PRIVATE_ADVISORY_QUESTION", Attachments: []AttachmentReference{{ID: item.ID, Version: item.Version, Hash: item.Hash}}, ConfirmSourceSharing: true, ConfirmRemoteProvider: true}, item
}

func TestAttachmentAdvicePublishesOnlyPrivateAdvisoryArtifact(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		data, _ := io.ReadAll(r.Body)
		if !bytes.Contains(data, []byte(`"inlineData"`)) || bytes.Contains(data, []byte("fileUri")) || !bytes.Contains(data, []byte("PRIVATE_ADVISORY_QUESTION")) {
			t.Error("media was not explicitly inline")
		}
		io.WriteString(w, attachmentAnswer)
	}))
	defer provider.Close()
	s, request, item := attachmentAdviceFixture(t, provider.URL)
	before, err := os.ReadFile(filepath.Join(s.manager.Root(), "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.RunAttachmentAdvice(t.Context(), request)
	if err != nil || result.State != "advisory" || !result.UsageKnown || result.Usage.TotalTokens != 11 || calls.Load() != 1 {
		t.Fatalf("advice: %+v %v calls=%d", result, err, calls.Load())
	}
	metadata, _ := json.Marshal(result)
	if bytes.Contains(metadata, []byte("PRIVATE_")) {
		t.Fatal("private content escaped advice receipt")
	}
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	detail, err := s.ArtifactDetail(t.Context(), query, result.Artifact.ID)
	if err != nil || !strings.Contains(detail.Text, "MODEL ADVICE") || !strings.Contains(detail.Text, "PRIVATE_ADVISORY_ANSWER") || !strings.Contains(detail.Text, item.Hash) {
		t.Fatalf("private advice: %+v %v", detail, err)
	}
	view, err := s.AttachmentOverview(t.Context(), query)
	if err != nil || len(view.Attachments) != 1 || calls.Load() != 1 {
		t.Fatalf("passive inventory: %+v %v", view, err)
	}
	history, err := s.ListAgentRuns(t.Context(), query)
	if err != nil || len(history.Runs) != 1 || history.Runs[0].Usage.ModelCalls != 1 {
		t.Fatalf("native history: %+v %v", history, err)
	}
	metadata, _ = json.Marshal(history)
	if bytes.Contains(metadata, []byte("PRIVATE_")) {
		t.Fatal("private content escaped run history")
	}
	after, err := os.ReadFile(filepath.Join(s.manager.Root(), "main.go"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("advice changed project source")
	}
}

func TestAttachmentAdviceAdmissionRejectsChangedOrUnsupportedSelections(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); io.WriteString(w, attachmentAnswer) }))
	defer provider.Close()
	for _, kind := range []string{"sharing", "remote", "configuration", "version", "model", "origin", "hash", "artifact version", "duplicate", "missing", "question", "profile", "deleted", "policy", "settings", "budget", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			s, request, item := attachmentAdviceFixture(t, provider.URL)
			ctx := t.Context()
			switch kind {
			case "sharing":
				request.ConfirmSourceSharing = false
			case "remote":
				request.ConfirmRemoteProvider = false
				s.runtimes.function.effective.RemoteProvider = true
			case "configuration":
				request.Configuration = "other"
			case "version":
				request.SettingsVersion++
			case "model":
				request.Model = "other"
			case "origin":
				request.ProviderOrigin = "https://other.example"
			case "hash":
				request.Attachments[0].Hash = strings.Repeat("a", 64)
			case "artifact version":
				request.Attachments[0].Version++
			case "duplicate":
				request.Attachments = append(request.Attachments, request.Attachments[0])
			case "missing":
				request.Attachments[0].ID = strings.Repeat("f", 64)
			case "question":
				request.Question = " "
			case "profile":
				s.runtimes.function.profile.Provider = config.OpenAIProvider
			case "deleted":
				_, err := s.ControlArtifact(ctx, item.ID, ArtifactControl{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Action: "delete", Version: item.Version, Hash: item.Hash})
				if err != nil {
					t.Fatal(err)
				}
			case "policy":
				if err := os.WriteFile(filepath.Join(s.manager.Root(), ".mini-orca", "context-policy.json"), []byte(`{"exclude":["main.go"]}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "settings":
				_, err := s.ControlAgentSettings(ctx, AgentSettingsControl{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Configuration: request.Configuration, Version: request.SettingsVersion, Action: "reset", Confirm: true})
				if err != nil {
					t.Fatal(err)
				}
			case "budget":
				s.agentRuntime.Limits.InputTokens = 1
				view, err := s.AttachmentOverview(ctx, AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision})
				if err != nil {
					t.Fatal(err)
				}
				request.Configuration = view.Configuration
			case "canceled":
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			if value, err := s.RunAttachmentAdvice(ctx, request); err == nil || value != nil {
				t.Fatalf("invalid admission: %+v %v", value, err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("rejected requests dispatched: %d", calls.Load())
	}
}

func TestAttachmentAdviceRejectsUnimportedAndForeignArtifactAuthority(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); io.WriteString(w, attachmentAnswer) }))
	defer provider.Close()
	s, request, _ := attachmentAdviceFixture(t, provider.URL)
	store, err := agentstate.Open(s.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"foreign", "ordinary artifact", "protected", "old revision"} {
		key := attachmentSession(request.ProjectID)
		projectID, revision := request.ProjectID, request.ProjectRevision
		if kind == "foreign" {
			projectID = "foreign"
		}
		if kind == "ordinary artifact" {
			key.ID = "another_session"
		}
		if kind == "old revision" {
			revision = "old"
		}
		_, binding, err := s.agentJournal(s.manager.Root())
		if err != nil {
			t.Fatal(err)
		}
		record, err := store.SaveArtifact(t.Context(), agentstate.ArtifactWrite{Key: agentstate.ArtifactKey{Session: key, Name: "evidence"}, Kind: "inline", MIME: "application/pdf", Data: []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n%%EOF"), DisplayName: "evidence.pdf", Protected: kind == "protected", Metadata: map[string]string{"kind": "attachment-v1"}, Provenance: map[string]string{"project": projectID, "revision": revision, "policy": binding.Policy}})
		if err != nil {
			t.Fatal(err)
		}
		request.Attachments = []AttachmentReference{{ID: artifactLibraryID(*record), Version: record.Version, Hash: record.Hash}}
		if value, err := s.RunAttachmentAdvice(t.Context(), request); err == nil || value != nil {
			t.Fatalf("invalid %s accepted: %+v %v", kind, value, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("unowned artifacts reached provider: %d", calls.Load())
	}
}

func TestAttachmentAdviceLateChangesAndStorageFailuresDoNotPublish(t *testing.T) {
	for _, kind := range []string{"settings", "delete", "policy", "storage", "provider"} {
		t.Run(kind, func(t *testing.T) {
			var s *Service
			var request AttachmentAdviceRequest
			var item *AgentArtifact
			var calls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				switch kind {
				case "settings":
					if _, err := s.ControlAgentSettings(t.Context(), AgentSettingsControl{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Configuration: request.Configuration, Version: request.SettingsVersion, Action: "reset", Confirm: true}); err != nil {
						t.Error(err)
					}
				case "delete":
					if _, err := s.ControlArtifact(t.Context(), item.ID, ArtifactControl{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Action: "delete", Version: item.Version, Hash: item.Hash}); err != nil {
						t.Error(err)
					}
				case "policy":
					if err := os.WriteFile(filepath.Join(s.manager.Root(), ".mini-orca", "context-policy.json"), []byte(`{"exclude":["main.go"]}`), 0600); err != nil {
						t.Error(err)
					}
				case "storage":
					paths, err := filepath.Glob(filepath.Join(s.manager.Root(), ".mini-orca", "agents", "*", "artifacts.json"))
					if err != nil || len(paths) != 1 {
						t.Error("catalog missing")
						return
					}
					if err := os.WriteFile(paths[0], []byte("preserve corrupt media"), 0600); err != nil {
						t.Error(err)
					}
				case "provider":
					w.WriteHeader(http.StatusServiceUnavailable)
					io.WriteString(w, "PRIVATE_FAILURE_BODY")
					return
				}
				io.WriteString(w, attachmentAnswer)
			}))
			defer provider.Close()
			s, request, item = attachmentAdviceFixture(t, provider.URL)
			value, err := s.RunAttachmentAdvice(t.Context(), request)
			if err == nil || value != nil || calls.Load() != 1 || strings.Contains(err.Error(), "PRIVATE_FAILURE_BODY") {
				t.Fatalf("late outcome published: %+v %v calls=%d", value, err, calls.Load())
			}
			if kind == "storage" {
				return
			}
			store, openErr := agentstate.Open(s.manager.Root())
			if openErr != nil {
				t.Fatal(openErr)
			}
			entries, readErr := store.ArtifactInventory(t.Context())
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, entry := range entries {
				if entry.Metadata["kind"] == "attachment-advice-v1" {
					t.Fatal("failed result published as advice")
				}
			}
		})
	}
}

func TestAttachmentAdviceLifecycleCancellationJoinsProviderWork(t *testing.T) {
	for _, action := range []string{"cancel", "reindex", "replace", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			entered := make(chan struct{})
			provider := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				close(entered)
				<-r.Context().Done()
			}))
			defer provider.Close()
			s, request, _ := attachmentAdviceFixture(t, provider.URL)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := s.RunAttachmentAdvice(ctx, request); done <- err }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("provider not entered")
			}
			switch action {
			case "cancel":
				cancel()
			case "reindex":
				if _, err := s.Reindex(); err != nil {
					t.Fatal(err)
				}
			case "replace":
				s.invalidateJobsForProjectChange(stopReasonProjectChanged)
			case "shutdown":
				stop, release := context.WithTimeout(t.Context(), 5*time.Second)
				defer release()
				if err := s.ShutdownChangeWorkflows(stop); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if err == nil || action == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("unexpected cancellation: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("owned attachment request leaked")
			}
		})
	}
}
