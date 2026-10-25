package agentruntime

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

var ErrLiveTransport = errors.New("live provider connection unavailable or disconnected")
var ErrLiveProtocol = errors.New("live provider returned an unsupported or malformed event")
var ErrLiveGoAway = errors.New("live provider requested disconnection")
var ErrLiveClosed = errors.New("live connection explicitly closed")

const liveSocketPath = "/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent"
const liveHeaderBytes = 16 << 10

// The transport is owned by one invocation. It has no global dialer, proxy,
// redirect, retry or session-resumption state.
type liveWire struct {
	conn *websocket.Conn
	stop func() bool
}

type liveWireConn struct {
	net.Conn
	reading, writing atomic.Int64
	ready            atomic.Bool
	input, output    int64
}

func (c *liveWireConn) Read(p []byte) (int, error) {
	limit := int64(liveHeaderBytes)
	if c.ready.Load() {
		limit += c.output
	}
	remaining := limit - c.reading.Load()
	if remaining <= 0 {
		return 0, &BudgetError{Limit: "live incoming bytes"}
	}
	p = p[:min(int64(len(p)), remaining)]
	n, err := c.Conn.Read(p)
	c.reading.Add(int64(n))
	return n, err
}

func (c *liveWireConn) Write(p []byte) (int, error) {
	limit := int64(liveHeaderBytes)
	if c.ready.Load() {
		limit += c.input
	}
	if int64(len(p)) > limit-c.writing.Load() {
		return 0, &BudgetError{Limit: "live outgoing bytes"}
	}
	n, err := c.Conn.Write(p)
	c.writing.Add(int64(n))
	return n, err
}

func liveEndpoint(profile config.ModelProfile) (*url.URL, error) {
	if !validLiveCredentials(profile) {
		return nil, ErrPolicyDenied
	}
	u, err := url.Parse(profile.APIBaseURL)
	if err != nil || !llm.SupportsLiveSessions(profile) || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return nil, ErrPolicyDenied
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return nil, ErrPolicyDenied
		}
	}
	if err := liveSocketScheme(u); err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + liveSocketPath
	return u, nil
}

func liveSocketScheme(u *url.URL) error {
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		ip := net.ParseIP(u.Hostname())
		if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return ErrPolicyDenied
		}
		u.Scheme = "ws"
	default:
		return ErrPolicyDenied
	}
	return nil
}

func validLiveCredentials(profile config.ModelProfile) bool {
	return profile.APIKey != "" && len(profile.APIKey) <= 8192 && strings.TrimSpace(profile.APIKey) == profile.APIKey && !strings.ContainsAny(profile.APIKey, "\r\n\x00") && !strings.HasPrefix(profile.APIKey, "auth_tokens/") && profile.AccessToken == "" && profile.VertexProject == "" && profile.VertexLocation == ""
}

func connectLiveWire(ctx context.Context, profile config.ModelProfile, input, output int64) (*liveWire, error) {
	endpoint, err := liveEndpoint(profile)
	if err != nil {
		return nil, err
	}
	var bounded *liveWireConn
	var stop func() bool
	dial := func(dialCtx context.Context, network, address string, secure bool) (net.Conn, error) {
		conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(dialCtx, network, address)
		if err != nil {
			return nil, err
		}
		raw := conn
		stop = context.AfterFunc(ctx, func() { _ = raw.Close() })
		if secure {
			tlsConn := tls.Client(conn, &tls.Config{ServerName: endpoint.Hostname(), MinVersion: tls.VersionTLS12})
			if err := tlsConn.HandshakeContext(dialCtx); err != nil {
				stop()
				_ = conn.Close()
				return nil, err
			}
			conn = tlsConn
		}
		bounded = &liveWireConn{Conn: conn, input: input, output: output}
		return bounded, nil
	}
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second, ReadBufferSize: 4096, WriteBufferSize: 4096,
		NetDialContext:    func(ctx context.Context, n, a string) (net.Conn, error) { return dial(ctx, n, a, false) },
		NetDialTLSContext: func(ctx context.Context, n, a string) (net.Conn, error) { return dial(ctx, n, a, true) },
	}
	conn, response, err := dialer.DialContext(ctx, endpoint.String(), http.Header{"X-Goog-Api-Key": []string{profile.APIKey}})
	if err != nil {
		if stop != nil {
			stop()
		}
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		return nil, ErrLiveTransport
	}
	// Keep cancellation bound to the invocation, not DialContext's temporary
	// handshake context, which ends as soon as the connection opens.
	stop()
	bounded.ready.Store(true)
	conn.SetReadLimit(min(output, 128<<10))
	wire := &liveWire{conn: conn}
	wire.stop = context.AfterFunc(ctx, func() { _ = conn.Close() })
	return wire, nil
}

func (w *liveWire) close() {
	w.stop()
	_ = w.conn.Close()
}

func liveIOError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	if errors.Is(err, ErrBudgetExceeded) {
		return err
	}
	if errors.Is(err, websocket.ErrReadLimit) {
		return &BudgetError{Limit: "live frame bytes"}
	}
	return ErrLiveTransport
}
