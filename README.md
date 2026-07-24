# Mini-Orca v2.0

> AI-powered code generation with human-in-the-loop orchestration

Mini-Orca is a multi-agent system that plans, implements, tests, and reviews code changes through a structured 5-phase workflow with human approval gates at every decision point.

---

## ✨ Features

- **5-Phase Workflow** — Planning → Coding → Testing → Review → Human Approval
- **Multi-Agent Architecture** — Specialized agents (Planner, Coder, Tester, Reviewer)
- **Skills System** — 20+ predefined skills, fully configurable per agent
- **Configurable Models** — Each phase can use a different AI model/provider
- **IDE-like Dashboard** — HTMX + Alpine.js frontend with dark theme
- **Git Integration** — Auto-commit, diff, and status tracking
- **Language Agnostic** — Auto-detects Go, Kotlin, Java, Rust, TypeScript, Python
- **Structured Logging** — Console + file output with rotation
- **Error Handling** — Comprehensive error types with retry logic
- **State Persistence** — JSON-based session and plan storage

---

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- LM Studio (or compatible OpenAI API)
- Node.js 18+ (for frontend dependencies)

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/mini-orca.git
cd mini-orca

# Install dependencies
go mod download

# Build
go build ./...

# Run
go run cmd/daemon/main.go
```

### Configuration

Create `config/config.yaml`:

```yaml
models:
  active_provider: "lm-studio"
  providers:
    lm-studio:
      base_url: "http://127.0.0.1:1234"
  phases:
    planning:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.3
    coding:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.1
    testing:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.2
    review:
      provider: "lm-studio"
      model: "qwen/qwen3-coder-30b"
      temperature: 0.2

agents:
  planner:
    skills:
      - solid_principles
      - clean_code
      - architecture_design
      - task_breakdown
  coder:
    skills:
      - solid_principles
      - clean_code
      - function_generation
      - struct_design
  tester:
    skills:
      - unit_testing
      - integration_testing
      - coverage_analysis
  reviewer:
    skills:
      - solid_principles
      - clean_code
      - style_check
      - security_audit

server:
  port: 8080
```

### Usage

1. Open `http://localhost:8080` in your browser
2. Create a new session with your project directory
3. The planner will analyze your requirements
4. Review and approve the generated plan
5. Watch as the system implements, tests, and reviews code
6. Final human approval before completion

---

## 📁 Project Structure

```
mini-orca/
├── cmd/
│   └── daemon/
│       └── main.go              # Entry point
├── internal/
│   ├── agent/                   # Multi-agent system
│   │   ├── planner.go           # Planning agent
│   │   ├── coder.go             # Coding agent
│   │   ├── tester.go            # Testing agent
│   │   ├── reviewer.go          # Review agent
│   │   ├── skills/              # Skills system
│   │   └── prompts/             # Prompt templates
│   ├── api/                     # HTTP API
│   │   ├── handlers/            # Request handlers
│   │   └── templates/           # HTMX templates + static assets
│   ├── config/                  # Configuration management
│   ├── errors/                  # Structured error types
│   ├── logging/                 # Structured logging
│   ├── model/                   # Model abstraction layer
│   │   ├── lm_studio.go         # LM Studio provider
│   │   └── router.go            # Phase-based model routing
│   ├── orchestrator/            # Workflow orchestration
│   │   ├── orchestrator.go      # Main orchestrator
│   │   ├── phase_router.go      # Phase management
│   │   └── human_gate.go        # Approval handling
│   ├── state/                   # State persistence
│   │   ├── session.go           # Session/Plan types
│   │   └── store.go             # JSON storage
│   ├── tools/                   # Project utilities
│   │   ├── executor.go          # Command executor
│   │   ├── file_ops.go          # File operations
│   │   ├── git_ops.go           # Git integration
│   │   └── formatter.go         # Code formatting
│   └── types/                   # Shared types
├── config/
│   └── config.yaml              # Configuration file
├── docs/
│   └── API.md                   # API documentation
├── plan/
│   ├── IMPLEMENTATION-PLAN.md   # Implementation roadmap
│   └── kotlin-native.md         # Kotlin desktop plan
├── go.mod
└── README.md
```

---

## 🔧 API Reference

See [docs/API.md](docs/API.md) for complete API documentation.

### Quick API Examples

```bash
# Get current session
curl http://localhost:8080/api/session

# Create a new session
curl -X POST http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{"name": "My Project", "project_dir": "/path/to/project"}'

# Approve planning phase
curl -X POST http://localhost:8080/api/approve \
  -H "Content-Type: application/json" \
  -d '{"phase": "planning_review", "message": "Approved"}'
```

---

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run specific package
go test ./internal/state/...
```

---

## 📚 Documentation

- [Implementation Plan](plan/IMPLEMENTATION-PLAN.md) — Step-by-step roadmap
- [API Reference](docs/API.md) — Complete API documentation
- [Kotlin Native Plan](plan/kotlin-native.md) — Desktop app roadmap

---

## 🗺️ Roadmap

| Phase | Status | Description |
|-------|--------|-------------|
| Model Abstraction | ✅ | Provider interface with LM Studio |
| Multi-Agent System | ✅ | 4 specialized agents with skills |
| Tool Executor | ✅ | Auto-detect project types |
| Orchestrator | ✅ | 5-phase workflow with human gates |
| HTMX Frontend | ✅ | IDE-like dashboard |
| Backend Polish | ✅ | Errors, logging, tests, docs |
| Kotlin Desktop | ⏳ | Native desktop app (planned) |

---

## 🤝 Contributing

Contributions are welcome! Please read the [Implementation Plan](plan/IMPLEMENTATION-PLAN.md) for the current roadmap.

---

## 📄 License

MIT
