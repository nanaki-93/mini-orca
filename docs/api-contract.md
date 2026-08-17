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
| GET | `/api/projects/current/files/info?path=…` | Safe selected-file information. |
| GET | `/api/projects/current/context?path=…` | Exact bounded context manifest without source text. |
| POST | `/api/chat/message` | Generate one focused preview; never writes code. |
| GET | `/api/chat/history` | Current activity history (project-scoped persistence follows in a later revision). |

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
mandatory; stale project or file state returns `409 Conflict`. The response is a candidate preview only. The
later `symbol_plus_imports` mode is reserved for validated minimal import edits.

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
