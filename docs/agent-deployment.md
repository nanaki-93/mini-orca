# Agent worker and local inspection

Mini-Orca's daemon remains a local service. Two separate commands support remote
advisory analysis and local debugging without giving a remote agent checkout
access. Build with `make build-agents`; use `make build-agents-linux` for Linux
amd64 binaries with `CGO_ENABLED=0`. Both commands are also included in `make build`
and `make build-linux`.

## Explicit worker startup

`build/mini-orca-agent-worker --help` describes the flags and starts no listener.
Actual startup requires `--serve`, an explicit YAML configuration, an operator
scope manifest and source-sharing confirmation:

```sh
build/mini-orca-agent-worker --serve --config /run/secrets/worker.yaml \
  --scope /run/secrets/scope.json --public-url https://worker.example/a2a \
  --listen 0.0.0.0:8080 --lifetime 10m --max-jobs 32 \
  --confirm-source-sharing --confirm-remote-provider
```

This is an operator command, not a deployment performed by Mini-Orca. The public
URL must end in `/a2a`, contain no credentials/query/fragment, and use HTTPS;
literal loopback HTTP is accepted for local operation. Keep non-loopback HTTP
behind a trusted TLS ingress. Without `--listen`, the worker uses
`127.0.0.1:9091`, or `0.0.0.0:$PORT` when the container host supplies `PORT`.

Use the existing [configuration schema](../CONFIG.md). All three model scopes
must be valid; only the **Function** HTTP model executes here. OpenAI-compatible,
Gemini and Vertex profiles use the production ADK adapter. CLI model profiles are
rejected. Provider credentials must be explicitly configured; the worker does
not discover a cloud account. The configured A2A server must be enabled:

```yaml
integrations:
  a2a_server:
    enabled: true
    timeout_seconds: 60
    max_concurrent: 2
    principals:
      - id: reviewer
        bearer_token_env: MINI_ORCA_WORKER_TOKEN
        project_id: example-project
        capabilities: [analysis]
```

Each principal must select the manifest's exact project and only `analysis`.
Supply distinct bearer secrets through the referenced environment variables;
32–4096 printable ASCII bytes are required. Never put secrets in command-line
arguments, manifests or version control. The configuration's daemon `bind`
remains loopback-only; the separate worker's `--listen` controls its listener.
Reading configuration does not grant the daemon access or connect integrations.

The operator's JSON manifest pins project/revision and one to eight relative
paths with their exact lowercase SHA-256 hashes:

```json
{
  "project_id": "example-project",
  "project_revision": "captured-revision",
  "files": [
    {"path": "src/example.go", "hash": "sha256:REPLACE_WITH_64_LOWERCASE_HEX_DIGITS"}
  ]
}
```

The placeholder hash must be replaced with the hash of the actual UTF-8 bytes.
The worker never opens these source paths. The manifest itself must fit in 16 KiB;
unknown fields, duplicate keys and trailing JSON are rejected. Source changes
require a new manifest and explicit worker restart, not remote grant expansion.

## Requests, budgets and results

`GET /.well-known/agent-card.json` publishes an analysis-only A2A 1.0 card without
project or credential data. Authenticate JSON-RPC calls to `/a2a` with
`Authorization: Bearer <configured-secret>`. `SendMessage` or
`SendStreamingMessage` accepts one user text part. Its text is this JSON shape:

```json
{
  "project_id": "example-project",
  "project_revision": "captured-revision",
  "question": "Explain the behavior and cite the relevant lines.",
  "files": {"src/example.go": "EXACT_CAPTURED_SOURCE_BYTES"},
  "confirm_source_sharing": true,
  "confirm_remote_provider": true
}
```

Use a unique message ID. The native A2A envelope uses `jsonrpc: "2.0"`, a bounded
string `id`, `method: "SendMessage"`, and
`params.message: {messageId, role: "ROLE_USER", parts: [{text}]}`. Serialize the
inner request as the text string. This source-carrying worker contract differs
from the daemon's grant-bound A2A contract; the existing TUI's remote-agent
delegation does not automatically upload source to workers.

The request may select a subset of the manifest. Every file must match its pinned
path and hash. Each file is nonempty UTF-8 without NUL, at most 32 KiB; total source
is at most 64 KiB. The question is at most 2048 bytes. Request text is at most
128 KiB and the JSON-RPC envelope at most 256 KiB. The daemon retains its original
8 KiB text ceiling. No file, shell, Git, deployment or source-writing tool exists
in the worker: `read_snippet` and `search_repository` operate on admitted bytes in
memory. Source/question text cannot change these permissions.

Startup source consent is required. Remote model use requires consent at both
startup and request admission. Tools and final publication recheck credential
availability, cancellation and lifetime. Rotating/removing the configured bearer
secret revokes subsequent checks; restart the process to admit a replacement.

Each process lives for 1 second–15 minutes and accepts at most 1–128 admitted
jobs (defaults: 15 minutes and 32 jobs). Failed model calls consume their job;
requests rejected before execution do not. Per-job limits are the lower of the
configured runtime limits and these ceilings:

| Resource | Worker ceiling |
| --- | --- |
| Model calls / tool calls | 4 / 16 |
| Input / output token charges | 131072 / 16384 |
| Output per model call | 4096 tokens |
| Concurrent model calls per job | 1 |
| Repairs | 0 |
| Job duration | Configured A2A timeout, at most 120 seconds |
| Concurrent jobs | Configured A2A concurrency, 1–4 |

Results carry `advisory: true`, project/revision, manifest `scope_hash`, usage,
`checks: "not_run"`, and a bounded answer with captured path/hash/line citations.
Malformed output or fabricated citations fail the task. Advice is not a checked
proposal and cannot be applied. Provider errors remain failures; there is no
automatic provider switch or application-level retry.

Task lookup, subscriptions and cancellation retain the native A2A principal
ownership checks. Tasks, sessions and replay protection are process-local and
disappear on restart. Limits are per process, not a shared tenant quota. A
timeout/disconnect can leave the caller uncertain; do not automatically resend
under a new message ID. When the process is still alive, query the known task as
its owner. A restarted worker cannot recover the old task or prove nonexecution.

## Container operation

The optional image has a pinned Go build stage and a nonroot scratch runtime with
CA certificates. Its default command is help. It contains no checkout, shell or
configuration file. Build it only when a Docker engine is available:

```sh
docker build --platform linux/amd64 -f Dockerfile.agent-worker -t mini-orca-agent-worker .
docker run --rm --network none mini-orca-agent-worker --help
```

Mount the private configuration and manifest read-only and supply secret
environment references through the deployment platform. Keep ingress
authenticated and restricted to intended callers; the worker bearer check is
still required. Configure request timeouts/concurrency consistently with the
worker limits. Cloud Run requires Linux amd64 and listening on `0.0.0.0:$PORT`,
terminates TLS at ingress, and allows ten seconds after SIGTERM before SIGKILL.
The worker cancels owned model calls before joining HTTP handlers within an
eight-second shutdown deadline. See the official
[Cloud Run container contract](https://docs.cloud.google.com/run/docs/container-contract).

Startup writes a source-free readiness record. HTTP/SDK diagnostics are not
written to process logs. Responses remain private content; ingress logging,
provider retention and operator access controls must match the intended use.
Do not treat short process lifetime as a provider deletion policy. Deployment,
billing, image publication and live account tests are separate operator actions.

Tests use fake providers and local listeners. Linux cross-builds validate the
binary target; they do not prove container startup, cloud IAM, ingress behavior
or live model quality. Container execution has not been verified in the current
environment because the Docker engine is unavailable.

## Read-only local inspection

```sh
build/mini-orca-agent-inspect --root /absolute/checkout --limit 100
build/mini-orca-agent-inspect --root /absolute/checkout --run INVOCATION_ID
build/mini-orca-agent-inspect --root /absolute/checkout --run INVOCATION_ID --private
build/mini-orca-agent-inspect --root /absolute/checkout --artifact ARTIFACT_ID --private
```

Default JSON contains recorded invocation ancestry, schema/state, node events,
usage and artifact metadata. It does not infer an unrecorded dependency graph or
executed check. `--run` filters runs and their events; `--artifact` filters the
artifact section. Other sections remain metadata only. IDs are taken from the
metadata output. The two selections cannot be combined.

Private content requires `--private` with one exact ID. A selected run exposes
its private session, including shared app/user state returned by the journal
owner. A selected artifact exposes that immutable version as `data_base64`.
Treat these outputs as source-bearing material. Unknown IDs and corrupt journals
fail rather than returning empty history; no repair, resume or migration occurs.

The inspector never constructs a service, configures a provider or writes local
metadata. An empty checkout returns empty history without creating `.mini-orca`.
It reports `truncated` when the per-section limit (1–500) omits records, alongside
available run/artifact counts. Output over 8 MiB fails before writing JSON; narrow
the selection. History retains only the journal's existing bounded event window.
Concurrent writers may advance the journal between section reads. Exit status is
0 for success/help, 2 for command-line usage errors, and 1 for inspection/output
failures.
