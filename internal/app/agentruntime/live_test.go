package agentruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"google.golang.org/genai"
)

func liveProvider(t *testing.T, handle func(*websocket.Conn, *http.Request)) *ProviderClient {
	t.Helper()
	var mu sync.Mutex
	var sockets []*websocket.Conn
	var workers sync.WaitGroup
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workers.Add(1)
		defer workers.Done()
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		sockets = append(sockets, conn)
		mu.Unlock()
		defer conn.Close()
		handle(conn, r)
	}))
	t.Cleanup(func() {
		server.Close()
		mu.Lock()
		for _, c := range sockets {
			_ = c.Close()
		}
		mu.Unlock()
		workers.Wait()
	})
	return NewProviderClient(config.ModelProfile{Provider: config.GeminiProvider, APIBaseURL: server.URL, APIKey: "PRIVATE_KEY", Model: "gemini-3.8-live", MaxTokens: 128})
}

func acceptLiveSetup(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	var request genai.LiveClientMessage
	if err := conn.ReadJSON(&request); err != nil {
		t.Error(err)
		return
	}
	if request.Setup == nil || request.Setup.Model != "models/gemini-3.8-live" || request.Setup.SessionResumption != nil || len(request.Setup.Tools) != 0 || request.Setup.OutputAudioTranscription == nil || request.Setup.RealtimeInputConfig == nil || !request.Setup.RealtimeInputConfig.AutomaticActivityDetection.Disabled {
		t.Error("invalid explicit setup")
	}
	if err := conn.WriteJSON(map[string]any{"setupComplete": map[string]any{}}); err != nil {
		t.Error(err)
	}
}

func liveOptions() LiveOptions {
	return LiveOptions{ID: "live-test", Connection: 1, Authorize: func(context.Context) error { return nil }}
}
func allowLive(context.Context) error { return nil }

func waitLive(t *testing.T, connection *LiveConnection) error {
	t.Helper()
	select {
	case <-connection.Done():
		return connection.Wait()
	case <-time.After(5 * time.Second):
		connection.Close()
		t.Fatal("live connection did not join")
		return nil
	}
}

func TestLiveUsesADKRunnerPrivateArtifactsAndConservativeAccounting(t *testing.T) {
	client := liveProvider(t, func(conn *websocket.Conn, r *http.Request) {
		if r.URL.Path != liveSocketPath || r.URL.RawQuery != "" || r.Header.Get("X-Goog-Api-Key") != "PRIVATE_KEY" || r.Header.Get("Authorization") != "" {
			t.Error("changed route or credential")
		}
		acceptLiveSetup(t, conn)
		var request genai.LiveClientMessage
		if err := conn.ReadJSON(&request); err != nil {
			t.Error(err)
			return
		}
		if request.ClientContent == nil || !request.ClientContent.TurnComplete || request.ClientContent.Turns[0].Parts[0].Text != "PRIVATE_LIVE_QUESTION" {
			t.Error("missing explicit input")
		}
		_ = conn.WriteJSON(map[string]any{"serverContent": map[string]any{"modelTurn": map[string]any{"role": "model", "parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": "audio/pcm;rate=24000", "data": []byte{1, 0, 2, 0}}}}}, "outputTranscription": map[string]any{"text": "PRIVATE_LIVE_ANSWER"}, "turnComplete": true}, "usageMetadata": map[string]any{"promptTokenCount": 7, "responseTokenCount": 3, "totalTokenCount": 10}})
		_ = conn.WriteJSON(map[string]any{"goAway": map[string]any{"timeLeft": "1s"}})
	})
	ctx, budget, store := persistedContext(t, t.TempDir())
	connection, err := OpenLive(ctx, client, liveOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Send(t.Context(), LiveInput{Text: "PRIVATE_LIVE_QUESTION"}, allowLive); err != nil {
		t.Fatal(err)
	}
	if err := waitLive(t, connection); !errors.Is(err, ErrLiveGoAway) {
		t.Fatal(err)
	}
	snapshot := connection.Snapshot()
	if snapshot.State != "go_away" || snapshot.Turns != 1 || snapshot.CompletedTurns != 1 || len(snapshot.Artifacts) != 2 || !snapshot.Reported || snapshot.ReportedInput != 7 || snapshot.ReportedOutput != 3 {
		t.Fatalf("snapshot: %+v", snapshot)
	}
	data, _ := json.Marshal(snapshot)
	if bytes.Contains(data, []byte("PRIVATE_")) {
		t.Fatal("private media escaped receipt")
	}
	usage := budget.Snapshot()
	if usage.ModelCalls != 1 || usage.ActiveCalls != 0 || usage.UnknownUsageCalls != 1 || usage.InputTokens != LiveMaxInputBytes+liveHeaderBytes || usage.OutputTokens != LiveMaxOutputBytes+liveHeaderBytes {
		t.Fatalf("accounting: %+v", usage)
	}
	var transcript string
	for _, ref := range snapshot.Artifacts {
		item, err := store.ExportArtifact(t.Context(), agentstate.ArtifactKey{Session: ref.Session, Name: ref.Name}, ref.Version)
		if err != nil {
			t.Fatal(err)
		}
		if item.Kind == "text" {
			transcript = string(item.Data)
		}
	}
	if !strings.Contains(transcript, "PRIVATE_LIVE_QUESTION") || !strings.Contains(transcript, "PRIVATE_LIVE_ANSWER") || !strings.Contains(transcript, "not verified") {
		t.Fatal("private transcript missing")
	}
	history, err := store.History(t.Context())
	if err != nil || len(history.Runs) != 1 || history.Runs[0].SchemaVersion != "live-v1" {
		t.Fatalf("ADK journal: %+v %v", history, err)
	}
	data, _ = json.Marshal(history)
	if bytes.Contains(data, []byte("PRIVATE_")) {
		t.Fatal("private text escaped history")
	}
	sessions, err := store.Sessions(t.Context(), "mini-orca", "local")
	if err != nil || len(sessions) != 1 || len(sessions[0].Events) < 2 {
		t.Fatalf("ADK events: %+v %v", sessions, err)
	}
}

func TestLiveCancellationAndRevokedSendCloseOwnedConnection(t *testing.T) {
	for _, kind := range []string{"cancel", "revoked", "close"} {
		t.Run(kind, func(t *testing.T) {
			closed := make(chan struct{})
			client := liveProvider(t, func(conn *websocket.Conn, _ *http.Request) {
				defer close(closed)
				acceptLiveSetup(t, conn)
				_, _, _ = conn.ReadMessage()
			})
			ctx, budget := budgetContext(t, nil)
			owned, cancel := context.WithCancel(ctx)
			defer cancel()
			connection, err := OpenLive(owned, client, liveOptions())
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "cancel":
				cancel()
			case "close":
				connection.Close()
			case "revoked":
				if err := connection.Send(t.Context(), LiveInput{Text: "DO_NOT_SEND"}, func(context.Context) error { return ErrPolicyDenied }); !errors.Is(err, ErrPolicyDenied) {
					t.Fatal(err)
				}
			}
			if err := waitLive(t, connection); err == nil {
				t.Fatal("missing close cause")
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("socket outlived cancellation")
			}
			if usage := budget.Snapshot(); usage.ActiveCalls != 0 || usage.UnknownUsageCalls != 1 {
				t.Fatalf("usage: %+v", usage)
			}
		})
	}
}

func TestLiveRefusesUnadmittedModelsAndExhaustedBudgetsBeforeConnect(t *testing.T) {
	var calls atomic.Int32
	client := liveProvider(t, func(*websocket.Conn, *http.Request) { calls.Add(1) })
	for _, kind := range []string{"model", "provider", "authority", "budget", "binding", "credential", "URL", "capacity"} {
		t.Run(kind, func(t *testing.T) {
			copy := NewProviderClient(client.profile)
			options := liveOptions()
			ctx, _ := budgetContext(t, func(l *config.AgentLimits) {
				if kind == "capacity" {
					l.InputTokens = 1
				}
			})
			switch kind {
			case "model":
				copy.profile.Model = "gemini-2.5-flash"
			case "provider":
				copy.profile.Provider = config.VertexProvider
			case "authority":
				options.Authorize = nil
			case "budget":
				ctx = t.Context()
			case "binding":
				var err error
				ctx, err = WithPolicy(ctx, NewProviderClient(config.ModelProfile{Model: "other"}))
				if err != nil {
					t.Fatal(err)
				}
			case "credential":
				copy.profile.APIKey = "auth_tokens/private"
			case "URL":
				copy.profile.APIBaseURL += "?key=secret"
			}
			if value, err := OpenLive(ctx, copy, options); err == nil || value != nil {
				t.Fatalf("accepted %s", kind)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("rejected admission connected")
	}
}

func TestLiveExplicitMediaAndInterruptUseNoURLsOrHistoryReplay(t *testing.T) {
	got := make(chan []byte, 8)
	client := liveProvider(t, func(conn *websocket.Conn, _ *http.Request) {
		acceptLiveSetup(t, conn)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			got <- data
		}
	})
	ctx, _ := budgetContext(t, nil)
	connection, err := OpenLive(ctx, client, liveOptions())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close(); waitLive(t, connection) })
	media := llm.Attachment{Name: "sample.pcm", MIME: llm.PCM16MIME, Data: []byte{1, 0, 2, 0}}
	if err := connection.Send(t.Context(), LiveInput{Media: &media}, allowLive); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"activityStart", "audio", "activityEnd"} {
		select {
		case data := <-got:
			if !bytes.Contains(data, []byte(field)) || bytes.Contains(data, []byte("fileUri")) {
				t.Fatalf("frame: %s", data)
			}
		case <-time.After(time.Second):
			t.Fatal("missing media frame")
		}
	}
	photo := nativeMediaFixtures(t)[1]
	if err := connection.Send(t.Context(), LiveInput{Media: &photo}, allowLive); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"activityStart", "video", "activityEnd"} {
		select {
		case data := <-got:
			if !bytes.Contains(data, []byte(field)) || bytes.Contains(data, []byte("fileUri")) {
				t.Fatalf("image frame: %s", data)
			}
		case <-time.After(time.Second):
			t.Fatal("missing image frame")
		}
	}
	if err := connection.Interrupt(t.Context(), allowLive); err != nil {
		t.Fatal(err)
	}
	data := <-got
	if !bytes.Contains(data, []byte(`"turnComplete":true`)) || bytes.Contains(data, []byte("turns")) {
		t.Fatalf("interrupt: %s", data)
	}
	for _, mime := range []string{"application/pdf", "audio/wav", "image/png"} {
		media.MIME = mime
		if err := connection.Send(t.Context(), LiveInput{Media: &media}, allowLive); err == nil {
			t.Fatalf("accepted %s", mime)
		}
	}
	select {
	case <-got:
		t.Fatal("rejected media dispatched")
	default:
	}
}
