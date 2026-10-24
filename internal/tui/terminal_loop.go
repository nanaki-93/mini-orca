package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func isCharDevice(in io.Reader) (*os.File, bool) {
	f, ok := in.(*os.File)
	if !ok {
		return nil, false
	}
	info, err := f.Stat()
	if err != nil || (info.Mode()&os.ModeCharDevice) == 0 {
		return nil, false
	}
	return f, true
}

// DetectTerminalSize inspects the active controlling terminal or environment for full-screen dimensions.
func DetectTerminalSize(fallbackWidth, fallbackHeight int) (int, int) {
	if fallbackWidth > 0 && fallbackHeight > 0 {
		return fallbackWidth, fallbackHeight
	}
	cols, rows := querySttySize(os.Stdin)
	if cols <= 0 || rows <= 0 {
		if tty, err := os.Open("/dev/tty"); err == nil {
			cols, rows = querySttySize(tty)
			_ = tty.Close()
		}
	}
	if cols <= 0 {
		cols, _ = strconv.Atoi(os.Getenv("COLUMNS"))
	}
	if rows <= 0 {
		rows, _ = strconv.Atoi(os.Getenv("LINES"))
	}
	if fallbackWidth > 0 {
		cols = fallbackWidth
	} else if cols <= 0 {
		cols = 120
	}
	if fallbackHeight > 0 {
		rows = fallbackHeight
	} else if rows <= 0 {
		rows = 36
	}
	return cols, rows
}

func querySttySize(f *os.File) (int, int) {
	if f == nil {
		return 0, 0
	}
	cmd := exec.Command("stty", "size")
	cmd.Stdin = f
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return 0, 0
	}
	rows, err1 := strconv.Atoi(fields[0])
	cols, err2 := strconv.Atoi(fields[1])
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return cols, rows
}

// DecodeInputBytes parses raw terminal input bytes into discrete KeyEvents.
func DecodeInputBytes(data []byte) []KeyEvent {
	var events []KeyEvent
	for i := 0; i < len(data); {
		if ev, consumed := decodeEscapeSequence(data[i:]); consumed > 0 {
			events = append(events, ev)
			i += consumed
			continue
		}
		ev, consumed := decodeSingleKey(data[i:])
		events = append(events, ev)
		i += consumed
	}
	return events
}

func decodeEscapeSequence(buf []byte) (KeyEvent, int) {
	if len(buf) == 0 || buf[0] != 0x1b {
		return KeyEvent{}, 0
	}
	if len(buf) >= 3 && buf[1] == '[' {
		switch buf[2] {
		case 'A':
			return KeyEvent{Key: "ArrowUp"}, 3
		case 'B':
			return KeyEvent{Key: "ArrowDown"}, 3
		case 'C':
			return KeyEvent{Key: "ArrowRight"}, 3
		case 'D':
			return KeyEvent{Key: "ArrowLeft"}, 3
		case 'Z':
			return KeyEvent{Key: "Tab", Shift: true}, 3
		}
	}
	return KeyEvent{Key: "Escape"}, 1
}

func decodeSingleKey(buf []byte) (KeyEvent, int) {
	switch buf[0] {
	case 0x03:
		return KeyEvent{Key: "c", Ctrl: true}, 1
	case 0x0b:
		return KeyEvent{Key: "k", Ctrl: true}, 1
	case '\r', '\n':
		return KeyEvent{Key: "Enter"}, 1
	case '\t':
		return KeyEvent{Key: "Tab"}, 1
	case 0x7f, 0x08:
		return KeyEvent{Key: "Backspace"}, 1
	}
	r, size := utf8.DecodeRune(buf)
	if r == utf8.RuneError && size <= 1 {
		return KeyEvent{Key: string(buf[0])}, 1
	}
	return KeyEvent{Key: string(r)}, size
}

type pollTickMsg time.Time

type pollResultMsg struct {
	res pollResult
}

type startAgentResultMsg struct {
	res startAgentResult
}

type changeActionResultMsg struct {
	res changeActionResult
}

type planInvestigationResultMsg struct{ result planInvestigationResult }

type agentRunActionResultMsg struct{ result agentRunActionResult }

func pollTickCmd() tea.Cmd {
	return tea.Tick(750*time.Millisecond, func(t time.Time) tea.Msg {
		return pollTickMsg(t)
	})
}

// BubbleModel adapts State to the Charmbracelet Bubble Tea tea.Model architecture.
type BubbleModel struct {
	agentStream       *agentRunStream
	nativeStreams     bool
	ctx               context.Context
	state             *State
	client            *Client
	width             int
	height            int
	pollScheduled     bool
	pollInFlight      bool
	activeAgentStarts int
}

// NewBubbleModel constructs a Bubble Tea model wrapping State and Client.
func NewBubbleModel(ctx context.Context, state *State, client *Client, width, height int) *BubbleModel {
	w, h := DetectTerminalSize(width, height)
	return &BubbleModel{
		nativeStreams: true,
		ctx:           ctx,
		state:         state,
		client:        client,
		width:         w,
		height:        h,
	}
}

// Init implements tea.Model.
func (m *BubbleModel) Init() tea.Cmd {
	if m.client != nil && m.state.hasActiveBackgroundJob() {
		m.pollScheduled = true
		return pollTickCmd()
	}
	return nil
}

// Update implements tea.Model, handling tea.WindowSizeMsg, tea.KeyMsg, KeyEvent, and poll ticks.
func (m *BubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch ev := msg.(type) {
	case tea.WindowSizeMsg:
		if ev.Width > 0 && ev.Height > 0 {
			m.width = ev.Width
			m.height = ev.Height
		}
	case tea.KeyMsg:
		cmds = append(cmds, m.applyKeyEventAsync(keyEventFromTea(ev))...)
	case KeyEvent:
		m.applyKeyEvent(ev)
	case startAgentResultMsg:
		m.activeAgentStarts = max(0, m.activeAgentStarts-1)
		_ = m.state.applyStartAgentResult(ev.res)
	case changeActionResultMsg:
		m.activeAgentStarts = max(0, m.activeAgentStarts-1)
		_ = m.state.applyChangeActionResult(ev.res)
	case planInvestigationResultMsg:
		m.activeAgentStarts = max(0, m.activeAgentStarts-1)
		m.state.applyPlanInvestigation(ev.result)
	case agentRunActionResultMsg, agentStreamUpdate:
		if cmd := m.applyAgentRunMessage(ev); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case pollTickMsg:
		m.pollScheduled = false
		if cmd := m.startAsyncPollCmd(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case pollResultMsg:
		m.pollInFlight = false
		m.state.applyPollResult(ev.res)
	}
	if m.state.Quitting {
		m.stopAgentStream()
		return m, tea.Quit
	}
	m.syncAgentStream(&cmds)
	m.maybeSchedulePollTick(&cmds)
	return m, batchTeaCmds(cmds)
}

func (m *BubbleModel) applyAgentRunMessage(msg tea.Msg) tea.Cmd {
	switch ev := msg.(type) {
	case agentRunActionResultMsg:
		m.state.applyAgentRunAction(ev.result)
	case agentStreamUpdate:
		return m.applyAgentStreamUpdate(ev)
	}
	return nil
}

func (m *BubbleModel) maybeSchedulePollTick(cmds *[]tea.Cmd) {
	if m.client != nil && (m.state.hasActiveBackgroundJob() || m.activeAgentStarts > 0) && !m.pollScheduled {
		m.pollScheduled = true
		*cmds = append(*cmds, pollTickCmd())
	}
}

func batchTeaCmds(cmds []tea.Cmd) tea.Cmd {
	switch len(cmds) {
	case 0:
		return nil
	case 1:
		return cmds[0]
	default:
		return tea.Batch(cmds...)
	}
}

func (m *BubbleModel) applyKeyEvent(ev KeyEvent) {
	m.state.HandleKey(ev)
	m.state.FlushActions(m.ctx, m.client)
}

func (m *BubbleModel) applyKeyEventAsync(ev KeyEvent) []tea.Cmd {
	m.state.HandleKey(ev)
	if m.client == nil || len(m.state.PendingActions) == 0 {
		m.state.PendingActions = nil
		return nil
	}
	actions := m.state.PendingActions
	m.state.PendingActions = nil
	var cmds []tea.Cmd
	for _, act := range actions {
		reqID := nextTUIReqID("act-" + act.Kind)
		if cmd, ok := m.prepareAsyncActionCmd(reqID, act); ok {
			cmds = append(cmds, cmd)
			continue
		}
		if err := m.state.dispatchAction(m.ctx, m.client, reqID, act); err != nil {
			m.state.Error = err.Error()
		}
	}
	return cmds
}

func (m *BubbleModel) prepareAsyncActionCmd(reqID string, act ActionRequest) (tea.Cmd, bool) {
	switch act.Kind {
	case "agent-run":
		if !m.state.acceptsAgentRunAction(act.AgentRun) {
			return nil, true
		}
		action := *act.AgentRun
		return func() tea.Msg { return agentRunActionResultMsg{result: executeAgentRunAction(m.ctx, m.client, action)} }, true
	case "investigate-plan":
		task, ok := m.state.preparePlanInvestigationTask(act)
		if !ok {
			return nil, true
		}
		m.activeAgentStarts++
		return func() tea.Msg {
			return planInvestigationResultMsg{result: executePlanInvestigation(m.ctx, m.client, task)}
		}, true
	case "start-agent", "save-plan":
		task, ok := m.state.prepareStartAgentTask(reqID, act)
		if !ok {
			return nil, true
		}
		m.activeAgentStarts++
		return func() tea.Msg {
			return startAgentResultMsg{res: executeStartAgentTask(m.ctx, m.client, task)}
		}, true
	case "repair-change", "check-change":
		task, ok := m.state.prepareChangeActionTask(reqID, act)
		if !ok {
			return nil, true
		}
		m.activeAgentStarts++
		return func() tea.Msg {
			return changeActionResultMsg{res: executeChangeActionTask(m.ctx, m.client, task)}
		}, true
	default:
		return nil, false
	}
}

func (m *BubbleModel) startAsyncPollCmd() tea.Cmd {
	if m.pollInFlight || m.client == nil {
		return nil
	}
	snap, ok := m.state.capturePollSnapshot()
	if !ok {
		return nil
	}
	m.pollInFlight = true
	return func() tea.Msg {
		return pollResultMsg{res: executePollSnapshot(m.ctx, m.client, snap)}
	}
}

// View implements tea.Model, rendering the full-screen Lip Gloss / Bubbles frame.
func (m *BubbleModel) View() string {
	return m.state.Render(m.width, m.height)
}

var teaNavigationKeys = map[tea.KeyType]KeyEvent{
	tea.KeyHome:      {Key: "Home"},
	tea.KeyEnd:       {Key: "End"},
	tea.KeyDelete:    {Key: "Delete"},
	tea.KeyPgUp:      {Key: "PageUp"},
	tea.KeyPgDown:    {Key: "PageDown"},
	tea.KeyUp:        {Key: "ArrowUp"},
	tea.KeyDown:      {Key: "ArrowDown"},
	tea.KeyLeft:      {Key: "ArrowLeft"},
	tea.KeyRight:     {Key: "ArrowRight"},
	tea.KeyTab:       {Key: "Tab"},
	tea.KeyShiftTab:  {Key: "Tab", Shift: true},
	tea.KeyEnter:     {Key: "Enter"},
	tea.KeyEsc:       {Key: "Escape"},
	tea.KeyBackspace: {Key: "Backspace"},
	tea.KeySpace:     {Key: " "},
	tea.KeyCtrlC:     {Key: "c", Ctrl: true},
	tea.KeyCtrlA:     {Key: "a", Ctrl: true},
	tea.KeyCtrlE:     {Key: "e", Ctrl: true},
	tea.KeyCtrlU:     {Key: "u", Ctrl: true},
	tea.KeyCtrlP:     {Key: "p", Ctrl: true},
	tea.KeyCtrlN:     {Key: "n", Ctrl: true},
	tea.KeyCtrlK:     {Key: "k", Ctrl: true},
}

func keyEventFromTea(msg tea.KeyMsg) KeyEvent {
	if msg.Type == tea.KeyRunes {
		return KeyEvent{Key: string(msg.Runes), Text: string(msg.Runes), Pasted: msg.Paste}
	}
	if key, ok := teaNavigationKeys[msg.Type]; ok {
		return key
	}
	return KeyEvent{Key: msg.String()}
}

// RunSession runs an interactive terminal session using Bubble Tea on TTYs or deterministic model updates on streams.
func (s *State) RunSession(ctx context.Context, client *Client, in io.Reader, out io.Writer, width, height int) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	model := NewBubbleModel(ctx, s, client, width, height)
	defer model.stopAgentStream()
	if _, isTTY := isCharDevice(in); isTTY {
		prog := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen(), tea.WithContext(ctx))
		_, err := prog.Run()
		return err
	}
	model.nativeStreams = false
	if _, err := fmt.Fprintf(out, "\033[2J\033[H%s\n", model.View()); err != nil {
		return fmt.Errorf("render initial frame: %w", err)
	}
	buf := make([]byte, 64)
	for !s.Quitting {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, err := in.Read(buf)
		if n > 0 {
			for _, ev := range DecodeInputBytes(buf[:n]) {
				_, _ = model.Update(ev)
				if s.Quitting {
					break
				}
			}
			w, h := DetectTerminalSize(width, height)
			_, _ = model.Update(tea.WindowSizeMsg{Width: w, Height: h})
			if _, wErr := fmt.Fprintf(out, "\033[2J\033[H%s\n", model.View()); wErr != nil {
				return fmt.Errorf("render frame: %w", wErr)
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read terminal input: %w", err)
		}
	}
	return nil
}
