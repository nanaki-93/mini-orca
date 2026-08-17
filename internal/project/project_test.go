package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

type mockChatClient struct{}

func (mockChatClient) Chat(context.Context, []llm.ChatMessage) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{
		Role: "assistant", Content: "## Purpose\nA small test project.\n\n## Architecture\nOne Go package.",
	}}}}, nil
}

type recordingChatClient struct{ messages []llm.ChatMessage }

func (c *recordingChatClient) Chat(_ context.Context, messages []llm.ChatMessage) (*llm.ChatResponse, error) {
	c.messages = append([]llm.ChatMessage(nil), messages...)
	return mockChatClient{}.Chat(context.Background(), messages)
}

func TestResolveFileRejectsSiblingPrefixAndSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "app")
	sibling := filepath.Join(parent, "app2")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sibling, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(sibling, "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveFile(root, filepath.Join("..", "app2", "secret.txt")); err == nil {
		t.Fatal("expected sibling-prefix traversal to be rejected")
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveFile(root, "link.txt"); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
	if _, err := ResolvePathForWrite(root, filepath.Join("..", "app2", "new.go")); err == nil {
		t.Fatal("expected output traversal to be rejected")
	}
	canonicalRoot, _ := CanonicalRoot(root)
	if output, err := ResolvePathForWrite(root, "new.go"); err != nil || output != filepath.Join(canonicalRoot, "new.go") {
		t.Fatalf("expected safe new output path, got %q, %v", output, err)
	}
}

func TestAnalyzerWritesAnalysisAndContextIncludesInventory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/app\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "internal"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "service.go"), []byte("package internal\n\nfunc Work() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := NewAnalyzer(mockChatClient{}).Analyze(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Type != "go" || analysis.FileCount != 3 || analysis.SourceFileCount != 2 {
		t.Fatalf("unexpected analysis: %+v", analysis)
	}
	data, err := os.ReadFile(filepath.Join(root, analysisRelativePath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "A small test project") || !strings.Contains(string(data), "internal/service.go") {
		t.Fatalf("analysis file missing AI summary or inventory:\n%s", data)
	}

	contextText, err := NewContextBuilder().Build(root, "main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"main.go", "internal/service.go", "func main()", "func Work()"} {
		if !strings.Contains(contextText, expected) {
			t.Errorf("context missing %q", expected)
		}
	}
}

func TestGetFileInfoRejectsLargeAndBinaryFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), []byte{0, 1, 2}, 0644); err != nil {
		t.Fatal(err)
	}
	info, err := GetFileInfo(root, "binary.bin")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Binary || info.Content != "" {
		t.Fatalf("expected binary file without content, got %+v", info)
	}
	large := make([]byte, maxFileViewBytes+1)
	for i := range large {
		large[i] = 'a'
	}
	if err := os.WriteFile(filepath.Join(root, "large.txt"), large, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := GetFileInfo(root, "large.txt"); err == nil {
		t.Fatal("expected oversized file to be rejected")
	}
}

func TestContextPolicyExcludesSecretsIgnoredAndGeneratedFiles(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		".gitignore":        "ignored.go\n",
		".mini-orcaignore":  "private/**\n",
		"main.go":           "package main\nfunc main() {}\n",
		".env":              "DATABASE_PASSWORD=must-not-leak\n",
		"private/server.go": "package private\nconst Key = \"must-not-leak\"\n",
		"ignored.go":        "package ignored\nconst Token = \"must-not-leak\"\n",
		"config.yaml":       "api_key: must-not-leak\n",
		"tls.key":           "must-not-leak\n",
		"app.min.js":        "const secret = 'must-not-leak'\n",
		"package-lock.json": "must-not-leak\n",
	}
	for path, content := range files {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".env", "config.yaml", "tls.key", "app.min.js", "package-lock.json", "ignored.go", "private/server.go"} {
		if decision := policy.Decide(path); decision.Include || decision.Reason == "" {
			t.Errorf("policy decision for %s = %+v, want excluded with reason", path, decision)
		}
	}
	if decision := policy.Decide("main.go"); !decision.Include {
		t.Fatalf("main.go unexpectedly excluded: %+v", decision)
	}

	client := &recordingChatClient{}
	if _, err := NewAnalyzer(client).Analyze(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	var prompt strings.Builder
	for _, message := range client.messages {
		prompt.WriteString(message.Content)
	}
	if strings.Contains(prompt.String(), "must-not-leak") {
		t.Fatalf("excluded content reached model prompt: %s", prompt.String())
	}
	if !strings.Contains(prompt.String(), "func main()") {
		t.Fatalf("eligible source did not reach model prompt: %s", prompt.String())
	}
}

func TestContextPolicyExcludesSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package outside\nconst Secret = \"must-not-leak\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.go")); err != nil {
		t.Fatal(err)
	}
	policy, err := NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	if decision := policy.Decide("linked.go"); decision.Include || !strings.Contains(decision.Reason, "symlink") {
		t.Fatalf("symlink decision = %+v, want exclusion", decision)
	}
}

func TestContextPolicyProjectOverrides(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".mini-orca"), 0755); err != nil {
		t.Fatal(err)
	}
	config := `{"include":["ignored.go"],"exclude":["drafts/**"]}`
	if err := os.WriteFile(filepath.Join(root, ".mini-orca", "context-policy.json"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.go\n"), 0644); err != nil {
		t.Fatal(err)
	}
	policy, err := NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	if decision := policy.Decide("ignored.go"); !decision.Include || decision.Reason != "project context-policy include" {
		t.Fatalf("include override = %+v", decision)
	}
	if decision := policy.Decide("drafts/main.go"); decision.Include || decision.Reason != "project context-policy exclude" {
		t.Fatalf("exclude override = %+v", decision)
	}
}
