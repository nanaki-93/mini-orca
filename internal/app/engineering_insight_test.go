package app

import (
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestOptionalInsightsDoNotRejectSemanticOrDraftParents(t *testing.T) {
	target := project.IndexFile{Path: "main.go", Language: "Go", Symbols: []project.SymbolInfo{{Name: "Run", Signature: "func Run()", Confidence: "exact", AtomicTarget: true}}}
	output := `{"purpose":"Explains.","responsibilities":[],"dependencies":[],"side_effects":[],"engineering_insight":{"mechanism":"A cache entry must remain tied to the reviewed file.","why_it_matters_here":"A late model result otherwise looks current after the source changes.","tradeoff_or_failure_mode":"More identity inputs make stale results unavailable sooner.","transferable_lesson":"Change the file hash and verify the cache reports stale."},"risks":[{"severity":"low","summary":"A conditional risk.","engineering_insight":{"mechanism":2}}],"suggestions":[{"title":"Keep identity","summary":"Retain the hash.","engineering_insight":{"mechanism":"Hashes bind advice to source.","why_it_matters_here":"Readers can see stale advice without treating it as current evidence.","tradeoff_or_failure_mode":"Additional identity comparisons can invalidate cached advice.","transferable_lesson":"Change the hash and verify the cached analysis becomes stale."}}],"symbol_explanations":{}}`
	parsed, err := parseSemanticAnalysis(output, target, "package main\nfunc Run() {}")
	if err != nil || parsed.EngineeringInsight == nil || parsed.Risks[0].EngineeringInsight != nil || parsed.Suggestions[0].EngineeringInsight == nil {
		t.Fatalf("parsed semantic insight = %+v, %v", parsed, err)
	}
	draft, err := ParseDeclarationDraftResponse(`{"version":"v1","declaration":"func Run() {}","explanation":"Proposal.","engineering_insight":{"mechanism":false}}`)
	if err != nil || draft.EngineeringInsight != nil {
		t.Fatalf("draft with malformed optional insight = %+v, %v", draft, err)
	}
}

func TestFileAnalysisInsightUsesCompleteGroundedAdviceOrOmission(t *testing.T) {
	target := project.IndexFile{Path: "gate.go", Language: "Go", Symbols: []project.SymbolInfo{{Name: "Gate.Wait", Signature: "func (g *Gate) Wait(ctx context.Context)", Confidence: "exact", AtomicTarget: true}}}
	lockSource := "package cases\n\nimport (\n\t\"context\"\n\t\"sync\"\n)\n\ntype Gate struct { mu sync.Mutex }\n\nfunc (g *Gate) Wait(ctx context.Context) {\n\tg.mu.Lock()\n\tdefer g.mu.Unlock()\n\t<-ctx.Done()\n}\n"
	complete := `{"mechanism":"Wait holds g.mu while it blocks on context cancellation.","why_it_matters_here":"If another caller needs g.mu, the lock spans <-ctx.Done() and that caller waits.","tradeoff_or_failure_mode":"Releasing the lock before waiting needs an ownership check before later state is published.","transferable_lesson":"Cancel one waiter while another acquires g.mu and expect the second caller to proceed only after release."}`
	incomplete := `{"mechanism":"Wait has a mutex.","why_it_matters_here":"Use locks carefully."}`
	parent := func(insight string) string {
		return `{"purpose":"Waits for cancellation.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{},"engineering_insight":` + insight + `}`
	}

	positive, err := parseSemanticAnalysis(parent(complete), target, lockSource)
	if err != nil || positive.EngineeringInsight == nil || positive.EngineeringInsight.TransferableLesson == "" {
		t.Fatalf("complete grounded insight = %+v, %v", positive.EngineeringInsight, err)
	}
	partial, err := parseSemanticAnalysis(parent(incomplete), target, lockSource)
	if err != nil || partial.Purpose != "Waits for cancellation." || partial.EngineeringInsight != nil {
		t.Fatalf("incomplete optional insight did not omit cleanly = %+v, %v", partial, err)
	}
}

func TestFileAnalysisInsightPreservesQualifiedUnknownCalleeBehavior(t *testing.T) {
	target := project.IndexFile{Path: "verify.go", Language: "Go", Symbols: []project.SymbolInfo{{Name: "Verify", Signature: "func Verify(ctx context.Context, verifier Verifier, user string) error", Confidence: "exact", AtomicTarget: true}}}
	source := "package cases\n\nimport \"context\"\n\ntype Verifier interface { Check(context.Context, string) error }\n\nfunc Verify(ctx context.Context, verifier Verifier, user string) error {\n\treturn verifier.Check(ctx, user)\n}\n"
	output := `{"purpose":"Delegates verification.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{},"engineering_insight":{"mechanism":"Verify delegates to verifier.Check.","why_it_matters_here":"The selected file shows the call with ctx and user but not the verifier implementation, so authorization behavior is unknown here.","tradeoff_or_failure_mode":"Adding a local authorization check may duplicate or conflict with the callee's policy.","transferable_lesson":"Use a fake Verifier and verify the forwarded arguments; test authorization behavior where the implementation is available."}}`
	parsed, err := parseSemanticAnalysis(output, target, source)
	if err != nil || parsed.EngineeringInsight == nil {
		t.Fatalf("qualified unknown-callee insight = %+v, %v", parsed.EngineeringInsight, err)
	}
	if strings.Contains(strings.ToLower(parsed.EngineeringInsight.WhyItMattersHere), "missing authorization") {
		t.Fatalf("qualified insight asserted an unobserved missing safeguard: %+v", parsed.EngineeringInsight)
	}
}

func TestEngineeringInsightPromptsRequireUsefulGroundedContentOrOmission(t *testing.T) {
	draft, err := declarationDraftMessages("Improve Run.", "", "context", "main.go", "Run", project.DeclarationEditReplaceSymbol)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := semanticPrompt("package main\nfunc Run() {}", project.Analysis{Name: "fixture", Type: "go"}, &project.ProjectIndex{}, project.IndexFile{Path: "main.go"}, project.ContextManifest{})
	if err != nil {
		t.Fatal(err)
	}
	performance, err := performancePrompt("package main\nfunc Run() {}", project.Analysis{Name: "fixture", Type: "go"}, project.IndexFile{Path: "main.go"})
	if err != nil {
		t.Fatal(err)
	}
	for name, prompt := range map[string]string{
		"draft": draft[0].Content, "explanation": declarationExplanationPrompt("context"), "file": semantic, "performance": performance,
	} {
		for _, required := range []string{"mechanism explains the concrete mechanism", "why_it_matters_here names the exact local evidence", "tradeoff_or_failure_mode names a real trade-off or failure condition", "transferable_lesson gives a reusable lesson with a concrete verification idea", "Omit trivial or generic lessons."} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("%s prompt is missing %q: %s", name, required, prompt)
			}
		}
	}
	for _, requirement := range []string{
		"describe that behavior as unknown unless TARGET_SOURCE demonstrates it",
		"states impact conditionally, with an if, when, or workload condition",
		"Otherwise omit it, especially for a trivial wrapper",
		"known-length append loop may grow a result slice",
	} {
		if !strings.Contains(semantic, requirement) {
			t.Fatalf("file analysis prompt is missing grounded-insight guidance %q", requirement)
		}
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
