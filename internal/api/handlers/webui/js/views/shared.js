// Rendering fragments shared by several views.

import { severityTone } from '../core/util.js';
import { badge } from '../ui/dom.js';

const STATE_TONES = { applied: 'success', reviewed: 'success', checked: 'info', draft: '', failed: 'danger', undone: 'warning', reverted: 'warning', recovery_required: 'danger' };
const WORKFLOW_TONES = { running: 'info', canceling: 'warning', failed: 'danger', canceled: '', stale: 'warning', interrupted: 'warning', outdated: 'warning', changes_requested: 'warning', awaiting_human_review: 'accent' };
const WORKFLOW_LABELS = { awaiting_human_review: 'awaiting review', changes_requested: 'changes requested' };

export function taskStateBadge(task) {
  const workflow = (task.workflow && task.workflow.status) || task.workflow_status || '';
  if (workflow && task.state !== 'applied') return badge(WORKFLOW_LABELS[workflow] || workflow, WORKFLOW_TONES[workflow] || '', workflow === 'running');
  if (task.freshness && task.freshness !== 'current' && task.state !== 'applied') return badge(task.freshness, 'warning');
  if (task.commit && task.commit.state === 'committed') return badge('committed', 'success');
  return badge(task.state || 'draft', STATE_TONES[task.state] || '');
}

export function findingPath(finding) {
  return (finding.location && finding.location.path) || (finding.task_spec && finding.task_spec.target_path) || '';
}

export function findingLine(finding) {
  return (finding.location && finding.location.start_line) || 0;
}

export function findingIsOpen(finding) {
  return !['dismissed', 'fixed', 'retired'].includes(String(finding.status || 'open'));
}

export function severityBadge(severity) {
  return badge(severity || 'info', severityTone(severity) === 'neutral' ? '' : severityTone(severity));
}

export function sourceLabel(finding) {
  if (finding.confidence === 'suggested') return 'AI suggestion';
  return finding.source ? 'Verified · ' + finding.source : 'Verified';
}
