# Analysis summary and Editor-only explorer plan

Date: 2026-09-02

Status: Approved; implementation pending

Task source: [`tasks/INDEX.md`](tasks/INDEX.md)

Execution prompt: [`tasks/PROMPT_EXECUTE_ALL_TASKS.md`](tasks/PROMPT_EXECUTE_ALL_TASKS.md)

## 1. Outcome

Improve the Desktop UX so each workspace has one clear responsibility:

- **Summary** presents project facts and the structured project interpretation.
- **Analysis** presents Analyze-all coverage, run progress, controls, and failures.
- **Bugs** presents verified findings and AI suggestions.
- **Editor** owns file browsing, per-file analysis, source inspection, and the guarded
  Target → Draft → Verify → Apply workflow.

The Analysis workspace must no longer require the user to inspect a card for every
successfully analyzed file. The file explorer and Editor context must no longer consume
space in project-level workspaces.

## 2. Approved decisions

### 2.1 Analysis is an operational summary

Interpret “analysis summary” as a summary of Analyze-all execution and project coverage.
The existing API already exposes the required source-free data:

- project coverage: total, fresh, stale, missing, running, and failed;
- job state and configured file/retry limits;
- per-job-file status, attempts, and sanitized error text.

Analysis will display:

1. **Project coverage** — the current revision’s coverage counts.
2. **Current or last run** — status, total candidates, completed, failed, running,
   remaining, and configured limits.
3. **Analyze-all controls** — Start, Pause, Resume, Cancel, and remote-provider
   confirmation with the existing guards.
4. **Analysis errors** — only files whose job status is failed or whose error is nonblank,
   showing relative path, attempt count, and error text.

Successful and pending files will not be rendered as individual cards. When there are no
failures, Analysis will show an explicit “No analysis errors in this run” state.

This plan does not synthesize a new semantic report from all file-analysis contents. That
would require a separate backend aggregation feature and different privacy/token decisions.

### 2.2 The explorer belongs to Editor

Choose the Editor-only layout:

- On wide windows, Summary, Analysis, and Bugs show the workspace rail plus a full-width
  workspace canvas.
- On wide windows, Editor shows the workspace rail, file explorer, Editor canvas, and
  contextual panel.
- Below 1000dp, Files and Context drawer actions appear only in Editor.
- Leaving Editor preserves the selected file and Editor stage; returning restores them.
- Selecting a file from the explorer or global file palette always activates Editor and
  opens that file.
- `Cmd/Ctrl+P` remains available from every workspace as the global route to a file.

The contextual panel is hidden together with the explorer outside Editor because it is
Editor-specific and currently shows only an empty Editor-context state there.

## 3. Current defects

- `AnalysisWorkspacePane` renders every Analyze-all file and gives each one a
  “View file analysis” action.
- The wide shell always reserves space for both the explorer and context panel, including
  in project-level workspaces.
- Narrow layouts always expose Files and Context drawer actions regardless of workspace.
- Explorer selection loads a file but does not itself enforce the Editor destination,
  while command-palette and finding routes implement that transition separately.

## 4. Implementation design

### 4.1 Pure Analysis presentation

Extend `AnalysisWorkspaceState.kt` with one pure presentation model derived from
`AnalyzeAllJob` and `AnalysisCoverage`. It must:

- keep coverage counts distinct from current/last-job counts;
- recognize the daemon job-file states `pending`, `running`, `completed`, and `failed`;
- treat a nonblank error as a failure even if an unknown status is received;
- calculate remaining work without negative values;
- retain file order for deterministic failure presentation;
- provide safe empty states for no project/job and no failures.

The Compose layer consumes this model and must not duplicate status classification.

### 4.2 Analysis workspace replacement

Replace the existing file-results list with compact summary panels and a failures-only
lazy list. Remove the Analysis-to-file callback chain and the dedicated
`openFileAnalysis` route made obsolete by the removed buttons.

Do not remove per-file analysis from Editor. Selecting a file in Editor and explicitly
running or refreshing its analysis must continue to use `EditorSurface.FileAnalysis`.

### 4.3 Workspace-aware shell

Derive one explicit `editorChromeVisible` decision from the active workspace. Use it for:

- wide explorer and its resize divider;
- wide context panel and its resize divider;
- narrow Files and Context top-bar actions;
- narrow drawer content and closure when leaving Editor.

Do not copy the workspace condition into unrelated composables. Preserve the 1000dp
breakpoint, saved pane widths, keyboard workspace navigation, and read-only views.

### 4.4 Consistent file navigation

Use one app-level file-opening action for routes whose intent is to inspect a file. It must
select `Workspace.Editor` before starting the existing guarded asynchronous file load.
Reuse it for explorer and file-palette selection. Finding navigation may retain its symbol
and line target while following the same workspace-first rule.

Do not change project revision/file hash guards, cancelation, or optional analysis/impact/
Git-status loading.

## 5. Delivery tasks and commits

| Task | Outcome | Required commit |
|---:|---|---|
| 75 | Record the approved plan, task queue, and safe commit workflow | `docs(tasks): define analysis workspace UX backlog` |
| 76 | Add the pure Analyze-all summary/failure presentation model and tests | `feat(desktop): summarize analyze-all results` |
| 77 | Replace per-file Analysis results with summary and failures only | `feat(desktop): simplify analysis workspace results` |
| 78 | Make explorer/context Editor-only and normalize file navigation | `feat(desktop): scope file navigation to editor` |
| 79 | Complete regression, responsive, and documentation acceptance | `test(desktop): verify analysis and editor workspace UX` |

Tasks execute strictly in numeric order. Each task receives exactly one commit after its
acceptance criteria pass. Task metadata changes belong in the same task commit. The agent
must stage and inspect only task-owned hunks so unrelated pre-existing worktree changes are
never included.

## 6. Safety and compatibility boundaries

- No daemon route, persisted data, OpenAPI schema, or configuration migration is needed.
- Analyze-all remains explicit and never starts during import or reindex.
- Remote-provider confirmation remains mandatory before prompt-bearing requests.
- Source and diff remain selectable and read-only.
- Only the isolated declaration/import draft remains editable.
- No automatic Apply, project mutation, scan, fix, push, or product-generated commit.
- No generated build output or local configuration may be edited or committed.
- Existing unrelated worktree changes must be preserved.
- The UI displays the daemon’s sanitized error string; exposing provider/internal failure
  details is outside this plan.

## 7. Verification

Focused verification:

- presentation and status-classification tests in `AnalysisWorkspaceStateTest`;
- shell, explorer visibility, navigation, and breakpoint tests in `DesktopShellTest`;
- keyboard/semantics checks in `DesktopAccessibilityTest`;
- guarded cross-workspace flow checks in `DesktopIntegrationCoverageTest`.

Task-level command:

```text
./desktop/gradlew -p desktop test
```

Every task also runs `git diff --check` and inspects both the worktree diff and staged diff.
The final task runs `make check` when the environment permits and records any manual GUI
checks that could not run.

## 8. Definition of done

1. Analysis shows project coverage and current/last-run totals without listing successful
   or pending files individually.
2. Only failed/error-bearing files are listed, with relative path, attempts, and sanitized
   error text.
3. Analyze-all lifecycle, limits, polling, cancellation, revision binding, and remote
   confirmation are unchanged.
4. Summary, Analysis, and Bugs do not render or reserve space for Editor side panes.
5. Editor retains the explorer and contextual panel on wide layouts and their drawers on
   narrow layouts.
6. Explorer and file-palette selection always open the requested indexed file in Editor.
7. Selected file and Editor flow state survive workspace changes.
8. Keyboard, textual semantics, 1000dp breakpoint behavior, source/diff read-only rules,
   and preview-first safety all remain covered.
9. Tasks 75–79 are Complete, moved under `tasks/completed/`, and represented by five
   reviewed commits with no unrelated user changes.
10. Desktop tests, `git diff --check`, and the supported final validation pass, with any
    environment-limited check reported honestly.
