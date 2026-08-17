# Mini-Orca Desktop

Start the Go daemon from the repository root, then launch the desktop client:

```bash
go run ./cmd/daemon
gradle -p desktop run
```

The client uses `http://localhost:8080` by default. Set `MINI_ORCA_URL` to point it at another daemon URL.
