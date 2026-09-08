#!/usr/bin/env python3
"""Run one PLAN task through a bounded writer, review, and validation gate."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import selectors
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Sequence


MODEL = "gpt-5.6-terra"
EFFORT = "high"
BRANCH = "codex/autopilot"
STATE_DIR = Path(".mini-orca/autopilot")
VALIDATOR = Path("Makefile")
MAX_ATTEMPTS = 3  # Initial implementation plus two repairs.
MAX_CARD_BYTES = 24 * 1024
MAX_FILES = 200
MAX_FILE_BYTES = 16 * 1024 * 1024
MAX_CANDIDATE_BYTES = 64 * 1024 * 1024
MAX_DIAGNOSTIC_BYTES = 2 * 1024 * 1024
TERMINAL_PHASES = {"failed", "integrated"}
PROTECTED_FILES = {
    ".gitignore",
    ".gitmodules",
    "AGENTS.md",
    "Makefile",
    "PLAN.md",
    "scripts/autopilot.py",
    "scripts/desktop-gradle.sh",
    "scripts/quality.sh",
    "scripts/tests/test_autopilot.py",
    "scripts/validate.sh",
    "tasks/README.md",
}
TASK_ID = re.compile(r"^[A-Z][A-Z0-9]*-[0-9]{2}$")


class DispatchError(RuntimeError):
    pass


@dataclass(frozen=True)
class Task:
    task_id: str
    title: str
    dependencies: tuple[str, ...]
    status: str
    card: str


@dataclass(frozen=True)
class ProcessResult:
    code: int
    stdout: bytes
    stderr: bytes
    timed_out: bool = False
    output_exhausted: bool = False
    launch_error: str | None = None


def now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


def signal_name(number: int | None) -> str | None:
    if number is None:
        return None
    try:
        return signal.Signals(number).name
    except ValueError:
        return f"SIG{number}"


def atomic_json(path: Path, value: dict[str, Any]) -> None:
    atomic_bytes(path, json.dumps(value, sort_keys=True).encode())


def atomic_bytes(path: Path, value: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(value)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(name, path)
        directory_descriptor = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory_descriptor)
        finally:
            os.close(directory_descriptor)
    finally:
        Path(name).unlink(missing_ok=True)


def safe_state_bytes(path: Path) -> bytes:
    if path.is_symlink() or path.stat().st_size > 1024 * 1024:
        raise DispatchError(f"unsafe state file {path.name}")
    try:
        return path.read_bytes()
    except OSError as exc:
        raise DispatchError(f"invalid state file {path.name}") from exc


def parse_json_bytes(path: Path, content: bytes) -> dict[str, Any]:
    try:
        value = json.loads(content)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise DispatchError(f"invalid state file {path.name}") from exc
    if not isinstance(value, dict):
        raise DispatchError(f"invalid state file {path.name}")
    return value


def read_json(path: Path) -> dict[str, Any]:
    return parse_json_bytes(path, safe_state_bytes(path))


def clean_environment(*, auth: bool, temporary_home: Path | None = None) -> dict[str, str]:
    allowed = {
        "LANG",
        "LC_ALL",
        "PATH",
        "SHELL",
        "SSL_CERT_DIR",
        "SSL_CERT_FILE",
        "TERM",
        "TMPDIR",
    }
    if auth:
        allowed.update({"CODEX_HOME", "HOME", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"})
    else:
        allowed.update(
            {
                "CGO_ENABLED",
                "GOCACHE",
                "GOMODCACHE",
                "GOPATH",
                "GRADLE_USER_HOME",
                "JAVA_HOME",
                "MINI_ORCA_JBR25_HOME",
                "MINI_ORCA_JDK21_HOME",
            }
        )
    environment = {key: os.environ[key] for key in allowed if key in os.environ}
    environment.setdefault("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")
    environment.setdefault("LANG", "C.UTF-8")
    if temporary_home is not None:
        for key in ("GOCACHE", "GOMODCACHE", "GOPATH"):
            if key not in environment:
                probe_environment = dict(environment)
                if os.environ.get("HOME"):
                    probe_environment["HOME"] = os.environ["HOME"]
                value = subprocess.run(
                    ["go", "env", key], env=probe_environment, text=True,
                    stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=False,
                ).stdout.strip()
                if value:
                    environment[key] = value
        scratch = str(temporary_home.resolve())
        if "GRADLE_USER_HOME" not in environment and os.environ.get("HOME"):
            environment["GRADLE_USER_HOME"] = str(Path(os.environ["HOME"]) / ".gradle")
        for key in ("GOCACHE", "GOMODCACHE", "GOPATH", "GRADLE_USER_HOME", "JAVA_HOME", "MINI_ORCA_JDK21_HOME", "MINI_ORCA_JBR25_HOME"):
            if key in environment:
                environment[key] = str(Path(environment[key]).resolve())
        environment["PATH"] = os.pathsep.join(str(Path(item).resolve()) for item in environment["PATH"].split(os.pathsep) if item)
        environment.update({
            "HOME": scratch,
            "TMPDIR": scratch,
            "GRADLE_OPTS": f"-Dorg.gradle.daemon=false -Djava.io.tmpdir={scratch}",
            "JAVA_TOOL_OPTIONS": f"-Djava.io.tmpdir={scratch}",
            "MINI_ORCA_AUTOPILOT_VALIDATION": "1",
        })
        if environment.get("GOMODCACHE"):
            environment.update({
                "GOPROXY": (Path(environment["GOMODCACHE"]) / "cache" / "download").as_uri(),
                "GOSUMDB": "off",
            })
        if Path("/Library/Developer/CommandLineTools").is_dir():
            environment["DEVELOPER_DIR"] = "/Library/Developer/CommandLineTools"
    return environment


def git_environment(extra: dict[str, str] | None = None) -> dict[str, str]:
    environment = clean_environment(auth=False)
    environment.update(
        {
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_TERMINAL_PROMPT": "0",
        }
    )
    environment.update(extra or {})
    return environment


def git_bytes(
    repo: Path,
    *arguments: str,
    check: bool = True,
    extra_environment: dict[str, str] | None = None,
) -> bytes:
    command = [
        "git",
        "-c",
        "core.hooksPath=/dev/null",
        "-c",
        "core.fsmonitor=false",
        "-c",
        "core.attributesFile=/dev/null",
        *arguments,
    ]
    completed = subprocess.run(
        command,
        cwd=repo,
        env=git_environment(extra_environment),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if len(completed.stdout) + len(completed.stderr) > 16 * 1024 * 1024:
        raise DispatchError("Git output exceeded its safety limit")
    if check and completed.returncode:
        detail = completed.stderr.decode(errors="replace").strip().splitlines()
        raise DispatchError(detail[-1] if detail else f"Git command failed: {arguments[0]}")
    return completed.stdout


def git_text(repo: Path, *arguments: str, **options: Any) -> str:
    return git_bytes(repo, *arguments, **options).decode(errors="strict").strip()


def bounded_process(
    command: Sequence[str],
    cwd: Path,
    environment: dict[str, str],
    timeout_seconds: float,
    max_output_bytes: int,
    stdin: str = "",
    started: Callable[[int | None], None] | None = None,
) -> ProcessResult:
    """Run one process while retaining no more than the combined output limit.

    Pipes are drained while the child runs so a verbose child cannot block on a
    full pipe.  A process group lets the dispatcher stop descendants that keep a
    pipe open after their parent exits.
    """
    try:
        process = subprocess.Popen(
            list(command), cwd=cwd, env=environment, stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True,
        )
    except OSError as exc:
        return ProcessResult(127, b"", str(exc).encode(errors="replace"), launch_error=str(exc))

    assert process.stdin is not None and process.stdout is not None and process.stderr is not None
    timed_out = False
    output_exhausted = False
    killed = False
    kill_deadline: float | None = None
    stdout = bytearray()
    stderr = bytearray()
    input_bytes = memoryview(stdin.encode())
    input_offset = 0
    selector = selectors.DefaultSelector()

    def register(stream: Any, events: int, name: str) -> None:
        os.set_blocking(stream.fileno(), False)
        selector.register(stream, events, name)

    def close(stream: Any) -> None:
        try:
            selector.unregister(stream)
        except (KeyError, ValueError):
            pass
        try:
            stream.close()
        except OSError:
            pass

    def kill_group() -> None:
        nonlocal killed, kill_deadline
        if killed:
            return
        killed = True
        kill_deadline = time.monotonic() + 1
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass

    try:
        register(process.stdout, selectors.EVENT_READ, "stdout")
        register(process.stderr, selectors.EVENT_READ, "stderr")
        if input_bytes:
            register(process.stdin, selectors.EVENT_WRITE, "stdin")
        else:
            close(process.stdin)
        if started:
            started(process.pid)
        deadline = time.monotonic() + timeout_seconds
        while selector.get_map() or process.poll() is None:
            current = time.monotonic()
            remaining = deadline - current
            if remaining <= 0 and not killed:
                timed_out = True
                kill_group()
            if killed and kill_deadline is not None and current >= kill_deadline:
                # A detached descendant can retain a copied pipe despite the
                # process-group kill.  Its output is no longer useful.
                for stream in (process.stdin, process.stdout, process.stderr):
                    close(stream)
            events = selector.select(max(0.01, min(max(remaining, 0), 0.1))) if selector.get_map() else []
            if not events:
                if process.poll() is not None and not killed:
                    # A descendant inherited one of the pipes.  It must not keep
                    # this invocation alive after its direct parent is reaped.
                    kill_group()
                if not selector.get_map() and process.poll() is None:
                    time.sleep(0.01)
                continue
            for key, _ in events:
                stream = key.fileobj
                if key.data == "stdin":
                    try:
                        written = os.write(stream.fileno(), input_bytes[input_offset:])
                    except BlockingIOError:
                        continue
                    except BrokenPipeError:
                        close(stream)
                        continue
                    input_offset += written
                    if input_offset == len(input_bytes):
                        close(stream)
                    continue
                room = max_output_bytes - len(stdout) - len(stderr)
                try:
                    data = os.read(stream.fileno(), max(1, min(64 * 1024, room + 1)))
                except BlockingIOError:
                    continue
                if not data:
                    close(stream)
                    continue
                destination = stdout if key.data == "stdout" else stderr
                if len(data) > room:
                    destination.extend(data[:max(room, 0)])
                    output_exhausted = True
                    kill_group()
                else:
                    destination.extend(data)
        process.wait()
    finally:
        try:
            for stream in (process.stdin, process.stdout, process.stderr):
                close(stream)
            selector.close()
            if process.poll() is None:
                kill_group()
                try:
                    process.wait(timeout=1)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
        finally:
            if started:
                started(None)
    return ProcessResult(process.returncode, bytes(stdout), bytes(stderr), timed_out, output_exhausted)


class Lease:
    def __init__(self, root: Path, stale_seconds: int, recover: bool):
        self.path = root / "lease"
        self.stale_seconds = stale_seconds
        self.recover = recover

    @staticmethod
    def alive(pid: Any) -> bool:
        if not isinstance(pid, int) or pid <= 0:
            return False
        try:
            os.kill(pid, 0)
            return True
        except (ProcessLookupError, PermissionError):
            return False

    def __enter__(self) -> "Lease":
        self.path.parent.mkdir(parents=True, exist_ok=True)
        try:
            self.path.mkdir()
        except FileExistsError:
            owner_path = self.path / "owner.json"
            owner = read_json(owner_path) if owner_path.is_file() else {}
            active = self.alive(owner.get("pid")) or self.alive(owner.get("child_pid"))
            try:
                acquired = dt.datetime.fromisoformat(str(owner.get("acquired_at")))
                age = (dt.datetime.now(dt.timezone.utc) - acquired).total_seconds()
            except (TypeError, ValueError):
                age = 0
            if active:
                raise DispatchError("another dispatcher owns the active lease")
            if not self.recover or age < self.stale_seconds:
                raise DispatchError("stale lease requires --recover-stale-lease")
            recovered = self.path.with_name(f"lease.recovered.{os.getpid()}")
            os.rename(self.path, recovered)
            self.path.mkdir()
            shutil.rmtree(recovered)
        self.update(None)
        return self

    def update(self, child_pid: int | None) -> None:
        atomic_json(
            self.path / "owner.json",
            {"pid": os.getpid(), "child_pid": child_pid, "acquired_at": now()},
        )

    def __exit__(self, *_: Any) -> None:
        owner_path = self.path / "owner.json"
        try:
            owner = read_json(owner_path)
            if owner.get("pid") == os.getpid():
                owner_path.unlink()
                self.path.rmdir()
        except (DispatchError, FileNotFoundError, OSError):
            pass


def parse_plan(path: Path) -> list[Task]:
    text = path.read_text()
    headings = list(re.finditer(r"(?m)^### ([A-Z][A-Z0-9]*-[0-9]{2}) — .+$", text))
    cards: dict[str, str] = {}
    for index, heading in enumerate(headings):
        end = headings[index + 1].start() if index + 1 < len(headings) else len(text)
        card = text[heading.start():end].strip()
        if len(card.encode()) > MAX_CARD_BYTES:
            raise DispatchError(f"task {heading.group(1)} exceeds the prompt byte limit")
        cards[heading.group(1)] = card

    tasks: list[Task] = []
    for line in text.splitlines():
        cells = [cell.strip() for cell in line.split("|")]
        if len(cells) != 7 or not TASK_ID.fullmatch(cells[1]):
            continue
        status = cells[5].split(maxsplit=1)[0]
        if status not in {"Pending", "Running", "Review", "Complete", "Blocked"}:
            raise DispatchError(f"task {cells[1]} has unsupported status {status}")
        dependencies = () if cells[3] == "—" else tuple(
            item.strip() for item in cells[3].split(",")
        )
        if cells[1] not in cards:
            raise DispatchError(f"task {cells[1]} has no task card")
        tasks.append(Task(cells[1], cells[2], dependencies, status, cards[cells[1]]))
    if not tasks:
        raise DispatchError("PLAN contains no task ledger")
    identifiers = {task.task_id for task in tasks}
    if len(identifiers) != len(tasks):
        raise DispatchError("PLAN contains duplicate task identifiers")
    for task in tasks:
        missing = set(task.dependencies) - identifiers
        if missing:
            raise DispatchError(f"task {task.task_id} has unknown dependencies")
    return tasks


def select_task(tasks: list[Task], requested: str | None, used: set[str]) -> Task:
    by_id = {task.task_id: task for task in tasks}
    candidates = [by_id[requested]] if requested in by_id else []
    if requested and not candidates:
        raise DispatchError(f"unknown task {requested}")
    if not requested:
        candidates = [task for task in tasks if task.status == "Pending" and task.task_id not in used]
    for task in candidates:
        if task.status != "Pending":
            raise DispatchError(f"task {task.task_id} is {task.status}, not Pending")
        incomplete = [dependency for dependency in task.dependencies if by_id[dependency].status != "Complete"]
        if incomplete:
            if requested:
                raise DispatchError(f"incomplete dependencies: {', '.join(incomplete)}")
            continue
        return task
    raise DispatchError("no ready Pending task")


def schema(required: list[str], properties: dict[str, Any]) -> dict[str, Any]:
    return {
        "type": "object",
        "additionalProperties": False,
        "required": required,
        "properties": properties,
    }


WORKER_SCHEMA = schema(
    ["task_id", "outcome", "changed_files", "checks"],
    {
        "task_id": {"type": "string"},
        "outcome": {"enum": ["completed", "blocked"]},
        "changed_files": {"type": "array", "maxItems": MAX_FILES, "items": {"type": "string"}},
        "checks": {"type": "array", "maxItems": 64, "items": {"type": "string"}},
    },
)
REVIEW_SCHEMA = schema(
    ["task_id", "decision", "findings"],
    {
        "task_id": {"type": "string"},
        "decision": {"enum": ["accepted", "rejected"]},
        "findings": {
            "type": "array",
            "maxItems": 16,
            "items": schema(
                ["code", "message"],
                {"code": {"type": "string"}, "message": {"type": "string", "maxLength": 1000}},
            ),
        },
    },
)


def event_result(payload: bytes, role: str, task_id: str) -> tuple[dict[str, Any], int]:
    try:
        events = [json.loads(line) for line in payload.decode().splitlines() if line]
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise DispatchError(f"{role} emitted invalid JSONL") from exc
    if not all(isinstance(event, dict) for event in events):
        raise DispatchError(f"{role} emitted invalid JSONL")
    if any(event.get("type") in {"error", "turn.failed"} for event in events):
        raise DispatchError(f"{role} emitted a failed event")
    starts = [event for event in events if event.get("type") == "thread.started"]
    completions = [event for event in events if event.get("type") == "turn.completed"]
    messages = []
    for event in events:
        if event.get("type") != "item.completed":
            continue
        item = event.get("item")
        if not isinstance(item, dict):
            raise DispatchError(f"{role} emitted invalid JSONL")
        if item.get("type") == "agent_message":
            messages.append(item.get("text"))
    if len(starts) != 1 or len(completions) != 1 or not messages:
        raise DispatchError(f"{role} lacks complete event evidence")
    usage = completions[0].get("usage", {})
    if not isinstance(usage, dict):
        raise DispatchError(f"{role} lacks token evidence")
    if not all(isinstance(usage.get(key), int) and usage[key] >= 0 for key in ("input_tokens", "output_tokens")):
        raise DispatchError(f"{role} lacks token evidence")
    try:
        value = json.loads(messages[-1])
    except (TypeError, json.JSONDecodeError) as exc:
        raise DispatchError(f"invalid {role} structured output") from exc
    if not isinstance(value, dict) or value.get("task_id") != task_id:
        raise DispatchError(f"invalid {role} structured output")
    if role == "worker":
        valid = (
            set(value) == {"task_id", "outcome", "changed_files", "checks"}
            and value.get("outcome") in {"completed", "blocked"}
            and isinstance(value.get("changed_files"), list)
            and len(value["changed_files"]) <= MAX_FILES
            and all(isinstance(item, str) for item in value["changed_files"])
            and isinstance(value.get("checks"), list)
            and len(value["checks"]) <= 64
            and all(isinstance(item, str) for item in value["checks"])
        )
    else:
        findings = value.get("findings")
        valid = (
            set(value) == {"task_id", "decision", "findings"}
            and value.get("decision") in {"accepted", "rejected"}
            and isinstance(findings, list)
            and len(findings) <= 16
            and all(
                isinstance(item, dict)
                and set(item) == {"code", "message"}
                and re.fullmatch(r"[A-Z][A-Z0-9_-]{1,63}", str(item.get("code", "")))
                and isinstance(item.get("message"), str)
                and len(item["message"]) <= 1000
                for item in findings
            )
            and ((value.get("decision") == "accepted" and not findings) or (value.get("decision") == "rejected" and bool(findings)))
        )
    if not valid:
        raise DispatchError(f"invalid {role} structured output")
    return value, usage["input_tokens"] + usage["output_tokens"]


class Dispatcher:
    def __init__(self, arguments: argparse.Namespace):
        self.args = arguments
        self.repo = Path(arguments.repo).resolve()
        self.state_root = self.repo / STATE_DIR
        for path in (self.repo / STATE_DIR.parts[0], self.state_root):
            if path.is_symlink():
                raise DispatchError("local state path must not contain symlinks")
        try:
            self.state_root.resolve().relative_to(self.repo)
        except ValueError as exc:
            raise DispatchError("local state path escapes the repository") from exc
        self.runs = self.state_root / "runs"
        self.lease: Lease | None = None
        self.state_path: Path | None = None
        self.state: dict[str, Any] | None = None
        self.codex = self.resolve_codex(arguments.codex)

    @staticmethod
    def resolve_codex(value: str) -> Path:
        resolved = shutil.which(value) if os.sep not in value else value
        if not resolved or not os.access(resolved, os.X_OK):
            raise DispatchError("configured Codex CLI is missing")
        return Path(resolved).resolve()

    def configuration(self) -> dict[str, Any]:
        stat = self.codex.stat()
        return {
            "model": MODEL,
            "reasoning_effort": EFFORT,
            "codex": str(self.codex),
            "codex_identity": [stat.st_size, stat.st_mtime_ns],
            "timeout_seconds": self.args.timeout_seconds,
            "token_budget": self.args.token_budget,
            "invocation_reserve_tokens": self.args.invocation_reserve_tokens,
            "max_output_bytes": self.args.max_output_bytes,
        }

    def check_cli(self) -> None:
        result = bounded_process(
            [str(self.codex), "exec", "--help"],
            self.repo,
            clean_environment(auth=True),
            max(1, min(10, self.args.timeout_seconds)),
            256 * 1024,
        )
        help_text = (result.stdout + result.stderr).decode(errors="replace")
        required = ("--sandbox", "--json", "--output-schema", "--ephemeral", "--ignore-user-config", "--ignore-rules")
        if result.code or any(option not in help_text for option in required):
            raise DispatchError("configured Codex CLI lacks required exec features")

    def config_digest(self) -> str:
        content = git_bytes(self.repo, "config", "--local", "--null", "--list")
        return hashlib.sha256(content).hexdigest()

    def clean_base(self) -> tuple[str, str]:
        root = Path(git_text(self.repo, "rev-parse", "--show-toplevel")).resolve()
        common = Path(git_text(self.repo, "rev-parse", "--git-common-dir"))
        if not common.is_absolute():
            common = (self.repo / common).resolve()
        if root != self.repo or common != (self.repo / ".git").resolve():
            raise DispatchError("repository Git boundaries are redirected")
        branch = git_text(self.repo, "symbolic-ref", "--short", "HEAD")
        if branch != BRANCH:
            raise DispatchError(f"dispatcher requires branch {BRANCH}")
        if git_text(self.repo, "status", "--porcelain=v1", "--untracked-files=all"):
            raise DispatchError("target branch has uncommitted changes")
        head = git_text(self.repo, "rev-parse", "HEAD")
        if git_text(self.repo, "rev-parse", BRANCH) != head:
            raise DispatchError("target branch identity is inconsistent")
        attributes = git_bytes(self.repo, "ls-files", "-z").split(b"\0")
        for raw_path in attributes:
            if raw_path and Path(os.fsdecode(raw_path)).name == ".gitattributes":
                data = git_bytes(self.repo, "show", f"{head}:{os.fsdecode(raw_path)}")
                if re.search(rb"(?:filter|diff|working-tree-encoding)\s*=", data, re.I):
                    raise DispatchError("Git attributes enable executable transforms")
        if subprocess.run(
            ["git", "check-ignore", "-q", str(STATE_DIR / "probe")],
            cwd=self.repo,
            env=git_environment(),
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        ).returncode:
            raise DispatchError(f"{STATE_DIR} is not ignored")
        return head, self.config_digest()

    def states(self) -> list[tuple[Path, dict[str, Any]]]:
        if not self.runs.exists():
            return []
        return [(path, read_json(path)) for path in sorted(self.runs.glob("*.json"))]

    def active_state(self, requested: str | None) -> tuple[Path, dict[str, Any]] | None:
        states = self.states()
        active = [(path, state) for path, state in states if state.get("phase") not in TERMINAL_PHASES]
        if len(active) > 1:
            raise DispatchError("multiple active runs prevent safe dispatch")
        if active and requested and active[0][1].get("task") != requested:
            raise DispatchError(f"active run belongs to {active[0][1].get('task')}")
        return active[0] if active else None

    def save(self, phase: str | None = None) -> None:
        assert self.state is not None and self.state_path is not None
        if phase:
            self.state["phase"] = phase
        self.state["updated_at"] = now()
        atomic_json(self.state_path, self.state)

    def state_template(self, task: Task, base: str, config_digest: str) -> dict[str, Any]:
        return {
            "version": 1,
            "task": task.task_id,
            "task_card_digest": hashlib.sha256(task.card.encode()).hexdigest(),
            "base": base,
            "branch": BRANCH,
            "repository_config_digest": config_digest,
            "configuration": self.configuration(),
            "worktree": str(STATE_DIR / "worktrees" / f"{task.task_id.lower()}-{base[:12]}"),
            "phase": "selected",
            "attempt": 1,
            "tokens_used": 0,
            "pending_invocation": None,
            "invocations": [],
            "candidate": None,
            "reviewer": None,
            "validator": None,
            "integration": {"authorized": False, "commit": None},
            "created_at": now(),
        }

    def create_state(self, task: Task, base: str, config_digest: str) -> None:
        self.state_path = self.runs / f"{task.task_id}.json"
        if self.state_path.exists():
            raise DispatchError(f"task {task.task_id} already has recorded state")
        self.state = self.state_template(task, base, config_digest)
        self.save()

    def private_directory(self, name: str) -> Path:
        path = self.state_root / name
        if path.is_symlink():
            raise DispatchError(f"private {name} path must not contain symlinks")
        path.mkdir(mode=0o700, parents=True, exist_ok=True)
        if path.is_symlink() or not path.is_dir():
            raise DispatchError(f"private {name} path is invalid")
        os.chmod(path, 0o700)
        return path

    def record_invocation_failure(self, role: str, invocation_id: str, stage: str, result: ProcessResult) -> None:
        """Persist bounded private evidence without copying CLI prose into state."""
        assert self.state is not None
        diagnostics = self.private_directory("diagnostics")
        prefix = f"{self.state['task']}-{invocation_id}"
        stdout_path = diagnostics / f"{prefix}.stdout"
        stderr_path = diagnostics / f"{prefix}.stderr"
        for path, content in ((stdout_path, result.stdout), (stderr_path, result.stderr)):
            if path.is_symlink():
                raise DispatchError("private diagnostics path must not contain symlinks")
            retained = content[:MAX_DIAGNOSTIC_BYTES]
            atomic_bytes(path, retained)
            os.chmod(path, 0o600)
        signal_number = -result.code if result.code < 0 else None
        self.state["failure"] = {
            "role": role,
            "stage": stage,
            "exit_code": result.code if result.code >= 0 else None,
            "signal": signal_number,
            "signal_name": signal_name(signal_number),
            "timed_out": result.timed_out,
            "output_exhausted": result.output_exhausted,
            "diagnostics": {
                "stdout": str(stdout_path.relative_to(self.repo)),
                "stderr": str(stderr_path.relative_to(self.repo)),
                "stdout_digest": hashlib.sha256(result.stdout[:MAX_DIAGNOSTIC_BYTES]).hexdigest(),
                "stderr_digest": hashlib.sha256(result.stderr[:MAX_DIAGNOSTIC_BYTES]).hexdigest(),
                "stdout_bytes": min(len(result.stdout), MAX_DIAGNOSTIC_BYTES),
                "stderr_bytes": min(len(result.stderr), MAX_DIAGNOSTIC_BYTES),
            },
            "recorded_at": now(),
        }
        self.save()

    def verify_state(self, task: Task, base: str, config_digest: str) -> None:
        assert self.state is not None
        integration = self.state.get("integration")
        target_already_integrated = (
            self.state.get("phase") == "integrating"
            and isinstance(integration, dict)
            and integration.get("authorized") is True
            and integration.get("commit") == base
        )
        valid = (
            self.state.get("version") == 1
            and self.state.get("task") == task.task_id
            and self.state.get("branch") == BRANCH
            and (self.state.get("base") == base or target_already_integrated)
            and self.state.get("repository_config_digest") == config_digest
            and self.state.get("configuration") == self.configuration()
            and (
                self.state.get("task_card_digest") == hashlib.sha256(task.card.encode()).hexdigest()
                or target_already_integrated
            )
            and isinstance(self.state.get("attempt"), int)
            and 1 <= self.state["attempt"] <= MAX_ATTEMPTS
        )
        if not valid:
            if self.state.get("base") != base and not target_already_integrated:
                raise DispatchError("target branch changed since dispatch began")
            if self.state.get("repository_config_digest") != config_digest:
                raise DispatchError("repository Git configuration changed")
            raise DispatchError("run state does not match this dispatcher configuration")

    def verify_interrupted_recovery(
        self, task: Task, selected: dict[str, Any], journal: dict[str, Any], reason_digest: str
    ) -> None:
        recovery = selected.get("recovery")
        if not isinstance(recovery, dict) or hashlib.sha256(str(recovery.get("reason", "")).encode()).hexdigest() != reason_digest:
            raise DispatchError("recovery reason does not match the authorization")
        archive_name = recovery.get("archive")
        archives = self.private_directory("archives")
        if not isinstance(archive_name, str):
            raise DispatchError("interrupted recovery archive is invalid")
        lexical_archive = self.repo / archive_name
        if lexical_archive.is_symlink():
            raise DispatchError("interrupted recovery archive is invalid")
        archive_path = lexical_archive.resolve()
        try:
            archive_path.relative_to(archives.resolve())
        except ValueError as exc:
            raise DispatchError("interrupted recovery archive is outside private state") from exc
        if archive_path.is_symlink() or not archive_path.is_file() or archive_path.stat().st_size > 1024 * 1024:
            raise DispatchError("interrupted recovery archive is invalid")
        archive_bytes = archive_path.read_bytes()
        archive_digest = hashlib.sha256(archive_bytes).hexdigest()
        if archive_digest != journal.get("old_digest") or archive_digest != recovery.get("archive_digest"):
            raise DispatchError("interrupted recovery archive identity is invalid")
        archived = parse_json_bytes(archive_path, archive_bytes)
        old_configuration = archived.get("configuration")
        selected_configuration = selected.get("configuration")
        old_tokens = archived.get("tokens_used")
        old_attempt = archived.get("attempt")
        pending = archived.get("pending_invocation")
        reserved = pending.get("reserved_tokens") if isinstance(pending, dict) else 0
        if (
            archived.get("phase") != "failed"
            or archived.get("task") != task.task_id
            or not isinstance(old_attempt, int)
            or not isinstance(old_tokens, int)
            or not isinstance(reserved, int)
            or old_tokens < reserved
            or selected.get("attempt") != old_attempt + 1
            or selected.get("tokens_used") != old_tokens
            or not isinstance(old_configuration, dict)
            or not isinstance(selected_configuration, dict)
            or selected_configuration.get("token_budget") != old_configuration.get("token_budget")
            or selected_configuration.get("invocation_reserve_tokens") != old_configuration.get("invocation_reserve_tokens")
            or recovery.get("prior_attempt") != old_attempt
            or recovery.get("prior_tokens_used") != old_tokens
        ):
            raise DispatchError("interrupted recovery accounting is invalid")

    def recover_failed_run(self, task: Task, tasks: list[Task], base: str, config_digest: str) -> dict[str, Any]:
        """Replace one failed run only after immutable archival and identity checks."""
        if task.status != "Pending":
            raise DispatchError(f"task {task.task_id} is {task.status}, not Pending")
        select_task(tasks, task.task_id, set())
        assert self.args.recovery_authorization_id and self.args.recovery_reason
        authorization_id = self.args.recovery_authorization_id
        if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._:-]{7,127}", authorization_id):
            raise DispatchError("recovery authorization id is invalid")
        if len(self.args.recovery_reason.encode()) > 1000:
            raise DispatchError("recovery reason exceeds the byte limit")
        active = self.active_state(task.task_id)
        active_recovery = active[1].get("recovery") if active else None
        if active and (not isinstance(active_recovery, dict) or active_recovery.get("authorization_id") != authorization_id):
            raise DispatchError("an active run prevents failed-run recovery")
        state_path = self.runs / f"{task.task_id}.json"
        if not state_path.is_file() or state_path.is_symlink():
            raise DispatchError(f"task {task.task_id} has no failed run to recover")
        raw_state = safe_state_bytes(state_path)
        old_state = parse_json_bytes(state_path, raw_state)
        authorization_digest = hashlib.sha256(authorization_id.encode()).hexdigest()
        reason_digest = hashlib.sha256(self.args.recovery_reason.encode()).hexdigest()
        old_digest = hashlib.sha256(raw_state).hexdigest()
        journals = self.private_directory("recoveries")
        journal_path = journals / f"{authorization_digest}.json"
        if journal_path.exists():
            journal = read_json(journal_path)
            if journal.get("task") != task.task_id:
                raise DispatchError("recovery authorization id was already used")
            if journal.get("reason_digest") != reason_digest:
                raise DispatchError("recovery reason does not match the authorization")
            if journal.get("complete") is True:
                raise DispatchError("recovery authorization id was already used")
            recovery = old_state.get("recovery")
            if isinstance(recovery, dict) and recovery.get("authorization_id") == authorization_id:
                self.verify_interrupted_recovery(task, old_state, journal, reason_digest)
                self.state_path, self.state = state_path, old_state
                self.verify_state(task, base, config_digest)
                if self.state.get("phase") != "selected":
                    raise DispatchError("interrupted recovery state is invalid")
                journal["complete"] = True
                journal["completed_at"] = now()
                atomic_json(journal_path, journal)
                return {
                    "task": task.task_id,
                    "phase": "selected",
                    "attempt": self.state["attempt"],
                    "tokens_used": self.state["tokens_used"],
                    "recovered": True,
                }
            if journal.get("old_digest") != old_digest:
                raise DispatchError("recovery authorization id was already used")
        else:
            journal = {
                "task": task.task_id,
                "old_digest": old_digest,
                "authorization_digest": authorization_digest,
                "reason_digest": reason_digest,
                "created_at": now(),
                "complete": False,
            }
            atomic_json(journal_path, journal)

        if old_state.get("phase") != "failed" or old_state.get("task") != task.task_id:
            raise DispatchError(f"task {task.task_id} has no failed run to recover")
        attempt = old_state.get("attempt")
        tokens_used = old_state.get("tokens_used")
        old_configuration = old_state.get("configuration")
        if not isinstance(attempt, int) or not 1 <= attempt < MAX_ATTEMPTS:
            raise DispatchError("failed run exhausted its attempt budget")
        if not isinstance(old_configuration, dict):
            raise DispatchError("failed run configuration is invalid")
        old_budget = old_configuration.get("token_budget")
        old_reserve = old_configuration.get("invocation_reserve_tokens")
        pending = old_state.get("pending_invocation")
        pending_reserve = pending.get("reserved_tokens") if isinstance(pending, dict) else 0
        if not isinstance(old_budget, int) or old_budget <= 0 or not isinstance(old_reserve, int) or old_reserve <= 0:
            raise DispatchError("failed run configuration is invalid")
        if self.args.token_budget != old_budget or self.args.invocation_reserve_tokens != old_reserve:
            raise DispatchError("failed-run recovery requires its original token configuration")
        if not isinstance(pending_reserve, int) or pending_reserve < 0 or not isinstance(tokens_used, int) or tokens_used < pending_reserve:
            raise DispatchError("failed run reservation accounting is invalid")
        if tokens_used + self.args.invocation_reserve_tokens > self.args.token_budget:
            raise DispatchError("failed run exhausted its token budget")
        self.verify_failed_candidate(old_state)

        archives = self.private_directory("archives")
        archive_path = archives / f"{task.task_id}-{old_digest[:16]}-{authorization_digest[:16]}.json"
        if archive_path.exists():
            if archive_path.is_symlink() or archive_path.read_bytes() != raw_state:
                raise DispatchError("failed run archive identity is invalid")
        else:
            atomic_bytes(archive_path, raw_state)
            os.chmod(archive_path, 0o600)

        new_state = self.state_template(task, base, config_digest)
        new_state.update(
            {
                "worktree": str(STATE_DIR / "worktrees" / f"{task.task_id.lower()}-recovery-{base[:12]}-{authorization_digest[:8]}"),
                "attempt": attempt + 1,
                "tokens_used": tokens_used,
                "recovery": {
                    "authorization_id": authorization_id,
                    "reason": self.args.recovery_reason,
                    "archive": str(archive_path.relative_to(self.repo)),
                    "archive_digest": old_digest,
                    "prior_attempt": attempt,
                    "prior_tokens_used": tokens_used,
                    "prepared_at": now(),
                },
            }
        )
        existing = read_json(state_path)
        existing_recovery = existing.get("recovery")
        if isinstance(existing_recovery, dict) and existing_recovery.get("authorization_id") == authorization_id:
            self.state_path, self.state = state_path, existing
        elif safe_state_bytes(state_path) == raw_state:
            atomic_json(state_path, new_state)
            self.state_path, self.state = state_path, new_state
        else:
            raise DispatchError("failed run state changed during recovery")
        journal["complete"] = True
        journal["completed_at"] = now()
        atomic_json(journal_path, journal)
        return {
            "task": task.task_id,
            "phase": "selected",
            "attempt": self.state["attempt"],
            "tokens_used": self.state["tokens_used"],
            "recovered": True,
        }

    def worktree(self) -> Path:
        assert self.state is not None
        return self.recorded_worktree(self.state)

    def recorded_worktree(self, state: dict[str, Any]) -> Path:
        recorded = state.get("worktree")
        if not isinstance(recorded, str):
            raise DispatchError("recorded worktree is invalid")
        path = (self.repo / recorded).resolve()
        expected = (self.state_root / "worktrees").resolve()
        try:
            path.relative_to(expected)
        except ValueError as exc:
            raise DispatchError("recorded worktree is outside local state") from exc
        return path

    def candidate_snapshot(self, worktree: Path, base: str) -> dict[str, Any]:
        descriptor, name = tempfile.mkstemp(prefix="recovery-index-", dir=self.state_root)
        os.close(descriptor)
        Path(name).unlink()
        environment = {"GIT_INDEX_FILE": name}
        try:
            git_bytes(worktree, "read-tree", base, extra_environment=environment)
            git_bytes(worktree, "add", "-A", "--", ".", extra_environment=environment)
            tree = git_text(worktree, "write-tree", extra_environment=environment)
        finally:
            Path(name).unlink(missing_ok=True)
        raw_files = git_bytes(worktree, "diff-tree", "--no-commit-id", "--name-only", "-r", "-z", base, tree)
        files = sorted(os.fsdecode(item) for item in raw_files.split(b"\0") if item)
        diff = git_bytes(worktree, "diff", "--binary", "--no-ext-diff", base, tree)
        return {"tree": tree, "diff": hashlib.sha256(diff).hexdigest(), "files": files}

    def verify_failed_candidate(self, state: dict[str, Any]) -> None:
        worktree = self.recorded_worktree(state)
        base = state.get("base")
        if not isinstance(base, str) or not worktree.is_dir():
            raise DispatchError("failed run worktree is missing")
        if git_text(worktree, "rev-parse", "HEAD") != base:
            raise DispatchError("failed run worktree history changed")
        if git_bytes(worktree, "symbolic-ref", "-q", "HEAD", check=False):
            raise DispatchError("failed run worktree is attached to a branch")
        ignored = git_text(worktree, "status", "--porcelain=v1", "--ignored=matching")
        if any(line.startswith("!! ") for line in ignored.splitlines()):
            raise DispatchError("failed run worktree contains ignored output")
        current = self.candidate_snapshot(worktree, base)
        recorded = state.get("candidate")
        if recorded is None:
            if current["tree"] != git_text(worktree, "rev-parse", f"{base}^{{tree}}"):
                raise DispatchError("failed run worktree has an unrecorded candidate")
            return
        if current != recorded:
            raise DispatchError("failed run candidate changed")

    def prepare_worktree(self) -> Path:
        assert self.state is not None
        path = self.worktree()
        base = str(self.state["base"])
        if self.state["phase"] == "selected":
            if path.exists():
                raise DispatchError("unregistered worktree path already exists")
            path.parent.mkdir(parents=True, exist_ok=True)
            git_text(self.repo, "worktree", "add", "--detach", str(path), base)
            self.save("ready")
        if not path.is_dir():
            raise DispatchError("recorded worktree is missing")
        if git_text(path, "rev-parse", "HEAD") != base:
            raise DispatchError("isolated worktree history changed")
        if git_bytes(path, "symbolic-ref", "-q", "HEAD", check=False):
            raise DispatchError("isolated worktree is attached to a branch")
        if self.config_digest() != self.state["repository_config_digest"]:
            raise DispatchError("repository Git configuration changed")
        return path

    def candidate(self, worktree: Path, *, allow_plan: bool = False) -> dict[str, Any]:
        assert self.state is not None
        if self.config_digest() != self.state["repository_config_digest"]:
            raise DispatchError("repository Git configuration changed")
        if git_text(self.repo, "rev-parse", BRANCH) != self.state["base"]:
            raise DispatchError("worker changed the target branch")
        if git_text(self.repo, "status", "--porcelain=v1", "--untracked-files=all"):
            raise DispatchError("worker changed the target checkout")
        if git_text(worktree, "rev-parse", "HEAD") != self.state["base"]:
            raise DispatchError("worker changed Git history")
        descriptor, name = tempfile.mkstemp(prefix="index-", dir=self.state_root)
        os.close(descriptor)
        Path(name).unlink()
        environment = {"GIT_INDEX_FILE": name}
        try:
            git_bytes(worktree, "read-tree", self.state["base"], extra_environment=environment)
            git_bytes(worktree, "add", "-A", "--", ".", extra_environment=environment)
            tree = git_text(worktree, "write-tree", extra_environment=environment)
        finally:
            Path(name).unlink(missing_ok=True)
        raw_files = git_bytes(worktree, "diff-tree", "--no-commit-id", "--name-only", "-r", "-z", self.state["base"], tree)
        files = sorted(os.fsdecode(item) for item in raw_files.split(b"\0") if item)
        if not files:
            raise DispatchError("worker produced no reviewable diff")
        if len(files) > MAX_FILES or any("\n" in path or "\r" in path for path in files):
            raise DispatchError("candidate exceeds the changed-file limit")
        protected = [
            path for path in files
            if (path in PROTECTED_FILES and not (allow_plan and path == "PLAN.md"))
            or Path(path).name == ".gitattributes"
            or path.startswith(".github/workflows/")
        ]
        if protected:
            raise DispatchError(f"worker changed protected control files: {', '.join(protected)}")
        total = 0
        for relative in files:
            path = worktree / relative
            if path.is_symlink():
                raise DispatchError(f"candidate contains a changed symlink: {relative}")
            if path.is_file():
                size = path.stat().st_size
                if size > MAX_FILE_BYTES:
                    raise DispatchError(f"candidate file exceeds size limit: {relative}")
                total += size
        if total > MAX_CANDIDATE_BYTES:
            raise DispatchError("candidate exceeds the aggregate size limit")
        diff = git_bytes(worktree, "diff", "--binary", "--no-ext-diff", self.state["base"], tree)
        return {"tree": tree, "diff": hashlib.sha256(diff).hexdigest(), "files": files}

    def discard_ignored_worker_output(self, worktree: Path) -> None:
        assert self.state is not None
        if git_text(worktree, "rev-parse", "HEAD") != self.state["base"]:
            raise DispatchError("worker changed Git history")
        git_bytes(worktree, "read-tree", self.state["base"])
        git_bytes(worktree, "add", "-A", "--", ".")
        git_bytes(worktree, "clean", "-fdx", "--", ".")

    def prompt(self, role: str, task: Task) -> str:
        assert self.state is not None
        repair = self.state.get("repair")
        repair_text = ""
        if repair:
            repair_text = "\nRepair the prior gate result:\n" + "\n".join(
                f"- {item['code']}: {item['message']}" for item in repair
            )
        if role == "worker":
            return (
                f"Implement only {task.task_id} in this isolated worktree. Follow AGENTS.md and the supplied task card. "
                "Do not edit automation controls, PLAN.md, Git history, files outside the worktree, push, release, deploy, "
                "or call external services. Run focused tests. Return only the required JSON.\n\n"
                f"{task.card}{repair_text}\n"
            )
        return (
            f"Freshly review only {task.task_id}. Inspect the exact uncommitted diff, callers, tests, AGENTS.md, and the "
            "supplied task card. Do not edit, call external services, or rely on worker claims. Return accepted only when "
            f"there are no blocking findings, using only the required JSON.\n\n{task.card}\n"
        )

    def invoke(self, role: str, task: Task, worktree: Path) -> dict[str, Any]:
        assert self.state is not None
        if self.state.get("pending_invocation"):
            raise DispatchError("an interrupted paid invocation cannot be retried automatically")
        reserve = self.args.invocation_reserve_tokens
        if self.state["tokens_used"] + reserve > self.args.token_budget:
            raise DispatchError("token budget cannot reserve another invocation")
        invocation_id = f"{role}-{self.state['attempt']}"
        self.state["tokens_used"] += reserve
        self.state["pending_invocation"] = {
            "id": invocation_id,
            "role": role,
            "reserved_tokens": reserve,
            "started_at": now(),
        }
        self.save()
        descriptor, schema_path = tempfile.mkstemp(prefix="schema-", suffix=".json", dir=self.state_root)
        selected_schema = WORKER_SCHEMA if role == "worker" else REVIEW_SCHEMA
        with os.fdopen(descriptor, "w") as stream:
            json.dump(selected_schema, stream)
        sandbox = "workspace-write" if role == "worker" else "read-only"
        command = [
            str(self.codex),
            "--ask-for-approval",
            "never",
            "exec",
            "--ignore-user-config",
            "--ignore-rules",
            "--strict-config",
            "-c",
            f'model_reasoning_effort="{EFFORT}"',
            "-c",
            "mcp_servers={}",
            "-c",
            "hooks={}",
            "-c",
            "shell_environment_policy.inherit=none",
            "--model",
            MODEL,
            "--sandbox",
            sandbox,
            "--ephemeral",
            "--json",
            "--output-schema",
            schema_path,
            "-C",
            str(worktree),
            "-",
        ]
        try:
            result = bounded_process(
                command,
                worktree,
                clean_environment(auth=True),
                self.args.timeout_seconds,
                self.args.max_output_bytes,
                self.prompt(role, task),
                self.lease.update if self.lease else None,
            )
        finally:
            Path(schema_path).unlink(missing_ok=True)
        if result.launch_error:
            self.record_invocation_failure(role, invocation_id, "launch", result)
            raise DispatchError(f"{role} launch failed")
        if result.timed_out:
            self.record_invocation_failure(role, invocation_id, "timeout", result)
            raise DispatchError(f"{role} timed out")
        if result.output_exhausted:
            self.record_invocation_failure(role, invocation_id, "output_limit", result)
            raise DispatchError(f"{role} output limit exhausted")
        if result.code < 0:
            self.record_invocation_failure(role, invocation_id, "signal", result)
            raise DispatchError(f"{role} process terminated by {signal_name(-result.code)}")
        if result.code:
            self.record_invocation_failure(role, invocation_id, "exit", result)
            raise DispatchError(f"{role} process failed with exit status {result.code}")
        try:
            value, tokens = event_result(result.stdout, role, task.task_id)
        except DispatchError:
            self.record_invocation_failure(role, invocation_id, "events", result)
            raise
        self.state["tokens_used"] += tokens - reserve
        self.state["invocations"].append(
            {
                "id": invocation_id,
                "role": role,
                "tokens": tokens,
                "events_digest": hashlib.sha256(result.stdout).hexdigest(),
                "finished_at": now(),
            }
        )
        self.state["pending_invocation"] = None
        self.save()
        if self.state["tokens_used"] > self.args.token_budget:
            raise DispatchError("token budget exhausted")
        return value

    def validate(self, worktree: Path, reviewed: dict[str, Any]) -> dict[str, Any]:
        assert self.state is not None
        control = worktree / VALIDATOR
        base_control = git_bytes(worktree, "show", f"{self.state['base']}:{VALIDATOR}")
        if control.is_symlink() or not control.is_file() or control.read_bytes() != base_control:
            raise DispatchError("fixed validation control differs from the captured base")
        with tempfile.TemporaryDirectory(prefix="validation-", dir=self.state_root) as directory:
            home = Path(directory)
            environment = clean_environment(auth=False, temporary_home=home)
            result = bounded_process(
                self.validation_command(worktree, home, ["make", f"TMPDIR={home.resolve()}", "check"], environment),
                worktree,
                environment,
                self.args.timeout_seconds,
                self.args.max_output_bytes,
                started=self.lease.update if self.lease else None,
            )
        if result.timed_out:
            raise DispatchError("validation timed out")
        if result.output_exhausted:
            raise DispatchError("validation output limit exhausted")
        self.discard_ignored_worker_output(worktree)
        after = self.candidate(worktree)
        if after != reviewed:
            raise DispatchError("validator changed the reviewed candidate")
        stages = []
        for line in result.stdout.decode(errors="replace").splitlines():
            match = re.fullmatch(r"(PASS|FAIL) (.+)", line.strip())
            if match:
                stages.append({"name": match.group(2), "outcome": match.group(1).lower()})
        return {
            "command": "make check",
            "code": result.code,
            "diff": after["diff"],
            "output_digest": hashlib.sha256(result.stdout + result.stderr).hexdigest(),
            "stages": stages,
            "finished_at": now(),
        }

    def validation_command(
        self, worktree: Path, temporary: Path, command: Sequence[str], environment: dict[str, str]
    ) -> list[str]:
        launcher = Path("/usr/bin/sandbox-exec")
        if sys.platform != "darwin" or not launcher.is_file():
            raise DispatchError("secure validation requires macOS sandbox-exec")

        def escaped(path: Path) -> str:
            return str(path.resolve()).replace("\\", "\\\\").replace('"', '\\"')

        readable = {worktree.resolve(), temporary.resolve(), Path(sys.prefix).resolve()}
        readable.update(Path(item).resolve() for item in environment["PATH"].split(os.pathsep) if item)
        if os.environ.get("HOME"):
            sdkman_java = Path(os.environ["HOME"]) / ".sdkman" / "candidates" / "java"
            if sdkman_java.is_dir():
                readable.add(sdkman_java.resolve())
        for key in (
            "GOCACHE", "GOMODCACHE", "GOPATH", "GRADLE_USER_HOME", "JAVA_HOME",
            "MINI_ORCA_JDK21_HOME", "MINI_ORCA_JBR25_HOME",
        ):
            if environment.get(key):
                readable.add(Path(environment[key]).resolve())
        common = Path(git_text(worktree, "rev-parse", "--git-common-dir"))
        readable.add((worktree / common).resolve() if not common.is_absolute() else common.resolve())
        readable.update(Path(item) for item in (
            "/System", "/usr", "/bin", "/sbin", "/Library", "/private/etc",
            "/private/var/db", "/dev", "/opt/homebrew", "/usr/local",
        ))
        ancestors = {parent for path in readable for parent in path.parents}
        reads = " ".join(f'(subpath "{escaped(path)}")' for path in sorted(readable, key=str))
        reads += " " + " ".join(f'(literal "{escaped(path)}")' for path in sorted(ancestors, key=str))
        reads += (
            ' (subpath "/var/select") (subpath "/private/var/select")'
            ' (literal "/var/select/developer_dir")'
            ' (literal "/private/var/select/developer_dir")'
        )
        writable = [worktree, temporary]
        writable.extend(Path(environment[key]) for key in ("GOCACHE", "GRADLE_USER_HOME") if environment.get(key))
        writes = " ".join(f'(subpath "{escaped(path)}")' for path in writable)
        profile = temporary / "validation.sb"
        profile.write_text(
            "(version 1)\n(deny default)\n(allow process*)\n(allow signal)\n"
            "(allow sysctl-read)\n(allow mach-lookup)\n(allow ipc*)\n"
            "(allow network* (local ip))\n"
            f"(allow file-read* {reads})\n"
            f'(allow file-write* {writes} (literal "/dev/null"))\n'
        )
        return [str(launcher), "-f", str(profile), *command]

    def repair_or_fail(self, reason: str, findings: list[dict[str, str]]) -> None:
        assert self.state is not None
        if self.state["attempt"] >= MAX_ATTEMPTS:
            raise DispatchError(f"{reason} after two repairs")
        self.state["attempt"] += 1
        self.state["repair"] = findings
        self.state["candidate"] = None
        self.state["reviewer"] = None
        self.state["validator"] = None
        self.save("ready")

    def mark_plan_complete(self, worktree: Path, task: Task) -> None:
        assert self.state is not None
        path = worktree / "PLAN.md"
        text = path.read_text()
        lines = text.splitlines(keepends=True)
        row_index = next((index for index, line in enumerate(lines) if line.startswith(f"| {task.task_id} |")), None)
        if row_index is None:
            raise DispatchError("task ledger row disappeared")
        cells = lines[row_index].rstrip("\n").split("|")
        cells[5] = " Complete "
        lines[row_index] = "|".join(cells) + "\n"
        updated = "".join(lines)
        evidence = (
            f"- Automated acceptance {now()[:10]}: attempt {self.state['attempt']}; a fresh review accepted diff "
            f"`{self.state['candidate']['diff'][:12]}` and `make check` passed."
        )
        heading = re.search(rf"(?m)^### {re.escape(task.task_id)} — .+$", updated)
        if not heading:
            raise DispatchError("task card disappeared")
        next_heading = re.search(r"(?m)^### [A-Z][A-Z0-9]*-[0-9]{2} — .+$", updated[heading.end():])
        insert_at = heading.end() + next_heading.start() if next_heading else len(updated)
        section = updated[heading.start():insert_at]
        if evidence not in section:
            updated = updated[:insert_at].rstrip() + "\n" + evidence + "\n\n" + updated[insert_at:].lstrip("\n")
        path.write_text(updated)

    def integrate(self, task: Task, worktree: Path) -> None:
        assert self.state is not None
        base, config_digest = self.clean_base()
        if base != self.state["base"] or config_digest != self.state["repository_config_digest"]:
            raise DispatchError("target branch changed before integration")
        reviewed = self.candidate(worktree)
        if reviewed != self.state.get("candidate"):
            raise DispatchError("reviewed candidate changed before integration")
        self.mark_plan_complete(worktree, task)
        result = self.candidate(worktree, allow_plan=True)
        self.state["result"] = result
        self.state["integration"] = {"authorized": True, "commit": None, "requested_at": now()}
        self.save("integrating")
        self.reconcile_integration(worktree)

    def reconcile_integration(self, worktree: Path) -> None:
        assert self.state is not None
        integration = self.state["integration"]
        if not integration.get("authorized") or not self.state.get("result"):
            raise DispatchError("integration lacks explicit authorization or result identity")
        head = git_text(worktree, "rev-parse", "HEAD")
        if head == self.state["base"]:
            candidate = self.candidate(worktree, allow_plan=True)
            if candidate != self.state["result"]:
                raise DispatchError("integration candidate changed")
            git_bytes(worktree, "add", "-A", "--", ".")
            staged_tree = git_text(worktree, "write-tree")
            if staged_tree != candidate["tree"]:
                raise DispatchError("staged tree differs from reviewed result")
            git_bytes(
                worktree,
                "-c",
                "user.name=Mini-Orca Autopilot",
                "-c",
                "user.email=autopilot@mini-orca.invalid",
                "commit",
                "-m",
                f"chore: complete {self.state['task']}",
            )
            head = git_text(worktree, "rev-parse", "HEAD")
        parent = git_text(worktree, "rev-parse", f"{head}^")
        tree = git_text(worktree, "rev-parse", f"{head}^{{tree}}")
        if parent != self.state["base"] or tree != self.state["result"]["tree"]:
            raise DispatchError("local task commit has unexpected identity")
        integration["commit"] = head
        self.save("integrating")
        target_head, config_digest = self.clean_base()
        if config_digest != self.state["repository_config_digest"]:
            raise DispatchError("repository Git configuration changed")
        if target_head == self.state["base"]:
            git_bytes(self.repo, "merge", "--ff-only", head)
        elif target_head != head:
            raise DispatchError("target branch diverged during integration")
        if git_text(self.repo, "rev-parse", "HEAD") != head:
            raise DispatchError("local fast-forward did not reach the task commit")
        self.save("integrated")

    def run_state(self, task: Task, integrate: bool) -> dict[str, Any]:
        assert self.state is not None
        try:
            if self.state.get("pending_invocation"):
                raise DispatchError("an interrupted paid invocation cannot be retried automatically")
            if self.state["phase"] == "integrating":
                if not integrate:
                    raise DispatchError("integration was interrupted; resume with --integrate")
                self.reconcile_integration(self.worktree())
            worktree = self.prepare_worktree() if self.state["phase"] != "integrated" else self.worktree()
            while self.state["phase"] not in {"awaiting_integration", "integrated"}:
                if self.state["phase"] == "ready":
                    worker = self.invoke("worker", task, worktree)
                    if worker["outcome"] != "completed":
                        raise DispatchError("worker reported blocked")
                    self.discard_ignored_worker_output(worktree)
                    self.state["candidate"] = self.candidate(worktree)
                    self.state["repair"] = None
                    self.save("worker_complete")
                if self.state["phase"] == "worker_complete":
                    before = self.candidate(worktree)
                    if before != self.state["candidate"]:
                        raise DispatchError("candidate changed before review")
                    review = self.invoke("reviewer", task, worktree)
                    after = self.candidate(worktree)
                    if after != before:
                        raise DispatchError("reviewer changed the candidate")
                    self.state["reviewer"] = {
                        "decision": review["decision"],
                        "finding_codes": [finding["code"] for finding in review["findings"]],
                        "diff": after["diff"],
                        "finished_at": now(),
                    }
                    self.save("review_complete")
                    if review["decision"] == "rejected":
                        self.repair_or_fail("review rejected", review["findings"])
                        continue
                if self.state["phase"] == "review_complete":
                    evidence = self.validate(worktree, self.state["candidate"])
                    self.state["validator"] = evidence
                    self.save("validation_complete")
                    if evidence["code"]:
                        self.repair_or_fail(
                            "validation failed",
                            [{"code": "VALIDATION_FAILED", "message": "The fixed repository validation gate failed."}],
                        )
                        continue
                    self.save("awaiting_integration")
            if self.state["phase"] == "awaiting_integration" and integrate:
                self.integrate(task, worktree)
            return {
                "task": task.task_id,
                "phase": self.state["phase"],
                "attempt": self.state["attempt"],
                "tokens_used": self.state["tokens_used"],
            }
        except DispatchError as exc:
            if self.state["phase"] not in TERMINAL_PHASES and self.state["phase"] != "awaiting_integration":
                self.state["error"] = str(exc)
                self.save("failed")
            raise

    def dispatch(self) -> dict[str, Any]:
        if self.args.timeout_seconds <= 0 or self.args.max_output_bytes < 1024:
            raise DispatchError("timeout and output limits must be positive")
        if self.args.token_budget <= 0 or self.args.invocation_reserve_tokens <= 0:
            raise DispatchError("token limits must be positive")
        if self.args.recover_failed_run:
            if self.args.dry_run or self.args.integrate:
                raise DispatchError("failed-run recovery cannot dry-run or integrate")
            if not self.args.task or not self.args.recovery_authorization_id or not self.args.recovery_reason:
                raise DispatchError("failed-run recovery requires --task, --recovery-authorization-id, and --recovery-reason")
            self.state_root.mkdir(parents=True, exist_ok=True)
            with Lease(self.state_root, self.args.lease_stale_seconds, self.args.recover_stale_lease) as lease:
                self.lease = lease
                base, config_digest = self.clean_base()
                tasks = parse_plan(self.repo / "PLAN.md")
                task = next((item for item in tasks if item.task_id == self.args.task), None)
                if task is None:
                    raise DispatchError(f"unknown task {self.args.task}")
                return self.recover_failed_run(task, tasks, base, config_digest)
        self.check_cli()
        base, config_digest = self.clean_base()
        tasks = parse_plan(self.repo / "PLAN.md")
        current = self.active_state(self.args.task)
        if any(task.status in {"Running", "Review"} for task in tasks) and not current:
            raise DispatchError("PLAN has an active task without local run state")
        used = {state.get("task") for _, state in self.states()}
        if current:
            state_path, state = current
            task = next((item for item in tasks if item.task_id == state.get("task")), None)
            if task is None:
                raise DispatchError("recorded task disappeared from PLAN")
            resume = True
        else:
            task = select_task(tasks, self.args.task, used)
            state_path = None
            state = None
            resume = False
        if self.args.dry_run:
            return {"task": task.task_id, "phase": state.get("phase", "selected") if state else "selected", "resume": resume}
        self.state_root.mkdir(parents=True, exist_ok=True)
        with Lease(self.state_root, self.args.lease_stale_seconds, self.args.recover_stale_lease) as lease:
            self.lease = lease
            base, config_digest = self.clean_base()
            if state is None:
                self.create_state(task, base, config_digest)
            else:
                self.state_path, self.state = state_path, state
                self.verify_state(task, base, config_digest)
            return self.run_state(task, self.args.integrate)


def arguments(argv: Sequence[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default=".", help=argparse.SUPPRESS)
    parser.add_argument("--codex", default="codex", help="Codex CLI executable (test injection only)")
    parser.add_argument("--task")
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--integrate", action="store_true", help="authorize a local task commit and fast-forward")
    parser.add_argument("--recover-stale-lease", action="store_true")
    parser.add_argument("--recover-failed-run", action="store_true", help="offline replacement of one selected failed run")
    parser.add_argument("--recovery-authorization-id")
    parser.add_argument("--recovery-reason")
    parser.add_argument("--lease-stale-seconds", type=int, default=3600)
    parser.add_argument("--timeout-seconds", type=float, default=1800)
    parser.add_argument("--token-budget", type=int, default=200000)
    parser.add_argument("--invocation-reserve-tokens", type=int, default=20000)
    parser.add_argument("--max-output-bytes", type=int, default=2 * 1024 * 1024)
    parsed = parser.parse_args(argv)
    if parsed.integrate and parsed.dry_run:
        parser.error("--integrate cannot be combined with --dry-run")
    return parsed


def main(argv: Sequence[str] | None = None) -> int:
    try:
        print(json.dumps(Dispatcher(arguments(sys.argv[1:] if argv is None else argv)).dispatch(), sort_keys=True))
        return 0
    except DispatchError as exc:
        print(f"autopilot: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
