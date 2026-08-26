# Mini-Orca Release Notes

## v4.3.0 — Focused file-scoped declaration drafts (2026-08-26)

Mini-Orca now presents a single local Desktop workflow across Summary, Analysis,
Bugs, and Editor: one active project, one open file, one selected or new Go
declaration, one editable draft, validation, focused checks, explicit Apply, and
immediate guarded Undo.

### What changed

- File-scoped chat sessions are pinned to project revision, open file, base hash,
  edit mode, and declaration target. Follow-up messages cannot retarget them.
- The model returns an isolated declaration and optional imports. The daemon
  composes the complete candidate in memory; source and diff views stay
  read-only.
- Manual declaration/import edits create a new draft revision and clear earlier
  validation and check evidence. Apply requires the reviewed id, revision, hash,
  project/file guards, and a named explicit confirmation.
- Summary exposes deterministic metrics and structured analysis; Analysis
  controls explicit bounded Analyze-all; Bugs separates verified parser/vet/test
  findings from AI suggestions with provenance and freshness.
- The API, OpenAPI contract, configuration guidance, and desktop smoke checklist
  now describe this workflow and its keyboard/responsive behavior.

### Migration notes

- Replace the retired one-shot POST /api/chat/message request with a file-scoped
  chat session, a session message, draft read/edit/validate/check/review routes,
  then explicit draft Apply or Undo.
- POST /api/chat/message remains registered only to return 410 Gone. GET
  /api/chat/history is a deprecated source-free activity alias; use GET
  /api/projects/current/activity instead.
- The older candidate comparison/check/export routes remain compatibility routes
  for their metadata only. New Desktop workflow code uses draft routes.

### Privacy and limits

- The daemon listens on loopback by default. Remote LLM endpoints require an
  explicit per-request confirmation before prompt content is sent.
- config.yaml is local and ignored by Git; use config.example.yaml as the
  tracked template. Credentials are never returned in API responses.
- Mini-Orca does not create multi-file edits, edit source directly, scan
  automatically, write drafts automatically, commit, or push.
- Exact declaration editing, composition, and required validators are Go-first.
  Other languages may be analyzed conservatively but do not receive equivalent
  declaration editing until dedicated validators exist.

## v4.2.0 — Desktop-only focused workflow (superseded)

The v4.2 preview workflow has been superseded by v4.3 file-scoped declaration
drafts. Consult the v4.3 migration notes and current API contract rather than
using its former one-shot generation examples.

