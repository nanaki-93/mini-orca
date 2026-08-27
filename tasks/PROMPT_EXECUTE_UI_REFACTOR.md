# Prompt — execute the Desktop UI refactor sequentially

Use this prompt with one primary Codex/Air implementation agent. The agent owns
the shared worktree and executes Tasks 57–74 one at a time in dependency order.

```text
You are the primary implementation agent for the Mini-Orca Desktop UI refactor.

Workspace objective:
Complete every Pending task from Task 57 through Task 74 sequentially. Deliver
the approved hybrid UI: Direction A's persistent Workbench shell, Direction C's
Focus Flow palette and Target → Draft → Verify → Apply sequence, and Direction B's
calm final Apply language.

Read completely before editing, in this order:
1. AGENTS.md
2. PLAN.md
3. design/ui-mocks/README.md
4. design/ui-mocks/IMPLEMENTATION_PLAN.md
5. tasks/README.md
6. tasks/INDEX.md
7. tasks/57_editor_flow_presentation_contract.md through
   tasks/74_ui_refactor_integration_acceptance.md in numeric order
8. The current Desktop production files and tests implicated by Task 57

Treat these mock sources as visual references, not runtime assets:
- design/ui-mocks/direction-a-workbench.html for shell density and hierarchy
- design/ui-mocks/direction-c-focus-flow.html for colors, stages, and diff review
- design/ui-mocks/direction-b-calm-review.html for final Apply language/hierarchy

Do not embed the HTML/PNG mocks in the application. Implement the design in
Kotlin/Compose Desktop with code-native theme tokens, components, and vectors.

Non-negotiable product boundaries:
- One active project, one open file, and one selected or new declaration.
- Source and composed diff remain selectable and read-only.
- Only the isolated declaration/import draft is editable.
- Sending, validation, checks, scans, Apply, and Undo remain explicit user actions.
- Apply remains revision/hash guarded and changes only the exact named file.
- No automatic writes, multi-file candidates, commits, pushes, scans, tests, or fixes.
- Remote-provider confirmation remains mandatory before prompt-bearing requests.
- Advisory impact and Git information remain read-only.
- State must be understandable with text and semantics, never color alone.
- Preserve keyboard operation and the below-1000dp drawer behavior.

Implementation boundaries:
- This is a Compose Desktop refactor. Do not change daemon routes or external API
  contracts unless a task explicitly requires a compatibility fix; no current task does.
- Do not add a framework, database, second UI, runtime font download, image-generation
  dependency, or speculative abstraction.
- Do not retain old and new implementations together. When a task replaces a surface,
  delete its superseded composables, branches, state, callbacks, tests, and documentation
  in that same task.
- Keep API/coroutine orchestration in MiniOrcaApp and presentation derivation pure where
  practical. Do not mirror project/file/draft/check truth in a second state machine.
- Use the exact semantic palette in the implementation plan. Prefer accessible clarity
  over pixel-perfect imitation when the two conflict.
- Preserve unrelated user changes and never edit generated build output, local config,
  credentials, desktop/build, desktop/.gradle, or desktop/.kotlin.
- Use rg/rg --files for discovery, apply_patch for manual edits, gofmt for Go if any Go
  file is legitimately touched, and the repository Gradle wrapper for Desktop checks.
- Do not create commits unless the user explicitly asks.
- Work as the single implementation writer. Do not start parallel implementation agents
  or allow overlapping writers in the shared worktree.

Starting procedure:
1. Inspect `git status --short` and record all pre-existing changes. The approved design
   mock/plan files may already be modified or staged; preserve them.
2. Read Task 57 and verify Task 56 is Complete in tasks/INDEX.md.
3. Inspect current code/tests before deciding exact file edits. Do not ask the user for
   facts available in the repository.
4. Share a concise task-boundary update: task ID, outcome, likely files, focused checks.
5. Change only Task 57's status and index row to In Progress when implementation starts.

Sequential execution loop for each task N from 57 through 74:
1. Dependency gate
   - Read the complete task file.
   - Verify every dependency is Complete and its acceptance behavior still exists.
   - If a dependency is incomplete or regressed, fix it only when that work is necessary
     to make the current dependency truthful; otherwise stop and report the exact blocker.

2. Scope and research
   - Inspect the implicated production code, tests, and approved mock references.
   - State the task boundary and focused validation before editing.
   - Implement only Task N. Do not absorb a later task merely because it is nearby.

3. Status
   - Set Task N to In Progress and update its tasks/INDEX.md status row.
   - Keep all other Pending task statuses unchanged.

4. Implementation
   - Make the smallest cohesive replacement that satisfies every implementation bullet.
   - Reuse existing workflow guards, API client, project state, Compose boundaries, and
     accessibility helpers.
   - Add/update focused regression tests with each behavior change.
   - Remove superseded code in the same task; do not add compatibility branches for the
     retired candidate UI.
   - Preserve source/diff read-only behavior and explicit mutation gates throughout.

5. Focused verification
   - Run the task's listed focused checks as soon as the relevant code compiles.
   - For every Desktop task, run `./desktop/gradlew -p desktop test`.
   - Run any additional `make check` gate explicitly listed by the task.
   - Always run `git diff --check` and inspect the complete task diff.
   - Do not weaken, skip, delete, or rewrite a valid test only to obtain green output.

6. Acceptance review
   - Check every acceptance criterion one by one against code and test evidence.
   - Check that no later-task behavior was partially introduced and no replaced code remains.
   - Check the worktree for unrelated modifications and generated output.

7. Complete the task
   - Only after all criteria and checks pass, change the task status to Complete.
   - Move the task file to tasks/completed/ without changing its number/name.
   - Update tasks/INDEX.md to link to tasks/completed/ and set the row to Complete.
   - Report a concise boundary update: behavior, files, checks/results, and any deliberate
     deferral to a named later task.

8. Continue automatically
   - Select Task N+1 immediately and repeat this loop.
   - Do not ask the user to say “continue” between tasks.
   - Do not stop merely because the refactor is large or a task required several iterations.

Blocking policy:
- Investigate failures and exhaust safe task-scoped fixes before declaring a blocker.
- Stop only for a real missing decision that would materially change the approved design,
  missing authority for an external/destructive action, an unavailable required dependency,
  or a failing baseline that cannot safely be resolved in scope.
- If blocked, leave the current task In Progress (or Pending if implementation never began),
  leave dependent tasks Pending, and report the exact evidence and required user decision.
- Never skip a blocked task, fabricate completion, or implement dependent work around it.

Communication during execution:
- Provide short progress updates at task boundaries and during any operation lasting more
  than about a minute.
- Lead each update with the current outcome/state, not a tool transcript.
- Do not flood the user with routine command output; report meaningful failures and gates.

Visual and interaction acceptance throughout:
- Wide shell: top bar + labeled workspace rail + explorer + canvas + contextual panel.
- Narrow shell: labeled Files/Context drawers below 1000dp; no squeezed four-pane layout.
- Editor: Target → Draft → Verify → Apply is visible, gated, reversible to earlier unlocked
  stages, and clamped backward when evidence becomes stale.
- Apply: “Nothing has changed yet,” exact target and proof, one explicit target-naming action,
  then an applied receipt/guarded Undo.
- Palette, dialogs, statuses, Summary, Analysis, and Bugs use the same semantic visual system.
- Critical status, freshness, provenance, confidence, validation, and selected/disabled state
  remain textual and accessible.

Final acceptance after Task 74:
1. Confirm Tasks 57–74 are Complete, moved under tasks/completed/, and truthfully linked.
2. Run:
   - `./desktop/gradlew -p desktop test`
   - `make check`
   - `git diff --check`
3. Execute/document the updated keyboard and responsive smoke checklist at wide layout,
   exactly 1000dp, and below 1000dp when the environment permits.
4. Verify a complete safe flow: select/import project → Editor Target → file/symbol → Draft
   chat → manual draft edit → Validate → Verify/checks → Apply decision → Apply → Undo.
5. Verify dirty, invalid, failed-check, stale revision, remote provider, disconnected, long
   path, and non-Git states.
6. Inspect the final diff for generated output, credentials/config, dead candidate UI, duplicate
   surfaces, automatic mutations, and unrelated changes.
7. Do not commit.

Final response:
- Lead with whether Tasks 57–74 and the selected UI were completed.
- Summarize behavior by foundation, shell, Editor stages, remaining workspaces, and accessibility.
- List tests/checks run and their results, plus any checks not run and why.
- State that no configuration/data migration is required, unless implementation proved otherwise.
- Link the implementation plan, task index, and key changed files using the environment's required
  file-link format.
```

