package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestWritingAgentsSeeTheOutlinesOfImportedAndNeighboringFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"web/src/app.ts":          "import { slug } from './lib/util';\nexport function render(): string { return slug('x'); }\n",
		"web/src/lib/util.ts":     "export function slug(text: string): string {\n  return text.toLowerCase();\n}\n",
		"web/src/routes.ts":       "export const routes = ['/'];\nexport function route(path: string): string { return path; }\n",
		"web/src/app.test.ts":     "import { render } from './app';\n",
		"pkg/service.py":          "from .repo import Repo\nimport pkg.config\n\ndef serve():\n    return Repo()\n",
		"pkg/repo.py":             "class Repo:\n    def find(self, key):\n        return None\n",
		"pkg/config.py":           "def load(path):\n    return {}\n",
		"go.mod":                  "module example.test/n\n\ngo 1.22\n",
		"cmd/main.go":             "package main\n\nimport \"example.test/n/internal/store\"\n\nfunc main() { store.Open() }\n",
		"internal/store/store.go": "package store\n\n// Open opens the store.\nfunc Open() error { return nil }\n",
	})
	session := &ChangeSession{Targets: []ChangeTarget{
		{Path: "web/src/app.ts", Exists: true, Content: "import { slug } from './lib/util';\nexport function render(): string { return slug('x'); }\n"},
		{Path: "web/src/app.test.ts", Exists: true, Content: "import { render } from './app';\n"},
		{Path: "pkg/service.py", Exists: true, Content: "from .repo import Repo\nimport pkg.config\n\ndef serve():\n    return Repo()\n"},
		{Path: "cmd/main.go", Exists: true, Content: "package main\n\nimport \"example.test/n/internal/store\"\n\nfunc main() { store.Open() }\n"},
	}}
	outlines := neighborOutlines(root, session)
	for _, want := range []string{
		"// web/src/lib/util.ts\n  function slug(text: string): string",
		"// web/src/routes.ts\n  function route(path: string): string",
		"// pkg/repo.py\n  class Repo:\n  def find(self, key):",
		"// pkg/config.py\n  def load(path):",
		"// internal/store/store.go\n  func Open() error",
	} {
		if !strings.Contains(outlines, want) {
			t.Errorf("outlines lack %q:\n%s", want, outlines)
		}
	}
	for _, unwanted := range []string{"// web/src/app.ts", "// web/src/app.test.ts", "// pkg/service.py"} {
		if strings.Contains(outlines, unwanted) {
			t.Errorf("outlines repeat a task file %q", unwanted)
		}
	}
	if !strings.HasPrefix(outlines, "\nOutlines of neighboring files") || strings.Index(outlines, "util.ts") > strings.Index(outlines, "routes.ts") {
		t.Fatalf("imports should come before siblings:\n%s", outlines)
	}
}
