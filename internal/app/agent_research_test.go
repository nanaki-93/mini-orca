package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	htmlparser "golang.org/x/net/html"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

const researchFixture = `{"modelVersion":"gemini-2.5-flash","candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"Café fact."}]},"groundingMetadata":{"webSearchQueries":["fixture question"],"searchEntryPoint":{"renderedContent":"<a href=\"https://www.google.com/search?q=fixture\">Search</a>"},"groundingChunks":[{"web":{"uri":"https://source.example/report","title":"Source"}}],"groundingSupports":[{"segment":{"startIndex":0,"endIndex":5,"text":"Café"},"groundingChunkIndices":[0]}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}`

func researchService(t *testing.T, endpoint string) (*Service, ResearchRequest) {
	t.Helper()
	s, _ := newSemanticAnalysisService(t, endpoint, 0)
	profile := s.runtimes.function.profile
	profile.Provider, profile.APIKey, profile.Model = config.GeminiProvider, "explicit-key", "gemini-2.5-flash"
	s.runtimes.function = newModelRuntime(profile, time.Minute, 3)
	s.integrations.config.GoogleSearch = true
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	file, err := s.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	return s, ResearchRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Question: "Research this fixture.", Sources: []ResearchSource{{Path: "main.go", Hash: file.ContentHash}}, ConfirmSearch: true, ConfirmSourceSharing: true, ConfirmRemoteProvider: true}
}

func TestResearchReportPreservesAttributionAndPrivateDisclosure(t *testing.T) {
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); io.WriteString(w, researchFixture) }))
	defer remote.Close()
	s, request := researchService(t, remote.URL)
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	report, err := s.RunResearch(t.Context(), request)
	if err != nil || report.State != "grounded" {
		t.Fatalf("research: %+v %v", report, err)
	}
	metadata, _ := json.Marshal(report)
	if strings.Contains(string(metadata), "Café") {
		t.Fatal("answer escaped the private report")
	}
	view, err := s.ResearchOverview(t.Context(), query)
	if err != nil || len(view.Reports) != 1 || view.State != "available" {
		t.Fatalf("overview: %+v %v", view, err)
	}
	page, err := s.ResearchReportHTML(t.Context(), query, report.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"Café<sup>", "source.example/report", "Provider-internal search usage is unknown", "5 input / 3 output", "sandbox=\"allow-popups allow-popups-to-escape-sandbox\""} {
		if !strings.Contains(string(page), required) {
			t.Fatalf("report omitted %q: %s", required, page)
		}
	}
	tokens := htmlparser.NewTokenizer(strings.NewReader(string(page)))
	var fragment string
	for tokens.Next() != htmlparser.ErrorToken {
		token := tokens.Token()
		if token.Data == "iframe" {
			for _, attr := range token.Attr {
				if attr.Key == "srcdoc" {
					fragment = attr.Val
				}
			}
		}
	}
	if fragment != `<a href="https://www.google.com/search?q=fixture">Search</a>` {
		t.Fatalf("original suggestions changed: %q", fragment)
	}
	history, err := s.ListAgentRuns(t.Context(), query)
	if err != nil || len(history.Runs) == 0 {
		t.Fatalf("missing native run: %+v %v", history, err)
	}
	detail, err := s.AgentRunDetail(t.Context(), query, history.Runs[0].ID, true)
	if err != nil || detail.Text == nil || detail.Text.State != "research_report_required" || detail.Text.Text != "" {
		t.Fatalf("plain answer disclosure: %+v %v", detail, err)
	}
	if err := os.WriteFile(filepath.Join(s.manager.Root(), "main.go"), []byte("package main\n// changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	page, err = s.ResearchReportHTML(t.Context(), query, report.ID)
	if err != nil || !strings.Contains(string(page), "earlier project snapshot") || calls.Load() != 1 {
		t.Fatalf("passive source freshness: %s %v calls=%d", page, err, calls.Load())
	}
}

func TestResearchAdmissionRejectsWithoutProviderWork(t *testing.T) {
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); io.WriteString(w, researchFixture) }))
	defer remote.Close()
	for _, kind := range []string{"disabled", "profile", "search", "source sharing", "stale source", "revision", "question", "budget", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			s, request := researchService(t, remote.URL)
			ctx := t.Context()
			switch kind {
			case "disabled":
				s.integrations.config.GoogleSearch = false
			case "profile":
				s.runtimes.function.profile.Model = "unsupported"
			case "search":
				request.ConfirmSearch = false
			case "source sharing":
				request.ConfirmSourceSharing = false
			case "stale source":
				request.Sources[0].Hash = strings.Repeat("a", 64)
			case "revision":
				request.ProjectRevision = "other"
			case "question":
				request.Question = " "
			case "budget":
				s.runtimes.function.effective.ContextMaxTokens = 1
			case "canceled":
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			if result, err := s.RunResearch(ctx, request); err == nil || result != nil {
				t.Fatalf("invalid request accepted: %+v %v", result, err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("rejected requests dispatched %d calls", calls.Load())
	}
}

func TestResearchDoesNotPublishLateSourcesOrUnsafeSuggestions(t *testing.T) {
	for _, kind := range []string{"source changed", "active HTML", "remote image", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			var s *Service
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := researchFixture
				switch kind {
				case "source changed":
					if err := os.WriteFile(filepath.Join(s.manager.Root(), "main.go"), []byte("package main\n// changed\n"), 0600); err != nil {
						t.Error(err)
					}
				case "active HTML":
					body = strings.Replace(body, "Search</a>", "Search</a><script>alert(1)</script>", 1)
				case "remote image":
					body = strings.Replace(body, "Search</a>", "Search</a><img src='https://remote.example/image'>", 1)
				case "canceled":
					cancel()
				}
				io.WriteString(w, body)
			}))
			defer remote.Close()
			var request ResearchRequest
			s, request = researchService(t, remote.URL)
			if report, err := s.RunResearch(ctx, request); err == nil || report != nil {
				t.Fatalf("unsafe publication: %+v %v", report, err)
			}
			view, err := s.ResearchOverview(t.Context(), AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision})
			if err != nil || len(view.Reports) != 0 {
				t.Fatalf("failed report published: %+v %v", view, err)
			}
		})
	}
}

func TestResearchReportEscapesContentAndRejectsMalformedAttribution(t *testing.T) {
	document := researchDocument{Question: "<script>question</script>", Answer: "<script>answer</script>", Grounding: &agentruntime.GroundingInfo{State: "ungrounded"}}
	page, err := renderResearchReport(document)
	if err != nil || strings.Contains(string(page), "<script>") || !strings.Contains(string(page), "&lt;script&gt;") {
		t.Fatalf("unescaped report: %s %v", page, err)
	}
	for _, fragment := range []string{`<a onclick="go()">x</a>`, `<meta http-equiv="refresh" content="0;url=https://example.com">`, `<a href="javascript:alert(1)">x</a>`, `<iframe srcdoc="x"></iframe>`} {
		document.Grounding.SuggestionsHTML = fragment
		if _, err := renderResearchReport(document); err == nil {
			t.Fatalf("active fragment accepted: %s", fragment)
		}
	}
	document.Grounding.SuggestionsHTML = ""
	document.Grounding.Supports = []agentruntime.GroundSupport{{Start: 0, End: 1000, Sources: []int{0}}}
	if _, err := renderResearchReport(document); err == nil {
		t.Fatal("invalid stored attribution accepted")
	}
}
