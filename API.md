# API Documentation

This document describes the API endpoints provided by the mini-orca daemon.

## General Endpoints

### Health Check
- **URL**: `/health`
- **Method**: `GET`
- **Description**: Returns the health status of the daemon.
- **Response**: `{"status":"ok","version":"<version>"}`

### Daemon Status
- **URL**: `/status`
- **Method**: `GET`
- **Description**: Returns the running status and registered agents.
- **Response**: `{"status":"running","version":"<version>","workflow":"single_coder_preview"}`

### System Info
- **URL**: `/api/system/info`
- **Method**: `GET`
- **Description**: Returns system information (OS, architecture, Go version, etc.).
- **Response**: JSON object with system details.

## Chat API

### Send Message
- **URL**: `/api/chat/message`
- **Method**: `POST`
- **Description**: Produces a focused preview for the `fix` action: one named function, method, type, interface, or class in one selected file. The default scope is `strict_symbol`; later candidate workflows may explicitly request `symbol_plus_imports` when required imports must change. The daemon supplies the active project's analysis, complete file inventory, build metadata, and bounded source context.
- **Body**: 
```json
{
  "message": "Precise behavior to implement",
  "file_path": "relative/path/to/selected-file.go",
  "target_symbol": "Type.Method",
  "project_id": "sha256:...",
  "project_revision": "sha256:...",
  "base_file_hash": "sha256:..."
}
```
- **Response**: JSON agent result containing a one-file generated preview. The endpoint never writes generated code automatically. Read-only `analyze_file` and `explain_symbol` actions do not produce candidates; `generate_test` must target a symbol in an already selected test file.

## Project API

### Import and analyze a project

- **URL**: `/api/projects/import`
- **Method**: `POST`
- **Body**: `{"project_path":"/absolute/path/to/project"}`
- **Description**: Activates the directory, scans its files, asks the configured LLM for an architectural analysis, and writes `.mini-orca/analysis.md`.

### Current project information

- **URL**: `/api/projects/current`
- **Method**: `GET`
- **Description**: Returns project type, build file, file/source/line totals, languages, file inventory, AI status, and summary.

### Selected file information

- **URL**: `/api/projects/current/files/info?path=relative/path`
- **Method**: `GET`
- **Description**: Returns safe, project-relative file metadata and text content. Binary files and files over 1 MiB are not displayed.

### Deterministic project index

- **URL**: `/api/projects/current/index`
- **Method**: `GET`
- **Description**: Returns deterministic metadata for context-policy-eligible project-relative files, including hashes, language, imports, diagnostics, and symbols. It never returns source text.

### Selected-file symbols

- **URL**: `/api/projects/current/files/symbols?path=relative/path`
- **Method**: `GET`
- **Description**: Returns valid atomic targets with name, kind, signature, line range, confidence, and atomic-target flag. Excluded files are rejected.

### Reindex project facts

- **URL**: `/api/projects/current/reindex`
- **Method**: `POST`
- **Body**: `{"project_revision":"sha256:..."}` (optional)
- **Description**: Rebuilds deterministic index facts without calling the model. A supplied revision must match the active project or the endpoint returns `409 Conflict`.

### Get Chat History
- **URL**: `/api/chat/history`
- **Method**: `GET`
- **Description**: Returns the message history for the current session.
- **Response**: JSON array of message objects.

### Effective model profile

- **URL**: `/api/models/current`
- **Method**: `GET`
- **Description**: Returns the effective profile used for generation, including the selected model, skills, temperature, token limit, timeout, and retry count. It never returns credentials or prompt bodies.

The daemon is a local desktop-client API. It does not document a browser IDE or automatic coder/tester/reviewer pipeline.
