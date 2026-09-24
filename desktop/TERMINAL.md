# Local interactive terminal

The terminal runs in the desktop process. It adds no command-execution HTTP API
and does not use the daemon's copied-workspace check sandbox.

## Pinned dependencies

| Library | Version | Repository / choice |
| --- | --- | --- |
| JediTerm core and Swing UI | 3.72 | [JetBrains repository](https://packages.jetbrains.team/maven/p/ij/intellij-dependencies/org/jetbrains/jediterm/jediterm-ui/3.72/); compatible Kotlin 2.1 metadata |
| Pty4J | 0.13.12 | [Maven Central](https://repo.maven.apache.org/maven2/org/jetbrains/pty4j/pty4j/0.13.12/); native PTY implementation |
| JNA / JNA Platform | 5.17.0 | Resolved with the existing Jewel dependency; Pty4J requests 5.14.0 |
| SLF4J API | 2.0.13 | Resolved from Pty4J; no transcript logger is installed |

JediTerm 3.73–3.76 require Kotlin 2.4.0; 3.72 preserves Mini-Orca's existing Kotlin
2.3 toolchain. The additional repository is restricted to `org.jetbrains.jediterm`.
The [3.72 upstream README](https://github.com/JetBrains/jediterm/blob/97c49c88c4a41c2213072281f5c936ecb8753fb5/README.md#licenses)
permits choosing Apache-2.0, which Mini-Orca selects; its POM lists LGPL only.
Pty4J is EPL-1.0. [Bundled notices](src/main/resources/terminal-licenses/README.txt)
include those licenses and the notices for JNA/libffi, SLF4J and foreign-platform
binaries contained in the original Pty4J jar. Dependency jars are not modified.

## Session contract

- Construction is Idle and starts no process. `start()` explicitly launches one
  interactive login shell. Supported executables are absolute paths to zsh, bash,
  sh or fish, selected from `SHELL` (default `/bin/zsh` on the supported host).
- The project path must be an existing, accessible absolute local directory;
  symlinks resolve to its canonical location. It is passed as cwd, never inserted
  into shell source. Container-only/unavailable paths fail with a retryable error.
- Starting, Running, Exited, Failed and Closed are explicit observable states.
  Duplicate starts are rejected. A failed launch can be retried; a closed owner
  cannot be restarted. Do not create a replacement while cleanup is pending.
- The stable JediTerm connector handles UTF-8 streams and resize. A single
  emulator reader consumes output; the pane must keep it attached/reading when
  hidden. Pty4J's macOS output-preservation option can delay natural exit until
  pending output is consumed. Scrollback is capped at 5,000 lines and terminal
  dimensions at 1,000 columns/rows.
- Closing is nonblocking. Its future reports the cleanup attempt; callers use a
  finite UI/shutdown wait. A native start already in progress is cleaned up when
  it returns. `cleanupPending` remains explicit if a process survives teardown.
  Graceful and forced shell waits each have a 750ms deadline. Descendants are
  captured using the owned PID before stopping the shell. Streams are closed even
  after natural exit; the reader monitor is never held while closing raw streams.
- Input/output stay in memory and are never added to app metadata, model prompts,
  or daemon requests. The shell retains its own startup/history behavior. Model
  output, report selection and project restore never write to terminal stdin.

The pane handles key routing, project-switch confirmation and file-freshness
checks. Verify native focus separately from the PTY tests.

## Supported native package

Verified target: locally built macOS arm64, using the documented JDK 21 Gradle
launcher and JBR 25 SDK/runtime. Windows, Linux and macOS Intel are not enabled by
the current session boundary. The dependency's cross-platform binaries alone do
not establish support for those hosts.

Pty4J includes universal arm64/x86_64 `darwin/libpty.dylib` and
`darwin/pty4j-unix-spawn-helper` resources. It extracts its native files at runtime;
a writable system temporary directory is required. JNA similarly loads its native
dispatch library from its jar. No separately installed PTY library is needed.
The app enables native access for its classpath and explicitly includes
`java.desktop`, `java.management`, `java.net.http` and `jdk.unsupported` in the
runtime image. Signing/notarization and other operating systems remain unverified.

## Validate the terminal

Set `MINI_ORCA_JDK21_HOME` and `MINI_ORCA_JBR25_HOME` as described in the desktop
runtime documentation. Run from the repository root:

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.DesktopTerminalSessionTest' -PterminalNativeSmoke=true
./scripts/desktop-gradle.sh createDistributable
./desktop/scripts/terminal-packaged-smoke.sh
./scripts/desktop-gradle.sh test detekt spotlessCheck
```

The native probe creates only its own temporary project. It checks real TTY file
descriptors, canonical cwd, UTF-8, 121×42 resize, Ctrl+C against a foreground sleep,
background-child cleanup and bounded close. It uses `/bin/sh -i` without user
login scripts, disables probe-only history expansion and retains only bounded
synthetic output on failure. Normal unit runs leave this explicit native probe
opted out; record the `terminalNativeSmoke=true` invocation separately.

The packaged runtime has no `bin/java`. The small validation-only JNI launcher
loads that package's `lib/server/libjvm.dylib`, uses its unmodified application jars
and invokes the same compiled synthetic test driver. It requires Xcode Command
Line Tools and Python 3, does not change generated package files, and does not
substitute the development JVM for the bundled runtime. An optional first argument
to the shell script selects another built `Mini-Orca.app`.

## Terminal pane

The current app starts maximized and remains resizable. Terminal is currently
docked beneath the workspace; its placement may change in a redesign. Select
**Terminal** at the bottom to open the pane and immediately start a local
interactive shell in the open project. **Ctrl+Shift+T** opens or focuses the pane.
Shell tabs sit alongside Terminal in the same bar: **+** starts another independent
shell, selecting a tab focuses that shell, and its **×** closes that shell and its
children. Closing the last tab leaves **+** available; reopening the collapsed pane
starts a shell if no tabs remain. Exited and failed tabs retain their state until
closed; reopening the pane does not automatically restart them. Use **+** to retry
with a new shell. No replacement can start while cleanup is pending.

Select **Terminal** again to collapse the dock. Collapsing, changing workspaces,
resizing, or switching shell tabs keeps each session's reader and bounded in-memory
scrollback. Launch,
exit and cleanup states remain visible in their tabs, with errors in the selected
terminal. A restored layout restores pane dimensions with Terminal collapsed.
Old Problems, Checks and Output selections are discarded; no layout preference
launches a shell.

The terminal owns shell keys, including Ctrl+C. On macOS, Cmd+C copies the current
selection and Cmd+V pastes; shell history, ANSI/full-screen programs, cursor motion
and scrolling are provided by JediTerm. **Ctrl+Shift+F12** returns focus to the
application. Resizing the dock updates the real PTY dimensions; terminal text
follows the source palette and
application text scale.

Opening another project while any shell is active requires **Cancel switch** or
**Close shells and switch**. Confirming closes every tab, including hidden shells.
Closing the application also waits for all terminal sessions to clean up; a shell
that cannot stop keeps the window open with the failure visible. A running shell
is never silently moved to another project's directory.

Returning from the terminal, or later entering Editor/Review, reads the selected
file again. Changed content invalidates old draft/check/file-analysis evidence
and marks the captured project analysis stale until reindexing. A failed read also
blocks the old draft. Use **Re-index project** in the project menu after shell
commands add, remove or rename files. No terminal output is parsed, persisted by Mini-Orca or sent to a
model; shell programs may maintain their own files according to their settings.
