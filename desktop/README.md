# Mini-Orca Desktop

Start the Go daemon from the repository root, then launch the desktop client:

```bash
go run ./cmd/daemon
./desktop/gradlew -p desktop run
```

The client uses `http://localhost:9090` by default. Set `MINI_ORCA_URL` to point it at another daemon URL.

## Desktop shell smoke check

With a project containing a few nested directories (or a larger fixture), import it and verify that filtering keeps matching paths selected, directory disclosure controls expand and collapse, pane dividers resize and retain their widths after restarting the app, and the `Code`, `Summary`, and `Changes` tabs keep the selected file. Use `Re-analyze` and confirm the selected file remains selected while freshness badges update with text as well as icons.

## Keyboard and accessibility checklist

With the same fixture, verify the focused workflow without a mouse:

- `⌘P` opens file navigation, `⌘⇧O` opens the selected-file symbol picker, and `⌘K` opens focused actions.
- Select a file and symbol through the palettes, enter a request, then use `⌘Enter` to generate a preview; press `Esc` while analysis or generation is active to cancel it.
- Use `⌘Tab` to move between Code, Summary, and Changes. Confirm focusable controls expose text labels and freshness, validation, severity, and connection state have text or icons in addition to color.
- Resize the window below 1000dp and verify the Files and Action controls open drawers rather than compressing all three panes.
- Check source and diff views remain read-only, preserve text selection, and highlight comments, strings, and language keywords without changing their content.
