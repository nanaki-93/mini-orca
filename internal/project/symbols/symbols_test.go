package symbols

import "testing"

func TestGenericExtractorsExtractImports(t *testing.T) {
	tests := []struct {
		language, source, expectedImport string
	}{
		{"Kotlin", "import kotlin.io.println\nclass Screen\nfun render() {}", "import kotlin.io.println"},
		{"Java", "import java.util.List;\npublic class App {}", "import java.util.List;"},
		{"TypeScript", "import { x } from 'x';\nexport function run() {}", "import { x } from 'x';"},
		{"JavaScript", "import { x } from 'x';\nexport async function run() {}", "import { x } from 'x';"},
		{"Python", "from pathlib import Path\ndef run(): pass", "from pathlib import Path"},
		{"Rust", "use std::fmt;\npub fn run() {}", "use std::fmt;"},
		{"Ruby", "require 'json'\ndef run\nend", "require 'json'"},
		{"Swift", "import Foundation\npublic func run() {}", "import Foundation"},
		{"C++", "#include <vector>\nint run() {\n  return 0;\n}", "#include <vector>"},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			imports := ExtractGenericFacts(test.language, []byte(test.source))
			if len(imports) != 1 || imports[0] != test.expectedImport {
				t.Fatalf("imports = %#v, want [%q]", imports, test.expectedImport)
			}
		})
	}
}
