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

var csiThreeByteKeys = map[byte]KeyEvent{
	'A': {Key: "ArrowUp"},
	'B': {Key: "ArrowDown"},
	'C': {Key: "ArrowRight"},
	'D': {Key: "ArrowLeft"},
	'H': {Key: "Home"},
	'F': {Key: "End"},
	'Z': {Key: "Tab", Shift: true},
}

var csiTildeKeys = map[byte]KeyEvent{
	'1': {Key: "Home"},
	'3': {Key: "Delete"},
	'4': {Key: "End"},
	'5': {Key: "PageUp"},
	'6': {Key: "PageDown"},
	'7': {Key: "Home"},
	'8': {Key: "End"},
}

var ctrlSingleKeys = map[byte]KeyEvent{
	0x01: {Key: "a", Ctrl: true},
	0x03: {Key: "c", Ctrl: true},
	0x05: {Key: "e", Ctrl: true},
	0x0b: {Key: "k", Ctrl: true},
	0x0e: {Key: "n", Ctrl: true},
	0x10: {Key: "p", Ctrl: true},
	0x15: {Key: "u", Ctrl: true},
}

func decodeEscapeSequence(buf []byte) (KeyEvent, int) {
	if len(buf) == 0 || buf[0] != 0x1b {
		return KeyEvent{}, 0
	}
	if len(buf) >= 4 && buf[1] == '[' && buf[3] == '~' {
		if ev, ok := csiTildeKeys[buf[2]]; ok {
			return ev, 4
		}
	}
	if len(buf) >= 3 && buf[1] == '[' {
		if ev, ok := csiThreeByteKeys[buf[2]]; ok {
			return ev, 3
		}
	}
	if len(buf) >= 3 && buf[1] == 'O' {
		switch buf[2] {
		case 'H':
			return KeyEvent{Key: "Home"}, 3
		case 'F':
			return KeyEvent{Key: "End"}, 3
		}
	}
	return KeyEvent{Key: "Escape"}, 1
}

func decodeSingleKey(buf []byte) (KeyEvent, int) {
	if ev, ok := ctrlSingleKeys[buf[0]]; ok {
		return ev, 1
	}
	switch buf[0] {
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

type missionResultMsg struct{ result missionResult }

type agentRunActionResultMsg struct{ result agentRunActionResult }

type agentSettingsResultMsg struct{ result agentSettingsResult }

type agentConnectionResultMsg struct{ result agentConnectionResult }

type agentExternalResultMsg struct{ result agentExternalResult }

type agentLibraryResultMsg struct{ result agentLibraryResult }

// syncActionsMsg carries daemon actions that still run inside Update. They are
// deferred by one message so the busy status renders before the request blocks.
type syncActionsMsg struct{ actions []queuedAction }

type queuedAction struct {
	reqID string
	act   ActionRequest
}

// spinnerTickMsg advances the status-bar spinner while work is in flight.
type spinnerTickMsg time.Time

const spinnerInterval = 120 * time.Millisecond

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg { return spinnerTickMsg(t) })
}

type agentMediaResultMsg struct{ result agentMediaResult }

func pollTickCmd() tea.Cmd {
	return tea.Tick(750*time.Millisecond, func(t time.Time) tea.Msg {
		return pollTickMsg(t)
	})
}

// BubbleModel adapts State to the Charmbracelet Bubble Tea tea.Model architecture.
type BubbleModel struct {
	mediaStream       *mediaStream
	agentStream       *agentRunStream
	missionStream     *missionExecutionStream
	nativeStreams     bool
	ctx               context.Context
	state             *State
	client            *Client
	width             int
	height            int
	pollScheduled     bool
	pollInFlight      bool
	spinnerScheduled  bool
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
	var cmds []tea.Cmd
	m.syncMissionExecutionStream(&cmds)
	m.syncMediaStream(&cmds)
	if m.client != nil && m.state.hasActiveBackgroundJob() {
		m.pollScheduled = true
		cmds = append(cmds, pollTickCmd())
	}
	return batchTeaCmds(cmds)
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
	case agentCloudResultMsg, agentTriggerResultMsg, agentMediaResultMsg, mediaStreamUpdate, missionResultMsg, missionExecutionUpdate, agentRunActionResultMsg, agentStreamUpdate, agentSettingsResultMsg, agentConnectionResultMsg, agentExternalResultMsg, agentLibraryResultMsg:
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
	case syncActionsMsg:
		cmds = append(cmds, m.runSyncActions(ev.actions)...)
	case spinnerTickMsg:
		m.spinnerScheduled = false
		m.state.spinnerFrame++
	}
	m.state.reconcileCloudScope()
	m.state.reconcileTriggerScope()
	m.state.reconcileMissionScope()
	m.state.reconcileMediaScope()
	if m.state.Quitting {
		m.state.cancelCloudRequest()
		m.state.cancelTriggerRequest()
		m.state.cancelMissionRequest()
		m.state.cancelMediaRequest()
		m.stopAgentStream()
		m.stopMissionExecutionStream()
		m.stopMediaStream()
		return m, tea.Quit
	}
	m.syncAgentStream(&cmds)
	m.syncMediaStream(&cmds)
	m.syncMissionExecutionStream(&cmds)
	m.maybeSchedulePollTick(&cmds)
	m.state.asyncWork = m.activeAgentStarts
	m.maybeScheduleSpinner(&cmds)
	return m, batchTeaCmds(cmds)
}

// maybeScheduleSpinner animates the status bar only while something runs, so an
// idle TUI stays quiet.
func (m *BubbleModel) maybeScheduleSpinner(cmds *[]tea.Cmd) {
	if m.spinnerScheduled || !m.state.showsActivity() {
		return
	}
	m.spinnerScheduled = true
	*cmds = append(*cmds, spinnerTickCmd())
}

// runSyncActions dispatches deferred actions in their original order, then
// clears the busy status and flushes anything those actions queued.
func (m *BubbleModel) runSyncActions(actions []queuedAction) []tea.Cmd {
	for _, queued := range actions {
		if m.client == nil {
			break
		}
		if err := m.state.dispatchAction(m.ctx, m.client, queued.reqID, queued.act); err != nil {
			m.state.Error = err.Error()
		}
	}
	m.state.Busy = ""
	return m.flushActionsAsync()
}

func (m *BubbleModel) applyAgentRunMessage(msg tea.Msg) tea.Cmd {
	switch ev := msg.(type) {
	case agentCloudResultMsg:
		m.state.applyCloudResult(ev.result)
	case agentTriggerResultMsg:
		m.state.applyTriggerResult(ev.result)
	case agentMediaResultMsg:
		m.state.applyMediaResult(ev.result)
	case mediaStreamUpdate:
		return m.applyMediaStreamUpdate(ev)
	case missionResultMsg:
		m.state.applyMissionResult(ev.result)
	case missionExecutionUpdate:
		return m.applyMissionExecutionUpdate(ev)
	case agentLibraryResultMsg:
		m.state.applyLibraryResult(ev.result)
	case agentExternalResultMsg:
		m.state.applyExternalResult(ev.result)
	case agentConnectionResultMsg:
		m.state.applyConnectionResult(ev.result)
	case agentSettingsResultMsg:
		m.state.applyAgentSettings(ev.result)
		return batchTeaCmds(m.flushActionsAsync())
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
	return m.flushActionsAsync()
}

func (m *BubbleModel) flushActionsAsync() []tea.Cmd {
	if m.client == nil || len(m.state.PendingActions) == 0 {
		m.state.PendingActions = nil
		return nil
	}
	actions := m.state.PendingActions
	m.state.PendingActions = nil
	var cmds []tea.Cmd
	var deferred []queuedAction
	for _, act := range actions {
		reqID := nextTUIReqID("act-" + act.Kind)
		if cmd, ok := m.prepareAsyncActionCmd(reqID, act); ok {
			cmds = append(cmds, cmd)
			continue
		}
		deferred = append(deferred, queuedAction{reqID: reqID, act: act})
	}
	if len(deferred) > 0 {
		m.state.Busy = busyLabel(deferred[0].act.Kind)
		cmds = append(cmds, func() tea.Msg { return syncActionsMsg{actions: deferred} })
	}
	return cmds
}

// busyLabel names a blocking daemon action for the status bar.
func busyLabel(kind string) string {
	switch kind {
	case "scan":
		return "Starting Go scan…"
	case "apply-commit":
		return "Applying reviewed changes…"
	case "merge-worktree":
		return "Merging worktree…"
	case "review-file":
		return "Recording file review…"
	case "open-file", "issue-source":
		return "Opening file…"
	case "load-change":
		return "Loading task…"
	case "reindex", "reindex-worktrees":
		return "Reindexing project…"
	case "reconnect", "open-project":
		return "Connecting to the daemon…"
	case "refresh-worktrees":
		return "Refreshing worktrees…"
	case "open-web":
		return "Opening the Web UI…"
	case "run-analysis", "reanalyze", "repair-analysis":
		return "Preparing analysis…"
	case "pause-analysis":
		return "Pausing analysis…"
	case "resume-analysis":
		return "Resuming analysis…"
	case "undo":
		return "Undoing applied changes…"
	case "set-trust":
		return "Saving execution trust…"
	case "reject-change":
		return "Rejecting task…"
	case "cancel-workflow":
		return "Canceling workflow…"
	case "cleanup-merged":
		return "Cleaning up merged worktree…"
	case "dismiss-finding", "dismiss-feature":
		return "Updating triage…"
	case "refresh-models":
		return "Refreshing models…"
	case "save-selection":
		return "Saving file scope…"
	case "bootstrap":
		return "Running bootstrap…"
	default:
		return "Working…"
	}
}

func (m *BubbleModel) prepareAsyncActionCmd(reqID string, act ActionRequest) (tea.Cmd, bool) {
	switch act.Kind {
	case "agent-triggers":
		return m.prepareTriggerCmd(act.AgentTriggers), true
	case "agent-cloud", "agent-media", "mission", "agent-library", "agent-external":
		return m.prepareScopedWorkCmd(act), true
	case "agent-connection":
		if !m.state.acceptsConnectionAction(act.AgentConnection) {
			return nil, true
		}
		action := *act.AgentConnection
		return func() tea.Msg {
			return agentConnectionResultMsg{result: executeConnectionAction(m.ctx, m.client, action)}
		}, true
	case "agent-settings":
		if !m.state.acceptsSettingsAction(act.AgentSettings) {
			return nil, true
		}
		action := *act.AgentSettings
		return func() tea.Msg { return agentSettingsResultMsg{result: executeAgentSettings(m.ctx, m.client, action)} }, true
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

func (m *BubbleModel) prepareScopedWorkCmd(act ActionRequest) tea.Cmd {
	switch act.Kind {
	case "agent-cloud":
		return m.prepareCloudCmd(act.AgentCloud)
	case "agent-media":
		return m.prepareMediaCmd(act.AgentMedia)
	case "mission":
		return m.prepareMissionCmd(act.Mission)
	case "agent-library":
		return m.prepareLibraryCmd(act.AgentLibrary)
	case "agent-external":
		return m.prepareExternalCmd(act.AgentExternal)
	default:
		return nil
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
	tea.KeyCtrlJ:     {Key: "Enter"}, // Accept LF as well as CR, matching stream input.
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
	defer model.stopMissionExecutionStream()
	defer model.stopMediaStream()
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
