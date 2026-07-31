# Mini-Orca API Contract

**Version**: 2.0.0  
**Base URL**: `http://localhost:8080`  
**Authentication**: None (v2 is unauthenticated)  
**Content-Type**: `application/json`

---

## Table of Contents

1. [Health & Status](#1-health--status)
2. [Session Management](#2-session-management)
3. [Human Gate](#3-human-gate)
4. [Project Management](#4-project-management)
5. [HTMX Rendering](#5-htmx-rendering)
6. [Error Handling](#6-error-handling)

---

## 1. Health & Status

### 1.1 Health Check

Returns the health status of the daemon.

- **Endpoint**: `GET /health`
- **Authentication**: None
- **Rate Limit**: None

**Response**

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | `{"status":"ok"}` |

---

### 1.2 Daemon Status

Returns the running status and registered agents.

- **Endpoint**: `GET /status`
- **Authentication**: None
- **Rate Limit**: None

**Response**

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | `{"status":"running","agents":["planner","coder","tester","reviewer"]}` |

---

## 2. Session Management

### 2.1 Create Session

Creates a new agent session for autonomous task execution.

- **Endpoint**: `POST /api/sessions`
- **Authentication**: None
- **Rate Limit**: None

**Request Body** (`SessionCreateRequest`)

```json
{
  "goal": "Implement user authentication module",
  "project_path": "/path/to/project",
  "project_type": "go"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `goal` | `string` | Yes | High-level goal of the session |
| `project_path` | `string` | Yes | Filesystem path to the project directory |
| `project_type` | `string` | No | Project type override (e.g., `go`, `python`, `typescript`). Auto-detected if omitted. |

**Response** (`SessionResponse`)

| Status | Content-Type | Body |
|--------|-------------|------|
| 201 Created | `application/json` | Session object |

```json
{
  "id": "session-1719000000000000000",
  "goal": "Implement user authentication module",
  "project_path": "/path/to/project",
  "project_type": "go",
  "current_phase": "planning",
  "status": "pending",
  "created_at": "2024-07-01T12:00:00Z",
  "updated_at": "2024-07-01T12:00:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique session identifier |
| `goal` | `string` | The session goal |
| `project_path` | `string` | Path to the project |
| `project_type` | `string` | Detected or specified project type |
| `current_phase` | `string` | Current phase: `planning`, `coding`, `testing`, `review` |
| `status` | `string` | Session status: `pending`, `running`, `paused`, `completed`, `cancelled` |
| `created_at` | `string` (ISO 8601) | Creation timestamp |
| `updated_at` | `string` (ISO 8601) | Last update timestamp |
| `error` | `string` | Optional error message (if session failed) |

---

### 2.2 List Sessions

Returns a list of all sessions with summaries.

- **Endpoint**: `GET /api/sessions`
- **Authentication**: None
- **Rate Limit**: None

**Response** (`SessionListResponse`)

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | Session list |

```json
{
  "sessions": [
    {
      "id": "session-1719000000000000000",
      "goal": "Implement user authentication module",
      "current_phase": "coding",
      "status": "running",
      "created_at": "2024-07-01T12:00:00Z"
    }
  ],
  "total": 1
}
```

| Field | Type | Description |
|-------|------|-------------|
| `sessions` | `SessionSummary[]` | List of session summaries |
| `total` | `integer` | Total number of sessions |

**SessionSummary**

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique session identifier |
| `goal` | `string` | The session goal |
| `current_phase` | `string` | Current phase |
| `status` | `string` | Current status |
| `created_at` | `string` (ISO 8601) | Creation timestamp |

---

### 2.3 Get Session Status

Returns the current status of a specific session.

- **Endpoint**: `GET /api/sessions/{id}`
- **Authentication**: None
- **Rate Limit**: None

**Path Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | Yes | Session ID |

**Response** (`SessionResponse`)

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | Session object |
| 400 Bad Request | `application/json` | Error object |
| 404 Not Found | `application/json` | Error object |

---

### 2.4 Start Session

Transitions a session from `pending` to `running` and begins the planning phase.

- **Endpoint**: `POST /api/sessions/{id}/start`
- **Authentication**: None
- **Rate Limit**: None

**Path Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | Yes | Session ID |

**Response**

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | `{"status":"started","session_id":"...","phase":"planning"}` |
| 400 Bad Request | `application/json` | Error object |
| 404 Not Found | `application/json` | Error object |
| 409 Conflict | `application/json` | Error object |

---

### 2.5 Pause Session

Pauses a running session.

- **Endpoint**: `POST /api/sessions/{id}/pause`
- **Authentication**: None
- **Rate Limit**: None

**Path Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | Yes | Session ID |

**Response**

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | `{"status":"paused","session_id":"..."}` |
| 400 Bad Request | `application/json` | Error object |
| 404 Not Found | `application/json` | Error object |
| 409 Conflict | `application/json` | Error object |

---

### 2.6 Resume Session

Resumes a paused session.

- **Endpoint**: `POST /api/sessions/{id}/resume`
- **Authentication**: None
- **Rate Limit**: None

**Path Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | Yes | Session ID |

**Response**

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | `{"status":"resumed","session_id":"..."}` |
| 400 Bad Request | `application/json` | Error object |
| 404 Not Found | `application/json` | Error object |
| 409 Conflict | `application/json` | Error object |

---

### 2.7 Stop Session

Stops a session, transitioning it to `completed` state.

- **Endpoint**: `POST /api/sessions/{id}/stop`
- **Authentication**: None
- **Rate Limit**: None

**Path Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | Yes | Session ID |

**Response**

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | `{"status":"stopped","session_id":"..."}` |
| 400 Bad Request | `application/json` | Error object |
| 404 Not Found | `application/json` | Error object |
| 409 Conflict | `application/json` | Error object |

---

## 3. Human Gate

### 3.1 Get Gate Status

Returns the status of the human approval gate for a session.

- **Endpoint**: `GET /api/sessions/{id}/gate`
- **Authentication**: None
- **Rate Limit**: None

**Path Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | Yes | Session ID |

**Response** (`GateResponse`)

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | Gate object |
| 404 Not Found | `application/json` | Error object |

```json
{
  "session_id": "session-1719000000000000000",
  "phase": "coding",
  "output": "Generated authentication module code...",
  "approved": false,
  "feedback": "",
  "timestamp": "2024-07-01T12:05:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | `string` | Session identifier |
| `phase` | `string` | Phase requiring approval |
| `output` | `string` | Content for human review |
| `approved` | `boolean` | Whether the gate has been approved |
| `feedback` | `string` | Optional human feedback |
| `timestamp` | `string` (ISO 8601) | Gate timestamp |

---

### 3.2 Respond to Gate

Provides feedback or approval to a pending gate request.

- **Endpoint**: `POST /api/sessions/{id}/gate`
- **Authentication**: None
- **Rate Limit**: None

**Path Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | Yes | Session ID |

**Request Body** (`GateResponseRequest`)

```json
{
  "action": "approve",
  "feedback": "Looks good, please add error handling"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | `string` | Yes | Action: `approve`, `reject`, or `edit` |
| `feedback` | `string` | No | Optional feedback from the human reviewer |

**Response**

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | `{"status":"accepted","action":"approve","session_id":"..."}` |
| 400 Bad Request | `application/json` | Error object |
| 404 Not Found | `application/json` | Error object |

---

## 4. Project Management

### 4.1 List Projects

Returns a list of all managed projects.

- **Endpoint**: `GET /api/projects`
- **Authentication**: None
- **Rate Limit**: None

**Response** (`ProjectListResponse`)

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | Project list |

```json
{
  "projects": [
    {
      "id": "proj-1719000000000000000",
      "name": "my-project",
      "type": "go",
      "file_count": 42,
      "created_at": "2024-07-01T12:00:00Z"
    }
  ],
  "total": 1
}
```

| Field | Type | Description |
|-------|------|-------------|
| `projects` | `ProjectSummary[]` | List of project summaries |
| `total` | `integer` | Total number of projects |

**ProjectSummary**

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique project identifier |
| `name` | `string` | Project name |
| `type` | `string` | Project type (`go`, `python`, `typescript`, `rust`, `java`, `kotlin`, `unknown`) |
| `file_count` | `integer` | Number of files in the project |
| `created_at` | `string` (ISO 8601) | Creation timestamp |

---

### 4.2 Create Project

Creates or opens a project from a filesystem path.

- **Endpoint**: `POST /api/projects`
- **Authentication**: None
- **Rate Limit**: None

**Request Body** (`ProjectCreateRequest`)

```json
{
  "name": "my-project",
  "path": "/path/to/project",
  "type": "go"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | No | Project name (derived from directory if omitted) |
| `path` | `string` | Yes | Filesystem path to the project directory |
| `type` | `string` | No | Project type override (auto-detected if omitted) |

**Response** (`ProjectResponse`)

| Status | Content-Type | Body |
|--------|-------------|------|
| 201 Created | `application/json` | Project object |
| 400 Bad Request | `application/json` | Error object |

```json
{
  "id": "proj-1719000000000000000",
  "name": "my-project",
  "path": "/path/to/project",
  "type": "go",
  "file_count": 42,
  "created_at": "2024-07-01T12:00:00Z",
  "updated_at": "2024-07-01T12:00:00Z"
}
```

---

### 4.3 List Project Files

Lists files and directories within a project.

- **Endpoint**: `GET /api/projects/{id}/files`
- **Authentication**: None
- **Rate Limit**: None

**Path Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | `string` | Yes | Project ID |

**Query Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | `string` | No | Subdirectory path within the project |

**Response** (`FileListResponse`)

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | File list |
| 404 Not Found | `application/json` | Error object |

```json
{
  "files": [
    {
      "name": "cmd",
      "path": "cmd",
      "is_dir": true,
      "size": 0,
      "modified_at": "2024-07-01T12:00:00Z"
    },
    {
      "name": "main.go",
      "path": "main.go",
      "is_dir": false,
      "size": 1024,
      "modified_at": "2024-07-01T12:00:00Z"
    }
  ],
  "total": 2,
  "project_path": "proj-1719000000000000000"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `files` | `FileEntry[]` | List of file/directory entries |
| `total` | `integer` | Total number of entries |
| `project_path` | `string` | Project ID |

**FileEntry**

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | File or directory name |
| `path` | `string` | Relative path from project root |
| `is_dir` | `boolean` | Whether this is a directory |
| `size` | `integer` | File size in bytes (0 for directories) |
| `modified_at` | `string` (ISO 8601) | Last modification time |

---

### 4.4 Get File Content

Returns the content of a specific file in a project.

- **Endpoint**: `GET /api/projects/files/{path}`
- **Authentication**: None
- **Rate Limit**: None

**Path Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | `string` | Yes | File path within the project |

**Response** (`FileContentResponse`)

| Status | Content-Type | Body |
|--------|-------------|------|
| 200 OK | `application/json` | File content |
| 400 Bad Request | `application/json` | Error object |
| 404 Not Found | `application/json` | Error object |

```json
{
  "path": "cmd/daemon/main.go",
  "content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}",
  "size": 65,
  "content_type": "text/x-go"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `path` | `string` | Relative file path |
| `content` | `string` | File content |
| `size` | `integer` | File size in bytes |
| `content_type` | `string` | MIME type (e.g., `text/x-go`, `application/json`) |

---

## 5. HTMX Rendering

These endpoints return HTML fragments for the HTMX-based frontend interface.

### 5.1 Render Phase

Renders the UI for a specific phase.

- **Endpoint**: `GET /api/render/phase/{name}`
- **Authentication**: None
- **Response**: HTML fragment

### 5.2 Render File Tree

Renders the file tree component.

- **Endpoint**: `GET /api/render/file-tree`
- **Authentication**: None
- **Response**: HTML fragment

### 5.3 Render Dashboard

Renders the dashboard component.

- **Endpoint**: `GET /api/render/dashboard`
- **Authentication**: None
- **Response**: HTML fragment

### 5.4 Main Page

Renders the main IDE page.

- **Endpoint**: `GET /`
- **Authentication**: None
- **Response**: Full HTML document

---

## 6. Error Handling

All error responses follow a consistent format:

```json
{
  "error": {
    "type": "bad_request",
    "message": "Human-readable error message",
    "details": "Optional technical details"
  }
}
```

**Error Types**

| Type | HTTP Status | Description |
|------|-------------|-------------|
| `bad_request` | 400 | Invalid request parameters or body |
| `not_found` | 404 | Resource not found |
| `conflict` | 409 | Invalid state transition |
| `internal` | 500 | Internal server error |

---

## Session Lifecycle

```
pending → running → paused → running → completed
                ↓
            (gate) → approved → next_phase
                         ↓
                      rejected → fix_required
```

**Phases**: `planning` → `coding` → `testing` → `review`

---

## Notes

- **Authentication**: None required for v2.0
- **Rate Limits**: None configured
- **CORS**: Not configured (same-origin only)
- **WebSocket**: Not implemented (polling via REST)
