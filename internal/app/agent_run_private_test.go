package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

func TestAgentPrivateToolsRequireSelectedInvocationDisclosure(t *testing.T) {
	s, store, query := agentHistoryFixture(t)
	run := seedAgentRun(t, s, store, "tools", "", time.Now().UTC())
	_, err := store.AppendEvent(t.Context(), run.Session, agentstate.Event{ID: "tool", Time: time.Now().UTC(), Data: json.RawMessage(`{"content":{"parts":[{"functionCall":{"name":"search","args":{"query":"PRIVATE_QUERY","token":"SECRET"}}}]}}`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.AgentRunDetail(t.Context(), query, run.ID, false)
	if err != nil || plain.Text != nil {
		t.Fatalf("implicit disclosure: %+v %v", plain, err)
	}
	private, err := s.AgentRunDetail(t.Context(), query, run.ID, true)
	if err != nil || private.Text == nil || len(private.Text.ToolRecords) != 1 {
		t.Fatalf("explicit disclosure: %+v %v", private, err)
	}
	encoded, _ := json.Marshal(private.Text)
	if !strings.Contains(string(encoded), "PRIVATE_QUERY") || strings.Contains(string(encoded), "SECRET") {
		t.Fatalf("invalid private projection: %s", encoded)
	}
	query.ProjectRevision = "foreign"
	if _, err := s.AgentRunDetail(t.Context(), query, run.ID, true); err == nil {
		t.Fatal("foreign private disclosure accepted")
	}
}
