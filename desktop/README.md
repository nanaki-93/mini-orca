# Mini-Orca Desktop

The client uses `http://localhost:9090` by default; `MINI_ORCA_URL` overrides it.
Start the Go daemon using the root [README](../README.md). All provider keys stay
in the ignored daemon configuration, not desktop preferences.

## Runtime and build

The checked-in build uses Gradle 9.1.0, Kotlin/Compose compiler 2.3.20, Compose
Multiplatform 1.11.0 and Jewel standalone `0.40.0-262.10315.125`. Use JBR
25 SDK for app launch and packaging. The root
[development guide](../README.md#development-and-documentation) owns clean-checkout
toolchain setup and full validation, including explicit JDK/JBR locations. Do not
commit a local runtime path or replace a system JDK as part of a task.

From the repository root:

```sh
MINI_ORCA_JDK21_HOME=/path/to/jdk-21 \
MINI_ORCA_JBR25_HOME=/path/to/jbr-25 \
  ./scripts/desktop-gradle.sh run

MINI_ORCA_JDK21_HOME=/path/to/jdk-21 \
MINI_ORCA_JBR25_HOME=/path/to/jbr-25 \
  ./scripts/desktop-gradle.sh test createDistributable
```

The build runs Gradle on Java 21 and explicitly launches Desktop `JavaExec` tasks,
including `run`, with the selected Java 25 toolchain. Calling `gradlew run` with a
Java 21 `JAVA_HOME` and no discoverable Java 25 toolchain cannot start the app.

Packaged images include `java.net.http` for the daemon client and `jdk.unsupported`
for Jewel's native bridge. Package with the JBR launcher, not the Detekt launcher.
Native behavior should be checked on the target host; component tests cannot
prove OS focus, popup placement or screen-reader behavior.

## Working in the app

On launch, the landing shows the last successfully opened project's remembered
path (or that no project is remembered) and automatically tries to restore it
from saved local data. Restore does not request a model or start a terminal. The
remembered path is not the current project until the restore succeeds; after a
successful open, Summary shows the current project. While an open is in progress,
the landing or loaded-project header labels the requested path separately from
the current project and shows restore or import progress, not analysis progress.

If restore fails, the requested path and diagnostic stay available. **Retry
restore** retries only that failed restore using saved local data, without a
model request; **Open project…** lets you choose another folder instead. A failed
import does not offer Retry restore. Daemon connectivity has separate feedback:
**Reconnect daemon** refreshes daemon status and model configuration but does
not retry the open. If local preferences cannot be read, the landing reports a
storage warning and **Open project…** remains available. If saving the last project
fails, the successfully opened project stays open, but it may not be remembered
for the next launch; this is a preference warning, not an opening failure.

The project menu shows **Open project…** without a loaded project and **Switch
project…** with one. Cmd/Ctrl+O uses the same native directory chooser. Opening
or an unfinished switch disables another chooser; Re-index is also unavailable
while opening or switching. Canceling the chooser leaves the current project,
opening error, draft, terminals and remembered path unchanged; it sends no
request. After choosing a folder, review the current project and requested full
path before importing. If a draft exists, **Approve draft discard for switch**
records intent without discarding it. If the Analyze destination needs remote
confirmation, confirm it and select **Continue with provider**; a local destination
needs no remote confirmation. **Cancel switch** at any pre-commit review stage
preserves the draft, current project and terminal tabs, without importing. If the project,
draft or Analyze destination changes during review, approval must be reviewed
again. The final **Switch project** (or **Close shells and switch** when tabs exist)
commits the switch. Choosing a folder alone does not cancel project work, discard
the draft or close shells.

After commitment, the app closes *all* old-project terminal tabs, including
hidden, starting, exited and failed tabs, and waits for verified cleanup before
discarding an approved draft and dispatching one import. Closed tabs cannot be
restored by dismissing the committed review. If cleanup fails or takes too long,
the current project and draft remain, import is blocked, and the review reports
that some tabs may already be closed; check Terminal before trying a new switch.
If import fails after successful cleanup, the attempted path and error remain
visible, but the discarded draft and closed tabs are not rolled back. Explicit
import may invoke the configured Analyze provider; unlike restore, it is not
guaranteed to be model-free.

With a project loaded, use **Re-index project** in the project menu to refresh its
local inventory and freshness after file changes. Re-index does not request a
model, run analysis, execute project code or refresh model findings. It is
unavailable during opening, switching or another re-index attempt. The header
shows indeterminate indexing progress (not a percentage), the accepted inventory
revision on success, or a re-index-specific diagnostic on failure. Prior inventory
and saved evidence remain available on failure with their existing freshness
labels; if follow-up workspace details cannot load after success, the header
reports them unavailable rather than claiming refreshed findings. A changed
revision makes an editable draft stale: its text remains recoverable, but old
validation, checks and Apply authority are lost. Use **Retry re-index** beside a
failed or canceled attempt, or activate **Re-index project** again explicitly;
neither reconnecting nor navigating retries it automatically. The Analysis
**Files** selection remains project-specific, including exclusions for
transiently absent paths; newly eligible files follow the daemon defaults.

Summary describes the project. Analysis owns one whole-project run and progress;
Bugs, Performance and Security own separate results. The scrollable left rail shows
icons and visible labels in the order Summary, Analysis, Bugs, Performance, Security,
Editor. Below them, separate Terminal, Commands and Models actions do not change
the selected workspace. Commands opens the actions palette; header search opens
indexed file search. Models opens the same configured model and destination details
as the footer counts, including when configuration is unavailable. Analysis status
appears immediately before the separate daemon connectivity status in the header;
neither is provider health. **Reconnect** reads daemon status and model
configuration; it does not contact a provider or execute project code. Editor
owns one declaration change. Go, Java and Kotlin
projects show a type icon beside the project name in the top bar.

Summary introduces the project name with its metadata-derived type beside it (Go,
Java, Kotlin or a generic/unknown label), followed by available purpose, build
metadata, indexed files, lines and languages. The description is visibly qualified:
current AI-generated interpretation, stale saved interpretation, or an unavailable,
running or failed description with its failure details. A current description with
no purpose is labeled unavailable; a re-indexed project's old overview does not
supply current facts. File coverage does not determine description freshness.
When starting is eligible, **Start analysis** in the introduction requests a scope
preview using the default full-run policy; it does not start a run or grant consent.
Review the preview and any required destination/Security confirmations in the
existing admission dialog, then explicitly start there. Preview progress and
failures remain visible; retry after failure is explicit. Active runs retain
Summary progress and their applicable Pause/Resume/Cancel controls instead of
offering a fresh Start.

Coverage and results share a region: a continuous-arc dial for saved selected-file
coverage sits beside the Bugs, Performance and Security cards at readable local
widths, or above them when space or text scale requires stacking. The percentage
is saved up-to-date files divided by selected files, with the count and denominator
shown in text; excluded files are not selected files. Coverage is not a health
score: even 100% does not mean the project is safe or free of findings. Positive
counts appear as proportional arcs and focusable legend toggles for up to date,
outdated, not analyzed, running, failed, incomplete and unavailable saved states.
Activating a legend entry opens a local, read-only inspection with its count,
meaning and, when a matching confirmed file selection is available, selectable
project-relative paths and specific saved-state explanations. Activate it again
to close it or choose another status to switch; inspection does not start analysis,
save selection or navigate away. Aggregate-only saved coverage can show counts
but not file paths; inspection says paths are unavailable. Unavailable coverage
and a confirmed empty selection have separate labels without a percentage. Run
status and applicable controls remain independent of saved coverage, including
during a rerun; the coverage status also exposes run and description details on
hover or keyboard focus. **View analysis** only opens Analysis. Analysis category
panels also stack when their local width cannot fit readable columns.

Three named Bugs, Performance and Security cards open their result pages with one
click or keyboard activation. Card counts come from run-reported category progress,
not necessarily loaded details; a dash means the count is unavailable, not zero.
A reported zero is not labeled as no findings until matching completed details load.
Detail loading and read failures remain labeled beside the count independently of
run status. Zero findings use a neutral surface and do not assert that a project
is safe. Bug priority counts require matching current details. Overall tool-reported
issues and AI suggestions appear separately below the cards, since those totals
cannot be reliably assigned to individual categories.

**File evidence** below the cards shows up to three project-relative paths, sorted
by path, with their saved analysis states and explanations. Its “Showing N of M
selected files” label counts the confirmed selection, excluding excluded files;
active-run progress and pending selection edits do not change these saved rows.
Loading, saving or a failed selection read/save can retain the last confirmed
selection with a visible notice. A confirmed empty selection is labeled separately
from unavailable selection. Aggregate-only saved coverage can show counts in the
dial but cannot provide file paths, so File evidence says paths are unavailable.
**All files** opens Analysis for the complete file inventory without changing the
selection or opening a source-editing target.

Architecture and Engineering insight sit beside each other at readable local
widths and stack in that order at narrower widths or larger text. A lone available
panel uses the full width. Packages / modules and Selected findings sit beside
each other where both columns are readable, stacking in that order at narrow
widths or larger text. Without modules, findings use the full width; absent
narrative sections leave no empty cards. Insight retains its lead content and
**More insight** disclosure. Module names, exact paths and responsibilities
remain selectable/readable. Entry points and next steps are omitted from Summary.
**Selected findings** appears after narrative, beside or below modules (if
present), before the project's saved Flows (if present) and the separate
Change lifecycle. It remains visible without narrative or modules. It shows
up to five loaded Bugs, Performance and Security results, with category,
severity or impact, exact location, evidence origin and material state. “Showing N of M loaded findings” is a bounded
preview of loaded evidence, not the run-reported category counts on the cards.
The panel labels not-loaded or failed details and retained stale, partial or canceled
evidence rather than treating missing rows as confirmed zero; only matching
completed empty results are labeled empty. **All Bugs results**, **All Performance
results** and **All Security results** open their complete category lists, including
results beyond the preview.

Activate a preview row to inspect its exact loaded result in the existing category
page; the destination reveals it even if local filters previously hid it. If that
result has disappeared or its identity is ambiguous, the page explains why it
cannot open the old target instead of selecting another result, and offers **View
current Bugs results** (or the corresponding category) to return to ordinary
browsing. A project or run change invalidates an old preview click. These rows,
category routes and **All files** are local inspection/navigation, not analysis,
scanning, fix preparation, project-code execution or source writes. The Summary
scroll keeps lower sections reachable.

The separate **Change lifecycle** section describes Mini-Orca's editing workflow,
not the analyzed project's Flows: Request → Draft → Validate → Checks → Review →
Apply. Only the isolated declaration/import draft is editable; Apply explicitly
changes one file under guards, and Undo is available only while its guards hold.
The stages are information, not actions or readiness indicators. **Open Editor**
only navigates to Editor; it does not change the draft or run any editing step.

Architecture and each project Flow show a bounded, locally rendered preview of
saved Mermaid flowcharts or sequence diagrams. **Expand diagram** opens a local
viewer without leaving Summary. It provides two-axis scrolling, 75–200% zoom in
25-point steps with reset, and a **Mermaid source** disclosure with selectable,
read-only original content and **Copy source**. Closing retains the diagram's zoom
and scroll position while the same Summary result is present. Loading and render
failures appear in the preview; failed or unsupported diagrams retain their complete
source instead of showing an invented graph. No browser, provider call, project-code
execution or source write is needed to inspect a diagram.

New Analysis results request Mermaid diagrams. Older prose reports remain readable,
with an unavailable preview and disabled Expand action; they are marked stale after
the prompt update. Run Analysis explicitly to replace them. Previously saved
single-line arrow chains also render as Mermaid; the viewer distinguishes the
original saved chain from generated Mermaid used only for rendering.

The diagram renderer bundles beautiful-mermaid and its licenses, using the embedded
GraalJS community runtime and JSVG for SVG text and arrow rendering. Normal Gradle builds use the checked-in bundle and do not
require Node.js. To rebuild it after changing `desktop/mermaid/renderer.js` or its pinned
dependencies, run `npm ci --prefix desktop/mermaid --ignore-scripts` and
`npm run build --prefix desktop/mermaid`. Commit the lockfile and generated resource
bundle together; do not edit `src/main/resources/mermaid/renderer.js` manually.

The app requests a maximized native window at startup and remains resizable.
Editor places visible Files, source/diff canvas and Context/Assistant/Review panes
side by side when the measured workspace can fit their readable minima at the
current text scale. Below that boundary they stack in a vertical scroller, with
bounded inner panes; hidden panes stay hidden. Source and composed diffs remain
selectable/read-only; only the isolated draft is editable. Tabs, draft actions and
long file paths remain available through local reflow or scrolling. Results also
switch from list/detail columns to separately scrollable stacked regions when
local width or text scale requires it; selection and filters are retained.

Saved Files, right-pane and Terminal dimensions are *preferences*, not fixed
allocations. The shell resolves effective sizes against available workspace width
and height without saving temporary clamps; enlarging the window restores sizes
that fit again. Only explicit splitter input changes the preferred size. Existing
visual-preference keys and valid values remain compatible: invalid non-finite
sizes recover to dimension defaults, finite out-of-range sizes clamp to maintained
bounds, and legacy bottom-tool keys are removed on save. No configuration or data
migration is needed beyond this compatible visual-preference normalization. Saved
Terminal dimensions restore with the dock collapsed; layout restore or reflow
never opens a shell.

**Source** and **Candidate diff** share file/declaration breadcrumbs. A validated
candidate opens a full-height Current/Candidate comparison with synchronized
vertical rows and independent horizontal scrolling; diffs default to **Side-by-side**, and **Unified** remains an explicit local choice. The Request → Draft →
Validate → Checks → Review strip reflects current evidence, including missing,
stale, failed and running states. Long metadata values can wrap and be selected;
recorded diagnostic output has a bounded preview and a local **Show full available
output** disclosure when more sanitized text was supplied to the client. Expanding
or copying evidence does not run checks. **Edit draft** opens the existing isolated
editor. Tabs and these navigation controls never generate, validate, run checks
or apply a change.

Context shows a short description for the selected declaration, with explicit
explanation and **Refactor** actions. Without a selected declaration, **Actions**
and **Details** provide file actions, metadata and expandable project context.
Labeled warning/error states use amber/red. Selecting a declaration or opening a
tab never sends a model request.

A current draft must be discarded explicitly before changing its target. Editing
it invalidates validation/check evidence. Review shows target identity, readiness,
three compact Validation / Focused checks / Source unchanged rows, and a summary
of reported required checks. Check details and read-only project context start
collapsed; failed output and validation diagnostics remain visible without hovering.
A check rerun shows Running even if the previous report passed. Failed or
canceled validation and focused-check attempts remain labeled beside their
retained evidence. Edit the draft and explicitly revalidate after a validation
transport failure; run focused checks again explicitly after a failed or
canceled check attempt. A prior pass alone cannot enable Apply while a later
attempt is unresolved.

The bottom action names the exact declaration/file scope. **Apply change** uses
the existing eligibility checks; a returned receipt alone can enable **Undo this
change**. Keep the action and its scope accessible in any new layout. **Edit draft** returns to the
existing editor. Check actions that execute a generated test show the exact
command and an explicit **Trust local execution & run checks** label; rerun is
available inside Check details. No disclosure or navigation executes those actions.

Findings and insights show freshness; daemon connectivity never proves that a
provider is connected. Non-loopback scopes require their own confirmation.

## Analysis and results

Use **Start analysis** to request a full-project preview, or **Analyze stale &
failed** for a daemon-selected subset. Both actions preview before starting;
returned included files reflect saved selection and eligibility, not every
physical project file. A full run requests fresh evidence for eligible stages;
selective retries may reuse fresh stages. Unchanged deterministic Security rules
can reuse saved results. **Resume** requests a fresh continuation preview for the
captured run, retaining its original refresh/retry policy and progress. Its
returned files are the admitted file set, not a count of remaining files; its
request bounds describe the current continuation. Ignored and unanalysed files
have no status dot in the file tree. Opening a file does not change the project
analysis scope.

The admission dialog names full, stale/failed or continuation scope while
preparing, after failure and during review. Review shows the daemon-returned
included and excluded counts, refresh/reuse policy and any saved-run stage
compatibility limit. Expand any included file to inspect its full project-relative
path and every returned stage's eligibility, cache disposition, model-request
allowance and reason. Excluded files retain their full paths and reasons, which
may reflect selection, policy, source eligibility or selective retry; an
ineligible stage on an included file is not a whole-file exclusion. The returned
stage summary separates applicable, cached/reused, request-bearing and
non-requesting work, including deterministic Security rules. Neither an empty
scope nor zero planned model requests proves successful analysis. Empty scopes
still expose exclusions but cannot Start or Resume.

Expected model requests exclude retries; the displayed inclusive maximum includes
them. The per-stage attempt limit includes the initial attempt. The returned file
and time limits bound the dispatch window, not the project inventory or an ETA;
further work needs explicit continuation. The daemon defaults to a 100-file,
900-second window; the desktop does not expose Run limits controls. Review also
shows each returned model destination's scope, profile, model, origin and
local/remote classification, with unavailable metadata labeled rather than
guessed. Reviewing, expanding or copying details sends nothing to a model. On a
preview failure, **Retry preview** explicitly requests the same captured scope,
limits and policy (and the same run for continuation), not a default full run.
If that identity is obsolete, close the dialog and request a new preview; closing
never confirms an action.

Under **Your confirmation**, acknowledge each required remote destination
independently. Checkbox labels include the returned scope, model and provider ID,
so destinations remain distinguishable even when their scopes or models match.
Local destinations remain inspectable without a remote-destination checkbox.
When required, **Include AI Security review** is a separate acknowledgment even
for an all-local preview. AI Security model findings are advisory and unverified,
not a verified scan or safety assurance; leaving this unchecked does not remove
Security stages from the plan. The dialog lists outstanding acknowledgments, and
an empty included-file scope or any missing acknowledgment blocks Start/Resume.

Starting or resuming may send eligible source and project context for the
previewed scope to the listed models under the existing context policy. The
preview describes the plan, not the exact content transmitted. Selecting a
checkbox alone sends nothing to a model. Only **Start analysis** or **Resume
analysis** explicitly admits the current preview. That admission is single-use,
including after a failed response; one admitted run may initiate multiple model
requests. Analysis does not execute project code or modify source files, and
analysis consent does not grant function-edit permission or execution trust.
Admission and other dialogs keep decisions below a scrollable body.

If an admission is rejected with HTTP 409, the old preview and confirmations
cannot be reused. When its captured scope is still valid, **Review fresh
preview** explicitly requests a new preview of that same scope and policy; all
required destinations and Security intent must be confirmed again before Start
or Resume. If the scope or continuation run has become obsolete, Close and choose
a current Analysis action instead. A timeout or transport failure is uncertain:
the run may already have started. The client reads durable run status without
resubmitting admission or offering a consent-reusing retry; Close and check the
current Analysis run before choosing a new action. Closing the dialog discards
transient consent but does not cancel an already admitted run.

Pause/Resume/Cancel retain truthful partial coverage and reported attempts.
Resume requires a fresh preview; startup never silently resumes a model request.
Analysis groups lifecycle, file progress, active paths and applicable run controls.
File totals come from the captured plan; only matching project/revision, queue,
file path/hash and planned-stage records contribute progress. Missing or inconsistent
records show incomplete or unavailable progress, not a percentage; a valid empty
scope is shown separately. Finished files include partial and failed stage outcomes:
finished does not mean successful, current or safe. Expand active paths to inspect
full names, and captured stage rows to see running, pending, finished and attention
breakdowns. Stage details show matching paths, statuses, reported attempts, reuse
and reasons (or a missing-diagnostic fallback). Attempts are not provider-call
counts. Ineligible stages keep their plan reasons without counting as operational
failures. Run failures are visible near status, with bounded diagnostic details.

Analysis labels the reported cumulative run time and, when supplied, the current
dispatch-window time separately; it does not calculate an ETA or infer duration
from timestamps. Created and Updated are labeled as such. A restored terminal run
is the latest saved run. If a different run replaces a terminal run actually
observed in this app session, **Previous observed run** discloses that snapshot's
identity, scope, lifecycle, reported time, timestamps and failures. At most one
previous terminal run is retained in memory; polling or continuing the same run
does not add history. It survives workspace navigation but clears on project
replacement. Older details are unavailable after restart: the current API restores
only the latest saved run, not an archive. Previous runs cannot supply current
progress, result rows or lifecycle controls; revision-mismatched history is marked
outdated.

Bugs, Performance and Security cards beside the overview remain navigable during
active and stopped runs. They distinguish run-reported counts from matching loaded
saved findings; unknown counts are not zero, and zero loaded findings alone does
not prove a completed-empty result. Opening a card only navigates; retained rows
and result-read errors remain visible in their category.

Each page shows all loaded findings for its category. Rounded rows retain severity,
summary and exact source location. Optional disclosures retain evidence and
verification details. The single **Prepare fix** action opens the existing
Assistant workflow only when the current declaration is eligible; row selection
does not prepare a fix. An unavailable or failed report is never presented as zero
findings, and an empty Security result is not assurance. If a saved-result read
fails, **Retry loading results** beside the result status reads saved data for
that category and path only; it does not start analysis. Loading and read errors
remain visible even with retained rows or filters that match nothing. **Clear
filters** changes only the local view; **View analysis** only navigates. A canceled
run needs a new admitted start, while a paused or interrupted run can be resumed
through a fresh preview. Verified Go scans, performance hypotheses, measured
benchmarks, source rules and model hypotheses
retain distinct evidence and execution requirements without routine badge labels.

## New Go functions

Open a Go file, including one containing only a package declaration, and select
**New function** in its header or Context. Assistant focuses the new-name field.
**New Go function** and **New Go type** also remain in Commands. An invalid,
reserved or existing name is rejected before a provider request. Preparing the
composer does not generate or write code. A different active draft requires an
explicit discard choice; canceling keeps that draft intact.

After generation, edit the candidate, validate it, run trusted focused checks and
review the read-only diff. Apply and Undo retain the existing file/hash guards.
Review keeps check diagnostics; Assistant keeps its current request failure;
Analysis and Bugs retain their own operational evidence. The bottom bar shows only
counts of distinct configured local and cloud models, aligned to the right;
select the counts to view model and destination details. Incomplete configuration
is labeled unavailable, not zero. The counts have no hover tooltips.

## Terminal

Explicitly selecting **Terminal** in the rail, selecting the collapsed dock's
**Terminal** control, or pressing **Ctrl+Shift+T** opens or focuses the dock and
may start a local shell rooted in the project if no session exists. Repeating the
rail action does not create another shell while a tab remains; use the expanded
dock's **Terminal** control to collapse it. Shell tabs share the Terminal bar: **+** starts another
shell and **×** closes its tab and process. Selecting tabs, collapsing the dock, changing
workspaces or resizing preserves each shell's process and in-memory scrollback.
When expanded, the dock uses an effective height bounded by the measured space
below the toolbar and above the footer, reserving workspace where possible; a
short window does not change the saved height or automatically collapse the dock.
Confirmed project switching closes every owned terminal tab after review;
application exit cleans up every owned session.

Terminal owns ordinary shell keystrokes, including Ctrl+C. Use **Ctrl+Shift+F12**
to restore app focus. Returning to source/Review rechecks file content and marks
old evidence stale. Use **Re-index project** in the project menu after inventory
changes. Saved dimensions restore with the terminal collapsed, without starting
a shell.
See [terminal support and reproduction](TERMINAL.md).

## Keys

| Shortcut | Context/action |
| --- | --- |
| Cmd/Ctrl+O | Open project… / Switch project… |
| Cmd/Ctrl+1–4 | Summary, Analysis, Bugs, Editor |
| Cmd/Ctrl+Tab | Cycle all workspaces, including Performance and Security |
| Cmd/Ctrl+P | Indexed file search |
| Cmd/Ctrl+Shift+O | Symbols in the current file |
| Cmd/Ctrl+K | Contextual command palette or eligible Assistant composer |
| Cmd/Ctrl+Shift+F | Bugs |
| Cmd/Ctrl+Shift+D | Current draft |
| Cmd/Ctrl+Enter | Generate/cancel when available |
| Cmd/Ctrl+Shift+V / Shift+C | Validate / focused checks when eligible |
| Ctrl+Shift+T | Open/focus Terminal from the application |
| Ctrl+Shift+F12 | Return from terminal input to Editor |
| Escape | Dismiss the top transient surface or cancel the active operation |

In the focused workspace rail group, arrows move focus through the six destinations
without selecting; Enter/Space activates the focused destination. Tab to the
separate utility actions; rail focus and workspace switching do not start a shell.
Use arrows and Enter/Space for other tree/tab/disclosure navigation. The
[keyboard checklist](KEYBOARD_SMOKE_CHECKLIST.md) covers native operation.
`DesktopVisualLayoutTest` exercises production Editor, Summary and results at
several viewports and text scales, with offscreen bounds/reflow assertions. These
captures are not native-window evidence: focus restoration, selection/copy, popup
placement, screen-reader output and real PTY resize still require the native
[keyboard checklist](KEYBOARD_SMOKE_CHECKLIST.md). Do not infer native success from
component tests.

Security entry and selection stay local. Whole-project analysis owns
passive rules and explicitly admitted advisory review. Prepare fix opens the
existing Assistant composer only for current, exact Go declarations; it does not
send a request or change source. No file-scoped start controls or duplicate bottom
Problems/Checks/Output panels remain.

UI work follows [UI_DESIGN_GUIDELINES.md](UI_DESIGN_GUIDELINES.md) for
interaction and accessibility, not for a fixed visual direction. Existing metadata
and visual-preference migration
is documented in the
[root guide](../README.md#existing-projects-and-preferences); model-scope names are unchanged.

### Choose files for project analysis

In **Analysis**, **Files** opens expanded for each project. Search file paths and
use **All**, **Needs attention**, **Up to date** or **Excluded** to inspect the
checklist locally; the match count is against the full returned inventory, and
filter counts follow the search query. Wide rows align File, Analysis state and
Details; at narrower widths or larger text they stack the path, textual state
and details. Paths and diagnostics are selectable, and **Details** reveals full
stage explanations without opening a file. Disclosure, search and filter survive
refresh, saves, lazy scrolling and navigation to another workspace in the same
project. Row details and list position follow paths that remain present. These
local choices reset for a different project and are not saved across app
restarts. File selection is independent of the open Editor file, and inspecting
Files does not start analysis.

The header shows confirmed selected and excluded counts. **Select all** and
**Exclude all** change *every eligible file* in the inventory, regardless of
search or filter matches; individual checkboxes change only their file. Files
excluded by policy or unsupported files show their reason and cannot be toggled.
Build, dependency and metadata folders are omitted from the list. Explicit
selection changes save automatically per project and survive closing the project
or app; newly indexed files start selected, and exclusions for temporarily absent
paths remain saved. Checkboxes and counts show the last confirmed selection until
a save succeeds. Saving selection does not start analysis or rewrite an existing
run; explicitly preview and start a new run to use it.

The file list shows saved analysis state and current progress from an admitted
active/paused run with matching project revision, plan identity and run/plan file hash.
Running and Pending describe that run; Finished does not upgrade cached results
to Up to date. Saved freshness and complete stage reasons remain in **Details**,
with stale/failed/unavailable saved state visible beside current progress. Stages
the admitted plan marks ineligible are not operational failures. Finished runs
return to saved-state presentation. Use **Needs attention** to see missing, outdated, failed,
or incomplete files. Unchecked files and files excluded by configuration appear
under **Excluded** and do not count as up to date or needing attention. Re-selecting
a file restores its saved analysis status. During active, paused or interrupted
runs, checkboxes are disabled, bulk selection controls are hidden, and the lock
reason stays visible even when Files is collapsed. Search, filters and details
remain usable. **Refresh files** explicitly reloads the saved checklist without
starting analysis and remains available while locked (unless a selection read or
write is already in progress). Initial loading or a failed first read does not
imply an empty inventory; with a prior confirmed selection, read/save errors keep
it visible with a diagnostic. After a failed or uncertain save, **Refresh files**
reads back the server's selection; it does not retry the edit automatically. Bugs,
Performance and Security boxes use tinted surfaces like Summary, with text states
for completion, partial coverage or failure independently of finding counts.
A reported zero on an Analysis card remains unconfirmed until matching saved
category details load; a failed detail read is labeled separately from run status.
Changing the selection does not rewrite a previous run.

Summary's overall status and coverage follow the current selected files, including
selection changes. The saved project description keeps its own freshness in the
status details; an older description does not mark current file analysis outdated.
