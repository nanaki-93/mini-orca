package agentruntime

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

func TestPrivateToolEventsAreBoundedRedactedAndExplicit(t *testing.T) {
	events := []agentstate.Event{{ID: "event", Data: json.RawMessage(`{"content":{"parts":[{"functionCall":{"id":"call","name":"read","args":{"path":"main.go","nested":{"API-key":"SECRET"}}}},{"functionResponse":{"id":"call","name":"read","response":{"result":"source","authorization":"SECRET"}}}]}}`)}}
	result, truncated, err := InspectToolEvents(events)
	if err != nil || truncated || len(result) != 2 {
		t.Fatalf("records: %+v %v %v", result, truncated, err)
	}
	if result[0].Kind != "arguments" || result[1].Kind != "result" || !strings.Contains(result[0].Value, "main.go") {
		t.Fatalf("lost tool identity: %+v", result)
	}
	for _, record := range result {
		if strings.Contains(record.Value, "SECRET") || !strings.Contains(record.Value, "[redacted]") {
			t.Fatalf("credential disclosure: %+v", record)
		}
	}
	for len(events) < 129 {
		events = append(events, events[0])
	}
	result, truncated, err = InspectToolEvents(events)
	if err != nil || !truncated || len(result) != 16 {
		t.Fatalf("history bound: %d %v %v", len(result), truncated, err)
	}
	long, err := inspectToolRecord("event", "call", "read", "result", map[string]any{"text": strings.Repeat("界", 2000)})
	if err != nil {
		t.Fatal(err)
	}
	if !long.Truncated || len(long.Value) > 2048 || !utf8.ValidString(long.Value) {
		t.Fatal("invalid tool value bound")
	}
	if _, _, err := InspectToolEvents([]agentstate.Event{{Data: json.RawMessage(`invalid`)}}); err == nil {
		t.Fatal("corrupt event became empty success")
	}
}
