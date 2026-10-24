package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type agentStreamUpdate struct {
	stream   *agentRunStream
	progress *agentProgressView
	private  *AgentRunDetail
	err      error
}

type agentRunStream struct {
	request   agentProgressRequest
	privateID string
	updates   chan agentStreamUpdate
	done      chan struct{}
	cancel    context.CancelFunc
}

func startAgentRunStream(parent context.Context, cl *Client, request agentProgressRequest, privateID string) *agentRunStream {
	ctx, cancel := context.WithCancel(parent)
	stream := &agentRunStream{request: request, privateID: privateID, updates: make(chan agentStreamUpdate, 1), done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(stream.done)
		defer close(stream.updates)
		var wg sync.WaitGroup
		if privateID != "" {
			wg.Add(1)
			go func() { defer wg.Done(); stream.pollPrivate(ctx, cl) }()
		}
		stream.readProgress(ctx, cl)
		wg.Wait()
	}()
	return stream
}

func (stream *agentRunStream) stop() {
	stream.cancel()
	<-stream.done
}

func (stream *agentRunStream) send(ctx context.Context, update agentStreamUpdate) bool {
	update.stream = stream
	select {
	case stream.updates <- update:
		return true
	case <-ctx.Done():
		return false
	}
}

func (stream *agentRunStream) readProgress(ctx context.Context, cl *Client) {
	cursor := ""
	delay := 250 * time.Millisecond
	for ctx.Err() == nil {
		var err error
		cursor, err = cl.StreamAgentEvents(ctx, nextTUIReqID("run-stream"), "/api/projects/current/agent-runs/events"+stream.request.Query, cursor, func(batch client.AgentEventBatch) error {
			progress := readAgentProgress(ctx, cl, stream.request)
			if progress.Status != "current" {
				return fmt.Errorf("refresh execution: %s", progress.Error)
			}
			if !stream.send(ctx, agentStreamUpdate{progress: progress}) {
				return ctx.Err()
			}
			delay = 250 * time.Millisecond
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		if !stream.send(ctx, agentStreamUpdate{err: err}) {
			return
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return
		}
		delay = min(10*time.Second, delay*2)
	}
}

func (stream *agentRunStream) pollPrivate(ctx context.Context, cl *Client) {
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		var detail AgentRunDetail
		err := cl.DoJSON(requestCtx, nextTUIReqID("run-private"), http.MethodGet, "/api/projects/current/agent-runs/"+url.PathEscape(stream.privateID)+stream.request.Query+"&include_text=true", nil, &detail)
		cancel()
		update := agentStreamUpdate{private: &detail, err: err}
		if !stream.send(ctx, update) {
			return
		}
		if err == nil && !detail.Run.Active {
			return
		}
	}
}

func waitAgentStream(stream *agentRunStream) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-stream.updates
		if !ok {
			return agentStreamUpdate{stream: stream}
		}
		return update
	}
}

func (m *BubbleModel) stopAgentStream() {
	if m.agentStream != nil {
		m.agentStream.stop()
		m.agentStream = nil
	}
}

func (m *BubbleModel) syncAgentStream(cmds *[]tea.Cmd) {
	if !m.nativeStreams || m.client == nil {
		m.stopAgentStream()
		return
	}
	if !m.state.AgentDetailOpen || m.state.Page != PageAgents || m.state.currentAgentDetail() == nil || m.ctx.Err() != nil {
		m.stopAgentStream()
		m.state.AgentDisclosure = nil
		return
	}
	request := *m.state.agentProgressRequest()
	privateID := ""
	if value := m.state.AgentDisclosure; value != nil && value.Detail != nil && m.state.matchesAgentRequest(value.Request) {
		privateID = value.Invocation
	}
	if m.agentStream != nil && m.agentStream.request == request && m.agentStream.privateID == privateID {
		return
	}
	m.stopAgentStream()
	m.state.AgentStreamStatus = "Connecting"
	m.agentStream = startAgentRunStream(m.ctx, m.client, request, privateID)
	*cmds = append(*cmds, waitAgentStream(m.agentStream))
}

func (m *BubbleModel) applyAgentStreamUpdate(update agentStreamUpdate) tea.Cmd {
	if update.stream != m.agentStream || m.agentStream == nil {
		return nil
	}
	if update.err != nil {
		m.state.AgentStreamStatus = "Reconnecting: " + update.err.Error()
		if update.private != nil && m.state.AgentDisclosure != nil {
			m.state.AgentDisclosure.Status = "unavailable"
		}
	} else {
		m.state.AgentStreamStatus = "Connected"
		m.state.applyAgentProgress(update.progress)
		if update.private != nil {
			m.state.applyAgentDisclosure(update.stream.request, update.stream.privateID, update.private)
		}
	}
	select {
	case <-m.agentStream.done:
		return nil
	default:
		return waitAgentStream(m.agentStream)
	}
}
