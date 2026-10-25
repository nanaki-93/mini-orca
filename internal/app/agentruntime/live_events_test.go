package agentruntime

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func TestLiveMalformedAndUngrantableEventsEndWithoutPrivateDiagnostics(t *testing.T) {
	for _, input := range []string{
		`{"serverContent":`,
		`{"serverContent":{},"serverContent":{}}`,
		`{"toolCall":{"functionCalls":[{"name":"write_source","args":{"secret":"PRIVATE_FRAME"}}]}}`,
		`{"sessionResumptionUpdate":{"newHandle":"PRIVATE_FRAME","resumable":true}}`,
		`{"serverContent":{"modelTurn":{"role":"user","parts":[{"text":"PRIVATE_FRAME"}]}}}`,
		`{"serverContent":{"modelTurn":{"role":"model","parts":[{"fileData":{"fileUri":"https://example.com/private"}}]}}}`,
		`{"serverContent":{"modelTurn":{"role":"model","parts":[{"inlineData":{"mimeType":"audio/wav","data":"AQI="}}]}}}`,
		`{"usageMetadata":{"promptTokenCount":-1,"responseTokenCount":2,"totalTokenCount":1}}`,
		`{"setupComplete":{}}`,
	} {
		t.Run(input[:min(35, len(input))], func(t *testing.T) {
			client := liveProvider(t, func(conn *websocket.Conn, _ *http.Request) {
				acceptLiveSetup(t, conn)
				_ = conn.WriteMessage(websocket.TextMessage, []byte(input))
			})
			ctx, budget := budgetContext(t, nil)
			connection, err := OpenLive(ctx, client, liveOptions())
			if err != nil {
				t.Fatal(err)
			}
			err = waitLive(t, connection)
			if err == nil || strings.Contains(err.Error(), "PRIVATE_FRAME") || strings.Contains(err.Error(), "https://") {
				t.Fatalf("unsafe error: %v", err)
			}
			if usage := budget.Snapshot(); usage.ActiveCalls != 0 || usage.UnknownUsageCalls != 1 {
				t.Fatalf("accounting: %+v", usage)
			}
		})
	}
}

func TestLiveFrameAndTurnLimitsTerminateConnections(t *testing.T) {
	t.Run("binary", func(t *testing.T) {
		client := liveProvider(t, func(conn *websocket.Conn, _ *http.Request) {
			acceptLiveSetup(t, conn)
			_ = conn.WriteMessage(websocket.BinaryMessage, []byte(`{"serverContent":{}}`))
		})
		ctx, _ := budgetContext(t, nil)
		connection, err := OpenLive(ctx, client, liveOptions())
		if err != nil {
			t.Fatal(err)
		}
		if err := waitLive(t, connection); !errors.Is(err, ErrLiveProtocol) {
			t.Fatal(err)
		}
	})
	t.Run("frame", func(t *testing.T) {
		client := liveProvider(t, func(conn *websocket.Conn, _ *http.Request) {
			acceptLiveSetup(t, conn)
			_ = conn.WriteMessage(websocket.TextMessage, []byte(strings.Repeat(" ", 129<<10)))
		})
		ctx, _ := budgetContext(t, nil)
		connection, err := OpenLive(ctx, client, liveOptions())
		if err != nil {
			t.Fatal(err)
		}
		if err := waitLive(t, connection); !errors.Is(err, ErrBudgetExceeded) {
			t.Fatal(err)
		}
	})
	t.Run("turns", func(t *testing.T) {
		client := liveProvider(t, func(conn *websocket.Conn, _ *http.Request) {
			acceptLiveSetup(t, conn)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		})
		ctx, _ := budgetContext(t, nil)
		connection, err := OpenLive(ctx, client, liveOptions())
		if err != nil {
			t.Fatal(err)
		}
		for range 8 {
			if err := connection.Interrupt(t.Context(), allowLive); err != nil {
				t.Fatal(err)
			}
		}
		if err := connection.Interrupt(t.Context(), allowLive); !errors.Is(err, ErrBudgetExceeded) {
			t.Fatal(err)
		}
		if err := waitLive(t, connection); !errors.Is(err, ErrBudgetExceeded) {
			t.Fatal(err)
		}
	})
}

func TestLiveChecksAuthorityBeforeEveryMediaFrame(t *testing.T) {
	client := liveProvider(t, func(conn *websocket.Conn, _ *http.Request) {
		acceptLiveSetup(t, conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	ctx, _ := budgetContext(t, nil)
	connection, err := OpenLive(ctx, client, liveOptions())
	if err != nil {
		t.Fatal(err)
	}
	checks := 0
	input := LiveInput{Media: &llm.Attachment{Name: "sample.pcm", MIME: llm.PCM16MIME, Data: []byte{1, 2, 3, 4}}}
	if err := connection.Send(t.Context(), input, func(context.Context) error {
		checks++
		if checks == 4 {
			return ErrPolicyDenied
		}
		return nil
	}); !errors.Is(err, ErrPolicyDenied) {
		t.Fatal(err)
	}
	if checks != 4 {
		t.Fatalf("dispatch checks=%d", checks)
	}
	if err := waitLive(t, connection); !errors.Is(err, ErrPolicyDenied) {
		t.Fatal(err)
	}
}

func TestLiveOutputRetentionFailureCancelsRunAndSettlesReservation(t *testing.T) {
	release := make(chan struct{})
	client := liveProvider(t, func(conn *websocket.Conn, _ *http.Request) {
		acceptLiveSetup(t, conn)
		<-release
		_ = conn.WriteJSON(map[string]any{"serverContent": map[string]any{"outputTranscription": map[string]any{"text": "PRIVATE_UNSAVED_OUTPUT"}}})
		_ = conn.WriteJSON(map[string]any{"goAway": map[string]any{"timeLeft": "1s"}})
	})
	root := t.TempDir()
	ctx, budget, _ := persistedContext(t, root)
	connection, err := OpenLive(ctx, client, liveOptions())
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	dirs, err := filepath.Glob(filepath.Join(root, ".mini-orca", "agents", "*"))
	if err != nil || len(dirs) != 1 {
		close(release)
		connection.Close()
		t.Fatalf("journal directory: %v %v", dirs, err)
	}
	if err := os.Mkdir(filepath.Join(dirs[0], "artifacts.json"), 0700); err != nil {
		close(release)
		connection.Close()
		t.Fatal(err)
	}
	close(release)
	if err := waitLive(t, connection); !errors.Is(err, agentstate.ErrPersistence) {
		t.Fatalf("retention failure: %v", err)
	}
	if snapshot := connection.Snapshot(); snapshot.State != "failed" || len(snapshot.Artifacts) != 0 {
		t.Fatalf("failed output published: %+v", snapshot)
	}
	if usage := budget.Snapshot(); usage.ActiveCalls != 0 || usage.UnknownUsageCalls != 1 || ctx.Err() == nil {
		t.Fatalf("unsettled failed run: %+v", usage)
	}
}
