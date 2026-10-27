// Review authority rules mirrored from the daemon. The daemon remains the
// authority: these helpers only decide which controls are offered, so a stale
// or incomplete proposal never presents Apply as available.

import type { ChangeHistoryEntry, ChangeMutationResult, ChangeSession, Check, FileChange, LoadedSession, UndoAvailability } from '@/api/types';
import type { Tone } from './util';

export type PhaseTone = 'passed' | 'failed' | 'running' | 'idle';

/** Normalizes check, stage and verdict states into four display tones. */
export function phaseTone(raw: string | undefined): PhaseTone {
  const s = (raw ?? '').toLowerCase().trim();
  if (['passed', 'completed', 'completed_empty', 'fresh', 'approve', 'approved', 'checked', 'reviewed', 'applied', 'committed', 'merged', 'verified'].includes(s)) return 'passed';
  if (['failed', 'error', 'rejected', 'changes_requested', 'stale', 'outdated', 'interrupted', 'conflicts', 'blocked'].includes(s)) return 'failed';
  if (['running', 'in_progress', 'working', 'queued', 'canceling', 'pausing', 'pending_review', 'waiting'].includes(s)) return 'running';
  return 'idle';
}

export function isFileReviewed(session: ChangeSession, path: string): boolean {
  return Boolean(session.hash) && session.file_review_hash === session.hash && (session.reviewed_files ?? []).includes(path);
}

/** Edits shown for review: the proposal plus pinned tests, or the delta since the last review. */
export function effectiveEdits(session: ChangeSession | null | undefined, sinceLastReview = false): FileChange[] {
  if (!session) return [];
  if (sinceLastReview) return session.review_diff ?? [];
  const seen = new Set<string>();
  return [...(session.changes ?? []), ...(session.pinned_tests ?? [])].filter(edit => {
    if (!edit?.path || seen.has(edit.path)) return false;
    seen.add(edit.path);
    return true;
  });
}

export function reviewProgress(session: ChangeSession) {
  const edits = session.changes ?? [];
  const reviewed = edits.filter(edit => isFileReviewed(session, edit.path)).length;
  const total = edits.length;
  return { reviewed, total, percent: total ? Math.round((reviewed / total) * 100) : 0 };
}

export interface CheckSummary {
  total: number;
  passed: number;
  failed: number;
  running: number;
  idle: number;
  label: string;
  tone: Tone;
}

/** Summarizes checks; only a failed required check blocks Apply, so optional failures read as a warning. */
export function checkSummary(checks: Check[] | undefined): CheckSummary {
  const list = checks ?? [];
  const counts = { total: list.length, passed: 0, failed: 0, running: 0, idle: 0 };
  for (const check of list) counts[phaseTone(check.state)]++;
  const requiredFailed = list.filter(check => check.required && phaseTone(check.state) === 'failed').length;
  const optionalFailed = counts.failed - requiredFailed;
  let label: string;
  let tone: Tone;
  if (!list.length) [label, tone] = ['Not run', 'neutral'];
  else if (requiredFailed) [label, tone] = [`${requiredFailed} required failed`, 'danger'];
  else if (counts.running) [label, tone] = ['Running', 'info'];
  else if (optionalFailed) [label, tone] = [`${optionalFailed} optional failed`, 'warning'];
  else if (counts.passed === list.length) [label, tone] = [`${counts.passed}/${list.length} passed`, 'success'];
  else [label, tone] = [`${counts.passed}/${list.length} passed`, 'warning'];
  return { ...counts, label, tone };
}

export function workflowActive(session: { workflow?: { status: string } } | null | undefined): boolean {
  const status = session?.workflow?.status;
  return status === 'running' || status === 'canceling';
}

export type ReviewStep = 'none' | 'generate' | 'agents' | 'review' | 'checks' | 'approve' | 'apply' | 'applied' | 'stale' | 'recovery';

export interface Readiness {
  editable: boolean;
  filesReviewed: boolean;
  checksPassed: boolean;
  canApprove: boolean;
  canApply: boolean;
  canUndo: boolean;
  step: ReviewStep;
  message: string;
}

/**
 * Apply needs every changed file reviewed at the current hash, all required
 * checks passed, a workflow (if any) waiting for human review, and an explicit
 * approval recorded for this exact hash.
 */
export function proposalReadiness(session: (ChangeSession & { undo?: UndoAvailability }) | null | undefined): Readiness {
  const result: Readiness = { editable: false, filesReviewed: false, checksPassed: false, canApprove: false, canApply: false, canUndo: false, step: 'none', message: 'Select a task.' };
  if (!session) return result;
  if (session.state === 'applied') {
    const undo = session.undo ?? { available: true, reason: '' };
    return { ...result, canUndo: undo.available, step: 'applied', message: undo.available ? 'Applied. Undo verifies the files are unchanged before restoring them.' : `Applied. ${undo.reason || 'Undo is not available for this task.'}` };
  }
  if (session.freshness !== 'current') return { ...result, step: 'stale', message: 'The project changed since this proposal was made. Start it again to continue.' };
  if (['undone', 'reverted', 'recovery_required'].includes(session.state)) return { ...result, step: 'recovery', message: `This task is ${session.state.replace('_', ' ')}.` };
  result.editable = true;
  const edits = session.changes ?? [];
  result.filesReviewed = edits.length > 0 && edits.every(edit => isFileReviewed(session, edit.path));
  const checks = session.checks ?? [];
  result.checksPassed = checks.length > 0 && checks.every(check => !check.required || check.state === 'passed');
  const workflowReady = !session.workflow || session.workflow.status === 'awaiting_human_review';
  result.canApprove = result.filesReviewed && result.checksPassed && workflowReady;
  result.canApply = result.canApprove && Boolean(session.hash) && session.reviewed_hash === session.hash;
  if (!edits.length) {
    result.step = workflowActive(session) ? 'agents' : 'generate';
    result.message = workflowActive(session) ? 'Agents are writing the change.' : 'No changes yet. Start the agents to generate them.';
  } else if (workflowActive(session)) {
    result.step = 'agents';
    result.message = 'Agents are still working on this version.';
  } else if (!result.filesReviewed) {
    result.step = 'review';
    result.message = 'Review each changed file and mark it viewed.';
  } else if (!result.checksPassed) {
    result.step = 'checks';
    result.message = 'Run checks; required checks must pass before approval.';
  } else if (!workflowReady) {
    result.step = 'agents';
    result.message = session.workflow?.reason || 'The agent workflow has not reached human review.';
  } else if (!result.canApply) {
    result.step = 'approve';
    result.message = 'Approve this exact version to enable Apply.';
  } else {
    result.step = 'apply';
    result.message = 'Approved. Apply writes these files to the checkout.';
  }
  return result;
}

/** The next unreviewed edit after `current`, wrapping around, or ''. */
export function nextUnreviewedPath(session: ChangeSession, edits: FileChange[], current: string): string {
  if (!edits.length) return '';
  const start = edits.findIndex(edit => edit.path === current);
  for (let offset = 1; offset <= edits.length; offset++) {
    const candidate = edits[(start + offset + edits.length) % edits.length];
    if (candidate && !isFileReviewed(session, candidate.path)) return candidate.path;
  }
  return '';
}

/** The guarded proposal identity every mutation request carries. */
export function changeIdentity(session: LoadedSession) {
  return {
    project_id: session.checkout.project_id || session.project_id,
    project_revision: session.checkout.project_revision || session.project_revision,
    revision: session.revision,
    hash: session.hash,
  };
}

/** Undo restores only the latest grouped Apply in a checkout. */
export function undoFromRecovery(session: Pick<ChangeSession, 'id'>, recovery: ChangeMutationResult | null | undefined): UndoAvailability {
  if (!recovery?.session_id) return { available: false, reason: 'No undo record exists for this checkout. Revert the change with Git.' };
  if (recovery.session_id !== session.id) return { available: false, reason: 'Undo covers only the latest Apply in this checkout. Revert this change with Git.' };
  if (!recovery.undo_available || recovery.state === 'undone') return { available: false, reason: 'This Apply was already undone.' };
  return { available: true, reason: '' };
}

export type TaskStatus = 'draft' | 'running' | 'needs_you' | 'ready' | 'applied' | 'failed' | 'stale' | 'canceled' | 'done';

export interface TaskStatusInfo {
  status: TaskStatus;
  label: string;
  tone: Tone;
}

/** A task's status for lists, from its history entry alone. */
export function taskStatus(entry: Pick<ChangeHistoryEntry, 'state' | 'freshness' | 'workflow_status' | 'commit' | 'revision' | 'hash'>): TaskStatusInfo {
  const wf = entry.workflow_status ?? '';
  if (entry.state === 'applied') return { status: 'applied', label: entry.commit?.state === 'committed' ? 'Committed' : 'Applied', tone: 'success' };
  if (entry.state === 'undone') return { status: 'done', label: 'Undone', tone: 'neutral' };
  if (wf === 'running' || wf === 'canceling' || wf === 'queued') return { status: 'running', label: wf === 'canceling' ? 'Canceling' : 'Agents working', tone: 'info' };
  if (entry.freshness && entry.freshness !== 'current') return { status: 'stale', label: 'Stale', tone: 'warning' };
  if (wf === 'awaiting_human_review') return { status: 'needs_you', label: 'Needs review', tone: 'accent' };
  if (wf === 'changes_requested') return { status: 'needs_you', label: 'Changes requested', tone: 'warning' };
  if (wf === 'failed' || wf === 'stale' || wf === 'outdated' || wf === 'interrupted') return { status: 'failed', label: wf === 'interrupted' ? 'Interrupted' : 'Failed', tone: 'danger' };
  if (wf === 'canceled') return { status: 'canceled', label: 'Canceled', tone: 'neutral' };
  if (entry.revision > 0 && entry.hash) return { status: 'needs_you', label: 'Needs review', tone: 'accent' };
  return { status: 'draft', label: 'Draft', tone: 'neutral' };
}
