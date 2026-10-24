package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func traceExportFixture() TraceExport {
	start := time.Unix(1700000000, 123).UTC()
	return TraceExport{TraceID: strings.Repeat("a", 32), Snapshot: strings.Repeat("b", 64), Incomplete: true, Truncated: true, Omitted: 3, Unfinished: 2,
		Spans: []agentstate.TraceSpan{{ID: "0123456789abcdef", ParentID: "abcdef0123456789", RunKey: TraceIdentity("PRIVATE_RUN"), OperationKey: TraceIdentity("PRIVATE_MODEL"), Kind: "provider", Status: "failed", Started: start, Ended: start.Add(time.Second), ElapsedNanos: 1e9}}}
}

func allowTraceExport(context.Context) error { return nil }

func TestTraceExportUsesOnlyAdmittedOTLPFieldsAndExplicitCredentials(t *testing.T) {
	var ambient, calls atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ambient.Add(1) }))
	defer other.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", other.URL)
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "secret=PRIVATE_RESOURCE")
	t.Setenv("TRACE_TOKEN", "PRIVATE_TOKEN")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/traces" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Authorization") != "Bearer PRIVATE_TOKEN" || strings.Contains(string(body), "PRIVATE_") || strings.Contains(string(body), other.URL) {
			t.Errorf("unsafe export envelope: %s", body)
		}
		var value struct {
			Resources []otlpResourceSpans `json:"resourceSpans"`
		}
		if err := json.Unmarshal(body, &value); err != nil {
			t.Error(err)
			return
		}
		if len(value.Resources) != 1 || len(value.Resources[0].Scopes) != 1 || len(value.Resources[0].Scopes[0].Spans) != 1 {
			t.Error("unexpected envelope")
			return
		}
		span := value.Resources[0].Scopes[0].Spans[0]
		attributes := value.Resources[0].Resource.Attributes
		if attributes[len(attributes)-1].Key != "mini_orca.unfinished_spans" || attributes[len(attributes)-1].Value.Integer != "2" {
			t.Error("unfinished span omission was not disclosed")
		}
		if span.TraceID != strings.Repeat("a", 32) || span.Kind != 3 || span.Status.Code != 2 || span.Started != "1700000000000000123" || span.Name != "mini_orca.provider" {
			t.Errorf("invalid OTLP mapping: %+v", span)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"futureField":{"ignored":true}}`)
	}))
	defer server.Close()
	got, err := ExportTrace(t.Context(), config.AgentTelemetryExport{Endpoint: server.URL + "/v1/traces", BearerEnv: "TRACE_TOKEN"}, traceExportFixture(), allowTraceExport)
	if err != nil || got.State != "exported" || got.Accepted == nil || *got.Accepted != 1 || *got.Rejected != 0 || calls.Load() != 1 || ambient.Load() != 0 {
		t.Fatalf("receipt=%+v calls=%d ambient=%d err=%v", got, calls.Load(), ambient.Load(), err)
	}
}

func TestTraceExportAcknowledgementPreservesPartialWarningAndUnknown(t *testing.T) {
	for _, tc := range []struct{ name, body, state string }{
		{"success", `{}`, "exported"},
		{"partial", `{"partialSuccess":{"rejectedSpans":"1","errorMessage":"PRIVATE_ERROR"}}`, "partial"},
		{"numeric", `{"partialSuccess":{"rejectedSpans":1}}`, "partial"},
		{"exponent", `{"partialSuccess":{"rejectedSpans":"1e0"}}`, "partial"},
		{"warning", `{"partialSuccess":{"rejectedSpans":"0","errorMessage":"PRIVATE_ERROR"}}`, "warning"},
		{"empty", "", "unknown"}, {"null", `null`, "unknown"}, {"array", `[]`, "unknown"}, {"trailing", `{} {}`, "unknown"},
		{"wrong count", `{"partialSuccess":{"rejectedSpans":"unknown"}}`, "unknown"},
		{"negative", `{"partialSuccess":{"rejectedSpans":-1}}`, "unknown"},
		{"excess", `{"partialSuccess":{"rejectedSpans":2}}`, "unknown"},
		{"rounding", `{"partialSuccess":{"rejectedSpans":1.00000000000000000001}}`, "unknown"},
		{"fraction syntax", `{"partialSuccess":{"rejectedSpans":"1/1"}}`, "unknown"},
		{"oversized exponent", `{"partialSuccess":{"rejectedSpans":"1e999999999"}}`, "unknown"},
		{"wrong warning", `{"partialSuccess":{"errorMessage":true}}`, "unknown"},
		{"oversized", `{"padding":"` + strings.Repeat("x", 64<<10) + `"}`, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			got, err := ExportTrace(t.Context(), config.AgentTelemetryExport{Endpoint: server.URL}, traceExportFixture(), allowTraceExport)
			if err != nil || got.State != tc.state || calls.Load() != 1 {
				t.Fatalf("receipt=%+v calls=%d err=%v", got, calls.Load(), err)
			}
			if tc.state == "unknown" && (got.Accepted != nil || got.Rejected != nil) {
				t.Fatal("unknown acceptance became zero")
			}
			data, _ := json.Marshal(got)
			if strings.Contains(string(data), "PRIVATE_") {
				t.Fatal("collector diagnostic escaped")
			}
		})
	}
}

func TestTraceExportRejectsRedirectsAndUnsupportedResponsesWithoutRetry(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	for _, mode := range []string{"redirect", "error", "mime", "encoding"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				switch mode {
				case "redirect":
					http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
					return
				case "error":
					w.WriteHeader(http.StatusServiceUnavailable)
				case "mime":
					w.Header().Set("Content-Type", "text/html")
				case "encoding":
					w.Header().Set("Content-Encoding", "gzip")
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			got, err := ExportTrace(t.Context(), config.AgentTelemetryExport{Endpoint: server.URL}, traceExportFixture(), allowTraceExport)
			if err != nil || got.State != "unknown" || calls.Load() != 1 || redirected.Load() != 0 {
				t.Fatal(got, err)
			}
		})
	}
}

func TestTraceExportAdmissionAndValidationPreventDispatch(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	destination := config.AgentTelemetryExport{Endpoint: server.URL}
	denied := errors.New("scope changed")
	if _, err := ExportTrace(t.Context(), destination, traceExportFixture(), func(context.Context) error { return denied }); !errors.Is(err, denied) {
		t.Fatal("admission bypassed", err)
	}
	for _, invalid := range []string{"kind", "key", "unfinished", "time", "trace", "count", "unfinished count", "duplicate"} {
		value := traceExportFixture()
		switch invalid {
		case "kind":
			value.Spans[0].Kind = "PRIVATE_SOURCE"
		case "key":
			value.Spans[0].OperationKey = "PRIVATE_MODEL"
		case "unfinished":
			value.Spans[0].Status = "interrupted"
		case "time":
			value.Spans[0].Ended = time.Time{}
		case "trace":
			value.TraceID = strings.Repeat("0", 32)
		case "count":
			value.Spans = make([]agentstate.TraceSpan, 257)
		case "unfinished count":
			value.Unfinished = 256
		case "duplicate":
			value.Spans = append(value.Spans, value.Spans[0])
		}
		if _, err := ExportTrace(t.Context(), destination, value, allowTraceExport); err == nil {
			t.Fatal("invalid snapshot sent", invalid)
		}
	}
	t.Setenv("TRACE_TOKEN", "bad\nvalue")
	destination.BearerEnv = "TRACE_TOKEN"
	if _, err := ExportTrace(t.Context(), destination, traceExportFixture(), allowTraceExport); err == nil {
		t.Fatal("invalid credential accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("preflight dispatched work")
	}
}

func TestTraceExportCancellationRetainsUnknownDeliveryAndStopsRequest(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) { checkTraceExportCancellation(t, cause) })
	}
}

func checkTraceExportCancellation(t *testing.T, cause error) {
	t.Helper()
	started, stopped := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	finished := make(chan TraceExportReceipt, 1)
	go func() {
		receipt, err := ExportTrace(ctx, config.AgentTelemetryExport{Endpoint: server.URL}, traceExportFixture(), allowTraceExport)
		if err != nil {
			t.Error(err)
		}
		finished <- receipt
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("export did not start")
	}
	cancel(cause)
	reason := "canceled"
	if cause == context.DeadlineExceeded {
		reason = "deadline_exceeded"
	}
	select {
	case receipt := <-finished:
		if receipt.State != "unknown" || receipt.Reason != reason || receipt.Accepted != nil {
			t.Fatal(receipt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("export did not stop")
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("collector request not canceled")
	}
	if calls.Load() != 1 {
		t.Fatal("cancellation replayed request")
	}
}
