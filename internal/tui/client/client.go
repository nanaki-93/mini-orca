package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	maxRequestBytes  = 2 << 20
	maxResponseBytes = 32 << 20
)

// Response holds the HTTP status code and raw response body from the daemon.
type Response struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Client communicates with the local Mini-Orca daemon over guarded HTTP routes.
type Client struct {
	base     string
	http     *http.Client
	mu       sync.Mutex
	requests map[string]context.CancelFunc
	canceled map[string]time.Time
	closed   bool
	root     string
}

var allowedRoutes = map[string]*regexp.Regexp{
	http.MethodGet:    regexp.MustCompile(`^/(health|status|api/models/(current|available)|api/projects/current|api/projects/current/(missions/[^/]+/execution|attachments|(live-sessions|artifacts|memory|missions)(/[^/]+)?|api-actions|research|delegations(/[^/]+)?|integrations|apis|remote-agents|agent-access|cloud-copies|agent-settings|agent-triggers|agent-runs(/[^/]+)?|events|changes(/(recovery|[^/]+))?|instructions|features|context|overview|findings|scan|execution-trust|index|files/(info|analysis)|impact|git(/(repository|workspace))?|analysis/(selection|run|results)))$`),
	http.MethodPost:   regexp.MustCompile(`^/api/projects/(import|restore|current/(attachments(/advice)?|live-sessions|missions/[^/]+/execution|(memory|missions)(/[^/]+)?|research|delegations|((live-sessions|artifacts|api-actions|delegations|integrations|apis|remote-agents|agent-access)/[^/]+/control)|cloud-copies/(preview|control)|agent-triggers/control|agent-settings/(control|skills/validate)|agent-runs/[^/]+/(control|input|export)|bootstrap/exec|git/(worktrees|switch|merge-preview|merge|abort-merge|discard-worktree|cleanup-merged)|changes(/[^/]+/(resume|messages|checks|review|review-file|plan|apply|commit|undo|verify|workflow(/(cancel|input))?))?|plans/investigate|instructions/proposal|features/(goals|generate)|reindex|scan|execution-trust|files/(analysis|security-scan|explanation)|security-review|analysis/(selection|preview|run|run/control)))$`),
	http.MethodPatch:  regexp.MustCompile(`^/api/projects/current/(findings|features)/[^/]+$`),
	http.MethodDelete: regexp.MustCompile(`^/api/projects/current/scan$`),
}

// NewClient validates the daemon origin URL and constructs a guarded Client.
func NewClient(base string) (*Client, error) {
	if !validDaemonOrigin(base) {
		return nil, fmt.Errorf("MINI_ORCA_URL must be an HTTP(S) daemon origin")
	}
	return &Client{
		base: strings.TrimSuffix(base, "/"),
		http: &http.Client{
			Timeout: 6 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        64,
				MaxIdleConnsPerHost: 32,
				IdleConnTimeout:     90 * time.Second,
			},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return fmt.Errorf("daemon redirects are not allowed")
			},
		},
		requests: make(map[string]context.CancelFunc),
		canceled: make(map[string]time.Time),
	}, nil
}

func validDaemonOrigin(base string) bool {
	u, err := url.Parse(base)
	if err != nil || u == nil || u.Host == "" || u.User != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.RawQuery == "" && u.Fragment == "" && (u.Path == "" || u.Path == "/")
}

func allowedRequest(method, target, body string) bool {
	u, err := url.ParseRequestURI(target)
	pattern := allowedRoutes[method]
	if err != nil || pattern == nil || !validTargetURI(u) || !pattern.MatchString(u.Path) {
		return false
	}
	return validRequestBody(method, body)
}

func validTargetURI(u *url.URL) bool {
	if u.Scheme != "" || u.Host != "" || u.Fragment != "" || u.RawPath != "" {
		return false
	}
	return !strings.Contains(u.Path, "..")
}

func validRequestBody(method, body string) bool {
	if len(body) > maxRequestBytes {
		return false
	}
	if (method == http.MethodGet || method == http.MethodDelete) && body != "" {
		return false
	}
	return body == "" || json.Valid([]byte(body))
}

// Request dispatches a guarded HTTP request with caller-controlled cancellation.
func (c *Client) Request(parent context.Context, id, method, target, body string) (Response, error) {
	if id == "" || len(id) > 128 || !allowedRequest(method, target, body) {
		return Response{}, fmt.Errorf("unsupported daemon request")
	}
	ctx, cancel, err := c.registerRequest(parent, id)
	if err != nil {
		return Response{}, err
	}
	defer c.unregisterRequest(id, cancel)

	resp, err := c.send(ctx, method, target, body)
	if err == nil && (target == "/api/projects/import" || target == "/api/projects/restore") {
		if err := c.recordProjectRoot(resp); err != nil {
			return Response{}, err
		}
	}
	return resp, err
}

func (c *Client) registerRequest(parent context.Context, id string) (context.Context, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(parent)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		cancel()
		return nil, nil, fmt.Errorf("tui client is closing")
	}
	if _, wasCanceled := c.canceled[id]; wasCanceled {
		delete(c.canceled, id)
		cancel()
		return nil, nil, context.Canceled
	}
	if _, exists := c.requests[id]; exists {
		cancel()
		return nil, nil, fmt.Errorf("request identity is already active")
	}
	c.requests[id] = cancel
	return ctx, cancel, nil
}

func (c *Client) unregisterRequest(id string, cancel context.CancelFunc) {
	cancel()
	c.mu.Lock()
	delete(c.requests, id)
	c.mu.Unlock()
}

func (c *Client) recordProjectRoot(resp Response) error {
	if resp.Status < 200 || resp.Status >= 300 {
		return nil
	}
	var proj struct {
		Path string `json:"path"`
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := json.Unmarshal([]byte(resp.Body), &proj); err != nil || proj.Path == "" {
		c.root = ""
		return fmt.Errorf("daemon returned an invalid project identity")
	}
	c.root = proj.Path
	return nil
}

// Root returns the last verified project root path from import or restore.
func (c *Client) Root() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.root
}

func (c *Client) send(ctx context.Context, method, target, body string) (Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+target, bytes.NewBufferString(body))
	if err != nil {
		return Response{}, fmt.Errorf("prepare daemon request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	httpClient := c.clientForRoute(method, req.URL.Path)
	resp, err := httpClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("daemon request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return Response{}, fmt.Errorf("read daemon response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return Response{}, fmt.Errorf("daemon response exceeds 32 MiB")
	}
	return Response{Status: resp.StatusCode, Body: string(data)}, nil
}

func (c *Client) clientForRoute(method, path string) *http.Client {
	if method == http.MethodPost && path == "/api/projects/current/features/generate" {
		clone := *c.http
		clone.Timeout = 0
		return &clone
	}
	return c.http
}

// Cancel cancels an in-flight request or records a tombstone if it has not started yet.
func (c *Client) Cancel(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cancel := c.requests[id]; cancel != nil {
		cancel()
		return
	}
	if id == "" || len(id) > 128 {
		return
	}
	for key, created := range c.canceled {
		if time.Since(created) > time.Minute {
			delete(c.canceled, key)
		}
	}
	if len(c.canceled) < 1024 {
		c.canceled[id] = time.Now()
	}
}

// Close marks the client closed and cancels all active requests.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	for _, cancel := range c.requests {
		cancel()
	}
	return nil
}

// DoJSON sends a JSON request and decodes a 2xx response into out (if non-nil).
func (c *Client) DoJSON(ctx context.Context, id, method, target string, payload, out any) error {
	var body string
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = string(raw)
	}
	resp, err := c.Request(ctx, id, method, target, body)
	if err != nil {
		return err
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return fmt.Errorf("%s", DecodeDaemonError(resp.Status, resp.Body))
	}
	if out != nil && strings.TrimSpace(resp.Body) != "" {
		if err := json.Unmarshal([]byte(resp.Body), out); err != nil {
			return fmt.Errorf("decode daemon response: %w", err)
		}
	}
	return nil
}

// DecodeDaemonError extracts an actionable message from a daemon error payload.
func DecodeDaemonError(status int, body string) string {
	var envelope struct {
		Error       string `json:"error"`
		Message     string `json:"message"`
		UserMessage string `json:"user_message"`
		Detail      string `json:"detail"`
		Details     struct {
			Hint string `json:"hint"`
		} `json:"details"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err == nil {
		msg := FirstNonEmpty(envelope.Message, envelope.Error)
		hint := FirstNonEmpty(envelope.Detail, envelope.Details.Hint, envelope.UserMessage)
		if hint != "" && !strings.Contains(msg, hint) {
			if msg != "" {
				return msg + " — " + hint
			}
			return hint
		}
		if msg != "" {
			return msg
		}
	}
	trimmed := strings.TrimSpace(body)
	if trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("daemon returned HTTP %d", status)
}

// FirstNonEmpty returns the first non-whitespace string from values.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
