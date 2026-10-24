package agentruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/server/adka2a/v2"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

type remoteFixture struct {
	server *httptest.Server
	card   a2a.AgentCard
	calls  atomic.Int32
	rpc    func(http.ResponseWriter, *http.Request, string, json.RawMessage, json.RawMessage)
}

func newRemoteFixture(t *testing.T) *remoteFixture {
	t.Helper()
	f := &remoteFixture{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer fixture-key" || r.Header.Get("A2A-Version") != "1.0" {
			t.Error("missing explicit protocol/auth headers")
		}
		if r.URL.Path == "/card" {
			json.NewEncoder(w).Encode(f.card)
			return
		}
		if r.URL.Path != "/rpc" {
			t.Error("unexpected URL", r.URL)
			w.WriteHeader(404)
			return
		}
		f.calls.Add(1)
		var envelope struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			t.Error(err)
			return
		}
		f.rpc(w, r, envelope.Method, envelope.ID, envelope.Params)
	}))
	f.card = a2a.AgentCard{Name: "fixture", Description: "Advisory fixture", Version: "1", SupportedInterfaces: []*a2a.AgentInterface{a2a.NewAgentInterface(f.server.URL+"/rpc", a2a.TransportProtocolJSONRPC)}, Capabilities: a2a.AgentCapabilities{Streaming: true}, DefaultInputModes: []string{"text/plain"}, DefaultOutputModes: []string{"text/plain"}, Skills: []a2a.AgentSkill{{ID: "review", Name: "Review"}}, IconURL: f.server.URL + "/never-fetch-icon"}
	t.Cleanup(f.server.Close)
	return f
}

func (f *remoteFixture) connection() RemoteConnection {
	return RemoteConnection{CardURL: f.server.URL + "/card", Endpoint: f.server.URL + "/rpc", BearerToken: "fixture-key", Skills: []string{"review"}, Timeout: 5 * time.Second}
}
func remoteAllow(context.Context) error                           { return nil }
func remoteSave(context.Context, agentstate.RemoteProgress) error { return nil }
func remoteRequest() RemoteRequest {
	return RemoteRequest{DelegationID: "delegation", AttemptID: "attempt", MessageID: "stored-message", Operation: "start", Kind: "review", Skill: "review", Prompt: "only admitted source", Previous: agentstate.RemoteProgress{State: "unknown", Usage: agentstate.RemoteUsage{Source: "unknown"}}}
}
func remoteTask(state a2a.TaskState) *a2a.Task {
	return &a2a.Task{ID: "remote-task", ContextID: "remote-context", Status: a2a.TaskStatus{State: state}}
}
func remoteReply(w http.ResponseWriter, id json.RawMessage, value any, stream bool) {
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": value})
	if stream {
		fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
	} else {
		w.Write(data)
	}
}

func TestRemoteNativeSendAndExplicitRecoveryNeverReplay(t *testing.T) {
	for _, operation := range []string{"start", "refresh", "reconnect", "cancel", "input"} {
		t.Run(operation, func(t *testing.T) {
			f := newRemoteFixture(t)
			request := remoteRequest()
			request.Operation = operation
			if operation != "start" {
				request.Previous.TaskID, request.Previous.ContextID, request.Previous.State = "remote-task", "remote-context", "input_required"
			}
			f.rpc = func(w http.ResponseWriter, r *http.Request, method string, id, params json.RawMessage) {
				if method != remoteMethod(request) {
					t.Errorf("method=%s want=%s", method, remoteMethod(request))
				}
				if operation == "start" || operation == "input" {
					var sent a2a.SendMessageRequest
					if err := json.Unmarshal(params, &sent); err != nil {
						t.Fatal(err)
					}
					if sent.Message.ID != request.MessageID || len(sent.Message.Metadata) != 0 || len(sent.Message.Parts) != 1 || !strings.HasSuffix(string(sent.Message.Parts[0].Content.(a2a.Text)), request.Prompt) {
						t.Error("unexpected sent context", string(params))
					}
				}
				task := remoteTask(a2a.TaskStateCompleted)
				switch operation {
				case "refresh", "cancel":
					remoteReply(w, id, task, false)
				case "reconnect":
					w.Header().Set("Content-Type", "text/event-stream")
					remoteReply(w, id, a2a.StreamResponse{Event: task}, true)
				default:
					remoteReply(w, id, a2a.StreamResponse{Event: task}, false)
				}
			}
			ctx, budget, _ := persistedContext(t, t.TempDir())
			inventory, err := DiscoverRemote(ctx, f.connection(), remoteAllow)
			if err != nil {
				t.Fatal(err)
			}
			result, err := RunRemote(ctx, f.connection(), inventory, request, remoteAllow, remoteSave)
			if err != nil || !result.Attempted || result.Progress.State != "completed" || result.Progress.TaskID != "remote-task" || result.Progress.Usage.Source != "unknown" {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			if f.calls.Load() != 1 || budget.Snapshot().ToolCalls != 3 || budget.Snapshot().ModelCalls != 0 {
				t.Fatalf("attempt/budget=%d %+v", f.calls.Load(), budget.Snapshot())
			}
		})
	}
}

func TestRemoteChangedCardAndMalformedRPC(t *testing.T) {
	for _, kind := range []string{"changed", "wrong-id", "duplicate", "auth"} {
		t.Run(kind, func(t *testing.T) {
			f := newRemoteFixture(t)
			f.rpc = func(w http.ResponseWriter, _ *http.Request, _ string, id, _ json.RawMessage) {
				switch kind {
				case "auth":
					w.WriteHeader(401)
				case "wrong-id":
					remoteReply(w, json.RawMessage(`"other"`), a2a.StreamResponse{Event: remoteTask(a2a.TaskStateCompleted)}, false)
				default:
					fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{},"result":{}}`, id)
				}
			}
			ctx, _ := budgetContext(t, nil)
			inventory, err := DiscoverRemote(ctx, f.connection(), remoteAllow)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "changed" {
				f.card.Version = "2"
			}
			result, err := RunRemote(ctx, f.connection(), inventory, remoteRequest(), remoteAllow, remoteSave)
			if err == nil {
				t.Fatal("invalid exchange succeeded")
			}
			if kind == "changed" && (result.Attempted || f.calls.Load() != 0) {
				t.Fatal("changed card dispatched")
			}
			if kind == "auth" && !errors.Is(err, ErrRemoteAuth) {
				t.Fatal(err)
			}
		})
	}
}

func TestRemoteCancellationJoinsStreamAndSavesRemoteCancel(t *testing.T) {
	f := newRemoteFixture(t)
	ctx, _, _ := persistedContext(t, t.TempDir())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	closed := make(chan struct{})
	f.rpc = func(w http.ResponseWriter, r *http.Request, method string, id, _ json.RawMessage) {
		if method == "CancelTask" {
			remoteReply(w, id, remoteTask(a2a.TaskStateCanceled), false)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		remoteReply(w, id, a2a.StreamResponse{Event: remoteTask(a2a.TaskStateWorking)}, true)
		<-r.Context().Done()
		close(closed)
	}
	inventory, err := DiscoverRemote(ctx, f.connection(), remoteAllow)
	if err != nil {
		t.Fatal(err)
	}
	request := remoteRequest()
	request.Stream = true
	save := func(_ context.Context, progress agentstate.RemoteProgress) error {
		if progress.State == "working" {
			cancel()
		}
		return nil
	}
	result, err := RunRemote(ctx, f.connection(), inventory, request, remoteAllow, save)
	if !errors.Is(err, context.Canceled) || result.Progress.State != "canceled" || f.calls.Load() != 2 {
		t.Fatalf("cleanup=%+v %v calls=%d", result, err, f.calls.Load())
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("stream not joined")
	}
}

func TestRemoteStreamRejectsDuplicateJSONAndUnfinishedFrame(t *testing.T) {
	for _, input := range []string{"data: {\"id\":\"x\",\"id\":\"x\"}\n\n", "data: {}\n", strings.Repeat("x", 65537)} {
		response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(input))}
		guarded, err := guardRemoteResponse(response, true, "x")
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.ReadAll(guarded.Body)
		guarded.Body.Close()
		if !errors.Is(err, ErrRemoteProtocol) {
			t.Fatal("malformed stream accepted", err)
		}
	}
}

func TestRemoteConsumesNativeADKArtifactModes(t *testing.T) {
	for _, mode := range []adka2a.OutputMode{adka2a.OutputArtifactPerRun, adka2a.OutputArtifactPerEvent} {
		t.Run(string(mode), func(t *testing.T) {
			worker, err := agent.New(agent.Config{Name: "native_fixture", Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
				return func(yield func(*session.Event, error) bool) {
					event := session.NewEvent(ctx, ctx.InvocationID())
					event.Author = ctx.Agent().Name()
					event.Content = &genai.Content{Role: "model", Parts: []*genai.Part{genai.NewPartFromText("Native advisory")}}
					event.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 3, CandidatesTokenCount: 4}
					yield(event, nil)
				}
			}})
			if err != nil {
				t.Fatal(err)
			}
			executor := adka2a.NewExecutor(adka2a.ExecutorConfig{RunnerConfig: runner.Config{AppName: "native_fixture", Agent: worker, SessionService: session.InMemoryService()}, OutputMode: mode})
			native := a2asrv.NewJSONRPCHandler(a2asrv.NewHandler(executor))
			f := newRemoteFixture(t)
			f.rpc = func(w http.ResponseWriter, r *http.Request, method string, id, params json.RawMessage) {
				data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "id": id, "params": params})
				if err != nil {
					t.Error(err)
					return
				}
				r.Body, r.ContentLength = io.NopCloser(bytes.NewReader(data)), int64(len(data))
				native.ServeHTTP(w, r)
			}
			ctx, _, _ := persistedContext(t, t.TempDir())
			inventory, err := DiscoverRemote(ctx, f.connection(), remoteAllow)
			if err != nil {
				t.Fatal(err)
			}
			request := remoteRequest()
			request.Stream = true
			result, err := RunRemote(ctx, f.connection(), inventory, request, remoteAllow, remoteSave)
			if err != nil || result.Progress.State != "completed" || len(result.Progress.Artifacts) != 1 {
				t.Fatalf("native artifacts: %+v %v", result, err)
			}
			artifact := result.Progress.Artifacts[0]
			if artifact.Text != "Native advisory" || !artifact.Complete || result.Progress.Usage.Source != "remote_reported" || result.Progress.Usage.InputTokens != 3 {
				t.Fatalf("native evidence: %+v", result.Progress)
			}
		})
	}
}
