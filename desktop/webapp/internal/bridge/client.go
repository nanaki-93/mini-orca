package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxResponseBytes = 32 << 20

type Response struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Client carries the same local HTTP contract as the Compose transport. Only
// compiled application code can reach it; no HTTP bridge is exposed to browsers.
type Client struct {
	base      string
	http      *http.Client
	mu        sync.Mutex
	requests  map[string]context.CancelFunc
	canceled  map[string]time.Time
	closed    bool
	projectMu sync.Mutex
	root      string
	terminals *Terminals
}

func NewClient(base string) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("MINI_ORCA_URL must be an HTTP(S) daemon origin")
	}
	return &Client{
		base: strings.TrimSuffix(base, "/"),
		http: &http.Client{Timeout: 6 * time.Minute, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("daemon redirects are not allowed")
		}},
		requests: make(map[string]context.CancelFunc), canceled: make(map[string]time.Time), terminals: NewTerminals(),
	}, nil
}

var routes = map[string]*regexp.Regexp{
	"GET":    regexp.MustCompile(`^/(health|status|api/models/current|api/projects/current/(context|overview|findings|scan|execution-trust|index|files/(info|symbols|analysis)|impact|git|performance|performance/context|analysis/(selection|run|results)|drafts/[^/]+/benchmarks))$`),
	"POST":   regexp.MustCompile(`^/api/projects/(import|restore|current/(reindex|scan|execution-trust|files/(analysis|security-scan|explanation)|security-review|analysis/(selection|preview|run|run/control)|chat/sessions(/[^/]+/messages)?|drafts/[^/]+/(validate|checks|benchmarks)|apply|undo))$`),
	"PATCH":  regexp.MustCompile(`^/api/projects/current/(drafts|findings)/[^/]+$`),
	"DELETE": regexp.MustCompile(`^/api/projects/current/scan$`),
}

func allowedRequest(method, target, body string) bool {
	u, err := url.ParseRequestURI(target)
	pattern := routes[method]
	return err == nil && pattern != nil && u.Scheme == "" && u.Host == "" && u.Fragment == "" && u.RawPath == "" &&
		!strings.Contains(u.Path, "..") && pattern.MatchString(u.Path) && len(body) <= 2<<20 &&
		((method != "GET" && method != "DELETE") || body == "") && (body == "" || json.Valid([]byte(body)))
}

func (c *Client) Request(parent context.Context, id, method, target, body string) (Response, error) {
	if id == "" || len(id) > 128 || !allowedRequest(method, target, body) {
		return Response{}, fmt.Errorf("unsupported daemon request")
	}
	ctx, cancel := context.WithCancel(parent)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		cancel()
		return Response{}, fmt.Errorf("desktop is closing")
	}
	if _, canceled := c.canceled[id]; canceled {
		delete(c.canceled, id)
		c.mu.Unlock()
		cancel()
		return Response{}, context.Canceled
	}
	if _, exists := c.requests[id]; exists {
		c.mu.Unlock()
		cancel()
		return Response{}, fmt.Errorf("request identity is already active")
	}
	c.requests[id] = cancel
	c.mu.Unlock()
	defer func() {
		cancel()
		c.mu.Lock()
		delete(c.requests, id)
		c.mu.Unlock()
	}()
	if target == "/api/projects/import" || target == "/api/projects/restore" {
		c.projectMu.Lock()
		defer c.projectMu.Unlock()
		if err := c.terminals.CloseAll(); err != nil {
			return Response{}, fmt.Errorf("close terminal sessions before switching projects: %w", err)
		}
		response, err := c.send(ctx, method, target, body)
		if err == nil && response.Status >= 200 && response.Status < 300 {
			var project struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(response.Body), &project); err != nil || project.Path == "" {
				c.root = ""
				return Response{}, fmt.Errorf("daemon returned an invalid project identity")
			}
			c.root = project.Path
		}
		return response, err
	}
	return c.send(ctx, method, target, body)
}

func (c *Client) send(ctx context.Context, method, target, body string) (Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.base+target, bytes.NewBufferString(body))
	if err != nil {
		return Response{}, fmt.Errorf("prepare daemon request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return Response{}, fmt.Errorf("daemon request failed: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return Response{}, fmt.Errorf("read daemon response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return Response{}, fmt.Errorf("daemon response exceeds 32 MiB")
	}
	return Response{Status: response.StatusCode, Body: string(data)}, nil
}

func (c *Client) Cancel(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cancel := c.requests[id]; cancel != nil {
		cancel()
		return
	}
	// IPC methods may reach Go in a different order. Keep a bounded tombstone
	// so cancellation that beats Request still prevents network dispatch.
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

func (c *Client) OpenTerminal(expectedRoot string, cols, rows int) (TerminalUpdate, error) {
	c.projectMu.Lock()
	defer c.projectMu.Unlock()
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return TerminalUpdate{}, fmt.Errorf("desktop is closing")
	}
	if c.root == "" || expectedRoot != c.root {
		return TerminalUpdate{}, fmt.Errorf("terminal project changed; reopen the project")
	}
	root, err := filepath.EvalSymlinks(c.root)
	if err != nil {
		return TerminalUpdate{}, fmt.Errorf("resolve terminal directory: %w", err)
	}
	return c.terminals.Open(root, cols, rows)
}

func (c *Client) Terminals() *Terminals { return c.terminals }

func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	for _, cancel := range c.requests {
		cancel()
	}
	c.mu.Unlock()
	// Project activation and shell startup must finish before the final sweep.
	c.projectMu.Lock()
	err := c.terminals.CloseAll()
	c.projectMu.Unlock()
	if err != nil {
		c.mu.Lock()
		c.closed = false
		c.mu.Unlock()
	}
	return err
}
