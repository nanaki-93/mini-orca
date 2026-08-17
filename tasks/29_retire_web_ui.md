# 29 — Remove the legacy HTMX web UI

## Goal

Retire browser IDE code after the desktop client satisfies the focused workflow.

## Depends on

Tasks 03 and 20–24, 28.

## Implementation

- Confirm desktop parity for import, browsing, selection, generation preview, errors, connection status, and activity history.
- Remove page routes, HTMX handlers, templates, static CSS/JavaScript, Tailwind/HTMX browser dependencies, and web-only response cache wiring.
- Keep health, status, and documented desktop API routes.
- Remove web-specific tests and update Docker/README so port 9090 is described as a local API, not an IDE URL.
- Review daemon bind address/CORS exposure for the local-machine model; do not expose sensitive project APIs unintentionally.

## Acceptance criteria

- The daemon starts and all desktop API tests pass with no HTML/static UI files or routes.
- The desktop client remains usable against the daemon end to end.

## Verification

- Search for removed web route/asset references, run Go and desktop tests, and perform desktop smoke test.
