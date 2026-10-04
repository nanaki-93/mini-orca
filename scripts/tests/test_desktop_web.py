import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


class DesktopWebHelperTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        (self.root / "scripts").mkdir()
        (self.root / "desktop/webapp/frontend").mkdir(parents=True)
        self.script = self.root / "scripts/desktop-web.sh"
        shutil.copyfile(ROOT / "scripts/desktop-web.sh", self.script)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.log = self.root / "calls"
        for name in ("node", "npm", "go"):
            self.executable(name, f'''#!/bin/sh
printf '%s\\n' '{name}'" $*" >> "$WEB_TEST_LOG"
if [ '{name}' = npm ] && [ "$*" = test ] && [ "${{WEB_TEST_FAIL:-}}" = npm ]; then
  printf 'browser fixture failed\\n' >&2
  exit 7
fi
''')
        self.executable("gofmt", '#!/bin/sh\nprintf "%s" "${WEB_TEST_FORMAT:-}"\n')
        self.executable("uname", '#!/bin/sh\nif [ "$1" = -s ]; then printf "%s\\n" "${WEB_TEST_OS:-Darwin}"; else printf "arm64\\n"; fi\n')
        (self.bin / "dirname").symlink_to(shutil.which("dirname"))

    def executable(self, name, content):
        path = self.bin / name
        path.write_text(content)
        path.chmod(0o755)

    def run_helper(self, action, **variables):
        result = subprocess.run(
            ["/bin/sh", str(self.script), action],
            cwd=self.root,
            env={**os.environ, "PATH": str(self.bin), "WEB_TEST_LOG": str(self.log), **variables},
            capture_output=True, text=True, timeout=10,
        )
        calls = self.log.read_text().splitlines() if self.log.exists() else []
        return result, calls

    def test_test_path_checks_production_assets_browser_and_native_lifecycle(self):
        result, calls = self.run_helper("test")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, ["npm ci --no-fund", "npm run build", "npm test", "go test -race ./internal/... ./cmd/...", "go vet ./internal/... ./cmd/..."])

    def test_browser_failure_keeps_diagnostic_and_exit_status(self):
        result, calls = self.run_helper("test", WEB_TEST_FAIL="npm")
        self.assertEqual(result.returncode, 7)
        self.assertIn("browser fixture failed", result.stderr)
        self.assertFalse(any(call.startswith("go ") for call in calls))

    def test_format_failure_is_not_reported_as_success(self):
        result, _ = self.run_helper("test", WEB_TEST_FORMAT="internal/bridge/client.go")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("internal/bridge/client.go", result.stderr)

    def test_native_packaging_has_explicit_platform_and_pinned_cli(self):
        result, calls = self.run_helper("build")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls[-1], "go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build -s -skipbindings -platform darwin/arm64")

    def test_unsupported_platform_does_not_build_native_package(self):
        result, calls = self.run_helper("build", WEB_TEST_OS="Linux")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("macOS arm64", result.stderr)
        self.assertFalse(any(call.startswith("go ") for call in calls))

    def test_invalid_action_has_no_tool_side_effect(self):
        result, calls = self.run_helper("install-system-tools")
        self.assertEqual(result.returncode, 2)
        self.assertEqual(calls, [])


if __name__ == "__main__":
    unittest.main()
