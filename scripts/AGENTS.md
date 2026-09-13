# Tooling and validation

Read [../AGENTS.md](../AGENTS.md) first. This is also the tooling guide for root
Makefile and `.github/workflows/` changes.

- Preserve ownership: `validate.sh` aggregates full validation, `quality.sh` owns
  pinned quality stages, `desktop-gradle.sh` resolves the launcher/toolchain,
  and Makefile/CI invoke those maintained paths. Fix a stage in its owner;
  do not introduce another competing validation script.
- Keep shell scripts compatible with their declared interpreter and Python with
  the documented runtime. Prefer the standard library. Quote paths and arguments;
  use subprocess argv instead of interpolating executable shell strings.
- Preserve meaningful exit codes, bounded output/timeouts and aggregate failure
  reporting. A tool that exits zero but reports findings may still fail the gate.
  Never swallow diagnostics or skip a failing stage to manufacture success.
- Keep quality tools pinned and separate from application dependencies. Do not
  relax thresholds, exclusions, checksums or static-analysis configuration to
  accommodate a change. Update build docs/tests when a supported command changes.
- Toolchain helpers must validate requested Java homes and explain mismatches.
  Do not silently use the wrong runtime, replace system installations or write
  machine-specific paths into versioned files.

## Dispatcher and runtime tools

`autopilot.py` and `insight_runtime.py` are operational tools, not automatic steps
for an ordinary edit. Read the relevant execution/history guide before changing
their behavior. Historical model selections, grants and unfinished cards do not
authorize launching agents, resuming schedulers or running live evaluations.

- Preserve dispatcher task/path boundaries, protected files, writer ownership,
  bounded repair attempts, review/validation gates and recovery receipts.
- Preserve runtime process identity, lifecycle locking, loopback policy, audit
  checks and offline/read-only preflight behavior. Signal only an owned process;
  retain evidence when cleanup cannot be established.
- Keep execution artifacts in their ignored locations. Do not rewrite historical
  receipts, campaign scores, sealed evaluation material or worktree copies.

## Verification

Run `python3 -m unittest discover -s scripts/tests -p 'test_*.py'` from the root.
Use `sh -n` for changed POSIX shell scripts and `bash -n` for changed Bash scripts.
Exercise changed validation behavior with fake executables/temporary directories,
including nonzero exits, findings with zero exit status and retained diagnostics.
For lifecycle changes, cover interruption, ownership mismatch and cleanup failure.

Do not invoke the real dispatcher, start a model server or download model artifacts
to test tooling. Ordinary build dependency resolution and fake-provider tests are
part of implementation. Changes to the validation pipeline also need its affected
real gates when available, with blocked stages reported explicitly.
