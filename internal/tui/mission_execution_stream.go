package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type missionProgressRequest struct {
	Parent                                  ProjectIdentity
	Scope                                   client.AgentSettingsScope
	Epoch, MissionID, ExecutionID, PlanHash string
}

type missionExecutionUpdate struct {
	stream *missionExecutionStream
	value  *client.AgentMissionExecution
	err    error
}

type missionExecutionStream struct {
	request missionProgressRequest
	updates chan missionExecutionUpdate
	done    chan struct{}
	cancel  context.CancelFunc
}

func startMissionExecutionStream(parent context.Context, cl *Client, request missionProgressRequest) *missionExecutionStream {
	ctx, cancel := context.WithCancel(parent)
	stream := &missionExecutionStream{request: request, updates: make(chan missionExecutionUpdate, 1), done: make(chan struct{}), cancel: cancel}
	go stream.run(ctx, cl)
	return stream
}

func (stream *missionExecutionStream) run(ctx context.Context, cl *Client) {
	defer close(stream.done)
	defer close(stream.updates)
	events := make(chan struct{}, 1)
	var readers sync.WaitGroup
	readers.Add(1)
	go func() { defer readers.Done(); stream.watchEvents(ctx, cl, events) }()
	defer func() { stream.cancel(); readers.Wait() }()
	// The bounded poll also observes leases and non-model integration effects.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-events:
			if time.Since(last) < 500*time.Millisecond {
				continue
			}
		}
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		value, err := cl.MissionExecution(bounded, nextTUIReqID("mission-progress"), stream.request.Scope, stream.request.MissionID)
		cancel()
		if err == nil && value == nil {
			err = fmt.Errorf("retained execution is unavailable")
		}
		last = time.Now()
		select {
		case stream.updates <- missionExecutionUpdate{stream: stream, value: value, err: err}:
		case <-ctx.Done():
			return
		}
	}
}

func (stream *missionExecutionStream) watchEvents(ctx context.Context, cl *Client, events chan<- struct{}) {
	cursor, delay := "", 250*time.Millisecond
	for ctx.Err() == nil {
		cursor, _ = cl.StreamAgentEvents(ctx, nextTUIReqID("mission-events"), missionResource(stream.request.Scope, "/agent-runs/events"), cursor, func(client.AgentEventBatch) error {
			select {
			case events <- struct{}{}:
			default:
			}
			delay = 250 * time.Millisecond
			return context.Cause(ctx)
		})
		// Event transport loss does not claim current progress. The independently
		// bounded execution read reports freshness or a visible failure.
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		delay = min(8*time.Second, delay*2)
	}
}

func (stream *missionExecutionStream) stop() { stream.cancel(); <-stream.done }

func waitMissionExecutionStream(stream *missionExecutionStream) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-stream.updates
		if !ok {
			return missionExecutionUpdate{stream: stream}
		}
		return update
	}
}

func (s *State) missionProgressRequest() *missionProgressRequest {
	v := s.Mission
	if !s.missionCurrent() || s.Page != PagePlan || s.PlanView != "mission" || v.History || !v.ExecutionOpen || v.Execution == nil || v.Pending != nil || v.Failure != "" || v.Parent != s.Project.ProjectIdentity {
		return nil
	}
	if v.Execution.OwnerState == "idle" {
		return nil
	}
	r := v.Execution.Execution
	return &missionProgressRequest{Parent: v.Parent, Scope: v.Scope, Epoch: v.Epoch, MissionID: r.Mission.ID, ExecutionID: r.ID, PlanHash: r.Mission.PlanHash}
}

func (m *BubbleModel) syncMissionExecutionStream(cmds *[]tea.Cmd) {
	request := m.state.missionProgressRequest()
	if !m.nativeStreams || m.client == nil || request == nil || m.ctx.Err() != nil {
		m.stopMissionExecutionStream()
		return
	}
	if m.missionStream != nil && m.missionStream.request == *request {
		return
	}
	m.stopMissionExecutionStream()
	m.missionStream = startMissionExecutionStream(m.ctx, m.client, *request)
	*cmds = append(*cmds, waitMissionExecutionStream(m.missionStream))
}

func (m *BubbleModel) stopMissionExecutionStream() {
	if m.missionStream != nil {
		m.missionStream.stop()
		m.missionStream = nil
	}
}

func (m *BubbleModel) applyMissionExecutionUpdate(update missionExecutionUpdate) tea.Cmd {
	if update.stream != m.missionStream || update.stream == nil {
		return nil
	}
	request := m.state.missionProgressRequest()
	if request == nil || *request != update.stream.request {
		m.stopMissionExecutionStream()
		return nil
	}
	v := m.state.Mission
	if update.err != nil {
		v.ProgressFailure = externalSafeText(update.err.Error())
	} else if update.value != nil {
		r := update.value.Execution
		if r.ID == request.ExecutionID && r.Mission.PlanHash == request.PlanHash && r.Revision >= v.Execution.Execution.Revision {
			v.Execution, v.ProgressFailure = update.value, ""
			m.state.invalidateMissionCandidate()
		}
	}
	select {
	case <-update.stream.done:
		m.missionStream = nil
		return nil
	default:
	}
	return waitMissionExecutionStream(update.stream)
}
