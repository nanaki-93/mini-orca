// Action boundaries: what each operation is allowed to send, and when it must
// send nothing at all. Uses a fake daemon and a scripted consent port.

import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createApi } from '../js/app/api.js';
import { createActions, createInitialState, mergeWorktreeSessions } from '../js/app/actions.js';
import { readySession } from './fixtures.mjs';

function harness({ confirm = true, responses = {} } = {}) {
  const requests = [];
  const toasts = [];
  const confirms = [];
  const state = createInitialState({ tab: 'diff', sessionId: '', file: '', worktreePath: '', docSection: 'overview', reviewSection: 'changes' });
  state.project = { project_id: 'project-1', project_revision: 'revision-1', path: '/repo' };
  state.mainProject = state.project;
  state.models = { scopes: {} };
  const fetchImpl = async (path, options) => {
    requests.push({ path, method: options.method, body: options.body ? JSON.parse(options.body) : undefined, headers: options.headers });
    const match = Object.keys(responses).find(key => path.includes(key));
    const value = match ? responses[match] : { state: 'applied' };
    if (value instanceof Error) return { ok: false, status: 409, text: async () => JSON.stringify({ error: value.message }) };
    return { ok: true, status: 200, text: async () => JSON.stringify(typeof value === 'function' ? value() : value) };
  };
  const ui = {
    confirm: async options => { confirms.push(options); return typeof confirm === 'function' ? confirm(options) : confirm; },
    prompt: async () => null,
    toast: (message, tone) => toasts.push({ message, tone })
  };
  const actions = createActions({ state, api: createApi(fetchImpl), ui, notify: () => {} });
  return { state, actions, requests, toasts, confirms };
}

test('every request carries the Web UI marker header', async () => {
  const h = harness();
  h.state.activeSession = readySession();
  await h.actions.applyProposal();
  assert.equal(h.requests[0].headers['X-Mini-Orca-Web-UI'], '1');
  assert.equal(h.requests[0].headers['Content-Type'], 'application/json');
});

test('Apply sends exactly one request with the captured identity and worktree scope', async () => {
  const h = harness({ responses: { '/apply': { state: 'applied' }, '/api/projects/current': { project_id: 'project-1', project_revision: 'revision-2' } } });
  h.state.activeSession = readySession({ _worktreePath: '/tmp/task checkout' });
  h.state.route.worktreePath = '';
  await h.actions.applyProposal();
  const apply = h.requests.filter(r => r.path.includes('/apply'));
  assert.equal(apply.length, 1);
  assert.match(apply[0].path, /\/changes\/change-1\/apply\?worktree_path=%2Ftmp%2Ftask%20checkout$/);
  assert.deepEqual(apply[0].body, { project_id: 'project-1', project_revision: 'revision-1', revision: 2, hash: 'hash-1', confirm: true });
});

test('canceling Apply, Undo or trust consent sends no request', async () => {
  const h = harness({ confirm: false });
  h.state.activeSession = readySession();
  await h.actions.applyProposal();
  h.state.activeSession = readySession({ state: 'applied' });
  await h.actions.undoProposal();
  await h.actions.toggleTrust();
  assert.equal(h.requests.length, 0);
  assert.equal(h.confirms.length, 3);
});

test('Apply and approval never mark unreviewed files implicitly', async () => {
  const h = harness();
  h.state.activeSession = readySession({ reviewed_files: [] });
  await h.actions.applyProposal();
  await h.actions.approveReview();
  assert.equal(h.requests.length, 0);
  h.state.activeSession = readySession({ reviewed_hash: '' });
  await h.actions.approveReview();
  assert.equal(h.requests.length, 1);
  assert.match(h.requests[0].path, /\/review$/);
});

test('Apply & commit binds the proposal Git identity and requires a message', async () => {
  const h = harness();
  h.state.activeSession = readySession({ git: { head: 'abc', branch: 'main' } });
  await h.actions.applyProposal({ commit: true, message: '   ' });
  assert.equal(h.requests.length, 0);
  assert.equal(h.toasts.at(-1).tone, 'warning');
  await h.actions.applyProposal({ commit: true, message: 'feat: validate input' });
  const apply = h.requests.find(r => r.path.includes('/apply'));
  assert.deepEqual(apply.body, { project_id: 'project-1', project_revision: 'revision-1', revision: 2, hash: 'hash-1', confirm: true, head: 'abc', branch: 'main', commit_message: 'feat: validate input' });
});

test('pending mutations prevent duplicate submissions and always release', async () => {
  const h = harness();
  let release;
  let calls = 0;
  const pending = h.actions.guarded('Working…', () => { calls++; return new Promise(resolve => { release = resolve; }); });
  assert.equal(h.state.pending, 'Working…');
  await h.actions.guarded('Again', async () => { calls++; });
  assert.equal(calls, 1);
  release();
  await pending;
  assert.equal(h.state.pending, '');
  await assert.rejects(h.actions.guarded('Fail', async () => { throw new Error('failure'); }), /failure/);
  assert.equal(h.state.pending, '');
});

test('reviewing a file records only that path and advances to the next unreviewed file', async () => {
  const updated = readySession({ changes: [{ path: 'a.go' }, { path: 'b.go' }], reviewed_files: ['a.go'], reviewed_hash: '' });
  const h = harness({ responses: { '/review-file': updated } });
  h.state.activeSession = readySession({ changes: [{ path: 'a.go' }, { path: 'b.go' }], reviewed_files: [], reviewed_hash: '' });
  await h.actions.reviewFile('a.go');
  assert.equal(h.requests.length, 1);
  assert.equal(h.requests[0].body.path, 'a.go');
  assert.equal(h.state.route.file, 'b.go');
  await h.actions.reviewFile('a.go');
  assert.equal(h.requests.length, 1, 'already reviewed files are not re-sent');
  await h.actions.reviewFile('not-in-proposal.go');
  assert.equal(h.requests.length, 1);
});

test('the since-review comparison cannot mark files reviewed', async () => {
  const h = harness();
  h.state.review.sinceLastReview = true;
  h.state.activeSession = readySession({ review_diff: [], reviewed_files: [] });
  await h.actions.reviewFile('main.go');
  assert.equal(h.requests.length, 0);
});

test('a late proposal response cannot replace the latest selection', async () => {
  const h = harness();
  const pendingResponses = [];
  h.state.changes = [{ id: 'first' }, { id: 'second' }];
  const api = createApi(() => new Promise(resolve => pendingResponses.push(resolve)));
  const actions = createActions({ state: h.state, api, ui: { confirm: async () => true, toast() {} }, notify() {} });
  const first = actions.loadSession('first');
  await Promise.resolve();
  const second = actions.loadSession('second');
  await Promise.resolve();
  pendingResponses[1]({ ok: true, status: 200, text: async () => JSON.stringify(readySession({ id: 'second' })) });
  await second;
  pendingResponses[0]({ ok: true, status: 200, text: async () => JSON.stringify(readySession({ id: 'first' })) });
  await first;
  assert.equal(h.state.activeSession.id, 'second');
});

test('a failed selection clears the previous proposal and its authority', async () => {
  const h = harness({ responses: { '/changes/missing': new Error('Unavailable') } });
  h.state.activeSession = readySession();
  await h.actions.loadSession('missing');
  assert.equal(h.state.activeSession, null);
  assert.equal(h.toasts.at(-1).tone, 'danger');
});

test('task scope validation rejects empty and over-limit scopes before any request', async () => {
  const h = harness();
  h.state.draft.title = 'Fix handler';
  h.state.draft.paths = Array.from({ length: 9 }, (_, i) => 'file' + i + '.go');
  assert.equal(await h.actions.createTask(true), false);
  assert.equal(h.requests.length, 0);
  assert.equal(h.state.draft.step, 2);
  assert.match(h.state.draft.error, /1 and 8/);
  h.state.draft.paths = ['main.go'];
  h.state.draft.mode = 'workflow';
  assert.equal(await h.actions.createTask(true), false, 'workflow needs a test path');
  assert.equal(h.requests.length, 0);
  assert.equal(h.actions.setDraftStep(3), true);
  h.state.draft.title = '';
  assert.equal(h.actions.setDraftStep(2), false);
  assert.equal(h.state.draft.errorField, 'title');
});

test('selecting a Go file adds its existing companion test, capped at eight paths', () => {
  const h = harness();
  h.state.index = { files: [{ path: 'a.go' }, { path: 'a_test.go' }, { path: 'b.go' }] };
  h.actions.toggleDraftPath('a.go');
  assert.deepEqual(h.state.draft.paths, ['a.go', 'a_test.go']);
  h.actions.toggleDraftPath('b.go');
  assert.deepEqual(h.state.draft.paths, ['a.go', 'a_test.go', 'b.go']);
  h.actions.toggleDraftPath('a.go');
  assert.deepEqual(h.state.draft.paths, ['a_test.go', 'b.go']);
});

test('starting the agent workflow requires consent and sends only confirmed remote profiles', async () => {
  const session = readySession({ targets: [{ path: 'main.go' }, { path: 'main_test.go' }], changes: [] });
  const h = harness({ responses: { '/agent-settings': { roles: { creation: 'bug', testing: 'code', review: 'analyze' } }, '/workflow': () => readySession({ workflow: { id: 'wf-1', status: 'running' } }) } });
  h.state.models = { scopes: { code: { model: 'remote-model', remote_provider: true, provider_origin: 'https://x' } } };
  h.state.activeSession = session;
  await h.actions.startWorkflow({ message: 'Build it' });
  const start = h.requests.find(r => r.path.includes('/workflow'));
  assert.ok(start);
  assert.deepEqual(start.body.models, { create: 'bug', test: 'code', review: 'analyze' });
  assert.deepEqual(start.body.confirmed_profiles, ['code']);
  assert.match(h.confirms[0].body, /remote-model/);
  assert.equal(h.state.activeSession.workflow.id, 'wf-1');

  const declined = harness({ confirm: false, responses: { '/agent-settings': {} } });
  declined.state.activeSession = session;
  await declined.actions.startWorkflow({ message: 'Build it' });
  assert.equal(declined.requests.filter(r => r.method === 'POST').length, 0);
});

test('workflow approval requires a complete review and an explicit decision', async () => {
  const waiting = { id: 'wf-1', status: 'awaiting_human_review', input: { request_id: 'req-1', status: 'waiting' } };
  const h = harness();
  h.state.activeSession = readySession({ workflow: waiting, reviewed_files: [] });
  await h.actions.respondWorkflowInput('approve');
  assert.equal(h.requests.length, 0);
  h.state.activeSession = readySession({ workflow: waiting });
  await h.actions.respondWorkflowInput('approve');
  assert.equal(h.requests.length, 1);
  assert.deepEqual({ decision: h.requests[0].body.decision, request_id: h.requests[0].body.request_id, workflow_id: h.requests[0].body.workflow_id }, { decision: 'approve', request_id: 'req-1', workflow_id: 'wf-1' });
  await h.actions.respondWorkflowInput('cancel-everything');
  assert.equal(h.requests.length, 1);
});

test('remote refinement asks consent and declining sends nothing', async () => {
  const h = harness({ confirm: false });
  h.state.models = { scopes: { code: { model: 'm', remote_provider: true } } };
  h.state.activeSession = readySession();
  assert.equal(await h.actions.refine('Improve errors'), false);
  assert.equal(h.requests.length, 0);
  const ok = harness({ responses: { '/messages': readySession({ revision: 3 }) } });
  ok.state.models = { scopes: { code: { model: 'm', remote_provider: true } } };
  ok.state.activeSession = readySession();
  assert.equal(await ok.actions.refine('Improve errors'), true);
  assert.equal(ok.requests[0].body.confirm_remote_provider, true);
  assert.equal(ok.requests[0].body.message, 'Improve errors');
});

test('Go scan and merge require execution trust before any request', async () => {
  const h = harness();
  h.state.trust = { trusted: false };
  await h.actions.runScan();
  h.state.gitWorkspace = { head: 'h', branch: 'main', worktrees: [{ path: '/wt', head: 'w', branch: 'task' }] };
  await h.actions.mergeWorktree('/wt');
  assert.equal(h.requests.length, 0);
  assert.equal(h.toasts.length, 2);
});

test('merge preview binds identities and merge needs explicit confirmation', async () => {
  const h = harness({ confirm: false, responses: { 'merge-preview': { diff: 'diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-a\n+b\n', commits: 'abc Fix' } } });
  h.state.trust = { trusted: true };
  h.state.gitWorkspace = { head: 'h', branch: 'main', worktrees: [{ path: '/wt', head: 'w', branch: 'task' }] };
  await h.actions.previewMerge('/wt');
  assert.equal(h.requests.length, 1);
  assert.equal(h.requests[0].body.source_head, 'w');
  assert.equal(h.state.merge.files[0].path, 'a.go');
  await h.actions.mergeWorktree('/wt');
  assert.equal(h.requests.length, 1, 'declined merge sends nothing');
});

test('analysis start echoes the preview identity and every provider confirmation', async () => {
  const preview = { identity: { queue_id: 'q' }, preview_id: 'pv-1', files: [{}, {}], providers: [{ id: 'p1', remote_confirmation_required: true, model: { model: 'm', provider_origin: 'https://x' } }, { id: 'p2' }], security_review_intent_required: true, max_model_requests: 6 };
  const h = harness({ responses: { 'analysis/preview': preview, 'analysis/run': { status: 'running' } } });
  await h.actions.runAnalysis();
  const start = h.requests.find(r => r.path.includes('analysis/run'));
  assert.deepEqual(start.body.confirmations, { provider_ids: ['p1', 'p2'], security_review: true });
  assert.equal(start.body.preview_id, 'pv-1');
  assert.match(h.confirms[0].body, /m at https:\/\/x/);
  const declined = harness({ confirm: false, responses: { 'analysis/preview': preview } });
  await declined.actions.runAnalysis();
  assert.equal(declined.requests.filter(r => r.path.includes('analysis/run')).length, 0);
});

test('worktree tasks are listed with their checkout and never duplicated', () => {
  const merged = mergeWorktreeSessions([{ id: 'a' }], { worktrees: [{ current: true, sessions: [{ id: 'x' }] }, { path: '/wt', branch: 'task', sessions: [{ id: 'a' }, { id: 'b' }] }] }, '');
  assert.deepEqual(merged.map(m => [m.id, m._worktreePath || '']), [['a', ''], ['b', '/wt']]);
  assert.deepEqual(mergeWorktreeSessions([{ id: 'a' }], null, '/wt').map(m => m._worktreePath), ['/wt']);
});
