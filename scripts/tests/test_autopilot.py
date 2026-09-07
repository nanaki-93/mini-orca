from __future__ import annotations

import datetime as dt
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
AUTOPILOT = ROOT / "scripts" / "autopilot.py"
sys.path.insert(0, str(ROOT / "scripts"))
import autopilot  # noqa: E402

PLAN = """# Test plan

| ID | Deliverable | Dependencies | Effort / risk | Status |
| --- | --- | --- | --- | --- |
| TST-00 | Baseline | — | S / low | Complete |
| TST-01 | Ready task | TST-00 | S / low | Pending |
| TST-02 | Dependent task | TST-01 | S / low | Pending |
| TST-03 | Blocked task | TST-00 | S / low | Blocked — evidence missing |

### TST-00 — Baseline
- Already complete.

### TST-01 — Ready task
- Add one deterministic result file.
- Accept: tests pass.

### TST-02 — Dependent task
- Must wait for TST-01.

### TST-03 — Blocked task
- Must not run.
"""

FAKE_CODEX = r'''#!/usr/bin/env python3
import json
import re
import subprocess
import sys
import time
from pathlib import Path

if sys.argv[1:] == ["exec", "--help"]:
    print("--sandbox --json --output-schema --ephemeral --ignore-user-config --ignore-rules")
    raise SystemExit(0)

control_path = Path(__file__).with_name("control.json")
control = json.loads(control_path.read_text())
args = sys.argv[1:]
worktree = Path(args[args.index("-C") + 1])
mode = (worktree / "mode.txt").read_text().strip()
role = "worker" if args[args.index("--sandbox") + 1] == "workspace-write" else "reviewer"
control[f"{role}_calls"] = control.get(f"{role}_calls", 0) + 1
control.setdefault("argv", []).append(args)
control_path.write_text(json.dumps(control))
call = control[f"{role}_calls"]
prompt = sys.stdin.read()
task_id = re.search(r"\b[A-Z][A-Z0-9]*-[0-9]{2}\b", prompt).group(0)

if role == "worker" and mode == "timeout":
    time.sleep(10)
if role == "worker" and mode == "crash":
    raise SystemExit(7)
if role == "worker":
    (worktree / "result.txt").write_text(f"worker {call}\n")
    if mode == "protected":
        (worktree / "PLAN.md").write_text("tampered\n")
    if mode == "validator_control":
        (worktree / "scripts" / "validate.sh").write_text("#!/bin/sh\nexit 0\n")
    if mode == "quality_control":
        (worktree / "scripts" / "quality.sh").write_text("#!/bin/sh\nexit 0\n")
    if mode == "ignored_poison":
        (worktree / "ignored").mkdir()
        (worktree / "ignored" / "poison").write_text("hidden\n")
    if mode == "history":
        subprocess.run(["git", "add", "result.txt"], cwd=worktree, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "hostile"], cwd=worktree, check=True, capture_output=True)
    value = {
        "task_id": task_id,
        "outcome": "completed",
        "changed_files": ["result.txt"],
        "checks": ["fake-focused"],
    }
else:
    rejected = mode == "review_reject" or (mode == "review_reject_once" and call == 1)
    value = {
        "task_id": task_id,
        "decision": "rejected" if rejected else "accepted",
        "findings": ([{"code": "FIX_REQUIRED", "message": "repair fixture"}] if rejected else []),
    }

message = "{" if mode == "invalid" and role == "worker" else json.dumps(value)
if mode == "output_exhaust" and role == "worker":
    print("x" * 8192)
print(json.dumps({"type": "thread.started", "thread_id": f"{role}-{call}"}))
print(json.dumps({"type": "item.completed", "item": {"type": "agent_message", "text": message}}))
usage = 1000 if mode == "budget" else 3
completed = {"type": "turn.completed", "usage": {"input_tokens": usage, "output_tokens": usage}}
if mode == "missing_usage" and role == "worker":
    completed.pop("usage")
print(json.dumps(completed))
'''


class RepositoryFixture:
    def __init__(self, root: Path):
        self.root = root
        self.repo = root / "repo"
        self.codex = root / "fake-codex"
        self.control = root / "control.json"
        self.runner = root / "test-dispatcher.py"

    def create(self) -> None:
        self.repo.mkdir()
        self.run("git", "init", "-b", "codex/autopilot")
        self.run("git", "config", "user.email", "test@example.invalid")
        self.run("git", "config", "user.name", "Autopilot Test")
        (self.repo / "PLAN.md").write_text(PLAN)
        (self.repo / "AGENTS.md").write_text("# Test rules\n")
        (self.repo / ".gitignore").write_text(".mini-orca/autopilot/\nignored/\n")
        (self.repo / "mode.txt").write_text("accept\n")
        (self.repo / "Makefile").write_text("check:\n\t@./scripts/validate.sh\n")
        scripts = self.repo / "scripts"
        scripts.mkdir()
        validator = scripts / "validate.sh"
        validator.write_text(self.validator_source())
        validator.chmod(0o755)
        (scripts / "autopilot.py").write_text("# protected dispatcher\n")
        tests = scripts / "tests"
        tests.mkdir()
        (tests / "test_autopilot.py").write_text("# protected tests\n")
        tasks = self.repo / "tasks"
        tasks.mkdir()
        (tasks / "README.md").write_text("# Test workflow\n")
        self.codex.write_text(FAKE_CODEX)
        self.codex.chmod(0o755)
        self.control.write_text("{}")
        self.runner.write_text(self.runner_source())
        self.run("git", "add", ".")
        self.run("git", "commit", "-m", "fixture")

    def runner_source(self) -> str:
        return f'''#!/usr/bin/env python3
import json
import sys
sys.path.insert(0, {str(ROOT / "scripts")!r})
import autopilot

class TestDispatcher(autopilot.Dispatcher):
    def validation_command(self, worktree, temporary, command, environment):
        return [str(worktree / "scripts" / "validate.sh")]

try:
    result = TestDispatcher(autopilot.arguments(sys.argv[1:])).dispatch()
    print(json.dumps(result, sort_keys=True))
except autopilot.DispatchError as exc:
    print(f"autopilot: {{exc}}", file=sys.stderr)
    raise SystemExit(1)
'''

    def validator_source(self) -> str:
        return f'''#!/usr/bin/env python3
import json
import os
from pathlib import Path

control_path = Path({str(self.control)!r})
control = json.loads(control_path.read_text())
control["validator_calls"] = control.get("validator_calls", 0) + 1
control["credential_seen"] = "OPENAI_API_KEY" in os.environ
control_path.write_text(json.dumps(control))
mode = Path("mode.txt").read_text().strip()
if mode == "validator_mutation":
    Path("result.txt").write_text("validator mutation\\n")
failed = mode == "validation_fail" or (mode == "validation_fail_once" and Path("result.txt").read_text() == "worker 1\\n")
print(("FAIL" if failed else "PASS") + " fake-validation")
raise SystemExit(1 if failed else 0)
'''

    def run(self, *args: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(args, cwd=self.repo, text=True, capture_output=True, check=True)

    def set_mode(self, mode: str) -> None:
        if (self.repo / "mode.txt").read_text().strip() == mode:
            return
        (self.repo / "mode.txt").write_text(mode + "\n")
        self.run("git", "add", "mode.txt")
        self.run("git", "commit", "-m", f"mode {mode}")

    def dispatch(
        self,
        *extra: str,
        environment: dict[str, str] | None = None,
        timeout: float = 15,
    ) -> subprocess.CompletedProcess[str]:
        env = os.environ.copy()
        env.update(environment or {})
        return subprocess.run(
            [
                sys.executable,
                str(self.runner),
                "--repo",
                str(self.repo),
                "--codex",
                str(self.codex),
                "--timeout-seconds",
                "2",
                "--token-budget",
                "100",
                "--invocation-reserve-tokens",
                "20",
                "--lease-stale-seconds",
                "1",
                *extra,
            ],
            text=True,
            capture_output=True,
            env=env,
            timeout=timeout,
        )

    def state_path(self, task: str = "TST-01") -> Path:
        return self.repo / ".mini-orca" / "autopilot" / "runs" / f"{task}.json"

    def state(self, task: str = "TST-01") -> dict[str, object]:
        return json.loads(self.state_path(task).read_text())

    def calls(self) -> dict[str, object]:
        return json.loads(self.control.read_text())


class DispatcherTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.fixture = RepositoryFixture(Path(self.temporary.name))
        self.fixture.create()

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def fresh(self, mode: str) -> RepositoryFixture:
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        fixture = RepositoryFixture(Path(temporary.name))
        fixture.create()
        fixture.set_mode(mode)
        return fixture

    def assert_failed(self, result: subprocess.CompletedProcess[str], message: str) -> None:
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn(message, result.stderr)

    def test_dry_run_selects_without_writing_and_enforces_dependencies(self) -> None:
        selected = self.fixture.dispatch("--dry-run")
        self.assertEqual(selected.returncode, 0, selected.stderr)
        self.assertEqual(json.loads(selected.stdout), {"phase": "selected", "resume": False, "task": "TST-01"})
        self.assertFalse((self.fixture.repo / ".mini-orca").exists())
        self.assert_failed(self.fixture.dispatch("--dry-run", "--task", "TST-02"), "incomplete dependencies")
        self.assert_failed(self.fixture.dispatch("--dry-run", "--task", "BAD-99"), "unknown task")

    def test_dirty_or_wrong_branch_base_is_rejected(self) -> None:
        (self.fixture.repo / "dirty.txt").write_text("dirty\n")
        self.assert_failed(self.fixture.dispatch("--dry-run"), "uncommitted changes")
        self.fixture.run("git", "add", "dirty.txt")
        self.fixture.run("git", "commit", "-m", "clean")
        self.fixture.run("git", "switch", "-c", "wrong")
        self.assert_failed(self.fixture.dispatch("--dry-run"), "requires branch")

    def test_active_and_stale_leases_stop_duplicate_schedules(self) -> None:
        lease = self.fixture.repo / ".mini-orca" / "autopilot" / "lease"
        lease.mkdir(parents=True)
        owner = {"pid": os.getpid(), "child_pid": None, "acquired_at": now_iso()}
        (lease / "owner.json").write_text(json.dumps(owner))
        self.assert_failed(self.fixture.dispatch(), "active lease")
        owner.update({"pid": 99999999, "acquired_at": "2000-01-01T00:00:00+00:00"})
        (lease / "owner.json").write_text(json.dumps(owner))
        self.assert_failed(self.fixture.dispatch(), "--recover-stale-lease")
        recovered = self.fixture.dispatch("--recover-stale-lease")
        self.assertEqual(recovered.returncode, 0, recovered.stderr)
        self.assertEqual(json.loads(recovered.stdout)["phase"], "awaiting_integration")

    def test_crash_timeout_budget_and_output_fail_closed(self) -> None:
        cases = [
            ("crash", (), "worker process failed"),
            ("timeout", ("--timeout-seconds", "0.1"), "worker timed out"),
            ("budget", (), "token budget exhausted"),
            ("output_exhaust", ("--max-output-bytes", "1024"), "output limit exhausted"),
            ("missing_usage", (), "lacks token evidence"),
            ("invalid", (), "invalid worker structured output"),
        ]
        for mode, options, message in cases:
            with self.subTest(mode=mode):
                fixture = self.fresh(mode)
                self.assert_failed(fixture.dispatch(*options), message)
                self.assertEqual(fixture.state()["phase"], "failed")

    def test_worker_cannot_change_controls_or_history(self) -> None:
        for mode, message in [
            ("protected", "protected control files"),
            ("validator_control", "protected control files"),
            ("quality_control", "protected control files"),
            ("history", "worker changed Git history"),
        ]:
            with self.subTest(mode=mode):
                self.assert_failed(self.fresh(mode).dispatch(), message)

    def test_ignored_worker_output_is_removed_before_review(self) -> None:
        fixture = self.fresh("ignored_poison")
        result = fixture.dispatch()
        self.assertEqual(result.returncode, 0, result.stderr)
        worktree = fixture.repo / fixture.state()["worktree"]
        self.assertFalse((worktree / "ignored").exists())

    def test_review_and_validation_each_allow_only_two_repairs(self) -> None:
        for mode, message in [
            ("review_reject", "review rejected after two repairs"),
            ("validation_fail", "validation failed after two repairs"),
        ]:
            with self.subTest(mode=mode):
                fixture = self.fresh(mode)
                self.assert_failed(fixture.dispatch(), message)
                calls = fixture.calls()
                self.assertEqual(calls["worker_calls"], 3)
                self.assertEqual(calls["reviewer_calls"], 3)

    def test_repair_receives_fresh_review_and_fixed_validation(self) -> None:
        fixture = self.fresh("validation_fail_once")
        result = fixture.dispatch(environment={"OPENAI_API_KEY": "must-not-reach-validation"})
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = fixture.calls()
        self.assertEqual((calls["worker_calls"], calls["reviewer_calls"], calls["validator_calls"]), (2, 2, 2))
        self.assertFalse(calls["credential_seen"])
        self.assertTrue(all("-o" not in argv and "--output-last-message" not in argv for argv in calls["argv"]))
        state = fixture.state()
        self.assertEqual(state["reviewer"]["diff"], state["validator"]["diff"])
        self.assertEqual(state["validator"]["command"], "make check")
        self.assertEqual(state["validator"]["stages"], [{"name": "fake-validation", "outcome": "pass"}])

    def test_validator_mutation_never_integrates(self) -> None:
        fixture = self.fresh("validator_mutation")
        base = fixture.run("git", "rev-parse", "HEAD").stdout.strip()
        self.assert_failed(fixture.dispatch("--integrate"), "validator changed")
        self.assertEqual(fixture.run("git", "rev-parse", "HEAD").stdout.strip(), base)

    def test_interrupted_paid_invocation_is_never_repeated(self) -> None:
        first = self.fixture.dispatch()
        self.assertEqual(first.returncode, 0, first.stderr)
        before = self.fixture.calls()["worker_calls"]
        state = self.fixture.state()
        state["phase"] = "ready"
        state["pending_invocation"] = {"id": "worker-2", "role": "worker", "reserved_tokens": 20}
        self.fixture.state_path().write_text(json.dumps(state))
        resumed = self.fixture.dispatch()
        self.assert_failed(resumed, "cannot be retried automatically")
        self.assertEqual(self.fixture.calls()["worker_calls"], before)

    def test_resume_rejects_changed_base_configuration_and_git_redirects(self) -> None:
        safe = self.fresh("accept")
        result = safe.dispatch(
            environment={
                "GIT_DIR": str(safe.root / "redirect"),
                "GIT_WORK_TREE": str(safe.root),
                "GIT_INDEX_FILE": str(safe.root / "index"),
                "GIT_CONFIG_GLOBAL": str(safe.root / "global-config"),
            }
        )
        self.assertEqual(result.returncode, 0, result.stderr)

        changed = self.fresh("accept")
        self.assertEqual(changed.dispatch().returncode, 0)
        (changed.repo / "other.txt").write_text("change\n")
        changed.run("git", "add", "other.txt")
        changed.run("git", "commit", "-m", "changed base")
        self.assert_failed(changed.dispatch(), "target branch changed")

        configured = self.fresh("accept")
        self.assertEqual(configured.dispatch().returncode, 0)
        configured.run("git", "config", "test.changed", "true")
        self.assert_failed(configured.dispatch(), "configuration changed")

    def test_authorized_fake_pilot_commits_locally_and_unblocks_next_task(self) -> None:
        base = self.fixture.run("git", "rev-parse", "HEAD").stdout.strip()
        pending = self.fixture.dispatch()
        self.assertEqual(json.loads(pending.stdout)["phase"], "awaiting_integration")
        self.assertEqual(self.fixture.run("git", "rev-parse", "HEAD").stdout.strip(), base)
        integrated = self.fixture.dispatch("--integrate")
        self.assertEqual(integrated.returncode, 0, integrated.stderr)
        self.assertEqual(json.loads(integrated.stdout)["phase"], "integrated")
        self.assertNotEqual(self.fixture.run("git", "rev-parse", "HEAD").stdout.strip(), base)
        self.assertIn("| TST-01 | Ready task | TST-00 | S / low | Complete |", (self.fixture.repo / "PLAN.md").read_text())
        next_task = self.fixture.dispatch("--dry-run")
        self.assertEqual(json.loads(next_task.stdout)["task"], "TST-02")
        self.assertFalse(self.fixture.run("git", "remote").stdout.strip())

        calls = self.fixture.calls()
        interrupted = self.fixture.state()
        interrupted["phase"] = "integrating"
        self.fixture.state_path().write_text(json.dumps(interrupted))
        resumed = self.fixture.dispatch("--integrate")
        self.assertEqual(resumed.returncode, 0, resumed.stderr)
        self.assertEqual(json.loads(resumed.stdout)["phase"], "integrated")
        self.assertEqual(self.fixture.calls(), calls)

    def test_cli_exposes_no_validator_or_sandbox_bypass(self) -> None:
        help_result = subprocess.run([sys.executable, str(AUTOPILOT), "--help"], text=True, capture_output=True, check=True)
        self.assertNotIn("--validator", help_result.stdout)
        self.assertNotIn("--sandbox-exec", help_result.stdout)
        outside = self.fixture.root / "outside-state"
        (self.fixture.repo / ".mini-orca").symlink_to(outside, target_is_directory=True)
        self.assert_failed(self.fixture.dispatch("--dry-run"), "must not contain symlinks")

    @unittest.skipUnless(sys.platform == "darwin", "production validator uses macOS sandbox-exec")
    def test_production_validation_sandbox_denies_external_writes(self) -> None:
        if os.environ.get("MINI_ORCA_AUTOPILOT_VALIDATION") == "1":
            self.skipTest("nested sandbox is unavailable")
        dispatcher = autopilot.Dispatcher(
            autopilot.arguments(["--repo", str(self.fixture.repo), "--codex", str(self.fixture.codex)])
        )
        outside = self.fixture.root / "outside-write"
        probe = self.fixture.repo / "sandbox-probe.sh"
        probe.write_text(
            "#!/bin/sh\n"
            f"if printf bad > {str(outside)!r} 2>/dev/null; then exit 9; fi\n"
            "printf ok > sandbox-inside\n"
        )
        probe.chmod(0o755)
        with tempfile.TemporaryDirectory(dir=self.fixture.repo) as directory:
            temporary = Path(directory)
            environment = autopilot.clean_environment(auth=False, temporary_home=temporary)
            result = subprocess.run(
                dispatcher.validation_command(dispatcher.repo, temporary, [str(probe.resolve())], environment),
                cwd=dispatcher.repo,
                env=environment,
                capture_output=True,
                check=False,
            )
        self.assertEqual(result.returncode, 0, result.stderr.decode(errors="replace"))
        self.assertFalse(outside.exists())
        self.assertEqual((self.fixture.repo / "sandbox-inside").read_text(), "ok")


def now_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


if __name__ == "__main__":
    unittest.main()
