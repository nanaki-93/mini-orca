package symbols

import "testing"

func TestGenericExtractorsRemainApproximate(t *testing.T) {
	tests := []struct{ language, source, name string }{
		{"Kotlin", "import kotlin.io.println\nclass Screen\nfun render() {}", "Screen"},
		{"Java", "import java.util.List;\npublic class App {}", "App"},
		{"TypeScript", "import { x } from 'x';\nexport function run() {}", "run"},
		{"JavaScript", "import { x } from 'x';\nexport async function run() {}", "run"},
		{"Python", "from pathlib import Path\ndef run(): pass", "run"},
		{"Rust", "use std::fmt;\npub fn run() {}", "run"},
		{"Ruby", "require 'json'\ndef run\nend", "run"},
		{"Swift", "import Foundation\npublic func run() {}", "run"},
		{"C++", "#include <vector>\nint run() {\n  return 0;\n}", "run"},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			imports, symbols := ExtractGenericFacts(test.language, []byte(test.source))
			if len(imports) != 1 || len(symbols) == 0 || symbols[0].Confidence != "approximate" {
				t.Fatalf("facts = %#v %#v", imports, symbols)
			}
			found := false
			for _, symbol := range symbols {
				if symbol.Name == test.name {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s absent from %#v", test.name, symbols)
			}
		})
	}
}

func TestGenericJavaExtractorRejectsExpressions(t *testing.T) {
	_, symbols := ExtractGenericFacts("Java", []byte("class App {\n  void run() {\n    new Worker().run();\n    return;\n  }\n}\n"))
	if len(symbols) != 2 || symbols[0].Name != "App" || symbols[1].Name != "run" {
		t.Fatalf("Java symbols = %#v, want only declarations", symbols)
	}
}

func TestGenericSymbolEndLineSupportsIndentationAndMultilineHeaders(t *testing.T) {
	_, pySymbols := ExtractGenericFacts("Python", []byte("def compute(\n    value: int,\n) -> int:\n    result = value + 1\n    return result\n\nx = 1\n"))
	if len(pySymbols) != 1 || pySymbols[0].StartLine != 1 || pySymbols[0].EndLine != 5 {
		t.Fatalf("Python symbols = %#v, want span 1..5", pySymbols)
	}
	_, tsSymbols := ExtractGenericFacts("TypeScript", []byte("export function compute(\n  value: number,\n): number\n{\n  return value + 1;\n}\n"))
	if len(tsSymbols) != 1 || tsSymbols[0].StartLine != 1 || tsSymbols[0].EndLine != 6 {
		t.Fatalf("TypeScript symbols = %#v, want span 1..6", tsSymbols)
	}
}
