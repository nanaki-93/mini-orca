// Realistic daemon state for browser tests. Values follow the Go DTOs the
// daemon serves (task states, workflow statuses, gitops integration states,
// finding sources), so the UI is exercised with what it really receives.

import type {
  ChangeHistoryEntry, ChangeSession, FileChange, Finding, GitWorkspace, ModelsCurrent, Overview, Project,
} from '../../src/api/types';

const now = Date.parse('2026-10-27T10:00:00Z');
export const ago = (minutes: number) => new Date(now - minutes * 60_000).toISOString();

export const project: Project = {
  project_id: 'p1',
  project_revision: 'r42',
  name: 'mini-orca',
  path: '/work/mini-orca',
  type: 'go',
  file_count: 748,
  total_lines: 169_089,
};

const identity = { project_id: project.project_id, project_revision: project.project_revision };

export const indexedPaths = [
  'cmd/daemon/main.go',
  'internal/api/handlers/webui.go',
  'internal/api/handlers/webui_test.go',
  'internal/app/service.go',
  'internal/llm/client.go',
  'README.md',
];

const added = (n: number, text: string) => ({ kind: 'added', new_line: n, text });
const removed = (n: number, text: string) => ({ kind: 'removed', old_line: n, text });
const context = (o: number, n: number, text: string) => ({ kind: 'context', old_line: o, new_line: n, text });

function change(path: string, lines: FileChange['diff']['lines'], summary: string): FileChange {
  return { path, content: '', hash: 'h-' + path, summary, diff: { lines } };
}

/** A task the agents finished: three changed files waiting for human review. */
export function reviewTask(): ChangeSession {
  return {
    ...identity,
    id: 'chg-1',
    kind: 'fix',
    title: 'Reject nested asset paths in the Web UI handler',
    revision: 3,
    hash: 'sha256:9f2c1a7be4',
    state: 'checked',
    freshness: 'current',
    updated_at: ago(6),
    acceptance_criteria: ['Nested asset paths return 404', 'Missing assets return 404 instead of 500'],
    targets: [
      { path: 'internal/api/handlers/webui.go', hash: 'a', exists: true },
      { path: 'internal/api/handlers/webui_test.go', hash: 'b', exists: true },
      { path: 'internal/api/handlers/mime.go', hash: '', exists: false },
    ],
    plan: { approach: 'Reject names with a slash or a parent segment before reading the embedded files.' },
    changes: [
      change('internal/api/handlers/webui.go', [context(41, 41, 'name := trim(r)'), removed(42, 'if name == "" {'), added(42, 'if name == "" || strings.Contains(name, "/") {')], 'Rejects nested names.'),
      change('internal/api/handlers/webui_test.go', [added(88, 'func TestServeAssetRejectsNestedPaths(t *testing.T) {}')], 'Table test for nested names.'),
      change('internal/api/handlers/mime.go', [added(1, 'package handlers')], 'Content types for assets.'),
    ],
    reviewed_files: [],
    file_review_hash: 'sha256:9f2c1a7be4',
    messages: [],
    checks: [
      { name: 'go vet', required: true, state: 'passed', command: ['go', 'vet', './...'], duration_ms: 4200 },
      { name: 'go test', required: true, state: 'passed', command: ['go', 'test', './...'], duration_ms: 18_350 },
      { name: 'staticcheck', required: false, state: 'failed', command: ['staticcheck', './...'], output: 'mime.go:5:6: should not use init', exit_code: 1 },
    ],
    git: { head: '41030b7e', branch: 'main' },
    workflow: {
      id: 'wf-1',
      status: 'awaiting_human_review',
      stages: [
        { name: 'create', status: 'completed', started_at: ago(19), finished_at: ago(16) },
        { name: 'test', status: 'completed', started_at: ago(16), finished_at: ago(11) },
        { name: 'review', status: 'completed', started_at: ago(11), finished_at: ago(7) },
        { name: 'human_review', status: 'waiting', started_at: ago(7) },
      ],
      review: { verdict: 'approve', summary: 'Safe and tested.', findings: [] },
      specialists: [{ id: 'correctness', status: 'completed', evidence: 'model', review: { verdict: 'approve', summary: 'Guard runs first.', findings: [] } }],
      input: { request_id: 'in-1', workflow_id: 'wf-1', status: 'waiting', action: 'approve', expires_at: ago(-60) },
    },
  };
}

export function historyEntry(session: ChangeSession, extra: Partial<ChangeHistoryEntry> = {}): ChangeHistoryEntry {
  return {
    ...identity,
    id: session.id,
    kind: session.kind,
    title: session.title,
    revision: session.revision,
    hash: session.hash,
    state: session.state,
    freshness: session.freshness,
    commit: session.commit,
    updated_at: session.updated_at,
    paths: session.targets.map(target => target.path),
    workflow_status: session.workflow?.status,
    ...extra,
  };
}

export const otherTasks: ChangeHistoryEntry[] = [
  { ...identity, id: 'chg-2', kind: 'feature', title: 'Show per-task model cost', revision: 0, hash: '', state: 'draft', freshness: 'current', updated_at: ago(2), workflow_status: 'running', paths: ['internal/app/agent_usage.go'] },
  { ...identity, id: 'chg-3', kind: 'performance', title: 'Cache the parsed index', revision: 2, hash: 'x', state: 'checked', freshness: 'current', updated_at: ago(55), workflow_status: 'failed', workflow_reason: 'go test timed out after 120s', paths: ['internal/project/index.go'] },
  { ...identity, id: 'chg-6', kind: 'fix', title: 'Report the stop reason', revision: 4, hash: 'z', state: 'applied', freshness: 'current', updated_at: ago(3000), commit: { state: 'committed', oid: 'f29fad51' }, paths: ['internal/app/change_pipeline.go'] },
];

export const findings: Finding[] = [
  { id: 'f1', source: 'deterministic', severity: 'high', title: 'Request body read without a size limit', message: 'io.ReadAll on r.Body.', location: { path: 'internal/api/handlers/change_handler.go', start_line: 88 }, status: 'open', category: 'security' },
  { id: 'f2', source: 'ai', severity: 'medium', title: 'Retry loop ignores cancellation', message: 'time.Sleep without ctx.Done().', location: { path: 'internal/llm/client.go', start_line: 212 }, status: 'open', category: 'bug' },
  { id: 'f3', source: 'vet', severity: 'low', title: 'Unchecked Close error', message: 'A failed Close can lose data.', location: { path: 'internal/storage/store.go', start_line: 64 }, status: 'dismissed', category: 'bug' },
];

export const overview: Overview = {
  ...identity,
  analysis_coverage: { fresh: 402, stale: 21, missing: 8 },
  analysis_run: { status: 'completed', updated_at: ago(180) },
  docs: {
    status: 'ready',
    context: { purpose: 'A local-first coding assistant.', goal_pursuit: ['Deterministic indexing'], topology: 'Loopback daemon', stack: ['Go', 'React'] },
    components: [{ name: 'App service', layer: 'application', path: 'internal/app', summary: 'Workflow orchestration.' }],
  },
};

export function gitWorkspace(): GitWorkspace {
  return {
    ...identity,
    available: true,
    root: project.path,
    head: '41030b7e',
    branch: 'main',
    dirty: false,
    conflicts: [],
    worktrees: [
      { path: project.path, head: '41030b7e', branch: 'main', current: true, integration: 'current', sessions: [] },
      { path: '/work/.mini-orca/wt/chg-2', head: 'aa11', branch: 'mo/chg-2', integration: 'up_to_date', mergeable: false, sessions: [otherTasks[0]] },
      { path: '/work/.mini-orca/wt/chg-7', head: 'bb22', branch: 'mo/chg-7', integration: 'behind', mergeable: true, overlap_paths: ['internal/app/git_merge.go'], sessions: [] },
    ],
  };
}

export const models: ModelsCurrent = {
  scopes: {
    analyze: { scope: 'analyze', model: 'gemini-2.5-pro', provider_origin: 'generativelanguage.googleapis.com', remote_provider: true },
    bug: { scope: 'bug', model: 'qwen3-coder', provider_origin: 'http://127.0.0.1:1234', remote_provider: false },
    code: { scope: 'code', model: 'qwen3-coder', provider_origin: 'http://127.0.0.1:1234', remote_provider: false },
  },
};

export const fileSource = 'package handlers\n\nfunc ServeAsset() {}\n';
