package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/toolserver"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// The live smoke tests run the Claude Code or agy installed on this machine
// on its signed-in plan, for one small agent run against a tool server whose
// tools only record their calls. They use a little of the plan, so they run
// only when asked:
//
//	MINI_ORCA_CLAUDE_SMOKE=1 go test ./internal/app -run TestRealClaude -v
//	MINI_ORCA_AGY_SMOKE=gemini-model-slug go test ./internal/app -run TestRealAgy -v

// smokeTools serves read_file and finish, recording each call.
func smokeTools(t *testing.T) (*toolserver.Server, func() []string) {
	t.Helper()
	var calls []string
	var mu sync.Mutex
	record := func(name string) func(context.Context, json.RawMessage) (string, error) {
		return func(_ context.Context, _ json.RawMessage) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, name)
			if name == "read_file" {
				return "notes.txt:\n1: The answer is 42.", nil
			}
			return "Finished.", nil
		}
	}
	schema := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"summary":{"type":"string"}},"additionalProperties":false}`)
	server, err := toolserver.Start(context.Background(), "Smoke test tools.", []toolserver.Tool{
		{Name: "read_file", Description: "Read a file of the task.", Schema: schema, ReadOnly: true, Call: record("read_file")},
		{Name: "finish", Description: "End the task with a summary.", Schema: schema, Call: record("finish")},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(calls)
	}
}

// runSmokeAgent runs one small agent run and checks that it used the tool
// server's tools, and only those, and reported its turns and usage. trace,
// when set, shows what the CLI wrote if the run fails.
func runSmokeAgent(t *testing.T, profile config.ModelProfile, trace func()) {
	t.Helper()
	if trace != nil {
		defer func() {
			if t.Failed() {
				trace()
			}
		}()
	}
	server, calls := smokeTools(t)
	client := llm.NewClient(profile)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	result, err := client.RunAgent(ctx, llm.AgentRun{
		System: "You work only through the tools you are given.", Prompt: "Call read_file with path notes.txt, then call finish with a one-line summary of what it says. Then reply Done.",
		MCPURL: server.URL, MCPToken: server.Token, ToolNames: []string{"read_file", "finish"}, MaxTurns: 8, ToolTimeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("%s run: %v (result %+v)", profile.Provider, err, result)
	}
	if got := calls(); !slices.Contains(got, "read_file") || !slices.Contains(got, "finish") {
		t.Fatalf("tool server calls = %v", got)
	}
	if result.ToolCalls < 2 || result.Turns < 2 || !result.UsageKnown || result.Usage.TotalTokens == 0 {
		t.Fatalf("result = %+v", result)
	}
	t.Logf("%s: %d turns, %d tool calls, %d tokens, model %s", profile.Provider, result.Turns, result.ToolCalls, result.Usage.TotalTokens, result.Model)
}

func TestRealClaudeCodeRunsOnItsPlanWithOnlyMiniOrcasTools(t *testing.T) {
	if os.Getenv("MINI_ORCA_CLAUDE_SMOKE") != "1" {
		t.Skip("set MINI_ORCA_CLAUDE_SMOKE=1 to run the installed Claude Code on your Claude plan")
	}
	profile := config.ModelProfile{Provider: config.ClaudeProvider, Model: "haiku", CLIPath: "claude"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	response, err := llm.NewClient(profile).Chat(ctx, []llm.ChatMessage{{Role: "user", Content: "Reply with the single word ok."}})
	if err != nil || !strings.Contains(strings.ToLower(response.Choices[0].Message.Content), "ok") {
		t.Fatalf("completion = %+v, %v", response, err)
	}
	runSmokeAgent(t, profile, nil)
}

func TestRealAgyRunsOnItsPlanWithOnlyMiniOrcasTools(t *testing.T) {
	model := os.Getenv("MINI_ORCA_AGY_SMOKE")
	if model == "" {
		t.Skip("set MINI_ORCA_AGY_SMOKE to a model slug from agy models to run the installed agy on your Google AI plan")
	}
	cli, trace := tracedCLI(t, "agy")
	runSmokeAgent(t, config.ModelProfile{Provider: config.AgyProvider, Model: model, CLIPath: cli}, trace)
}

// tracedCLI wraps an installed CLI so that its output and diagnostics are
// also kept, and returns the wrapper and a func that logs them. The smoke
// runs use only the test's own prompt and tools.
func tracedCLI(t *testing.T, name string) (string, func()) {
	t.Helper()
	real, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("%s is not installed: %v", name, err)
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, name+"-traced")
	script := fmt.Sprintf("#!/bin/bash\n%q \"$@\" 2> >(tee %q >&2) | tee %q\nexit ${PIPESTATUS[0]}\n", real, filepath.Join(dir, "stderr.txt"), filepath.Join(dir, "stream.jsonl"))
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return wrapper, func() {
		for _, file := range []string{"stream.jsonl", "stderr.txt"} {
			data, _ := os.ReadFile(filepath.Join(dir, file))
			t.Logf("--- %s %s:", name, file)
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				if len(line) > 600 {
					line = line[:600] + "…"
				}
				t.Log(line)
			}
		}
	}
}
