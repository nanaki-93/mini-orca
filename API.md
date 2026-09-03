# Mini-Orca API guide

The local daemon API is for the Compose Desktop client. The authoritative route
inventory and behavior notes are in [docs/api-contract.md](docs/api-contract.md);
machine-readable request and response schemas are in
[docs/openapi.yaml](docs/openapi.yaml).

The current API is centered on file-scoped chat sessions and editable Go
declaration drafts. New clients open a session pinned to one project/file/symbol,
send messages that cannot retarget it, then read, revise, validate, check, and
explicitly apply the resulting draft. The retired `POST /api/chat/message`
route is documented only as a `410 Gone` migration response.

The four desktop workspaces consume distinct API data: Summary uses the project
overview; Analysis uses explicit one-file analysis and bounded Analyze-all;
Bugs reads provenance-labelled findings and explicit Go scan results; and
Editor uses one file-scoped chat session plus draft review routes. See the
[route contract](docs/api-contract.md) for the complete, once-only live route
inventory and request guards.

All source-mutating behavior is confirmation-gated, hash/revision-guarded,
one-file-only, and auditable. The daemon does not provide a browser IDE,
automatic scans, automatic writes, multi-file changes, commits, or pushes.

## Scoped model and repair contract

`GET /api/models/current` reports safe metadata for the fixed `analyze`, `bug`,
and `function` profiles; legacy top-level fields remain the effective function
profile. It never includes an API key. Import sends prompt content only to
`analyze`, file analysis and Analyze-all only to `bug`, and declaration messages
only to `function`. Confirmation is evaluated independently for each remote
scope. An optional configured `reasoning_effort` is reported as non-secret
metadata and forwarded only in the selected scope's Chat Completions request.

An AI finding may include a validated `task_spec` for one exact Go declaration.
Pass that spec only when opening a matching replace-symbol chat session. The
daemon revalidates it against the current project revision, file hash, indexed
symbol, and source before pinning it to the session and every resulting draft.
An optional `go_test_candidate` is never written to the project: after a user
validates a draft and starts checks, it runs under a daemon-selected temporary
`_test.go` filename in the copied check workspace and must fail on the base then
pass on the candidate.

`POST /api/projects/current/chat/sessions/{sessionID}/messages` accepts
`repair: true` only for the latest task-bound draft with current failed checks.
The Desktop action supplies bounded sanitized check output as the ordinary next
message in that same session and parent-draft chain. The daemon allows at most
three repair requests per session; checks never start a provider call by
themselves. Passing checks return to the normal human diff review and explicit
Apply flow.

The API binds to loopback by default. `/api/models/current` exposes safe model
metadata for `analyze`, `bug`, and `function`; its top-level fields remain the
effective `function` profile for compatibility. A request that sends prompt
content must include `confirm_remote_provider: true` only when its own scope is
non-loopback: import uses `analyze`, file analysis uses `bug`, and declaration
generation uses `function`. Local credentials belong only in ignored
`config.yaml`; they are never returned by the API.
