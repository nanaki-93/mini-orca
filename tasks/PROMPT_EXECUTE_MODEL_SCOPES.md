# Prompt — execute Tasks 95–102 sequentially

Use this prompt with one primary Codex implementation agent for the scoped-model
profiles backlog.

```text
You are the sole implementation agent for Mini-Orca Tasks 95–102.

Objective:
Implement configurable analyze, bug, and function model scopes from PLAN.md. Execute
Tasks 95 through 102 strictly in numeric order. Finish and verify each task before
starting the next. Do not ask the user to say “continue” between tasks. Do not stage,
commit, or push unless the user separately requests it.

Read completely before acting, in this order:
1. AGENTS.md
2. PLAN.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/95_scoped_model_configuration.md
6. tasks/96_openai_compatible_api_base.md
7. tasks/97_route_model_scopes.md
8. tasks/98_scope_model_api_confirmation.md
9. tasks/99_desktop_model_scope_awareness.md
10. tasks/100_bug_task_specification.md
11. tasks/101_function_context_and_repair.md
12. tasks/102_model_scopes_acceptance.md
13. Current production code, tests, docs, and completed dependency tasks implicated by
    Task 95

Required outcome:
- Analyze, bug, and function independently select a configured local or online
  OpenAI-compatible API base and model.
- GPT, Claude compatibility, Gemini compatibility, Ollama, LM Studio, and compatible
  gateways fit the same small client contract.
- Existing llm and agents.coder.model configurations retain their behavior.
- Project import uses analyze; selected-file analysis and Analyze-all use bug; file
  chat and declaration revisions use function.
- The effective-model API, cache provenance, context preview, Desktop labels, and
  remote confirmation agree on the active scope.
- A bug finding can carry one exact validated task spec and optional temporary Go test.
- Function prompts are small, declaration-focused, and bounded by the configured
  context budget.
- Check-driven repair is explicit, uses the existing pinned chat, and stops after three
  repair revisions.
- The existing human validation/check/diff/Apply process and one-file write boundary
  remain unchanged.

Non-negotiable boundaries:
- Preserve one active project, one open file, one selected/new declaration, one
  editable draft, and explicit Apply/Undo.
- Keep source and composed diff selectable and read-only.
- No model call on restore, reindex, browsing, filtering, navigation, verified scans,
  validation, checks, review, Apply, or Undo. The explicit repair action is the only
  check-related action allowed to start a model call.
- Do not add an autonomous pipeline, native provider SDK, model marketplace, provider
  account UI, credential database, Keychain, encryption, key rotation, background
  repair, automatic source/test write, commit, or push.
- API keys may be read from ignored personal config. Never log, return, persist in
  model metadata, place in errors, or add to tracked examples/tests.
- Keep provider prompts inside the existing context policy and manifest boundaries.
- Keep project revision, file hash, session target, draft revision/hash, candidate
  validation, and asynchronous-response guards.
- Do not edit generated output, local config.yaml, desktop/build, desktop/.gradle, or
  desktop/.kotlin.
- Use one implementation writer. Do not create another Codex task or delegate writes.

Starting-worktree and dependency gate:
1. Run git status --short, git diff --name-only, and git diff --cached --name-only.
2. Require Task 94 to be Complete. Record every pre-existing modification or untracked
   file as user-owned, including the plan/task documents if supplied by the user.
3. Do not require a clean worktree, but stop if a required edit overlaps a user-owned
   hunk and cannot be changed without overwriting it.
4. Never use reset, checkout, restore, stash, clean, rebase, or destructive history
   operations.
5. Never stage or commit during this sequence unless a later explicit user request
   grants that authority.

Mandatory task commentary:
- Before Task N, post one update beginning exactly:
  “Starting Task N — <task title>: ...”
- After Task N passes verification, post a separate update beginning exactly:
  “Task N complete — ...”
- At start, name scope, likely files, focused verification, and pre-existing changes.
- At completion, name behavior, commands/results, changed files, and next task.
- Post concise progress updates at least every 60 seconds during long work.

Sequential loop for every Task N from 95 through 102:

1. Dependency and boundary check
   - Read Task N completely again.
   - Verify every dependency is Complete and its behavior still exists.
   - Inspect current code, tests, documentation, and relevant diffs.
   - Confirm Task N is the first ready incomplete task.

2. Start
   - Post the mandatory Starting Task N commentary.
   - Mark only Task N and its tasks/INDEX.md row In Progress when implementation begins.

3. Implement only Task N
   - Make the smallest cohesive change satisfying its implementation and acceptance
     requirements.
   - Follow Clean Code, KISS, existing package boundaries, and AGENTS.md.
   - Replace obsolete implementation when the task changes a function; do not retain
     parallel old and new paths.
   - Add focused tests with each behavior change.
   - Do not absorb a later task because it touches nearby code.

4. Verify
   - Run every command in Task N.
   - Run git diff --check and inspect the complete task-owned diff.
   - Verify each acceptance criterion explicitly. Never weaken a valid safety or
     regression assertion to make a test pass.
   - Automated provider tests use httptest only. Do not make paid/public model calls.

5. Complete metadata
   - Only after all criteria pass, set Task N to Complete, move it to tasks/completed/
     without renaming it, and update tasks/INDEX.md.
   - For Task 102, also finish documentation and set PLAN.md to Complete with truthful
     manual-provider status.
   - Do not stage or commit. Confirm unrelated user changes remain unaltered.

6. Report and continue
   - Post the mandatory Task N complete commentary.
   - Continue immediately to the next task without waiting for confirmation.

Task ownership:
- Task 95 owns scope configuration, resolution, validation, defaults, and examples.
- Task 96 owns API-base URL joining and OpenAI-compatible HTTP behavior.
- Task 97 owns Service routing, execution reuse, provenance, and cache freshness.
- Task 98 owns effective-model API shape and daemon confirmation enforcement.
- Task 99 owns Desktop model state, labels, confirmation, and import request support.
- Task 100 owns strict bug task specs, optional test parsing, findings, and Prepare fix.
- Task 101 owns small function context, task pinning, temporary test checks, and explicit
  repair revisions.
- Task 102 owns end-to-end acceptance, final cleanup, public docs, and full validation.

Blocking policy:
- Investigate failures and exhaust safe task-scoped fixes before reporting a blocker.
- Stop for an incomplete dependency, inseparable user-owned overlap, missing material
  product decision, unavailable required local dependency, or a required check that
  cannot safely be fixed in scope.
- Leave a started blocked task In Progress; otherwise leave it Pending.
- Never skip a blocked task, mark partial work Complete, fabricate provider results,
  or continue into a dependent task.

Final acceptance:
1. Confirm Tasks 95–102 are Complete and linked under tasks/completed/.
2. Confirm the complete validation matrix from Task 102 ran, or report exact limits.
3. Confirm automated tests made no public model calls and no tracked key exists.
4. Confirm the preview-first workflow, one-file Apply boundary, and explicit repair
   behavior remain intact.
5. Confirm no files are staged and no commit or push was created.

Final response:
- Lead with whether all eight tasks completed.
- List Tasks 95–102 with status and one-line outcome.
- Summarize automated and manual validation, including anything not run.
- Confirm provider compatibility boundaries, personal-key handling, workflow safety,
  preserved unrelated changes, and that no commit was created.
- Link PLAN.md, tasks/INDEX.md, and key changed files using absolute paths.
```
