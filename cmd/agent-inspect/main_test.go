package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestAgentInspectorCommandIsPassiveAndReportsErrors(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"--help"}, 0}, {nil, 2}, {[]string{"--root", root}, 0},
		{[]string{"--root", root, "--private"}, 1}, {[]string{"--root", root, "--run", "missing"}, 1},
		{[]string{"--root", root, "--limit", "501"}, 1}, {[]string{"--root", root, "extra"}, 2},
	} {
		var output, diagnostics bytes.Buffer
		if code := run(t.Context(), tc.args, &output, &diagnostics); code != tc.code {
			t.Fatal(tc, code, diagnostics.String())
		}
		if output.Len() > 0 && !json.Valid(output.Bytes()) {
			t.Fatal("not JSON", output.String())
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("inspection created files", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var output, diagnostics bytes.Buffer
	if run(ctx, []string{"--root", root}, &output, &diagnostics) != 1 || !strings.Contains(diagnostics.String(), "canceled") {
		t.Fatal("cancellation lost", diagnostics.String())
	}
}
