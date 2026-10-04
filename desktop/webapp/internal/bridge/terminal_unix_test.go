//go:build darwin || linux

package bridge

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func awaitOutput(t *testing.T, terminals *Terminals, id, expected string) string {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		update, err := terminals.Read(id, 0)
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(update.Data)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), expected) {
			return string(data)
		}
		time.Sleep(10 * time.Millisecond)
	}
	update, _ := terminals.Read(id, 0)
	data, _ := base64.StdEncoding.DecodeString(update.Data)
	t.Fatalf("terminal did not output %q: %s", expected, data)
	return ""
}

func TestPTYLifecycleResizeAndIndependentSessions(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	root := filepath.Join(t.TempDir(), "folder with spaces")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	terminals := NewTerminals()
	t.Cleanup(func() {
		if err := terminals.CloseAll(); err != nil {
			t.Error(err)
		}
	})
	first, err := terminals.Open(root, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	second, err := terminals.Open(root, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("terminal identities collide")
	}
	if err := terminals.Write(first.ID, "stty -echo\r"); err != nil {
		t.Fatal(err)
	}
	if err := terminals.Write(first.ID, "printf 'terminal-✓\\n'; pwd\r"); err != nil {
		t.Fatal(err)
	}
	awaitOutput(t, terminals, first.ID, root)
	awaitOutput(t, terminals, first.ID, "terminal-✓")
	if err := terminals.Resize(first.ID, 103, 33); err != nil {
		t.Fatal(err)
	}
	if err := terminals.Write(first.ID, "stty size\r"); err != nil {
		t.Fatal(err)
	}
	awaitOutput(t, terminals, first.ID, "33 103")
	if err := terminals.Resize(first.ID, 0, 24); err == nil {
		t.Fatal("invalid resize accepted")
	}
	if err := terminals.Close(first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := terminals.Read(first.ID, 0); err == nil {
		t.Fatal("closed session remained readable")
	}
	if err := terminals.Write(second.ID, "echo second-still-running\r"); err != nil {
		t.Fatal(err)
	}
	awaitOutput(t, terminals, second.ID, "second-still-running")
}

func TestTerminalCloseKillsBackgroundJobs(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	terminals := NewTerminals()
	t.Cleanup(func() {
		if err := terminals.CloseAll(); err != nil {
			t.Error(err)
		}
	})
	session, err := terminals.Open(t.TempDir(), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if err := terminals.Write(session.ID, "stty -echo; sleep 30 & printf '\\nchild=%s\\n' $!\r"); err != nil {
		t.Fatal(err)
	}
	output := awaitOutput(t, terminals, session.ID, "\r\nchild=")
	var child int
	for _, line := range strings.Split(output, "\n") {
		_, _ = fmt.Sscanf(strings.TrimSpace(line), "child=%d", &child)
	}
	if child < 2 {
		t.Fatalf("missing child PID: %s", output)
	}
	if err := terminals.Close(session.ID); err != nil {
		t.Fatal(err)
	}
	data, err := exec.Command("/bin/ps", "-p", fmt.Sprint(child), "-o", "stat=").Output()
	if err == nil && !strings.HasPrefix(strings.TrimSpace(string(data)), "Z") {
		t.Fatalf("terminal child is still alive: %s", data)
	}
}

func TestTerminalCursorBoundsAndShellValidation(t *testing.T) {
	t.Setenv("SHELL", "sh")
	if _, err := NewTerminals().Open(t.TempDir(), 80, 24); err == nil {
		t.Fatal("relative shell path accepted")
	}
	terminals := NewTerminals()
	terminals.sessions["test"] = &terminal{state: "exited", data: []byte("tail"), cursor: 100}
	update, err := terminals.Read("test", 0)
	if err != nil || !update.Reset || update.Cursor != 100 {
		t.Fatalf("lost-output cursor not reported: %#v %v", update, err)
	}
	update, err = terminals.Read("test", 98)
	if err != nil || update.Reset || update.Data != base64.StdEncoding.EncodeToString([]byte("il")) {
		t.Fatalf("incremental read: %#v %v", update, err)
	}
}

func TestTerminalCleanupDoesNotAdoptReusedProcessIdentity(t *testing.T) {
	output := "101 1 101 ttys004 S Sun Oct 4 01:00:00 2026\n" +
		"102 101 102 ?? S Sun Oct 4 01:00:01 2026\n" +
		"103 101 103 ttys004 Z Sun Oct 4 01:00:02 2026\n" +
		"201 1 201 ttys008 S Sun Oct 4 02:00:00 2026\n" +
		"301 1 301 ?? S Sun Oct 4 03:00:00 2026\n"
	captured := map[int]string{201: "Sun Oct 4 00:00:00 2026", 301: "Sun Oct 4 03:00:00 2026"}
	processes := selectTerminalProcesses(output, "ttys004", captured)
	ids := map[int]bool{}
	for _, process := range processes {
		ids[process.id] = true
	}
	if len(ids) != 3 || !ids[101] || !ids[102] || !ids[301] || ids[201] {
		t.Fatalf("cleanup selected unrelated/reused processes: %#v", ids)
	}
}
