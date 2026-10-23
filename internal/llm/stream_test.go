package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func streamDelta(t *testing.T, delta map[string]any, finish string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"id": "response_1", "model": "fixture",
		"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return "data: " + string(data) + "\n\n"
}

func streamCall(index any, id, name, arguments string) map[string]any {
	return map[string]any{"index": index, "id": id, "type": "function", "function": map[string]any{"name": name, "arguments": arguments}}
}

func streamClient(body string) *Client {
	client := NewClient(testProfile("http://provider.invalid", ""))
	client.transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	return client
}

func TestGenerationStreamAccumulatesTextAndFinalUsage(t *testing.T) {
	body := ": heartbeat\r\n\r\n" + streamDelta(t, map[string]any{"role": "assistant", "content": "hel"}, "") +
		streamDelta(t, map[string]any{"content": "lo"}, "") + streamDelta(t, nil, "stop") +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\n" +
		"data: [DONE]\n\n"
	input := generationInput()
	input.Stream = true
	result, deltas, err := collectGeneration(streamClient(body), context.Background(), input)
	if err != nil || result == nil || result.Message.Content != "hello" || result.FinishReason != "stop" || result.Usage == nil || result.Usage.TotalTokens != 5 || !reflect.DeepEqual(deltas, []string{"hel", "lo"}) {
		t.Fatalf("result=%+v deltas=%v error=%v", result, deltas, err)
	}
}

func TestGenerationStreamPreservesParallelToolIdentity(t *testing.T) {
	body := streamDelta(t, map[string]any{"tool_calls": []any{streamCall(1, "call_b", "read_file", `{"path":`), streamCall(0, "call_a", "read_file", `{"path":`)}}, "") +
		streamDelta(t, map[string]any{"tool_calls": []any{streamCall(0, "", "", `"a.go"}`), streamCall(1, "", "", `"b.go"}`)}}, "tool_calls") + "data: [DONE]\n\n"
	input := generationInput()
	input.Stream = true
	result, deltas, err := collectGeneration(streamClient(body), context.Background(), input)
	if err != nil || result == nil || len(result.Message.ToolCalls) != 2 || len(deltas) != 0 {
		t.Fatalf("result=%+v deltas=%v error=%v", result, deltas, err)
	}
	calls := result.Message.ToolCalls
	if calls[0].ID != "call_a" || calls[1].ID != "call_b" || string(calls[0].Arguments) != `{"path":"a.go"}` || string(calls[1].Arguments) != `{"path":"b.go"}` {
		t.Fatalf("tool identity/order changed: %+v", calls)
	}
}

func TestGenerationStreamNeverPublishesIncompleteOrInvalidResults(t *testing.T) {
	text := streamDelta(t, map[string]any{"content": "partial"}, "")
	stop := streamDelta(t, nil, "stop")
	done := "data: [DONE]\n\n"
	usage := "data: {\"choices\":[],\"usage\":{\"total_tokens\":2}}\n\n"
	for name, body := range map[string]string{
		"truncated":               text,
		"missing marker":          text + stop,
		"missing terminal choice": text + done,
		"token limit":             text + streamDelta(t, nil, "length") + done,
		"refusal":                 streamDelta(t, map[string]any{"refusal": "private refusal"}, "stop") + done,
		"provider error":          text + "event: error\ndata: private-provider-data\n\n",
		"invalid JSON":            text + "data: {bad}\n\n",
		"changed identity":        text + strings.Replace(stop, "response_1", "response_2", 1) + done,
		"changed model":           text + strings.Replace(stop, "fixture", "another", 1) + done,
		"late delta":              text + stop + text + done,
		"premature usage":         usage + text + stop + done,
		"duplicate usage":         text + stop + usage + usage + done,
		"negative usage":          text + stop + strings.Replace(usage, ":2", ":-2", 1) + done,
		"missing tool index":      streamDelta(t, map[string]any{"tool_calls": []any{streamCall(nil, "call_1", "read_file", `{"path":"a.go"}`)}}, "tool_calls") + done,
		"tool index gap":          streamDelta(t, map[string]any{"tool_calls": []any{streamCall(1, "call_1", "read_file", `{"path":"a.go"}`)}}, "tool_calls") + done,
		"duplicate tool id":       streamDelta(t, map[string]any{"tool_calls": []any{streamCall(0, "call_1", "read_file", `{"path":"a.go"}`), streamCall(1, "call_1", "read_file", `{"path":"b.go"}`)}}, "tool_calls") + done,
		"oversized frame":         "data: " + strings.Repeat("x", maxStreamFrameBytes) + "\n\n",
		"oversized stream":        strings.Repeat(": heartbeat\n\n", maxProviderResponseBytes/10) + text + stop + done,
	} {
		t.Run(name, func(t *testing.T) {
			input := generationInput()
			input.Stream = true
			result, _, err := collectGeneration(streamClient(body), context.Background(), input)
			if result != nil || !errors.Is(err, ErrUnusableResponse) || strings.Contains(err.Error(), "private") {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}
}

func TestGenerationStreamClosesOnCancellationOrEarlyConsumerExit(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		t.Run(map[bool]string{false: "consumer exit", true: "cancel"}[cancelRequest], func(t *testing.T) {
			closed := make(chan struct{})
			frame := streamDelta(t, map[string]any{"content": "partial"}, "")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request generationWireRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if !request.Stream || request.StreamOptions == nil || !request.StreamOptions.IncludeUsage {
					t.Error("stream usage was not requested")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, frame)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				close(closed)
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input := generationInput()
			input.Stream = true
			var receivedError error
			for event, err := range NewClient(testProfile(server.URL, "")).Generate(ctx, input) {
				if err != nil {
					receivedError = err
					break
				}
				if event.Result != nil {
					t.Fatal("partial stream produced a result")
				}
				if cancelRequest {
					cancel()
				} else {
					break
				}
			}
			if cancelRequest && !errors.Is(receivedError, context.Canceled) {
				t.Fatalf("cancellation=%v", receivedError)
			}
			select {
			case <-closed:
			case <-time.After(2 * time.Second):
				t.Fatal("provider request survived its consumer")
			}
		})
	}
}

func TestGenerationStreamRejectsNonStreamingProviderResponse(t *testing.T) {
	client := NewClient(testProfile("http://provider.invalid", ""))
	client.transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"choices":[]}`))}, nil
	})
	input := generationInput()
	input.Stream = true
	if result, _, err := collectGeneration(client, context.Background(), input); result != nil || !errors.Is(err, ErrUnusableResponse) {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}
