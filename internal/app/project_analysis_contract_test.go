package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const projectReportFixture = `{"purpose":"Runs a small daemon.","architecture":"flowchart TD\n A[\"Client\"] --> B[\"Daemon\"]","components":["main.go (Daemon): Handles requests."],"entry_points":["main.Run"],"flows":["sequenceDiagram\n Client->>Daemon: Request\n Daemon-->>Client: Response"],"risks":[{"severity":"low","summary":"Add request coverage.","engineering_insight":null}],"next_steps":["Add coverage"],"engineering_insight":null}`

func TestAnalyzeProjectRequestsSchemaAndRestoresDiagramReport(t *testing.T) {
	var received llm.ChatRequest
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// An unconstrained reply can contain Markdown even when the prompt asks
		// for JSON. Only the structured request establishes the output contract.
		output := "```json\n" + projectReportFixture + "\n```"
		if received.ResponseFormat != nil && received.ResponseFormat.Type == "json_schema" {
			output = projectReportFixture
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "returned-model", Choices: []llm.ChatChoice{{FinishReason: "stop", Message: llm.ChatMessage{Role: "assistant", Content: output}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)

	analysis, err := service.AnalyzeProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.AIStatus != project.ProjectAnalysisStatusFresh || analysis.Report.Model != "returned-model" || !strings.Contains(analysis.Report.Architecture, `A["Client"]`) || len(analysis.Report.Flows) != 1 {
		t.Fatalf("project report = %+v", analysis.Report)
	}
	format := received.ResponseFormat
	if format == nil || format.Type != "json_schema" || format.JSONSchema == nil || !format.JSONSchema.Strict || format.JSONSchema.Name == "" {
		t.Fatalf("project response format = %+v", format)
	}
	schema := compileReviewSchema(t, format.JSONSchema.Schema)
	assertReviewSchemaAccepts(t, schema, projectReportFixture, true)
	insight := `{"mechanism":"The handler delegates to the daemon.","why_it_matters_here":"Requests enter through one module.","tradeoff_or_failure_mode":"A blocked daemon delays the response.","transferable_lesson":"Block the handler in a test and verify cancellation."}`
	assertReviewSchemaAccepts(t, schema, strings.ReplaceAll(projectReportFixture, `"engineering_insight":null`, `"engineering_insight":`+insight), true)
	for _, invalid := range []string{
		strings.Replace(projectReportFixture, `"components":["main.go (Daemon): Handles requests."]`, `"components":[{"name":"Daemon"}]`, 1),
		strings.Replace(projectReportFixture, `"severity":"low"`, `"severity":"critical"`, 1),
		strings.Replace(projectReportFixture, `"Runs a small daemon."`, `""`, 1),
		strings.Replace(projectReportFixture, `"Runs a small daemon."`, `"`+strings.Repeat("x", 2049)+`"`, 1),
		strings.Replace(projectReportFixture, `"next_steps":["Add coverage"]`, `"next_steps":[`+strings.Repeat(`"Step",`, 32)+`"Step"]`, 1),
		strings.Replace(projectReportFixture, `"engineering_insight":null`, `"engineering_insight":{"mechanism":"partial"}`, 1),
		strings.Replace(projectReportFixture, `"purpose":`, `"unexpected":true,"purpose":`, 1),
		strings.Replace(projectReportFixture, `"next_steps":["Add coverage"],`, "", 1),
	} {
		assertReviewSchemaAccepts(t, schema, invalid, false)
	}
	restored, err := service.RestoreProject(root)
	if err != nil || restored.AIStatus != project.ProjectAnalysisStatusFresh || restored.Report.Architecture != analysis.Report.Architecture || len(restored.Report.Flows) != 1 || restored.Report.Flows[0] != analysis.Report.Flows[0] || calls != 1 {
		t.Fatalf("restored report = %+v, calls = %d, error = %v", restored, calls, err)
	}
}

func TestAnalyzeProjectKeepsInventoryAndExplainsUnusableReplies(t *testing.T) {
	for _, test := range []struct {
		name, output, finish, refusal, failure string
		status                                 int
	}{
		{name: "truncated JSON", output: `{"purpose":"partial`, finish: "length", failure: "output limit"},
		{name: "complete JSON at limit", output: projectReportFixture, finish: "length", failure: "output limit"},
		{name: "refusal", output: projectReportFixture, finish: "stop", refusal: "PRIVATE_PROVIDER_REASON", failure: "declined"},
		{name: "content filter", output: projectReportFixture, finish: "content_filter", failure: "declined"},
		{name: "tool call", output: projectReportFixture, finish: "tool_calls", failure: "did not finish"},
		{name: "malformed JSON", output: `{"PRIVATE_OUTPUT":true}`, finish: "stop", failure: "structured report"},
		{name: "rejected schema", status: http.StatusBadRequest, failure: "rejected the structured report request"},
		{name: "unprocessable schema", status: http.StatusUnprocessableEntity, failure: "rejected the structured report request"},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				if test.status != 0 {
					http.Error(w, "PRIVATE_PROVIDER_ERROR", test.status)
					return
				}
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{FinishReason: test.finish, Message: llm.ChatMessage{Role: "assistant", Content: test.output, Refusal: test.refusal}}}})
			}))
			defer server.Close()
			service, root := newSemanticAnalysisService(t, server.URL, 0)
			analysis, err := service.AnalyzeProject(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if analysis.AIStatus != project.ProjectAnalysisStatusFailed || analysis.FileCount != 1 || analysis.Report.Purpose != "" || !strings.Contains(analysis.Report.Failure, test.failure) || strings.Contains(analysis.Report.Failure, "PRIVATE_") || calls != 1 {
				t.Fatalf("failed analysis = %+v, calls = %d", analysis, calls)
			}
			restored, err := service.RestoreProject(root)
			if err != nil || restored.AIStatus != project.ProjectAnalysisStatusFailed || restored.Report.Failure != analysis.Report.Failure || calls != 1 {
				t.Fatalf("restored failure = %+v, calls = %d, error = %v", restored, calls, err)
			}
		})
	}
}
