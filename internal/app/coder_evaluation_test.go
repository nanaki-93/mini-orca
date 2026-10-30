package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func coderEvaluationTask() CoderTask {
	return CoderTask{
		Files:    map[string]string{"go.mod": "module example.test/coder\n\ngo 1.22\n", "calc.go": toolCalcSource, "calc_test.go": toolCalcTest},
		Kind:     "feature",
		Title:    "Add Sub",
		Request:  "Add Sub(a, b) next to Add.",
		Paths:    []string{"calc.go"},
		Criteria: []string{"Sub returns a minus b."},
	}
}

func TestTheCoderEvaluationRunsEitherCoderOnAScratchProject(t *testing.T) {
	withSub := strings.Replace(toolCalcSource, "a + b }\n", "a + b }\n\n// Sub subtracts.\nfunc Sub(a, b int) int { return a - b }\n", 1)
	server := changeProvider(t, func() string {
		return `{"explanation":"Added Sub.","changes":[{"path":"calc.go","content":` + strconv.Quote(withSub) + `}]}`
	})
	oneShot, err := RunCoder(context.Background(), scopedTestConfig(server.URL), coderEvaluationTask(), false)
	if err != nil || oneShot.Tools || len(oneShot.Changes) != 1 || oneShot.Changes[0].Content != withSub || oneShot.Usage.ModelCalls != 1 || oneShot.Stage != nil {
		t.Fatalf("one-shot attempt = %+v, %v", oneShot, err)
	}
	if _, err := RunCoder(context.Background(), scopedTestConfig(server.URL), coderEvaluationTask(), true); err == nil || !strings.Contains(err.Error(), "use a pi model") {
		t.Fatalf("a tool run on a model without tools = %v", err)
	}

	dir := t.TempDir()
	steps, _ := json.Marshal([]fakePiStep{
		{Tool: "edit_file", Arguments: map[string]any{"path": "calc.go", "old_text": "a + b }\n", "new_text": "a + b }\n\n// Sub subtracts.\nfunc Sub(a, b int) int { return a - b }\n"}},
		{Tool: "create_file", Arguments: map[string]any{"path": "sub.go", "content": "package main\n\n// Neg negates.\nfunc Neg(a int) int { return -a }\n"}},
		{Tool: "finish", Arguments: map[string]any{"summary": "Added Sub and Neg."}},
		{Text: "Done."},
	})
	if err := os.WriteFile(filepath.Join(dir, "coder.json"), steps, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakePiEnv, "1")
	t.Setenv("MINI_ORCA_FAKE_PI_DIR", dir)
	cfg := scopedTestConfig(server.URL)
	cfg.ModelScopes.Bug = piScope("fake/coder")
	tools, err := RunCoder(context.Background(), cfg, coderEvaluationTask(), true)
	if err != nil || !tools.Tools || len(tools.Changes) != 2 || tools.Stage == nil || tools.Stage.Calls != 3 || tools.Stage.Turns != 4 || tools.Usage.ModelCalls != 4 || tools.Usage.InputTokens != 4000 {
		t.Fatalf("tool attempt = %+v (stage %+v), %v", tools, tools.Stage, err)
	}
}
