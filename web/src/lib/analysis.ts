// Analysis run read models for the dashboard: overall progress, why stages
// failed, and each file's stages, from the run the daemon reports.

import type { AnalysisRunFile, AnalysisRunSummary, AnalysisStageProgress } from '@/api/types';
import type { Tone } from './util';

export const STAGES: { stage: string; label: string; short: string }[] = [
  { stage: 'semantic', label: 'Bugs', short: 'Bugs' },
  { stage: 'performance', label: 'Performance', short: 'Perf' },
  { stage: 'security_rules', label: 'Security rules', short: 'Rules' },
  { stage: 'security_ai', label: 'Security review', short: 'Sec AI' },
];

export const SECTION_LABELS: Record<string, string> = { bugs: 'Bugs', performance: 'Performance', security: 'Security' };

const ACTIVE = new Set(['queued', 'running', 'pausing', 'canceling']);
const DONE = new Set(['completed', 'completed_empty', 'partial', 'failed', 'skipped', 'unavailable', 'canceled', 'stale']);

/** Whether a run is still dispatching work. */
export function runActive(run: AnalysisRunSummary | null | undefined): boolean {
  return Boolean(run && ACTIVE.has(run.status));
}

/** Whether a stopped run can be resumed with a fresh preview. */
export function runResumable(run: AnalysisRunSummary | null | undefined): boolean {
  return Boolean(run?.identity && ['paused', 'interrupted'].includes(run.status));
}

export function statusTone(status: string): Tone {
  switch (status) {
    case 'completed':
    case 'completed_empty':
      return 'success';
    case 'running':
    case 'queued':
    case 'pausing':
    case 'canceling':
      return 'info';
    case 'failed':
      return 'danger';
    case 'partial':
    case 'paused':
    case 'interrupted':
    case 'stale':
      return 'warning';
    default:
      return 'neutral';
  }
}

export function statusLabel(status: string): string {
  return { completed_empty: 'no findings', completed: 'done' }[status] ?? status.replace(/_/g, ' ');
}

export interface RunProgress {
  total: number;
  done: number;
  running: number;
  failed: number;
  succeeded: number;
  percent: number;
}

/** Stage-level progress over every file; skipped and unavailable stages count as done. */
export function runProgress(run: AnalysisRunSummary | null | undefined): RunProgress {
  const stages = (run?.files ?? []).flatMap(file => file.stages);
  const done = stages.filter(stage => DONE.has(stage.status)).length;
  const progress = {
    total: stages.length,
    done,
    running: stages.filter(stage => stage.status === 'running').length,
    failed: stages.filter(stage => stage.status === 'failed').length,
    succeeded: stages.filter(stage => stage.status === 'completed' || stage.status === 'completed_empty' || stage.status === 'partial').length,
  };
  return { ...progress, percent: progress.total ? Math.round((done / progress.total) * 100) : 0 };
}

export interface FailureGroup {
  reason: string;
  count: number;
  stages: string[];
  files: string[];
}

/** Failed or interrupted stages grouped by their reason, most frequent first. */
export function failureGroups(run: AnalysisRunSummary | null | undefined): FailureGroup[] {
  const groups = new Map<string, FailureGroup>();
  for (const file of run?.files ?? []) {
    for (const stage of file.stages) {
      if (!['failed', 'interrupted', 'unavailable'].includes(stage.status)) continue;
      const reason = stage.reason || 'No reason was recorded.';
      const group = groups.get(reason) ?? { reason, count: 0, stages: [], files: [] };
      group.count++;
      if (!group.stages.includes(stage.stage)) group.stages.push(stage.stage);
      if (!group.files.includes(file.path)) group.files.push(file.path);
      groups.set(reason, group);
    }
  }
  return [...groups.values()].sort((a, b) => b.count - a.count || a.reason.localeCompare(b.reason));
}

export type FileFilter = 'all' | 'failed' | 'findings' | 'active';

/** The state a file is in for filtering: failed beats running beats done. */
export function fileState(file: AnalysisRunFile): 'failed' | 'active' | 'done' {
  if (file.stages.some(stage => stage.status === 'failed')) return 'failed';
  if (file.stages.some(stage => !DONE.has(stage.status))) return 'active';
  return 'done';
}

export function fileFindings(file: AnalysisRunFile): number {
  return file.stages.reduce((sum, stage) => sum + (stage.finding_count ?? 0), 0);
}

export function filterFiles(files: AnalysisRunFile[], filter: FileFilter, query: string): AnalysisRunFile[] {
  const q = query.trim().toLowerCase();
  return files
    .filter(file => (filter === 'all' ? true : filter === 'findings' ? fileFindings(file) > 0 : fileState(file) === filter))
    .filter(file => !q || file.path.toLowerCase().includes(q))
    .sort((a, b) => order(fileState(a)) - order(fileState(b)) || fileFindings(b) - fileFindings(a) || a.path.localeCompare(b.path));
}

function order(state: ReturnType<typeof fileState>): number {
  return { failed: 0, active: 1, done: 2 }[state];
}

export function stageOf(file: AnalysisRunFile, stage: string): AnalysisStageProgress | undefined {
  return file.stages.find(entry => entry.stage === stage);
}

/** "1h 4m", "3m 20s" or "45s". */
export function formatDuration(seconds: number | undefined): string {
  const s = Math.max(0, Math.round(seconds ?? 0));
  if (s >= 3600) return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
  if (s >= 60) return `${Math.floor(s / 60)}m ${s % 60}s`;
  return `${s}s`;
}
