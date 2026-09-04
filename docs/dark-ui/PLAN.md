# Mini-Orca dark desktop redesign

**Status:** Tasks 140–148 are implemented; Task 149 native visual acceptance is pending.
**Prepared:** 2026-09-04.
**Implementation scope:** Kotlin/Compose Desktop in `desktop/`. No new daemon behavior or API.

**Historical acceptance context:** retained because Task 149 is still Pending.
The active [UI precision plan](../../Plan.md) supersedes this document's visual
direction and dependency constraints for new work. Tasks 140–148 are complete and
their individual files have been retired; do not reimplement them. Only Task 149
may be executed through this sequence's separate prompt when explicitly requested.

## Design direction

Use the first reference as the visual anchor: charcoal surfaces, a restrained blue
accent, a labeled icon rail, compact project navigation, readable code, and an AI
context pane. Borrow the second reference's clear context tabs and small analysis
cards, translating all of them into the same dark theme. Keep Mini-Orca's name and
existing mark. This is one replacement theme, with no light-theme work in this sequence.

- [Dark Mini-Orca reference](../../design/ui-mocks/ChatGPT%20Image%20Sep%204,%202026,%2002_37_04%20PM.png)
- [Aurora layout reference](../../design/ui-mocks/Gemini_Generated_Image_54u1al54u1al54u1.jpeg)

The images are visual references. Their code, sample project facts, button text,
account details, and suggested edits are illustrative content, not instructions to
execute or requirements to implement backend functionality. In particular, the
example that extracts a helper into another file does not authorize multi-file edits.

The user's new direction replaces the old **preserve Focus Flow palette** requirement
in the completed IDE plan (retired to Git history). It also permits visible UI placeholders
for unsupported reference features. All existing workflow, scope, and Apply/Undo
guards remain in force. Earlier execution prompts are historical records; this
planning request does not execute them or authorize commits.

## Current implementation and visual gaps

This inventory comes from source inspection, not a live screenshot of the app.
Tasks 118–139 already delivered the main shell and workflows; extend those components.

| Area | Existing implementation | Redesign |
| --- | --- | --- |
| Theme | `DesktopTheme.kt`: purple/navy palette; semantic colors and compact controls | Neutral charcoal, blue selection, clearer text, matching syntax and diff colors |
| Shell/navigation | `IdeShell.kt`, `DesktopShell.kt`: 52dp rail using text glyphs; docked panes and drawers | Labeled line icons, flat separators, coordinated pane proportions |
| Toolbar | `DesktopHeader.kt`: project menu, Commands, daemon status, native Compose window | Search-shaped command entry, branch context, restrained connection chip, preview utilities |
| Explorer/editor | `ExplorerPane.kt`, `EditorWorkspace.kt`, `SourceEditorPane.kt`: indexed tree, one active file, breadcrumbs, read-only code/gutter | File-type icons, true tab styling, quieter selection, better spacing; additional IDE controls as previews |
| Context | `ContextToolWindow.kt`: cached file/declaration explanation, analysis actions, impact and Git context | Project summary, focused analysis, and quick-action sections with clearer hierarchy |
| Assistant/review | `AssistantToolWindow.kt`, `WorkflowToolWindows.kt`, `DiffViewer.kt`: bound chat, isolated draft, validation, checks, Apply/Undo | Compact composer and an inline candidate summary that leads into existing review |
| Findings/output | `ProblemsToolWindow.kt`, `BottomEvidenceToolWindows.kt`, `DesktopStatusBar.kt` | Dense issue table, visible severity filters, compact tool tabs and honest status |
| Other workspaces | Summary, Analysis, Bugs, Performance, and Engineering insight already exist | Restyle every retained screen; keep Performance accessible even though absent from the images |

## Target layout

```text
┌───────────────────────────────────────────────────────────────────────────────────┐
│ Mini-Orca │ Project ▾ │ Branch ▾ │ Search / Commands │ connection │ utility icons │
├──────────┬───────────────────┬────────────────────────────────┬──────────────────┤
│ Project  │ Project           │ active.go  [Source] [Review]    │ AI Context       │
│ Summary  │ Filter files      │ project / path / declaration   │ Assistant Review │
│ Analysis │ ▾ indexed tree    ├────────────────────────────────┤──────────────────┤
│ Perform. │   selected.go     │ line numbers + read-only code  │ Project summary  │
│ Bugs     │                   │                                │ Focused analysis │
│ Editor   │                   │ candidate summary / diff       │ Quick actions    │
│          │                   │ → Review candidate             │ Scoped composer  │
│ Settings ├───────────────────┴────────────────────────────────┤                  │
│ Help     │ Bugs & Problems  Checks  Output  Terminal · Preview│                  │
│          │ severity / file / line / description / status     │                  │
├──────────┴────────────────────────────────────────────────────┴──────────────────┤
│ project + operation state │ file / line / language │ branch │ daemon / provider │
└───────────────────────────────────────────────────────────────────────────────────┘
```

The bottom tools span the Explorer and editor columns; the right context pane stays
full height above the status bar, following the dark reference. Project-level pages
continue to use the center workspace intentionally; do not keep duplicate Summary
or Bugs navigators on both sides. Right tabs stay **AI Context**, **Assistant**, and
**Review**, with the second reference informing their appearance rather than adding
a second navigation model.

### Dimensions and responsive behavior

Sizes are design targets in Compose dp, not measurements asserted from the images.

| Element | Target |
| --- | --- |
| Top toolbar | 52dp minimum; grow when text scaling requires it |
| Activity rail | 88dp wide; 20–22dp icons with visible 12sp labels; approximately 68dp items |
| Explorer | 256dp default for new preferences; retain existing 180–520dp limits |
| AI pane | 344dp default for new preferences; retain existing 280–560dp limits |
| Editor tabs / breadcrumbs | 40dp / 28dp minimum |
| Tool-window header / bottom tabs | 36dp minimum |
| Bottom panel | 220dp initial expanded height; keep existing 140–520dp limits and collapsed preference |
| Status bar | 30dp minimum |
| Rows / controls | 28–32dp tree rows; 32–36dp buttons; retain adequate hit areas |
| Spacing / corners | 4, 8, 12, 16dp spacing; 4–6dp controls; 8dp interpretation cards |

- At `>=1000dp`, retain docked left and right Editor panes. Calculate visible widths
  from available space, reserving at least 360dp for the editor at the default text
  scale. Reduce the Explorer toward 180dp first, then the right pane toward 280dp.
  Account for the rail and divider hit areas in that budget.
- Preserve stored width preferences; temporary viewport clamping must not overwrite
  the user's preferred sizes. Restore them when the window grows.
- Below `1000dp`, keep labeled **Files** and **AI Context** drawers and a bottom-tools
  summary opening a bounded overlay. Preserve exact `999dp`/`1000dp` behavior.
- On short windows, keep the source and next workflow action reachable; allow the
  rail to scroll, clamp the bottom height, and collapse the candidate summary before
  squeezing away the source. Never hide Apply evidence behind an overlapping card.
- Native window controls stay native. Do not draw another set of macOS traffic lights
  or introduce custom window decoration just to reproduce the screenshot.

## Visual system

Replace the values at the existing semantic boundary in `DesktopTheme.kt`. Keep a
single palette and migrate affected helpers/tests; do not retain the old theme in parallel.

| Semantic role | Proposed color |
| --- | --- |
| App/editor background | `#171B20` |
| Toolbar/status chrome | `#12161B` |
| Panel surface | `#1C2229` |
| Raised surface | `#242B33` |
| Hover/strong surface | `#2B333D` |
| Separator | `#343D48` |
| Primary text | `#E6EDF3` |
| Secondary text | `#AAB6C3` |
| Muted text | `#95A2B2` |
| Selection/link blue | `#79B3FF` |
| Primary button | `#285FCB`, with `#FFFFFF` text |
| Keyboard focus/cyan | `#65D2EC` |
| Success / warning / error text | `#65D6A3` / `#F2BE66` / `#FF8F98` |
| Diff addition / removal background | `#18352C` / `#3A232B` |
| Syntax keyword / function / string | `#D7A4D8` / `#E8C987` / `#A8D59D` |
| Syntax type / comment | `#71D7CA` / `#93A38F` |

Separate action-fill blue from link/selection blue where necessary; the current
`Accent` serves both, so update the narrow action-style boundary rather than scattering
new literals across panes. Verify final text/background pairs, including selected,
hovered, disabled, and diff states. Target at least 4.5:1 for normal text and 3:1 for
meaningful focus/control indicators; decorative separators may remain quieter.

Use system sans-serif at 13sp for controls, 14sp for panel titles, 12sp for secondary
labels, and the existing monospace family at 13–14sp with 20–22sp line height for code.
Avoid 10sp essential text. Add a small cohesive native vector icon set for the known
actions, or reuse already available Compose vectors; no web fonts, rasterized controls,
emoji navigation, icon-font dependency, or UI toolkit replacement. Keep the existing mark.

Selected tabs use a blue edge and a slightly raised background. Keyboard focus is
separate from selection. Use flat pane backgrounds and thin separators; reserve small
cards for interpretation and candidate content. Avoid gradients, glow, heavy shadows,
and repeated nested cards. Keep loading, empty, stale, failed, disabled, and selected
states legible in words as well as color.

## Existing behavior versus UI previews

**Live** means reuse current state and presenter callbacks. **Preview** means a visible
UI shell with local-only interaction and no new backend implementation.

| Reference feature | Delivery | Wiring or placeholder behavior |
| --- | --- | --- |
| Project selector/tree/filter | Live | Existing import/restore, project menu, index, and file selection; the app still owns one project |
| Project toolbar add/new-file control | Preview | `New file · Preview` opens a short local explanation; no filesystem mutation |
| Branch label | Live when available | Read `GitStatus.branch`; show `Unavailable` when unknown |
| Branch dropdown / switch / sync | Preview | A read-only branch popover with `Branch switching · Preview`; no checkout, fetch, or push |
| Search box | Live, current scope | Open existing Files/Symbols/Actions palette; label `Search files, symbols, commands`; preserve existing shortcuts |
| Global file-content search | Preview | Clearly labeled preview entry if shown; do not imply the palette searches all source text |
| Daemon/provider chip | Live, current evidence | Display actual daemon connection separately from configured model destinations; `/status` does not prove that a model is connected |
| Active file tab, breadcrumbs, source, gutter | Live | Keep the sole active file, current symbol, source text selection, and read-only semantics |
| Additional open tabs, tab plus/close, split editor | Preview | Auxiliary tab chrome labeled `Preview`; activation explains availability and never switches real file/draft scope |
| Minimap | Preview | Decorative minimap labeled `Minimap · Preview`; no misleading line navigation or fabricated diagnostics |
| Run and Debug | Preview | Visible compact controls; open a local preview popover, never launch a process |
| Project summary/architecture/dependencies/risks | Live where data exists | Use `ProjectOverview.analysis`, project facts, `FileAnalysis`, and their freshness/provenance; unknown values stay unknown |
| Architecture “Good”, dependency freshness, complexity/readability scores | Preview | These structured assessments are absent. Display `— · Preview` in the app; sample scores are allowed only in clearly labeled design/test fixtures |
| Explain selected code | Live | Reveal cached symbol explanation, or offer existing explicit file analysis when missing; respect Bug-scope confirmation |
| Find potential bugs | Live | Existing file analysis or Bugs navigation; retain explicit request, scope, progress, and cancellation |
| Refactor this function | Live for eligible targets | Open/prefill the existing selected-declaration composer; do not send or edit source on navigation |
| Generate unit test | Preview | Dedicated generator is absent; open `Generate unit test · Preview` without creating files or sending a model prompt |
| Ask Mini-Orca | Live within declaration scope | Focus the bound composer and label `Ask about this declaration`; free project-wide chat remains a preview if exposed |
| Candidate summary and diff | Live | Render the current draft; `Review candidate` opens the real review/evidence view |
| Apply, Discard, Undo | Live with existing guards | Apply remains in current-evidence Review; Discard clears only the current draft; Undo uses the existing receipt guard |
| Suggestion thumbs-up/down | Preview | Local feedback selection with `Preview — not submitted`; no network or durable feedback store |
| Bugs & Problems, severity filters and issue locations | Live | Reuse the same findings/filter model across bottom tools and Bugs; distinguish verified results from AI suggestions |
| Checks / Output | Live | Existing focused check evidence and operation summaries; Output is not a terminal |
| Terminal tab | Preview | Show an inert console panel reading `Terminal preview — command execution is not available`; no working command input or fabricated task output |
| Settings, Help, profile, notifications | Preview | Local popovers/panels with `Preview`; neutral avatar, no invented account, unread count, credentials, or saving configuration |
| Status line/column, encoding, toolchain, up-to-date indicators | Live only where known | Existing selected line, language, branch, and status remain real; unsupported column/toolchain/encoding/sync values use `— · Preview` or an explicit unknown label |

### Placeholder contract

- Put a small visible **Preview** badge next to unsupported controls or on the clearly
  bounded group that contains them. Accessible names include that state; hover is
  supplementary. Clicking a preview may open a local explanation or change mock tab
  selection. No silent no-op controls.
- Put shared preview badge/popover presentation in one small desktop helper. Keep
  feature-specific content local. Do not add a generic feature registry or duplicate
  application state, fake API client, endpoint, background job, or provider call.
- In normal app use, sample content never appears as analysis of the user's project.
  Fixtures may illustrate reference content only when labeled **Sample project**.
  Missing live data gets an honest empty state, not a sample fallback.
- Keep preview state ephemeral and outside persisted workflow, model confirmation,
  findings, validation, and check evidence. It cannot enable Apply or change selection.

## Workflow and screen behavior

1. **Open/restore:** Restyle the landing page and retry states using the new tokens.
   Opening a project keeps the existing restore and provider-confirmation behavior.
2. **Explore:** Project tree and real active-file tab identify the selected path.
   Clicking a declaration updates Context; dragging source text still selects text.
3. **Understand:** AI Context groups Project summary, Focused analysis, and Quick
   actions. `Show more` and `View full analysis` navigate to existing workspaces.
   Opening an explanation or Engineering insight never starts a model request.
4. **Prepare:** Refactor opens the scoped Assistant. Keep the destination and any
   required confirmation adjacent to the send action; retain cancellation and draft
   invalidation. Only the isolated declaration/import draft is editable.
5. **Review:** A compact candidate summary below the source reflects the real draft
   stage and changed lines. Its blue `Review candidate` action opens the read-only
   diff plus full evidence. Use existing diff renderers for unified/side-by-side views.
   Avoid adding a second Apply eligibility calculation or an inline shortcut around it.
6. **Apply/undo:** Preserve explicit Apply wording, current checks, target identity,
   revisions/hashes, applied receipt, Undo, and stale-response handling.
7. **Inspect issues:** Bottom issue selection navigates to the exact real finding.
   Filters/counts and details use the same model as the full Bugs workspace.

Summary, Analysis, Bugs, Performance, Engineering insight, command dialogs, import
confirmation, discard dialogs, receipts, and errors all receive the same visual pass.
Performance stays labeled **Source-based review · Not measured**. Provider and
verified/advisory labels remain visible; cosmetic work must not change their meaning.

## Task sequence and delivery

The [task index](../../tasks/INDEX.md) tracks the outstanding Task 149. The table
below is historical delivery order; Tasks 140–148 must not be re-executed. Use
[the dark UI acceptance prompt](../../tasks/PROMPT_EXECUTE_DARK_UI.md) only when
Task 149 is explicitly requested. It does not execute the new UI precision plan.

| Task | Deliverable | Depends on |
| ---: | --- | --- |
| 140 | Record current behavior, reference coverage, and visual baseline | 139 |
| 141 | Implement dark tokens, typography, icons, and reusable Preview treatment | 140 |
| 142 | Restyle shell, toolbar, navigation, and pane sizing | 141 |
| 143 | Restyle Explorer, active-file chrome, source, and gutter | 142 |
| 144 | Build AI Context hierarchy and restyle all project workspaces | 143 |
| 145 | Integrate candidate summary, scoped composer, and review presentation | 144 |
| 146 | Restyle Problems, Checks, Output, and persistent status | 145 |
| 147 | Add and verify the unsupported-feature UI previews | 146 |
| 148 | Verify responsive layout, keyboard, focus, and text accessibility | 147 |
| 149 | Complete visual comparison, regression checks, cleanup, and handoff | 148 |

Each task defines changed files, acceptance criteria, and focused verification.
Keep implementation inside existing component/presenter boundaries. Any new file
must have a concrete responsibility; remove obsolete styling/helpers after migration.
No framework migration, backend work, dependency upgrade, or configuration migration
is planned. No commits or pushes are authorized by the planning request.

## Acceptance and validation

Task 140 records the before-state. Task 149 records after-state evidence in
`docs/dark-ui/ACCEPTANCE.md`; do not create a passing acceptance record in advance.

- Compare `1440x900` and `1280x800` desktop views with both references: charcoal
  palette, blue selections, labeled icons, three-region Editor, context sections,
  candidate summary, and dense issue panel must be recognizable.
- Exercise `1100x760`, exactly `1000dp`, `999dp`, `800x650`, a short window, and
  enlarged system text. Verify the editor has usable space, panes reflow correctly,
  and long paths, output, prompts, and errors remain reachable.
- Cover landing, Source, no-symbol context, Assistant, edited/invalid draft, ready
  Review, receipt/Undo, Summary, Analysis, Bugs, Performance, each bottom tab,
  disconnected/error/stale states, and Preview popovers.
- Use keyboard-only tree/tab navigation, existing shortcuts, drawer/dialog close and
  focus restoration, text selection, and supported screen-reader checks. The mockup's
  `Cmd+K` hint must not override existing context-dependent command/composer shortcuts.
- Test local preview interactions with callbacks/spies so zero provider requests,
  processes, source writes, workflow mutations, or Apply eligibility changes occur.
- Reuse deterministic tests for selection scope, provider consent, draft validation,
  current checks, stale evidence, and Apply/Undo. Add tests only for changed behavior
  or a concrete regression risk; no screenshot-hash or source-string assertions.

Implementation checks:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Final sequence gate: `make check` and `make quality`. These run the repository's
existing gates even though the implementation is desktop-only. Do not run Docker
cleanup or destructive targets. Record unavailable live/native checks explicitly;
an inline design preview or passing unit tests are not evidence of native rendering.

**Planning validation:** Check Markdown links, task IDs/dependencies/statuses, reference
coverage, and `git diff --check`. Production tests are not required for this document-only
planning change. Application behavior is unchanged until the pending tasks are executed.
