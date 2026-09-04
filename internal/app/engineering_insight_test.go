package app

import (
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestOptionalInsightsDoNotRejectSemanticOrDraftParents(t *testing.T) {
	target := project.IndexFile{Path: "main.go", Language: "Go", Symbols: []project.SymbolInfo{{Name: "Run", Signature: "func Run()", Confidence: "exact", AtomicTarget: true}}}
	output := `{"purpose":"Explains.","responsibilities":[],"dependencies":[],"side_effects":[],"engineering_insight":{"mechanism":"A cache entry must remain tied to the reviewed file.","why_it_matters_here":"A late model result otherwise looks current after the source changes."},"risks":[{"severity":"low","summary":"A conditional risk.","engineering_insight":{"mechanism":2}}],"suggestions":[{"title":"Keep identity","summary":"Retain the hash.","engineering_insight":{"mechanism":"Hashes bind advice to source.","why_it_matters_here":"Readers can see stale advice without treating it as current evidence."}}],"symbol_explanations":{}}`
	parsed, err := parseSemanticAnalysis(output, target, "package main\nfunc Run() {}")
	if err != nil || parsed.EngineeringInsight == nil || parsed.Risks[0].EngineeringInsight != nil || parsed.Suggestions[0].EngineeringInsight == nil {
		t.Fatalf("parsed semantic insight = %+v, %v", parsed, err)
	}
	draft, err := ParseDeclarationDraftResponse(`{"version":"v1","declaration":"func Run() {}","explanation":"Proposal.","engineering_insight":{"mechanism":false}}`)
	if err != nil || draft.EngineeringInsight != nil {
		t.Fatalf("draft with malformed optional insight = %+v, %v", draft, err)
	}
}

func TestManualDraftEditClearsCurrentInsight(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(DraftCreateRequest{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "func Run() {}", EngineeringInsight: &project.EngineeringInsight{Mechanism: "Original", WhyItMattersHere: "Original revision"}})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.UpdateDraft(DraftUpdateRequest{ID: draft.ID, ExpectedRevision: draft.Revision, Declaration: "func Run() { println(\"edited\") }"})
	if err != nil || updated.EngineeringInsight != nil {
		t.Fatalf("manual draft edit retained insight = %+v, %v", updated, err)
	}
}
