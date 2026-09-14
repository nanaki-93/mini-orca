package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestPerformanceRetryCompletesRangesSpanningDeclarations(t *testing.T) {
	for _, test := range []struct {
		name        string
		start, end  int
		wrongSymbol string
		symbols     []project.SymbolInfo
	}{
		{"command and helper", 38, 57, "logResult", []project.SymbolInfo{{Name: "command", StartLine: 12, EndLine: 45}, {Name: "logResult", StartLine: 47, EndLine: 64}}},
		{"logging methods", 39, 65, "Info", []project.SymbolInfo{{Name: "Info", StartLine: 39, EndLine: 44}, {Name: "Warn", StartLine: 46, EndLine: 51}, {Name: "Error", StartLine: 53, EndLine: 58}, {Name: "Debug", StartLine: 60, EndLine: 65}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			lines := make([]string, 69)
			lines[0] = "package main"
			for _, symbol := range test.symbols {
				lines[symbol.StartLine-1] = "func " + symbol.Name + "() {"
				lines[symbol.EndLine-1] = "}"
			}
			currentReply := strings.Replace(validPerformanceLineReview, `"start_line":5,"end_line":5`, fmt.Sprintf(`"start_line":%d,"end_line":%d`, test.start, test.end), 1)
			legacyReply := strings.Replace(currentReply, `"title":`, `"symbol":"`+test.wrongSymbol+`","title":`, 1)
			var current atomic.Bool
			server, calls := analysisResponseServer(t, func(stage AnalysisStage) string {
				if stage != AnalysisStagePerformance {
					return emptyAnalysisReply(stage)
				}
				if current.Load() {
					return currentReply
				}
				return legacyReply
			})
			s, root := newSemanticAnalysisService(t, server.URL, 0)
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Reindex(); err != nil {
				t.Fatal(err)
			}
			preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 1}, nil)
			if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
				t.Fatal(err)
			}
			failed := completedAnalysisRun(t, s)
			if failed.Files[0].Stages[1].Status != AnalysisStageFailed || failed.Files[0].Stages[1].Reason != project.PerformanceInvalidAnchors.Error() {
				t.Fatalf("old response did not reproduce failure: %+v", failed)
			}
			snapshot, err := s.preparePerformanceReview("main.go")
			if err != nil {
				t.Fatal(err)
			}
			schema := compileReviewSchema(t, performanceReviewResponseSchema(snapshot).Schema)
			assertReviewSchemaAccepts(t, schema, legacyReply, false)
			assertReviewSchemaAccepts(t, schema, currentReply, true)
			current.Store(true)
			retry := retryPreviewFor(t, s, nil)
			if retry.ExpectedModelRequests != 1 {
				t.Fatalf("retry=%+v", retry)
			}
			if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(retry)); err != nil {
				t.Fatal(err)
			}
			completed := completedAnalysisRun(t, s)
			if completed.Status != AnalysisRunCompleted || calls.Load() != 4 {
				t.Fatalf("completion=%+v calls=%d", completed, calls.Load())
			}
			_, _, policy, err := s.performanceInputs()
			if err != nil {
				t.Fatal(err)
			}
			report, err := project.LoadPerformanceFileReport(root, "main.go", snapshot.file.ContentHash, policy)
			if err != nil || report == nil || len(report.Findings) != 1 || report.Warning != "" {
				t.Fatalf("report=%+v err=%v", report, err)
			}
			finding := report.Findings[0]
			if finding.Symbol != "" || finding.StartLine != test.start || finding.EndLine != test.end {
				t.Fatalf("range was altered: %+v", finding)
			}
			s.analysisRun = &analysisRunController{}
			assertSelectionStages(t, readSelectionFor(t, s), "main.go", "fresh", "up to date")
			if next := retryPreviewFor(t, s, nil); len(next.Files) != 0 || calls.Load() != 4 {
				t.Fatalf("completed range still needs retry: %+v", next)
			}
		})
	}
}
