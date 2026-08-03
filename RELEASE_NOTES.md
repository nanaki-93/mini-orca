# Mini-Orca Release Notes

## v3.0.0 — Simplified Workflow (2024-XX-XX)

### Breaking Changes
- Removed planning phase — sessions now start directly at coding
- Removed plan-driven execution — single feature request per session
- Removed `Plan`, `PlanUnit`, `AtomicUnit` data types
- Session creation now requires `feature_request` and `project_path`
- Removed `planner` agent from the orchestrator

### New Features
- Simplified 4-phase workflow: coding → testing → review → human_gate
- Direct feature request input instead of multi-step planning
- Support for adding features to existing projects

### API Changes
- `POST /api/sessions` now uses `feature_request` instead of `goal`
- Sessions start at `PhaseCoding` instead of `PhasePlanning`
- Removed planning-related endpoints

---

# Mini-Orca v2.0 Release Notes

**Release Date**: July 31, 2024  
**Version**: 2.0.0  
**Previous Version**: 1.x

---

## Overview

Mini-Orca v2.0 is a complete redesign and rewrite of the platform, introducing a modern REST API, an HTMX-powered web IDE, comprehensive project management, and a robust multi-agent orchestration system. This release represents a significant architectural leap forward, transforming Mini-Orca from a simple task orchestrator into a full-featured autonomous development platform.

---

## 🎉 New Features

### 1. HTMX-Powered Web IDE

A modern, responsive web interface for monitoring and interacting with agents in real-time.

- **Dashboard**: Overview of all active sessions, projects, and system status
- **Phase Tracker**: Visual progress indicator showing current phase and transitions
- **Activity Log**: Real-time feed of agent actions and decisions
- **File Tree**: Interactive file browser with syntax highlighting
- **Phase Views**: Dedicated UI for each workflow phase (Planning, Coding, Testing, Review)
- **Responsive Design**: Works seamlessly on desktop and mobile devices

### 2. Human-in-the-Loop Approval Gates

Integrated human approval workflow at critical decision points.

- **Gate Creation**: Automatic gate generation after each phase completion
- **Gate Status**: Real-time tracking of pending approvals
- **Feedback Loop**: Structured feedback mechanism for iterative improvement
- **Phase Transition**: Automatic progression on approval, rollback on rejection

### 3. Multi-Agent Orchestration

Four specialized agents collaborate to complete complex tasks autonomously.

- **Planner Agent**: Breaks down tasks into atomic units with detailed plans
- **Coder Agent**: Generates clean, well-documented code following best practices
- **Tester Agent**: Creates comprehensive unit and integration tests
- **Reviewer Agent**: Reviews code for quality, security, and best practices

### 4. Phase-Based Workflow

Structured execution through four distinct phases with human gates between them.

- **Planning**: Task breakdown and atomic unit generation
- **Coding**: Implementation with automated testing
- **Testing**: Validation and edge case coverage
- **Review**: Code quality assessment and security checks

### 5. Project Management

Comprehensive project lifecycle management through the REST API.

- **Project Creation**: Automatic project type detection (Go, Python, TypeScript, Rust, Java, Kotlin)
- **File Browser**: Recursive file listing with metadata
- **File Content**: Direct file content retrieval with MIME type detection
- **Multi-Project Support**: Manage multiple projects simultaneously

### 6. Session Lifecycle Management

Full control over agent session state through a RESTful API.

- **Create**: Initialize new sessions with goals and project context
- **Start/Pause/Resume/Stop**: Complete lifecycle control
- **Session History**: Detailed history of all phase transitions and actions
- **State Persistence**: Session state survives daemon restarts

### 7. REST API v2

A complete REST API with OpenAPI/Swagger documentation.

- **14 Endpoints**: Comprehensive coverage of all operations
- **OpenAPI Spec**: Machine-readable API contract (`docs/openapi.yaml`)
- **Interactive Docs**: Swagger UI for exploration and testing (`docs/swagger-ui.html`)
- **Consistent Error Format**: Standardized error responses with user-friendly messages
- **Session Management**: Full CRUD operations for sessions
- **Project Management**: Project and file operations

### 8. Extensible Skills System

Add custom knowledge and tools to agents via configuration.

- **Knowledge Skills**: Prompt templates for specialized knowledge
- **Tool Skills**: Integration with code formatters, linters, and test runners
- **Configurable**: Define skills in `config.yaml` without code changes
- **Pre-Built Skills**: Includes task breakdown, code generation, testing, and review skills

### 9. LLM Provider Agnostic

Support for multiple LLM providers with phase-specific model overrides.

- **LM Studio**: Local LLM hosting support
- **Phase Overrides**: Different models for planning, coding, testing, and review
- **Provider Configuration**: Easy provider setup in `config.yaml`
- **Model Discovery**: Automatic model listing from providers

### 10. Structured Logging

Comprehensive logging with sensitive data redaction.

- **JSON/Text Formats**: Configurable log output format
- **Log Levels**: debug, info, warn, error
- **File Output**: Optional log file writing
- **Sensitive Data Redaction**: Automatic redaction of passwords, API keys, tokens

### 11. Multi-Language Support

Built-in executors for multiple programming languages.

- **Go**: gofmt, go vet, go test
- **Python**: Black, flake8, pytest
- **TypeScript**: prettier, eslint, jest
- **Rust**: rustfmt, clippy, cargo test
- **Java**: spotless, checkstyle, junit
- **Kotlin**: ktlint, detekt, junit

### 12. Project Type Detection

Automatic detection of project type and configuration.

- **Supported Types**: Go, Python, TypeScript, Rust, Java, Kotlin
- **Detection Rules**: Based on project files (go.mod, package.json, Cargo.toml, etc.)
- **Fallback**: Generic shell executor for unknown project types

### 13. Error Handling

Centralized error management with user-friendly messages.

- **Error Types**: internal, not_found, bad_request, unauthorized, forbidden, conflict
- **User Messages**: Clear, actionable error messages for end users
- **Developer Messages**: Detailed technical information for debugging
- **HTTP Status Codes**: Proper RESTful status codes

### 14. Version Management

Application version tracking and reporting.

- **Semantic Versioning**: Follows SemVer 2.0.0
- **Version Endpoint**: `/status` and `/health` include version info
- **Version Constant**: Centralized version in `internal/version/version.go`

---

## 🐛 Bug Fixes

### Infrastructure

- Fixed issue where daemon would crash on missing config file (now uses defaults)
- Fixed race condition in session store during concurrent access
- Fixed file path traversal vulnerability in file content endpoint
- Fixed memory leak in response cache (now properly evicts stale entries)

### Agent System

- Fixed planner agent generating duplicate atomic units
- Fixed coder agent ignoring formatting constraints
- Fixed tester agent not detecting edge cases in conditional logic
- Fixed reviewer agent missing security vulnerabilities in string handling

### API

- Fixed session status not updating after gate approval
- Fixed project file listing not handling empty directories
- Fixed gate response not triggering phase transition on approval
- Fixed session list not sorting by creation time

### Logging

- Fixed sensitive data not being redacted in JSON log format
- Fixed log file not being created when filename is specified
- Fixed log level not being applied correctly on startup

### UI

- Fixed phase tracker not updating during phase transitions
- Fixed file tree not refreshing after project file changes
- Fixed activity log not scrolling to latest entries
- Fixed mobile layout issues on session detail page

---

## ⚠️ Breaking Changes

### 1. Module Path Change

**Before (v1.x)**:
```go
import "github.com/nanaki-93/mini-orca/internal/..."
```

**After (v2.0)**:
```go
import "github.com/nanaki-93/mini-orca/v2/internal/..."
```

The module path has been updated to include the major version suffix `/v2` to comply with Go module versioning.

### 2. API Endpoint Changes

**Session Creation**:

| v1.x | v2.0 |
|------|------|
| `POST /create` | `POST /api/sessions` |
| Body: `{"goal": "..."}` | Body: `{"goal": "...", "project_path": "...", "project_type": "..."}` |

**Session Status**:

| v1.x | v2.0 |
|------|------|
| `GET /session/:id` | `GET /api/sessions/:id` |

**Session Actions**:

| v1.x | v2.0 |
|------|------|
| `POST /session/:id/start` | `POST /api/sessions/:id/start` |
| `POST /session/:id/pause` | `POST /api/sessions/:id/pause` |
| `POST /session/:id/resume` | `POST /api/sessions/:id/resume` |
| `POST /session/:id/stop` | `POST /api/sessions/:id/stop` |

**New Endpoints** (no v1.x equivalent):

- `GET /api/sessions` - List all sessions
- `GET /api/sessions/:id/gate` - Get gate status
- `POST /api/sessions/:id/gate` - Respond to gate
- `GET /api/projects` - List projects
- `POST /api/projects` - Create project
- `GET /api/projects/:id/files` - List project files
- `GET /api/projects/files/:path` - Get file content
- `GET /api/render/*` - HTMX rendering endpoints

**Removed Endpoints** (no v2.0 equivalent):

- `POST /create` - Replaced by `POST /api/sessions`
- `GET /session/:id` - Replaced by `GET /api/sessions/:id`
- `POST /session/:id/:action` - Replaced by individual action endpoints

### 3. Configuration Changes

**Phase Names**:

| v1.x | v2.0 |
|------|------|
| `plan` | `planning` |
| `develop` | `coding` |
| `validate` | `testing` |
| `audit` | `review` |
| `approve` | `human_review` |

**Agent Configuration**:

| v1.x | v2.0 |
|------|------|
| `agent: planner` | `agents: planner` |
| `agent: coder` | `agents: coder` |
| `agent: tester` | `agents: tester` |
| `agent: reviewer` | `agents: reviewer` |

**New Configuration Options**:

- `retry` section (max_retries, backoff_base, backoff_max)
- `logging.sensitive_keys` (list of keys to redact)
- `skills.knowledge` and `skills.tools` sections
- `models.phases.human_review` phase configuration

### 4. Error Response Format

**v1.x Format**:
```json
{"error": "Description of the error"}
```

**v2.0 Format**:
```json
{
  "type": "bad_request",
  "message": "Technical error description",
  "user_message": "User-friendly error message",
  "code": 400
}
```

### 5. Session State Model

**v1.x**: Simple state machine with limited phase transitions.

**v2.0**: Enhanced state model with:
- Explicit status tracking (`pending`, `running`, `paused`, `completed`, `cancelled`)
- Phase history tracking
- Atomic unit storage
- Test results storage
- Review report storage

---

## 📋 Migration Guide from v1 to v2

### Step 1: Update Dependencies

Update your go.mod to use the v2 module path:

```bash
# Before (v1.x)
go get github.com/nanaki-93/mini-orca@latest

# After (v2.0)
go get github.com/nanaki-93/mini-orca/v2@latest
```

### Step 2: Update Imports

Update all import paths in your code:

```bash
# Before (v1.x)
import "github.com/nanaki-93/mini-orca/internal/..."

# After (v2.0)
import "github.com/nanaki-93/mini-orca/v2/internal/..."
```

### Step 3: Update Configuration

Update your `config.yaml` to use v2.0 format:

```yaml
# Before (v1.x)
agent:
  planner:
    skills: [...]

# After (v2.0)
agents:
  planner:
    skills: [...]

# Add new sections
retry:
  max_retries: 3
  backoff_base: 1000
  backoff_max: 30000

logging:
  sensitive_keys:
    - password
    - secret
    - api_key
    - token
    - authorization

skills:
  knowledge:
    task_breakdown: "Break down complex tasks..."
    # ... more knowledge skills
  tools:
    formatter: "Format code according to project standards"
    # ... more tool skills
```

### Step 4: Update API Calls

Update all API endpoints from v1.x format to v2.0 format:

```bash
# Before (v1.x)
curl -X POST http://localhost:8080/create \
  -H "Content-Type: application/json" \
  -d '{"goal": "Implement auth"}'

# After (v2.0)
curl -X POST http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{"goal": "Implement auth", "project_path": "/path/to/project"}'
```

### Step 5: Update Error Handling

Update your error handling code to work with the new error format:

```go
// Before (v1.x)
var resp map[string]interface{}
json.Unmarshal(body, &resp)
error := resp["error"].(string)

// After (v2.0)
type AppError struct {
    Type        string `json:"type"`
    Message     string `json:"message"`
    UserMessage string `json:"user_message"`
    Code        int    `json:"code"`
}
var resp AppError
json.Unmarshal(body, &resp)
userMsg := resp.UserMessage
```

### Step 6: Update Phase References

Update any hardcoded phase references:

```go
// Before (v1.x)
phase := "plan"

// After (v2.0)
phase := "planning"
```

### Step 7: Test Thoroughly

After migration, test the following scenarios:

1. Session creation with project path
2. Session lifecycle (start, pause, resume, stop)
3. Gate approval workflow
4. Project file listing
5. File content retrieval
6. Error handling with new format

---

## 📊 Statistics

| Metric | v1.x | v2.0 | Change |
|--------|------|------|--------|
| API Endpoints | 5 | 14 | +180% |
| Supported Languages | 3 | 6 | +100% |
| Agent Types | 2 | 4 | +100% |
| Test Coverage | ~60% | ~85% | +25% |
| Lines of Code | ~5,000 | ~12,000 | +140% |
| Configuration Options | 15 | 35 | +133% |

---

## 🔮 What's Next?

### Planned for v2.1

- WebSocket support for real-time updates
- Docker and Docker Compose deployment
- Plugin system for custom agents
- Enhanced file editor with diff viewing
- Multi-user support with authentication

### Roadmap

- **v2.2**: AI-assisted code review with suggestions
- **v2.3**: Integration with GitHub/GitLab
- **v3.0**: Kubernetes deployment support, horizontal scaling

---

## 📚 Documentation

- **API Documentation**: [docs/api-contract.md](docs/api-contract.md)
- **OpenAPI Spec**: [docs/openapi.yaml](docs/openapi.yaml)
- **Interactive API Docs**: [docs/swagger-ui.html](docs/swagger-ui.html)
- **Configuration Guide**: [CONFIG.md](CONFIG.md)
- **Configuration Example**: [config.example.yaml](config.example.yaml)
- **Contributing Guide**: [CONTRIBUTING.md](CONTRIBUTING.md)

---

## 🙏 Acknowledgments

Thank you to all contributors who made v2.0 possible. Special thanks to the early testers who provided valuable feedback during the development process.

---

## 📄 License

Mini-Orca v2.0 is released under the MIT License. See [LICENSE](LICENSE) for details.

---

**End of Release Notes**
