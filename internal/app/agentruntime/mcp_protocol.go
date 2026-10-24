package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This guard surrounds actual SDK dispatch, including native ADK's retry path.
// One short-lived session permits discovery and at most one outbound tool call.
type mcpProtocolGuard struct {
	mu        sync.Mutex
	requests  int
	pages     int
	bytes     int
	tools     map[string]*mcp.Tool
	callName  string
	called    bool
	authorize func(context.Context) error
}

func (g *mcpProtocolGuard) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		if err := g.admit(ctx, method, request); err != nil {
			return nil, err
		}
		result, err := next(ctx, method, request)
		if err != nil {
			return nil, err
		}
		switch method {
		case "tools/list":
			page, ok := result.(*mcp.ListToolsResult)
			if !ok || page == nil {
				return nil, fmt.Errorf("invalid MCP discovery result")
			}
			err = g.capture(page)
		case "tools/call":
			err = validateMCPResult(result)
		}
		return result, err
	}
}

func (g *mcpProtocolGuard) admit(ctx context.Context, method string, request mcp.Request) error {
	if err := g.authorize(ctx); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests++
	if g.requests > 16 {
		return fmt.Errorf("MCP protocol request limit reached")
	}
	switch method {
	case "initialize", "notifications/initialized", "notifications/cancelled", "ping":
		return nil
	case "tools/list":
		g.pages++
		if g.pages <= 4 {
			return nil
		}
	case "tools/call":
		params, ok := request.GetParams().(*mcp.CallToolParams)
		if ok && params != nil && params.Name == g.callName && !g.called {
			g.called = true
			return nil
		}
	}
	return fmt.Errorf("MCP operation requires fresh admission")
}

func (g *mcpProtocolGuard) capture(page *mcp.ListToolsResult) error {
	data, err := json.Marshal(page)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.bytes += len(data)
	if g.bytes > mcpFrameLimit || len(g.tools)+len(page.Tools) > 64 {
		return fmt.Errorf("MCP discovery exceeds inventory limit")
	}
	for _, item := range page.Tools {
		if item == nil || item.Name == "" || g.tools[item.Name] != nil {
			return fmt.Errorf("MCP inventory contains an invalid or duplicate tool")
		}
		g.tools[item.Name] = item
	}
	return nil
}

func validateMCPResult(result mcp.Result) error {
	value, ok := result.(*mcp.CallToolResult)
	if !ok || value == nil || value.IsError {
		return fmt.Errorf("MCP tool failed or returned an invalid result")
	}
	if !mcpSafeNumbers(value.StructuredContent) {
		return fmt.Errorf("MCP result integers outside the exact SDK range must use strings")
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) > maxToolPayload-256 {
		return fmt.Errorf("MCP result exceeds tool byte limit")
	}
	for _, content := range value.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok || text == nil {
			return fmt.Errorf("MCP tool returned unsupported media or resource content")
		}
	}
	return nil
}

// The pinned SDK decodes arbitrary JSON through float64. Reject integers at
// the precision boundary, where a distinct source integer may have rounded to
// this value; IDs outside that range must be encoded as strings by the server.
func mcpSafeNumbers(value any) bool {
	switch value := value.(type) {
	case float64:
		return !math.IsInf(value, 0) && !math.IsNaN(value) && (math.Trunc(value) != value || math.Abs(value) < 1<<53)
	case map[string]any:
		for _, child := range value {
			if !mcpSafeNumbers(child) {
				return false
			}
		}
	case []any:
		for _, child := range value {
			if !mcpSafeNumbers(child) {
				return false
			}
		}
	}
	return true
}
