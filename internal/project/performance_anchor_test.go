package project

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func performanceRangeReply(t *testing.T, start, end int, symbol string) string {
	t.Helper()
	var reply map[string][]map[string]any
	if err := json.Unmarshal([]byte(validPerformanceFindingsJSON()), &reply); err != nil {
		t.Fatal(err)
	}
	item := reply["findings"][0]
	item["start_line"], item["end_line"] = start, end
	delete(item, "symbol")
	if symbol != "" {
		item["symbol"] = symbol
	}
	data, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPerformanceLineRangesDeriveOnlyUnambiguousEnclosingSymbols(t *testing.T) {
	commandSymbols := []SymbolInfo{{Name: "command", StartLine: 12, EndLine: 45}, {Name: "logResult", StartLine: 47, EndLine: 64}}
	methodSymbols := []SymbolInfo{{Name: "Logger.Info", StartLine: 39, EndLine: 44}, {Name: "Logger.Warn", StartLine: 46, EndLine: 51}, {Name: "Logger.Error", StartLine: 53, EndLine: 58}, {Name: "Logger.Debug", StartLine: 60, EndLine: 65}}
	for _, test := range []struct {
		name       string
		start, end int
		symbols    []SymbolInfo
		want       string
	}{
		{"command and helper span", 38, 57, commandSymbols, ""},
		{"multiple logging methods", 39, 65, methodSymbols, ""},
		{"within command", 38, 45, commandSymbols, "command"},
		{"whole method", 39, 44, methodSymbols, "Logger.Info"},
		{"blank between methods", 45, 45, methodSymbols, ""},
		{"without symbols", 39, 65, nil, ""},
		{"nested declaration", 42, 42, []SymbolInfo{{Name: "Outer", StartLine: 1, EndLine: 69}, {Name: "Inner", StartLine: 40, EndLine: 44}}, "Inner"},
		{"ambiguous declaration", 42, 42, []SymbolInfo{{Name: "One", StartLine: 40, EndLine: 44}, {Name: "Two", StartLine: 40, EndLine: 44}}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				symbols := slices.Clone(test.symbols)
				if reverse {
					slices.Reverse(symbols)
				}
				findings, warning, err := ParsePerformanceFindings(performanceRangeReply(t, test.start, test.end, ""), "fixture.go", strings.Repeat("// fixture\n", 69), symbols)
				if err != nil || warning != "" || len(findings) != 1 {
					t.Fatalf("findings=%+v warning=%q err=%v", findings, warning, err)
				}
				if findings[0].Symbol != test.want || findings[0].StartLine != test.start || findings[0].EndLine != test.end {
					t.Fatalf("anchor=%+v", findings[0])
				}
			}
		})
	}
}

func TestPerformanceRejectsInvalidLinesAndLegacyMismatchedSymbols(t *testing.T) {
	symbols := []SymbolInfo{{Name: "Logger.Info", StartLine: 39, EndLine: 44}, {Name: "Logger.Warn", StartLine: 46, EndLine: 51}}
	for _, test := range []struct {
		name, source string
		start, end   int
		symbol       string
	}{
		{"legacy mismatched label", strings.Repeat("// fixture\n", 65), 39, 65, "Logger.Info"},
		{"legacy unknown label", strings.Repeat("// fixture\n", 65), 39, 44, "Missing"},
		{"zero line", strings.Repeat("// fixture\n", 65), 0, 1, ""},
		{"reversed range", strings.Repeat("// fixture\n", 65), 44, 39, ""},
		{"past final newline", strings.Repeat("// fixture\n", 65), 65, 66, ""},
		{"past unterminated line", strings.Repeat("// fixture\n", 64) + "// last", 65, 66, ""},
		{"empty source", "", 1, 1, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			findings, _, err := ParsePerformanceFindings(performanceRangeReply(t, test.start, test.end, test.symbol), "fixture.go", test.source, symbols)
			if !errors.Is(err, PerformanceInvalidAnchors) || len(findings) != 0 {
				t.Fatalf("invalid anchor accepted: %+v %v", findings, err)
			}
		})
	}
}
