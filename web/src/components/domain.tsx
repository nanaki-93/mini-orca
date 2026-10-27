// Domain-specific display pieces shared by several pages.

import { Bug, FileCode2, FileJson, FileText, FlaskConical, Gauge, Lightbulb, Shield, Sparkles, type LucideIcon } from 'lucide-react';
import type { ChangeHistoryEntry, Finding } from '@/api/types';
import { taskStatus, type TaskStatus } from '@/lib/readiness';
import { isTestPath, severityTone } from '@/lib/util';
import { Badge, cx } from './ui';

export const KINDS: { value: string; label: string; icon: LucideIcon }[] = [
  { value: 'fix', label: 'Fix', icon: Bug },
  { value: 'feature', label: 'Feature', icon: Lightbulb },
  { value: 'performance', label: 'Performance', icon: Gauge },
  { value: 'security', label: 'Security', icon: Shield },
];

export function kindInfo(kind: string) {
  return KINDS.find(k => k.value === kind) ?? { value: kind, label: kind === 'instructions' ? 'Instructions' : kind, icon: Sparkles };
}

export function KindTag({ kind }: { kind: string }) {
  const info = kindInfo(kind);
  return (
    <span className="kind-tag">
      <info.icon size={12} aria-hidden />
      {info.label}
    </span>
  );
}

const STATUS_GLYPH: Record<TaskStatus, string> = {
  draft: 'draft', running: 'running', needs_you: 'review', ready: 'ready', applied: 'done', failed: 'failed', stale: 'stale', canceled: 'canceled', done: 'done',
};

/** A Linear-style status glyph: circle states drawn in CSS. */
export function StatusGlyph({ status, size = 14 }: { status: TaskStatus; size?: number }) {
  return <span className={cx('glyph', `glyph-${STATUS_GLYPH[status]}`)} style={{ width: size, height: size }} aria-hidden />;
}

export function TaskStatusBadge({ entry }: { entry: Pick<ChangeHistoryEntry, 'state' | 'freshness' | 'workflow_status' | 'commit' | 'revision' | 'hash'> }) {
  const info = taskStatus(entry);
  return (
    <span className={cx('status-pill', `status-${info.tone}`)}>
      <StatusGlyph status={info.status} size={12} />
      {info.label}
    </span>
  );
}

export function SeverityBadge({ severity }: { severity: string }) {
  const tone = severityTone(severity);
  return <Badge tone={tone}>{severity || 'info'}</Badge>;
}

export function sourceLabel(finding: Finding): string {
  const source = (finding.source || '').toLowerCase();
  if (source.includes('semantic') || source.includes('ai') || source.includes('model')) return 'AI suggestion';
  if (source.includes('scan') || source.includes('vet') || source.includes('staticcheck')) return 'Verified by scan';
  if (source.includes('security')) return 'Security review';
  return finding.source || 'Finding';
}

export function isOpenFinding(finding: Finding): boolean {
  return !['dismissed', 'fixed', 'retired'].includes(finding.status || 'open');
}

export function FileIcon({ path, size = 14 }: { path: string; size?: number }) {
  const ext = path.slice(path.lastIndexOf('.') + 1).toLowerCase();
  const Icon = isTestPath(path) ? FlaskConical : ext === 'json' || ext === 'yaml' || ext === 'yml' || ext === 'toml' ? FileJson : ext === 'md' || ext === 'txt' ? FileText : FileCode2;
  return <Icon size={size} className={cx('file-icon', `file-${ext}`, isTestPath(path) && 'file-test')} aria-hidden />;
}
