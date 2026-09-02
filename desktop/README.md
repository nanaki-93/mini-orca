# Mini-Orca Desktop

Start the Go daemon from the repository root, then launch the desktop client:

```bash
go run ./cmd/daemon
./desktop/gradlew -p desktop run
```

The client uses `http://localhost:9090` by default. Set `MINI_ORCA_URL` to point it at another daemon URL. The daemon is loopback-only by default; if its configured LLM provider is non-loopback, the user must confirm that destination before any prompt-bearing request. Keep provider credentials in the ignored local `config.yaml`.

The Editor workflow is Go-first: exact replace/create declaration drafts receive composition and focused validation. Other language views remain analysis-only until equivalent validators are available.

## Opening a project

Mini-Orca starts on a dedicated landing state. It shows only the product identity, an
**Open project** action, and concise opening or retry feedback. Press `Cmd/Ctrl+O` to
open the same project chooser from the keyboard. Until a project opens, workspace,
file, symbol, draft, palette, and project-management shortcuts are unavailable.

After import, the top bar shows the project name and one compact text-labeled connection
state. Workspace navigation is deliberately concise: Summary, Analysis, Bugs, and
Editor do not show inventory counters or implementation identifiers.

## Focused workflow

Summary presents deterministic project facts and optional structured analysis.
Analysis starts, pauses, resumes, or cancels bounded sequential Analyze-all only
when you request it; it shows coverage, the current or last run, and only
failed/error-bearing files. Bugs groups filtered findings by high, medium, low, then
fallback priority while each card retains its verified/tool or AI provenance; its filters
and triage change finding state, never source. Editor
owns indexed-file browsing, contextual file analysis, chat, the editable declaration/import
draft, validation, checks, review, Apply, and Undo for one file and one selected
or new Go declaration. Click within an indexed declaration to select the most specific
declaration and show its explanation in Context. Click outside declarations to clear
stale symbol context while keeping the focused source line.
Source remains read-only: dragging selects text and does not retarget or edit a draft.

For an exact atomic Go function, method, or type, Context
offers **Edit `<symbol>`**. It opens the bound Replace composer without contacting a
provider or changing source. To create
a declaration, open Commands and choose **Create declaration**; that route alone asks
for the new Go name. If another draft is active, changing targets requires an explicit
in-memory draft-discard decision. A persistent Editor header identifies the active file
by basename and project-relative path in both source and review; when no file is open it
uses a neutral no-file state. Successful validation opens the diff and Review, where
focused checks, exact Apply, receipt, and Undo remain together.

A manual draft edit always creates a new revision and invalidates the prior
validation and check results. Re-run both before Apply becomes available. The
source and composed diff panes are intentionally selectable but read-only.

## Compact controls and action meaning

Buttons and single-line fields use a shared compact size while retaining text labels,
keyboard focus, and disabled state. Violet identifies the dominant action; cyan is for
navigation; mint indicates a positive guarded transition; amber signals attention or
Undo; rose signals cancellation or destructive interruption; neutral controls support
secondary actions. Color reinforces the visible label and status text rather than
replacing them.

## Desktop shell smoke check

With a project containing a few nested directories (or a larger fixture), import it and verify Summary, Analysis, Bugs, and Editor navigation. Summary, Analysis, and Bugs use the full workspace canvas without Explorer or Context panes. In Analysis, confirm coverage and current/last-run totals are visible, only analysis errors are listed, and no result opens a file. Confirm the Analyze-all limits share a compact row and related actions wrap rather than clip when space is narrow. In Bugs, use the Filters disclosure to reveal advanced fields only when needed; confirm high, medium, low, and fallback sections are nonempty, ordered, and retain card-level provenance. In Editor, filtering keeps matching paths selected; disclosure controls expand and collapse; pane dividers retain their widths after restarting the app; and the source is selectable and read-only. Confirm the header names the active basename and relative path in both source and review, including duplicate basenames, and never retains a previous file after it closes. Click within a declaration to open its Context explanation, or outside one to clear stale symbol context while retaining the focused line; drag to select source text without changing the declaration or draft. A chat proposal stays bound to the selected file. Use the direct Edit action for an exact declaration, or Commands for Create declaration. Drafts without imports omit the `Required imports` field; existing imports remain editable. Edit only the declaration draft, validate it, run focused checks in Review, confirm Apply names the file and symbol, then use Undo. Re-analyze and verify old drafts become stale while textual freshness badges update.

## Keyboard and accessibility checklist

With the same fixture, verify the focused workflow without a mouse:

- `⌘P` opens file navigation from every workspace; choosing an indexed file activates Editor and opens that exact path. `⌘⇧O` opens the selected-file symbol picker, and `⌘K` opens the Replace composer only for an eligible selected declaration. `⌘1`–`⌘4` select Summary, Analysis, Bugs, and Editor; `⌘Tab` cycles them.
- Select an eligible declaration, enter a chat request, then use `⌘Enter` to send. Use Commands for **Create declaration**. Edit only the declaration draft, use `⌘⇧V` to validate and `⌘⇧C` for focused checks when those actions are visible. Hidden or unavailable actions do not fire. Press `Esc` only to close the active dialog or cancel the active request.
- Use `⌘⇧F` to return to Bugs and Tab through filters and finding actions. Confirm focusable controls expose text labels and selected/disabled state; freshness, validation, severity, confidence, scan/job state, and connection state remain understandable without color.
- Resize the window to exactly 1000dp and below 1000dp. At 1000dp, confirm Editor retains its wide Explorer and Context panes. Below it, Files and Context drawers appear only in Editor; leaving Editor closes an open drawer and Summary, Analysis, and Bugs retain their full canvas. At every size, the Editor header remains above the scrollable source or diff canvas.
- Check source and composed diff views remain read-only, preserve text selection, and highlight comments, strings, and language keywords without changing their content. Below 1000dp, clicking within a source declaration or choosing one in the symbol palette opens the labeled Context drawer without clearing its source highlight.
