package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckCopiesLinkInstalledDependencies(t *testing.T) {
	source := t.TempDir()
	for name, content := range map[string]string{
		"web/package.json":                 `{"name":"web"}`,
		"web/node_modules/vitest/index.js": "module.exports = 1;\n",
		"py/.venv/pyvenv.cfg":              "home = /usr/bin\n",
		"py/.venv/lib/site.py":             "x = 1\n",
		"py/venv/app.py":                   "print('not an environment')\n",
	} {
		full := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	destination := filepath.Join(t.TempDir(), "copy")
	if err := CopyWorkspace(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	for _, linked := range []string{"web/node_modules", "py/.venv"} {
		info, err := os.Lstat(filepath.Join(destination, linked))
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s was not linked: %v %v", linked, info, err)
		}
	}
	if info, err := os.Lstat(filepath.Join(destination, "py/venv/app.py")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("a source folder named venv was not copied: %v", err)
	}
	copied, err := FingerprintWorkspace(context.Background(), destination)
	if err != nil {
		t.Fatal(err)
	}
	original, err := WorkspaceFingerprint(context.Background(), source)
	if err != nil || original != copied {
		t.Fatalf("fingerprints differ: %s vs %s (%v)", original, copied, err)
	}
}
