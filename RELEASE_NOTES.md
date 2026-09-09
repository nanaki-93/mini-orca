# Mini-Orca release notes

## v4.4.0 — Focused local desktop workflow

Current capabilities include independent `analyze`, `bug` and `function` model
profiles, scoped provider confirmation, file-bound declaration drafts, temporary
checks, explicit Apply and guarded Undo. Source and composed diffs stay read-only.
The loopback API has one current contract; old browser routes and model fallback
configuration are removed.

Security rule scans, advisory review, source-based Performance review and explicit
benchmark measurements are implemented. The Jewel Desktop uses the shared dark
IDE components. Local acceptance is complete for macOS arm64 Desktop and the
Linux arm64 container on the tested Docker Desktop host; see the exact
[evidence and limits](docs/RELEASE_ACCEPTANCE.md#rel-01-final-acceptance--2026-09-09).

Engineering-insight qualification is on standby after failed quality evaluations.
Existing insights remain optional, unqualified AI interpretation. Live-provider
compatibility is limited to the tested local v13 selected-file request contract;
external-provider compatibility and broader AI factual reliability are unclaimed.

The [plan](PLAN.md) records completed delivery and user-approved deferrals. This
local acceptance does not publish a release. No configuration migration or project
reset is required. Older release history remains in Git; [CONFIG.md](CONFIG.md)
and the [API guide](docs/api-contract.md) own supported setup and contracts.
