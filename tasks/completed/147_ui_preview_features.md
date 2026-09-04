# 147 — Add unsupported-feature UI previews

## Status

Complete

## Depends on

Task 146.

## Goal

Include the remaining reference controls as clearly identified, locally interactive
UI mocks, without implementing their backend functionality.

## Implementation

- Complete every Preview row in the [feature matrix](../docs/dark-ui/PLAN.md#existing-behavior-versus-ui-previews):
  new-file control, branch switching/sync, optional content-search entry, extra tabs,
  tab add/close, split editor, minimap, Run/Debug, assessment scores, unit-test
  generation, feedback, Terminal, Settings/Help/profile/notifications, and unsupported
  status fields. Free chat is a preview only if that broader entry is exposed.
- Use the shared Preview badge/popover. Place controls at their reference locations;
  keep project/file-specific actions unavailable until their relevant context exists.
- Show real branch context inside its preview dropdown when known. Do not invent an
  account, notification count, analysis score, provider health, or terminal result.
- Use `— · Preview` for unimplemented assessments. Additional tab labels clearly say
  Preview and cannot change the real selected file or draft. A decorative minimap
  cannot imply click-to-navigate or real diagnostics.
- Add a Terminal tab within the existing bottom-pane presentation; its content is
  inert and states that command execution is unavailable. Do not instantiate a shell.
- Let feedback selection change locally with `Preview — not submitted`. Settings,
  Help, account, and notification panels show their scope without saving configuration,
  sending messages, opening external links, or reading credentials.
- Keep all preview state ephemeral and outside workflow, provider consent, draft/check
  evidence, and persistence. Remove any temporary sample fallback in live panes.

## Likely files

`PreviewFeature.kt`, `DesktopHeader.kt`, `EditorWorkspace.kt`, `ContextToolWindow.kt`,
`IdeShell.kt`, `DesktopLayoutState.kt`, `DesktopApp.kt`, and preview interaction tests.
No `ApiClient.kt`, Go, daemon configuration, or API-contract implementation changes.

## Acceptance criteria

- Every unsupported visible reference control has a visible Preview state and an
  accessible explanation; there are no silent active-looking no-ops.
- Every Live feature still uses real state; preview content cannot be confused with
  an analysis result, focused check, active provider connection, or applied change.
- Preview activation cannot call a provider/API, launch a process, write source,
  mutate workflow selection/evidence, or enable Apply.
- Escape/dismiss restores focus; preview panels work in both docked and narrow layouts.
- Existing persisted bottom-tab preferences still load safely after any enum addition.

## Verification and completion

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.
Use callback spies or a rejecting test boundary for preview interactions; assert no
workflow/network/mutation callbacks and no eligibility changes. Test Terminal open/close,
feedback reset, extra-tab identity, missing branch, and preference compatibility.
Record complete matrix coverage, then move the task/update the index; no unrequested commit.
