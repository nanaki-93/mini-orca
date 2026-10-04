//go:build darwin || linux

package bridge

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const terminalBufferBytes = 1 << 20

type TerminalUpdate struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Data   string `json:"data"`
	Cursor int64  `json:"cursor"`
	Reset  bool   `json:"reset"`
	Error  string `json:"error"`
}

type terminal struct {
	mu       sync.Mutex
	inputMu  sync.Mutex
	file     *os.File
	cmd      *exec.Cmd
	done     chan struct{}
	readDone chan struct{}
	state    string
	failure  string
	data     []byte
	cursor   int64
	tty      string
}

type Terminals struct {
	mu       sync.Mutex
	sessions map[string]*terminal
	next     uint64
}

func NewTerminals() *Terminals { return &Terminals{sessions: make(map[string]*terminal)} }

func terminalSize(cols, rows int) (*pty.Winsize, error) {
	if cols < 1 || rows < 1 || cols > 1000 || rows > 1000 {
		return nil, fmt.Errorf("terminal dimensions must be between 1 and 1000")
	}
	return &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}, nil
}

func interactiveShell() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	if !filepath.IsAbs(shell) {
		return "", fmt.Errorf("SHELL must be an absolute path")
	}
	switch filepath.Base(shell) {
	case "zsh", "bash", "sh", "fish":
		return shell, nil
	default:
		return "", fmt.Errorf("supported shells: zsh, bash, sh, fish")
	}
}

func (t *Terminals) Open(root string, cols, rows int) (TerminalUpdate, error) {
	size, err := terminalSize(cols, rows)
	if err != nil {
		return TerminalUpdate{}, err
	}
	shell, err := interactiveShell()
	if err != nil {
		return TerminalUpdate{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.sessions) >= 8 {
		return TerminalUpdate{}, fmt.Errorf("close a terminal before opening another (limit: 8)")
	}
	for _, session := range t.sessions {
		session.mu.Lock()
		pending := session.state == "cleanup_failed"
		session.mu.Unlock()
		if pending {
			return TerminalUpdate{}, fmt.Errorf("retry the failed terminal cleanup before opening another")
		}
	}
	command := exec.Command(shell, "-l", "-i")
	command.Dir = root
	command.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	file, err := pty.StartWithSize(command, size)
	if err != nil {
		return TerminalUpdate{}, fmt.Errorf("start terminal: %w", err)
	}
	tty, err := processTTY(command.Process.Pid)
	if err != nil {
		stopErr := command.Process.Kill()
		closeErr := file.Close()
		waitErr := command.Wait()
		return TerminalUpdate{}, errors.Join(err, stopErr, closeErr, waitErr)
	}
	t.next++
	id := strconv.FormatUint(t.next, 10)
	session := &terminal{file: file, cmd: command, done: make(chan struct{}), readDone: make(chan struct{}), state: "running", tty: tty}
	t.sessions[id] = session
	go session.read()
	go session.wait()
	return TerminalUpdate{ID: id, State: "running"}, nil
}

func (s *terminal) read() {
	defer close(s.readDone)
	buffer := make([]byte, 16*1024)
	for {
		n, err := s.file.Read(buffer)
		s.mu.Lock()
		s.data = append(s.data, buffer[:n]...)
		s.cursor += int64(n)
		if len(s.data) > terminalBufferBytes {
			s.data = append([]byte(nil), s.data[len(s.data)-terminalBufferBytes:]...)
		}
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) && !errors.Is(err, os.ErrClosed) {
			s.failure = fmt.Sprintf("terminal output: %v", err)
			s.state = "failed"
		}
		s.mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (s *terminal) wait() {
	err := s.cmd.Wait()
	s.mu.Lock()
	if s.state == "running" {
		s.state = "exited"
		if err != nil {
			s.failure = err.Error()
		}
	}
	s.mu.Unlock()
	close(s.done)
}

func (t *Terminals) session(id string) (*terminal, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.sessions[id]
	if s == nil {
		return nil, fmt.Errorf("terminal session is closed")
	}
	return s, nil
}

func (t *Terminals) Read(id string, cursor int64) (TerminalUpdate, error) {
	s, err := t.session(id)
	if err != nil {
		return TerminalUpdate{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	start := s.cursor - int64(len(s.data))
	reset := cursor < start || cursor > s.cursor || cursor < 0
	if reset {
		cursor = start
	}
	return TerminalUpdate{ID: id, State: s.state, Error: s.failure, Cursor: s.cursor, Reset: reset,
		Data: base64.StdEncoding.EncodeToString(s.data[cursor-start:])}, nil
}

func (t *Terminals) Write(id, data string) error {
	if len(data) > 64*1024 {
		return fmt.Errorf("terminal input exceeds 64 KiB")
	}
	s, err := t.session(id)
	if err != nil {
		return err
	}
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	s.mu.Lock()
	running := s.state == "running"
	s.mu.Unlock()
	if !running {
		return fmt.Errorf("terminal is not running")
	}
	done := make(chan error, 1)
	go func() {
		_, err := io.WriteString(s.file, data)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		closeErr := t.Close(id)
		return errors.Join(fmt.Errorf("terminal input stalled; session stopped"), closeErr)
	}
}

func (t *Terminals) Resize(id string, cols, rows int) error {
	size, err := terminalSize(cols, rows)
	if err != nil {
		return err
	}
	s, err := t.session(id)
	if err != nil {
		return err
	}
	return pty.Setsize(s.file, size)
}

func processTTY(pid int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "tty=").Output()
	if err != nil {
		return "", fmt.Errorf("identify terminal owner: %w", err)
	}
	tty := strings.TrimSpace(string(output))
	if tty == "" || tty == "?" || tty == "??" {
		return "", fmt.Errorf("shell has no controlling terminal")
	}
	return tty, nil
}

type ownedProcess struct {
	id, parent, group int
	tty, started      string
}

// The PTY remains open until cleanup finishes, so its device cannot be reused.
// Captured descendants retain their birth identity if the shell exits first.
func terminalProcesses(tty string, captured map[int]string) ([]ownedProcess, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,ppid=,pgid=,tty=,stat=,lstart=").Output()
	if err != nil {
		return nil, fmt.Errorf("inspect terminal processes: %w", err)
	}
	return selectTerminalProcesses(string(output), tty, captured), nil
}

func selectTerminalProcesses(output, tty string, captured map[int]string) []ownedProcess {
	entries := []ownedProcess{}
	owned := map[int]bool{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || strings.HasPrefix(fields[4], "Z") {
			continue
		}
		var e ownedProcess
		if _, err := fmt.Sscan(line, &e.id, &e.parent, &e.group, &e.tty); err != nil {
			continue
		}
		e.started = strings.Join(fields[5:], " ")
		entries = append(entries, e)
		if e.tty == tty || captured[e.id] == e.started {
			owned[e.id] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, e := range entries {
			if owned[e.parent] && !owned[e.id] {
				owned[e.id], changed = true, true
			}
		}
	}
	result := []ownedProcess{}
	for _, e := range entries {
		if owned[e.id] && e.id > 1 && e.group != syscall.Getpgrp() {
			result = append(result, e)
		}
	}
	return result
}

func signalProcesses(processes []ownedProcess, signal syscall.Signal) error {
	var result error
	for _, process := range processes {
		if err := syscall.Kill(process.id, signal); err != nil && !errors.Is(err, syscall.ESRCH) {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (t *Terminals) Close(id string) (result error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.sessions[id]
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.state = "closing"
	s.mu.Unlock()
	defer func() {
		if result != nil {
			s.mu.Lock()
			s.state, s.failure = "cleanup_failed", result.Error()
			s.mu.Unlock()
		}
	}()
	processes, err := terminalProcesses(s.tty, nil)
	if err != nil {
		return err
	}
	captured := make(map[int]string)
	for _, process := range processes {
		captured[process.id] = process.started
	}
	if err := signalProcesses(processes, syscall.SIGHUP); err != nil {
		return fmt.Errorf("stop terminal: %w", err)
	}
	select {
	case <-s.done:
	case <-time.After(750 * time.Millisecond):
	}
	processes, err = terminalProcesses(s.tty, captured)
	if err != nil {
		return err
	}
	if err := signalProcesses(processes, syscall.SIGKILL); err != nil {
		return fmt.Errorf("kill terminal jobs: %w", err)
	}
	for _, process := range processes {
		captured[process.id] = process.started
	}
	select {
	case <-s.done:
	case <-time.After(750 * time.Millisecond):
		return fmt.Errorf("terminal cleanup timed out")
	}
	deadline := time.Now().Add(750 * time.Millisecond)
	for {
		remaining, err := terminalProcesses(s.tty, captured)
		if err != nil {
			return err
		}
		if len(remaining) == 0 {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("terminal jobs are still stopping; retry closing this tab")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := s.file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return fmt.Errorf("close PTY: %w", err)
	}
	select {
	case <-s.readDone:
	case <-time.After(750 * time.Millisecond):
		return fmt.Errorf("terminal output cleanup timed out")
	}
	delete(t.sessions, id)
	return nil
}

func (t *Terminals) CloseAll() error {
	t.mu.Lock()
	ids := make([]string, 0, len(t.sessions))
	for id := range t.sessions {
		ids = append(ids, id)
	}
	t.mu.Unlock()
	var result error
	for _, id := range ids {
		result = errors.Join(result, t.Close(id))
	}
	return result
}
