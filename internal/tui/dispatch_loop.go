package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

// Blocking daemon actions (Apply, merge, reindex, opening a file, …) mutate
// State as they go, so they cannot simply move to a pure tea.Cmd. Instead they
// run on a background goroutine that owns BubbleModel.mu. The client releases
// that lock while a request waits on the network (see stateGate), which lets
// the UI goroutine re-render the busy frame, animate the spinner and accept
// Esc. Other input is queued and replayed in order once the action finishes,
// so the action always observes the state it was started from.

// maxQueuedKeys bounds typeahead recorded while an action runs.
const maxQueuedKeys = 64

// dispatchDoneMsg reports that a background batch of blocking actions ended.
type dispatchDoneMsg struct{ canceled bool }

// dispatchRun tracks one batch of blocking actions. It is owned by the UI
// goroutine; the background goroutine only receives its actions and context.
type dispatchRun struct {
	actions     []queuedAction
	label       string
	started     time.Time
	running     bool
	cancel      context.CancelFunc
	cancelable  bool
	canceling   bool
	quit        bool
	queued      []tea.Msg
	queuedKeys  int
	droppedKeys int
	notice      string
}

// cancelableKinds are read-only actions that are safe to abandon midway.
// Mutations (Apply, merge, Undo, trust, cleanup, …) always run to completion
// so the daemon and the TUI agree on what happened.
var cancelableKinds = map[string]bool{
	"reconnect":         true,
	"open-project":      true,
	"load-change":       true,
	"open-file":         true,
	"issue-source":      true,
	"refresh-worktrees": true,
	"open-web":          true,
}

func allCancelable(actions []queuedAction) bool {
	for _, queued := range actions {
		if !cancelableKinds[queued.act.Kind] {
			return false
		}
	}
	return len(actions) > 0
}

// stateGate releases the model's state lock while daemon requests wait on I/O.
// It counts overlapping I/O so actions that fan out requests on several
// goroutines reacquire the lock only once every request has returned.
type stateGate struct {
	state *sync.Mutex
	mu    sync.Mutex
	inIO  int
}

func (g *stateGate) BeginIO() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.inIO++
	if g.inIO == 1 {
		g.state.Unlock()
	}
}

func (g *stateGate) EndIO() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.inIO--
	if g.inIO == 0 {
		g.state.Lock()
	}
}

// beginDispatch records a batch of blocking actions and shows its busy status.
// It reports false when another batch is already pending; the actions are then
// queued again and flushed once that batch finishes.
func (m *BubbleModel) beginDispatch(actions []queuedAction) bool {
	if m.dispatch != nil {
		for _, queued := range actions {
			m.state.PendingActions = append(m.state.PendingActions, queued.act)
		}
		return false
	}
	label := busyLabel(actions[0].act.Kind)
	m.dispatch = &dispatchRun{actions: actions, label: label, started: time.Now(), cancelable: allCancelable(actions)}
	m.state.Busy = label
	return true
}

// startDispatch launches the pending batch on a background goroutine.
func (m *BubbleModel) startDispatch() tea.Cmd {
	run := m.dispatch
	if run == nil || run.running {
		return nil
	}
	run.running = true
	ctx, cancel := context.WithCancel(m.ctx)
	run.cancel = cancel
	actions := run.actions
	return func() tea.Msg {
		m.mu.Lock()
		defer m.mu.Unlock()
		gated := client.WithIOGate(ctx, &stateGate{state: &m.mu})
		for _, queued := range actions {
			if ctx.Err() != nil {
				break
			}
			if err := m.state.dispatchAction(gated, m.client, queued.reqID, queued.act); err != nil && !errors.Is(err, context.Canceled) {
				m.state.Error = err.Error()
			}
		}
		return dispatchDoneMsg{canceled: ctx.Err() != nil}
	}
}

// updateDuringDispatch keeps the UI responsive without touching state that the
// running action owns.
func (m *BubbleModel) updateDuringDispatch(msg tea.Msg) tea.Cmd {
	run := m.dispatch
	switch ev := msg.(type) {
	case syncActionsMsg:
		return m.startDispatch()
	case tea.WindowSizeMsg:
		if ev.Width > 0 && ev.Height > 0 {
			m.width, m.height = ev.Width, ev.Height
		}
	case spinnerTickMsg:
		m.spinnerScheduled = false
		if m.mu.TryLock() {
			m.state.spinnerFrame++
			m.mu.Unlock()
		}
	case tea.KeyMsg:
		m.handleKeyDuringDispatch(run, ev)
	default:
		run.queued = append(run.queued, msg)
	}
	if m.spinnerScheduled {
		return nil
	}
	m.spinnerScheduled = true
	return spinnerTickCmd()
}

func (m *BubbleModel) handleKeyDuringDispatch(run *dispatchRun, msg tea.KeyMsg) {
	ev := keyEventFromTea(msg)
	switch {
	case ev.Ctrl && strings.EqualFold(ev.Key, "c"):
		run.quit = true
		m.cancelDispatch(run)
	case ev.Key == "Escape":
		m.cancelDispatch(run)
	case run.queuedKeys >= maxQueuedKeys:
		run.droppedKeys++
	default:
		run.queued = append(run.queued, msg)
		run.queuedKeys++
	}
}

func (m *BubbleModel) cancelDispatch(run *dispatchRun) {
	if !run.cancelable {
		run.notice = "finishing safely, can't cancel"
		return
	}
	if run.running && run.cancel != nil {
		run.cancel()
	}
	run.canceling = true
}

// requeue carries input that a replay could not apply over to the next batch.
func (m *BubbleModel) requeue(msgs []tea.Msg) {
	for _, msg := range msgs {
		if _, ok := msg.(tea.KeyMsg); ok {
			m.dispatch.queuedKeys++
		}
		m.dispatch.queued = append(m.dispatch.queued, msg)
	}
}

// dispatchDetail describes the running action for the command bar.
func (m *BubbleModel) dispatchDetail() string {
	run := m.dispatch
	if run == nil {
		return ""
	}
	var parts []string
	if elapsed := time.Since(run.started); elapsed >= time.Second {
		parts = append(parts, fmt.Sprintf("%ds", int(elapsed/time.Second)))
	}
	switch {
	case run.canceling:
		parts = append(parts, "canceling…")
	case run.notice != "":
		parts = append(parts, run.notice)
	case run.cancelable:
		parts = append(parts, "Esc cancel")
	}
	if run.queuedKeys > 0 {
		parts = append(parts, fmt.Sprintf("%d %s queued", run.queuedKeys, pluralWord(run.queuedKeys, "key", "keys")))
	}
	if run.droppedKeys > 0 {
		parts = append(parts, fmt.Sprintf("%d ignored", run.droppedKeys))
	}
	return strings.Join(parts, " · ")
}

func pluralWord(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// finishDispatch releases the busy status, flushes follow-up actions and
// replays input that arrived while the batch ran.
func (m *BubbleModel) finishDispatch(done dispatchDoneMsg) tea.Cmd {
	run := m.dispatch
	m.dispatch = nil
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Busy = ""
	m.state.busyDetail = ""
	if run == nil {
		return m.settle(nil)
	}
	if run.cancel != nil {
		run.cancel()
	}
	if done.canceled {
		if strings.Contains(m.state.Error, context.Canceled.Error()) {
			m.state.Error = ""
		}
		m.state.CmdOutput = "Canceled: " + strings.TrimSuffix(run.label, "…")
	}
	if run.quit {
		m.state.Quitting = true
		return m.settle(nil)
	}
	cmds := m.flushActionsAsync()
	for i, msg := range run.queued {
		if m.dispatch != nil {
			m.requeue(run.queued[i:])
			break
		}
		if m.state.Quitting {
			break
		}
		if cmd := m.update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m.settle(cmds)
}
