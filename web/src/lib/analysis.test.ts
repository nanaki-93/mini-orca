import { describe, expect, it } from 'vitest';
import type { AnalysisRunFile, AnalysisRunSummary } from '@/api/types';
import { failureGroups, fileState, filterFiles, formatDuration, runActive, runProgress, runResumable } from './analysis';

const file = (path: string, statuses: [string, string?, number?][]): AnalysisRunFile => ({
  path,
  stages: statuses.map(([status, reason, count], i) => ({ stage: ['semantic', 'performance', 'security_rules', 'security_ai'][i], status, attempts: 1, cached: false, reason, finding_count: count })),
});

const run: AnalysisRunSummary = {
  status: 'running',
  identity: { project_id: 'p', project_revision: 'r', policy_fingerprint: 'x', provider_fingerprint: 'y', queue_id: 'q', id: 'a', generation: 'g' },
  files: [
    file('b.go', [['failed', 'unreachable'], ['failed', 'unreachable'], ['completed_empty'], ['failed', 'timed out']]),
    file('a.go', [['completed', undefined, 2], ['running'], ['pending'], ['pending']]),
    file('c.md', [['skipped'], ['completed_empty'], ['skipped'], ['completed', undefined, 1]]),
  ],
};

describe('analysis', () => {
  it('measures progress over every stage', () => {
    expect(runProgress(run)).toEqual({ total: 12, done: 9, running: 1, failed: 3, succeeded: 4, percent: 75 });
    expect(runProgress(null).percent).toBe(0);
    expect(runActive(run)).toBe(true);
    expect(runResumable({ ...run, status: 'paused' })).toBe(true);
    expect(runResumable({ ...run, status: 'partial' })).toBe(false);
  });

  it('groups failures by reason, most frequent first', () => {
    expect(failureGroups(run)).toEqual([
      { reason: 'unreachable', count: 2, stages: ['semantic', 'performance'], files: ['b.go'] },
      { reason: 'timed out', count: 1, stages: ['security_ai'], files: ['b.go'] },
    ]);
  });

  it('filters files and puts failed ones first', () => {
    const files = run.files!;
    expect(files.map(fileState)).toEqual(['failed', 'active', 'done']);
    expect(filterFiles(files, 'all', '').map(f => f.path)).toEqual(['b.go', 'a.go', 'c.md']);
    expect(filterFiles(files, 'findings', '').map(f => f.path)).toEqual(['a.go', 'c.md']);
    expect(filterFiles(files, 'all', '.MD').map(f => f.path)).toEqual(['c.md']);
  });

  it('formats durations', () => {
    expect([formatDuration(45), formatDuration(139), formatDuration(3720)]).toEqual(['45s', '2m 19s', '1h 2m']);
  });
});
