# Mini-Orca Release Notes

## v4.4.0 — Reduced loopback API contract (2026-09-03)

- Added strict fixed `analyze`, `bug`, and `function` model profiles. Each
  profile is required, accepts an OpenAI Chat Completions-compatible API base,
  model, and optional local key; legacy `llm` and agent fallbacks are retired.
- Added safe scope metadata and per-scope remote confirmation across the daemon,
  API, cache provenance, and Desktop. Provider keys are never returned or logged.
- Added optional per-scope `reasoning_effort` forwarding for Chat Completions
  providers, with validation, safe Desktop/API metadata, and cache invalidation.
- AI file findings can now supply one bounded, exact task specification. A
  reviewed optional Go test runs only in the temporary check workspace and must
  fail on the base then pass on the candidate.
- Function proposals now receive declaration-focused, token-capped context. A
  failed task check can start at most three explicit **Revise with check output**
  messages in the existing session; there is no automatic repair loop.
- Native provider SDKs, credential vaults, provider account UI, automatic source
  or test writes, commits, and pushes remain intentionally out of scope.
- Removed unconsumed activity, audit-history, current-project, transient
  draft/session read, system, analysis-delete, and compatibility endpoints. The
  model catalog now contains scoped profiles only; every JSON request uses the
  same size-limited, strict one-object decoder and revision guard.
- Consolidated retained metadata writes behind one private, synced atomic
  replacement path. The canonical project report is now only
  `.mini-orca/project-analysis.json`; legacy `.mini-orca/analysis.md` and
  activity files are no longer generated. Existing legacy files are never
  deleted automatically and can be removed manually.
- Consolidated maintained documentation around one human API guide and its
  OpenAPI schema, replaced the detailed pre-cleanup task archive with a concise
  historical ledger, and removed retired UI mock material. The runtime Docker
  image now contains only the daemon binary and required runtime packages.

Manual provider and GUI acceptance remains required before a release operator
claims compatibility with a personal OpenAI, Claude-compatible,
Gemini-compatible, Ollama, LM Studio, or custom gateway configuration.

## v4.3.0 — Focused file-scoped declaration drafts (2026-08-26)

Mini-Orca now presents a single local Desktop workflow across Summary, Analysis,
Bugs, and Editor: one active project, one open file, one selected or new Go
declaration, one editable draft, validation, focused checks, explicit Apply, and
immediate guarded Undo.

### What changed

- File-scoped chat sessions are pinned to project revision, open file, base hash,
  edit mode, and declaration target. Follow-up messages cannot retarget them.
- The model returns an isolated declaration and optional imports. The daemon
  composes the complete candidate in memory; source and diff views stay
  read-only.
- Manual declaration/import edits create a new draft revision and clear earlier
  validation and check evidence. Apply requires the reviewed id, revision, hash,
  project/file guards, and a named explicit confirmation.
- Summary exposes deterministic metrics and structured analysis; Analysis
  controls explicit bounded Analyze-all; Bugs separates verified parser/vet/test
  findings from AI suggestions with provenance and freshness.
- The API, OpenAPI contract, configuration guidance, and desktop smoke checklist
  now describe this workflow and its keyboard/responsive behavior.

### Privacy and limits

- The daemon listens on loopback by default. Remote LLM endpoints require an
  explicit per-request confirmation before prompt content is sent.
- config.yaml is local and ignored by Git; use config.example.yaml as the
  tracked template. Credentials are never returned in API responses.
- Mini-Orca does not create multi-file edits, edit source directly, scan
  automatically, write drafts automatically, commit, or push.
- Exact declaration editing, composition, and required validators are Go-first.
  Other languages may be analyzed conservatively but do not receive equivalent
  declaration editing until dedicated validators exist.

### Release acceptance evidence

- On 2026-08-27, `make check` passed: Go formatting, unit tests, race tests,
  vet, and the desktop suite. The named desktop integration, accessibility, and
  draft-review workflow tests also passed.
- Route/OpenAPI/API-contract and canonical-version checks passed, as did the
  release-fixture tests for replace/create targets, secret exclusions, parser,
  vet, test, AI-suggestion provenance, stale state, and Git/non-Git coverage.
  `git diff --check` passed and `config.yaml` remains ignored and untracked.
- `docs/RELEASE_ACCEPTANCE.md` records the reproducible desktop keyboard,
  narrow-layout, remote/offline, and Git/non-Git manual checks. These require a
  release operator with a local desktop session; they do not expose prompts,
  source, or credentials in the release record.

## v4.2.0 — Desktop-only focused workflow (superseded)

The v4.2 preview workflow has been superseded by v4.3 file-scoped declaration
drafts. Consult the v4.3 migration notes and current API contract rather than
using its former one-shot generation examples.
