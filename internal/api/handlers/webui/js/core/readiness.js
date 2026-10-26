// Review authority rules mirrored from the daemon. The daemon remains the
// authority: these helpers only decide which controls are offered, so a stale
// or incomplete proposal never presents Apply as available.

import { asArray } from './util.js';

export const REVIEW_SECTIONS = ['changes', 'checks', 'pipeline', 'spec', 'apply'];

/** Normalizes the many check, stage and scan states into four display tones. */
export function normalizePhaseState(raw) {
  const s = String(raw || '').toLowerCase().trim();
  if (['passed', 'completed', 'completed_empty', 'fresh', 'approve', 'approved', 'checked', 'reviewed', 'applied', 'committed', 'merged'].includes(s)) return 'passed';
  if (['failed', 'error', 'rejected', 'changes_requested', 'stale', 'outdated', 'interrupted', 'conflicts'].includes(s)) return 'failed';
  if (['running', 'in_progress', 'working', 'queued', 'canceling', 'pausing', 'pending_review', 'waiting'].includes(s)) return 'running';
  return 'idle';
}

export function isFileReviewed(session, path) {
  if (!session || !session.hash || session.file_review_hash !== session.hash) return false;
  return asArray(session.reviewed_files).includes(path);
}

/** Edits shown in the diff view: the full proposal plus pinned tests, or the delta since review. */
export function effectiveEdits(session, sinceLastReview = false) {
  if (!session) return [];
  if (sinceLastReview) return asArray(session.review_diff);
  const seen = new Set();
  return asArray(session.changes).concat(asArray(session.pinned_tests)).filter(edit => {
    if (!edit || !edit.path || seen.has(edit.path)) return false;
    seen.add(edit.path);
    return true;
  });
}

export function reviewProgress(session) {
  const edits = asArray(session && session.changes);
  const reviewed = edits.filter(edit => isFileReviewed(session, edit.path)).length;
  const total = edits.length || asArray(session && session.targets).length;
  return { reviewed, total, percent: total ? Math.round((reviewed / total) * 100) : 0 };
}

export function checkSummary(session) {
  const checks = asArray(session && session.checks);
  const counts = { total: checks.length, passed: 0, failed: 0, running: 0, idle: 0, requiredFailed: 0 };
  for (const check of checks) {
    const tone = normalizePhaseState(check.state);
    counts[tone]++;
    if (check.required && check.state !== 'passed' && tone !== 'idle') counts.requiredFailed++;
  }
  counts.label = !checks.length ? 'Not run' : counts.failed ? counts.failed + ' failed' : counts.running ? 'Running' : counts.passed + '/' + checks.length + ' passed';
  counts.tone = !checks.length ? 'neutral' : counts.failed ? 'danger' : counts.running ? 'info' : counts.passed === checks.length ? 'success' : 'warning';
  return counts;
}

export function workflowActive(session) {
  const status = session && session.workflow && session.workflow.status;
  return status === 'running' || status === 'canceling';
}

/**
 * proposalReadiness reports which review controls the current proposal allows.
 * Apply needs every changed file reviewed at the current hash, all required
 * checks passed, a workflow (if any) waiting for human review, and an explicit
 * approval recorded for this exact hash.
 */
export function proposalReadiness(session) {
  const result = { editable: false, filesReviewed: false, checksPassed: false, canApprove: false, canApply: false, canUndo: false, step: 'none', message: 'Select a proposal to review.' };
  if (!session) return result;
  if (session.state === 'applied') {
    result.canUndo = true;
    result.step = 'applied';
    result.message = 'Applied. Undo verifies the current source before restoring it.';
    return result;
  }
  if (session.freshness !== 'current') {
    result.step = 'stale';
    result.message = 'This proposal is stale or unavailable. Refresh the workspace before continuing.';
    return result;
  }
  if (['undone', 'reverted', 'recovery_required'].includes(session.state)) {
    result.step = 'recovery';
    result.message = 'Proposal is ' + session.state + '. Inspect its recovery state in the TUI.';
    return result;
  }
  result.editable = true;
  const edits = asArray(session.changes);
  result.filesReviewed = edits.length > 0 && edits.every(edit => isFileReviewed(session, edit.path));
  const checks = asArray(session.checks);
  result.checksPassed = checks.length > 0 && checks.every(check => !check.required || check.state === 'passed');
  const workflowReady = !session.workflow || session.workflow.status === 'awaiting_human_review';
  result.canApprove = result.filesReviewed && result.checksPassed && workflowReady;
  result.canApply = result.canApprove && Boolean(session.hash) && session.reviewed_hash === session.hash;
  if (!edits.length) {
    result.step = workflowActive(session) ? 'pipeline' : 'spec';
    result.message = workflowActive(session) ? 'The agent workflow is generating changes.' : 'Generate changes to begin.';
  } else if (!result.filesReviewed) {
    result.step = 'changes';
    result.message = 'Review each changed file, then mark it reviewed.';
  } else if (!result.checksPassed) {
    result.step = 'checks';
    result.message = 'Run checks and resolve required failures before approval.';
  } else if (!workflowReady) {
    result.step = 'pipeline';
    result.message = 'The workflow is not waiting for human review yet.';
  } else if (!result.canApply) {
    result.step = 'apply';
    result.message = 'Approve your review to enable Apply.';
  } else {
    result.step = 'apply';
    result.message = 'Review approved. Apply writes these changes to the selected checkout.';
  }
  return result;
}

/** Returns the path of the next unreviewed edit after `current`, wrapping around, or ''. */
export function nextUnreviewedPath(session, edits, current) {
  if (!session || !edits.length) return '';
  const start = edits.findIndex(edit => edit.path === current);
  for (let offset = 1; offset <= edits.length; offset++) {
    const candidate = edits[(start + offset + edits.length) % edits.length];
    if (candidate && !isFileReviewed(session, candidate.path)) return candidate.path;
  }
  return '';
}

/** Builds the guarded proposal identity every mutation request carries. */
export function changeIdentity(session, project) {
  return {
    project_id: session._checkoutProjectId || session.project_id || (project && project.project_id),
    project_revision: session._checkoutProjectRevision || session.project_revision || (project && project.project_revision),
    revision: session.revision,
    hash: session.hash
  };
}

/** Picks the single most useful next step for the dashboard. */
export function nextAction(state) {
  const session = state.activeSession;
  if (state.gitWorkspace && asArray(state.gitWorkspace.conflicts).length) {
    return { tone: 'danger', title: 'Merge has conflicts', detail: asArray(state.gitWorkspace.conflicts).length + ' conflicted path(s) block further Git actions.', action: 'open-page', page: 'git', label: 'Open Git' };
  }
  if (session) {
    const ready = proposalReadiness(session);
    const progress = reviewProgress(session);
    const title = session.title || session.id;
    if (workflowActive(session)) return { tone: 'info', title: 'Agents are working on “' + title + '”', detail: 'Follow each stage live in the pipeline.', action: 'open-review', section: 'pipeline', label: 'View pipeline' };
    if (ready.step === 'changes') return { tone: 'warning', title: 'Review ' + (progress.total - progress.reviewed) + ' file(s) in “' + title + '”', detail: progress.reviewed + ' of ' + progress.total + ' reviewed.', action: 'open-review', section: 'changes', label: 'Continue review' };
    if (ready.step === 'checks') return { tone: 'warning', title: 'Verify “' + title + '”', detail: checkSummary(session).label + '. Required checks must pass before approval.', action: 'open-review', section: 'checks', label: 'Open checks' };
    if (ready.canApply) return { tone: 'success', title: '“' + title + '” is ready to apply', detail: 'Review approved at the current hash.', action: 'open-review', section: 'apply', label: 'Apply changes' };
    if (ready.canApprove) return { tone: 'success', title: 'Approve “' + title + '”', detail: 'Files reviewed and checks passed.', action: 'open-review', section: 'apply', label: 'Approve review' };
    if (ready.step === 'spec') return { tone: 'info', title: 'Generate changes for “' + title + '”', detail: 'The task is captured but has no proposal yet.', action: 'open-review', section: 'spec', label: 'Generate' };
  }
  const open = asArray(state.findings).filter(f => !['dismissed', 'fixed', 'retired'].includes(String(f.status || 'open'))).length;
  if (open) return { tone: 'neutral', title: open + ' open issue(s) to triage', detail: 'Turn a finding into a focused fix task.', action: 'open-page', page: 'issues', label: 'Triage issues' };
  return { tone: 'neutral', title: 'Start a focused change', detail: 'Describe an outcome, pick up to eight files and review the generated diff.', action: 'new-task', label: 'New task' };
}
