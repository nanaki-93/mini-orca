# 82 — Remove technical metadata and redundant UI copy

## Status

Pending

## Goal

Remove low-value counts, revisions, hashes, and repeated explanations from the routine
Desktop presentation while preserving all identity and safety behavior internally.

## Depends on

Task 81.

## Required task commentary

- Before editing, post a concise user-facing commentary update beginning with
  `Starting Task 82` and name the presentation surfaces, likely files, copy audit, and
  focused tests.
- After verification, post a concise update beginning with `Task 82 complete` and state
  which metadata/copy was removed, which safety text remains, tests run, and that Task 83
  is next.
- During long-running work, continue posting brief progress commentary at least every
  60 seconds. These are user-facing task updates, not source-code comments.

## Implementation

- Audit visible Compose `Text`, button labels, status messages, presentation helpers,
  accessibility labels, and tests before editing. Do not remove identity fields from
  serialized models or guard logic.
- Simplify top-bar project identity to the project name only. Remove the displayed
  project revision SHA.
- Remove numeric counts from workspace navigation. Replace
  `workspaceRailLabel(workspace, counts)` with a concise label contract and remove
  `WorkspaceCounts`, `workspaceCounts`, callback parameters, and tests if no remaining
  supported UI consumes them.
- Remove the Explorer metadata subtitle containing indexed-file count and project
  revision. Keep the Explorer title, filter, tree, relative paths, language, selection,
  and analysis freshness.
- Simplify Project Summary:
  - keep project type and useful build metadata;
  - show detected language names without per-language counts;
  - remove project file/source/line inventory and project revision;
  - preserve model interpretation, coverage, findings, and workspace navigation where
    still useful.
- Remove visible raw identity values from Editor-related presentation:
  - file hash from Target;
  - project revision and base hash from bound conversation;
  - draft revision/hash from the draft card and Editor stage messages;
  - draft revision/hash details from Verify evidence;
  - project revision/resulting hash from Apply/Undo receipt;
  - revision/hash wording from disabled and stale messages when a human-readable
    `out of date` or `no longer matches` message is sufficient.
- Remove project revision from finding status text and Analyze-all status descriptions.
  Keep freshness/stale language and internal revision matching.
- Remove raw project revision from transient Desktop status messages after Apply/Undo or
  reindex.
- Apply the persistent-copy rule from `PLAN.md`:
  - remove page subtitles that restate the heading;
  - remove the Explorer freshness legend;
  - remove the always-visible sentence below Editor stage controls;
  - remove suggestion summaries printed below suggestion buttons;
  - keep one read-only indication around a diff and remove repeated explanations;
  - remove the sentence below Send message;
  - remove finding-classification descriptions when the heading is sufficient;
  - remove duplicated lifecycle/control detail already shown by a status/progress row.
- Preserve visible copy for errors, loading, stale state, validation diagnostics, failed
  checks, remote-provider destination/confirmation, a blocked required next step, exact
  Apply file/symbol, and Apply/Undo result.
- Preserve fuller explanations in accessibility semantics or an explicit on-demand
  details disclosure only when they materially help. Do not hide actionable errors solely
  in a tooltip.
- Remove obsolete fields, helpers, imports, parameters, branches, and exact-copy tests
  made redundant by the simplified presentation. Do not retain old and new strings in
  parallel.
- Do not change action layout, Bugs filter disclosure, connection placement, API models,
  or identity checks in this task.

## Acceptance criteria

- The top bar, Explorer, workspace rail, Project Summary, Target, Draft, Verify, Apply,
  Bugs, Analysis, and transient status copy contain no raw project revision or hash.
- Project-wide file/source/line and per-language counts are absent from Project Summary;
  selected-file size and line count remain available in Target/file analysis.
- Workspace navigation contains only concise workspace names.
- No always-visible explanatory sentence remains directly below a self-explanatory button
  unless it is a current error, blocked reason, remote-provider warning, or Apply safety
  requirement.
- Empty/loading/failure/stale states, validation diagnostics, focused-check failures,
  remote confirmation, and explicit Apply target remain clear without relying on color.
- Revision/hash values remain present and unchanged in API models, request identity,
  asynchronous stale-response rejection, draft/check matching, Apply, and Undo guards.
- No daemon, API, persistence, configuration, or migration change is introduced.

## Verification

- Update `DesktopShellTest`, `ProjectSummaryPaneTest`,
  `DesktopAccessibilityTest`, `AnalysisWorkspaceStateTest`,
  `BugsWorkspaceStateTest`, `EditorWorkspaceTest`,
  `ReviewEvidencePaneTest`, and related exact-copy tests where affected.
- Add focused assertions that display helpers omit revision/hash/count values while guard
  tests continue proving those values are enforced internally.
- Search visible presentation code for remaining raw-identity interpolation. Review every
  remaining `projectRevision`, `contentHash`, `baseFileHash`, and `draft.hash`
  occurrence; keep only non-presentation uses required for correctness.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

- Inspect the complete Task 82 diff for weakened guards, removed actionable feedback,
  leftover obsolete helpers, and unrelated changes.

## Completion

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, and
update its link/status in `tasks/INDEX.md`. Do not create a commit; this task definition
does not authorize commits.
