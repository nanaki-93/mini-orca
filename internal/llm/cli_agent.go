package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

// ErrAgentRunsUnsupported marks a provider that cannot run a tool-using agent.
var ErrAgentRunsUnsupported = errors.New("this model's provider cannot run a tool-using agent")

// AgentRun is one tool-using model run inside a CLI. The CLI's own tools are
// off; the model's only tools are those of the MCP server the run names.
type AgentRun struct {
	System, Prompt string
	// MCPURL and MCPToken reach the run's tool server.
	MCPURL, MCPToken string
	// MaxTurns ends the run after that many model turns.
	MaxTurns int
	// ToolTimeout bounds one tool call.
	ToolTimeout time.Duration
}

// AgentRunResult is what a run did, without its transcript.
type AgentRunResult struct {
	Model string
	// Usage sums the model's turns; UsageKnown is false when a turn did not report it.
	Usage      ChatUsage
	UsageKnown bool
	Turns      int
	ToolCalls  int
	ToolErrors int
	// Final is the model's last text.
	Final string
	// TurnLimit reports that the run stopped at MaxTurns.
	TurnLimit bool
}

const maxAgentOutputBytes = 256 << 20

// piToolExtension registers the run's tool server with pi and blocks every
// other tool, so the model can act only through Mini-Orca's gates.
const piToolExtension = `// Mini-Orca's tools for one agent run: the run's MCP server, and nothing else.
export default function (pi) {
	const url = process.env.MINI_ORCA_MCP_URL;
	const token = process.env.MINI_ORCA_MCP_TOKEN;
	const maxTurns = Number(process.env.MINI_ORCA_MAX_TURNS || "0");
	const timeout = Number(process.env.MINI_ORCA_TOOL_TIMEOUT || "900");
	if (!url || !token) throw new Error("Mini-Orca's tool server is not configured for this run");
	pi.registerMcpServer("mini-orca", { url, headers: { Authorization: "Bearer " + token }, exposure: "direct", timeout, description: "Mini-Orca's tools for this task" });
	pi.on("tool_call", async (event) => {
		if (!String(event.toolName).startsWith("mcp__mini_orca__")) {
			return { block: true, reason: "Only Mini-Orca's tools are available in this run." };
		}
	});
	let turns = 0;
	pi.on("turn_end", async (_event, ctx) => {
		turns++;
		if (maxTurns > 0 && turns >= maxTurns) ctx.abort();
	});
}
`

// SupportsAgentRuns reports whether the provider can run a tool-using agent.
func (c *Client) SupportsAgentRuns() bool {
	return c.profile.Provider == config.PiProvider
}

// RunAgent runs the model in its CLI until it ends its turn, reaches
// MaxTurns, or ctx ends. The result is returned with an error too when the
// run used the model before failing, so its usage can be charged.
func (c *Client) RunAgent(ctx context.Context, run AgentRun) (*AgentRunResult, error) {
	if !c.SupportsAgentRuns() {
		return nil, ErrAgentRunsUnsupported
	}
	if strings.TrimSpace(run.Prompt) == "" || run.MCPURL == "" || run.MCPToken == "" {
		return nil, fmt.Errorf("llm client: %w: an agent run needs a prompt and a tool server", ErrRequestRejected)
	}
	executable, err := exec.LookPath(c.profile.CLIPath)
	if err != nil {
		return nil, fmt.Errorf("llm client: %w: %s CLI executable is unavailable; install it on the daemon's PATH or set cli_path", ErrRequestRejected, c.profile.Provider)
	}
	directory, err := os.MkdirTemp("", "mini-orca-agent-")
	if err != nil {
		return nil, fmt.Errorf("llm client: cannot create private CLI working directory")
	}
	defer os.RemoveAll(directory)
	args, err := preparePiAgent(directory, c.profile, run.System)
	if err != nil {
		return nil, err
	}
	timeout := int(run.ToolTimeout.Seconds())
	if timeout <= 0 {
		timeout = 900
	}
	env := append(os.Environ(), "MINI_ORCA_MCP_URL="+run.MCPURL, "MINI_ORCA_MCP_TOKEN="+run.MCPToken,
		"MINI_ORCA_MAX_TURNS="+strconv.Itoa(max(run.MaxTurns, 0)), "MINI_ORCA_TOOL_TIMEOUT="+strconv.Itoa(timeout))
	result, err := runAgentProcess(ctx, executable, directory, args, env, run.Prompt, c.profile)
	if result != nil && run.MaxTurns > 0 && result.Turns >= run.MaxTurns {
		result.TurnLimit = true
	}
	return result, err
}

func preparePiAgent(directory string, profile config.ModelProfile, system string) ([]string, error) {
	if err := writeCLIFile(directory, "system.txt", system); err != nil {
		return nil, err
	}
	settings := `{"retry":{"enabled":false,"provider":{"maxRetries":0}},"compaction":{"enabled":false}}`
	if err := writeCLIFile(directory, ".pi/settings.json", settings); err != nil {
		return nil, err
	}
	if err := writeCLIFile(directory, "mini-orca-tools.ts", piToolExtension); err != nil {
		return nil, err
	}
	args := []string{"--mode", "json", "--no-session", "--no-extensions", "--extension", "builtin:mcp", "--extension", filepath.Join(directory, "mini-orca-tools.ts"),
		"--tools", "mcp__mini_orca__*", "--exclude-tools", "codemode,tool_search,list_mcp_resources,list_mcp_resource_templates,read_mcp_resource",
		"--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--approve",
		"--system-prompt", filepath.Join(directory, "system.txt"), "--model", profile.Model}
	if profile.ReasoningEffort != "" {
		effort := profile.ReasoningEffort
		if effort == "none" {
			effort = "off"
		}
		args = append(args, "--thinking", effort)
	}
	return args, nil
}

func runAgentProcess(ctx context.Context, executable, directory string, args, env []string, prompt string, profile config.ModelProfile) (*AgentRunResult, error) {
	owned, cancel := context.WithCancel(ctx)
	defer cancel()
	process := exec.CommandContext(owned, executable, args...)
	process.Dir, process.Env = directory, env
	process.Stdin = strings.NewReader(prompt)
	stderr := &cliOutputBuffer{limit: 64 * 1024, cancel: cancel}
	process.Stderr = stderr
	process.WaitDelay = time.Second
	if err := configureCLIProcess(process); err != nil {
		return nil, err
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("llm client: cannot read the CLI's output")
	}
	if err := process.Start(); err != nil {
		return nil, cliProcessError(profile.Provider, err)
	}
	decoder := agentEventDecoder{profile: profile, result: &AgentRunResult{Model: profile.Model, UsageKnown: true}}
	readErr := decoder.read(io.LimitReader(stdout, maxAgentOutputBytes))
	if readErr != nil {
		cancel()
	}
	err = process.Wait()
	if process.Process != nil {
		if cleanupErr := process.Cancel(); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrProcessDone) {
			return decoder.result, fmt.Errorf("llm client: CLI process-group cleanup failed")
		}
	}
	switch {
	case ctx.Err() != nil:
		return decoder.result, fmt.Errorf("llm client: agent run ended: %w", context.Cause(ctx))
	case readErr != nil:
		return decoder.result, readErr
	case decoder.failure != nil:
		return decoder.result, decoder.failure
	case err != nil:
		return decoder.result, cliProcessError(profile.Provider, err)
	case !decoder.started:
		return decoder.result, unusableCLIResponse("pi did not start the agent run")
	}
	return decoder.result, nil
}

// agentEventDecoder follows pi's JSON event stream: turns, tool calls and
// the usage of every assistant message.
type agentEventDecoder struct {
	profile config.ModelProfile
	result  *AgentRunResult
	started bool
	failure error
}

func (d *agentEventDecoder) read(stream io.Reader) error {
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 64*1024), maxProviderResponseBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		if err := d.event(line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return unusableCLIResponse("pi's event stream exceeded its limits")
	}
	return nil
}

func (d *agentEventDecoder) event(data []byte) error {
	var event struct {
		Type    string          `json:"type"`
		Message json.RawMessage `json:"message"`
		IsError bool            `json:"isError"`
	}
	if json.Unmarshal(data, &event) != nil {
		return unusableCLIResponse("malformed pi event")
	}
	switch event.Type {
	case "agent_start":
		d.started = true
	case "turn_end":
		d.result.Turns++
	case "tool_execution_end":
		d.result.ToolCalls++
		if event.IsError {
			d.result.ToolErrors++
		}
	case "message_end":
		return d.message(event.Message)
	}
	return nil
}

func (d *agentEventDecoder) message(data json.RawMessage) error {
	var role struct {
		Role string `json:"role"`
	}
	if json.Unmarshal(data, &role) != nil {
		return unusableCLIResponse("malformed pi message")
	}
	// User and tool-result messages echo context; only the model's turns count.
	if role.Role != "assistant" {
		return nil
	}
	var message struct {
		StopReason   string `json:"stopReason"`
		ErrorMessage string `json:"errorMessage"`
		Model        string `json:"model"`
		Content      []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage *struct {
			Input  int `json:"input"`
			Output int `json:"output"`
			Total  int `json:"totalTokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &message) != nil {
		return unusableCLIResponse("malformed pi assistant message")
	}
	if !matchesPiModel(d.profile.Model, message.Model) {
		return unusableCLIResponse("pi returned a different model than configured; use an exact model ID")
	}
	if message.Usage == nil {
		d.result.UsageKnown = false
	} else {
		d.result.Usage.PromptTokens += message.Usage.Input
		d.result.Usage.CompletionTokens += message.Usage.Output
		d.result.Usage.TotalTokens += message.Usage.Total
	}
	var text strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	if strings.TrimSpace(text.String()) != "" {
		d.result.Final = text.String()
	}
	switch message.StopReason {
	case "error":
		d.failure = piCompletionError(message.StopReason, message.ErrorMessage)
	case "stop", "toolUse", "aborted", "length":
		d.failure = nil
	}
	return nil
}
