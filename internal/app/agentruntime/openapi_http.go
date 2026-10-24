package agentruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type APIConnection struct {
	Endpoint, BearerToken string
	Timeout               time.Duration
	Begin                 func(context.Context) (context.Context, func(), error)
}

// APIAction is the exact source-bearing request presented for human review.
// It never includes credentials or an implicitly acquired authentication scope.
type APIAction struct {
	OperationID  string          `json:"operation_id"`
	OperationKey string          `json:"operation_key"`
	Method       string          `json:"method"`
	URL          string          `json:"url"`
	Body         json.RawMessage `json:"body,omitempty"`
}

func (a APIAction) Fingerprint() string {
	data, _ := json.Marshal(a)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

type APIResponse struct {
	Status    int
	Data      json.RawMessage
	Attempted bool
}

func apiEndpoint(endpoint string) (*url.URL, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || u.Opaque != "" {
		return nil, fmt.Errorf("invalid configured API destination")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return nil, fmt.Errorf("API requires HTTPS except literal loopback")
	}
	return u, nil
}

func (operation APIOperation) Prepare(endpoint string, args json.RawMessage) (APIAction, error) {
	var result APIAction
	if len(args) > maxToolPayload {
		return result, fmt.Errorf("API arguments exceed byte limit")
	}
	var values map[string]any
	if err := apiDecode(args, &values); err != nil {
		return result, err
	}
	schema, err := resolveToolSchema(operation.InputSchema)
	if err != nil {
		return result, err
	}
	if err := schema.Validate(values); err != nil {
		return result, fmt.Errorf("API arguments do not match the admitted schema")
	}
	u, err := apiEndpoint(endpoint)
	if err != nil {
		return result, err
	}
	path, query, err := apiArgumentURL(operation, values)
	if err != nil {
		return result, err
	}
	u.Path, u.RawQuery = strings.TrimSuffix(u.Path, "/")+path, query.Encode()
	if len(u.String()) > 4096 {
		return result, fmt.Errorf("API URL exceeds byte limit")
	}
	result = APIAction{OperationID: operation.ID, Method: operation.Method, URL: u.String()}
	identity, _ := json.Marshal([]string{endpoint, operation.Method, operation.Path})
	key := sha256.Sum256(identity)
	result.OperationKey = hex.EncodeToString(key[:])
	if body, exists := values["body"]; exists {
		result.Body, err = json.Marshal(body)
		if err != nil {
			return APIAction{}, err
		}
	}
	return result, nil
}

func apiArgumentURL(operation APIOperation, values map[string]any) (string, url.Values, error) {
	path, query := operation.Path, url.Values{}
	for _, parameter := range operation.Parameters {
		group, _ := values[parameter.In].(map[string]any)
		value, exists := group[parameter.Name]
		if !exists {
			continue
		}
		text, err := apiScalar(value)
		if err != nil {
			return "", nil, err
		}
		if parameter.In == "query" {
			query.Set(parameter.Name, text)
			continue
		}
		if text == "" || text == "." || text == ".." || strings.ContainsAny(text, "/\\%\x00\r\n") {
			return "", nil, fmt.Errorf("unsafe path parameter")
		}
		path = strings.ReplaceAll(path, "{"+parameter.Name+"}", text)
	}
	if strings.ContainsAny(path, "{}") {
		return "", nil, fmt.Errorf("unresolved path parameter")
	}
	return path, query, nil
}

func apiScalar(value any) (string, error) {
	switch value := value.(type) {
	case string:
		if len(value) <= 2048 && !strings.ContainsAny(value, "\x00\r\n") {
			return value, nil
		}
	case json.Number:
		return value.String(), nil
	case bool:
		return strconv.FormatBool(value), nil
	}
	return "", fmt.Errorf("API parameter must be a bounded scalar")
}

// ExecuteAPI performs one HTTP exchange. An attempted write with any exchange
// or validation error has an unknown outcome; callers must retain its receipt.
func ExecuteAPI(ctx context.Context, connection APIConnection, operation APIOperation, arguments json.RawMessage, expected string, authorize func(context.Context) error) (result APIResponse, err error) {
	if authorize == nil || connection.Timeout <= 0 || connection.Timeout > time.Minute {
		return result, ErrPolicyDenied
	}
	bounded, cancel := context.WithTimeout(ctx, connection.Timeout)
	defer cancel()
	if connection.Begin != nil {
		var finish func()
		bounded, finish, err = connection.Begin(bounded)
		if err != nil {
			return result, err
		}
		defer finish()
	}
	if err := authorize(bounded); err != nil {
		return result, err
	}
	action, err := operation.Prepare(connection.Endpoint, arguments)
	if err != nil || action.Fingerprint() != expected {
		return result, ErrPolicyDenied
	}
	if err := validateAPIAction(connection, operation, action); err != nil {
		return result, err
	}
	request, err := apiHTTPRequest(bounded, action, connection.BearerToken)
	if err != nil {
		return result, err
	}
	// A fresh HTTP/1 transport cannot replay on a reused connection or through
	// HTTP/2 stream retries. No cookie jar, proxy, redirect or SDK retry exists.
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, DisableCompression: true, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, ResponseHeaderTimeout: connection.Timeout, TLSHandshakeTimeout: 10 * time.Second, MaxResponseHeaderBytes: 16 << 10}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: connection.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("API redirects are disabled") }}
	if err := authorize(bounded); err != nil {
		return result, err
	}
	result.Attempted = true
	response, err := client.Do(request)
	if err != nil {
		return result, fmt.Errorf("external API exchange failed")
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close external API response"))
		}
	}()
	result.Status = response.StatusCode
	result.Data, err = apiResponseData(response, operation)
	return result, err
}

func apiHTTPRequest(ctx context.Context, action APIAction, credential string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, action.Method, action.URL, bytes.NewReader(action.Body))
	if err != nil {
		return nil, fmt.Errorf("invalid API request")
	}
	request.GetBody = nil
	request.Header.Set("Accept", "application/json")
	if len(action.Body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	return request, nil
}

func validateAPIAction(connection APIConnection, operation APIOperation, action APIAction) error {
	base, err := apiEndpoint(connection.Endpoint)
	if err != nil {
		return err
	}
	u, err := url.Parse(action.URL)
	if err != nil || u.Scheme != base.Scheme || u.Host != base.Host || u.User != nil || u.Fragment != "" || action.OperationID != operation.ID || action.Method != operation.Method || len(action.Body) > maxToolPayload || len(action.URL) > 4096 {
		return ErrPolicyDenied
	}
	prefix := strings.TrimSuffix(base.Path, "/") + "/"
	if !strings.HasPrefix(u.Path, prefix) || strings.ContainsAny(connection.BearerToken, "\x00\r\n") {
		return ErrPolicyDenied
	}
	return nil
}

func apiResponseData(response *http.Response, operation APIOperation) (json.RawMessage, error) {
	schema, exists := operation.Responses[strconv.Itoa(response.StatusCode)]
	if !exists {
		return nil, fmt.Errorf("API returned an undeclared or unsuccessful status")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxToolPayload-1023))
	if err != nil || len(data) >= maxToolPayload-1023 {
		return nil, fmt.Errorf("API response failed or exceeded its byte limit")
	}
	if response.StatusCode == 204 && len(data) == 0 {
		return json.RawMessage(`{}`), nil
	}
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, fmt.Errorf("API response must be application/json")
	}
	var value map[string]any
	if err := apiDecode(data, &value); err != nil {
		return nil, fmt.Errorf("API response is invalid JSON")
	}
	contract, err := resolveToolSchema(schema)
	if err != nil {
		return nil, err
	}
	if err := contract.Validate(value); err != nil {
		return nil, fmt.Errorf("API response does not match the declared schema")
	}
	return data, nil
}
