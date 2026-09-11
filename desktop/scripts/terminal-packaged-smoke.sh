#!/usr/bin/env sh
# macOS arm64 proof using the packaged runtime and jars plus the synthetic test driver.
set -eu
terminal_script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
terminal_desktop_dir=$(CDPATH= cd "$terminal_script_dir/.." && pwd)
terminal_app=${1:-"$terminal_desktop_dir/build/compose/binaries/main/app/Mini-Orca.app"}
terminal_sdk=${MINI_ORCA_JBR25_HOME:-${MINI_ORCA_JAVA25_HOME:-}}
if [ ! -f "$terminal_sdk/include/jni.h" ]; then
  printf '%s\n' 'Set MINI_ORCA_JBR25_HOME to the verified Java 25 SDK for JNI headers.' >&2
  exit 2
fi
terminal_jvm="$terminal_app/Contents/runtime/Contents/Home/lib/server/libjvm.dylib"
if [ ! -f "$terminal_jvm" ] || [ ! -f "$terminal_desktop_dir/build/classes/kotlin/test/io/miniorca/desktop/DesktopTerminalSessionTestKt.class" ]; then
  printf '%s\n' 'Build desktop tests and createDistributable before running the packaged PTY probe.' >&2
  exit 2
fi
terminal_classpath=$(python3 - "$terminal_app" "$terminal_desktop_dir" <<'PY'
from pathlib import Path
import os, sys
app, desktop = map(Path, sys.argv[1:])
jars = sorted((app / 'Contents/app').glob('*.jar'))
if not jars:
    raise SystemExit('Packaged application jars are missing')
print(os.pathsep.join(str(path.resolve()) for path in [desktop / 'build/classes/kotlin/test', *jars]))
PY
)
terminal_probe_dir=$(mktemp -d "${TMPDIR:-/tmp}/mini-orca-packaged-pty.XXXXXX")
trap 'rm -f "$terminal_probe_dir/probe"; rmdir "$terminal_probe_dir"' EXIT HUP INT TERM
xcrun clang -Wall -Wextra -Werror \
  -I "$terminal_sdk/include" -I "$terminal_sdk/include/darwin" \
  "$terminal_script_dir/terminal-packaged-smoke.c" -o "$terminal_probe_dir/probe"
printf 'Using packaged runtime: %s\n' "$terminal_jvm"
"$terminal_probe_dir/probe" "$terminal_jvm" "$terminal_classpath"
