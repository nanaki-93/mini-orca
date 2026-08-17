package project

import "testing"

func TestGenericExtractorsRemainApproximate(t *testing.T) {
	tests := []struct{ language, source, name string }{
		{"Kotlin", "import kotlin.io.println\nclass Screen\nfun render() {}", "Screen"},
		{"Java", "import java.util.List;\npublic class App {}", "App"},
		{"TypeScript", "import { x } from 'x';\nexport function run() {}", "run"},
		{"Python", "from pathlib import Path\ndef run(): pass", "run"},
		{"Rust", "use std::fmt;\npub fn run() {}", "run"},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			imports, symbols := extractGenericFacts(test.language, []byte(test.source))
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
