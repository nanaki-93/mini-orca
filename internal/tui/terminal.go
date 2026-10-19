package tui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"
)

func enableRawTerminal(in io.Reader) func() {
	f, ok := in.(*os.File)
	if !ok {
		return func() {}
	}
	info, err := f.Stat()
	if err != nil || (info.Mode()&os.ModeCharDevice) == 0 {
		return func() {}
	}
	saveCmd := exec.Command("stty", "-g")
	saveCmd.Stdin = f
	savedBytes, err := saveCmd.Output()
	if err != nil {
		return func() {}
	}
	saved := strings.TrimSpace(string(savedBytes))
	rawCmd := exec.Command("stty", "-icanon", "-echo", "min", "1", "time", "0")
	rawCmd.Stdin = f
	if err := rawCmd.Run(); err != nil {
		return func() {}
	}
	return func() {
		restoreCmd := exec.Command("stty", saved)
		restoreCmd.Stdin = f
		_ = restoreCmd.Run()
	}
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

// FlushActions dispatches queued PendingActions to the daemon and clears the queue.
func (s *State) FlushActions(ctx context.Context, client *Client) {
	if client == nil || len(s.PendingActions) == 0 {
		s.PendingActions = nil
		return
	}
	actions := s.PendingActions
	s.PendingActions = nil
	for idx, act := range actions {
		reqID := fmt.Sprintf("act-%d-%s", idx, act.Kind)
		if err := s.dispatchAction(ctx, client, reqID, act); err != nil {
			s.Error = err.Error()
		}
	}
}

func (s *State) dispatchAction(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "reconnect":
		return s.SyncFromDaemon(ctx, client, "")
	case "open-project":
		return s.SyncFromDaemon(ctx, client, act.Target)
	case "reindex":
		var idx ProjectIndex
		if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/reindex", map[string]any{}, &idx); err != nil {
			return err
		}
		if s.AcceptProjectResponse(idx.ProjectIdentity) {
			s.Index = &idx
		}
		return nil
	case "scan":
		var scan Scan
		if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/scan", map[string]any{}, &scan); err != nil {
			return err
		}
		if s.AcceptProjectResponse(scan.ProjectIdentity) {
			s.Scan = &scan
		}
		return nil
	case "open-file":
		return s.fetchFileInfo(ctx, client, reqID, act.Target)
	default:
		return s.dispatchMutationAction(ctx, client, reqID, act)
	}
}

func (s *State) fetchFileInfo(ctx context.Context, client *Client, reqID, path string) error {
	var info FileInfo
	target := "/api/projects/current/files/info?path=" + url.QueryEscape(path)
	if err := client.DoJSON(ctx, reqID, http.MethodGet, target, nil, &info); err != nil {
		return err
	}
	if s.AcceptFileResponse(info.Path, "") {
		s.File = &info
	}
	return nil
}

func (s *State) dispatchMutationAction(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "dismiss-finding":
		if act.Issue == nil {
			return nil
		}
		target := "/api/projects/current/findings/" + url.PathEscape(act.Issue.ID)
		return client.DoJSON(ctx, reqID, http.MethodPatch, target, map[string]string{"status": "dismissed"}, nil)
	case "dismiss-feature":
		target := "/api/projects/current/features/" + url.PathEscape(act.Target)
		return client.DoJSON(ctx, reqID, http.MethodPatch, target, map[string]string{"status": "dismissed"}, nil)
	case "set-trust":
		body := map[string]bool{"trusted": s.WorkspaceTrust}
		return client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/execution-trust", body, nil)
	case "undo":
		if act.Target == "" {
			return nil
		}
		target := "/api/projects/current/changes/" + url.PathEscape(act.Target) + "/undo"
		return client.DoJSON(ctx, reqID, http.MethodPost, target, map[string]any{}, nil)
	default:
		return nil
	}
}

// RunSession runs an interactive terminal session reading keystrokes from in and writing ANSI frames to out.
func (s *State) RunSession(ctx context.Context, client *Client, in io.Reader, out io.Writer, width, height int) error {
	restore := enableRawTerminal(in)
	defer restore()
	if _, err := fmt.Fprintf(out, "\033[2J\033[H%s\n", s.Render(width, height)); err != nil {
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
				s.HandleKey(ev)
				s.FlushActions(ctx, client)
				if s.Quitting {
					break
				}
			}
			if _, wErr := fmt.Fprintf(out, "\033[2J\033[H%s\n", s.Render(width, height)); wErr != nil {
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
