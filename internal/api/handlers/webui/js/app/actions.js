// Workspace operations. This module never touches the DOM: it reads and writes
// the shared state, calls the daemon and asks the UI port for consent
// (confirm/prompt) and feedback (toast). Every mutation:
//   * runs at most once at a time (state.pending),
//   * carries the captured project/proposal identity,
//   * asks for explicit confirmation where the daemon requires `confirm`, and
//   * ignores late responses that belong to an earlier selection.

import { withWorktree, query } from './api.js';
import { asArray, companionTestPath, isTestPath, plural, unique } from '../core/util.js';
import { changeIdentity, effectiveEdits, isFileReviewed, nextUnreviewedPath, proposalReadiness, workflowActive } from '../core/readiness.js';
import { mergeRequest, workflowDestinations, workflowRequest } from '../core/workflow.js';
import { parseGitDiff } from '../core/diff.js';

export const MAX_TASK_PATHS = 8;

export function createInitialState(route) {
  return {
    route,
    project: null,
    mainProject: null,
    models: null,
    version: '',
    index: null,
    trust: null,
    gitWorkspace: null,
    changes: [],
    findings: [],
    overview: null,
    scan: null,
    activeSession: null,
    sessionRequest: 0,
    pending: '',
    loaded: false,
    loadError: '',
    connection: 'connecting',
    explorer: { expanded: new Set(), filter: '', tabs: [], info: null, analysis: null, explanation: null, loadingPath: '' },
    review: { mode: 'split', sinceLastReview: false, allFiles: false, expandedGaps: {}, filter: '', workflowMessage: null, autoRepair: false, refineMessage: '', commit: false, commitMessage: null },
    issues: { category: 'all', status: 'open', severity: 'all', q: '', selected: '' },
    checks: { filter: 'all' },
    tasks: { q: '' },
    draft: emptyDraft(),
    merge: { path: '', preview: null, files: [], loading: false }
  };
}

export function emptyDraft() {
  return { step: 1, title: '', kind: 'feature', spec: '', paths: [], pathQuery: '', criteria: '', mode: 'workflow', autoRepair: false, error: '', errorField: '' };
}

export function createActions({ state, api, ui, notify }) {
  const changed = () => notify && notify();
  const scoped = path => withWorktree(path, state.route.worktreePath);
  const sessionScoped = (session, path) => withWorktree(path, (session && session._worktreePath) || state.route.worktreePath || '');
  const changePath = (session, suffix) => sessionScoped(session, '/api/projects/current/changes/' + encodeURIComponent(session.id) + suffix);

  function fail(prefix, err) {
    ui.toast(prefix + ': ' + (err && err.message ? err.message : String(err)), 'danger');
  }

  /** Runs one mutation at a time; a second call while one is pending is ignored. */
  async function guarded(label, fn) {
    if (state.pending) return undefined;
    state.pending = label;
    changed();
    try {
      return await fn();
    } finally {
      state.pending = '';
      changed();
    }
  }

  function remoteScope(scope) {
    const model = state.models && state.models.scopes && state.models.scopes[scope];
    return model && model.remote_provider ? model : null;
  }

  async function confirmRemote(scope, action) {
    const model = remoteScope(scope);
    if (!model) return { ok: true, remote: false };
    const ok = await ui.confirm({
      title: 'Send context to a remote provider?',
      body: action + ' uses ' + (model.model || scope) + ' at ' + (model.provider_origin || 'a remote provider') + ' (scope: ' + scope + '). Bounded project context for this request leaves your machine.',
      confirmLabel: 'Send request'
    });
    return { ok, remote: true };
  }

  // ---------------------------------------------------------------- loading

  async function loadAll() {
    try {
      const [status, models] = await Promise.all([
        api.get('/status').catch(() => null),
        api.get('/api/models/current').catch(() => null)
      ]);
      state.version = status && status.version ? status.version : state.version;
      state.models = models;
      state.mainProject = await api.get('/api/projects/current');
      if (state.route.worktreePath) {
        try {
          state.project = await api.get(scoped('/api/projects/current'));
        } catch (_) {
          ui.toast('The selected worktree is unavailable; showing the main checkout.', 'warning');
          state.route.worktreePath = '';
          state.project = state.mainProject;
        }
      } else {
        state.project = state.mainProject;
      }
      if (!state.project || !state.project.project_id) {
        state.loaded = true;
        changed();
        return;
      }
      const ident = query({ project_id: state.project.project_id, project_revision: state.project.project_revision });
      const rev = query({ project_revision: state.project.project_revision });
      const mainIdent = query({ project_id: state.mainProject.project_id, project_revision: state.mainProject.project_revision });
      const results = await Promise.allSettled([
        api.get(scoped('/api/projects/current/index')),
        api.get(scoped('/api/projects/current/execution-trust' + rev)),
        api.get('/api/projects/current/git/workspace' + mainIdent),
        api.get(scoped('/api/projects/current/changes' + ident)),
        api.get(scoped('/api/projects/current/findings' + rev)),
        api.get(scoped('/api/projects/current/overview' + rev)),
        api.get(scoped('/api/projects/current/scan' + rev))
      ]);
      const value = i => (results[i].status === 'fulfilled' ? results[i].value : undefined);
      if (value(0)) state.index = value(0);
      if (value(1)) state.trust = value(1);
      if (results[2].status === 'fulfilled') state.gitWorkspace = value(2);
      state.changes = mergeWorktreeSessions(asArray(value(3)), state.gitWorkspace, state.route.worktreePath);
      if (results[4].status === 'fulfilled' && value(4)) state.findings = asArray(value(4).findings);
      if (results[5].status === 'fulfilled') state.overview = value(5);
      if (results[6].status === 'fulfilled') state.scan = value(6) || null;
      state.loaded = true;
      state.loadError = '';
      if (!state.route.sessionId && state.changes.length) state.route.sessionId = state.changes[0].id;
      changed();
      if (state.route.sessionId) await loadSession(state.route.sessionId);
      else state.activeSession = null;
      if (state.route.file && state.route.tab === 'files') await openFile(state.route.file);
      changed();
    } catch (err) {
      state.loaded = true;
      state.loadError = err.message;
      changed();
      fail('Workspace load failed', err);
    }
  }

  async function checkoutIdentity(worktreePath) {
    if (!worktreePath && state.mainProject && state.mainProject.project_id) {
      return { project_id: state.mainProject.project_id, project_revision: state.mainProject.project_revision };
    }
    const project = await api.get(withWorktree('/api/projects/current', worktreePath));
    return { project_id: project.project_id, project_revision: project.project_revision };
  }

  async function loadSession(sessionId) {
    const request = ++state.sessionRequest;
    // Reloading the same proposal keeps it visible; selecting another one
    // drops the previous proposal's controls until the new one arrives.
    if (!state.activeSession || state.activeSession.id !== sessionId) state.activeSession = null;
    changed();
    if (!sessionId || !state.project) {
      state.activeSession = null;
      return;
    }
    const entry = state.changes.find(change => change.id === sessionId);
    const worktreePath = (entry && entry._worktreePath) || state.route.worktreePath || '';
    try {
      const ident = await checkoutIdentity(worktreePath);
      const loaded = await api.get(withWorktree('/api/projects/current/changes/' + encodeURIComponent(sessionId) + query(ident), worktreePath));
      if (request !== state.sessionRequest) return;
      loaded._worktreePath = worktreePath;
      loaded._checkoutProjectId = ident.project_id;
      loaded._checkoutProjectRevision = ident.project_revision;
      state.activeSession = loaded;
      if (!loaded.last_reviewed_hash) state.review.sinceLastReview = false;
      state.route.sessionId = sessionId;
      const edits = effectiveEdits(loaded, state.review.sinceLastReview);
      if (state.route.tab === 'diff' && !edits.some(edit => edit.path === state.route.file)) {
        state.route.file = nextUnreviewedPath(loaded, edits, '') || (edits[0] ? edits[0].path : '');
      }
      changed();
    } catch (err) {
      if (request !== state.sessionRequest) return;
      state.activeSession = null;
      changed();
      fail('Could not load task ' + sessionId, err);
    }
  }

  /** Replaces the active proposal with a server response while keeping checkout metadata. */
  function adopt(previous, next) {
    if (!next) return;
    next._worktreePath = previous._worktreePath || '';
    next._checkoutProjectId = previous._checkoutProjectId || next.project_id;
    next._checkoutProjectRevision = previous._checkoutProjectRevision || next.project_revision;
    if (state.activeSession && state.activeSession.id === previous.id) state.activeSession = next;
  }

  // ---------------------------------------------------------------- review

  async function reviewFile(path) {
    const session = state.activeSession;
    if (!session || !path || state.review.sinceLastReview || !proposalReadiness(session).editable) return;
    if (!asArray(session.changes).some(edit => edit.path === path) || isFileReviewed(session, path)) return;
    await guarded('Saving review…', async () => {
      try {
        const updated = await api.post(changePath(session, '/review-file'), Object.assign(changeIdentity(session, state.project), { path }));
        adopt(session, updated);
        const next = nextUnreviewedPath(state.activeSession, effectiveEdits(state.activeSession, false), path);
        if (next) state.route.file = next;
        ui.toast(next ? 'Marked ' + path + ' reviewed. Next: ' + next : 'All files reviewed. Run checks next.', 'success');
      } catch (err) {
        fail('File review failed', err);
      }
    });
  }

  async function approveReview() {
    const session = state.activeSession;
    if (!proposalReadiness(session).canApprove) return;
    await guarded('Approving…', async () => {
      try {
        const next = await api.post(changePath(session, '/review'), changeIdentity(session, state.project));
        adopt(session, next);
        ui.toast('Review approved for this exact version. You can apply it now.', 'success');
      } catch (err) {
        fail('Approval failed', err);
      }
    });
  }

  async function runChecks() {
    const session = state.activeSession;
    if (!session || !proposalReadiness(session).editable || !asArray(session.changes).length) return;
    await guarded('Running checks…', async () => {
      try {
        const updated = await api.post(changePath(session, '/checks'), Object.assign(changeIdentity(session, state.project), { run_lint: true, run_tests: true }));
        adopt(session, updated);
        const failed = asArray(updated && updated.checks).filter(check => check.state === 'failed').length;
        ui.toast(failed ? failed + ' check(s) failed. Inspect diagnostics before approval.' : 'Checks completed.', failed ? 'warning' : 'success');
      } catch (err) {
        fail('Checks failed to run', err);
      }
    });
  }

  async function applyProposal(options = {}) {
    const session = state.activeSession;
    if (!proposalReadiness(session).canApply) return;
    const destination = session._worktreePath || (state.project && state.project.path) || 'the current checkout';
    const commit = Boolean(options.commit && session.git && session.git.head);
    const message = String(options.message || '').trim();
    if (commit && !message) {
      ui.toast('Enter a commit message or turn off “Commit after apply”.', 'warning');
      return;
    }
    const ok = await ui.confirm({
      title: commit ? 'Apply and commit?' : 'Apply reviewed changes?',
      body: 'Write “' + (session.title || session.id) + '” to ' + destination + (commit ? ' and commit it on ' + (session.git.branch || 'the current branch') + '.' : '.') + ' Undo stays available while the files are unchanged.',
      confirmLabel: commit ? 'Apply & commit' : 'Apply changes'
    });
    if (!ok) return;
    await guarded('Applying…', async () => {
      try {
        const body = Object.assign(changeIdentity(session, state.project), { confirm: true });
        if (commit) Object.assign(body, { head: session.git.head, branch: session.git.branch, commit_message: message });
        const result = await api.post(changePath(session, '/apply'), body);
        const committed = result && result.commit && result.commit.state === 'committed';
        ui.toast(committed ? 'Applied and committed.' : 'Applied ' + (session.title || session.id) + '.', 'success');
        if (result && result.commit && result.commit.state === 'failed') ui.toast('Source was applied but the commit failed: ' + (result.commit.error || 'unknown error'), 'warning');
        await loadAll();
      } catch (err) {
        fail('Apply failed', err);
      }
    });
  }

  async function undoProposal() {
    const session = state.activeSession;
    if (!proposalReadiness(session).canUndo) return;
    const ok = await ui.confirm({
      title: 'Undo applied changes?',
      body: 'Restore the files changed by “' + (session.title || session.id) + '”. The daemon verifies they still match the applied version first.',
      confirmLabel: 'Undo changes',
      danger: true
    });
    if (!ok) return;
    await guarded('Undoing…', async () => {
      try {
        await api.post(changePath(session, '/undo'), Object.assign(changeIdentity(session, state.project), { confirm: true }));
        ui.toast('Changes undone.', 'success');
        await loadAll();
      } catch (err) {
        fail('Undo failed', err);
      }
    });
  }

  async function refine(message) {
    const session = state.activeSession;
    const text = String(message || '').trim();
    if (!session || !proposalReadiness(session).editable) return false;
    if (!text) {
      ui.toast('Describe what should change first.', 'warning');
      return false;
    }
    const consent = await confirmRemote('code', 'Refining “' + (session.title || session.id) + '”');
    if (!consent.ok) return false;
    return Boolean(await guarded('Generating…', async () => {
      try {
        const updated = await api.post(changePath(session, '/messages'), Object.assign(changeIdentity(session, state.project), { message: text, profile: 'code', confirm_remote_provider: consent.remote }));
        adopt(session, updated);
        state.review.sinceLastReview = false;
        ui.toast('Proposal updated. Review the new diff.', 'success');
        return true;
      } catch (err) {
        fail('Generation failed', err);
        return false;
      }
    }));
  }

  // ---------------------------------------------------------------- agent workflow

  async function startWorkflow(options = {}) {
    const session = state.activeSession;
    if (!session) return false;
    const message = String(options.message || session.title || '').trim();
    if (!message) {
      ui.toast('Describe the change for the agents first.', 'warning');
      return false;
    }
    let settings = null;
    try {
      settings = await api.get(sessionScoped(session, '/api/projects/current/agent-settings' + query({ project_id: session._checkoutProjectId || session.project_id, project_revision: session._checkoutProjectRevision || session.project_revision })));
    } catch (_) {
      settings = null; // Daemon defaults apply; unknown destinations are never auto-confirmed.
    }
    const choice = workflowDestinations(settings, state.models);
    const lines = choice.destinations.map(d => d.stage + ': ' + d.model + ' at ' + d.origin);
    const security = session.kind === 'security';
    const ok = await ui.confirm({
      title: 'Start the agent workflow?',
      body: 'Agents will create the change, write and run tests in an isolated workspace, then run specialist reviews. Nothing is applied without your review.' +
        (lines.length ? '\n\nRemote destinations:\n• ' + lines.join('\n• ') : '\n\nAll selected models run locally.') +
        (security ? '\n\nThis is a security task: a security specialist will review it.' : ''),
      confirmLabel: 'Start workflow'
    });
    if (!ok) return false;
    return Boolean(await guarded('Starting agents…', async () => {
      try {
        const body = workflowRequest(changeIdentity(session, state.project), message, choice, { autoRepair: options.autoRepair, confirmSecurity: security, planHash: session.plan && session.plan.hash });
        const updated = await api.post(changePath(session, '/workflow'), body);
        adopt(session, updated);
        ui.toast('Agent workflow started. Progress updates live.', 'info');
        return true;
      } catch (err) {
        fail('Workflow could not start', err);
        return false;
      }
    }));
  }

  async function cancelWorkflow() {
    const session = state.activeSession;
    if (!session || !workflowActive(session)) return;
    const ok = await ui.confirm({ title: 'Cancel the workflow?', body: 'Running agents stop. No source is applied.', confirmLabel: 'Cancel workflow', cancelLabel: 'Keep running', danger: true });
    if (!ok) return;
    await guarded('Canceling…', async () => {
      try {
        const updated = await api.post(changePath(session, '/workflow/cancel'), { project_id: session._checkoutProjectId || session.project_id, project_revision: session._checkoutProjectRevision || session.project_revision, workflow_id: session.workflow.id });
        adopt(session, updated);
        ui.toast('Workflow canceled.', 'info');
      } catch (err) {
        fail('Cancel failed', err);
      }
    });
  }

  async function respondWorkflowInput(decision) {
    const session = state.activeSession;
    const input = session && session.workflow && session.workflow.input;
    if (!input || input.status !== 'waiting' || !['approve', 'reject'].includes(decision)) return;
    if (decision === 'approve' && !proposalReadiness(session).canApprove) {
      ui.toast('Review every file and pass required checks before approving.', 'warning');
      return;
    }
    const ok = await ui.confirm({
      title: decision === 'approve' ? 'Approve this proposal?' : 'Reject this proposal?',
      body: decision === 'approve' ? 'Records your approval for this exact version. Apply stays a separate step.' : 'The workflow ends and this version can no longer be approved.',
      confirmLabel: decision === 'approve' ? 'Approve' : 'Reject',
      danger: decision === 'reject'
    });
    if (!ok) return;
    await guarded(decision === 'approve' ? 'Approving…' : 'Rejecting…', async () => {
      try {
        const updated = await api.post(changePath(session, '/workflow/input'), Object.assign(changeIdentity(session, state.project), { workflow_id: session.workflow.id, request_id: input.request_id, decision }));
        adopt(session, updated);
        ui.toast(decision === 'approve' ? 'Approved.' : 'Rejected.', 'success');
      } catch (err) {
        fail('Response failed', err);
      }
    });
  }

  // ---------------------------------------------------------------- tasks

  function draftError(message, field, step) {
    state.draft.error = message;
    state.draft.errorField = field;
    state.draft.step = step;
    changed();
    return false;
  }

  /** Moves the task wizard; validation never sends a request. */
  function setDraftStep(step) {
    const draft = state.draft;
    if (step > 1 && !draft.title.trim()) return draftError('Give your task a short title.', 'title', 1);
    if (step > 2 && (draft.paths.length < 1 || draft.paths.length > MAX_TASK_PATHS)) return draftError('Choose between 1 and ' + MAX_TASK_PATHS + ' files to continue.', 'paths', 2);
    draft.step = Math.max(1, Math.min(3, step));
    draft.error = '';
    draft.errorField = '';
    changed();
    return true;
  }

  function toggleDraftPath(path) {
    const paths = state.draft.paths.slice();
    const index = paths.indexOf(path);
    if (index >= 0) {
      paths.splice(index, 1);
    } else {
      paths.push(path);
      const companion = companionTestPath(path);
      const files = asArray(state.index && state.index.files);
      if (companion && files.some(file => file.path === companion) && !paths.includes(companion)) paths.push(companion);
    }
    state.draft.paths = unique(paths).slice(0, MAX_TASK_PATHS);
    if (paths.length > MAX_TASK_PATHS) ui.toast('A task can target at most ' + MAX_TASK_PATHS + ' files.', 'warning');
    changed();
  }

  function draftFromFinding(finding) {
    const location = finding.location || {};
    const path = location.path || (finding.task_spec && finding.task_spec.target_path) || '';
    const paths = [];
    if (path) {
      paths.push(path);
      const companion = companionTestPath(path);
      if (companion && asArray(state.index && state.index.files).some(file => file.path === companion)) paths.push(companion);
    }
    const criteria = asArray(finding.task_spec && finding.task_spec.acceptance_criteria);
    state.draft = Object.assign(emptyDraft(), {
      title: 'Fix: ' + (finding.title || finding.id || 'finding'),
      kind: 'fix',
      spec: 'Finding at ' + (path || 'workspace') + (location.start_line ? ':' + location.start_line : '') + '\n' + (finding.message || '') + (finding.evidence ? '\n\nEvidence:\n' + finding.evidence : ''),
      paths,
      criteria: criteria.length ? criteria.join('\n') : 'Resolve “' + (finding.title || finding.id) + '” without breaking existing callers\nAdd or update a regression test'
    });
    changed();
  }

  function draftFromFile(path) {
    state.draft = emptyDraft();
    if (path) toggleDraftPath(path);
    changed();
  }

  async function createTask(generate) {
    const draft = state.draft;
    if (!state.project) return null;
    const title = draft.title.trim();
    if (!title) return draftError('Give your task a short title.', 'title', 1);
    if (draft.paths.length < 1 || draft.paths.length > MAX_TASK_PATHS) return draftError('Choose between 1 and ' + MAX_TASK_PATHS + ' files.', 'paths', 2);
    const criteria = draft.criteria.split('\n').map(s => s.trim()).filter(Boolean);
    const mode = generate ? draft.mode : 'draft';
    if (mode === 'workflow' && !draft.paths.some(isTestPath)) return draftError('The agent workflow needs a test file in scope (for example *_test.go). Add one or switch to quick generation.', 'paths', 2);
    const created = await guarded('Creating task…', async () => {
      try {
        return await api.post(scoped('/api/projects/current/changes'), {
          project_id: state.project.project_id,
          project_revision: state.project.project_revision,
          kind: draft.kind,
          title,
          paths: draft.paths,
          acceptance_criteria: criteria.length ? criteria : ['Changes pass verification checks'],
          plan: draft.spec.trim() ? { problem: title, approach: draft.spec.trim() } : undefined
        });
      } catch (err) {
        fail('Task creation failed', err);
        return null;
      }
    });
    if (!created) return null;
    state.route.sessionId = created.id;
    created._worktreePath = state.route.worktreePath;
    created._checkoutProjectId = created.project_id || state.project.project_id;
    created._checkoutProjectRevision = created.project_revision || state.project.project_revision;
    state.activeSession = created;
    state.draft = emptyDraft();
    const prompt = title + (draft.spec.trim() ? '\n\nSpecification:\n' + draft.spec.trim() : '');
    if (mode === 'workflow') await startWorkflow({ message: prompt, autoRepair: draft.autoRepair });
    else if (mode === 'quick') await refine(prompt);
    else ui.toast('Draft saved.', 'success');
    await loadAll();
    return created;
  }

  // ---------------------------------------------------------------- issues

  async function triageFinding(finding, status) {
    if (!state.project || !finding) return;
    await guarded('Updating…', async () => {
      try {
        await api.patch(scoped('/api/projects/current/findings/' + encodeURIComponent(finding.id)), { project_revision: state.project.project_revision, status });
        finding.status = status;
        ui.toast(status === 'dismissed' ? 'Issue dismissed.' : 'Issue reopened.', 'success');
      } catch (err) {
        fail('Triage failed', err);
      }
    });
  }

  // ---------------------------------------------------------------- workspace

  async function runScan() {
    if (!state.project) return;
    if (!(state.trust && state.trust.trusted)) {
      ui.toast('The Go scan runs project code. Grant execution trust in Settings first.', 'warning');
      return;
    }
    await guarded('Starting scan…', async () => {
      try {
        state.scan = await api.post(scoped('/api/projects/current/scan'), { project_revision: state.project.project_revision });
        ui.toast('Go scan started in an isolated copy of the workspace.', 'info');
      } catch (err) {
        fail('Scan failed to start', err);
      }
    });
  }

  async function cancelScan() {
    if (!state.project) return;
    await guarded('Canceling scan…', async () => {
      try {
        state.scan = await api.del(scoped('/api/projects/current/scan' + query({ project_revision: state.project.project_revision })));
        ui.toast('Scan canceled.', 'info');
      } catch (err) {
        fail('Cancel failed', err);
      }
    });
  }

  async function reindex() {
    if (!state.project) return;
    await guarded('Reindexing…', async () => {
      try {
        await api.post(scoped('/api/projects/current/reindex'), { project_revision: state.project.project_revision });
        ui.toast('Project files reindexed.', 'success');
        await loadAll();
      } catch (err) {
        fail('Reindex failed', err);
      }
    });
  }

  async function toggleTrust() {
    if (!state.project) return;
    const next = !(state.trust && state.trust.trusted);
    const commands = asArray(state.trust && state.trust.commands).map(argv => asArray(argv).join(' '));
    const ok = await ui.confirm({
      title: next ? 'Trust this project to run checks?' : 'Revoke execution trust?',
      body: next ? 'Checks and scans will run these commands in an isolated copy of the workspace:\n• ' + (commands.join('\n• ') || 'project check commands') : 'Checks, scans and the agent workflow will stop running project code.',
      confirmLabel: next ? 'Grant trust' : 'Revoke trust',
      danger: !next
    });
    if (!ok) return;
    await guarded('Saving…', async () => {
      try {
        state.trust = await api.post(scoped('/api/projects/current/execution-trust'), { project_revision: state.project.project_revision, confirm: next });
        ui.toast(next ? 'Execution trust granted.' : 'Execution trust revoked.', 'success');
      } catch (err) {
        fail('Could not update trust', err);
      }
    });
  }

  async function runAnalysis() {
    if (!state.project) return;
    await guarded('Preparing analysis…', async () => {
      try {
        const limits = { batch_files: 64, budget_seconds: 300, max_attempts_per_stage: 2 };
        const preview = await api.post(scoped('/api/projects/current/analysis/preview'), {
          project_id: state.project.project_id,
          project_revision: state.project.project_revision,
          scope: 'project',
          refresh: false,
          limits
        });
        const providers = asArray(preview && preview.providers);
        const remote = providers.filter(p => p.remote_confirmation_required).map(p => (p.model && (p.model.model || p.model.scope)) + ' at ' + ((p.model && p.model.provider_origin) || 'remote provider'));
        const files = asArray(preview && preview.files).length;
        const security = Boolean(preview && preview.security_review_intent_required);
        const ok = await ui.confirm({
          title: 'Run project analysis?',
          body: 'Analyzes ' + plural(files, 'file') + ' (up to ' + ((preview && preview.max_model_requests) || 0) + ' model requests) for bugs, performance and security, then refreshes the generated docs.' +
            (security ? '\n\nIncludes an AI security review of eligible files.' : '') +
            (remote.length ? '\n\nRemote providers:\n• ' + remote.join('\n• ') + '\nBounded source context leaves your machine.' : ''),
          confirmLabel: 'Run analysis'
        });
        if (!ok) return;
        await api.post(scoped('/api/projects/current/analysis/run'), {
          identity: preview.identity,
          preview_id: preview.preview_id,
          limits,
          refresh: false,
          confirmations: { provider_ids: providers.map(p => p.id).filter(Boolean), security_review: security }
        });
        ui.toast('Analysis started. Progress appears on Checks.', 'info');
        await loadAll();
      } catch (err) {
        fail('Analysis could not start', err);
      }
    });
  }

  async function controlAnalysis(action) {
    const run = state.overview && state.overview.analysis_run;
    if (!run || !run.identity || !['pause', 'cancel'].includes(action)) return;
    if (action === 'cancel') {
      const ok = await ui.confirm({ title: 'Cancel the analysis run?', body: 'Completed files keep their results; remaining files are not analyzed.', confirmLabel: 'Cancel run', cancelLabel: 'Keep running', danger: true });
      if (!ok) return;
    }
    await guarded(action === 'pause' ? 'Pausing…' : 'Canceling…', async () => {
      try {
        await api.post(scoped('/api/projects/current/analysis/run/control'), { identity: run.identity, action });
        ui.toast(action === 'pause' ? 'Analysis paused. Resume it from the TUI.' : 'Analysis canceled.', 'info');
        await loadAll();
      } catch (err) {
        fail('Analysis control failed', err);
      }
    });
  }

  async function openFile(path) {
    if (!path || !state.project) return;
    const explorer = state.explorer;
    state.route.file = path;
    if (!explorer.tabs.includes(path)) explorer.tabs = explorer.tabs.concat(path).slice(-8);
    explorer.loadingPath = path;
    explorer.explanation = null;
    changed();
    const q = query({ path });
    const [info, analysis] = await Promise.allSettled([
      api.get(scoped('/api/projects/current/files/info' + q)),
      api.get(scoped('/api/projects/current/files/analysis' + query({ path, project_revision: state.project.project_revision })))
    ]);
    if (state.route.file !== path) return;
    explorer.loadingPath = '';
    if (info.status !== 'fulfilled' || !info.value) {
      explorer.info = null;
      changed();
      fail('Could not open ' + path, info.reason || new Error('file unavailable'));
      return;
    }
    explorer.info = info.value;
    explorer.analysis = analysis.status === 'fulfilled' && analysis.value && analysis.value.status === 'fresh' ? analysis.value : null;
    changed();
  }

  function closeTab(path) {
    const explorer = state.explorer;
    const index = explorer.tabs.indexOf(path);
    if (index < 0) return '';
    explorer.tabs = explorer.tabs.filter(tab => tab !== path);
    if (state.route.file !== path) { changed(); return state.route.file; }
    const next = explorer.tabs[Math.min(index, explorer.tabs.length - 1)] || '';
    state.route.file = next;
    explorer.info = null;
    changed();
    return next;
  }

  async function explainFile() {
    const info = state.explorer.info;
    if (!info || !state.project) return;
    const isGo = info.path.endsWith('.go');
    const scope = isGo ? 'code' : 'bug';
    const consent = await confirmRemote(scope, 'Explaining ' + info.path);
    if (!consent.ok) return;
    await guarded('Explaining…', async () => {
      try {
        if (isGo && info.content_hash) {
          const explanation = await api.post(scoped('/api/projects/current/files/explanation'), { project_id: state.project.project_id, project_revision: state.project.project_revision, base_file_hash: info.content_hash, target_path: info.path, confirm_remote_provider: consent.remote });
          if (state.explorer.info === info) state.explorer.explanation = explanation;
        } else {
          const analysis = await api.post(scoped('/api/projects/current/files/analysis'), { path: info.path, project_revision: state.project.project_revision, refresh: true, confirm_remote_provider: consent.remote });
          if (state.explorer.info === info) state.explorer.analysis = analysis;
        }
        ui.toast('Explanation ready.', 'success');
      } catch (err) {
        fail('Explanation failed', err);
      }
    });
  }

  async function switchProject() {
    const current = state.project ? state.project.path : '';
    const next = await ui.prompt({ title: 'Open another project', label: 'Absolute path of a local project', value: current, confirmLabel: 'Open project' });
    const path = next ? next.trim() : '';
    if (!path || path === current) return;
    await guarded('Opening project…', async () => {
      try {
        try {
          await api.post('/api/projects/restore', { project_path: path });
        } catch (_) {
          await api.post('/api/projects/import', { project_path: path });
        }
        Object.assign(state.route, { worktreePath: '', sessionId: '', file: '' });
        state.explorer.tabs = [];
        state.activeSession = null;
        ui.toast('Opened ' + path, 'success');
        await loadAll();
      } catch (err) {
        fail('Could not open project', err);
      }
    });
  }

  async function setCheckout(worktreePath) {
    if (state.pending) return;
    state.sessionRequest++;
    state.activeSession = null;
    Object.assign(state.route, { worktreePath: worktreePath || '', sessionId: '', file: '' });
    state.explorer.tabs = [];
    state.explorer.info = null;
    changed();
    await loadAll();
  }

  // ---------------------------------------------------------------- git

  function findWorktree(path) {
    return asArray(state.gitWorkspace && state.gitWorkspace.worktrees).find(tree => tree.path === path) || null;
  }

  async function previewMerge(path) {
    const worktree = findWorktree(path);
    if (!worktree || !state.gitWorkspace || !state.mainProject) return;
    state.merge = { path, preview: null, files: [], loading: true };
    changed();
    try {
      const preview = await api.post('/api/projects/current/git/merge-preview', mergeRequest(state.gitWorkspace, worktree, state.mainProject));
      if (state.merge.path !== path) return;
      state.merge = { path, preview, files: parseGitDiff(preview && preview.diff), loading: false };
    } catch (err) {
      if (state.merge.path === path) state.merge = { path, preview: null, files: [], loading: false };
      fail('Merge preview failed', err);
    }
    changed();
  }

  async function mergeWorktree(path) {
    const worktree = findWorktree(path);
    if (!worktree || !state.gitWorkspace || !state.mainProject) return;
    if (!(state.trust && state.trust.trusted)) {
      ui.toast('Merging can run repository filters and merge drivers. Grant execution trust first.', 'warning');
      return;
    }
    const ok = await ui.confirm({
      title: 'Merge ' + (worktree.branch || 'worktree') + ' into ' + (state.gitWorkspace.branch || 'the current branch') + '?',
      body: 'Merges the reviewed worktree HEAD ' + String(worktree.head || '').slice(0, 7) + ', then removes the merged worktree and its branch. Conflicts stop the merge so you can resolve or abort it.' + (asArray(worktree.overlap_paths).length ? '\n\nOverlapping paths: ' + worktree.overlap_paths.join(', ') : ''),
      confirmLabel: 'Merge'
    });
    if (!ok) return;
    await guarded('Merging…', async () => {
      try {
        const result = await api.post('/api/projects/current/git/merge', mergeRequest(state.gitWorkspace, worktree, state.mainProject));
        if (result && result.state === 'conflicts') ui.toast('Merge stopped with conflicts. Resolve them in your editor or abort the merge.', 'warning');
        else ui.toast('Merged ' + (worktree.branch || path) + '.', 'success');
        state.merge = { path: '', preview: null, files: [], loading: false };
        await loadAll();
      } catch (err) {
        fail('Merge failed', err);
      }
    });
  }

  async function abortMerge() {
    const ws = state.gitWorkspace;
    if (!ws || !ws.merge_head || !state.mainProject) return;
    const ok = await ui.confirm({ title: 'Abort the merge?', body: 'Discards conflict resolutions and restores the branch to its state before the merge.', confirmLabel: 'Abort merge', danger: true });
    if (!ok) return;
    await guarded('Aborting merge…', async () => {
      try {
        await api.post('/api/projects/current/git/abort-merge', { project_id: state.mainProject.project_id, project_revision: state.mainProject.project_revision, head: ws.head, branch: ws.branch, merge_head: ws.merge_head, confirm: true, confirm_execution: true });
        ui.toast('Merge aborted.', 'success');
        await loadAll();
      } catch (err) {
        fail('Abort failed', err);
      }
    });
  }

  return {
    loadAll, loadSession, reviewFile, approveReview, runChecks, applyProposal, undoProposal, refine,
    startWorkflow, cancelWorkflow, respondWorkflowInput,
    setDraftStep, toggleDraftPath, draftFromFinding, draftFromFile, createTask,
    triageFinding, runScan, cancelScan, reindex, toggleTrust, runAnalysis, controlAnalysis,
    openFile, closeTab, explainFile, switchProject, setCheckout,
    previewMerge, mergeWorktree, abortMerge, guarded
  };
}

/** Lists main-checkout tasks plus tasks living in other worktrees. */
export function mergeWorktreeSessions(base, workspace, activeWorktree) {
  const out = [];
  const seen = new Set();
  for (const entry of base) {
    if (!entry || !entry.id || seen.has(entry.id)) continue;
    seen.add(entry.id);
    out.push(Object.assign({}, entry, activeWorktree && !entry._worktreePath ? { _worktreePath: activeWorktree } : {}));
  }
  if (!activeWorktree && workspace) {
    for (const tree of asArray(workspace.worktrees)) {
      if (tree.current || !tree.path) continue;
      for (const entry of asArray(tree.sessions)) {
        if (!entry || !entry.id || seen.has(entry.id)) continue;
        seen.add(entry.id);
        out.push(Object.assign({}, entry, { _worktreePath: tree.path, _branch: tree.branch || '' }));
      }
    }
  }
  return out;
}
