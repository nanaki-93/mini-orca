# 33 — Fix repository configuration and version hygiene

## Status

Pending

## Goal

Remove tracked local configuration risk and make every displayed/documented version derive from the canonical version source.

## Depends on

Task 32.

## Implementation

- Stop tracking local `config.yaml` while preserving the developer's local file and add it to `.gitignore`; keep `config.example.yaml` as the only repository example.
- Inspect tracked history for credential-like values without printing secrets; report any suspected exposure and do not rewrite history without explicit user authorization.
- Remove hard-coded desktop, Makefile-comment, and documentation version drift where values can be read or injected from `internal/version` or the daemon status response.
- Add a regression check proving local configuration is ignored and the example remains valid.
- Do not modify provider credentials or generate a replacement local configuration.

## Acceptance criteria

- A local `config.yaml` is ignored and absent from the tracked file list.
- Desktop and daemon present the same canonical version.
- The example configuration tests continue to pass without reading local credentials.

## Verification

- Run `go test ./internal/config ./cmd/daemon` and `./desktop/gradlew -p desktop test`.
- Run `git check-ignore config.yaml`, `make fmt-check`, and `git diff --check`.
