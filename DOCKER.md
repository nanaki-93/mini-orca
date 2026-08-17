# Docker Usage Guide

Mini-Orca's container runs the daemon API for the Compose Desktop client on the
same machine; it does not serve a browser IDE. The daemon defaults to loopback
for local safety, so container examples explicitly set
`MINI_ORCA_BIND_ADDRESS=0.0.0.0:9090` only to make the published port reachable.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start](#quick-start)
3. [Docker Images](#docker-images)
4. [Configuration](#configuration)
5. [Development](#development)
6. [Production Deployment](#production-deployment)
7. [Troubleshooting](#troubleshooting)

---

## Prerequisites

- **Docker**: Version 20.10+ ([Install Docker](https://docs.docker.com/get-docker/))
- **Docker Compose**: Version 2.0+ (included with Docker Desktop)
- **Disk Space**: Minimum 2GB for base image, 4GB recommended
- **Memory**: Minimum 1GB RAM, 2GB recommended

---

## Quick Start

### Option 1: Using Makefile (Recommended)

```bash
# Build and run with Docker
make docker-build
make docker-run

# Or use docker-compose
make compose-up
```

### Option 2: Using Docker Commands

```bash
# Build the image
docker build -t mini-orca:4.1.0 .

# Run the container
docker run -d \
  --name mini-orca \
  --restart unless-stopped \
  -p 9090:9090 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v $(pwd)/projects:/app/projects:rw \
  -v $(pwd)/logs:/app/logs:rw \
  -e MINI_ORCA_CONFIG=/app/config.yaml \
  mini-orca:4.1.0
```

### Option 3: Using Docker Compose

```bash
# Start with docker-compose
docker compose up -d --build

# Start with LM Studio included
docker compose --profile with-llm up -d --build
```

---

## Docker Images

### Available Tags

| Tag | Description |
|-----|-------------|
| `mini-orca:4.1.0` | Current desktop-API build |
| `mini-orca:latest` | Latest build (may be unstable) |
| `mini-orca:dev` | Development build |

### Image Layers

The Docker image uses a multi-stage build:

1. **Builder Stage**: Go 1.22 Alpine with build dependencies
2. **Runtime Stage**: Minimal Alpine Linux with runtime dependencies

### Image Size

| Stage | Size |
|-------|------|
| Builder | ~800MB |
| Runtime | ~150MB |
| Total | ~150MB (production) |

---

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MINI_ORCA_CONFIG` | Path to config file | `config.yaml` |
| `MINI_ORCA_LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |

### Volume Mounts

| Mount | Purpose | Read/Write |
|-------|---------|------------|
| `/app/config.yaml` | Configuration file | Read-only |
| `/app/projects` | Project directory | Read-write |
| `/app/logs` | Log files | Read-write |

### Example Configuration

```yaml
# config.yaml
llm:
  base_url: "http://lm-studio:1234"  # Use container name
  api_key: ""
  model: ""
  temperature: 0.7
  max_tokens: 8192

agents:
  coder:
    skills:
      - code_generation
      - refactoring
      - debugging
    model: ""
    timeout_seconds: 300

retry:
  max_retries: 3
  backoff_base: 1000
  backoff_max: 30000

logging:
  level: "info"
  format: "json"
  filename: "/app/logs/mini-orca.log"
  sensitive_keys:
    - "password"
    - "secret"
    - "api_key"
    - "token"
    - "authorization"
```

---

## Development

### Local Development with Docker

```bash
# Run in development mode with hot reload
make dev

# Or use air for automatic restart
make dev-watch
```

### Docker Compose for Development

```yaml
# docker-compose.dev.yml
version: '3.8'

services:
  mini-orca:
    build:
      context: .
      dockerfile: Dockerfile
    volumes:
      - .:/app
      - /app/node_modules
    environment:
      - MINI_ORCA_CONFIG=/app/config.yaml
      - MINI_ORCA_LOG_LEVEL=debug
    ports:
      - "9090:9090"
    command: air  # Hot reload
```

### Build Arguments

| Argument | Description | Default |
|----------|-------------|---------|
| `BUILD_DATE` | Build timestamp | Current UTC time |
| `VCS_REF` | Git commit hash | Current commit |

---

## Production Deployment

### Kubernetes Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mini-orca
  labels:
    app: mini-orca
spec:
  replicas: 2
  selector:
    matchLabels:
      app: mini-orca
  template:
    metadata:
      labels:
        app: mini-orca
    spec:
      containers:
        - name: mini-orca
          image: mini-orca:4.1.0
          ports:
            - containerPort: 9090
          resources:
            requests:
              memory: "512Mi"
              cpu: "250m"
            limits:
              memory: "2Gi"
              cpu: "1000m"
          livenessProbe:
            httpGet:
              path: /health
              port: 9090
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 9090
            initialDelaySeconds: 3
            periodSeconds: 5
          volumeMounts:
            - name: config
              mountPath: /app/config.yaml
              subPath: config.yaml
              readOnly: true
            - name: projects
              mountPath: /app/projects
            - name: logs
              mountPath: /app/logs
      volumes:
        - name: config
          configMap:
            name: mini-orca-config
        - name: projects
          persistentVolumeClaim:
            claimName: mini-orca-projects
        - name: logs
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: mini-orca
spec:
  selector:
    app: mini-orca
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9090
  type: LoadBalancer
```

### Docker Compose for Production

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  mini-orca:
    image: mini-orca:4.1.0
    restart: always
    ports:
      - "9090:9090"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./projects:/app/projects:rw
      - ./logs:/app/logs:rw
    environment:
      - MINI_ORCA_CONFIG=/app/config.yaml
      - MINI_ORCA_LOG_LEVEL=warn
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '1.0'
        reservations:
          memory: 512M
          cpus: '0.25'
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:9090/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s
```

### CI/CD Pipeline

```yaml
# .github/workflows/docker.yml
name: Docker Build and Push

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v3

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v2

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Login to Docker Hub
        uses: docker/login-action@v2
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}

      - name: Build and push
        uses: docker/build-push-action@v4
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            mini-orca:latest
            mini-orca:${{ github.ref_name }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

---

## Troubleshooting

### Common Issues

#### 1. Port Already in Use

```bash
# Check what's using port 9090
lsof -i :9090

# Stop existing container
make docker-stop

# Run on different port
docker run -p 8081:9090 mini-orca:4.1.0
```

#### 2. Permission Denied

```bash
# Fix volume permissions
chmod 755 ./projects ./logs

# Or run as root (not recommended)
docker run --user root -v $(pwd)/projects:/app/projects mini-orca:4.1.0
```

#### 3. Config File Not Found

```bash
# Verify config file exists
ls -la config.yaml

# Use default config
docker run -d mini-orca:4.1.0

# Or specify config path
docker run -e MINI_ORCA_CONFIG=/app/config.yaml -v $(pwd)/config.yaml:/app/config.yaml mini-orca:4.1.0
```

#### 4. Health Check Failing

```bash
# Check container logs
docker logs mini-orca

# Check if container is running
docker ps

# Restart container
docker restart mini-orca
```

#### 5. Out of Memory

```bash
# Increase memory limits
docker run -m 4g --memory-swap 4g mini-orca:4.1.0

# Or in docker-compose
# deploy:
#   resources:
#     limits:
#       memory: 4G
```

### Debug Mode

```bash
# Run with debug logging
docker run -e MINI_ORCA_LOG_LEVEL=debug mini-orca:4.1.0

# Run interactively
docker run -it --entrypoint sh mini-orca:4.1.0

# Execute commands in running container
docker exec -it mini-orca sh
```

### Logs

```bash
# View logs
docker logs mini-orca

# Follow logs
docker logs -f mini-orca

# Logs with tail
docker logs --tail 100 mini-orca

# Saved logs (if configured)
cat logs/mini-orca.log
```

---

## Best Practices

### Security

1. **Use non-root user**: The Dockerfile creates a non-root user `appuser`
2. **Mount config as read-only**: `-v $(pwd)/config.yaml:/app/config.yaml:ro`
3. **Limit resources**: Use `--memory` and `--cpus` flags
4. **Keep images updated**: Regularly rebuild with latest base images
5. **Scan for vulnerabilities**: Use `docker scan` or Trivy

### Performance

1. **Use multi-stage builds**: Reduces image size
2. **Cache Go modules**: Copy `go.mod` and `go.sum` before source code
3. **Enable build cache**: Use `--cache-from` and `--cache-to`
4. **Use Alpine base**: Smaller attack surface and image size
5. **Set resource limits**: Prevent resource exhaustion

### Monitoring

1. **Health checks**: Built-in health endpoint at `/health`
2. **Structured logging**: JSON format for log aggregation
3. **Metrics**: Consider adding Prometheus metrics endpoint
4. **Log rotation**: Configure log file size limits

---

## Makefile Commands Reference

| Command | Description |
|---------|-------------|
| `make help` | Show all available targets |
| `make build` | Build locally |
| `make test` | Run tests with coverage |
| `make docker-build` | Build Docker image |
| `make docker-run` | Run container |
| `make docker-stop` | Stop container |
| `make docker-logs` | Show container logs |
| `make compose-up` | Start with docker-compose |
| `make compose-up-llm` | Start with LM Studio |
| `make compose-down` | Stop docker-compose |
| `make docker-clean` | Remove Docker resources |
| `make compose-clean` | Remove docker-compose resources |
| `make dev` | Run locally for development |
| `make version` | Show version information |

---

## Support

For issues and questions:

- **GitHub Issues**: [mini-orca/issues](https://github.com/nanaki-93/mini-orca/issues)
- **Documentation**: [README.md](../README.md)
- **API Docs**: [docs/api-contract.md](../docs/api-contract.md)

---

**End of Docker Usage Guide**
