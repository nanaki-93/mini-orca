package app

import (
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestDeclarationDraftMessagesPinOneTargetAndConversation(t *testing.T) {
	messages, err := declarationDraftMessages("Improve Run.", "func Run() {}", "selected context", "main.go", "Run", project.DeclarationEditReplaceSymbol)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Role != "user" {
		t.Fatalf("messages = %+v", messages)
	}
	content := messages[0].Content
	for _, required := range []string{"Target file: main.go", "Target symbol: Run", "Edit mode: replace_symbol", "Earlier file-scoped conversation", "func Run() {}", "selected context", "Return exactly one complete Go function"} {
		if !strings.Contains(content, required) {
			t.Fatalf("request is missing %q: %s", required, content)
		}
	}
}

func TestDeclarationDraftMessagesRejectIncompleteIdentity(t *testing.T) {
	for _, test := range []struct {
		name   string
		prompt string
		path   string
		symbol string
		mode   project.DeclarationEditMode
	}{
		{name: "blank prompt", path: "main.go", symbol: "Run", mode: project.DeclarationEditReplaceSymbol},
		{name: "blank path", prompt: "change", symbol: "Run", mode: project.DeclarationEditReplaceSymbol},
		{name: "blank symbol", prompt: "change", path: "main.go", mode: project.DeclarationEditReplaceSymbol},
		{name: "unknown mode", prompt: "change", path: "main.go", symbol: "Run", mode: "unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := declarationDraftMessages(test.prompt, "", "context", test.path, test.symbol, test.mode); err == nil {
				t.Fatal("expected identity validation error")
			}
		})
	}
}
