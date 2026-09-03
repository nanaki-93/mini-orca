# Scoped model profiles plan

Date: 2026-09-03

Status: Complete

## Outcome

Allow Mini-Orca to select an independent model and endpoint for each existing model
scope:

- `analyze`: project-wide architectural analysis;
- `bug`: selected-file semantic analysis, Analyze-all, bug discovery, and task-spec
  creation;
- `function`: one selected declaration proposal and its explicit repair revisions.

Each scope may use a local model or an online model such as GPT, Claude, or Gemini.
The choice is local configuration loaded when the daemon starts. This feature does not
add a runtime model marketplace, an autonomous agent pipeline, a new workspace, or a
new source-mutation path.

The existing product process remains authoritative:

1. Open one project and create deterministic project facts plus an optional model
   interpretation.
2. Explicitly analyze files or run Analyze-all and review Bugs findings.
3. Prepare a fix for one exact file and declaration.
4. Receive and optionally revise one isolated declaration draft.
5. Explicitly validate, run checks, review the read-only diff, and Apply.

Mini-Orca continues to make no automatic project writes, multi-file candidate edits,
commits, or pushes.

## Audited baseline

- `internal/config.Config` has one flat `llm` endpoint. `agents.coder.model` can
  override only the model name; it cannot select another endpoint or API key.
- `app.Service` creates an analysis client and coder client from the same base URL and
  credentials.
- Project import uses the analysis client, while selected-file analysis currently goes
  through the coder retry path and records the coder profile.
- Analyze-all calls the same selected-file analysis method sequentially.
- File-scoped chat already returns one declaration-only draft and the daemon composes
  the complete candidate itself.
- `.mini-orca/index.json` already stores deterministic files, imports, symbols,
  signatures, and locations. `.mini-orca/project-analysis.json` and per-file analysis
  caches already store model provenance.
- `GET /api/models/current` and Desktop state expose only one effective model and one
  remote-provider flag.
- Remote-provider confirmation exists in prompt-bearing request contracts, but it is
  currently evaluated as one global provider state. Project import also needs Desktop
  support for sending its existing confirmation field.
- Function generation currently uses the general project context builder, whose
  default budget is larger than the desired small-model function prompt.
- Candidate validation and checks already run in an isolated project copy and are
  explicit user actions.

## Product decisions

### Three fixed scopes, configured at startup

Add `model_scopes` beside the existing `llm` configuration. The fixed scope names are
`analyze`, `bug`, and `function`; arbitrary dynamic agents are not introduced.

An explicit scope profile uses an OpenAI-compatible API base URL ending at the API
prefix. The existing client payload remains Chat Completions JSON. Examples of valid
API bases include:

- OpenAI: `https://api.openai.com/v1`
- Claude compatibility: `https://api.anthropic.com/v1`
- Gemini compatibility: `https://generativelanguage.googleapis.com/v1beta/openai`
- Ollama: `http://localhost:11434/v1`
- LM Studio: `http://localhost:1234/v1`

This is intentionally a compatibility-based integration. Native Anthropic Messages,
Gemini `generateContent`, OpenAI Responses, vendor SDKs, tool calls, streaming, and
provider-specific reasoning controls are not part of this backlog. The selected model
may still use its provider-default reasoning behavior.

Reference documentation:

- [OpenAI Chat Completions](https://platform.openai.com/docs/api-reference/chat/create)
- [Claude OpenAI SDK compatibility](https://platform.claude.com/docs/en/cli-sdks-libraries/libraries/openai-sdk)
- [Gemini OpenAI compatibility](https://ai.google.dev/gemini-api/docs/openai)

### Configuration contract and fallback

Proposed configuration:

```yaml
llm:
  base_url: "http://localhost:1234"
  api_key: ""
  model: ""
  temperature: 0.7
  max_tokens: 8192

model_scopes:
  analyze:
    api_base_url: "https://api.openai.com/v1"
    api_key: "replace-in-local-config"
    model: "replace-with-provider-model-id"
    temperature: 0.1
    max_tokens: 16000
    context_max_tokens: 120000

  bug:
    api_base_url: "https://api.anthropic.com/v1"
    api_key: "replace-in-local-config"
    model: "replace-with-provider-model-id"
    temperature: 0.1
    max_tokens: 12000
    context_max_tokens: 32000

  function:
    api_base_url: "http://localhost:11434/v1"
    api_key: ""
    model: "replace-with-local-qwen-model-id"
    temperature: 0.2
    max_tokens: 4096
    context_max_tokens: 4000
```

Resolution rules:

1. When an explicit scope has `api_base_url`, that complete scope profile is used and
   `model` is required. An empty API key is valid for a local endpoint.
2. A missing `analyze` or `bug` profile inherits the complete legacy `llm` profile.
3. A missing `function` profile uses `agents.coder.model` when present and inherits
   the remaining legacy `llm` settings; otherwise it inherits `llm` completely.
4. Existing configuration without `model_scopes` behaves exactly as before.
5. Existing operation timeouts and retry configuration remain authoritative. Model
   profiles do not introduce another timeout or retry system.
6. Optional numeric fields must distinguish omission from an explicit zero where zero
   is a valid provider value.

`config.yaml` remains ignored and may contain the personal user's API keys. This plan
does not add Keychain integration, encryption at rest, a secrets database, key
rotation, account management, or a credential UI. The minimum retained safeguards are:

- never return or log API keys;
- never include keys in cache identity or provider errors;
- keep provider response bodies out of user-facing errors;
- require the existing explicit confirmation before source is sent to a non-loopback
  provider.

### Existing operations mapped to scopes

| Existing operation | Scope | Model behavior |
| --- | --- | --- |
| Project import and architectural report | `analyze` | Configured local or online model |
| Restore and deterministic reindex | none | No model call |
| Selected-file Analyze/Refresh | `bug` | Configured local or online model |
| Analyze-all worker | `bug` | Same sequential one-file process |
| Verified parser/vet/test scan | none | Existing local tools only |
| File-scoped chat proposal and revision | `function` | Configured local or online model |
| Draft validation, checks, review, Apply, Undo | none | Existing guarded process |

The daemon keeps one `Service`. It owns three resolved model runtimes and routes its
existing methods to them. Do not add an agent coordinator, queue, event bus, workflow
engine, or automatic handoff between scopes.

### API-base behavior

Add an API-base-aware client constructor that appends `/chat/completions` and, only
where supported, `/models`. Keep the current legacy constructor behavior so existing
tests and integrations using host-style `llm.base_url` remain compatible.

Model listing is informational and non-blocking. A configured model remains usable
when a provider does not expose a compatible `/models` endpoint. All production model
requests continue to carry caller cancellation and use the existing bounded response
reader and sanitized status errors.

### Model provenance and cache freshness

Every persisted interpretation records:

- scope (`analyze`, `bug`, or `function`);
- actual/configured model identity;
- sanitized provider origin (scheme, host, and optional port; no key, query, or user
  information).

Project and file analysis cache matching must include scope, model, and provider
origin. Changing any of them makes an older interpretation stale instead of silently
presenting it as current. Old cache documents without provider origin remain readable
but become stale under an explicit scoped profile.

Declaration drafts produced by a model retain the `function` model provenance through
manual draft revisions and review. Provenance is metadata only and does not weaken
project revision, file hash, draft revision, or candidate hash checks.

### Scope-aware context budgets

Keep the existing context policy, exclusions, manifest, and bounded assembly. Add
scope-specific budgets to the same context builder:

- `analyze` may use a large configured budget for repository understanding;
- `bug` keeps the selected file first and includes only bounded relevant project
  context;
- `function` is capped at a small prompt, normally 2,000–4,000 tokens.

The function context contains the task specification, selected declaration (or create
stub), package/import facts, and directly relevant signatures. It does not include
unrelated complete source files. The model continues to return one declaration, not a
patch or complete repository file.

Context manifests remain source-free and gain additive scope/model/provider metadata
so the Desktop can show what destination will receive the displayed context.

### Bug task specification

Extend the existing structured file-analysis risk contract with an optional strict
task specification:

```json
{
  "schema_version": "1",
  "target_path": "internal/example/service.go",
  "target_symbol": "Service.Run",
  "target_signature": "func (s *Service) Run(input string) error",
  "acceptance_criteria": [
    "empty input returns the documented validation error",
    "valid input behavior remains unchanged"
  ],
  "non_goals": [
    "do not change another declaration",
    "do not add a production dependency"
  ],
  "test_candidate": {
    "name": "TestServiceRunRejectsEmptyInput",
    "content": "package example\n\n..."
  }
}
```

The cloud or local `bug` model may propose the target symbol and test, but the daemon
must:

- force `target_path` to the currently analyzed file;
- resolve `target_signature` from the deterministic index;
- reject missing, ambiguous, approximate, or non-atomic targets;
- bound and sanitize every string and list;
- accept a test candidate only for Go, only for the target package, and only after it
  parses as a test file;
- never write the test candidate into the imported project.

The task specification travels with the existing AI finding and `Prepare fix` action.
Opening or filtering Bugs still does not call a model. Project-wide suggestions without
an exact target remain advisory and cannot start an edit.

### Temporary test and repair behavior

When an exact task specification has a valid test candidate, the existing explicit
draft-check action may place that test in the temporary copied workspace under a
daemon-chosen non-conflicting `_test.go` filename. It should establish that the test
fails against the captured base and passes against the candidate. Neither version is
written to the imported project.

Transport/provider retries remain the existing bounded retry behavior. Test-driven
repair is separate and remains user-triggered:

1. The user requests one function draft.
2. The user explicitly validates and runs checks.
3. If checks fail, the UI offers `Revise with check output`.
4. The existing pinned chat session sends sanitized failure output to the configured
   `function` model, with at most three repair revisions for that task.
5. A passing result returns to the existing human diff review and explicit Apply.

There is no background `generate → test → regenerate` loop. This keeps the current
preview-first process and gives the personal user control over online usage and cost.
Mini-Orca does not create a commit; the user commits outside the application.

### Effective model API and Desktop presentation

Keep `GET /api/models/current` and add a `scopes` object. Preserve its current
top-level function/coder fields for compatibility. Each public scope entry contains
only non-secret effective metadata: scope, model, provider origin, remote flag,
temperature, maximum output, context budget, timeout, and retry limit.

Desktop state keeps the three profiles separately. Analysis, Bugs, and Editor show the
effective model for their action without adding a model-selection screen. Remote
confirmation is evaluated per scope:

- `analyze` before project import;
- `bug` before selected-file analysis or Analyze-all;
- `function` before a chat proposal or repair revision.

The daemon is the final confirmation authority. Desktop must use model-provider
locality, not the daemon HTTP endpoint locality, when deciding whether confirmation is
needed.

## Delivery sequence

| Task | Outcome |
| ---: | --- |
| [95](tasks/95_scoped_model_configuration.md) | Add the backward-compatible three-scope configuration and resolver |
| [96](tasks/96_openai_compatible_api_base.md) | Support explicit OpenAI-compatible API bases for local and online providers |
| [97](tasks/97_route_model_scopes.md) | Route existing service operations and cache provenance to the correct scope |
| [98](tasks/98_scope_model_api_confirmation.md) | Expose effective scopes and enforce scope-specific remote confirmation |
| [99](tasks/99_desktop_model_scope_awareness.md) | Make Desktop model display and confirmation scope-aware |
| [100](tasks/100_bug_task_specification.md) | Produce, validate, persist, and prepare strict bug task specifications |
| [101](tasks/101_function_context_and_repair.md) | Add small function context, temporary test checks, and explicit bounded repair |
| [102](tasks/102_model_scopes_acceptance.md) | Complete integration, documentation, migration, and release acceptance |

Tasks execute in numeric order. One implementation writer owns one task at a time.
They do not authorize commits; commit only if the user separately requests it.

Use `tasks/PROMPT_EXECUTE_MODEL_SCOPES.md` to execute the complete backlog.

## Verification

Focused tests must use local `httptest` providers and temporary repositories. They
must never require a real API key, paid model call, Ollama/MLX installation, or network
access.

Required final checks:

```text
make fmt-check
go test ./...
make test-race
make vet
./desktop/gradlew -p desktop test
make check
git diff --check
```

Manual acceptance covers one all-local configuration, one mixed cloud/cloud/local
configuration, and one online `function` profile. Use personal test projects and do
not place real credentials in screenshots, fixtures, logs, or tracked files.

## Completion record

On 2026-09-03, the scoped-model acceptance suite passed using only local `httptest`
providers and temporary repositories: `make fmt-check`, `go test ./...`, `make
test-race`, `make vet`, `./desktop/gradlew -p desktop test`, `make check`, and `git
diff --check`. The all-local model-server, online-compatible provider, mixed-provider,
and interactive Desktop checks are not run because this environment has no personal
credentials, local model server, or interactive Compose session. The exact manual
procedure and limitation are recorded in `docs/RELEASE_ACCEPTANCE.md`; no provider
compatibility claim is implied by this automated result.

## Definition of done

- `analyze`, `bug`, and `function` can independently select a local or online
  OpenAI-compatible API base and model.
- Legacy `llm` and `agents.coder.model` configurations retain their behavior.
- GPT, Claude compatibility, Gemini compatibility, Ollama, and LM Studio can be
  represented without provider-specific application architecture.
- Each existing operation uses only its assigned scope, and no-model operations remain
  model-free.
- Remote confirmation, context preview, displayed model identity, and persisted
  provenance are correct for the selected scope.
- Changing a scoped model/provider makes old model interpretations stale.
- Bug output can carry one validated, exact, bounded task specification.
- Function prompts remain small and declaration-scoped.
- Generated test candidates run only in an explicit temporary check workspace.
- Failed checks can drive at most three explicit repair revisions through the existing
  chat session.
- Source/diff read-only behavior, revision/hash guards, explicit Apply/Undo, and the
  one-file mutation boundary remain intact.
- No key-management subsystem, native provider SDK, automatic repair loop, automatic
  project test write, commit, or push is introduced.
- All automated checks pass and any unavailable manual provider check is recorded
  truthfully.
