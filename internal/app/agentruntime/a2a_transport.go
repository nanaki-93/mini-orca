package agentruntime

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ErrRemoteAuth = errors.New("remote agent authentication is required")
var ErrRemoteTransport = errors.New("remote agent exchange did not complete")
var ErrRemoteProtocol = errors.New("remote agent response is unsupported or invalid")

type RemoteConnection struct {
	CardURL, Endpoint, BearerToken string
	Skills                         []string
	Timeout                        time.Duration
}

type remoteCleanupKey struct{}

// remoteHTTP owns one primary exchange and at most one cancellation exchange.
// A fresh HTTP/1 transport cannot retry a reused connection or HTTP/2 stream.
type remoteHTTP struct {
	connection                  RemoteConnection
	primaryMethod               string
	authorize                   func(context.Context) error
	base                        *http.Transport
	mu                          sync.Mutex
	primary, cleanup, attempted bool
}

func newRemoteHTTP(connection RemoteConnection, method string, authorize func(context.Context) error) (*http.Client, *remoteHTTP, error) {
	if authorize == nil || connection.Timeout <= 0 || connection.Timeout > time.Minute || strings.ContainsAny(connection.BearerToken, "\x00\r\n") {
		return nil, nil, ErrPolicyDenied
	}
	card, err := apiEndpoint(connection.CardURL)
	if err != nil {
		return nil, nil, err
	}
	endpoint, err := apiEndpoint(connection.Endpoint)
	if err != nil || card.Scheme != endpoint.Scheme || card.Host != endpoint.Host {
		return nil, nil, ErrPolicyDenied
	}
	transport := &remoteHTTP{connection: connection, primaryMethod: method, authorize: authorize, base: &http.Transport{Proxy: nil, DisableKeepAlives: true, DisableCompression: true, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: connection.Timeout, MaxResponseHeaderBytes: 16 << 10}}
	client := &http.Client{Transport: transport, Timeout: connection.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrPolicyDenied }}
	return client, transport, nil
}

func (t *remoteHTTP) RoundTrip(request *http.Request) (*http.Response, error) {
	method, id, err := t.admit(request)
	if err != nil {
		return nil, err
	}
	copy := request.Clone(request.Context())
	copy.GetBody = nil
	copy.Header = make(http.Header)
	copy.Header.Set("Content-Type", "application/json")
	copy.Header.Set("Accept", request.Header.Get("Accept"))
	copy.Header.Set("A2A-Version", "1.0")
	if t.connection.BearerToken != "" {
		copy.Header.Set("Authorization", "Bearer "+t.connection.BearerToken)
	}
	t.mu.Lock()
	cleanup, _ := request.Context().Value(remoteCleanupKey{}).(bool)
	if method != "card" && !cleanup {
		t.attempted = true
	}
	t.mu.Unlock()
	response, err := t.base.RoundTrip(copy)
	if err != nil {
		return nil, ErrRemoteTransport
	}
	return guardRemoteResponse(response, copy.Header.Get("Accept") == "text/event-stream", id)
}

func (t *remoteHTTP) admit(request *http.Request) (string, string, error) {
	method, id, err := t.requestMethod(request)
	if err != nil {
		return "", "", err
	}
	cleanup, _ := request.Context().Value(remoteCleanupKey{}).(bool)
	if !cleanup {
		if err := t.authorize(request.Context()); err != nil {
			return "", "", err
		}
	} else if method != "CancelTask" {
		return "", "", ErrPolicyDenied
	}
	budget := BudgetFromContext(request.Context())
	if budget == nil {
		return "", "", ErrPolicyDenied
	}
	if err := budget.AdmitTool(request.Context()); err != nil {
		return "", "", err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if cleanup {
		if t.cleanup {
			return "", "", ErrPolicyDenied
		}
		t.cleanup = true
	} else {
		if method != t.primaryMethod || t.primary {
			return "", "", ErrPolicyDenied
		}
		t.primary = true
	}
	return method, id, nil
}

func (t *remoteHTTP) requestMethod(request *http.Request) (string, string, error) {
	if request.Method == "GET" && request.URL.String() == t.connection.CardURL {
		return "card", "", nil
	}
	if request.Method != "POST" || request.URL.String() != t.connection.Endpoint || request.Body == nil {
		return "", "", ErrPolicyDenied
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, (1<<20)+1))
	closeErr := request.Body.Close()
	if err != nil || closeErr != nil || len(data) > 1<<20 {
		return "", "", ErrPolicyDenied
	}
	var envelope struct {
		Method  string          `json:"method"`
		Version string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Params  json.RawMessage `json:"params"`
	}
	if apiDecode(data, &envelope) != nil || envelope.Version != "2.0" || !remoteIdentity(envelope.ID) {
		return "", "", ErrPolicyDenied
	}
	request.Body = io.NopCloser(bytes.NewReader(data))
	return envelope.Method, envelope.ID, nil
}

func (t *remoteHTTP) wasAttempted() bool { t.mu.Lock(); defer t.mu.Unlock(); return t.attempted }

func guardRemoteResponse(response *http.Response, stream bool, id string) (*http.Response, error) {
	if response.StatusCode != 200 {
		_ = response.Body.Close()
		if response.StatusCode == 401 || response.StatusCode == 403 {
			return nil, ErrRemoteAuth
		}
		return nil, ErrRemoteTransport
	}
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	expected := "application/json"
	if stream {
		expected = "text/event-stream"
	}
	if err != nil || media != expected {
		_ = response.Body.Close()
		return nil, ErrRemoteProtocol
	}
	if stream {
		scanner := bufio.NewScanner(io.LimitReader(response.Body, (512<<10)+1))
		scanner.Buffer(make([]byte, 4096), 64<<10)
		response.Body = &remoteStreamBody{body: response.Body, scanner: scanner, id: id}
		return response, nil
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	closeErr := response.Body.Close()
	if err != nil || closeErr != nil || len(data) > 64<<10 || !jsontext.Value(data).IsValid() {
		return nil, ErrRemoteProtocol
	}
	if id != "" && validRemoteRPC(data, id) != nil {
		return nil, ErrRemoteProtocol
	}
	response.Body = io.NopCloser(bytes.NewReader(data))
	return response, nil
}

type remoteStreamBody struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	pending bytes.Buffer
	total   int
	id      string
}

func (r *remoteStreamBody) Read(buffer []byte) (int, error) {
	if r.pending.Len() == 0 {
		if err := r.frame(); err != nil {
			return 0, err
		}
	}
	return r.pending.Read(buffer)
}

func (r *remoteStreamBody) frame() error {
	var frame, data strings.Builder
	for r.scanner.Scan() {
		line := r.scanner.Text()
		r.total += len(line) + 1
		if r.total > 512<<10 || frame.Len()+len(line) > 64<<10 {
			return fmt.Errorf("%w: remote stream exceeded its byte limit", ErrRemoteProtocol)
		}
		frame.WriteString(line + "\n")
		if strings.HasPrefix(line, "data:") {
			data.WriteString(strings.TrimPrefix(line[5:], " ") + "\n")
		}
		if line == "" {
			if data.Len() > 0 && validRemoteRPC([]byte(data.String()), r.id) != nil {
				return ErrRemoteProtocol
			}
			r.pending.WriteString(frame.String())
			return nil
		}
	}
	if r.scanner.Err() != nil || frame.Len() > 0 {
		return ErrRemoteProtocol
	}
	return io.EOF
}
func (r *remoteStreamBody) Close() error { return r.body.Close() }

func validRemoteRPC(data []byte, id string) error {
	var envelope struct {
		Version string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if apiDecode(data, &envelope) != nil || envelope.Version != "2.0" || envelope.ID != id || (len(envelope.Result) == 0) == (len(envelope.Error) == 0) {
		return ErrRemoteProtocol
	}
	return nil
}
