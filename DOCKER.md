# Docker deployment

The container runs only the Mini-Orca loopback daemon for the Compose Desktop
client. It is not a browser IDE or a multi-user service. The daemon is
loopback-only by default; the container examples make a published port an
explicit deployment choice with `MINI_ORCA_BIND_ADDRESS=0.0.0.0:9090`.

## Build and run

Create an ignored local configuration from the tracked template, fill all three
required model scopes, then build the image:

```bash
cp config.example.yaml config.yaml
docker build -t mini-orca:4.4.0 .
```

The runtime image contains the static daemon binary plus Alpine's certificate
and timezone data. It intentionally contains no Go source, configuration, or
project files. Mount the configuration and only the project directories that
the daemon may inspect:

```bash
docker run --rm --name mini-orca \
  -p 9090:9090 \
  -v "$(pwd)/config.yaml:/app/config.yaml:ro" \
  -v "$(pwd)/projects:/app/projects:rw" \
  -e MINI_ORCA_CONFIG=/app/config.yaml \
  -e MINI_ORCA_BIND_ADDRESS=0.0.0.0:9090 \
  mini-orca:4.4.0
```

Use a project path inside the mounted `/app/projects` directory. A mounted
project receives local `.mini-orca/` metadata and, after an explicit Apply,
the one reviewed source-file change. Keep `config.yaml` and its credentials
outside Git.

## Compose

`docker compose up --build` uses the same configuration and project mounts.
The optional local LM Studio service is available with:

```bash
docker compose --profile with-llm up --build
```

Set each scope's `api_base_url` to `http://lm-studio:1234/v1` before using that
profile. The Compose file uses the current Compose Specification and therefore
has no legacy top-level `version` declaration.

## Health and release verification

The image health check calls `GET /health` on the container's loopback address.
After starting a container, verify both liveness and its runtime contents:

```bash
curl --fail http://localhost:9090/health
docker inspect --format='{{.State.Health.Status}}' mini-orca
docker image inspect mini-orca:4.4.0
```

Before release, build the image without cleanup commands and confirm the health
check is `healthy`. Do not claim container verification when Docker is
unavailable. The daemon API is documented in
[docs/api-contract.md](docs/api-contract.md); scoped provider configuration and
the migration from retired fields are in [CONFIG.md](CONFIG.md).
