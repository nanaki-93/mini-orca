# Mini-Orca desktop API contract

**Version:** 4.1.0
**Base URL:** `http://localhost:9090`
**Client:** the local Compose Desktop application
**Content type:** `application/json`

The daemon is the local LLM boundary. It supports one active project and returns
preview-only output for one selected symbol in one selected file. It does not
offer a browser IDE, autonomous multi-file changes, automatic commits, or a
coder/tester/reviewer pipeline.

## Live routes

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness and daemon version. |
| GET | `/status` | Daemon state and `single_coder_preview` workflow identifier. |
| GET | `/api/system/info` | Local daemon system details. |
| GET | `/api/models/current` | Effective coder model/profile without credentials. |
| POST | `/api/projects/import` | Import and analyze a selected project. |
| GET | `/api/projects/current` | Current project analysis. |
| GET | `/api/projects/current/index` | Deterministic index of eligible project files and symbols. |
| GET | `/api/projects/current/files/info?path=…` | Safe selected-file information. |
| GET | `/api/projects/current/files/symbols?path=…` | Valid atomic targets for one eligible file. |
| GET | `/api/projects/current/impact?path=…&symbol=…` | Advisory source-free impact references; never expands model context. |
| GET | `/api/projects/current/git?path=…` | Read-only target-file Git availability, branch, and status. |
| GET | `/api/projects/current/files/analysis?path=…&project_revision=…` | Cached one-file semantic-analysis status. |
| POST | `/api/projects/current/files/analysis` | Analyze exactly one selected file. |
| DELETE | `/api/projects/current/files/analysis?path=…&project_revision=…` | Clear one selected-file analysis cache entry. |
| GET | `/api/projects/current/analysis-job?project_revision=…` | Source-free Analyze-all progress for the active project revision. |
| POST | `/api/projects/current/analysis-job` | Explicitly start bounded, sequential semantic analysis. |
| POST | `/api/projects/current/analysis-job/pause?project_revision=…` | Pause after the active one-file request completes. |
| POST | `/api/projects/current/analysis-job/resume` | Resume a persisted paused job. |
| POST | `/api/projects/current/analysis-job/cancel?project_revision=…` | Cancel the active request and pending work. |
| POST | `/api/projects/current/reindex` | Refresh deterministic facts without an LLM call. |
| POST | `/api/projects/current/candidates/checks` | Run focused checks in an isolated candidate workspace. |
| POST | `/api/projects/current/apply` | Explicitly apply one checked, validated candidate. |
| POST | `/api/projects/current/undo` | Restore the immediately preceding unchanged applied file. |
| GET | `/api/projects/current/audit?project_revision=…` | Source-free apply/undo audit history. |
| GET | `/api/projects/current/context?path=…&action=…` | Exact bounded context manifest without source text; `action=analyze_file` previews the semantic-analysis request. |
| POST | `/api/chat/message` | Generate one focused preview; never writes code. |
| GET | `/api/chat/history` | Current project-scoped, source-free activity history. |

`docs/openapi.yaml` is the machine-readable source for request and response
schemas. The route-contract test compares this table's API inventory with the
registered desktop API routes.

## Focused generation

```json
{
  "message": "Return a typed error for blank names",
  "file_path": "internal/user/service.go",
  "target_symbol": "UserService.Create",
  "project_id": "sha256:...",
  "project_revision": "sha256:...",
  "base_file_hash": "sha256:..."
}
```

The active generation action is `fix` with `strict_symbol` scope. The requested
file and symbol, active project id/revision, and selected-file base hash are
mandatory; stale project or file state returns `409 Conflict`. `scope_mode` is
optional and defaults to `strict_symbol`; `symbol_plus_imports` requests the
later validated minimal-import exception. The response contains a generation id,
captured base hash, effective profile, source-free context manifest, normalized
candidate hash, and unvalidated candidate content. It is preview-only and is
never persisted or applied by this endpoint.

The daemon asks models for one versioned JSON object with matching target path,
symbol, and scope. For compatible local models it also accepts exactly one
complete-file fenced code block. Multiple blocks, prose, unknown response
fields, mismatched target metadata, and missing candidate content are rejected.

When the configured model endpoint is not loopback/local, include
`"confirm_remote_provider": true` in an import or generation request after the
desktop user has reviewed the destination. The daemon rejects remote prompt
delivery without that explicit signal.

## Error shape

Every API failure uses this shape:

```json
{
  "type": "bad_request",
  "message": "technical error description",
  "user_message": "actionable message for the desktop user",
  "code": 400
}
```

Paths supplied to selected-file endpoints must be project-relative and pass the
daemon's canonical-path and symlink protections.

## Project index and symbols

`GET /api/projects/current/index` returns deterministic metadata only: eligible
project-relative paths, hashes, language facts, imports, diagnostics, and symbol
locations. It never returns source text. `GET /api/projects/current/files/symbols`
returns the selected file's `name`, `kind`, `signature`, line range, confidence,
and `atomic_target` flag.

`POST /api/projects/current/reindex` accepts an optional
`{"project_revision":"sha256:..."}` body. If supplied, it must match the active
revision or the daemon returns `409 Conflict`; a successful response is the new
deterministic index and revision. Missing projects return `404`, excluded files
return `403`, invalid paths return `400`, and binary files return `422`.

## Sequential Analyze-all

Analyze-all is an explicit cache-warming operation; importing or reindexing a
project never starts it. `POST /api/projects/current/analysis-job` requires the
active `project_revision` and accepts optional `max_files` (at most 500) and
`max_retries` (at most 3). It analyzes only eligible stale or missing text-file
summaries, strictly one model request at a time. Progress is persisted as
source-free metadata in `.mini-orca/sessions/analyze-all.json`.

Pause lets the current request finish and retains its valid cache entry. Cancel
propagates cancellation to the current model request and preserves previously
completed entries. Reindexing invalidates an active job; resume is allowed only
for a paused job at the same project revision.
