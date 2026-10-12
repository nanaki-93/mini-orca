# Local interactive terminal

The Wails host owns PTY sessions; React renders them with xterm.js. See the
[desktop guide](webapp/README.md) for terminal use and native verification.

Opening or restoring a project starts no shell. **New terminal** explicitly
starts a local shell in the project. Hiding the drawer retains the session;
closing a tab or switching projects cleans up its process. Failed cleanup stays
visible and blocks moving the session to a different project.

Output and scrollback remain in memory. Mini-Orca neither saves transcripts nor
sends them to a provider. Model output never becomes shell input. Returning to
source rechecks its freshness; adding, deleting or renaming files requires
reindexing. The terminal does not grant project-execution trust to model checks.

`make web-test` covers the native bridge and PTY lifecycle with temporary projects
and synthetic shell output. `make web-build` verifies the macOS arm64 package;
unit/component tests do not establish native focus or screen-reader behavior.
