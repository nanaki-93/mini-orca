package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The test binary plays pi when MINI_ORCA_FAKE_PI is set: it checks that it
// was started with only Mini-Orca's tools, connects to the run's tool server
// and calls the tools of a script, writing pi's JSON event stream.
const fakePiEnv = "MINI_ORCA_FAKE_PI"

// fakePiStep is one model turn: a tool call, or the final text.
type fakePiStep struct {
	Tool      string         `json:"tool,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Text      string         `json:"text,omitempty"`
	// Exit ends the fake with this status, as a crashed CLI would.
	Exit int `json:"exit,omitempty"`
}

// fakePiRecord is what the fake saw and got, for the test to read.
type fakePiRecord struct {
	Role    string   `json:"role"`
	Prompt  string   `json:"prompt"`
	Results []string `json:"results"`
}

type piBearer struct{ token string }

func (b piBearer) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(request)
}

func fakePi() int {
	args := os.Args[1:]
	argument := func(flag string) string {
		if index := slices.Index(args, flag); index >= 0 && index+1 < len(args) {
			return args[index+1]
		}
		return ""
	}
	if argument("--tools") != "mcp__mini_orca__*" || !slices.Contains(args, "builtin:mcp") || argument("--mode") != "json" {
		fmt.Fprintln(os.Stderr, "fake pi: not limited to Mini-Orca's tools:", args)
		return 3
	}
	system, _ := os.ReadFile(argument("--system-prompt"))
	role := "coder"
	if strings.Contains(string(system), "You are Mini-Orca's testing agent") {
		role = "tester"
	}
	dir := os.Getenv("MINI_ORCA_FAKE_PI_DIR")
	var steps []fakePiStep
	if data, err := os.ReadFile(filepath.Join(dir, role+".json")); err == nil {
		_ = json.Unmarshal(data, &steps)
	}
	prompt, _ := io.ReadAll(os.Stdin)
	record := fakePiRecord{Role: role, Prompt: string(prompt)}
	defer func() {
		data, _ := json.Marshal(record)
		_ = os.WriteFile(filepath.Join(dir, role+".log.json"), data, 0o600)
	}()
	model := argument("--model")
	model = model[strings.LastIndex(model, "/")+1:]
	emit := func(event map[string]any) {
		data, _ := json.Marshal(event)
		fmt.Println(string(data))
	}
	assistant := func(stop string, text string) map[string]any {
		return map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "model": model, "stopReason": stop,
			"content": []map[string]any{{"type": "text", "text": text}}, "usage": map[string]any{"input": 1000, "output": 50, "totalTokens": 1050}}}
	}
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "fake-pi", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: os.Getenv("MINI_ORCA_MCP_URL"), HTTPClient: &http.Client{Transport: piBearer{os.Getenv("MINI_ORCA_MCP_TOKEN")}}, MaxRetries: -1}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake pi: connect:", err)
		return 4
	}
	defer session.Close()
	emit(map[string]any{"type": "session", "version": 3})
	emit(map[string]any{"type": "agent_start"})
	for _, step := range steps {
		if step.Exit != 0 {
			return step.Exit
		}
		if step.Tool == "" {
			emit(assistant("stop", step.Text))
			emit(map[string]any{"type": "turn_end"})
			break
		}
		emit(assistant("toolUse", ""))
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: step.Tool, Arguments: step.Arguments})
		text, failed := "", true
		if err == nil {
			failed = result.IsError
			for _, content := range result.Content {
				if part, ok := content.(*mcp.TextContent); ok {
					text += part.Text
				}
			}
		} else {
			text = err.Error()
		}
		record.Results = append(record.Results, text)
		emit(map[string]any{"type": "tool_execution_end", "toolName": "mcp__mini_orca__" + step.Tool, "isError": failed})
		emit(map[string]any{"type": "turn_end"})
	}
	emit(map[string]any{"type": "agent_end", "willRetry": false})
	emit(map[string]any{"type": "agent_settled", "aborted": false})
	return 0
}
