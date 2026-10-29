// Agent workflow presentation and request shaping. Consent is captured per
// request: every remote destination the run will use is listed and confirmed.

import type { AgentSettings, BudgetUsage, ChangeWorkflow, CostEstimate, GitWorkspace, GitWorktree, ModelsCurrent, ProjectIdentity } from '@/api/types';
import { durationBetween } from './util';
import { phaseTone, type PhaseTone } from './readiness';

export const STAGE_LABELS: Record<string, string> = {
  create: 'Write the change',
  test: 'Write and run tests',
  review: 'Specialist review',
  human_review: 'Your review',
};

export const STAGE_ACTORS: Record<string, string> = {
  create: 'Creation agent',
  test: 'Testing agent',
  review: 'Reviewers',
  human_review: 'You',
};

export const SPECIALIST_LABELS: Record<string, string> = {
  correctness: 'Correctness',
  architecture: 'Architecture',
  test_quality: 'Test quality',
  security: 'Security',
  performance: 'Performance',
};

export interface StageView {
  name: string;
  label: string;
  actor: string;
  status: string;
  tone: PhaseTone;
  reason: string;
  model: string;
  durationMs: number;
  startedAt?: string;
  usage?: BudgetUsage;
  cost?: CostEstimate;
}

export function workflowStages(workflow: ChangeWorkflow | undefined): StageView[] {
  // A checks start keeps the tests: its test stage only runs the checks,
  // until a repair has the testing agent write them again.
  const checksOnly = workflow?.resume_from === 'checks' && !workflow.auto_repair_attempts;
  return (workflow?.stages ?? []).map(stage => ({
    name: stage.name,
    label: checksOnly && stage.name === 'test' ? 'Run the checks' : STAGE_LABELS[stage.name] ?? stage.name,
    actor: checksOnly && stage.name === 'test' ? 'Checks' : STAGE_ACTORS[stage.name] ?? '',
    status: stage.status || 'pending',
    tone: phaseTone(stage.status),
    reason: stage.reason ?? '',
    model: checksOnly && stage.name === 'test' ? '' : stage.model?.model ?? '',
    durationMs: durationBetween(stage.started_at, stage.finished_at),
    startedAt: stage.started_at,
    usage: stage.usage,
    cost: stage.cost,
  }));
}

/** The reviewers' overall verdict: the daemon's aggregate, else derived from the specialists. */
export function reviewVerdict(workflow: ChangeWorkflow | undefined): 'approve' | 'changes_requested' | 'pending' | 'none' {
  if (!workflow) return 'none';
  const aggregate = workflow.review?.verdict;
  if (aggregate === 'approve' || aggregate === 'changes_requested') return aggregate;
  const specialists = workflow.specialists ?? [];
  if (specialists.some(sp => sp.review?.verdict === 'changes_requested' || sp.review?.verdict === 'reject')) return 'changes_requested';
  if (specialists.length && specialists.every(sp => sp.review?.verdict === 'approve')) return 'approve';
  return 'pending';
}

/** The human override of the reviewers' rejection, when it covers this exact hash. */
export function activeOverride(workflow: ChangeWorkflow | undefined, hash: string | undefined) {
  const override = workflow?.override;
  return override && hash && override.proposal_hash === hash && workflow?.review?.proposal_hash === hash ? override : undefined;
}

const RUN_LABELS: Record<string, string> = {
  queued: 'Queued',
  running: 'Working',
  canceling: 'Stopping',
  awaiting_human_review: 'Finished',
  changes_requested: 'Finished with requested changes',
  failed: 'Failed',
  canceled: 'Stopped',
  interrupted: 'Interrupted',
  stale: 'Outdated',
  outdated: 'Outdated',
};

/** How far the agent run got, worded for the review gate. */
export function agentRunLabel(workflow: ChangeWorkflow | undefined, hasChanges: boolean): string {
  if (!workflow) return hasChanges ? 'Manual proposal' : 'Not started';
  return RUN_LABELS[workflow.status] ?? workflow.status.replace(/_/g, ' ');
}

/** A short human label for a stage that has no duration yet. */
export function stageStatusLabel(stage: Pick<StageView, 'name' | 'status' | 'tone'>): string {
  if (stage.name === 'human_review' && (stage.status === 'waiting' || stage.status === 'awaiting_human_review')) return 'Waiting for you';
  if (stage.tone === 'running') return 'Running…';
  return (stage.status || 'pending').replace(/_/g, ' ').replace(/^./, c => c.toUpperCase());
}

export interface SpecialistView {
  id: string;
  label: string;
  status: string;
  tone: PhaseTone;
  verdict: string;
  summary: string;
  findings: string[];
  notes: string[];
  evidence: string;
  reason: string;
  model: string;
}

export function workflowSpecialists(workflow: ChangeWorkflow | undefined): SpecialistView[] {
  return (workflow?.specialists ?? []).map(item => ({
    id: item.id,
    label: SPECIALIST_LABELS[item.id] ?? item.id,
    status: item.status || 'pending',
    tone: phaseTone(item.review?.verdict ?? item.status),
    verdict: item.review?.verdict ?? '',
    summary: item.review?.summary ?? '',
    findings: item.review?.findings ?? [],
    notes: item.review?.notes ?? [],
    evidence: item.evidence ?? '',
    reason: item.reason ?? '',
    model: item.model?.model ?? '',
  }));
}

const BUILD_MANIFESTS = new Set(['go.mod', 'go.sum', 'go.work', 'go.work.sum', 'package.json', 'package-lock.json', 'npm-shrinkwrap.json', 'pnpm-lock.yaml', 'yarn.lock', 'tsconfig.json', 'Cargo.toml', 'Cargo.lock', 'pyproject.toml', 'setup.py', 'setup.cfg', 'requirements.txt', 'Pipfile', 'Pipfile.lock', 'Makefile', 'justfile']);

const directoryOf = (path: string) => (path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : '');

/**
 * Whether writing `changed` likely makes a task with `paths` stale: a shared
 * file or package directory, or a build manifest above one of its files. The
 * daemon decides exactly from each task's recorded scope.
 */
export function changeOverlaps(changed: string[], paths: string[]): boolean {
  return changed.some(file => {
    const dir = directoryOf(file);
    const name = file.slice(dir ? dir.length + 1 : 0);
    return paths.some(path => {
      const own = directoryOf(path);
      return path === file || own === dir || (BUILD_MANIFESTS.has(name) && (dir === '' || own === dir || own.startsWith(dir + '/')));
    });
  });
}

export const DEFAULT_ROLES = { creation: 'bug', testing: 'code', review: 'analyze' } as const;

export interface Destination {
  stage: string;
  selection: string;
  model: string;
  origin: string;
}

export interface DestinationChoice {
  selections: { create: string; test: string; review: string };
  destinations: Destination[];
  confirmed: string[];
  local: { stage: string; model: string }[];
}

/** Resolves role selections and which of them reach a remote provider. */
export function workflowDestinations(settings: AgentSettings | null | undefined, models: ModelsCurrent | null | undefined): DestinationChoice {
  const roles = { ...DEFAULT_ROLES, ...(settings?.roles ?? {}) };
  const selections = { create: roles.creation, test: roles.testing, review: roles.review };
  const scopes = models?.scopes ?? {};
  const catalog = settings?.models ?? [];
  const destinations: Destination[] = [];
  const local: { stage: string; model: string }[] = [];
  const confirmed: string[] = [];
  const roleOf = { create: 'creation', test: 'testing', review: 'review' } as const;
  for (const stage of ['create', 'test', 'review'] as const) {
    const selection = selections[stage];
    const model = settings?.role_models?.[roleOf[stage]] ?? scopes[selection] ?? catalog.find(entry => entry.scope === selection);
    if (model?.remote_provider) {
      destinations.push({ stage, selection, model: model.model || selection, origin: model.provider_origin || 'remote provider' });
      if (!confirmed.includes(selection)) confirmed.push(selection);
    } else {
      local.push({ stage, model: model?.model || selection });
    }
  }
  return { selections, destinations, confirmed, local };
}

export function workflowRequest(identity: ProjectIdentity & { revision: number; hash: string }, message: string, choice: DestinationChoice, options: { autoRepair?: boolean; confirmSecurity?: boolean; planHash?: string; includeComments?: boolean; start?: RunStart } = {}) {
  return {
    ...identity,
    message,
    models: choice.selections,
    confirmed_profiles: choice.confirmed,
    confirm_security: Boolean(options.confirmSecurity),
    auto_repair: Boolean(options.autoRepair),
    plan_hash: options.planHash || undefined,
    include_comments: options.includeComments || undefined,
    resume_from: options.start && options.start !== 'create' ? options.start : undefined,
  };
}

/** Where a run of the agents starts on a task that already has a proposal. */
export type RunStart = 'checks' | 'test' | 'create';

export interface RunStartOption {
  value: RunStart;
  label: string;
  detail: string;
  /** Why the daemon would refuse this start, or ''. */
  blocked: string;
}

/**
 * The starts a run offers, and the one chosen first. Without a proposal the
 * agents write it from the brief and there is nothing to choose. With one, the
 * first choice keeps it as it is (checks, then reviewers) unless it has no
 * tests yet. Keeping the proposal keeps the creation stage, so the daemon
 * requires its model unchanged since the last run.
 */
export function runStartOptions(session: { changes?: { path: string }[] | null; workflow?: ChangeWorkflow }, createModel: string, isTestPath: (path: string) => boolean): { options: RunStartOption[]; initial: RunStart } {
  const changes = session.changes ?? [];
  if (!changes.length) return { options: [], initial: 'create' };
  const kept = session.workflow?.stages.find(stage => stage.name === 'create')?.model?.profile;
  const blocked = session.workflow && kept !== createModel ? 'The creation model changed since the last run, so the agents write the change again.' : '';
  const options: RunStartOption[] = [
    { value: 'checks', label: 'Check and review this version', detail: 'Keeps the code and tests as they are: runs the checks, then the reviewers.', blocked },
    { value: 'test', label: 'Write new tests, then check and review', detail: 'Keeps the implementation; the testing agent rewrites the test files.', blocked },
    { value: 'create', label: 'Write the change again', detail: 'The creation agent revises the code from the brief, the conversation and this version, then tests, checks and review follow.', blocked: '' },
  ];
  const initial: RunStart = blocked ? 'create' : changes.some(change => isTestPath(change.path)) ? 'checks' : 'test';
  return { options, initial };
}

/** The guarded merge (or preview) request for a worktree into the main checkout. */
export function mergeRequest(workspace: GitWorkspace, worktree: GitWorktree, project: ProjectIdentity) {
  return {
    project_id: project.project_id,
    project_revision: project.project_revision,
    head: workspace.head,
    branch: workspace.branch,
    path: worktree.path,
    source_head: worktree.head,
    source_branch: worktree.branch,
    confirm: true,
    confirm_execution: true,
  };
}

export function parseCommits(text: string | undefined): { sha: string; subject: string }[] {
  return (text ?? '')
    .split('\n')
    .map(line => line.trim())
    .filter(Boolean)
    .map(line => {
      const space = line.indexOf(' ');
      return space > 0 ? { sha: line.slice(0, space), subject: line.slice(space + 1) } : { sha: line, subject: '' };
    });
}

/** Explains why the agent workflow cannot start for this task, or ''. */
export function workflowBlocker(session: { state: string; freshness: string; kind: string; workflow?: ChangeWorkflow; targets?: { path: string }[] } | null, trusted: boolean, isTestPath: (path: string) => boolean): string {
  if (!session) return 'Select a task first.';
  if (session.state === 'applied') return 'This task is already applied.';
  if (session.freshness && session.freshness !== 'current') return 'The project changed since this task was captured.';
  if (session.kind === 'instructions') return 'Instruction edits use the manual proposal flow.';
  if (session.workflow && ['running', 'canceling'].includes(session.workflow.status)) return 'Agents are already working on this task.';
  if (!(session.targets ?? []).some(target => isTestPath(target.path))) return 'Add a test file to the scope so the testing agent can write tests.';
  if (!trusted) return 'Allow this project to run checks (Settings → Trust) so agents can verify their work.';
  return '';
}
