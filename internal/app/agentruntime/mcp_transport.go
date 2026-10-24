package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpFrameLimit = 64 << 10

// MCPConnection carries an admitted endpoint or a host-owned process factory.
// Secrets and process handles never enter descriptors or session state.
type MCPConnection struct {
	Endpoint    string
	BearerToken string
	Timeout     time.Duration
	Start       func(context.Context) (io.ReadCloser, io.WriteCloser, func() error, error)
	Begin       func(context.Context) (context.Context, func(), error)
}

type singleMCPTransport struct {
	config  MCPConnection
	used    atomic.Bool
	mu      sync.Mutex
	cleanup func() error
}

func (t *singleMCPTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	if !t.used.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("MCP automatic reconnection is disabled")
	}
	if t.config.Start != nil {
		reader, writer, cleanup, err := t.config.Start(ctx)
		if err != nil {
			return nil, err
		}
		t.mu.Lock()
		t.cleanup = cleanup
		t.mu.Unlock()
		return (&mcp.IOTransport{Reader: reader, Writer: writer, MaxLineLength: mcpFrameLimit}).Connect(ctx)
	}
	endpoint, err := url.Parse(t.config.Endpoint)
	if err != nil || endpoint.Host == "" {
		return nil, fmt.Errorf("invalid admitted MCP endpoint")
	}
	base := &http.Transport{Proxy: nil, DisableCompression: true, ResponseHeaderTimeout: t.config.Timeout, TLSHandshakeTimeout: 10 * time.Second, MaxResponseHeaderBytes: mcpFrameLimit}
	t.mu.Lock()
	t.cleanup = func() error { base.CloseIdleConnections(); return nil }
	t.mu.Unlock()
	client := &http.Client{Timeout: t.config.Timeout, Transport: &mcpHTTPTransport{base: base, endpoint: endpoint, token: t.config.BearerToken}, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("MCP redirects are disabled") }}
	return (&mcp.StreamableClientTransport{Endpoint: endpoint.String(), HTTPClient: client, MaxRetries: -1, DisableStandaloneSSE: true, MaxEventSize: mcpFrameLimit}).Connect(ctx)
}

func (t *singleMCPTransport) close() error {
	t.mu.Lock()
	cleanup := t.cleanup
	t.cleanup = nil
	t.mu.Unlock()
	if cleanup != nil {
		return cleanup()
	}
	return nil
}

type mcpHTTPTransport struct {
	base     http.RoundTripper
	endpoint *url.URL
	token    string
}

func (t *mcpHTTPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	u := request.URL
	if u.String() != t.endpoint.String() || request.Host != "" && request.Host != t.endpoint.Host || request.Method != http.MethodPost && request.Method != http.MethodDelete {
		return nil, fmt.Errorf("MCP destination or method changed")
	}
	if request.ContentLength < 0 || request.ContentLength > mcpFrameLimit {
		return nil, fmt.Errorf("MCP request exceeds byte limit")
	}
	bound := request.Clone(request.Context())
	bound.Header.Del("Authorization")
	if t.token != "" {
		bound.Header.Set("Authorization", "Bearer "+t.token)
	}
	response, err := t.base.RoundTrip(bound)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		closeErr := response.Body.Close()
		return nil, errors.Join(fmt.Errorf("MCP HTTP status %d", response.StatusCode), closeErr)
	}
	response.Body = &nativeBoundedBody{ReadCloser: response.Body, remaining: mcpFrameLimit}
	return response, nil
}
