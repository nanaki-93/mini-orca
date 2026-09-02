# 84 — Complete Desktop UX refinement acceptance

## Status

Pending

## Goal

Prove and document that the simplified Desktop UI meets the approved empty-state,
information-density, semantic-color, responsive, accessibility, and preview-first
requirements.

## Depends on

Tasks 80–83.

## Required task commentary

- Before editing, post a concise user-facing commentary update beginning with
  `Starting Task 84` and name the acceptance matrix, documentation, full validation,
  and any manual GUI limitations.
- After verification, post a concise update beginning with `Task 84 complete` and state
  the complete UX outcome, all commands/results, manual checks, and any limitation before
  giving the final all-tasks response.
- During long-running work, continue posting brief progress commentary at least every
  60 seconds. These are user-facing task updates, not source-code comments.

## Implementation

- Review the finished behavior against every requirement and definition-of-done item in
  `PLAN.md`. Fix only regressions within Tasks 80–83; do not introduce a new feature
  backlog during acceptance.
- Confirm automated coverage proves:
  - exclusive no-project landing visibility;
  - Open project progress, failure/retry, and successful transition;
  - no-project shortcut gating and `Cmd/Ctrl+O`;
  - semantic action roles and compact size mappings;
  - absence of routine counts, revisions, and hashes;
  - preserved internal revision/hash/request guards;
  - reduced helper copy with retained actionable/safety feedback;
  - one connection/status presentation;
  - Bugs filter disclosure and Analyze-all compact limits;
  - responsive action grouping and Editor stage semantics;
  - wide, exactly 1000dp, and narrow Editor/project-workspace rules.
- Update `desktop/README.md` to describe:
  - the no-project landing state;
  - `Cmd/Ctrl+O`;
  - concise workspace navigation;
  - semantic action colors and compact controls only where user guidance benefits.
- Update `desktop/KEYBOARD_SMOKE_CHECKLIST.md` with a reproducible no-project shortcut
  gate and project-open keyboard flow.
- Update `docs/RELEASE_ACCEPTANCE.md` with reproducible empty/open project, wide/narrow,
  color/text-state, and preview-first evidence.
- Update other current user-facing documentation only when it would otherwise contradict
  shipped behavior. Do not rewrite historical completed task files.
- After every implementation and acceptance item passes, update `PLAN.md` from
  `Approved; implementation pending` to a truthful completed status.
- Perform a final source audit:
  - no obsolete button API, duplicate compact field, workspace count plumbing, old
    revision/hash display helper, or duplicate connection string;
  - no project-only callback can run from the landing state;
  - no generated output, local configuration, credential, API schema, or daemon change
    belongs to this backlog.
- Preserve all pre-existing unrelated worktree changes and report any validation affected
  by them honestly.

## Acceptance criteria

- Every item in `PLAN.md` section 9 has automated or reproducible manual evidence.
- With no project open, only product identity, Open project, and contextual
  progress/retry feedback are present and only Open project is actionable.
- With a project open, the compact workspace remains keyboard accessible and responsive
  without numeric navigation clutter or raw identity strings.
- Action purpose/priority is visually distinct, text-labeled, and accessible without
  relying on color alone.
- Redundant button explanations are gone while errors, blocked reasons, remote-provider
  confirmation, validation/check evidence, exact Apply target, and receipt remain clear.
- Source/diff remain read-only; the declaration/import draft remains the only editable
  generated artifact; Apply and Undo remain explicit and identity guarded.
- Documentation and smoke checks match the shipped UI.
- No API, daemon, persistence, configuration, or migration step is required.
- Tasks 80–83 are Complete before Task 84 is marked Complete.

## Verification

- Run:

  ```text
  ./desktop/gradlew -p desktop test
  make check
  git diff --check
  ```

- If `make check` cannot run because of an environment limitation or a pre-existing
  unrelated failure, run every available constituent check, capture the exact failure,
  and do not report a passing full gate.
- Manually inspect, or explicitly record as not run:
  - startup with no project;
  - chooser cancel, open progress, failed open/retry, and successful open;
  - Summary, Analysis, Bugs, Target, Draft, Verify, and Apply;
  - wide, exactly 1000dp, and narrow window sizes;
  - keyboard-only navigation and focus;
  - long labels, wrapped action groups, compact inputs, hover/pressed/disabled states;
  - color contrast plus readable textual state;
  - read-only source/diff and explicit guarded Apply.
- Inspect `git status --short`, the complete diff, and task/index links. Confirm no
  unrelated change was overwritten.

## Completion

After every criterion passes, set this task to Complete, move it to
`tasks/completed/`, update its link/status in `tasks/INDEX.md`, and update the plan
status. Do not create a commit; this task definition does not authorize commits.
