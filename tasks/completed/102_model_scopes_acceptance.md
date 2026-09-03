# 102 — Complete scoped-model acceptance

## Status

Complete

## Goal

Verify the scoped-model feature end to end, finish public documentation and migration
guidance, and prove that the existing preview-first architecture remains intact.

## Depends on

Tasks 95–101.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 102` and name the full
  integration matrix, documentation, full validation, and manual provider checks.
- After verification, post a separate update beginning with `Task 102 complete` and
  state the shipped outcome, commands/results, manual limitations, changed files, and
  that the sequence is finished.

## Implementation

- Review the implementation against every `PLAN.md` decision and definition-of-done
  item. Fix only gaps belonging to Tasks 95–101.
- Add an end-to-end local integration harness with three independent `httptest`
  providers proving Analyze → Bug task → Function draft routing and no cross-scope
  calls.
- Prove all-local, mixed cloud/cloud/local metadata, and all-remote confirmation
  behavior without making public-network requests.
- Verify deterministic reindex, verified scans, draft validation/checks, Apply, and Undo
  make no model request.
- Update:
  - `README.md` and `desktop/README.md`;
  - `CONFIG.md` and `config.example.yaml`;
  - `API.md`, `docs/api-contract.md`, and `docs/openapi.yaml`;
  - `docs/RELEASE_ACCEPTANCE.md` and release notes where required.
- Document API-base examples for OpenAI, Claude compatibility, Gemini compatibility,
  Ollama, LM Studio, and a custom compatible gateway. Use placeholder model IDs and
  keys rather than claims about a permanently current model name.
- Document the personal-tool credential policy: ignored local config is supported;
  keys are not logged or returned; no vault, encryption, Keychain, rotation, or account
  UI is provided.
- Document legacy fallback, cache-staleness behavior, daemon restart after config
  changes, provider compatibility limitations, explicit remote confirmation, temporary
  tests, repair limit, and external manual commits.
- Remove obsolete single-provider helpers, tests, and documentation made redundant by
  the final implementation. Do not retain parallel old and new routing code.
- Set `PLAN.md` to Complete only after automated criteria pass and manual checks are
  completed or their exact limitation is recorded.

## Acceptance criteria

- Every definition-of-done item in `PLAN.md` has automated or reproducible manual
  evidence.
- Legacy, all-local, mixed, and online-compatible configurations are documented and
  covered without real paid calls in the automated suite.
- Model/provenance/confirmation presentation agrees across daemon, API, cache, and
  Desktop.
- Bug task specs and optional tests cannot escape their exact selected target or
  temporary workspace.
- Function repair stays explicit, bounded to three actions, and returns to human diff
  review.
- No native provider SDK, credential subsystem, runtime model marketplace, automatic
  repair loop, automatic source/test write, commit, or push exists.
- No legacy scoped-routing implementation remains alongside its replacement.

## Verification

- Run:

  ```text
  make fmt-check
  go test ./...
  make test-race
  make vet
  ./desktop/gradlew -p desktop test
  make check
  git diff --check
  ```

- Manually test, where personal credentials and providers are available:
  - one local profile through LM Studio or Ollama;
  - one online compatibility profile from OpenAI, Claude, or Gemini;
  - one mixed `analyze`/`bug` online plus `function` local configuration;
  - remote confirmation, context preview, one bug task, one failed temporary test
    repair, review, and explicit Apply.
- Never put a key in a screenshot, log excerpt, task file, fixture, or tracked config.
- If a manual provider is unavailable, record it as not run; do not block deterministic
  automated acceptance or claim it passed.

## Completion

Only after all criteria pass, set this task and its index row to Complete, move it to
`tasks/completed/`, and set `PLAN.md` to Complete with truthful manual-test status. Do
not stage or commit unless the user separately asks.
