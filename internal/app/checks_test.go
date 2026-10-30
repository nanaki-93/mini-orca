package app

import (
	"context"
	"strings"
	"testing"
)

func TestCheckCancellationAndEvidenceSanitization(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	check := service.runCheck(ctx, t.TempDir(), analysis.ProjectRevision, "tests", true, []string{"go", "test", "./..."}, false, true)
	if check.State != CheckCanceled {
		t.Fatalf("canceled task check = %+v", check)
	}
	evidence := sanitizeCheckOutput("/tmp/work/api_key=private\npassword: hidden\n"+strings.Repeat("x", 8*1024+1), false, "/tmp/work", "/project")
	if strings.Contains(evidence, "private") || strings.Contains(evidence, "hidden") || !strings.Contains(evidence, "[output truncated]") || !strings.Contains(evidence, "<workspace>") {
		t.Fatalf("sanitized evidence = %q", evidence)
	}
}
