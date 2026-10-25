package project

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func performanceRangeReply(t *testing.T, start, end int) string {
	t.Helper()
	var reply map[string][]map[string]any
	if err := json.Unmarshal([]byte(validPerformanceFindingsJSON()), &reply); err != nil {
		t.Fatal(err)
	}
	item := reply["findings"][0]
	item["start_line"], item["end_line"] = start, end
	data, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPerformanceLineRangesPreserveSourceSpans(t *testing.T) {
	for _, test := range []struct {
		name       string
		start, end int
	}{
		{"command and helper span", 38, 57},
		{"multiple logging methods", 39, 65},
		{"within command", 38, 45},
		{"whole method", 39, 44},
		{"blank between methods", 45, 45},
		{"single line", 42, 42},
	} {
		t.Run(test.name, func(t *testing.T) {
			findings, warning, err := ParsePerformanceFindings(performanceRangeReply(t, test.start, test.end), "fixture.go", strings.Repeat("// fixture\n", 69))
			if err != nil || warning != "" || len(findings) != 1 {
				t.Fatalf("findings=%+v warning=%q err=%v", findings, warning, err)
			}
			if findings[0].StartLine != test.start || findings[0].EndLine != test.end {
				t.Fatalf("anchor=%+v", findings[0])
			}
		})
	}
}

func TestPerformanceRejectsInvalidLines(t *testing.T) {
	for _, test := range []struct {
		name, source string
		start, end   int
	}{
		{"zero line", strings.Repeat("// fixture\n", 65), 0, 1},
		{"reversed range", strings.Repeat("// fixture\n", 65), 44, 39},
		{"past final newline", strings.Repeat("// fixture\n", 65), 65, 66},
		{"past unterminated line", strings.Repeat("// fixture\n", 64) + "// last", 65, 66},
		{"empty source", "", 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			findings, _, err := ParsePerformanceFindings(performanceRangeReply(t, test.start, test.end), "fixture.go", test.source)
			if !errors.Is(err, PerformanceInvalidAnchors) || len(findings) != 0 {
				t.Fatalf("invalid anchor accepted: %+v %v", findings, err)
			}
		})
	}
}
