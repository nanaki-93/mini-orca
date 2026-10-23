package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

const maxAgentPrivateText = 64 << 10

type agentRunController struct {
	mu      sync.Mutex
	active  map[string]context.CancelFunc
	text    map[string]AgentPrivateText
	order   []string
	stopped map[string]bool
}

func newAgentRunController() *agentRunController {
	return &agentRunController{active: map[string]context.CancelFunc{}, text: map[string]AgentPrivateText{}, stopped: map[string]bool{}}
}

func (c *agentRunController) started(run agentstate.Run, cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active[run.ID] = cancel
}

func (c *agentRunController) ended(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.active, id)
	delete(c.stopped, id)
}

func (c *agentRunController) state(id string) (active, canceling bool) {
	if c == nil {
		return false, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active[id] != nil, c.stopped[id]
}

func (c *agentRunController) cancel(id string) error {
	c.mu.Lock()
	cancel := c.active[id]
	if cancel != nil {
		c.stopped[id] = true
	}
	c.mu.Unlock()
	if cancel == nil {
		return fmt.Errorf("invocation is not active in this daemon")
	}
	cancel()
	return nil
}

func (c *agentRunController) appendText(id, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active[id] == nil {
		return
	}
	value, exists := c.text[id]
	if !exists {
		if len(c.order) >= 64 {
			delete(c.text, c.order[0])
			c.order = c.order[1:]
		}
		c.order = append(c.order, id)
		value.Available, value.State = true, "partial_model_output"
	}
	if !value.Truncated {
		value.Text, value.Truncated = boundAgentText(value.Text + text)
	}
	c.text[id] = value
}

func (c *agentRunController) privateText(id string) AgentPrivateText {
	if c == nil {
		return AgentPrivateText{State: "unavailable"}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if value, exists := c.text[id]; exists {
		return value
	}
	return AgentPrivateText{State: "unavailable"}
}

func boundAgentText(text string) (string, bool) {
	if len(text) <= maxAgentPrivateText {
		return text, false
	}
	end := maxAgentPrivateText
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end], true
}

func (s *Service) observeAgentRuns(ctx context.Context) context.Context {
	if s.agentRuns == nil {
		return ctx
	}
	return agentruntime.WithObserver(ctx, &agentruntime.Observer{Started: s.agentRuns.started, Ended: s.agentRuns.ended, Text: s.agentRuns.appendText, Changed: func() { s.notifyEvent("agent_run") }})
}

func progressCursor(epoch string, sequence uint64) string {
	if epoch == "" {
		return ""
	}
	return epoch + ":" + strconv.FormatUint(sequence, 10)
}

func parseProgressCursor(cursor string) (string, uint64, error) {
	if cursor == "" {
		return "", 0, nil
	}
	epoch, sequence, ok := strings.Cut(cursor, ":")
	if !ok || len(cursor) > 180 || strings.ContainsAny(epoch, "/\\\x00\r\n ") || epoch == "" {
		return "", 0, fmt.Errorf("invalid agent progress cursor")
	}
	value, err := strconv.ParseUint(sequence, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid agent progress sequence")
	}
	return epoch, value, nil
}

func (s *Service) AgentRunEvents(ctx context.Context, query AgentRunQuery) (*AgentEventBatch, error) {
	history, _, err := s.readAgentHistory(ctx, query)
	if err != nil {
		return nil, err
	}
	epoch, sequence, err := parseProgressCursor(query.Cursor)
	if err != nil {
		return nil, err
	}
	reset := query.Cursor == "" || epoch != history.Epoch || sequence > history.Sequence
	if len(history.Events) > 0 && sequence < history.Events[0].Sequence-1 {
		reset = true
	}
	batch := &AgentEventBatch{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Epoch: history.Epoch, Cursor: progressCursor(history.Epoch, history.Sequence), Reset: reset, Events: []AgentRunEvent{}}
	if reset {
		query.Cursor, query.Limit = "", 100
		batch.Snapshot, err = s.listAgentHistory(history, query)
		if err != nil {
			return nil, err
		}
		// A reset establishes the current state at this cursor. Older events must
		// not regress a completed snapshot; details expose retained node history.
		return batch, nil
	}
	for _, event := range history.Events {
		if event.Sequence <= sequence || event.ProjectID != query.ProjectID {
			continue
		}
		if len(batch.Events) == 128 {
			batch.HasMore = true
			break
		}
		batch.Events = append(batch.Events, publicProgress(event))
		batch.Cursor = progressCursor(history.Epoch, event.Sequence)
	}
	if !batch.HasMore {
		batch.Cursor = progressCursor(history.Epoch, history.Sequence)
	}
	return batch, nil
}

func publicProgress(event agentstate.ProgressEvent) AgentRunEvent {
	return AgentRunEvent{Sequence: event.Sequence, RunID: event.RootID, InvocationID: event.RunID, ParentID: event.ParentID, ProjectID: event.ProjectID, ProjectRevision: event.ProjectRevision, NodeID: event.NodeID, Kind: event.Kind, State: event.State, Time: event.Time, Usage: event.Usage, Evidence: event.Evidence, RequestID: event.RequestID}
}
