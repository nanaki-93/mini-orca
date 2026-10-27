// Agent workflow presentation and request shaping. Consent is captured per
// request: every remote destination the run will use is listed and confirmed.

import type { AgentSettings, ChangeWorkflow, GitWorkspace, GitWorktree, ModelsCurrent, ProjectIdentity } from '@/api/types';
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
}

export function workflowStages(workflow: ChangeWorkflow | undefined): StageView[] {
  return (workflow?.stages ?? []).map(stage => ({
    name: stage.name,
    label: STAGE_LABELS[stage.name] ?? stage.name,
    actor: STAGE_ACTORS[stage.name] ?? '',
    status: stage.status || 'pending',
    tone: phaseTone(stage.status),
    reason: stage.reason ?? '',
    model: stage.model?.model ?? '',
    durationMs: durationBetween(stage.started_at, stage.finished_at),
    startedAt: stage.started_at,
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
    evidence: item.evidence ?? '',
    reason: item.reason ?? '',
    model: item.model?.model ?? '',
  }));
}

const DEFAULT_ROLES = { creation: 'bug', testing: 'code', review: 'analyze' } as const;

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
  for (const stage of ['create', 'test', 'review'] as const) {
    const selection = selections[stage];
    const model = scopes[selection] ?? catalog.find(entry => entry.scope === selection);
    if (model?.remote_provider) {
      destinations.push({ stage, selection, model: model.model || selection, origin: model.provider_origin || 'remote provider' });
      if (!confirmed.includes(selection)) confirmed.push(selection);
    } else {
      local.push({ stage, model: model?.model || selection });
    }
  }
  return { selections, destinations, confirmed, local };
}

export function workflowRequest(identity: ProjectIdentity & { revision: number; hash: string }, message: string, choice: DestinationChoice, options: { autoRepair?: boolean; confirmSecurity?: boolean; planHash?: string } = {}) {
  return {
    ...identity,
    message,
    models: choice.selections,
    confirmed_profiles: choice.confirmed,
    confirm_security: Boolean(options.confirmSecurity),
    auto_repair: Boolean(options.autoRepair),
    plan_hash: options.planHash || undefined,
  };
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
