// Realistic daemon state for browser tests. Values follow the Go DTOs the
// daemon serves (task states, workflow statuses, gitops integration states,
// finding sources), so the UI is exercised with what it really receives.

import type {
  AgentSettings, AnalysisRunFile, AnalysisRunSummary, AnalysisStageProgress, AvailableModels, BudgetUsage, ChangeHistoryEntry, ChangeSession, CostEstimate, FileChange, Finding, GitWorkspace, Model, ModelsCurrent, Overview, Project, UsageReport,
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
        { name: 'create', status: 'completed', started_at: ago(19), finished_at: ago(16), usage: spend(2, 18_400, 2_100), cost: estimate(0.061) },
        { name: 'test', status: 'completed', started_at: ago(16), finished_at: ago(11), usage: spend(1, 9_800, 1_300), cost: estimate(0.034) },
        { name: 'review', status: 'completed', started_at: ago(11), finished_at: ago(7), usage: spend(3, 31_000, 1_900), cost: estimate(0.099) },
        { name: 'human_review', status: 'waiting', started_at: ago(7) },
      ],
      review: { proposal_hash: 'sha256:9f2c1a7be4', verdict: 'approve', summary: 'Safe and tested.', findings: [] },
      specialists: [{ id: 'correctness', status: 'completed', evidence: 'model', review: { verdict: 'approve', summary: 'Guard runs first.', findings: [] } }],
      input: { request_id: 'in-1', workflow_id: 'wf-1', status: 'waiting', action: 'approve', expires_at: ago(-60) },
    },
  };
}

/** The same task after the reviewers requested changes: no human input yet. */
export function rejectedTask(): ChangeSession {
  const task = reviewTask();
  const blocking = 'Move the MIME table into its own package.';
  return {
    ...task,
    reviewed_files: task.changes.map(change => change.path),
    workflow: {
      ...task.workflow!,
      status: 'changes_requested',
      input: undefined,
      stages: task.workflow!.stages.map(stage => (stage.name === 'human_review' ? { name: stage.name, status: 'blocked' } : stage)),
      review: { proposal_hash: task.hash, verdict: 'changes_requested', summary: '2 specialist reviews completed; 1 requested changes.', findings: [blocking], notes: ['Consider a table-driven test.'] },
      specialists: [
        { id: 'correctness', status: 'completed', evidence: 'model_advice', review: { proposal_hash: task.hash, verdict: 'approve', summary: 'Guard runs first.', findings: [], notes: ['Consider a table-driven test.'] } },
        { id: 'architecture', status: 'completed', evidence: 'model_advice', review: { proposal_hash: task.hash, verdict: 'changes_requested', summary: 'Package boundary.', findings: [blocking] } },
      ],
    },
  };
}

function spend(calls: number, input: number, output: number): BudgetUsage {
  return { model_calls: calls, input_tokens: input, output_tokens: output };
}

function estimate(total: number): CostEstimate {
  return { status: 'estimated', currency: 'USD', price_version: 'fixture-v1', known_subtotal: total, total, reasons: [] };
}

/** Agent usage per task, as GET /api/projects/current/usage reports it. */
export function usage(task: ChangeSession): UsageReport {
  const row = { runs: 4, usage: spend(6, 59_200, 5_300), cost: estimate(0.194) };
  return {
    ...identity,
    currency: 'USD',
    price_version: 'fixture-v1',
    tasks: [
      { session_id: task.id, title: task.title, kind: task.kind, state: task.state, updated_at: task.updated_at, ...row },
      { session_id: 'chg-3', title: 'Cache the parsed index', kind: 'performance', state: 'checked', updated_at: ago(55), runs: 2, usage: spend(3, 22_000, 2_400), cost: estimate(0.071) },
    ],
    other: { runs: 9, usage: spend(14, 120_000, 9_000), cost: { status: 'unknown', currency: 'USD', known_subtotal: 0.31, reasons: ['model_rate_unavailable'] } },
    total: { runs: 15, usage: spend(23, 201_200, 16_700), cost: { status: 'unknown', currency: 'USD', known_subtotal: 0.575, reasons: ['model_rate_unavailable'] } },
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

const runIdentity = { project_id: identity.project_id, project_revision: identity.project_revision, policy_fingerprint: '3:pol', provider_fingerprint: 'prov', queue_id: 'queue', id: 'analysis-1', generation: 'gen-1' };
const unreachable = 'The model provider could not be reached. Check that it is running and that config.yaml names the right address.';
const stage = (name: string, status: string, extra: Partial<AnalysisStageProgress> = {}): AnalysisStageProgress => ({ stage: name, status, attempts: status === 'pending' ? 0 : 1, cached: false, ...extra });

/** An analysis run like the one a daemon without config.yaml leaves: every model stage failed. */
export function analysisRun(status = 'partial'): AnalysisRunSummary {
  const failed = (path: string): AnalysisRunFile => ({ path, language: 'Go', stages: [stage('semantic', 'failed', { reason: unreachable }), stage('performance', 'failed', { reason: unreachable }), stage('security_rules', 'completed_empty', { finding_count: 0 }), stage('security_ai', 'failed', { reason: unreachable })] });
  return {
    identity: runIdentity,
    status,
    files: [
      failed('internal/vault/store.go'),
      failed('internal/vault/crypto.go'),
      { path: 'cmd/list.go', language: 'Go', stages: [stage('semantic', 'completed', { finding_count: 2 }), stage('performance', 'completed_empty', { finding_count: 0 }), stage('security_rules', 'completed', { finding_count: 1 }), stage('security_ai', status === 'running' ? 'running' : 'completed_empty', { finding_count: 0 })] },
      { path: 'README.md', language: 'Text', stages: [stage('semantic', 'skipped', { reason: 'Not eligible for semantic source analysis.', attempts: 0 }), stage('performance', 'completed_empty', { finding_count: 0 }), stage('security_rules', 'skipped', { reason: 'Passive security rules require a Go source file.', attempts: 0 }), stage('security_ai', 'completed_empty', { finding_count: 0 })] },
    ],
    sections: [
      { category: 'bugs', status: 'partial', coverage: { total: 3, pending: 0, running: 0, succeeded: 1, partial: 0, failed: 2, skipped: 0, unavailable: 0 }, finding_count: 2 },
      { category: 'performance', status: 'partial', coverage: { total: 4, pending: 0, running: 0, succeeded: 2, partial: 0, failed: 2, skipped: 0, unavailable: 0 }, finding_count: 0 },
      { category: 'security', status: 'partial', coverage: { total: 7, pending: 0, running: 0, succeeded: 4, partial: 0, failed: 2, skipped: 1, unavailable: 0 }, finding_count: 1 },
    ],
    plan: {
      limits: { batch_files: 64, budget_seconds: 1800, max_attempts_per_stage: 2 },
      providers: [{ id: 'p-bug', stages: ['semantic'], model: { scope: 'bug', model: 'local-bug-model', provider_origin: 'http://localhost:1234', remote_provider: false } }],
    },
    elapsed_seconds: 139,
    created_at: ago(30),
    updated_at: ago(28),
  };
}

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

// The model catalog: the configured scopes' models plus what the Agy CLI lists.
const agyModel = (id: string): Model => ({ scope: '', profile: 'agy:' + id, model: id, provider_origin: 'cli://agy', remote_provider: true, reasoning_effort: 'medium' });
export const availableModels: AvailableModels = {
  models: [
    { id: 'configured:' + 'a'.repeat(64), name: 'gemini-2.5-pro', provider: 'gemini', source: 'configured', location: 'remote', model: { ...models.scopes.analyze, scope: '', profile: 'configured:' + 'a'.repeat(64) } },
    { id: 'agy:gemini-3.8-flash-medium', name: 'Gemini 3.8 Flash', provider: 'agy', source: 'agy', location: 'unknown', model: agyModel('gemini-3.8-flash-medium') },
    { id: 'agy:gemini-3.8-pro-high', name: 'Gemini 3.8 Pro', provider: 'agy', source: 'agy', location: 'unknown', model: agyModel('gemini-3.8-pro-high') },
  ],
  defaults: { analyze: 'configured:' + 'a'.repeat(64) },
  pi: { status: 'unavailable', message: 'Pi models could not be loaded. Check that Pi is installed, signed in, and its model configuration is valid, then refresh models.' },
  agy: { status: 'ready' },
};

/** The daemon's agent settings for `roles` and `limits`, resolved like the service does. */
export function agentSettings(version: number, roles: Record<string, string> | null, limits: Record<string, number> | null): AgentSettings {
  const ceilings = { model_calls: 40, tool_calls: 120, input_tokens: 400_000, output_tokens: 60_000, elapsed_seconds: 1_800, concurrent_calls: 2, repairs: 2 };
  const effective = roles ?? { creation: 'bug', testing: 'code', review: 'analyze' };
  const resolve = (id: string): Model | undefined => models.scopes[id] ?? availableModels.models.find(choice => choice.id === id)?.model;
  const roleModels: AgentSettings['role_models'] = {};
  for (const role of ['creation', 'testing', 'review'] as const) {
    const model = resolve(effective[role]);
    if (model) roleModels[role] = model;
  }
  return {
    ...project,
    configuration: 'c'.repeat(64),
    version,
    state: roles ? 'overridden' : 'configured',
    roles: effective,
    limits: limits ?? ceilings,
    ceilings,
    skills: [],
    models: Object.values(models.scopes),
    role_models: roleModels,
  };
}

export const fileSource = 'package handlers\n\nfunc ServeAsset() {}\n';
