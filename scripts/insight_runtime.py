#!/usr/bin/env python3
"""Bounded lifecycle for the audited local MLX-VLM insight runtime.

This utility deliberately has no request path.  ``check`` only reads files,
distribution metadata, process metadata, and a loopback socket.  ``start`` is
the sole operation that preloads the model, and ``stop`` will signal only a
process whose PID and start identity still match the state it created.
"""

from __future__ import annotations

import argparse
import contextlib
import fcntl
import hashlib
import json
import os
import signal
import socket
import stat
import subprocess
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping, Sequence


CHECK_TIMEOUT_SECONDS = 10
SHUTDOWN_TIMEOUT_SECONDS = 15
READINESS_TIMEOUT_SECONDS = 300
HEALTH_BODY_LIMIT_BYTES = 64 * 1024
STATE_FILE_NAME = "runtime-state.json"
LOG_FILE_NAME = "runtime.log"
LOCK_FILE_NAME = "lifecycle.lock"
MUTABLE_RUNTIME_FILES = (LOCK_FILE_NAME, STATE_FILE_NAME, LOG_FILE_NAME, f"{STATE_FILE_NAME}.tmp")
OS_ENVIRONMENT_KEYS = ("PATH", "HOME", "TMPDIR", "LANG", "LC_CTYPE")
FIXED_ENVIRONMENT = {
    "MAX_KV_SIZE": "119552",
    "MLX_VLM_MAX_NUM_SEQS": "1",
    "MLX_VLM_ENABLE_THINKING": "1",
    "MLX_VLM_MAX_TOKENS": "4096",
    "MLX_VLM_MODEL_DISCOVERY": "served",
    "MLX_TRUST_REMOTE_CODE": "false",
    "HF_HUB_OFFLINE": "1",
    "TRANSFORMERS_OFFLINE": "1",
    "PYTHONDONTWRITEBYTECODE": "1",
}
REQUIRED_REQUEST = {
    "model": "./models/qwen38-v12-thinking-schema-1",
    "temperature": 1,
    "max_tokens": 4096,
    "reasoning_effort": "low",
    "top_p": 0.95,
    "top_k": 20,
}
FORBIDDEN_REQUEST_KEYS = ("min_p", "presence_penalty", "repeat_penalty")
TRACKED_REQUIREMENTS_FILE = Path(__file__).with_name("insight-runtime-requirements.txt")


class RuntimeError(Exception):
    """A fail-closed preflight or lifecycle error suitable for the CLI."""


class HealthUnavailable(RuntimeError):
    """The owned server may still be starting; retry within readiness bounds."""


class ListenerUnavailable(RuntimeError):
    """The server has not bound its loopback listener yet; retry safely."""


@dataclass(frozen=True)
class RuntimeConfig:
    candidate_dir: Path
    python: Path
    python_version: str
    port: int
    model: str
    artifact: Path
    artifact_identity_hash: str
    artifact_identity_file: Path
    artifact_identity_sha256: str
    audit_file: Path
    audit_sha256: str
    requirements_file: Path
    requirements_sha256: str
    state_dir: Path
    distributions: Mapping[str, str]
    source_hashes: Mapping[str, str]
    readiness_timeout_seconds: int

    @property
    def endpoint(self) -> str:
        return f"http://127.0.0.1:{self.port}"


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def ensure_runtime_directory(config: RuntimeConfig, *, create: bool) -> bool:
    """Validate the one mutable directory without following any symlink."""
    expected = config.candidate_dir / "runtime"
    if config.state_dir != expected:
        raise RuntimeError("runtime.json state_dir must be the literal candidate runtime directory")
    if expected.is_symlink():
        raise RuntimeError("runtime directory must not be a symlink")
    if not expected.exists():
        if not create:
            return False
        expected.mkdir(mode=0o700)
    if not expected.is_dir():
        raise RuntimeError("runtime path must be a directory")
    if stat.S_IMODE(expected.stat().st_mode) != 0o700:
        if not create:
            raise RuntimeError("runtime directory must be mode 0700")
        expected.chmod(0o700)
    for name in MUTABLE_RUNTIME_FILES:
        leaf = expected / name
        if leaf.is_symlink():
            raise RuntimeError(f"runtime mutable file must not be a symlink: {name}")
        if leaf.exists() and not leaf.is_file():
            raise RuntimeError(f"runtime mutable path must be a regular file: {name}")
    return True


def runtime_file(config: RuntimeConfig, name: str, *, create_directory: bool) -> Path:
    if name not in MUTABLE_RUNTIME_FILES:
        raise RuntimeError("unsupported mutable runtime file")
    ensure_runtime_directory(config, create=create_directory)
    path = config.state_dir / name
    if path.is_symlink():
        raise RuntimeError(f"runtime mutable file must not be a symlink: {name}")
    return path


def write_private_json(config: RuntimeConfig, value: Mapping[str, Any]) -> None:
    path = runtime_file(config, STATE_FILE_NAME, create_directory=True)
    temporary = runtime_file(config, f"{STATE_FILE_NAME}.tmp", create_directory=True)
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
    try:
        descriptor = os.open(temporary, flags, 0o600)
    except OSError as error:
        raise RuntimeError(f"cannot safely create runtime state: {error}") from error
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as output:
            json.dump(value, output, indent=2, sort_keys=True)
            output.write("\n")
        os.replace(temporary, path)
    except BaseException:
        temporary.unlink(missing_ok=True)
        raise


def read_json(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise RuntimeError(f"cannot read JSON {path}: {error}") from error
    if not isinstance(value, dict):
        raise RuntimeError(f"JSON object required: {path}")
    return value


def require_string(value: Mapping[str, Any], key: str) -> str:
    result = value.get(key)
    if not isinstance(result, str) or not result:
        raise RuntimeError(f"runtime.json requires non-empty string {key!r}")
    return result


def require_int(value: Mapping[str, Any], key: str, *, minimum: int, maximum: int) -> int:
    result = value.get(key)
    if isinstance(result, bool) or not isinstance(result, int) or not minimum <= result <= maximum:
        raise RuntimeError(f"runtime.json requires {key!r} between {minimum} and {maximum}")
    return result


def relative_path(candidate_dir: Path, raw: str, name: str) -> Path:
    path = Path(raw)
    if path.is_absolute() or ".." in path.parts:
        raise RuntimeError(f"runtime.json {name!r} must be relative to candidate-dir")
    # Keep the lexical candidate path.  The configured Python is normally a
    # venv symlink to the host interpreter, which is valid but resolves outside.
    return candidate_dir / path


def read_config(candidate_dir: Path) -> RuntimeConfig:
    candidate_dir = candidate_dir.resolve()
    config_path = candidate_dir / "runtime.json"
    if not config_path.is_file() or stat.S_IMODE(config_path.stat().st_mode) != 0o600:
        raise RuntimeError("runtime.json must exist with mode 0600")
    raw = read_json(config_path)
    if raw.get("schema_version") != 1:
        raise RuntimeError("runtime.json schema_version must be 1")
    port = require_int(raw, "port", minimum=1024, maximum=65535)
    if port != 1235:
        raise RuntimeError("runtime.json port must be the dedicated loopback port 1235")
    model = require_string(raw, "wire_model")
    if model != REQUIRED_REQUEST["model"]:
        raise RuntimeError(f"wire_model must be {REQUIRED_REQUEST['model']!r}")
    if raw.get("host") != "127.0.0.1":
        raise RuntimeError("runtime.json host must be 127.0.0.1")
    request = raw.get("request")
    if request != REQUIRED_REQUEST:
        raise RuntimeError("runtime.json request must exactly match the audited wire settings")
    for name in FORBIDDEN_REQUEST_KEYS:
        if name in request:
            raise RuntimeError(f"runtime.json request must omit unsupported {name!r}")
    environment = raw.get("environment")
    if environment != FIXED_ENVIRONMENT:
        raise RuntimeError("runtime.json environment must exactly match the fixed runtime environment")
    if raw.get("max_num_seqs") != 1 or raw.get("thinking") is not True:
        raise RuntimeError("runtime.json must enable thinking with exactly one sequence")
    if raw.get("speculative_decoding") is not False or raw.get("apc") is not False:
        raise RuntimeError("runtime.json must disable speculative decoding and APC")
    distributions = raw.get("distributions")
    source_hashes = raw.get("server_source_sha256")
    if not isinstance(distributions, dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in distributions.items()):
        raise RuntimeError("runtime.json distributions must be a string version map")
    required_distributions = parse_requirements(relative_path(candidate_dir, require_string(raw, "requirements_file"), "requirements_file"))
    if distributions != required_distributions:
        raise RuntimeError("runtime.json distributions must exactly match the reviewed requirements lock")
    if not isinstance(source_hashes, dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in source_hashes.items()):
        raise RuntimeError("runtime.json server_source_sha256 must be a string hash map")
    readiness = raw.get("readiness_timeout_seconds", READINESS_TIMEOUT_SECONDS)
    if isinstance(readiness, bool) or not isinstance(readiness, int) or not 1 <= readiness <= READINESS_TIMEOUT_SECONDS:
        raise RuntimeError("runtime.json readiness_timeout_seconds must be between 1 and 300")
    if require_string(raw, "state_dir") != "runtime":
        raise RuntimeError("runtime.json state_dir must be the literal value 'runtime'")
    return RuntimeConfig(
        candidate_dir=candidate_dir,
        python=relative_path(candidate_dir, require_string(raw, "python"), "python"),
        python_version=require_string(raw, "python_version"),
        port=port,
        model=model,
        artifact=Path(require_string(raw, "artifact_realpath")),
        artifact_identity_hash=require_string(raw, "artifact_realpath_sha256"),
        artifact_identity_file=relative_path(candidate_dir, require_string(raw, "artifact_identity_file"), "artifact_identity_file"),
        artifact_identity_sha256=require_string(raw, "artifact_identity_sha256"),
        audit_file=relative_path(candidate_dir, require_string(raw, "audit_file"), "audit_file"),
        audit_sha256=require_string(raw, "audit_sha256"),
        requirements_file=relative_path(candidate_dir, require_string(raw, "requirements_file"), "requirements_file"),
        requirements_sha256=require_string(raw, "requirements_sha256"),
        state_dir=candidate_dir / "runtime",
        distributions=distributions,
        source_hashes=source_hashes,
        readiness_timeout_seconds=readiness,
    )


def child_environment(parent: Mapping[str, str]) -> dict[str, str]:
    """Build a new environment; never inherit runtime tuning from the shell."""
    environment = {key: parent[key] for key in OS_ENVIRONMENT_KEYS if parent.get(key)}
    environment.update(FIXED_ENVIRONMENT)
    return environment



def parse_requirements(path: Path) -> dict[str, str]:
    """Read package/version pins only; hashes are protected by the file digest."""
    result: dict[str, str] = {}
    try:
        for line in path.read_text(encoding="utf-8").splitlines():
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            pin = line.split(" ", 1)[0]
            name, separator, version = pin.partition("==")
            if not separator or not name or not version:
                raise RuntimeError(f"invalid reviewed requirement pin: {line!r}")
            result[name] = version
    except OSError as error:
        raise RuntimeError(f"cannot read reviewed requirements lock: {error}") from error
    if not result:
        raise RuntimeError("reviewed requirements lock is empty")
    return result

def run_metadata(python: Path, python_version: str, distributions: Mapping[str, str]) -> None:
    if not python.is_file() or not os.access(python, os.X_OK):
        raise RuntimeError(f"configured Python is not executable: {python}")
    code = (
        "import importlib.metadata as m, json, sys; "
        "wanted=json.loads(sys.argv[1]); "
        "actual={name:m.version(name) for name in wanted}; "
        "print(json.dumps({'python':'.'.join(map(str, sys.version_info[:3])), 'distributions':actual}, sort_keys=True))"
    )
    try:
        completed = subprocess.run(
            [str(python), "-c", code, json.dumps(distributions, sort_keys=True)],
            capture_output=True,
            check=True,
            text=True,
            timeout=CHECK_TIMEOUT_SECONDS,
        )
        actual = json.loads(completed.stdout)
    except (subprocess.SubprocessError, json.JSONDecodeError) as error:
        raise RuntimeError(f"cannot read pinned distribution metadata: {error}") from error
    if actual.get("python") != python_version:
        raise RuntimeError(f"Python version drifted: expected {python_version}, got {actual.get('python')}")
    if actual.get("distributions") != dict(distributions):
        raise RuntimeError("distribution versions drifted from the reviewed requirements lock")


def verify_audit(config: RuntimeConfig) -> dict[str, Any]:
    if not config.audit_file.is_file() or sha256_file(config.audit_file) != config.audit_sha256:
        raise RuntimeError("accepted runtime compatibility audit hash does not match")
    audit = read_json(config.audit_file)
    reviewed = audit.get("reviewed_runtime")
    candidate = audit.get("candidate_model")
    if not isinstance(reviewed, dict) or not isinstance(candidate, dict):
        raise RuntimeError("accepted audit is missing reviewed runtime or candidate model")
    selected = reviewed.get("dependency_lock", {}).get("selected")
    audited_hashes = reviewed.get("server_source_sha256")
    if audited_hashes != dict(config.source_hashes):
        raise RuntimeError("runtime.json server hashes do not match the accepted audit")
    if not isinstance(selected, dict):
        raise RuntimeError("accepted audit is missing selected distributions")
    audited_versions = {name: item.get("version") for name, item in selected.items() if isinstance(item, dict)}
    if any(config.distributions.get(name) != version for name, version in audited_versions.items()):
        raise RuntimeError("runtime.json selected distribution versions do not match the accepted audit")
    if candidate.get("artifact_realpath") != str(config.artifact) or candidate.get("artifact_realpath_sha256") != config.artifact_identity_hash:
        raise RuntimeError("runtime.json artifact identity does not match the accepted audit")
    return audit


def verify_source_hashes(config: RuntimeConfig) -> None:
    site_packages = config.python.parent.parent / "lib" / "python3.11" / "site-packages" / "mlx_vlm" / "server"
    for name, expected in config.source_hashes.items():
        path = site_packages / name
        if not path.is_file() or sha256_file(path) != expected:
            raise RuntimeError(f"installed runtime source hash drift: {path}")


def verify_model_link(config: RuntimeConfig) -> None:
    link = config.candidate_dir / config.model
    if not link.is_symlink():
        raise RuntimeError(f"wire model must be a symlink: {link}")
    if not config.artifact.is_dir() or link.resolve() != config.artifact.resolve():
        raise RuntimeError("wire model symlink does not resolve to the audited artifact")
    identity = hashlib.sha256(str(config.artifact.resolve()).encode("utf-8")).hexdigest()
    if identity != config.artifact_identity_hash:
        raise RuntimeError("model artifact realpath identity drifted")
    record = read_json(config.artifact_identity_file)
    if sha256_file(config.artifact_identity_file) != config.artifact_identity_sha256:
        raise RuntimeError("artifact identity record hash does not match")
    if record.get("realpath") != str(config.artifact.resolve()) or record.get("generation_requests") != 0:
        raise RuntimeError("artifact identity record does not match the audited local model")
    files = record.get("files")
    if not isinstance(files, dict) or not files:
        raise RuntimeError("artifact identity record has no file hashes")
    for name, item in files.items():
        if not isinstance(item, dict) or not isinstance(item.get("bytes"), int) or not isinstance(item.get("sha256"), str):
            raise RuntimeError(f"artifact identity record has invalid entry for {name!r}")
        model_file = config.artifact / name
        if not model_file.is_file() or model_file.stat().st_size != item["bytes"] or sha256_file(model_file) != item["sha256"]:
            raise RuntimeError(f"model artifact hash drift: {model_file}")
    audit = verify_audit(config)
    audited_files = audit["candidate_model"].get("artifact_identity", {})
    for name, expected in audited_files.items():
        if files.get(name, {}).get("sha256") != expected:
            raise RuntimeError(f"artifact identity record disagrees with accepted audit for {name}")


def port_is_free(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as probe:
        probe.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            probe.bind(("127.0.0.1", port))
        except OSError:
            return False
    return True


def state_path(config: RuntimeConfig) -> Path:
    return config.state_dir / STATE_FILE_NAME


def log_path(config: RuntimeConfig) -> Path:
    return config.state_dir / LOG_FILE_NAME


@contextlib.contextmanager
def lifecycle_lock(config: RuntimeConfig):
    """Serialize state-changing lifecycle operations without locking check."""
    path = runtime_file(config, LOCK_FILE_NAME, create_directory=True)
    flags = os.O_WRONLY | os.O_CREAT | getattr(os, "O_NOFOLLOW", 0)
    try:
        descriptor = os.open(path, flags, 0o600)
    except OSError as error:
        raise RuntimeError(f"cannot safely open lifecycle lock: {error}") from error
    os.chmod(path, 0o600)
    try:
        try:
            fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise RuntimeError("another insight runtime lifecycle operation is in progress") from error
        yield
    finally:
        fcntl.flock(descriptor, fcntl.LOCK_UN)
        os.close(descriptor)


def process_identity(pid: int) -> str | None:
    try:
        completed = subprocess.run(
            ["ps", "-p", str(pid), "-o", "lstart=", "-o", "command="],
            capture_output=True,
            check=False,
            text=True,
            timeout=CHECK_TIMEOUT_SECONDS,
        )
    except subprocess.SubprocessError as error:
        raise RuntimeError(f"cannot inspect runtime process {pid}: {error}") from error
    value = completed.stdout.strip()
    if completed.returncode == 0 and value:
        return value
    if completed.returncode == 1 and not value:
        return None
    raise RuntimeError(f"cannot determine whether runtime process {pid} is still owned")


def read_state(config: RuntimeConfig) -> dict[str, Any] | None:
    if not ensure_runtime_directory(config, create=False):
        return None
    path = runtime_file(config, STATE_FILE_NAME, create_directory=False)
    if not path.exists():
        return None
    if stat.S_IMODE(path.stat().st_mode) != 0o600:
        raise RuntimeError("runtime state must be mode 0600")
    return read_json(path)


def owned_live_state(config: RuntimeConfig) -> dict[str, Any] | None:
    state = read_state(config)
    if state is None:
        return None
    pid = state.get("pid")
    start_identity = state.get("start_identity")
    if isinstance(pid, int) and isinstance(start_identity, str) and process_identity(pid) == start_identity:
        return state
    return None


def remove_stale_state(config: RuntimeConfig) -> None:
    if read_state(config) is not None and owned_live_state(config) is None:
        runtime_file(config, STATE_FILE_NAME, create_directory=False).unlink(missing_ok=True)


def check(config: RuntimeConfig) -> dict[str, Any]:
    """Read-only gate.  It never imports MLX, starts a server, or sends HTTP."""
    verify_audit(config)
    if not config.requirements_file.is_file() or sha256_file(config.requirements_file) != config.requirements_sha256:
        raise RuntimeError("reviewed requirements lock hash does not match")
    if not TRACKED_REQUIREMENTS_FILE.is_file() or sha256_file(TRACKED_REQUIREMENTS_FILE) != config.requirements_sha256:
        raise RuntimeError("tracked runtime requirements do not match the reviewed lock")
    verify_model_link(config)
    verify_source_hashes(config)
    run_metadata(config.python, config.python_version, config.distributions)
    if not port_is_free(config.port):
        raise RuntimeError(f"loopback port {config.port} is occupied")
    state = read_state(config)
    if state is not None:
        raise RuntimeError("runtime state already exists; run stop after confirming ownership")
    return {"status": "checked", "endpoint": config.endpoint, "generation_requests": 0}


def require_lms_empty() -> None:
    """Do not risk a second resident copy when LM Studio state is unknown."""
    try:
        completed = subprocess.run(
            ["lms", "ps", "--json"], capture_output=True, check=True, text=True, timeout=CHECK_TIMEOUT_SECONDS
        )
        value = json.loads(completed.stdout)
    except (OSError, subprocess.SubprocessError, json.JSONDecodeError) as error:
        raise RuntimeError(f"cannot verify LM Studio has no resident model: {error}") from error
    if value == []:
        return
    if isinstance(value, dict) and value.get("models") == [] and set(value) <= {"models", "data"}:
        return
    raise RuntimeError("LM Studio reports a resident model or an unrecognized process listing; unload it first")


def launch_command(config: RuntimeConfig) -> list[str]:
    return [
        str(config.python), "-m", "mlx_vlm.server", "--host", "127.0.0.1", "--port", str(config.port),
        "--model", config.model, "--model-discovery", "served", "--max-num-seqs", "1",
        "--max-kv-size", "119552", "--max-tokens", "4096", "--enable-thinking", "--log-level", "WARNING",
    ]


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


def fetch_health(config: RuntimeConfig) -> dict[str, Any]:
    request = urllib.request.Request(config.endpoint + "/health", method="GET")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    try:
        with opener.open(request, timeout=2) as response:
            if response.status != 200:
                raise HealthUnavailable(f"health endpoint returned status {response.status}")
            body = response.read(HEALTH_BODY_LIMIT_BYTES + 1)
            if len(body) > HEALTH_BODY_LIMIT_BYTES:
                raise RuntimeError("health endpoint response exceeded the bounded body limit")
            payload = json.loads(body)
    except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as error:
        raise HealthUnavailable(f"health endpoint is not ready: {error}") from error
    if not isinstance(payload, dict):
        raise RuntimeError("health endpoint returned a non-object response")
    return payload


def verify_listener_owner(pid: int, port: int) -> None:
    """Require the one loopback listener to belong to the recorded child."""
    try:
        completed = subprocess.run(
            ["/usr/sbin/lsof", "-nP", f"-iTCP:{port}", "-sTCP:LISTEN", "-Fpn"],
            capture_output=True,
            check=False,
            text=True,
            timeout=CHECK_TIMEOUT_SECONDS,
        )
    except (OSError, subprocess.SubprocessError) as error:
        raise RuntimeError(f"cannot inspect loopback listener ownership: {error}") from error
    if completed.returncode == 1 and not completed.stdout.strip():
        raise ListenerUnavailable("runtime listener is not ready")
    if completed.returncode != 0:
        raise RuntimeError("cannot determine loopback listener ownership")
    records: list[tuple[int, str]] = []
    current_pid: int | None = None
    for line in completed.stdout.splitlines():
        if line.startswith("p"):
            try:
                current_pid = int(line[1:])
            except ValueError as error:
                raise RuntimeError("lsof returned an invalid listener PID") from error
        elif line.startswith("n") and current_pid is not None:
            records.append((current_pid, line[1:]))
    expected = f"127.0.0.1:{port}"
    if records == []:
        raise ListenerUnavailable("runtime listener is not ready")
    if records != [(pid, expected)]:
        raise RuntimeError("loopback listener is not owned solely by the recorded runtime process")


def verify_health(config: RuntimeConfig, health: Mapping[str, Any]) -> None:
    if health.get("status") != "healthy" or health.get("loaded_model") != config.model:
        raise RuntimeError("health endpoint did not report the audited loaded model")
    if health.get("effective_context_limit") != 119552 or health.get("configured_context_limit") != 119552:
        raise RuntimeError("health endpoint did not report the fixed context limit")
    if health.get("continuous_batching_enabled") is not True or health.get("apc_enabled") is not False:
        raise RuntimeError("health endpoint did not report the required runtime settings")
    loaded = health.get("loaded_models")
    if not isinstance(loaded, dict) or set(loaded) != {"text_generation"}:
        raise RuntimeError("health endpoint reported an unexpected extra model lane")
    text_model = loaded["text_generation"]
    if not isinstance(text_model, dict) or text_model.get("model") != config.model:
        raise RuntimeError("health endpoint did not report the one expected model lane")


def terminate_owned(config: RuntimeConfig, state: Mapping[str, Any], timeout: int = SHUTDOWN_TIMEOUT_SECONDS) -> bool:
    pid = state.get("pid")
    start_identity = state.get("start_identity")
    if not isinstance(pid, int) or not isinstance(start_identity, str) or process_identity(pid) != start_identity:
        return False
    os.kill(pid, signal.SIGTERM)
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if process_identity(pid) is None:
            return True
        time.sleep(0.1)
    if process_identity(pid) == start_identity:
        os.kill(pid, signal.SIGKILL)
        deadline = time.monotonic() + 3
        while time.monotonic() < deadline:
            if process_identity(pid) is None:
                return True
            time.sleep(0.1)
    return process_identity(pid) is None


def reap_started_process(process: subprocess.Popen[Any]) -> bool:
    """Terminate and reap the exact Popen child created by this start attempt."""
    if process.poll() is not None:
        process.wait(timeout=1)
        return True
    process.terminate()
    try:
        process.wait(timeout=SHUTDOWN_TIMEOUT_SECONDS)
        return True
    except subprocess.TimeoutExpired:
        process.kill()
        try:
            process.wait(timeout=3)
            return True
        except subprocess.TimeoutExpired:
            return False


def start(config: RuntimeConfig) -> dict[str, Any]:
    with lifecycle_lock(config):
        remove_stale_state(config)
        check(config)
        require_lms_empty()
        log = runtime_file(config, LOG_FILE_NAME, create_directory=True)
        log_flags = os.O_WRONLY | os.O_CREAT | os.O_APPEND | getattr(os, "O_NOFOLLOW", 0)
        try:
            descriptor = os.open(log, log_flags, 0o600)
        except OSError as error:
            raise RuntimeError(f"cannot safely open runtime log: {error}") from error
        try:
            os.fchmod(descriptor, 0o600)
        except OSError as error:
            os.close(descriptor)
            raise RuntimeError(f"cannot set runtime log permissions: {error}") from error
        process: subprocess.Popen[Any] | None = None
        state: dict[str, Any] | None = None
        try:
            with os.fdopen(descriptor, "ab", buffering=0) as output:
                process = subprocess.Popen(
                    launch_command(config), cwd=config.candidate_dir, env=child_environment(os.environ),
                    stdout=output, stderr=subprocess.STDOUT, start_new_session=True,
                )
            identity = process_identity(process.pid)
            if identity is None:
                raise RuntimeError("server exited before its process identity could be recorded")
            state = {"schema_version": 1, "pid": process.pid, "start_identity": identity, "endpoint": config.endpoint,
                     "model": config.model, "command_sha256": hashlib.sha256("\0".join(launch_command(config)).encode()).hexdigest()}
            write_private_json(config, state)
            deadline = time.monotonic() + config.readiness_timeout_seconds
            while time.monotonic() < deadline:
                if process.poll() is not None:
                    process.wait(timeout=1)
                    raise RuntimeError(f"server exited during readiness with status {process.returncode}")
                try:
                    verify_listener_owner(process.pid, config.port)
                    health = fetch_health(config)
                    verify_health(config, health)
                    verify_listener_owner(process.pid, config.port)
                    return {"status": "started", "endpoint": config.endpoint, "pid": process.pid}
                except (HealthUnavailable, ListenerUnavailable):
                    time.sleep(0.25)
            raise RuntimeError("server readiness timed out")
        except BaseException as error:
            if process is not None:
                if reap_started_process(process):
                    if state is not None:
                        runtime_file(config, STATE_FILE_NAME, create_directory=False).unlink(missing_ok=True)
                else:
                    raise RuntimeError("server launch failed and the new process could not be reaped") from error
            raise


def stop(config: RuntimeConfig) -> dict[str, Any]:
    with lifecycle_lock(config):
        state = read_state(config)
        if state is None:
            return {"status": "not_running"}
        if terminate_owned(config, state):
            runtime_file(config, STATE_FILE_NAME, create_directory=False).unlink(missing_ok=True)
            return {"status": "stopped", "pid": state["pid"]}
        if owned_live_state(config) is None:
            runtime_file(config, STATE_FILE_NAME, create_directory=False).unlink(missing_ok=True)
            return {"status": "stale_state_removed"}
        raise RuntimeError("owned server did not stop within the bounded shutdown timeout")


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("check", "start", "stop"))
    parser.add_argument("--candidate-dir", required=True, type=Path)
    args = parser.parse_args(argv)
    try:
        config = read_config(args.candidate_dir)
        result = {"check": check, "start": start, "stop": stop}[args.operation](config)
    except RuntimeError as error:
        print(f"insight runtime: {error}", file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
