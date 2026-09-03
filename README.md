# mini-orca

Mini-Orca is a local-first, Go-first desktop coding assistant for one deliberate
change: one active project, one open file, one selected or new declaration, one
editable AI draft, validation, focused checks, and explicit Apply.

The Compose Desktop client has four workspaces—Summary, Analysis, Bugs, and
Editor. Source and composed diff views remain selectable and read-only. Only the
isolated declaration draft is editable.

## Safe workflow

1. Import one local project to build deterministic facts and a structured
   project analysis.
2. In Bugs, review a fresh suggested task and use **Prepare fix**, or in Editor
   select an exact declaration to replace, or name a new top-level declaration
   to create.
3. Open a file-scoped chat session and request a proposal. Its project, file,
   base hash, mode, and target cannot change in later messages.
4. Edit only the returned declaration/import draft. Validate the displayed
   revision, run focused checks, and review the read-only diff. A reviewed Go
   task test runs only in a temporary copy; a failed task check can offer up to
   three explicit **Revise with check output** requests in the same session.
5. Confirm Apply for that named file and declaration. Undo can restore only the
   immediately preceding unchanged apply.

Mini-Orca never creates multi-file changes, edits source directly, runs scans
automatically, writes drafts or tests automatically, commits, or pushes.

## Model scopes and provider compatibility

`analyze`, `bug`, and `function` are three fixed independently configured
scopes. Project import uses `analyze`; selected-file analysis and Analyze-all
use `bug`; file-scoped declaration proposals and explicit repairs use
`function`. All three `model_scopes` profiles are required at startup; there is
no flat `llm` fallback or agent-model override.

Each explicit scope uses the OpenAI Chat Completions-compatible API base and a
model ID. An optional per-scope `reasoning_effort` is passed unchanged through
the Chat Completions request when the selected provider/model supports it. This
represents OpenAI, Claude-compatible, Gemini-compatible, Ollama, LM Studio, and
compatible gateways without native vendor SDKs, streaming, tool calls, or
provider account management. See [CONFIG.md](CONFIG.md) for safe placeholder
examples and migration rules.

The daemon loads configuration at startup; restart it after changing
`config.yaml`. A non-loopback scope requires an explicit confirmation for its
own prompt request. That confirmation never authorizes another scope. Model
metadata, caches, and the Desktop display expose no API keys; changing a scope
model, provider, or reasoning effort makes old AI interpretations stale.

## Project intelligence

Import and reindex are deterministic. Verified Go parser, vet, and test scans,
one-file semantic analysis, and Analyze-all are explicit user actions. Bugs
keeps verified tool findings distinct from AI suggestions, including provenance,
confidence, lifecycle state, and freshness.

The canonical persisted project interpretation is
`.mini-orca/project-analysis.json`. Mini-Orca no longer reads or writes the
retired `.mini-orca/analysis.md` projection or activity files. Existing legacy
files are left untouched and may be removed manually from a project’s
`.mini-orca` directory when no longer needed.

Exact declaration editing, composition, and required parsing/formatting are
currently Go-first. Other languages can have conservative analysis and symbol
information, but not equivalent exact editing or validators.

## Run locally

Prerequisites: Go 1.22+ and a configured OpenAI-compatible or local LLM endpoint.

```bash
cp config.example.yaml config.yaml
go run ./cmd/daemon
./desktop/gradlew -p desktop run
```

The daemon binds to `http://localhost:9090` on loopback by default. The desktop
uses that URL unless `MINI_ORCA_URL` is set. A non-loopback provider must be
explicitly confirmed in each request that can send prompt content—project
analysis, file analysis, Analyze-all, a chat message, or an explicit repair—after
the user reviews that scope’s destination. Keep `config.yaml` local; it is
ignored by Git.

## Loopback API contract

Integrations open a file-scoped session, send its messages, then edit the
returned declaration draft before validation, checks, review, and an explicit
Apply. The supported routes are listed once in the [API guide](API.md) and
[route contract](docs/api-contract.md).

## Documentation

- [Desktop usage](desktop/README.md), including keyboard and responsive smoke checks
- [API guide](API.md) and the authoritative [route contract](docs/api-contract.md)
- [OpenAPI contract](docs/openapi.yaml)
- [Configuration reference](CONFIG.md)
- [Release notes](RELEASE_NOTES.md)

## Development checks

```bash
go test ./...
./desktop/gradlew -p desktop test
make check
```

## License

MIT
