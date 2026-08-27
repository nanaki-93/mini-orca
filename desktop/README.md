# Mini-Orca Desktop

Start the Go daemon from the repository root, then launch the desktop client:

```bash
go run ./cmd/daemon
./desktop/gradlew -p desktop run
```

The client uses `http://localhost:9090` by default. Set `MINI_ORCA_URL` to point it at another daemon URL. The daemon is loopback-only by default; if its configured LLM provider is non-loopback, the user must confirm that destination before any prompt-bearing request. Keep provider credentials in the ignored local `config.yaml`.

The Editor workflow is Go-first: exact replace/create declaration drafts receive composition and focused validation. Other language views remain analysis-only until equivalent validators are available.

## Focused workflow

Summary presents deterministic project facts and optional structured analysis.
Analysis starts, pauses, resumes, or cancels bounded sequential Analyze-all only
when you request it. Bugs separates parser/vet/test findings from AI
suggestions; its filters and triage change finding state, never source. Editor
pins chat, the editable declaration/import draft, validation, checks, review,
Apply, and Undo to the one file and one selected or new Go declaration.

A manual draft edit always creates a new revision and invalidates the prior
validation and check results. Re-run both before Apply becomes available. The
source and composed diff panes are intentionally selectable but read-only.

## Desktop shell smoke check

With a project containing a few nested directories (or a larger fixture), import it and verify Summary, Analysis, Bugs, and Editor navigation; filtering keeps matching paths selected; disclosure controls expand and collapse; and pane dividers retain their widths after restarting the app. In Editor, confirm the source is selectable and read-only, the file/symbol brief remains visible, and a chat proposal stays bound to the selected file. Edit only the declaration draft, validate it, run checks, confirm Apply names the file and symbol, then use Undo. Re-analyze and verify old drafts become stale while textual freshness badges update.

## Keyboard and accessibility checklist

With the same fixture, verify the focused workflow without a mouse:

- `⌘P` opens file navigation, `⌘⇧O` opens the selected-file symbol picker, and `⌘K` opens the file-scoped action route. `⌘1`–`⌘4` select Summary, Analysis, Bugs, and Editor; `⌘Tab` cycles them.
- Select or create one target, enter a chat request, then use `⌘Enter` to send. Edit only the declaration draft, use `⌘⇧V` to validate and `⌘⇧C` for focused checks. Press `Esc` only to close the active dialog or cancel the active request.
- Use `⌘⇧F` to return to Bugs and Tab through filters and finding actions. Confirm focusable controls expose text labels and selected/disabled state; freshness, validation, severity, confidence, scan/job state, and connection state remain understandable without color.
- Resize the window to exactly 1000dp and below 1000dp; verify the wide panes remain stable at the breakpoint and labeled Files and Context controls open drawers below it rather than compressing all three panes.
- Check source and composed diff views remain read-only, preserve text selection, and highlight comments, strings, and language keywords without changing their content. The compact brief must remain above source in the narrow layout.
