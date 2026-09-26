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

Summary starts with the project name, purpose and indexed metadata. A segmented
coverage bar shows the current selected files: up to date, outdated, not analyzed,
running, failed, incomplete and unavailable. Zero-size segments are omitted;
unavailable coverage and an empty selection keep separate labels. **View analysis**
only opens Analysis. The status retains current run lifecycle and exposes saved
project-description freshness/failure details on hover or keyboard focus.

Three named Bugs, Performance and Security cards open their result pages with one
click or keyboard activation. Card counts come from run-reported category progress,
not necessarily loaded details; a dash means the count is unavailable, not zero.
A reported zero is not labeled as no findings until matching completed details load.
Detail loading and read failures remain labeled beside the count independently of
run status. Zero findings use a neutral surface and do not assert that a project
is safe. Bug priority counts require matching current details. Overall tool-reported
issues and AI suggestions appear separately below the cards, since those totals
cannot be reliably assigned to individual categories.

At readable local widths, Architecture and flat Packages / modules rows occupy
the left column; Engineering insight and Flows occupy the right. On a narrower
Summary pane or with larger text, the category cards and narrative columns stack
and remain reachable by scrolling. Analysis category panels also stack when their
local width cannot fit readable columns.

Module names, exact paths and responsibilities remain selectable/readable. Entry
points and next steps are omitted from Summary.

Architecture and Flows place **Show diagram** beside the section heading. Diagrams
start collapsed, with the button disabled when a diagram is unavailable. Mermaid
flowcharts and sequence diagrams render locally; expanded diagrams provide zoom
and selectable Mermaid source. No browser, network request, provider call or
project-code execution is needed to display a diagram.

New Analysis results request Mermaid diagrams. Older prose reports remain readable and
are marked stale after the prompt update; run Analysis explicitly to replace them.
Previously saved arrow chains also render as Mermaid. Invalid or unsupported diagrams
keep their source visible with an error instead of an invented diagram.

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

Use **Start analysis** to refresh model analysis for every included project file,
or **Analyze stale & failed** to include only files with stale or failed analysis.
Both actions preview their scope before starting. Start bypasses saved model
results; selective retries can reuse fresh stages. Unchanged deterministic
Security rules can reuse their saved results. Resume continues the admitted run
with its original refresh choice and retained progress. Ignored and unanalysed
files have no status dot in the file tree. Opening a file does not change the
project analysis scope.
The preview shows exclusions, stage eligibility, cache use, expected requests and
inclusive retry bounds. Confirm each displayed
remote destination and explicit Security review intent. Each confirmation is a
checkbox; selecting it alone does not start the run. Admission and other dialogs
keep their decisions below a scrollable body, and closing them does not confirm
an action. One admission coordinates the existing specialized producers; it can make multiple model requests.

The daemon retains its default 100-file/900-second dispatch bounds; the desktop
does not expose Run limits controls.
Pause/Resume/Cancel retain truthful partial coverage and cumulative attempts.
Resume requires a fresh preview; startup never silently resumes a model request.
Analysis groups file progress, active paths and the current run controls in one
rounded panel. Finished-file counts include failed/partial stages and do not imply
successful analysis. Unknown file totals stay unavailable; no ETA is inferred.
Stored elapsed time and previous-run details remain available alongside failures.
Named Bugs, Performance and Security cards retain live coverage and unknown counts;
clicking a card only navigates.
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

In **Analysis**, **Files** opens expanded for each project and can be collapsed
locally. Search and state filters remain usable during a run. The current table aligns File, Analysis state and Details. Its arrangement may
change in a future responsive design.
The footer counts matching files against the full list. **Select all** and
**Exclude all** apply to all eligible
files, regardless of the search filter. Changes save automatically per project
and survive closing the project or app. Newly indexed files start selected;
ignored paths remain saved even if temporarily absent.

Excluded or unsupported files show their reason. Build, dependency and metadata
folders are omitted from the list. Use **Refresh files** to reload the checklist.
Finish or cancel an active/paused run before changing its files; start a new
analysis to use the saved selection. Existing results remain available.

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
reason stays visible even when Files is collapsed. **Refresh files** remains
available; load/save errors retain the confirmed selection. **Refresh files**
reads the confirmed selection again, not an unsaved failed edit. Bugs,
Performance and Security boxes use tinted surfaces like Summary, with text states
for completion, partial coverage or failure independently of finding counts.
A reported zero on an Analysis card remains unconfirmed until matching saved
category details load; a failed detail read is labeled separately from run status.
Changing the selection does not rewrite a previous run.

Summary's overall status and coverage follow the current selected files, including
selection changes. The saved project description keeps its own freshness in the
status details; an older description does not mark current file analysis outdated.
