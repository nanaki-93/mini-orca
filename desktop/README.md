# Mini-Orca Desktop

Start the Go daemon from the repository root, then launch the desktop client:

```bash
go run ./cmd/daemon
./desktop/gradlew -p desktop run
```

The client uses `http://localhost:9090` by default. Set `MINI_ORCA_URL` to point it at another daemon URL. The daemon is loopback-only by default. The Desktop labels the effective `analyze`, `bug`, and `function` destinations independently; each non-loopback scope needs confirmation before its own prompt-bearing request. Keep provider credentials in the ignored local `config.yaml`.

The Editor workflow is Go-first: exact replace/create declaration drafts receive composition and focused validation. Other language views remain analysis-only until equivalent validators are available.

## Build compatibility

The checked-in wrapper uses Gradle 8.6 with Kotlin 2.0.21 and Compose
Multiplatform 1.7.0. Kotlin's [Gradle compatibility table](https://kotlinlang.org/docs/gradle-configure-project.html#apply-the-plugin)
lists Gradle 8.6 as fully supported for Kotlin 2.0.20–2.0.21; the
[Compose Multiplatform 1.7.0 release](https://blog.jetbrains.com/kotlin/2024/10/compose-multiplatform-1-7-0-released/)
documents its Kotlin 2.0.20 pairing. Keep the wrapper and plugins aligned, and
use `./desktop/gradlew -p desktop test --warning-mode all` to check the desktop
build without a globally installed Gradle.

## Opening a project

After the first successful project open, Mini-Orca remembers the canonical local path.
Later launches automatically restore that project from its persisted local analysis and
refresh deterministic facts without contacting the model. If restoration is unavailable,
Mini-Orca falls back to its dedicated landing state with concise retry feedback. Press
`Cmd/Ctrl+O` to open the project chooser from the keyboard. Until a project opens,
workspace, file, symbol, draft, palette, and project-management shortcuts are unavailable.

## IDE shell

After import, Mini-Orca uses one stable source-first shell:

- At widths of at least `1000dp`, Project is docked left; Context, Assistant, and Review
  are docked right; Problems, Checks, and Output are docked below the editor; and a
  persistent status bar reports the trusted current project, file, analysis, provider, and
  daemon state.
- Below `1000dp`, **Files** and **Context** open labeled modal drawers, while the bottom
  area becomes a compact summary that opens a bounded overlay. Leaving Editor closes an
  incompatible drawer without changing the selected file, declaration, draft, or evidence.
- The toolbar keeps the project identity, connection state, Command entry point, and narrow
  drawer actions visible. Infrequent project operations live under the labeled **Project**
  menu before essential state is hidden.

The Project tool window contains only indexed, project-relative navigation. Its file-analysis
state is always textual (for example, **Fresh**, **Stale**, or **Failed**), not a color-only
indicator. The editor has one active file, breadcrumbs, a read-only Source/Review surface, a
dedicated gutter, and selectable source or diff text. Mini-Orca does not imply general source
editing, multi-file tabs, terminal execution, or VCS operations.

## Dark desktop presentation and previews

The desktop uses charcoal surfaces, blue selection and action states, a labeled 88dp line-icon
rail, compact editor chrome, structured AI Context, a current-draft candidate summary, and a
persistent status strip. At 1000dp and above, temporary pane clamping preserves a 360dp editor
without overwriting stored Explorer or AI Context widths; below that boundary the existing
Files/AI Context drawers and bounded bottom overlay remain in use.

Some reference-style controls are deliberately **Preview** only: new file, branch actions,
content search, extra tabs/split/minimap, Run/Debug, assessment scores, unit-test generation,
feedback, Terminal, and settings/help. Their dialogs state the limitation, are local-only, and
cannot call a provider/API, execute a process, write source, alter workflow evidence, or enable
Apply. The Terminal tab is inert and has no command input.

## Focused workflow and safety

Summary provides compact deterministic facts and advisory interpretation. Analysis starts,
pauses, resumes, or cancels bounded sequential Analyze-all only after an explicit request and
lists only sanitized failures. Bugs reuses the shared compact Problems rows, filters, lifecycle
actions, and a selected-details region; selecting or filtering a finding never changes source.

Editor remains scoped to one project, one indexed file, and one selected symbol or new
declaration. Context exposes file/declaration facts and the explicit **Edit `<symbol>`** route.
Assistant owns the bound request and editable draft; Review contains validation, current focused
checks, exact Apply wording, the receipt, and Undo. Changing a target while a draft is active
requires the existing discard decision. Draft edits invalidate prior validation and check evidence.
Only the guarded Apply and Undo operations can mutate source; source and composed diffs are
selectable and read-only throughout.

Where an existing result includes an optional Engineering insight, its compact in-page panel is
collapsed by default and labelled **AI interpretation**. Opening or closing it is local display
only, and its disclosure preference is shared across result pages. Missing insights have no
placeholder. Stale file/project owners are labelled **Outdated — source changed**; an edited
draft clears the prior proposal insight for the current candidate.

## Keyboard and accessibility

- `Cmd/Ctrl+P` opens indexed files, `Cmd/Ctrl+Shift+O` opens symbols in the active file, and
  `Cmd/Ctrl+1` through `4` continue to select Summary, Analysis, Bugs, and Editor. The
  Performance workspace is available beside Analysis and from `Cmd/Ctrl+K`; `Cmd/Ctrl+Tab`
  includes it in workspace cycling.
- `Cmd/Ctrl+K` focuses the eligible Assistant request, `Cmd/Ctrl+Shift+D` focuses the current
  draft, `Cmd/Ctrl+Enter` generates or cancels generation, `Cmd/Ctrl+Shift+V` validates, and
  `Cmd/Ctrl+Shift+C` runs focused checks when the guarded action is available.
- `Cmd/Ctrl+Shift+F` opens Bugs. Arrow keys move within the Project tree and tool-window tab
  groups; `Enter` or `Space` activates the focused tab. `Escape` closes only the topmost dialog,
  drawer, or bottom overlay, or cancels the active cancellable operation.
- Tab to **Engineering insight** where shown and press Enter or Space to disclose it; **Close
  insight** returns keyboard focus to the opener without triggering a model request.

All actions retain text labels or accessible names, state remains textual in addition to color,
and cyan indicates keyboard focus. The full release-operator matrix, including screen-reader,
text-scaling, and viewport checks, is maintained in
[`KEYBOARD_SMOKE_CHECKLIST.md`](KEYBOARD_SMOKE_CHECKLIST.md).

## Verification status

The automated desktop and repository suites cover layout breakpoint behavior, source/diff
read-only safety, selection scope, stale responses, provider confirmation, validation/check
identity, Apply/Undo, command navigation, status state, and compact findings presentation. This
environment has no interactive Mini-Orca window or configured provider fixture, so live
screenshots, assistive-technology checks, and provider-backed end-to-end runs remain explicit
release-operator checks rather than claimed passes.
