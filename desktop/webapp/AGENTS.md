# Desktop web client

Read the root and desktop guides. This directory owns the Wails host and React
client; the Compose-specific controls and Gradle commands apply to the legacy
client only. Preserve the same daemon wire contracts and workflow boundaries.

- `internal/bridge` owns native HTTP transport, cancellation and PTY sessions.
  Keep daemon browser-origin protections unchanged. Never execute model text.
- `frontend/src` owns typed wire models, immutable workflow state and presentation.
  Guard late responses with project/file/draft identities, not cancellation alone.
  Keep source and diffs read-only, consent explicit and model content inert.
- The host is a separate Go module so GUI dependencies do not enter the daemon.
  Frontend assets and Wails bindings are generated; edit their sources only.
- Run `make web-test` and `make web-build` from the repository root. Use the
  browser component tests for layout and interaction; native packaging and PTY
  tests provide separate evidence. macOS arm64 is the supported native target.
