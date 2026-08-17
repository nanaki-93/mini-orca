# v4.2 desktop-only release acceptance

Run this checklist from a clean working tree after `make check`.

## Fixture

Use `internal/project/testdata/release-fixture`. It contains ignored placeholder
secrets, a Go source file, a Go test, and intentionally malformed Go source
stored with a fixture extension so formatter and compiler discovery skip it.
Copy it to a temporary directory. For Git coverage, run `git init` in one copy;
leave a second copy unchanged to verify the non-Git fallback.

## Automated evidence

| Milestone requirement | Evidence |
|---|---|
| Import/reindex, deterministic facts, safe relative paths | `go test ./internal/project ./internal/api/handlers` |
| Context exclusions, token budgeting, and no secret prompt delivery | `go test ./internal/project -run 'Context|Policy'` |
| Semantic one-file summary, malformed/timeout failures, freshness | `go test ./internal/app -run 'AnalyzeFile|AnalyzeAll'` |
| Real symbol selection and strict target-only validation | `go test ./internal/project -run 'Symbol|Generation'` |
| Generation parsing, stale protection, checks, Apply/Undo | `go test ./internal/app ./internal/api/handlers` |
| Analyze-all cancellation and revision safety | `go test ./internal/app -run AnalyzeAll` |
| Candidate comparison and source-free report export | `go test ./internal/app -run 'CompareCandidates|ExportReview'` |
| Offline, malformed, non-200, oversized, empty, and canceled provider behavior | `go test ./internal/llm ./internal/agent ./internal/api/handlers` |
| Desktop import-to-preview state, summaries, comparison client, and keyboard behavior | `./desktop/gradlew -p desktop test` |
| No browser UI routes or assets remain | `go test ./cmd/daemon -run 'BrowserUI|OpenAPIRoutes'` and `rg -n -i 'htmx|/static/|/api/render/' cmd internal --glob '*.go'` |

## Desktop manual smoke

Start the daemon and run `./desktop/gradlew -p desktop run`. Import a copied
fixture and confirm: file navigation, summary and context inspection, a selected
symbol, a rejected out-of-scope preview, focused checks, Apply/Undo, Analyze-all
cancel, comparison/export, and an offline-provider error. Repeat the keyboard
and narrow-window checks in `desktop/README.md`.

No accepted operation may name more than one target file and symbol, and Apply
must remain unavailable until scope validation, base hash, and required checks
pass.
