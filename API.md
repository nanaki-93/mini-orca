# Mini-Orca API guide

The local daemon API is for the Compose Desktop client. The authoritative route
inventory and behavior notes are in [docs/api-contract.md](docs/api-contract.md);
machine-readable request and response schemas are in
[docs/openapi.yaml](docs/openapi.yaml).

The current API is centered on file-scoped chat sessions and editable Go
declaration drafts. New clients open a session pinned to one project/file/symbol,
send messages that cannot retarget it, then read, revise, validate, check, and
explicitly apply the resulting draft. The retired `POST /api/chat/message`
route is documented only as a `410 Gone` migration response.

All source-mutating behavior is confirmation-gated, hash/revision-guarded,
one-file-only, and auditable. The daemon does not provide a browser IDE,
automatic scans, automatic writes, multi-file changes, commits, or pushes.

The API binds to loopback by default. For a non-loopback LLM provider, requests
that send prompt content must include `confirm_remote_provider: true` after
the user has reviewed the provider destination. Local credentials belong only in
the ignored `config.yaml`; they are never returned by the API.

