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
2. In Editor, open one Go file and select an exact declaration to replace, or
   name a new top-level declaration to create.
3. Open a file-scoped chat session and request a proposal. Its project, file,
   base hash, mode, and target cannot change in later messages.
4. Edit only the returned declaration/import draft. Validate the displayed
   revision, run scoped checks, and review the read-only diff.
5. Confirm Apply for that named file and declaration. Undo can restore only the
   immediately preceding unchanged apply.

Mini-Orca never creates multi-file changes, edits source directly, runs scans
automatically, writes drafts automatically, commits, or pushes.

## Project intelligence

Import and reindex are deterministic. Verified Go parser, vet, and test scans,
one-file semantic analysis, and Analyze-all are explicit user actions. Bugs
keeps verified tool findings distinct from AI suggestions, including provenance,
confidence, lifecycle state, and freshness.

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
analysis, file analysis, Analyze-all, or a chat message—after the user reviews
the destination. Keep `config.yaml` local; it is ignored by Git.

## Migrating from the preview workflow

The former whole-file generation and activity-oriented UI is retired. Do not
send a request to `POST /api/chat/message`: it deliberately returns `410 Gone`.
New integrations open a file-scoped session, send its messages, then read or
edit the resulting declaration draft before validation, checks, review, and an
explicit Apply. `GET /api/chat/history` remains only as a deprecated alias for
source-free activity; it is not a conversation transcript. See the [API guide](API.md)
for the exact route contract.

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
