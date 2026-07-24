# Release Notes — Mini-Orca v2.0

> Release Date: January 2024

---

## 🎉 What's New in v2.0

Mini-Orca v2.0 is a complete rewrite with a multi-agent architecture, structured error handling, comprehensive logging, and a production-ready frontend.

---

## ✨ New Features

### Multi-Agent Architecture
- **Planner Agent** — Analyzes requirements and creates execution plans with atomic units
- **Coder Agent** — Implements exactly one atomic unit per execution
- **Tester Agent** — Creates and runs tests for implemented code
- **Reviewer Agent** — Reviews code quality, style, and correctness

### Skills System
- 20+ predefined skills (knowledge + tool types)
- Configurable per-agent in YAML
- Custom skills via API
- Export/Import as JSON

### IDE-like Frontend
- HTMX + Alpine.js for real-time updates
- File tree with syntax highlighting
- Phase tracker with progress indicators
- Activity log with timestamps
- Dark/Light theme support
- Responsive design

### Structured Error Handling
- Comprehensive error types for all failure scenarios
- Error codes for API responses
- Retry logic with exponential backoff
- Helper functions: `IsNotFound()`, `IsRetryable()`, `IsTerminal()`

### Structured Logging
- Console + file output with rotation
- 5 log levels: DEBUG, INFO, WARN, ERROR, FATAL
- Configurable log retention
- Test logger for unit testing

### Language-Agnostic Tools
- Auto-detects: Go, Kotlin, Java, Rust, TypeScript, Python
- Project-specific build/test/lint commands
- Atomic file operations
- Git integration with auto-commit

---

## 🏗️ Architecture Changes

### Before (v1.x)
```
Single Agent → Direct LLM calls → Shell commands
```

### After (v2.0)
```
Planner → Coder → Tester → Reviewer
    ↓        ↓        ↓         ↓
  Skills  Skills   Skills    Skills
    ↓        ↓        ↓         ↓
  Model Router → LM Studio/OpenAI
```

---

## 📦 Package Structure

| Package | Description | Files |
|---------|-------------|-------|
| `agent/` | Multi-agent system | 8 |
| `api/` | HTTP API + templates | 16 |
| `config/` | Configuration management | 2 |
| `errors/` | Structured error types | 2 |
| `logging/` | Structured logging | 2 |
| `model/` | Model abstraction | 3 |
| `orchestrator/` | Workflow orchestration | 4 |
| `state/` | State persistence | 3 |
| `tools/` | Project utilities | 6 |
| `types/` | Shared types | 1 |

---

## 🧪 Testing

| Package | Tests | Status |
|---------|-------|--------|
| `internal/errors` | 19 | ✅ |
| `internal/logging` | 13 | ✅ |
| `internal/model` | 18 | ✅ |
| `internal/state` | 22 | ✅ |
| `internal/tools` | 17 | ✅ |
| **Total** | **89** | **All Passing** |

---

## 📚 Documentation

- [README.md](README.md) — Updated project overview
- [docs/API.md](docs/API.md) — Complete API reference
- [plan/kotlin-native.md](plan/kotlin-native.md) — Kotlin desktop app plan
- Package-level `doc.go` files for all internal packages

---

## 🔧 Configuration

New `config.yaml` structure:

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
    skills: ["solid_principles", "clean_code"]
  coder:
    skills: ["function_generation", "clean_code"]
  tester:
    skills: ["unit_testing", "coverage_analysis"]
  reviewer:
    skills: ["style_check", "security_audit"]

server:
  port: 8080
```

---

## 🐛 Bug Fixes

- Fixed session directory detection in `ListSessions()`
- Fixed plan persistence to create session directories
- Fixed logging test for slog level zero (INFO is 0)
- Fixed frontend API endpoint consistency

---

## 🚀 Performance Improvements

- Debounced file refresh operations
- Optimized CSS with custom properties
- Reduced bundle size
- Faster template rendering

---

## ♿ Accessibility

- Focus indicators for keyboard navigation
- Skip-to-content link
- ARIA labels on interactive elements
- Screen reader support

---

## 📋 Migration Guide (v1.x → v2.0)

### Breaking Changes

1. **Configuration** — New `config.yaml` structure (see above)
2. **API Endpoints** — All endpoints prefixed with `/api/`
3. **State Format** — JSON format changed (sessions stored in subdirectories)

### Migration Steps

1. Back up your existing sessions: `cp -r state/ state-backup/`
2. Update `config.yaml` to new format
3. Start the daemon — it will create new state directories
4. Existing sessions are preserved in `state-backup/`

---

## 📊 Statistics

| Metric | Value |
|--------|-------|
| Go Source Files | 42 |
| Test Files | 5 |
| Total Tests | 89 |
| Test Pass Rate | 100% |
| Lines of Documentation | 500+ |
| API Endpoints | 12 |
| Predefined Skills | 20 |
| Supported Languages | 6 |

---

## 🙏 Credits

Built with:
- [Go](https://golang.org/)
- [HTMX](https://htmx.org/)
- [Alpine.js](https://alpinejs.dev/)
- [LM Studio](https://lmstudio.ai/)

---

*Mini-Orca v2.0 — AI-powered code generation with human oversight*
