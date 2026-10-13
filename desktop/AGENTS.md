# Desktop client

Read [../AGENTS.md](../AGENTS.md) and [UI_DESIGN_GUIDELINES.md](UI_DESIGN_GUIDELINES.md) before editing.
The Wails/React client is the supported desktop; [README.md](README.md) owns setup and usage.

- `internal/bridge` owns native HTTP transport, cancellation and PTY sessions.
  Keep daemon browser-origin protections unchanged. Never execute model text.
- `frontend/src` owns typed wire models, immutable workflow state and presentation.
  Guard late responses with project/file/draft identities, not cancellation alone.
  Keep source and diffs read-only, consent explicit and model content inert.
- The host is a separate Go module so GUI dependencies do not enter the daemon.
  Frontend assets and Wails bindings are generated; edit their sources only.
- Keep daemon interactions and immutable workflow transitions in the workspace
  owner. Components render state and emit actions; visual state stays local.
- Use typed wire models and existing API transport. Preserve structured failures,
  optional values, cancellation and project/file/proposal identity guards.
- Reuse shared controls and domain eligibility rules. Keep actions reachable,
  keyboard focus visible and errors actionable; move supporting detail into
  disclosures instead of repeating explanations around each action.
- Navigation and disclosures are passive. Source/diffs remain read-only; Apply
  and Undo retain their guards. Model-authored HTML, links and images stay inert.
- Requests and background work need accessible progress indicators. Distinguish
  unknown, unavailable, stale, partial, canceled and failed results from success.
- Terminal processes belong to native bridge sessions. Hiding a terminal keeps
  it alive; closing/project switching cleans it up. Never persist transcripts or
  send them to models. Recheck source freshness on return.
- Run `make web-test`; packaging/host changes also require `make web-build` on
  macOS arm64. Browser tests exercise production components at relevant viewport
  and text sizes. Do not claim OS focus or screen-reader evidence from them.
