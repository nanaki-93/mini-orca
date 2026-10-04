# AI changes, feature suggestions, and project instructions

Implement a shared proposal workflow in the Wails/React client: selected findings or feature requests become checked, versioned changes with read-only diffs, explicit Apply, guarded Undo, and local history. Add scoped AGENTS.md loading and a guided editor, plus separate, goal-aware feature suggestions. Preserve existing declaration APIs and the legacy client; broader changes initially support Go source and Markdown, including new files, with a maximum of eight explicitly selected paths.

## Task 1 — [x] Resolve scoped project instructions

**Target files**
- `internal/project/instructions.go` — bounded, policy-filtered root-to-directory AGENTS.md resolution and instruction identity.
- `internal/project/instructions_test.go` — inheritance, exclusion, missing files, unsafe paths and size limits.
- `internal/project/write_path.go` — contained, symlink-free paths for existing and new files.
- `internal/project/write_path_test.go` — traversal, aliases, symlinks and missing parent behavior.
- `internal/project/function_context.go` — include effective instructions in declaration context and manifest.
- `internal/project/function_context_test.go` — prove applicable instructions reach prompts.

**Inputs / dependencies**
- Existing context policy, path containment, context manifests and source hash conventions.

**Implementation rules**
- AGENTS.md remains the source of truth. Resolve root instructions followed by applicable directory instructions; show origin and scope. Missing files are valid; unreadable, oversized or unsafe files fail explicitly.
- Instructions guide generation and never grant remote-provider consent, execution trust or Apply authority. Do not automatically run command text found in instructions.
- Resolve write paths within the canonical project and reject symlinks, protected metadata, traversal and ambiguous spellings. No source writes occur during resolution.

**Verification command**
`go test ./internal/project`

## Task 2 — [x] Persist shared change conversations and generate proposals

**Target files**
- `internal/app/change_session.go` — immutable selected scope, revisioned conversations and provider generation.
- `internal/app/change_store.go` — private, bounded local history with strict persisted schema and conflict detection.
- `internal/app/change_context.go` — bounded selected source, effective instructions and strict structured response validation.
- `internal/app/change_types.go` — change/session/evidence wire types and identities.
- `internal/app/change_session_test.go` — history/resume, revisions, cancellation, malformed responses, stale scope and consent.
- `internal/app/service.go` — workflow concurrency ownership.

**Inputs / dependencies**
- Task 1; configured Function scope, retry transport and atomic storage primitives.

**Implementation rules**
- Fixes, optimizations, new features and instruction edits use one stored proposal representation. A model may change only explicitly captured paths; no deletes, shell commands or retargeting.
- Local history contains source/drafts and conversation text only in excluded project-local metadata with private permissions. Read/restore must never dispatch a provider or grant previous approval/check authority.
- Bound files, source, messages, history and model responses. Preserve cancellation and recheck project, policy, instruction and source identities after asynchronous work.
- New revisions clear check/review evidence. A manual instruction proposal uses the same diff/review lifecycle without a provider call.

**Verification command**
`go test ./internal/app -run 'TestChange'`

## Task 3 — [x] Check, review, apply and undo proposals across files

**Target files**
- `internal/app/change_checks.go` — formatting/parsing, trusted copied-workspace checks, bounded repair evidence and review guards.
- `internal/app/change_apply.go` — durable recovery journal, guarded grouped writes, rollback and Undo.
- `internal/app/change_checks_test.go` — checks, trust, changed candidates and baseline regressions.
- `internal/app/change_apply_test.go` — approval, fresh hashes, new files, partial writes and restart recovery.
- `internal/app/change_types.go` — evidence and mutation receipts.
- `internal/app/change_store.go` — store current evidence and recovery state.

**Inputs / dependencies**
- Tasks 1 and 2; existing controlled Go argv, copied workspace, process cancellation and source write primitives.

**Implementation rules**
- Parse and format Go candidates; display Markdown changes as text. Tests/vet run only with project execution trust, in a copied workspace, using fixed argv. Requested failed checks block Apply.
- Repair is explicit, limited to three attempts and tied to the current failed revision. Review records the exact proposal hash; Apply requires current review, checks and explicit confirmation.
- Revalidate every captured target and policy before writing. Journal original content before the first grouped write; recover failed or interrupted writes without overwriting unrelated edits. Never describe partial source mutation as no mutation.
- Undo checks every post-Apply hash and restores/deletes only the files changed by the proposal. Persist recovery across restart and preserve permissions.

**Verification command**
`go test ./internal/app -run 'TestChange'`

## Task 4 — [ ] Generate and save goal-aware feature suggestions

**Target files**
- `internal/app/feature_suggestions.go` — separate source-free advisory suggestions, project goals, status and freshness.
- `internal/app/feature_suggestions_test.go` — structured output, selected context, dismissal/save, stale input and failures.
- `internal/app/change_context.go` — reuse policy-filtered context and instruction rules.

**Inputs / dependencies**
- Tasks 1 and 2; configured Analyze scope and deterministic project facts.

**Implementation rules**
- Each idea has benefit, local evidence, affected paths, estimated effort, acceptance criteria and a stable identity. Do not count ideas as Bugs/Performance/Security findings or verified evidence.
- Generate only after explicit user intent and required provider consent. Save project goals, ideas and saved/dismissed status locally; history reads are passive.
- Validate all suggested paths against the eligible inventory. Preserve generation failure and stale states; reject invalid structured output and retain previous readable results.

**Verification command**
`go test ./internal/app -run 'TestFeature'`

## Task 5 — [ ] Expose guarded workflow and instruction APIs

**Target files**
- `internal/api/handlers/change_handler.go` — thin shared-change, history and instruction-preview handlers.
- `internal/api/handlers/change_handler_test.go` — successful and rejected HTTP requests.
- `internal/api/handlers/feature_handler.go` — suggestion generation, goals and triage handlers.
- `internal/api/handlers/feature_handler_test.go` — suggestion request contracts.
- `cmd/daemon/main.go` — route registrations.
- `desktop/webapp/internal/bridge/client.go` — native allowlist additions.
- `desktop/webapp/internal/bridge/client_test.go` — allowlist and rejection coverage.
- `docs/api-contract.md` — new contracts, local history and consent/recovery semantics.
- `docs/openapi.yaml` — corresponding paths and request/response schemas.

**Inputs / dependencies**
- Tasks 1–4; existing strict HTTP decoding and loopback/browser policy.

**Implementation rules**
- New endpoints use typed requests, bounded decoding and existing structured error responses. Keep all existing declaration contracts compatible.
- Preview/list reads perform no provider dispatch, code execution or source writes. Instruction preview exposes resolved files, effective text and preset choices.
- Mutation requests carry project/session/revision/hash guards. Provider generation and trusted execution keep independent consent.

**Verification command**
`go test ./internal/api/handlers ./cmd/daemon && (cd desktop/webapp && go test ./internal/bridge)`

## Task 6 — [ ] Deliver chat, automatic fix preparation and local history

**Target files**
- `desktop/webapp/frontend/src/change-workspace.tsx` — primary chat, selected file scope, proposal diff, checks, review/apply/undo and history.
- `desktop/webapp/frontend/src/workspace.ts` — shared-change state, guarded asynchronous actions and bounded automatic preparation.
- `desktop/webapp/frontend/src/models.ts` — typed new contracts.
- `desktop/webapp/frontend/src/main.tsx` — primary Chat workspace routing.
- `desktop/webapp/frontend/src/results.tsx` — finding-to-chat handoff and Prepare fix action.
- `desktop/webapp/frontend/src/editor.tsx` — simplify declaration controls and reachability from source inspection.
- `desktop/webapp/frontend/src/style.css` — responsive chat/diff layouts.
- `desktop/webapp/frontend/tests/fixture.mjs` — deterministic new API fixture behavior.
- `desktop/webapp/frontend/tests/run.mjs` — chat revisions, approval invalidation, history, fixes and compact/large-text checks.

**Inputs / dependencies**
- Task 5; existing immutable workspace ownership and UI/accessibility guidelines.

**Implementation rules**
- Chat is the main implementation surface. Source inspection and existing declaration draft editing remain reachable through secondary controls; read-only source/diffs never become unrestricted editors.
- Generate, validate and prepare checks from explicitly selected fixes; stop at diff review. Seek execution trust before code checks and fresh provider consent for repair. Preserve failure, cancellation and unavailable evidence.
- History restore is passive; stale proposals remain readable but cannot be applied. Guard late responses across navigation and project/session/revision changes.
- A proposal can include up to eight selected Go/Markdown paths, including user-named new files. Show scope before provider dispatch and all file diffs before approval.

**Verification command**
`make web-test`

## Task 7 — [ ] Deliver Features and the AGENTS.md wizard

**Target files**
- `desktop/webapp/frontend/src/features.tsx` — goals, advisory suggestion cards, save/dismiss and chat handoff.
- `desktop/webapp/frontend/src/instructions.tsx` — guided scope/preset/custom-text editor, existing content and diff preview.
- `desktop/webapp/frontend/src/workspace.ts` — instruction and suggestion lifecycles.
- `desktop/webapp/frontend/src/models.ts` — typed instruction/suggestion contracts.
- `desktop/webapp/frontend/src/main.tsx` — Features and Instructions navigation.
- `desktop/webapp/frontend/src/analysis.tsx` — Features section entry.
- `desktop/webapp/frontend/src/style.css` — responsive wizard/suggestion layout.
- `desktop/webapp/frontend/tests/fixture.mjs` — instruction and feature fixture behavior.
- `desktop/webapp/frontend/tests/run.mjs` — wizard, inherited rules, saved suggestions, empty/failed/stale states and layout checks.

**Inputs / dependencies**
- Tasks 5 and 6.

**Implementation rules**
- Presets and custom text remain user-editable; existing AGENTS.md content is preserved until explicitly edited. Register existing files by reading them rather than duplicating instruction storage.
- Wizard save produces a manual shared proposal and diff; it never bypasses Review/Apply. Show effective inherited instructions and their origins.
- Feature suggestions are labeled advisory, use project goals and offer Discuss, Save and Dismiss. Discuss prepopulates scope and acceptance criteria without starting generation.

**Verification command**
`make web-test`

## Task 8 — [ ] Verify applied changes and complete documentation/validation

**Target files**
- `internal/app/change_verify.go` — explicitly requested focused checks after Apply, with persisted verified/failed/unavailable states.
- `internal/app/change_verify_test.go` — fresh post-Apply identity, trust, cancellation and failed verification.
- `internal/api/handlers/change_handler.go` — verification endpoint.
- `internal/api/handlers/change_handler_test.go` — verification request guards.
- `cmd/daemon/main.go` — verification route.
- `desktop/webapp/internal/bridge/client.go` — verification allowlist.
- `desktop/webapp/frontend/src/workspace.ts` — post-Apply verification action and evidence.
- `desktop/webapp/frontend/src/change-workspace.tsx` — distinguish Applied from Verified; expose focused reanalysis.
- `desktop/webapp/frontend/src/models.ts` — verification contract.
- `desktop/webapp/frontend/tests/fixture.mjs` — verification fixture.
- `desktop/webapp/frontend/tests/run.mjs` — verification/reanalysis behavior.
- `README.md` — current product workflow and limits.
- `desktop/webapp/README.md` — chat, features, instructions, history and recovery use.
- `docs/api-contract.md` — verification and final compatibility/limits.
- `docs/openapi.yaml` — verification path/schema.

**Inputs / dependencies**
- Tasks 1–7.

**Implementation rules**
- Applied is not Verified. Verification uses fresh source hashes and separately authorized project checks; optional source reanalysis requires its own provider consent and cannot certify a model hypothesis as resolved.
- Keep legacy benchmark comparison accessible and label unmeasured optimization claims. Do not automatically mark original findings fixed solely because a write succeeded.
- Document implemented behavior and supported languages honestly. Run all applicable gates and report unavailable gates with exact reasons; do not change thresholds or unrelated tooling to pass.
- Review the full diff and commit each completed implementation task separately as explicitly requested.

**Verification command**
`./scripts/validate.sh && make web-build && git diff --check`
