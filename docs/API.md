# Mini-Orca API Reference

> REST API for Mini-Orca v2.0

All API endpoints return JSON responses. Base URL: `http://localhost:8080/api`

---

## Table of Contents

- [Sessions](#sessions)
- [Phase Management](#phase-management)
- [Approvals](#approvals)
- [Configuration](#configuration)
- [Skills](#skills)
- [Code Operations](#code-operations)
- [Error Responses](#error-responses)

---

## Sessions

### Get Current Session

```
GET /api/session
```

Returns the current session state.

**Response:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "My Project",
  "phase": "coding",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T11:45:00Z",
  "plan": {
    "id": "plan-1",
    "status": "in_progress",
    "atomic_units": 5,
    "completed_units": 2,
    "pending_units": 3
  }
}
```

### List Sessions

```
GET /api/sessions
```

Returns all sessions.

**Response:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "My Project",
    "phase": "coding",
    "created": "2024-01-15T10:30:00Z",
    "updated": "2024-01-15T11:45:00Z"
  }
]
```

### Create Session

```
POST /api/sessions
```

**Request Body:**
```json
{
  "name": "My Project",
  "description": "A sample project",
  "project_dir": "/path/to/project"
}
```

**Response:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "My Project",
  "phase": "planning"
}
```

---

## Phase Management

### Update Phase

```
POST /api/session/phase
```

Manually transition the session to a new phase.

**Request Body:**
```json
{
  "phase": "coding"
}
```

**Valid Phases:**
- `planning`
- `planning_review`
- `coding`
- `testing`
- `review`
- `human_review`
- `complete`
- `cancelled`

---

## Approvals

### Approve Phase

```
POST /api/approve
```

**Request Body:**
```json
{
  "phase": "planning_review",
  "message": "Plan looks good"
}
```

**Response:**
```json
{
  "status": "approved",
  "phase": "planning_review",
  "message": "Plan looks good",
  "approved_at": "2024-01-15T12:00:00Z"
}
```

### Reject Phase

```
POST /api/reject
```

**Request Body:**
```json
{
  "phase": "planning_review",
  "message": "Need more details on error handling"
}
```

### Get Pending Approvals

```
GET /api/pending-approvals
```

**Response:**
```json
[
  {
    "phase": "planning_review",
    "name": "Planning Review",
    "reason": "Review the generated plan before proceeding"
  }
]
```

---

## Configuration

### Get Configuration

```
GET /api/config
```

**Response:**
```json
{
  "models": {
    "active_provider": "lm-studio",
    "providers": {
      "lm-studio": {
        "base_url": "http://127.0.0.1:1234"
      }
    },
    "phases": {
      "planning": {
        "provider": "lm-studio",
        "model": "qwen/qwen3-coder-30b",
        "temperature": 0.3
      }
    }
  },
  "agents": {
    "planner": {
      "skills": ["solid_principles", "clean_code"]
    }
  },
  "server": {
    "port": 8080
  }
}
```

### Update Configuration

```
POST /api/config
```

Send the complete configuration object to update.

---

## Skills

### Get Skills

```
GET /api/skills
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "solid_principles",
    "type": "knowledge",
    "priority": 5,
    "description": "Apply SOLID principles",
    "enabled": true
  }
]
```

### Add Skill

```
POST /api/skills
```

**Request Body:**
```json
{
  "name": "custom_skill",
  "type": "knowledge",
  "description": "My custom skill",
  "priority": 3,
  "prompt": "Follow these guidelines..."
}
```

### Update Skills

```
PUT /api/skills
```

Send the complete skills array to update all skills.

### Delete Skill

```
DELETE /api/skills/:id
```

---

## Code Operations

### Insert Function

```
POST /api/insert
```

Insert a standalone function into the project.

**Request Body:**
```json
{
  "function_code": "func GetUser(id int) (*User, error) { ... }",
  "file_path": "user.go",
  "position": "append"
}
```

**Position Options:**
- `append` - Add to end of file
- `prepend` - Add to beginning of file
- `after` - Add after a marker comment (use `marker` field)

### Save Code

```
POST /api/save
```

Save code to a file.

**Request Body:**
```json
{
  "code": "// Your code here",
  "file_path": "main.go"
}
```

### Format Code

```
POST /api/format
```

Format code using the project's formatter.

**Request Body:**
```json
{
  "code": "// Code to format"
}
```

**Response:**
```json
{
  "formatted": "// Formatted code"
}
```

---

## Error Responses

All errors follow this format:

```json
{
  "error": true,
  "code": "model_request_failed",
  "message": "LLM call failed: timeout",
  "details": null
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `session_not_found` | 404 | Session does not exist |
| `agent_not_found` | 404 | Agent not registered |
| `file_not_found` | 404 | File not found |
| `api_not_found` | 404 | API endpoint not found |
| `model_request_failed` | 502 | LLM provider error |
| `model_rate_limited` | 429 | Provider rate limit |
| `tool_execution_failed` | 500 | Tool command failed |
| `service_unavailable` | 503 | Service temporarily down |
| `validation_failed` | 400 | Invalid input data |
| `invalid_transition` | 400 | Invalid phase transition |
| `approval_rejected` | 400 | Approval rejected |
| `state_corrupted` | 500 | State file corrupted |
| `permission_denied` | 403 | Permission denied |
| `model_error` | 500 | Model configuration error |

---

## WebSocket Events (Future)

Real-time updates via WebSocket:

```
WS /api/ws
```

**Messages:**
- `phase_change` - Session phase changed
- `agent_start` - Agent started execution
- `agent_complete` - Agent finished
- `event` - New activity log entry

---

*Generated for Mini-Orca v2.0*
