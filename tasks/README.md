# Mini-Orca task workflow

This directory contains the active backlog for
[UI refinement](../desktop/UI_REFINEMENT_PLAN.md), the prior
[dark desktop redesign](../docs/dark-ui/PLAN.md) with outstanding Task 149 acceptance,
and completed records for
[engineering insights and project performance analysis](../docs/insights-performance/PLAN.md),
the [IDE UI plan](../plan.md), and earlier work.

## Layout

- Pending tasks live in this directory as `NN_snake_case.md`.
- Completed tasks move to `completed/` without changing their identifier.
- [`INDEX.md`](INDEX.md) is the authoritative dependency order and status list.
- [`PROMPT_EXECUTE_UI_REFINEMENT.md`](PROMPT_EXECUTE_UI_REFINEMENT.md) executes
  Tasks 150–160 in strict order, with one verified local commit per task, when
  explicitly requested. Writing or reviewing these documents does not execute them.
- [`PROMPT_EXECUTE_DARK_UI.md`](PROMPT_EXECUTE_DARK_UI.md) describes implementation
  of pending Tasks 140–149 when requested. It does not authorize commits.
- [`PROMPT_EXECUTE_INSIGHTS_PERFORMANCE.md`](PROMPT_EXECUTE_INSIGHTS_PERFORMANCE.md)
  is the completed execution record for Tasks 133–139, with one verified local commit per task.
- [`PROMPT_EXECUTE_IDE_UI.md`](PROMPT_EXECUTE_IDE_UI.md) is the completed execution record for
  Tasks 118–132. It required strict sequencing, one implementation writer, and one verified
  commit per task.
- [`PROMPT_EXECUTE_LEGACY_CLEANUP.md`](PROMPT_EXECUTE_LEGACY_CLEANUP.md) and completed
  Tasks 103–117 remain as the audit record for the previous cleanup sequence.
- [`INDEX.md`](INDEX.md) contains the concise historical ledger for Tasks 01–102. Git
  history is the full archive.

## Planned UI refinement

Tasks 150–160 implement `desktop/UI_REFINEMENT_PLAN.md`: remove duplicate Project
navigation, unify IDE tokens, correct Analysis width, replace inconsistent menus
and disclosures, rebuild Summary, reduce global interface prose, and verify the
complete replacement. Preserve the real Files tree/project operations, Analysis's
accepted visual hierarchy, and all existing workflow safety boundaries.

The new execution prompt requires exactly one reviewed local commit per task after
its tests and acceptance pass. Task 150 owns the explicitly listed initial planning
artifacts together with baseline/characterization work; there is no extra planning
commit. Stage only task-owned files/hunks, preserve unrelated work, and do not push.
Use its resume and failed-commit rules rather than duplicating completed work.

Task 149 is not a hard dependency of this sequence. It retains its outstanding
native/repository acceptance status and is not automatically completed by the new
backlog. Do not run an earlier prompt merely because this document links it.

## Prior dark desktop redesign

Tasks 140–149 use the two supplied images to replace the purple/navy palette with
charcoal-and-blue graphics, clearer icons and panel grouping, and visible Preview
controls for missing features. The existing Compose architecture, Performance
workspace, and preview-first Apply/Undo workflow remain in place.

The new user direction supersedes historical palette-preservation requirements and
the ban on displaying unsupported controls, but only to allow the plan's explicitly
local UI previews. It does not add backend functionality. Creating the plan/backlog
does not execute it, create Codex tasks, or authorize commits. Begin implementation
only when requested; do not run a historical prompt because it is linked here.

## Completed insights and performance sequence

Tasks 133–139 are complete after the completed IDE sequence.
They add a small closeable/reopenable Engineering insight panel to existing result
pages and an independent, explicitly unmeasured Performance review section.
There are no quizzes, games, learning profiles, runtime profilers, or new automatic
source writes. The feature plan defines the complete scope and safety contract.

That completed sequence used an explicitly invoked prompt requiring one verified
commit per numbered task. Task 133 owned its initial feature-plan/task/prompt
artifacts together with baseline tests. Those historical instructions do not apply
commit authorization to new planning or implementation requests.

## Completed IDE redesign

Tasks 118–132 completed the IDE-style Compose Desktop redesign while preserving the current
Focus Flow palette and Mini-Orca's one-project, one-file, one-symbol, preview-first workflow.
The completed task records and [`INDEX.md`](INDEX.md) provide the delivery ledger.

Task 118 established the plan, task files, index/workflow updates, execution prompt, and
characterization baseline. Tasks 119–132 then implemented and verified each dependency-ordered
slice without rewriting the preview-first safety contract.

## Status lifecycle

Each task has exactly one status: `Pending`, `In Progress`, or `Complete`.

1. Select the first Pending task in the explicitly requested sequence whose
   dependencies are Complete; do not select an unrelated earlier Pending task.
2. One implementation agent owns that task and its writes until verification finishes.
3. Follow the selected prompt's agent policy. Tasks 140–149 and 150–160 use one
   implementation agent without delegation or parallel task implementation.
4. Run the task's focused checks and `git diff --check`.
5. Only after every acceptance criterion passes, prepare Complete status, move the
   task under `completed/`, and update `INDEX.md`. In a commit-per-task sequence,
   completion is confirmed only when the isolated task commit succeeds.
6. Inspect the full task-owned diff. Preserve unrelated user work.
7. Do not stage or commit unless the user explicitly requests commits. Executing
   `PROMPT_EXECUTE_UI_REFINEMENT.md` includes the user's requested one commit per
   task. A historical per-task commit rule applies only to its invoked sequence.
8. Post completion commentary and continue to the next ready authorized task without
   waiting for a separate “continue” message.

Do not skip a blocked dependency, duplicate its intended behavior in a later task, or
mark a task Complete based on partial implementation.

## Shared implementation rules

- Read `AGENTS.md`, `desktop/UI_DESIGN_GUIDELINES.md` for UI work, the active
  feature plan, this file, `INDEX.md`, the selected
  execution prompt, and the selected task before implementation. Completed plans and
  prompts supply historical context, not automatic instructions for new work.
- Preserve the one-project, one-open-file, one-symbol, preview-first workflow.
- Keep source and diff selectable/read-only; only the isolated declaration/import
  draft is editable.
- Generation, analysis, selection, navigation, validation, and checks never write
  source. Apply and Undo remain the only explicit guarded source mutations.
- Keep remote-provider destination and confirmation visible in text before sending
  project context.
- Preserve keyboard navigation, state labels, and the exact `1000dp` responsive
  boundary. Tasks 140–149 intentionally replace the old palette with the active plan's
  charcoal theme; palette preservation applies only to the completed sequences.
- Fit changes into the existing presenter and feature-state boundaries. Do not
  add a generic docking engine, event bus, or duplicate workflow state. A compatible
  Jewel component adoption is evaluated only within Task 154's explicit boundary;
  it does not authorize a broad framework/toolchain migration.
- Do not add general source editing, multi-file tabs/changes, terminal, run/debug,
  filesystem mutation, automatic fixes, automatic commits, or VCS write features.
  Tasks 140–149 may display the active plan's Preview controls for missing features,
  without implementing those capabilities or connecting the previews to backend work.
- Add deterministic behavior-focused tests with every changed interaction or state.
- Remove obsolete UI branches, helpers, call sites, and tests when their replacement
  becomes authoritative; do not preserve parallel shells.
- Preserve unrelated user changes and never edit generated build output.
- Post the required user-facing commentary before and after every task, with concise
  progress updates during work lasting more than 60 seconds.
- The task commentary requirements are conversation updates, not instructions to add
  source-code comments.
- Commit authorization comes from the user's explicit request, including execution
  of the requested UI refinement prompt or an expressly invoked historical prompt
  that requires commits. The older dark UI prompt does not request commits. No
  sequence authorizes pushes or unrelated history changes.

## Verification baseline

- Desktop formatting/static/tests: `./desktop/gradlew -p desktop spotlessCheck detekt test`
- Desktop tests: `./desktop/gradlew -p desktop test`
- Go formatting: `make fmt-check`
- Go tests: `go test ./...`
- Race tests where required: `make test-race`
- Static analysis: `make vet`
- Full release gate: `make check`
- Project quality gate: `make quality`
- Every task: `git diff --check`

The final report must state each task's status in the executed sequence, behavior
changed, files changed, acceptance criteria verified, commands run, configuration or
migration needs, and any blocker or unavailable manual check. Report commit hashes
only when commits were explicitly requested and actually created.
