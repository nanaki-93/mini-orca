#!/usr/bin/env sh
# Starts the disposable native Apply/Undo acceptance environment. Drive its window separately.
set -eu

native_script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
native_desktop_dir=$(CDPATH= cd "$native_script_dir/.." && pwd)
native_root_dir=$(CDPATH= cd "$native_desktop_dir/.." && pwd)
native_app=${1:-"$native_desktop_dir/build/compose/binaries/main/app/Mini-Orca.app"}
native_test_class="$native_desktop_dir/build/classes/kotlin/test/io/miniorca/desktop/DesktopNativeAcceptanceHarnessKt.class"

if [ ! -f "$native_test_class" ] || [ ! -d "$native_app" ]; then
  printf '%s\n' 'Build desktop test classes plus createDistributable before starting native acceptance.' >&2
  exit 2
fi

native_temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/mini-orca-native-acceptance.XXXXXX")
native_evidence_base=${MINI_ORCA_NATIVE_ACCEPTANCE_EVIDENCE:-"$native_desktop_dir/build/reports/ide-ux/final"}
mkdir -p "$native_evidence_base"
native_evidence_dir=$(mktemp -d "$native_evidence_base/native-acceptance.XXXXXX")
native_project_dir="$native_temp_dir/project"
native_original="$native_temp_dir/original-main.go"
native_observed="$native_temp_dir/applied-observed"
native_harness_app="$native_temp_dir/Mini-Orca-Native-Acceptance.app"
native_bundle_suffix=$(basename "$native_temp_dir" | tr -cd '[:alnum:]')
native_provider_pid=
native_daemon_pid=
native_daemon_binary=
native_fixture_pid=
native_monitor_pid=
native_cleanup_failed=0

native_stop_pid() {
  native_stop_pid_value=$1
  if [ -z "$native_stop_pid_value" ] || ! kill -0 "$native_stop_pid_value" 2>/dev/null; then
    return 0
  fi
  kill "$native_stop_pid_value" 2>/dev/null || true
  native_stop_attempt=0
  while kill -0 "$native_stop_pid_value" 2>/dev/null && [ "$native_stop_attempt" -lt 20 ]; do
    native_stop_attempt=$((native_stop_attempt + 1))
    sleep 0.1
  done
  if kill -0 "$native_stop_pid_value" 2>/dev/null; then
    kill -KILL "$native_stop_pid_value" 2>/dev/null || true
  fi
}

native_stop_temp_app_processes() {
  for native_pid in $(ps -axo pid=,command= | awk -v root="$native_temp_dir/Mini-Orca-Native-Acceptance.app" '$0 ~ root { print $1 }'); do
    if [ "$native_pid" != "$$" ]; then
      native_stop_pid "$native_pid"
    fi
  done
}

native_cleanup() {
  for native_log in provider.log daemon.log fixture.log; do
    if [ -f "$native_temp_dir/$native_log" ]; then
      cp "$native_temp_dir/$native_log" "$native_evidence_dir/$native_log"
    fi
  done
  native_stop_temp_app_processes
  for native_pid in "$native_monitor_pid" "$native_fixture_pid" "$native_daemon_pid" "$native_provider_pid"; do
    native_stop_pid "$native_pid"
  done
  native_stop_temp_app_processes
  if [ -n "${native_daemon_port:-}" ] && [ -n "${native_provider_port:-}" ]; then
    if ! python3 - "$native_daemon_port" "$native_provider_port" > "$native_evidence_dir/cleanup.txt" 2>&1 <<'PYCLEANUP'
import socket
import sys

active = False
for port in sys.argv[1:]:
    probe = socket.socket()
    probe.settimeout(0.2)
    try:
        probe.connect(("127.0.0.1", int(port)))
    except OSError:
        print(f"port {port}: closed")
    else:
        active = True
        print(f"port {port}: still accepting connections")
    finally:
        probe.close()
raise SystemExit(1 if active else 0)
PYCLEANUP
    then
      native_cleanup_failed=1
    fi
  fi
  rm -rf "$native_temp_dir"
}
native_exit() {
  native_status=$?
  native_cleanup
  if [ "$native_cleanup_failed" -ne 0 ]; then
    return 1
  fi
  return "$native_status"
}
trap native_exit EXIT
trap 'exit 1' HUP INT TERM

native_ports=$(python3 - <<'PY'
import socket
sockets = []
try:
    for _ in range(2):
        sock = socket.socket()
        sock.bind(("127.0.0.1", 0))
        sockets.append(sock)
    print(" ".join(str(sock.getsockname()[1]) for sock in sockets))
finally:
    for sock in sockets:
        sock.close()
PY
)
set -- $native_ports
native_daemon_port=$1
native_provider_port=$2

mkdir -p "$native_project_dir"
printf '%s\n' 'module example.com/nativeacceptance' 'go 1.22' > "$native_project_dir/go.mod"
printf '%s\n' 'package main' '' 'func Run() { println("original") }' > "$native_project_dir/main.go"
cp "$native_project_dir/main.go" "$native_original"
cp "$native_original" "$native_evidence_dir/original-main.go"
shasum -a 256 "$native_original" > "$native_evidence_dir/original-main.go.sha256"

printf '%s\n' \
  'model_scopes:' \
  '  analyze:' \
  "    api_base_url: \"http://127.0.0.1:$native_provider_port/v1\"" \
  '    api_key: ""' \
  '    model: "native-acceptance"' \
  '  bug:' \
  "    api_base_url: \"http://127.0.0.1:$native_provider_port/v1\"" \
  '    api_key: ""' \
  '    model: "native-acceptance"' \
  '  function:' \
  "    api_base_url: \"http://127.0.0.1:$native_provider_port/v1\"" \
  '    api_key: ""' \
  '    model: "native-acceptance"' \
  "project_path: \"$native_project_dir\"" \
  'logging:' \
  '  level: "error"' \
  '  format: "json"' \
  > "$native_temp_dir/config.yaml"

python3 -u - "$native_provider_port" <<'PY' > "$native_temp_dir/provider.log" 2>&1 &
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

proposal = '{"version":"v1","declaration":"func Run() { println(\\"applied\\") }","explanation":"Deterministic native acceptance proposal."}'

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        self.rfile.read(length)
        if self.path != "/v1/chat/completions":
            self.send_error(404)
            return
        body = json.dumps({"model": "native-acceptance", "choices": [{"message": {"role": "assistant", "content": proposal}}]}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, *_):
        pass

HTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
PY
native_provider_pid=$!

native_daemon_binary="$native_temp_dir/mini-orca-daemon"
(
  cd "$native_root_dir"
  go build -o "$native_daemon_binary" ./cmd/daemon
) > "$native_temp_dir/daemon.log" 2>&1
MINI_ORCA_CONFIG="$native_temp_dir/config.yaml" \
  MINI_ORCA_BIND_ADDRESS="127.0.0.1:$native_daemon_port" \
  "$native_daemon_binary" >> "$native_temp_dir/daemon.log" 2>&1 &
native_daemon_pid=$!

native_wait_for_health() {
  native_attempt=0
  while [ "$native_attempt" -lt 100 ]; do
    if python3 - "$native_daemon_port" <<'PY'
import sys
from urllib.request import urlopen
try:
    with urlopen(f"http://127.0.0.1:{sys.argv[1]}/health", timeout=0.2) as response:
        raise SystemExit(0 if response.status == 200 else 1)
except Exception:
    raise SystemExit(1)
PY
    then
      return 0
    fi
    native_attempt=$((native_attempt + 1))
    sleep 0.1
  done
  printf '%s\n' 'Disposable daemon did not become healthy.' >&2
  return 1
}
native_wait_for_health

cp -R "$native_app" "$native_harness_app"
(
  cd "$native_desktop_dir/build/classes/kotlin/test"
  jar --create --file "$native_harness_app/Contents/app/native-acceptance-harness.jar" \
    io/miniorca/desktop/DesktopNativeAcceptanceHarness*.class
)
python3 - "$native_harness_app" "$native_bundle_suffix" <<'PY'
from pathlib import Path
import plistlib
import sys

app = Path(sys.argv[1])
suffix = sys.argv[2]
config = app / "Contents/app/Mini-Orca.cfg"
contents = config.read_text()
contents = contents.replace(
    "app.mainclass=io.miniorca.desktop.MainKt",
    "app.mainclass=io.miniorca.desktop.DesktopNativeAcceptanceHarnessKt\napp.classpath=$APPDIR/native-acceptance-harness.jar",
    1,
)
config.write_text(contents)
plist_path = app / "Contents/Info.plist"
with plist_path.open("rb") as source:
    plist = plistlib.load(source)
name = f"Mini-Orca Native Acceptance {suffix}"
plist["CFBundleIdentifier"] = f"io.miniorca.desktop.nativeacceptance.{suffix}"
plist["CFBundleName"] = name
plist["CFBundleDisplayName"] = name
with plist_path.open("wb") as destination:
    plistlib.dump(plist, destination)
PY

MINI_ORCA_ACCEPTANCE_PROJECT="$native_project_dir" \
  MINI_ORCA_URL="http://127.0.0.1:$native_daemon_port" \
  "$native_harness_app/Contents/MacOS/Mini-Orca" \
  > "$native_temp_dir/fixture.log" 2>&1 &
native_fixture_pid=$!
printf '%s\n' \
  "root=$native_temp_dir" \
  "evidence=$native_evidence_dir" \
  "daemon_pid=$native_daemon_pid" \
  "provider_pid=$native_provider_pid" \
  "fixture_pid=$native_fixture_pid" \
  "daemon_port=$native_daemon_port" \
  "provider_port=$native_provider_port" \
  > "$native_evidence_dir/environment.txt"

(
  while kill -0 "$native_fixture_pid" 2>/dev/null; do
    if [ ! -f "$native_observed" ] &&
        grep -F 'println("applied")' "$native_project_dir/main.go" >/dev/null 2>&1; then
      : > "$native_observed"
      cp "$native_project_dir/main.go" "$native_evidence_dir/applied-main.go"
      shasum -a 256 "$native_project_dir/main.go" > "$native_evidence_dir/applied-main.go.sha256"
    fi
    sleep 0.05
  done
) &
native_monitor_pid=$!

printf '%s\n' "NATIVE_ACCEPTANCE_READY root=$native_temp_dir evidence=$native_evidence_dir daemon_pid=$native_daemon_pid provider_pid=$native_provider_pid fixture_pid=$native_fixture_pid daemon_port=$native_daemon_port provider_port=$native_provider_port"
wait "$native_fixture_pid"
native_fixture_pid=
cp "$native_project_dir/main.go" "$native_evidence_dir/restored-main.go"
shasum -a 256 "$native_project_dir/main.go" > "$native_evidence_dir/restored-main.go.sha256"

if [ ! -f "$native_observed" ]; then
  printf '%s\n' 'Native Apply was not observed in the disposable source file.' >&2
  exit 1
fi
if ! cmp -s "$native_original" "$native_project_dir/main.go"; then
  printf '%s\n' 'Native Undo did not restore the exact original source bytes.' >&2
  exit 1
fi
printf '%s\n' 'Native guarded Apply/Undo restored the disposable source bytes.'
