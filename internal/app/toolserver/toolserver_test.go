package toolserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearer struct {
	token string
}

func (b bearer) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(request)
}

func echoTools() []Tool {
	return []Tool{
		{Name: "echo", Description: "Echo the text.", ReadOnly: true, Schema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
			Call: func(_ context.Context, arguments json.RawMessage) (string, error) {
				var input struct{ Text string }
				if err := json.Unmarshal(arguments, &input); err != nil {
					return "", err
				}
				return "echo: " + input.Text, nil
			}},
		{Name: "fail", Description: "Always fails.", Schema: json.RawMessage(`{"type":"object"}`),
			Call: func(context.Context, json.RawMessage) (string, error) { return "", errors.New("this tool refuses") }},
	}
}

func TestAToolServerServesItsToolsOnlyWithItsToken(t *testing.T) {
	server, err := Start(context.Background(), "Use these tools.", echoTools())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if !strings.HasPrefix(server.URL, "http://127.0.0.1:") || len(server.Token) != 64 {
		t.Fatalf("server = %s %d", server.URL, len(server.Token))
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: bearer{server.Token}}, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil || len(listed.Tools) != 2 || !listed.Tools[0].Annotations.ReadOnlyHint && !listed.Tools[1].Annotations.ReadOnlyHint {
		t.Fatalf("tools = %+v, %v", listed, err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"text": "hi"}})
	if err != nil || result.IsError || result.Content[0].(*mcp.TextContent).Text != "echo: hi" {
		t.Fatalf("echo = %+v, %v", result, err)
	}
	failed, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "fail"})
	if err != nil || !failed.IsError || failed.Content[0].(*mcp.TextContent).Text != "this tool refuses" {
		t.Fatalf("a failing tool must reach the model as a failed call: %+v, %v", failed, err)
	}

	for name, header := range map[string]string{"no token": "", "wrong token": "Bearer " + strings.Repeat("0", 64)} {
		request, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		request.Header.Set("Content-Type", "application/json")
		if header != "" {
			request.Header.Set("Authorization", header)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s: status %d", name, response.StatusCode)
		}
	}
}

func TestAToolServerStopsWithItsRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server, err := Start(ctx, "", echoTools())
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	<-server.done
	if _, err := http.Post(server.URL, "application/json", strings.NewReader("{}")); err == nil {
		t.Fatal("the server still answers after its run ended")
	}
	server.Close()
	if _, err := Start(context.Background(), "", []Tool{{Name: "bad", Schema: json.RawMessage(`{"type":"string"}`), Call: echoTools()[0].Call}}); err == nil {
		t.Fatal("a tool without an object schema was accepted")
	}
}
