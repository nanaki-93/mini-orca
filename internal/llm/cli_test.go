package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

const cliFixtureContent = `{"answer":"café 世界"}`

func TestCLIProvidersSendOnlyControlledArgumentsAndValidateStructuredResponses(t *testing.T) {
	for _, provider := range []config.ModelProvider{config.AgyProvider, config.PiProvider} {
		t.Run(string(provider), func(t *testing.T) {
			profile, capture := cliFixture(t, provider, "success")
			profile.ReasoningEffort = "high"
			client := NewClient(profile)
			schema := JSONSchema{Name: "answer", Schema: json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`)}
			response, err := client.ChatWithJSONSchema(context.Background(), []ChatMessage{
				{Role: "system", Content: "Only explain the supplied source."},
				{Role: "user", Content: "source-private-marker; $(not-a-command)\n/skill @secret.txt"},
				{Role: "assistant", Content: "Earlier answer"},
				{Role: "user", Content: "Follow up"},
			}, schema)
			if err != nil {
				t.Fatal(err)
			}
			if response.Choices[0].Message.Content != cliFixtureContent || response.Usage.TotalTokens != 30 {
				t.Fatalf("response = %+v", response)
			}
			var request cliCapture
			readTestJSON(t, capture, &request)
			if !strings.Contains(request.Input, "source-private-marker") || !strings.Contains(request.Input, "Earlier answer") || !strings.Contains(request.System, "Only explain") || !strings.Contains(request.System, `"additionalProperties":false`) {
				t.Fatalf("lost conversation or schema: %+v", request)
			}
			if strings.Contains(strings.Join(request.Args, " "), "source-private-marker") || request.Directory == "" || request.PrivateMode != 0700 {
				t.Fatalf("unsafe CLI arguments or directory: %+v", request)
			}
			if _, err := os.Stat(request.Directory); !os.IsNotExist(err) {
				t.Fatalf("request directory was retained: %v", err)
			}
			assertCLIFlags(t, provider, request.Args)
			if provider == config.AgyProvider && (!strings.Contains(request.System, "tools: []") || !strings.Contains(request.System, "commandExecutionPolicy: off")) {
				t.Fatal("agy agent must have no tools")
			}
			if provider == config.PiProvider && !strings.Contains(request.Settings, `"enabled":false`) {
				t.Fatal("pi automatic retry and compaction must be disabled")
			}
		})
	}
}

func TestCLIProvidersRejectFailuresWithoutReturningDiagnostics(t *testing.T) {
	for _, provider := range []config.ModelProvider{config.AgyProvider, config.PiProvider} {
		for _, mode := range []string{"exit", "malformed", "partial", "tool", "failed", "empty", "oversized", "stderr-limit"} {
			t.Run(string(provider)+"/"+mode, func(t *testing.T) {
				profile, capture := cliFixture(t, provider, mode)
				_, err := NewClient(profile).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "source-private-marker"}})
				if err == nil || strings.Contains(err.Error(), "source-private-marker") || strings.Contains(err.Error(), "credential-private-marker") {
					t.Fatalf("CLI failure = %v", err)
				}
				var request cliCapture
				readTestJSON(t, capture, &request)
				if _, err := os.Stat(request.Directory); !os.IsNotExist(err) {
					t.Fatalf("failed request retained private files: %v", err)
				}
			})
		}
	}
}

func TestCLISchemaMismatchAndExternalReferencesFailClosed(t *testing.T) {
	profile, capture := cliFixture(t, config.PiProvider, "success")
	client := NewClient(profile)
	_, err := client.ChatWithJSONSchema(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}, JSONSchema{Name: "answer", Schema: json.RawMessage(`{"type":"object","required":["missing"]}`)})
	if !errors.Is(err, ErrUnusableResponse) {
		t.Fatalf("schema mismatch = %v", err)
	}
	if err := os.Remove(capture); err != nil {
		t.Fatal(err)
	}
	_, err = client.ChatWithJSONSchema(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}, JSONSchema{Name: "answer", Schema: json.RawMessage(`{"$ref":"file:///private-schema.json"}`)})
	if !errors.Is(err, ErrStructuredRequestRejected) {
		t.Fatalf("external reference = %v", err)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("invalid schema started a process")
	}
}

func TestCLIUnavailableAndCanceledRequestsDoNotStart(t *testing.T) {
	profile, capture := cliFixture(t, config.PiProvider, "success")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewClient(profile).Chat(ctx, []ChatMessage{{Role: "user", Content: "hello"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request = %v", err)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("canceled request started a process")
	}
	profile.CLIPath = filepath.Join(t.TempDir(), "missing-private-path")
	_, err := NewClient(profile).Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}})
	if !errors.Is(err, ErrRequestRejected) || strings.Contains(err.Error(), "private-path") {
		t.Fatalf("unavailable executable = %v", err)
	}
}

func TestCLIOptionalFinalContentRequiresNormalCompletion(t *testing.T) {
	for _, provider := range []config.ModelProvider{config.AgyProvider, config.PiProvider} {
		t.Run(string(provider), func(t *testing.T) {
			profile, _ := cliFixture(t, provider, "empty")
			client := NewClient(profile).WithOptionalFinalContent()
			schema := JSONSchema{Name: "review", Schema: json.RawMessage(`{"type":"object","required":["findings"]}`)}
			response, err := client.ChatWithJSONSchema(context.Background(), []ChatMessage{{Role: "user", Content: "Review this file."}}, schema)
			if err != nil || response.Choices[0].Message.Content != "" {
				t.Fatalf("normal blank review = %+v, %v", response, err)
			}
			t.Setenv("MINI_ORCA_TEST_CLI", "failed")
			if _, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "Review this file."}}); !errors.Is(err, ErrUnusableResponse) {
				t.Fatalf("abnormal review completion = %v", err)
			}
		})
	}
}

func TestCLIProtocolRejectsIncorrectModelsAndExtraTurns(t *testing.T) {
	for _, provider := range []config.ModelProvider{config.AgyProvider, config.PiProvider} {
		profile := config.ModelProfile{Provider: provider, Model: "fixture-model"}
		valid := cliFixtureOutput(provider, "success")
		for name, output := range map[string]string{
			"missing model": strings.Replace(valid, `"model":"fixture-model"`, `"model":""`, 1),
			"wrong model":   strings.Replace(valid, `"model":"fixture-model"`, `"model":"fallback-model"`, 1),
			"extra turn":    valid + valid,
		} {
			t.Run(string(provider)+"/"+name, func(t *testing.T) {
				if _, err := decodeCLIResponse(profile, []byte(output)); !errors.Is(err, ErrUnusableResponse) {
					t.Fatalf("invalid CLI protocol = %v", err)
				}
			})
		}
	}
}

func TestCLIPiAcceptsUserEventsAndRejectsToolsAndRetries(t *testing.T) {
	profile := config.ModelProfile{Provider: config.PiProvider, Model: "fixture-provider/fixture-model"}
	valid := cliFixtureOutput(config.PiProvider, "success")
	user := "{\"type\":\"message_end\",\"message\":{\"role\":\"user\",\"content\":\"input\"}}\n"
	withUser := strings.Replace(valid, "{\"type\":\"agent_start\"}\n", "{\"type\":\"agent_start\"}\n"+user, 1)
	if _, err := decodeCLIResponse(profile, []byte(withUser)); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{
		"tool message": strings.Replace(withUser, `"role":"user"`, `"role":"toolResult"`, 1),
		"tool call":    strings.Replace(valid, `"type":"text"`, `"type":"toolCall"`, 1),
		"retry":        strings.Replace(valid, `"willRetry":false`, `"willRetry":true`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCLIResponse(profile, []byte(output)); !errors.Is(err, ErrUnusableResponse) {
				t.Fatalf("invalid Pi protocol = %v", err)
			}
		})
	}
}

func TestCLICancellationAndDeadlineStopOwnedProcess(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint(deadline), func(t *testing.T) {
			profile, capture := cliFixture(t, config.PiProvider, "wait")
			ctx, cancel := context.WithCancel(context.Background())
			want := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 400*time.Millisecond)
				want = context.DeadlineExceeded
			}
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := NewClient(profile).Chat(ctx, []ChatMessage{{Role: "user", Content: "hello"}})
				done <- err
			}()
			waitForCLIFile(t, capture)
			if !deadline {
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, want) {
					t.Fatalf("cancellation error = %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("CLI did not stop within its cleanup deadline")
			}
		})
	}
}

func TestCLICompletionAndCancellationStopDescendants(t *testing.T) {
	for _, mode := range []string{"success-child", "wait-child"} {
		t.Run(mode, func(t *testing.T) {
			profile, capture := cliFixture(t, config.PiProvider, mode)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := NewClient(profile).Chat(ctx, []ChatMessage{{Role: "user", Content: "hello"}})
				done <- err
			}()
			waitForCLIFile(t, capture)
			var child cliChildCapture
			readTestJSON(t, capture+".child", &child)
			t.Cleanup(func() {
				process, err := os.FindProcess(child.PID)
				if err == nil {
					_ = process.Kill()
				}
			})
			if mode == "wait-child" {
				cancel()
			}
			select {
			case err := <-done:
				if mode == "wait-child" && !errors.Is(err, context.Canceled) || mode == "success-child" && err != nil {
					t.Fatalf("CLI completion = %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("CLI did not stop")
			}
			deadline := time.Now().Add(time.Second)
			for {
				connection, err := net.DialTimeout("tcp", child.Address, time.Second)
				if err != nil {
					break
				}
				_ = connection.Close()
				if time.Now().After(deadline) {
					t.Fatal("CLI descendant survived request completion")
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}

type cliChildCapture struct {
	PID     int
	Address string
}

func serveCLIHelperChild(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	data, _ := json.Marshal(cliChildCapture{PID: os.Getpid(), Address: listener.Addr().String()})
	if err := os.WriteFile(os.Getenv("MINI_ORCA_TEST_CAPTURE")+".child", data, 0600); err != nil {
		t.Fatal(err)
	}
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		_ = connection.Close()
	}
}

type cliCapture struct {
	Args        []string
	Directory   string
	PrivateMode uint32
	Input       string
	System      string
	Settings    string
}

func cliFixture(t *testing.T, provider config.ModelProvider, mode string) (config.ModelProfile, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("CLI providers require process-group support")
	}
	t.Setenv("MINI_ORCA_TEST_CLI", mode)
	t.Setenv("MINI_ORCA_TEST_PROVIDER", string(provider))
	capture := filepath.Join(t.TempDir(), "capture.json")
	t.Setenv("MINI_ORCA_TEST_CAPTURE", capture)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(t.TempDir(), "fake cli")
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(executable, "'", "'\\''") + "' -test.run '^TestCLIHelperProcess$' -- \"$@\"\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return config.ModelProfile{Provider: provider, CLIPath: command, Model: "fixture-model"}, capture
}

func TestCLIHelperProcess(t *testing.T) {
	mode := os.Getenv("MINI_ORCA_TEST_CLI")
	if mode == "" {
		return
	}
	if mode == "child" {
		serveCLIHelperChild(t)
		os.Exit(0)
	}
	if mode == "success-child" || mode == "wait-child" {
		child := exec.Command(os.Args[0], "-test.run=^TestCLIHelperProcess$")
		child.Env = append(os.Environ(), "MINI_ORCA_TEST_CLI=child")
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		waitForCLIFile(t, os.Getenv("MINI_ORCA_TEST_CAPTURE")+".child")
	}
	input, _ := io.ReadAll(os.Stdin)
	directory, _ := os.Getwd()
	info, _ := os.Stat(directory)
	agentFile := ".agents/agents/mini-orca.md"
	if os.Getenv("MINI_ORCA_TEST_PROVIDER") == "pi" {
		agentFile = "system.txt"
	}
	system, _ := os.ReadFile(filepath.Join(directory, agentFile))
	settings, _ := os.ReadFile(filepath.Join(directory, ".pi/settings.json"))
	capture, _ := json.Marshal(cliCapture{os.Args[3:], directory, uint32(info.Mode().Perm()), string(input), string(system), string(settings)})
	if os.WriteFile(os.Getenv("MINI_ORCA_TEST_CAPTURE"), capture, 0600) != nil {
		os.Exit(99)
	}
	fmt.Fprint(os.Stderr, "credential-private-marker source-private-marker")
	switch mode {
	case "exit":
		os.Exit(7)
	case "wait", "wait-child":
		for {
			time.Sleep(time.Hour)
		}
	case "malformed":
		fmt.Print("credential-private-marker")
	case "oversized":
		fmt.Print(strings.Repeat("x", maxProviderResponseBytes+1))
	case "stderr-limit":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 64*1024+1))
	default:
		fmt.Print(cliFixtureOutput(config.ModelProvider(os.Getenv("MINI_ORCA_TEST_PROVIDER")), mode))
	}
	os.Exit(0)
}

func cliFixtureOutput(provider config.ModelProvider, mode string) string {
	content := cliFixtureContent
	if mode == "empty" {
		content = ""
	}
	if provider == config.AgyProvider {
		init := `{"event":"init","init":{"agent":"mini-orca","model":"fixture-model","tools":[]}}` + "\n"
		if mode == "partial" {
			return init
		}
		if mode == "tool" {
			return strings.Replace(init, `"tools":[]`, `"tools":["run_command"]`, 1)
		}
		status := "SUCCESS"
		if mode == "failed" {
			status = "WAITING"
		}
		result, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]any{"status": status, "response": content, "usage": map[string]int{"input_tokens": 10, "output_tokens": 20, "total_tokens": 30}}})
		return init + string(result) + "\n"
	}
	start := "{\"type\":\"agent_start\"}\n"
	if mode == "partial" {
		return start
	}
	if mode == "tool" {
		return start + "{\"type\":\"tool_execution_start\"}\n"
	}
	stop := "stop"
	if mode == "failed" {
		stop = "length"
	}
	message, _ := json.Marshal(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "model": "fixture-model", "stopReason": stop, "content": []any{map[string]string{"type": "thinking", "thinking": "not the answer"}, map[string]string{"type": "text", "text": content}}, "usage": map[string]int{"input": 10, "output": 20, "totalTokens": 30}}})
	return start + string(message) + "\n{\"type\":\"agent_end\",\"willRetry\":false}\n"
}

func assertCLIFlags(t *testing.T, provider config.ModelProvider, args []string) {
	t.Helper()
	joined := strings.Join(args, " ")
	flags := []string{"--model fixture-model"}
	if provider == config.PiProvider {
		flags = append(flags, "--print", "--mode json", "--no-tools", "--no-session", "--no-extensions", "--no-skills", "--no-context-files", "--offline", "--thinking high")
	} else {
		flags = append(flags, "--input-format stream-json", "--output-format stream-json", "--agent mini-orca", "--disable-slash-commands", "--effort high", "--json-schema")
	}
	for _, flag := range flags {
		if !strings.Contains(joined, flag) {
			t.Errorf("missing %s in %q", flag, joined)
		}
	}
}

func readTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}

func waitForCLIFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if data, err := os.ReadFile(path); err == nil && json.Valid(data) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("CLI did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
