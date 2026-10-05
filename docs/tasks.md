# Analysis and summary improvements

Improve the Wails/React summary, feature discovery and analysis model selection. Implement the tasks in order and commit every verified step separately; preserve explicit consent, immutable identities and legacy defaults.

## Task 1 — [x] Collapse summary architecture and flows

**Target files**
- `desktop/webapp/frontend/src/overview.tsx` — default-closed Architecture and Project flows disclosures.
- `desktop/webapp/frontend/tests/run.mjs` — collapsed/expanded Summary, diagram fallbacks and local-only navigation.

**Inputs / dependencies**
- Existing Disclosure control and inert diagram renderer.

**Implementation rules**
- Default to closed on Summary mount, while keeping all saved content and Explore accessible by keyboard. The dedicated diagrams page remains expanded.

**Verification command**
`make web-test && git diff --check`

## Task 2 — [x] Replace analysis status badges with accessible dots

**Target files**
- `desktop/webapp/frontend/src/ui.tsx` — shared status dot.
- `desktop/webapp/frontend/src/overview.tsx` — top-right summary status dots.
- `desktop/webapp/frontend/src/analysis.tsx` — compact run/file states.
- `desktop/webapp/frontend/src/results.tsx` — compact analysis states.
- `desktop/webapp/frontend/src/features.tsx` — compact report state.
- `desktop/webapp/frontend/src/style.css` — indicator colors and placement.
- `desktop/webapp/frontend/tests/run.mjs` — state, accessibility, layout and contrast checks.
- `desktop/webapp/frontend/tests/fixture.mjs` — status fixtures.

**Inputs / dependencies**
- Task 1; existing wire status semantics.

**Implementation rules**
- Completed and completed_empty are both green. Successful reports are green, incomplete/pending/stale/unavailable states yellow, failures red. Remove visible completion pills from affected surfaces.
- Status remains available as accessible text and a tooltip. Keep error details and unknown counts distinct from zero.

**Verification command**
`make web-test && git diff --check`

## Task 3 — [x] Remove redundant suggestion badges

**Target files**
- `desktop/webapp/frontend/src/ui.tsx` — suppress suggested/ai_suggestion badges for all shared callers.
- `desktop/webapp/frontend/src/features.tsx` — remove redundant AI pill.
- `desktop/webapp/frontend/src/overview.tsx` — remove summary suggestion pill.
- `desktop/webapp/frontend/src/results.tsx` — retain meaningful confidence/provenance.
- `desktop/webapp/frontend/tests/run.mjs` — badge removal with content and verified provenance retained.

**Inputs / dependencies**
- Task 2.

**Implementation rules**
- Remove redundant labels, not suggestion content or actions. Preserve distinctions between advisory ideas, verified findings and performance hypotheses through existing headings/source metadata.

**Verification command**
`make web-test && git diff --check`

## Task 4 — [x] Make feature empty and failure messages concise

**Target files**
- `desktop/webapp/frontend/src/features.tsx` — concise unavailable/not-generated/empty/filtered/failed copy and recovery.
- `desktop/webapp/frontend/tests/run.mjs` — saved versus requested failure and empty/filter cases.
- `desktop/webapp/frontend/tests/fixture.mjs` — failure fixtures.
- `internal/app/feature_suggestions.go` — concise persisted failure message.
- `internal/app/feature_suggestions_test.go` — retain previous ideas on failure.

**Inputs / dependencies**
- Task 3; saved failures raise an alert only after requested generation in this project session.

**Implementation rules**
- Empty successful discovery is an empty result. Preserve unavailable/failed/not-generated distinctions and useful diagnostics in optional details. Keep previous ideas and retry actions accessible.

**Verification command**
`go test ./internal/app -run 'TestFeature' && make web-test && git diff --check`

## Task 5 — [ ] Run feature discovery independently with more time

**Target files**
- `internal/app/analysis_run.go` — owned concurrent file/feature workers with separate deadlines and coordinated settlement.
- `internal/app/analysis_run_features.go` — feature publication/cancellation with cumulative attempt guards.
- `internal/app/analysis_run_progress.go` — aggregate worker states if required.
- `internal/app/analysis_run_features_test.go` — deterministic concurrency, file-budget expiry, pause/cancel, retries and stale-result cases.
- `internal/app/analysis_run_test.go` — lifecycle fixtures if required.
- `internal/app/feature_suggestions.go` — longer bounded feature deadline and grounded discovery prompt.
- `internal/app/feature_suggestions_test.go` — deadline/prompt behavior and valid empty results.
- `desktop/webapp/frontend/src/analysis.tsx` — simultaneous file and feature progress.
- `desktop/webapp/frontend/tests/run.mjs` — independent progress and available saved findings.
- `desktop/webapp/frontend/tests/fixture.mjs` — concurrent progress fixtures.
- `desktop/webapp/README.md` — independent discovery behavior.
- `docs/api-contract.md` — lifecycle/deadline contract.
- `docs/openapi.yaml` — align descriptions.

**Inputs / dependencies**
- Task 4; both workers remain owned by one admitted run.

**Implementation rules**
- Feature discovery starts independently of file batches and receives at least ten minutes, honoring longer configured Analyze timeouts. File dispatch remains bounded by its own batch/time budget.
- Pause/cancel/project changes and persistence faults stop both workers. Keep source/goal/selection identities, consent, exclusions, durable cumulative retries and late-publication guards.
- Improve discovery instructions around current capabilities, goals and useful gaps without forcing fabricated suggestions or concealing provider failure.

**Verification command**
`go test -race ./internal/app && go test ./internal/api/handlers ./cmd/daemon && make web-test && git diff --check`

## Task 6 — [ ] Select configured models before analysis

**Target files**
- `internal/app/analysis_models.go` — immutable configured-profile choices for code/review/features.
- `internal/app/analysis_models_test.go` — actual dispatch, defaults, invalid profiles, cache isolation, consent, admission and resume.
- `internal/app/analysis_run.go` — optional model choices on preview/start/plan and stage requests.
- `internal/app/analysis_run_preview.go` — chosen-provider identity, cache, availability and admission.
- `internal/app/analysis_run_retry.go` — chosen-model retry allowances/freshness.
- `internal/app/analysis_run_features.go` — selected feature runtime.
- `internal/app/analysis_run_store.go` — optional choice validation and schema-1 compatibility.
- `internal/app/analysis_run_results.go` — chosen-model evidence reads.
- `internal/app/analysis_file.go` — selected runtime dispatch/snapshots without global mutation.
- `internal/app/file_analysis.go` — explicit semantic runtime and cache identity.
- `internal/app/performance_review.go` — selected runtime snapshot validation.
- `internal/app/security_review.go` — selected runtime validation and actual-provider consent.
- `internal/app/analysis_selection_status.go` — model-aware status helpers if required.
- `internal/app/analysis_compatibility.go` — retain fixed-scope compatibility defaults.
- `internal/app/analysis_run_test.go` — admission fixture helpers.
- `internal/app/analysis_run_features_test.go` — feature provider/prompt fixtures if required.
- `internal/app/feature_suggestions.go` — selected-provider confirmation.
- `internal/api/handlers/analysis_handler_test.go` — model preview/start and rejection contract.
- `desktop/src/main/kotlin/io/miniorca/desktop/Models.kt` — compatible optional model fields.
- `desktop/webapp/frontend/src/models.ts` — typed model choices.
- `desktop/webapp/frontend/src/workspace.ts` — pass selections and confirm start intent explicitly.
- `desktop/webapp/frontend/src/analysis.tsx` — configured-model selects and checkbox removal.
- `desktop/webapp/frontend/tests/run.mjs` — choices, refreshed previews, consent, no-dispatch and resume.
- `desktop/webapp/frontend/tests/fixture.mjs` — selected models and preview fixtures.
- `desktop/webapp/README.md` — selection and confirmation behavior.
- `docs/api-contract.md` — optional inputs, defaults and identity behavior.
- `docs/openapi.yaml` — optional model schemas.

**Inputs / dependencies**
- Task 5. Use configured models unless the user requests provider discovery before this step begins.

**Implementation rules**
- Offer configured models for code, review and feature discovery. Defaults use Bug for code and Analyze for review/features. Choices affect dispatch, provenance, caches, accounting and preview identity, apply only to this run and survive resume.
- Reject invalid or changed choices before dispatch. Preserve old clients/runs omitting selections; client input never supplies credentials or endpoints.
- Replace preview consent checkboxes with explicit start confirmation naming remote destinations and Security review. Model selection and preview remain free of provider requests.

**Verification command**
`go test -race ./internal/app ./internal/api/handlers && make web-test && ./scripts/desktop-gradle.sh test spotlessCheck detekt && git diff --check`

## Task 7 — [ ] Review and validate all improvements

**Target files**
- `docs/tasks.md` — verified final completion state.

**Inputs / dependencies**
- Tasks 1–6.

**Implementation rules**
- Review the full diff for scope, accessibility, failure visibility, identities, cancellation, accounting and compatibility. Run full gates/native packaging and accurately report unrelated or blocked checks. Commit completion and leave a clean working tree.

**Verification command**
`go fmt ./... && ./scripts/validate.sh && make web-build && git diff --check`
