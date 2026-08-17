# Mini-Orca Desktop

Start the Go daemon from the repository root, then launch the desktop client:

```bash
go run ./cmd/daemon
gradle -p desktop run
```

The client uses `http://localhost:9090` by default. Set `MINI_ORCA_URL` to point it at another daemon URL.

## Desktop shell smoke check

With a project containing a few nested directories (or a larger fixture), import it and verify that filtering keeps matching paths selected, directory disclosure controls expand and collapse, pane dividers resize and retain their widths after restarting the app, and the `Code`, `Summary`, and `Changes` tabs keep the selected file. Use `Re-analyze` and confirm the selected file remains selected while freshness badges update with text as well as icons.
