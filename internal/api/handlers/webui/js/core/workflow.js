// Agent workflow and Git request shaping. Consent is captured per request:
// every remote destination the run will use must be listed and confirmed.

import { asArray, durationBetween, isTestPath } from './util.js';
import { normalizePhaseState } from './readiness.js';

export const STAGE_LABELS = { create: 'Create', test: 'Tests', review: 'Specialist review', human_review: 'Human review' };
export const SPECIALIST_LABELS = { correctness: 'Correctness', architecture: 'Architecture', test_quality: 'Test quality', security: 'Security', performance: 'Performance' };
const DEFAULT_ROLES = { creation: 'bug', testing: 'code', review: 'analyze' };

export function workflowStages(workflow) {
  return asArray(workflow && workflow.stages).map(stage => ({
    name: stage.name,
    label: STAGE_LABELS[stage.name] || stage.name,
    status: stage.status || 'pending',
    tone: normalizePhaseState(stage.status),
    reason: stage.reason || '',
    model: stage.model && stage.model.model ? (stage.model.provider_origin ? stage.model.model + ' · ' + stage.model.provider_origin : stage.model.model) : '',
    durationMs: durationBetween(stage.started_at, stage.finished_at)
  }));
}

export function workflowSpecialists(workflow) {
  return asArray(workflow && workflow.specialists).map(item => ({
    id: item.id,
    label: SPECIALIST_LABELS[item.id] || item.id,
    status: item.status || 'pending',
    tone: normalizePhaseState(item.review && item.review.verdict ? item.review.verdict : item.status),
    verdict: item.review ? item.review.verdict : '',
    summary: item.review ? item.review.summary : '',
    findings: asArray(item.review && item.review.findings),
    evidence: item.evidence || '',
    reason: item.reason || ''
  }));
}

/** Explains why the agent workflow cannot start for this session, or ''. */
export function workflowBlocker(session, trusted) {
  if (!session) return 'Select a task first.';
  if (session.state === 'applied') return 'This proposal is already applied.';
  if (session.freshness && session.freshness !== 'current') return 'Refresh the stale proposal first.';
  if (session.kind === 'instructions') return 'Instruction edits use the manual proposal flow.';
  if (session.workflow && (session.workflow.status === 'running' || session.workflow.status === 'canceling')) return 'A workflow is already running.';
  const paths = asArray(session.targets).map(t => t.path).concat(asArray(session.paths));
  if (!paths.some(isTestPath)) return 'Add a test file (for example *_test.go) to the task scope so the testing agent can write tests.';
  if (!trusted) return 'Grant local execution trust in Settings so checks can run.';
  return '';
}

/** Resolves role selections and which of them reach a remote provider. */
export function workflowDestinations(settings, models) {
  const roles = Object.assign({}, DEFAULT_ROLES, settings && settings.roles);
  const selections = { create: roles.creation, test: roles.testing, review: roles.review };
  const scopes = (models && models.scopes) || {};
  const catalog = asArray(settings && settings.models);
  const destinations = [];
  const confirmed = [];
  for (const stage of ['create', 'test', 'review']) {
    const selection = selections[stage];
    const model = scopes[selection] || catalog.find(entry => entry.scope === selection) || null;
    if (model && model.remote_provider) {
      destinations.push({ stage, selection, model: model.model || selection, origin: model.provider_origin || 'remote provider' });
      if (!confirmed.includes(selection)) confirmed.push(selection);
    }
  }
  return { selections, destinations, confirmed };
}

export function workflowRequest(identity, message, choice, options = {}) {
  return Object.assign({}, identity, {
    message,
    models: choice.selections,
    confirmed_profiles: choice.confirmed,
    confirm_security: Boolean(options.confirmSecurity),
    auto_repair: Boolean(options.autoRepair),
    plan_hash: options.planHash || undefined
  });
}

/** Builds the guarded merge (or preview) request for a worktree into the main checkout. */
export function mergeRequest(workspace, worktree, project) {
  return {
    project_id: project.project_id,
    project_revision: project.project_revision,
    head: workspace.head,
    branch: workspace.branch,
    path: worktree.path,
    source_head: worktree.head,
    source_branch: worktree.branch,
    confirm: true,
    confirm_execution: true
  };
}

export function parseCommits(text) {
  return String(text || '').split('\n').map(line => line.trim()).filter(Boolean).map(line => {
    const space = line.indexOf(' ');
    return space > 0 ? { sha: line.slice(0, space), subject: line.slice(space + 1) } : { sha: line, subject: '' };
  });
}
