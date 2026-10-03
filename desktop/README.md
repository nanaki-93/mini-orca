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
Component tests do not prove OS focus, popup placement or screen-reader behavior;
interactive app testing is not an acceptance requirement.

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

Summary starts with the project, analysis status and three result categories.
**File evidence**, **Project details** and **Editing guide** start collapsed;
expand them locally when needed. Their state survives scrolling and resets when
the project or revision changes. Errors, selection warnings and run controls
remain visible without opening these sections. Long project descriptions use
**Show full response** to reveal the complete selectable text; the expansion
survives scrolling until the description or project changes.

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

Expanding **File evidence** below the cards shows up to three project-relative
paths, sorted by path, with their saved analysis states and explanations. Its
“Showing N of M selected files” label counts the confirmed selection, excluding excluded files;
active-run progress and pending selection edits do not change these saved rows.
Loading, saving or a failed selection read/save can retain the last confirmed
selection with a visible notice. A confirmed empty selection is labeled separately
from unavailable selection. Aggregate-only saved coverage can show counts in the
dial but cannot provide file paths, so File evidence says paths are unavailable.
**All files** opens Analysis for the complete file inventory without changing the
selection or opening a source-editing target.

Inside **Project details**, Architecture and Engineering insight sit beside each
other at readable local widths and stack at narrower widths or larger text. A lone
available panel uses the full width. Packages / modules and Selected findings sit beside
each other where both columns are readable, stacking in that order at narrow
widths or larger text. Without modules, findings use the full width; absent
narrative sections leave no empty cards. Insight retains its lead content and
**More insight** disclosure. Module names, exact paths and responsibilities
remain selectable/readable. Entry points and next steps are omitted from Summary.
**Selected findings** appears after narrative, beside or below modules (if
present), before the project's saved Flows (if present) and the separate
Editing guide. It remains available inside Project details without narrative or
modules. It shows up to five loaded Bugs, Performance and Security results, with category,
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

The separate **Editing guide** opens the **Change lifecycle** section describing
Mini-Orca's editing workflow, not the analyzed project's Flows:
Request → Draft → Validate → Checks → Review →
Apply. Only the isolated declaration/import draft is editable; Apply explicitly
changes one file under guards, and Undo is available only while its guards hold.
The stages are information, not actions or readiness indicators. **Open Editor**
only navigates to Editor; it does not change the draft or run any editing step.

After expanding Project details, Architecture and each project Flow show a
bounded, locally rendered preview of saved Mermaid flowcharts or sequence
diagrams. **Expand diagram** opens a local viewer without leaving Summary.
It provides two-axis scrolling, 75–200% zoom in
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
current text scale. Below that boundary the editor appears first in a vertical
scroller, followed by Files and the tools pane. The fixed **Editor**, **Files** and
**Tools** shortcuts scroll directly to each visible pane and support keyboard
activation. The source/diff canvas fits the visible area; its header scrolls
separately when space is tight. Hidden panes stay hidden. Source and composed diffs
remain selectable/read-only; only the isolated draft is editable. Tabs, draft actions and
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

Context shows the selected declaration's name, project-relative path, bounded
source range, language, confidence/eligibility and selectable signature. Source/index
correspondence compares the loaded file with the matching indexed hash; it is
separate from saved file-analysis freshness and does not monitor disk changes.
On-demand explanation has distinct unavailable, loading, current, stale, canceled
and failed states beside its content and recovery action. A matching saved
file-analysis description is labeled separately, not treated as an on-demand
result. **Explain**/**Refresh** shows the Function model destination and requires
current Function-scope confirmation for a remote provider; **Cancel explanation**
stops a running request. **Refactor** prepares the existing composer without
sending or changing source. Declaration details, file details, project context and
read-only references are local disclosures. References are a file-scoped advisory
indexed preview, not a verified caller list; unavailable, mismatched and empty
previews are distinguished. Without a selected declaration, **Actions** and
**Details** retain file actions and context. Selecting a declaration, opening a
tab or expanding details never requests an explanation, runs project code or
writes source.

### Assistant requests and conversation

Assistant names the current project-relative file, declaration and operation before a
request. For an exact Go function or method, **Fix**, **Refactor** and **Document**
prepare short, unsent intents; other eligible declarations can use freeform intent.
Complete the labeled **Intent** (or creation **Behavior**) field with a specific
request. A preset lead alone, blank intent or constraints alone cannot Send.
**Advanced constraints** is optional and starts collapsed; it is appended to a
valid request when supplied. Preparing a preset, opening constraints or navigating
does not contact a provider or replace a draft. Press Enter to add a line; use
**Send message** (or **Generate function/type** for creation) or Cmd/Ctrl+Enter
to request one isolated candidate. **Cancel request** stops the current local
request without clearing an existing draft. A timed-out request needs another
explicit Send; local cancellation cannot guarantee cancellation at the provider.

Review the Function scope, model and provider origin before Send. A remote Function
destination needs its own confirmation: Analyze/Security confirmation and project-code
execution trust do not authorize it. Inspect context is a local, file-scoped
preview, not confirmation or the exact declaration payload. Sending generates a
candidate in the editable declaration/import draft; it never writes source. Use
validation, focused checks, Review and an explicit **Apply change** to edit the
source under the existing guards.

The separate **Conversation** keeps daemon-backed requests and model responses in
order, along with submitted requests that are still running, failed or canceled.
Failure details stay beside the originating request when you retry; they do not
replace previous successful turns or erase the draft. Model prose is selectable
and can be expanded to its full available text; model-supplied links, images and
HTML are inert. Retained history from an earlier declaration/revision is labeled
with its original scope and does not authorize work on the new target. History
belongs to the current project/file session, not a persistent global archive.
Changing a target with a draft still requires explicit discard approval.

### Inspect Assistant context

In Assistant, **Inspect context** opens a read-only dialog immediately for the
selected project-relative file and Assistant intent. It makes one local daemon
file-context preview request; Fix, Refactor, Document and creation intents use the
supported `fix` preview action. Without a project or selected file, the dialog
explains why inspection is unavailable instead of requesting a preview. Loading,
failed, stale and canceled inspections are labeled separately from a successful
empty manifest. **Retry** replaces the displayed result only if its captured
target still applies; **Cancel** stops a loading inspection and keeps the dialog
open with a canceled state and **Retry**. **Close** and Escape dismiss without
sending. A changed target or model destination cannot silently attach an old
response to the new selection. Dismissal returns keyboard focus to the
initiating control when it survives, otherwise to a safe workspace control.

A ready preview shows the returned destination, estimated tokens, supplied
positive limits, truncation, and every included file's reported size, estimate,
hash and truncation alongside every excluded path and supplied reason. Missing
metadata is labeled unavailable; inclusion reasons are not supplied. Values and
diagnostics are selectable in the scrolling dialog. This is a **file-scoped
preview**, not the exact declaration prompt: a later declaration request may
compose different context and additional prompt material. Inspecting is local;
it does not contact a model provider, grant Function consent, run code or write
source. **Send** and **Explain** still require their own authorization, including
remote-provider confirmation when applicable.

A current draft must be discarded explicitly before changing its target. In
Assistant, **Editable draft** shows the project-relative target path, declaration
name and replacement/new-declaration scope (Function or Type when known), the
server draft revision, and a separate generated, locally edited, validating,
validated, invalid or stale status. Only the isolated declaration and **Required
imports** fields are editable; source and composed diffs remain selectable and
read-only. The import field stays available when empty. Enter comma-separated
import paths; spaces, trailing commas and partial entries remain in the field
while editing, with nonblank entries trimmed for explicit validation. An empty
list adds no required imports; it does not remove imports from the source file.
Plain Enter in the declaration inserts a line, not a validation request.

Editing either field invalidates validation/check readiness, even if the import
list normalizes to the same entries or the original text is restored. Moving the
caret or changing selection does not invalidate evidence. **Validate draft**
explicitly sends the candidate for validation; validation alone neither runs
checks nor writes source. Retained diagnostics are labeled as earlier evidence,
not approval of the current candidate. **Discard draft…** opens a confirmation;
**Keep draft**, Escape or dismissal preserves both inputs and evidence. Confirmed
discard clears the current in-memory draft/conversation and focused checks without
writing source; a confirmation for an older revision or edited buffer cannot
silently discard newer work. Review shows target identity, readiness,
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

Analysis groups lifecycle, file progress, active paths and applicable run controls.
For a queued or running run, **Pause** and **Cancel** request a control; while the
request is in flight the run says “Requesting pause…” or “Requesting
cancellation…”, not Paused or Canceled. Once the daemon accepts Pause, “Pause
requested” means the current stage can finish but no new stage will start;
**Cancel** remains available while pausing. An accepted cancellation says active
work is stopping: Cancel stops active requests and future dispatch, but keeps
completed evidence. The daemon may report a settled state immediately or finish
work before a control takes effect; follow the reported status rather than
assuming the requested outcome. Stopped runs show the reported stop reason beside
recovery, or say when no reason was supplied. Full diagnostics remain under
**Run diagnostic**.

**Resume → fresh preview** is available for a current paused or interrupted run,
including a run stopped at its dispatch limit. It requests a new continuation
preview; review its scope and give fresh destination and Security confirmations
as applicable before **Resume analysis** in the admission dialog. Neither
startup, navigation nor closing a dialog resumes a run or reuses consent. A
canceled run has no Resume: use **Start new analysis** to request a new preview,
then explicitly admit it with the required confirmations. Completed evidence
and reported attempts remain available after Pause or Cancel. The run shows
**Cumulative attempts reported** when its captured file-stage inventory is
complete and valid; otherwise it says **Cumulative attempts unavailable**.
Each file-stage contributes once (including shared semantic work); attempts are
reported accounting, not findings or a count of billable provider requests.

If Pause or Cancel is rejected, the run shows the rejection and reads durable
status without repeating the control. If its response times out or transport
fails, the outcome is unconfirmed: the client reads status rather than assuming
success or retrying. If that status read fails, the last accepted snapshot and
evidence remain visible but cannot authorize Start or Resume. Use **Refresh
status** beside the run to explicitly read status again; it does not resend a
control or admission. **Refresh files** separately reloads the saved file
checklist, not run status. Control/status errors stay on the run surface, while
preview or admission failures retain their admission-dialog recovery actions.
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
not prove a completed-empty result. **About counts** reveals the count explanation
without changing the visible cards or starting work. Opening a card only navigates;
retained rows and result-read errors remain visible in their category.

Each result page shows all loaded findings for its category. Search by title or
path and filter by severity (including unknown severity); **Clear filters**
restores the local view. Selecting a row inspects evidence only. Bugs rows show
severity, title and location; details retain the full title, exact path/line,
message, evidence origin and material state. **Evidence and fix criteria** expands
additional evidence, task criteria, non-goals and engineering insight. Stale,
partial and canceled evidence remains labeled; model suggestions are not verified
tool reports. Bugs details offer separate **Open source** and **Prepare fix**
actions. Open source navigates to the indexed path and supplied source line for
read-only inspection, even when a fix task is absent or the finding is stale; a
missing indexed path disables it with an explanation. It does not prefill Assistant.
Prepare fix requires a current finding with a reviewed task, matching indexed
source hash and one exact eligible Go declaration. Its blocked reason appears in
details. An eligible preparation opens the declaration in Editor and prefills
Assistant without sending a model request, running checks or writing source.

Opening a different source target or preparing a fix while a draft or Assistant
input would be lost asks for explicit discard confirmation. Cancel or Escape
keeps the current target, draft and input; confirm only for the unchanged finding
and draft. Same-target source inspection retains the draft. If the finding, project
or input changed while confirmation was pending, choose the action again. A
failed source read cannot prepare the request. Triage is a separate explicit
action, never a consequence of row selection. An unavailable or failed report is
never presented as zero findings, and an empty Security result is not assurance.
If a saved-result read fails, **Retry loading results** beside the result status
reads saved data for that category and path only; it does not start analysis.
Loading and read errors remain visible even with retained rows or filters that
match nothing. **View analysis** only navigates. A canceled run needs a new
admitted start, while a paused or interrupted run can be resumed through a fresh
preview. Verified Go scans, performance hypotheses, measured benchmarks, source
rules and model hypotheses retain distinct evidence and execution requirements
without routine badge labels.

### Verified Go scan

Bugs contains a separate **Verified Go scan** tool section. Starting is available
only for a detected root Go module with a current, nonblank project identity and
revision, confirmed scan absence or a known terminal report, and no unresolved
scan operation. Other languages, unknown project types, missing identity and
unread/unavailable/unknown status have a disabled action with an adjacent
explanation; retained diagnostics remain
inspectable. The daemon reports toolchain, platform and execution failures.

Before activation, the section shows project ID/revision and whole-project scope,
independent of the **Analysis → Files** selection:

- Parser inspection of indexed Go source (`parse`), with no subprocess command.
- `go vet ./...` (`vet`), argv `["go", "vet", "./..."]`.
- `go test ./...` (`tests`), argv `["go", "test", "./..."]`.

There is no formatting phase. Commands run in a temporary copied workspace; the
scan workflow does not edit original source. Tests and package initialization can
execute project code, however: **a copied workspace is not a security sandbox**
and does not prevent arbitrary effects from that code. Only run a project you
trust.

**Trust project-code execution & run checks** explicitly reads execution scope,
grants current-daemon-session trust for this project/revision, then starts the
scan. The client checks both trust responses against the current identity and
exact project-code argv `["go", "test", "./..."]`, and requires a trusted
acknowledgment before starting. Changed identity or scope blocks subsequent
requests. Trust is in memory and clears on project replacement, restore or
re-index. It is separate from model/provider confirmation and from Review/Apply
authority; scans do not request a model. Import, restore, re-index,
Bugs navigation, finding selection, diagnostic disclosure and copying do not
grant trust or start a scan. The [API contract](../docs/api-contract.md#live-routes)
owns the execution-trust and scan routes.

**Starting** appears immediately, even with an older completed report retained;
repeated activation cannot start another pending scan. A running scan offers
**Cancel checks**. **Cancellation requested** means the request is pending or the
daemon still reports active work, not that cancellation succeeded. Only a matching
terminal report settles the outcome; it may finish before cancellation takes
effect. A failed or timed-out start says **Start unconfirmed** if execution may
have begun; a failed cancel says **Cancellation unconfirmed**. Neither retries
execution automatically or erases prior evidence.

Use **Refresh scan status** after a read/operation failure or uncertain outcome.
It reads scan status without granting trust, starting again or resending Cancel;
a matching active report resumes polling. It is unavailable during a status read,
Start or pending cancellation. Unread, confirmed absence, unavailable status and
failed live polling stay distinct. If cancellation polling fails or ends without a
matching terminal report (including absent, unknown or foreign status), cancellation
becomes unconfirmed and status recovery is available; it does not resend Cancel.
Read failures remain visible beside cancellation or unsupported-project explanations.
Accepted terminal reports refresh tool findings; enrichment failure retains
diagnostics and prior rows with a visible unavailable/stale warning rather than
claiming current or zero findings. Foreign
or obsolete responses cannot replace current scan evidence.

**Command and output** reveals every returned phase in order, including
`workspace` failures and unknown phases, with its reported state, command and
selectable, read-only sanitized output. **Completed** describes the report
lifecycle, not a pass for every phase. Failed, skipped and canceled phases retain
their labels; missing phases have no reported outcome, and **No output reported
for this phase** does not mean Passed. The UI shows nonzero exit codes for failed
or canceled command phases, not an invented parser/default-zero success.
**Show full available output** expands the UI preview to all recorded sanitized
text; **Daemon output limit reached** means omitted text is unavailable
and cannot be recovered by expanding. Disclosure resets on project/revision or
report replacement. Scope, trust warning, status, failures and recovery remain
outside optional diagnostics. Tool evidence is scoped to these checks, distinct
from model suggestions, and is not a general safety or bug-free assurance.

### Security evidence and advisory review

Security uses the shared results browser for loaded source-rule matches, model
hypotheses and semantic findings. Search by title or path, filter by severity and
select a row to inspect it; these local interactions and **Report metadata** do
not scan, prepare a fix or request a model. Typed rows label **Source rule** for
deterministic rule matches, **Model hypothesis** for AI suspicions and **Evidence
type unavailable** for other source/evidence pairings. A rule match identifies a
pattern, not a confirmed vulnerability; a model hypothesis is unverified advice.
Supplied verification state and confidence do not independently verify either.

Typed details retain the complete title and path, supplied positive range and
symbol, status and freshness, observed condition, remediation, preconditions,
safe verification idea (not performed), rule/category, confidence, CWE/reference,
triage and verification state, and engineering insight. **Report metadata**
expands scope, source hash, ruleset and supplied model/profile/provider/version/time
fields; missing values are labeled rather than guessed. Report failures and
blocked-action reasons remain visible with the disclosure collapsed. Failed,
partial or unavailable report reasons remain visible even without finding rows
and alongside another report's findings; saved-result read errors can retain old
rows. **Retry loading results** reloads saved category data, not analysis. A
completed empty report does not establish safety, and missing AI evidence is not
zero model hypotheses. Stale and incomplete coverage retain their labels.

On a typed finding, **Open source** is independent of **Prepare fix**: it opens
the one indexed path at the supplied positive line for read-only inspection,
including retained stale evidence, without Assistant prefill. An invalid or
ambiguous target reports why it cannot open; no nearby declaration is substituted.
Inspecting the already selected file preserves its draft. **Prepare fix** is
available only for a uniquely loaded, current completed or partial report with
matching nonblank indexed source hash and one exact eligible Go declaration and
range. Its disabled reason explains missing/stale evidence or an unsupported or
inexact target. After activation, loaded source and symbols must still match the
captured file, hash, declaration and selection before Editor/Assistant receives
an unsent request based on the supplied evidence. Failed or obsolete reads cannot
prepare it; neither action runs checks or writes source. Target-changing actions
that would replace a draft or Assistant input ask for explicit discard approval;
Cancel/Escape keeps that work, and changed result, project, draft or input
invalidates the approval. Only successful admitted transitions replace input.

**Review Security intent** opens Analysis locally, without previewing, scanning
or starting a run. AI Security review belongs to the whole-project Analysis
Start/Resume preview and admission dialog, not a Security-only dispatcher. Review
the returned file/stage scope and destinations there, confirm every required
remote destination and the separate **Include AI Security review** intent when
required (even with local models), then explicitly **Start analysis** or
**Resume analysis**. Changing the preview or resuming requires fresh admission.

### Performance hypotheses and benchmark evidence

Performance uses the shared result browser for loaded typed opportunities and
semantic findings. Search title/path, filter by impact and select a row to inspect
it without preparing a fix. Typed opportunity details show the full relative
path, supplied positive line and symbol, report status/freshness, observed pattern,
qualitative potential impact, model confidence (not a measurement or speedup
probability), recommendation, workload conditions, trade-offs, verification plan,
warning and engineering insight. Absent evidence fields are labeled rather than invented;
**Report metadata** expands the saved profile/model/provider and report identity.
A partial or stale report retains its warning. These model recommendations are
unmeasured; a candidate benchmark comparison does not measure the selected
hypothesis.

On a typed opportunity, **Open source** inspects the one indexed file at its
supplied positive line, including from retained stale results; a missing line
is not replaced with an unrelated declaration. It does not prepare Assistant.
If the file is missing or ambiguous in the active index, the action reports a
failure. **Prepare fix** is a separate action: it requires current completed or
partial evidence, a matching nonblank source hash, and one exact eligible Go
declaration whose indexed anchor still matches the loaded file and symbols.
The disabled action shows a specific blocked reason when preflight fails. A
successful preparation opens Editor/Assistant with a behavior-preserving request
using the supplied pattern, recommendation, workload, trade-offs and verification
plan; it does not send, run checks or write source. Failed or obsolete reads
cannot prepare the request. Changing targets with a draft or Assistant input
requires discard confirmation; cancel/Escape preserves them, and an approval
is invalid if the opportunity, project, draft or input changes. Same-file source
inspection retains the draft. Semantic Performance rows use the shared finding
details and task-based preparation eligibility. Their separate **Open source**
action opens the indexed path and supplied line without preparing Assistant; if
the path is unavailable in the index, the control is disabled with a reason.

**Explore benchmark evidence** opens a local disclosure, without a candidate or
catalog request. Discovery requires an open project, selected file and applicable
validation for the exact current draft. Draft/editor text, project/revision,
target path and base file hash must still match the open file. Missing, edited,
validating, invalid or stale candidates show a blocked reason; listing does not
automatically validate them.

**List compatible benchmarks** explicitly requests the read-only daemon catalog;
**Refresh compatible benchmarks** repeats discovery after a lookup. Listing shows
**Listing · read-only discovery**, not execution, and suppresses duplicate lookups.
Refresh immediately clears the old catalog and selection. All returned choices
remain in daemon order, with none selected automatically; explicitly select one
again after reviewing each refreshed catalog. Selection is local and must match
the exact returned choice, including argv and scope. Empty, unavailable and failed
lookups keep their own explanations and allow explicit Refresh when the candidate
is eligible. An invalidated catalog or stale selection requires fresh discovery
and selection. After a stale-draft conflict, refresh the source and validate the
current draft before listing again.

Before running, inspect **Selected benchmark**, **Project ID**, **Project revision**,
**Target path**, **Validated draft revision**, **Package working directory** and
**Opaque scope guard (identity metadata)**. The working directory is relative to
the project root (`.` means the root); the opaque scope is an identity guard, not
a directory. **Daemon-returned argv (read-only)** shows every argument separately
as a JSON-quoted `argv[index]` entry, preserving argument boundaries rather than
constructing a shell command. **Copy argv** copies those entries; path and scope
text are also selectable/read-only. An incomplete name, scope or argv blocks
execution with a reason. Required scope, warnings and recovery remain outside
**Report metadata** and **Measurement details**.

**Trust and run selected benchmark** first reads execution trust, then grants it
only for matching project/revision and the exact trust contract `go test ./...`;
the client requires a matching trusted acknowledgment before requesting the
benchmark. This is current-daemon-session project/revision trust, **broader than
benchmark-only permission** and separate from provider consent and Apply authority.
Granting trust does not itself execute `go test ./...`; the combined action requests
the selected benchmark separately. **Run selected benchmark** uses catalog-reported
trust without silently renewing it. Benchmark execution runs imported project code
in baseline/candidate copies and may have external file or network effects:
**copied workspaces are not a security sandbox**. Only run code you trust.

Admission shows **Admitting · execution trust** / **Checking execution trust…**
separately from **Running · explicit local execution** / **Comparing benchmark…**.
Repeat execution and discovery controls are disabled while admission/comparison
is active. Project, file, draft, validation or choice changes invalidate obsolete
work; a changed choice never runs automatically. Each request stage and result
publication rechecks current identity. A mismatched or untrusted trust acknowledgment,
scope change or daemon-reported trust expiry revokes reusable selection: explicitly
Refresh and select again, then review trust before admitting another run. Admission
request failures remain visible and never proceed to comparison or retry execution
automatically. A comparison timeout may mean execution already started, with no
new measurements confirmed. Local stopping/invalidation does not prove the daemon
canceled code that already began.

The latest comparison status and its reason remain visible beside **Explore
benchmark evidence**, even with optional details collapsed. Daemon outcomes use
**Comparison canceled · daemon**, **Comparison failed · daemon**, **Comparison
unavailable · daemon** or **Comparison unsupported · daemon status**; they are
not trust-admission failures or successful empty measurements. **Completed · no
new measurements** means neither side returned a nonempty sample list. A completed
response with measurements is assessed separately: completion alone is not an
improvement. Admission failures, local stopping and transport uncertainty remain
distinct from daemon-confirmed outcomes. Unavailability clears catalog/selection
authority; explicitly refresh and select again before another run. No outcome
retries execution automatically.

**Measurement details** opens **Benchmark evidence** for the candidate and
selected benchmark, not a measured gain for a model hypothesis. It shows **Metric**,
**Baseline median**, **Candidate median** and **Change / availability** for
**Time (ns/op)**, **Bytes (B/op)** and **Allocations (allocs/op)**. **Baseline samples**
and **Candidate samples** count each side's actual returned samples, independently
of per-metric valid-observation coverage. At narrower local pane widths or larger
text, the columns become stacked metrics with the same Baseline/Candidate median
labels; no metric is hidden.

Availability distinguishes **measurement not returned**, **empty samples**,
**unavailable**, **partial**, **invalid samples** and **complete**. Omitted/null
bytes or allocations are unavailable, never zero; recorded zero remains zero.
Valid observations remain readable as medians even in partial or invalid evidence;
for an even number of valid observations, the median averages the middle pair.
Complete comparison requires five valid samples per side for all three metrics.
Iterations and time must be positive, time finite, and present memory values
nonnegative. Invalid observations remain inspectable but prevent a complete claim.
Incomplete evidence has no observed-change claim for the affected metric, and a
lower time median alone cannot establish a win with incomplete memory evidence.
Zero-baseline changes use text such as “from zero to …”, not infinite percentages.

**Inconclusive · noisy samples** identifies relative range greater than 10% of
the median; **Inconclusive · opposing memory signals** identifies conflicting
bytes/allocation changes. Lower time with higher memory is an explicit trade-off,
not an unconditional win. Limitations sit beside the observations, and essential
inconclusive/stale reasons remain visible when measurement details are collapsed.
**Measured · selected benchmark** is scoped to that benchmark, not project-wide
performance or the selected recommendation.

During a replacement attempt, discovery/admission/running status takes precedence
over earlier measurements; a later daemon failure, cancellation or unavailability
stays primary. Earlier measurements remain under **Prior measurement details** /
**Prior benchmark evidence** when the new response supplies none. Refresh, cleared
selection, changed argv/scope or candidate/source identity removes current claims;
stale recorded values remain inspectable with their reason and **Historical
change** labels. Current claims require the eligible current candidate and exact
catalog selection, matching full draft/source identity, benchmark, scope and argv.
Retained evidence belongs to the current review lifecycle, not persistent history
or another review's candidate. It cannot authorize another run.

When the latest response contains no measurements, **Latest response details**
opens **Latest comparison response** separately from any prior measurements.
It retains the **Daemon status** and **Daemon reason**, with its own recorded
identity/argv and sample details; a missing measurement is not an empty sample
list. **Copy displayed response details** copies only that response's displayed
fields, including optional sections while expanded, never prior measurements.

Inside measurement or latest-response details, **Recorded conditions & identity**
exposes the comparison's benchmark, target path, project ID/revision, draft ID/revision/hash,
base file hash and opaque scope guard. **Recorded -count**, **Recorded -benchtime**
and **Recorded -benchmem** derive only from the returned command (separated or
`-flag=value` forms). **Recorded comparison argv (read-only)** shows all indexed
JSON-quoted arguments, not a replacement selection's command. Missing metadata
says **not recorded**; no machine, toolchain, timestamp or workload-size facts are
invented. **Returned sample details** exposes each side's returned sample count,
iterations, time, bytes and allocations, preserving unavailable versus zero.

Evidence text is selectable/read-only. **Copy displayed benchmark evidence**
copies its heading, assessment, median/availability labels and displayed evidence;
the **Recorded conditions & identity** and **Returned sample details** sections
are included only while expanded. Expand both for a full evidence copy, then paste
into a scratch document to inspect it. Success says **Displayed benchmark evidence
copied.**; failure says **Could not copy benchmark evidence: …** locally, without changing
the workflow outcome. This differs from **Copy argv**, which copies the currently
selected catalog command, not recorded comparison evidence.

Benchmarks remain optional evidence, not an unconditional Apply prerequisite, and
do not bypass existing Review/Apply/Undo guards. Navigation, finding selection,
disclosure, copying and resizing do not list benchmarks, grant trust, execute code,
contact a provider or write source. No new API, configuration or data migration
is required.

Ordinary automated tests use fake transport or action counters, without a running
daemon, provider or project-code execution. Component fixtures qualify rendering
and local interaction, not native-window behavior.

## New Go functions and types

Open a Go source file, including a package-only file, and choose **New function**
in its header or Context, or **New Go function** / **New Go type** in Commands.
Assistant opens with the new-name field focused. Use **Declaration kind** to
switch between Function and Type without losing the name, Behavior or optional
constraints. A conflicting draft requires explicit discard confirmation; keeping
it preserves the draft and input. Preparation, switching kind and inspecting
context do not generate a candidate or change source.

Enter a new name and meaningful **Behavior**, then explicitly **Generate function**
or **Generate type**. Names are trimmed; Go identifiers including Unicode letters,
`_helper` and `any` are allowed. Empty, malformed, keyword, workflow-reserved
(`_`, `init`, `main`) and already reported names in the current file are rejected.
Valid-name feedback refers only to the loaded *current file snapshot*; the daemon
rechecks the name and file identity before generation. It does not promise
package-wide uniqueness or a current-disk check. Unsupported files and busy
requests show why creation is unavailable. A remote Function destination requires
its own confirmation before sending; constraints alone do not replace Behavior.
Cancel retains existing drafts and request history. A successful response creates
an isolated editable declaration/import draft, not a validated or applied edit.

Edit the candidate, validate it, run trusted focused checks and review the read-only
diff. Apply and Undo retain the existing file/hash guards.
Review keeps check diagnostics; Assistant retains failures and cancellations beside
their originating requests within the current file conversation; Analysis and Bugs
retain their own operational evidence. The bottom bar shows only
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
Use arrows and Enter/Space for other tree/tab/disclosure navigation.
`DesktopVisualLayoutTest` exercises production Editor, Summary and results at
several viewports and text scales, with offscreen bounds/reflow assertions.
These captures establish only the component behavior tested, not OS focus,
popup placement, screen-reader output or real PTY behavior.

Security selection is local; its source, preparation and whole-project review
routes are described [above](#security-evidence-and-advisory-review). No
file-scoped start controls or duplicate bottom Problems/Checks/Output panels
remain.

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

Validation is an explicit stage in the isolated draft. It shows the server revision
(and any local edits), current or retained diagnostics, and **Cancel validation**
while a request is pending. After validation, **Review focused checks** opens Review
without executing checks. Validation composes a candidate in memory; it does not
run tests or write source. A response for different candidate content cannot approve
the draft, and a changed source identity blocks current validation evidence.

Review keeps Validation, Focused checks and Source unchanged together. Check details
and full available output are local disclosures; rerunning requires its own explicit
action even after a pass. A generated focused test shows its exact command and draft
scope before **Trust local execution & run checks**. Trust must still match the
project revision and candidate when execution starts. Missing or skipped check rows
are labeled without claiming readiness; failed attempts retain prior output without
reusing it as current approval.

After a failed focused check, **Revise with check output** explicitly sends a bounded
repair request for the same candidate and pinned task. Review shows the next attempt
out of three; current Function edits destination consent is still required. **Edit
draft manually** remains available. A stale or edited candidate, canceled checks,
a different conversation or an exhausted limit blocks repair. A failed or canceled
request keeps the draft and useful diagnostics; a returned revision needs new
validation and checks.

The read-only Review comparison identifies the candidate declaration, server revision,
file and requested imports above Current/Candidate. Its scope area scrolls separately
when paths or imports are long, leaving both code panes available. Short canvases
keep the full scope in a local **Candidate scope** disclosure beside the comparison
controls. Review breadcrumbs
follow the retained candidate, while Source keeps the inspected declaration. Switching
Side-by-side/Unified is local; only the isolated draft in Assistant is editable.

Apply keeps the exact candidate scope and revision in the decision region, including
when validation, checks or source identity block it. An explicit Apply starts one
guarded request; a second activation while it is pending sends nothing. The running
state waits for a daemon receipt. Failure and conflict diagnostics stay beside the
action; a conflict marks the draft stale and preserves it for recovery.

A returned Apply receipt records its submitted declaration/file and the daemon's
project revision, resulting hash and supplied audit details. **Refresh source**
retries only the source read. Undo becomes available after the refreshed file and
project match that receipt; an external edit, expired backup or conflict blocks it.
A pending or failed Undo keeps the Apply receipt and its local diagnostic. A
successful Undo refreshes source and cannot expose an older Undo chain, even if a
server response advertises one. Accepting a new candidate replaces the prior receipt.

Files, Symbols and Commands use a local palette with all loaded results available
in a scrolling list. File paths wrap for disambiguation. Arrow keys select from the
filter or move focus between result controls; Enter activates the current result,
and Space activates a focused result while remaining ordinary text in the filter.
Preparation commands keep their existing target checks and discard confirmation.

The footer and Models rail entry open the same read-only provider details. Separate
Analyze, Bugs and Function edits rows show configured models and selectable
destinations. Shared model/destination pairs count once; incomplete model or
destination metadata leaves totals unavailable. A displayed run's captured providers
stay labeled separately from current configuration and daemon connectivity. Opening
this dialog neither tests provider health nor grants consent to send project context.
