package agentruntime

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

const traceExportTimeout = 8 * time.Second

type TraceExport struct {
	TraceID, Snapshot     string
	Incomplete, Truncated bool
	Omitted               uint64
	Unfinished            int
	Spans                 []agentstate.TraceSpan
}

type TraceExportReceipt struct {
	State    string `json:"state"`
	Sent     int    `json:"sent"`
	Accepted *int   `json:"accepted,omitempty"`
	Rejected *int   `json:"rejected,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type otlpValue struct {
	String  string `json:"stringValue,omitempty"`
	Integer string `json:"intValue,omitempty"`
}
type otlpAttribute struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}
type otlpSpan struct {
	TraceID    string          `json:"traceId"`
	ID         string          `json:"spanId"`
	ParentID   string          `json:"parentSpanId,omitempty"`
	Name       string          `json:"name"`
	Kind       int             `json:"kind"`
	Started    string          `json:"startTimeUnixNano"`
	Ended      string          `json:"endTimeUnixNano"`
	Attributes []otlpAttribute `json:"attributes"`
	Status     struct {
		Code int `json:"code"`
	} `json:"status"`
}
type otlpScopeSpans struct {
	Scope struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"scope"`
	Spans []otlpSpan `json:"spans"`
}
type otlpResourceSpans struct {
	Resource struct {
		Attributes []otlpAttribute `json:"attributes"`
	} `json:"resource"`
	Scopes []otlpScopeSpans `json:"scopeSpans"`
}

func traceAttribute(key, value string) otlpAttribute {
	return otlpAttribute{Key: key, Value: otlpValue{String: value}}
}

// ExportTrace uses only this admitted projection. It never installs global
// exporters, detects resources, reads OTEL settings, or retries a POST.
func ExportTrace(ctx context.Context, destination config.AgentTelemetryExport, snapshot TraceExport, admit func(context.Context) error) (TraceExportReceipt, error) {
	if err := destination.Validate(); err != nil {
		return TraceExportReceipt{}, err
	}
	if destination.Endpoint == "" || admit == nil {
		return TraceExportReceipt{}, fmt.Errorf("trace export requires a configured destination and fresh admission")
	}
	body, count, err := encodeTraceExport(snapshot)
	if err != nil {
		return TraceExportReceipt{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, traceExportTimeout)
	defer cancel()
	request, err := traceExportRequest(ctx, destination, body)
	if err != nil {
		return TraceExportReceipt{}, err
	}
	if err := admit(ctx); err != nil {
		return TraceExportReceipt{}, err
	}
	if err := context.Cause(ctx); err != nil {
		return TraceExportReceipt{}, err
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext, TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 16 << 10, DisableCompression: true, DisableKeepAlives: true, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: traceExportTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	receipt := TraceExportReceipt{State: "unknown", Sent: count, Reason: "transport_error"}
	response, err := client.Do(request)
	if err != nil {
		receipt.Reason = traceExportFailure(ctx, receipt.Reason)
		return receipt, nil
	}
	defer response.Body.Close()
	return traceExportResponse(ctx, response, receipt), nil
}

func traceExportRequest(ctx context.Context, destination config.AgentTelemetryExport, body []byte) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, destination.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create trace export request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	if destination.BearerEnv != "" {
		token := os.Getenv(destination.BearerEnv)
		if token == "" || len(token) > 8192 || strings.IndexFunc(token, unicode.IsSpace) >= 0 || strings.IndexFunc(token, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("configured telemetry credential is unavailable or invalid")
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request, nil
}

func encodeTraceExport(snapshot TraceExport) ([]byte, int, error) {
	if !traceHex(snapshot.TraceID, 16) || !traceHex(snapshot.Snapshot, 32) || len(snapshot.Spans) == 0 || len(snapshot.Spans) > 256 || snapshot.Unfinished < 0 || snapshot.Unfinished > 256-len(snapshot.Spans) || snapshot.Omitted > 1<<50 {
		return nil, 0, fmt.Errorf("invalid bounded trace snapshot")
	}
	scope := otlpScopeSpans{Spans: make([]otlpSpan, 0, len(snapshot.Spans))}
	scope.Scope.Name, scope.Scope.Version = "mini-orca", "1"
	seen := map[string]bool{}
	for _, span := range snapshot.Spans {
		if seen[strings.ToLower(span.ID)] {
			return nil, 0, fmt.Errorf("duplicate trace export identity")
		}
		seen[strings.ToLower(span.ID)] = true
		value, err := projectOTLPSpan(snapshot.TraceID, span)
		if err != nil {
			return nil, 0, err
		}
		scope.Spans = append(scope.Spans, value)
	}
	resource := otlpResourceSpans{Scopes: []otlpScopeSpans{scope}}
	resource.Resource.Attributes = []otlpAttribute{traceAttribute("service.name", "mini-orca"), traceAttribute("mini_orca.snapshot", snapshot.Snapshot), traceAttribute("mini_orca.history_incomplete", strconv.FormatBool(snapshot.Incomplete)), traceAttribute("mini_orca.history_truncated", strconv.FormatBool(snapshot.Truncated)), {Key: "mini_orca.omitted_spans", Value: otlpValue{Integer: strconv.FormatUint(snapshot.Omitted, 10)}}, {Key: "mini_orca.unfinished_spans", Value: otlpValue{Integer: strconv.Itoa(snapshot.Unfinished)}}}
	data, err := json.Marshal(struct {
		Resources []otlpResourceSpans `json:"resourceSpans"`
	}{[]otlpResourceSpans{resource}})
	if err != nil || len(data) > 1<<20 {
		return nil, 0, fmt.Errorf("trace export encoding exceeds its bounded format")
	}
	return data, len(scope.Spans), nil
}

func traceHex(value string, size int) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == size && strings.Trim(value, "0") != ""
}

func validOTLPSpan(span agentstate.TraceSpan) bool {
	if !traceHex(span.ID, 8) || strings.EqualFold(span.ParentID, span.ID) || span.ElapsedNanos < 0 || !slices.Contains([]string{"invocation", "node", "tool", "provider", "retry", "summary", "delegation", "check"}, span.Kind) {
		return false
	}
	if span.ParentID != "" && !traceHex(span.ParentID, 8) {
		return false
	}
	for _, key := range []string{span.RunKey, span.OperationKey} {
		if key != "" && !traceHex(key, 32) {
			return false
		}
	}
	return slices.Contains([]string{"completed", "failed", "canceled", "timed_out"}, span.Status) && validOTLPTime(span.Started) && validOTLPTime(span.Ended) && !span.Ended.Before(span.Started)
}

func validOTLPTime(value time.Time) bool {
	return value.UnixNano() > 0 && time.Unix(0, value.UnixNano()).Equal(value)
}

func projectOTLPSpan(traceID string, span agentstate.TraceSpan) (otlpSpan, error) {
	if !validOTLPSpan(span) {
		return otlpSpan{}, fmt.Errorf("trace export requires validated finished source-free spans")
	}
	value := otlpSpan{TraceID: traceID, ID: span.ID, ParentID: span.ParentID, Name: "mini_orca." + span.Kind, Kind: 1, Started: strconv.FormatInt(span.Started.UnixNano(), 10), Ended: strconv.FormatInt(span.Ended.UnixNano(), 10)}
	if span.Kind == "provider" || span.Kind == "delegation" {
		value.Kind = 3
	}
	value.Status.Code = 2
	if span.Status == "completed" {
		value.Status.Code = 1
	}
	value.Attributes = []otlpAttribute{traceAttribute("mini_orca.outcome", span.Status), {Key: "mini_orca.elapsed_nanos", Value: otlpValue{Integer: strconv.FormatInt(span.ElapsedNanos, 10)}}}
	for _, pair := range [][2]string{{"mini_orca.run_key", span.RunKey}, {"mini_orca.operation_key", span.OperationKey}} {
		if pair[1] != "" {
			value.Attributes = append(value.Attributes, traceAttribute(pair[0], pair[1]))
		}
	}
	return value, nil
}

func traceExportFailure(ctx context.Context, fallback string) string {
	if errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	if ctx.Err() != nil {
		return "canceled"
	}
	return fallback
}

func traceExportResponse(ctx context.Context, response *http.Response, receipt TraceExportReceipt) TraceExportReceipt {
	receipt.Reason = "unexpected_response"
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode != http.StatusOK || err != nil || media != "application/json" || response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
		return receipt
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	receipt.Reason = traceExportFailure(ctx, "invalid_acknowledgement")
	if err != nil || len(data) > 64<<10 {
		return receipt
	}
	rejected, warning, err := decodeTraceAcknowledgement(data, receipt.Sent)
	if err != nil {
		return receipt
	}
	accepted := receipt.Sent - rejected
	receipt.Accepted, receipt.Rejected, receipt.State, receipt.Reason = &accepted, &rejected, "exported", ""
	if warning {
		receipt.State, receipt.Reason = "warning", "collector_warning"
	}
	if rejected > 0 {
		receipt.State, receipt.Reason = "partial", "collector_rejected"
	}
	return receipt
}

func decodeTraceAcknowledgement(data []byte, sent int) (int, bool, error) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(data, &value); err != nil || value == nil {
		return 0, false, fmt.Errorf("invalid trace acknowledgement")
	}
	partial, ok := value["partialSuccess"]
	if !ok || string(partial) == "null" {
		return 0, false, nil
	}
	var reply struct {
		Rejected json.RawMessage `json:"rejectedSpans"`
		Message  string          `json:"errorMessage"`
	}
	if err := json.Unmarshal(partial, &reply); err != nil {
		return 0, false, err
	}
	rejected, err := traceRejectedCount(reply.Rejected, sent)
	return rejected, reply.Message != "", err
}

func traceRejectedCount(data json.RawMessage, sent int) (int, error) {
	if len(data) == 0 || string(data) == "null" {
		return 0, nil
	}
	value := string(data)
	if data[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return 0, err
		}
	}
	if err := validateTraceCountEncoding(value); err != nil {
		return 0, err
	}
	// Preserve exact integer semantics for both numeric and string ProtoJSON
	// values, including exponents, without rounding through float64.
	count, ok := new(big.Rat).SetString(value)
	if !ok || !count.IsInt() || !count.Num().IsInt64() || count.Sign() < 0 || count.Num().Int64() > int64(sent) {
		return 0, fmt.Errorf("invalid rejected trace count")
	}
	return int(count.Num().Int64()), nil
}

func validateTraceCountEncoding(value string) error {
	if len(value) > 128 || !json.Valid([]byte(value)) {
		return fmt.Errorf("invalid rejected trace count")
	}
	if i := strings.IndexAny(value, "eE"); i >= 0 {
		exponent, err := strconv.ParseInt(value[i+1:], 10, 16)
		if err != nil || exponent < -100 || exponent > 100 {
			return fmt.Errorf("invalid rejected trace exponent")
		}
	}
	return nil
}
