import assert from 'node:assert/strict';
import { test } from 'node:test';
import { checkSummary, effectiveEdits, isFileReviewed, nextAction, nextUnreviewedPath, normalizePhaseState, proposalReadiness, reviewProgress } from '../js/core/readiness.js';
import { buildSegments, diffStats, hunkStarts, parseGitDiff, splitRows } from '../js/core/diff.js';
import { fuzzyScore, highlight, rankItems } from '../js/core/fuzzy.js';
import { ancestors, buildTree, visibleRows } from '../js/core/tree.js';
import { buildSearch, normalizePage, parseRoute } from '../js/core/route.js';
import { companionTestPath, esc, isTestPath, timeAgo } from '../js/core/util.js';
import { readySession } from './fixtures.mjs';
import { mergeRequest, parseCommits, workflowBlocker, workflowDestinations, workflowRequest } from '../js/core/workflow.js';


test('Apply is unavailable for absent, stale, unreviewed, unchecked or busy proposals', () => {
  for (const session of [null, readySession({ freshness: 'stale' }), readySession({ freshness: undefined }),
    readySession({ changes: [] }), readySession({ reviewed_files: [] }), readySession({ file_review_hash: 'old' }),
    readySession({ reviewed_hash: 'old' }), readySession({ checks: [] }),
    readySession({ checks: [{ required: true, state: 'failed' }] }),
    readySession({ checks: [{ required: true, state: 'canceled' }] }),
    readySession({ workflow: { status: 'running' } }), readySession({ state: 'applied' })]) {
    assert.equal(proposalReadiness(session).canApply, false, JSON.stringify(session));
  }
  assert.equal(proposalReadiness(readySession()).canApply, true);
  assert.equal(proposalReadiness(readySession({ checks: [{ required: false, state: 'skipped' }] })).canApply, true);
  assert.equal(proposalReadiness(readySession({ workflow: { status: 'awaiting_human_review' } })).canApply, true);
});

test('pinned tests and the since-review comparison never change review authority', () => {
  const session = readySession({ changes: [{ path: 'main.go' }, { path: 'other.go' }], review_diff: [{ path: 'main.go' }] });
  assert.equal(proposalReadiness(session).canApply, false);
  assert.equal(proposalReadiness(readySession({ pinned_tests: [{ path: 'main_test.go' }] })).canApply, true);
  assert.deepEqual(effectiveEdits(readySession({ pinned_tests: [{ path: 'main_test.go' }, { path: 'main.go' }] })).map(e => e.path), ['main.go', 'main_test.go']);
  assert.deepEqual(effectiveEdits(readySession({ review_diff: [] }), true), []);
});

test('review marks only count at the current proposal hash', () => {
  assert.equal(isFileReviewed(readySession(), 'main.go'), true);
  assert.equal(isFileReviewed(readySession({ file_review_hash: 'stale' }), 'main.go'), false);
  assert.equal(isFileReviewed(readySession({ hash: '' }), 'main.go'), false);
  assert.deepEqual(reviewProgress(readySession({ changes: [{ path: 'a' }, { path: 'main.go' }] })), { reviewed: 1, total: 2, percent: 50 });
});

test('readiness steps guide the reviewer to the next required action', () => {
  assert.equal(proposalReadiness(readySession({ changes: [] })).step, 'spec');
  assert.equal(proposalReadiness(readySession({ changes: [], workflow: { status: 'running' } })).step, 'pipeline');
  assert.equal(proposalReadiness(readySession({ reviewed_files: [] })).step, 'changes');
  assert.equal(proposalReadiness(readySession({ checks: [] })).step, 'checks');
  assert.equal(proposalReadiness(readySession({ reviewed_hash: '' })).step, 'apply');
  assert.equal(proposalReadiness(readySession({ freshness: 'stale' })).step, 'stale');
});

test('next unreviewed navigation wraps and skips reviewed files', () => {
  const session = readySession({ changes: [{ path: 'a.go' }, { path: 'b.go' }, { path: 'c.go' }], reviewed_files: ['a.go', 'c.go'] });
  const edits = effectiveEdits(session);
  assert.equal(nextUnreviewedPath(session, edits, 'c.go'), 'b.go');
  assert.equal(nextUnreviewedPath(session, edits, ''), 'b.go');
  assert.equal(nextUnreviewedPath(readySession(), effectiveEdits(readySession()), 'main.go'), '');
});

test('phase states keep failed, canceled and unrun meanings distinct', () => {
  assert.equal(normalizePhaseState('passed'), 'passed');
  assert.equal(normalizePhaseState('changes_requested'), 'failed');
  assert.equal(normalizePhaseState('running'), 'running');
  assert.equal(normalizePhaseState('canceled'), 'idle');
  assert.equal(normalizePhaseState(''), 'idle');
  const summary = checkSummary(readySession({ checks: [{ state: 'passed' }, { state: 'failed', required: true }] }));
  assert.equal(summary.label, '1 failed');
  assert.equal(summary.tone, 'danger');
  assert.equal(checkSummary(readySession({ checks: [] })).label, 'Not run');
});

test('dashboard next action prioritizes conflicts, then the active proposal, then issues', () => {
  assert.equal(nextAction({ gitWorkspace: { conflicts: ['a.go'] }, activeSession: readySession() }).page, 'git');
  assert.equal(nextAction({ activeSession: readySession({ reviewed_files: [] }) }).section, 'changes');
  assert.equal(nextAction({ activeSession: readySession() }).section, 'apply');
  assert.equal(nextAction({ findings: [{ status: 'open' }, { status: 'dismissed' }] }).page, 'issues');
  assert.equal(nextAction({ findings: [{ status: 'dismissed' }] }).action, 'new-task');
});

test('diff segments collapse long unchanged runs and expand on request', () => {
  const lines = Array.from({ length: 30 }, (_, i) => ({ kind: 'context', old_line: i + 1, new_line: i + 1, text: 'l' + i }));
  lines[15] = { kind: 'added', new_line: 16, text: 'new' };
  const segments = buildSegments(lines, 3);
  assert.deepEqual(segments.map(s => s.type), ['gap', 'lines', 'gap']);
  assert.equal(segments[0].count, 12);
  assert.equal(segments[1].end - segments[1].start, 7);
  const expanded = buildSegments(lines, 3, new Set([0]));
  assert.deepEqual(expanded.map(s => s.type), ['lines', 'gap']);
  assert.deepEqual(buildSegments([{ kind: 'context', text: 'x' }], 3).map(s => s.type), ['gap']);
  assert.deepEqual(buildSegments([], 3), []);
});

test('diff stats, hunks and split rows pair removals with additions', () => {
  const lines = [{ kind: 'context', text: 'a' }, { kind: 'removed', text: 'b' }, { kind: 'removed', text: 'c' }, { kind: 'added', text: 'B' }, { kind: 'context', text: 'd' }, { kind: 'added', text: 'e' }];
  assert.deepEqual(diffStats({ diff: { lines } }), { added: 2, removed: 2 });
  assert.deepEqual(hunkStarts(lines), [1, 5]);
  const rows = splitRows(lines);
  assert.equal(rows.length, 5);
  assert.equal(rows[1].left.text, 'b');
  assert.equal(rows[1].right.text, 'B');
  assert.equal(rows[2].right, null);
});

test('git diff text parses into per-file numbered lines', () => {
  const files = parseGitDiff('diff --git a/x.go b/x.go\nindex 1..2 100644\n--- a/x.go\n+++ b/x.go\n@@ -1,2 +1,2 @@\n keep\n-old\n+new\ndiff --git a/img.png b/img.png\nBinary files a/img.png and b/img.png differ\n');
  assert.equal(files.length, 2);
  assert.equal(files[0].path, 'x.go');
  assert.deepEqual(files[0].lines.filter(l => l.kind !== 'hunk').map(l => [l.kind, l.old_line || 0, l.new_line || 0]), [['context', 1, 1], ['removed', 2, 0], ['added', 0, 2]]);
  assert.equal(files[1].binary, true);
  assert.deepEqual(parseCommits('abc123 Fix bug\n\ndef456 Add tests'), [{ sha: 'abc123', subject: 'Fix bug' }, { sha: 'def456', subject: 'Add tests' }]);
});

test('fuzzy ranking prefers contiguous basename matches and highlights safely', () => {
  const items = ['internal/api/handlers/webui_handler.go', 'internal/app/service.go', 'docs/api-contract.md'].map(path => ({ path }));
  const ranked = rankItems(items, 'service', item => item.path);
  assert.equal(ranked[0].item.path, 'internal/app/service.go');
  assert.equal(rankItems(items, 'zzz', item => item.path).length, 0);
  assert.ok(fuzzyScore('wbh', 'webui_handler.go'));
  assert.equal(fuzzyScore('xq', 'abc'), null);
  assert.equal(highlight('<a>', [1], esc), '&lt;<mark>a</mark>&gt;');
});

test('file tree compacts single-child folders and honors expansion', () => {
  const tree = buildTree([{ path: 'internal/api/a.go' }, { path: 'internal/api/b.go' }, { path: 'README.md' }]);
  const collapsed = visibleRows(tree, new Set());
  assert.deepEqual(collapsed.map(r => r.label), ['internal/api', 'README.md']);
  const open = visibleRows(tree, new Set(['internal/api']));
  assert.deepEqual(open.map(r => r.label), ['internal/api', 'a.go', 'b.go', 'README.md']);
  assert.deepEqual(ancestors('a/b/c.go'), ['a', 'a/b']);
});

test('routes keep TUI deep links and legacy tab names working', () => {
  assert.equal(parseRoute('?session_id=chg-1&file=main.go').tab, 'diff');
  assert.equal(parseRoute('?file=main.go').tab, 'files');
  assert.equal(parseRoute('?tab=understand').tab, 'docs');
  assert.equal(parseRoute('?tab=review&section=pipeline').reviewSection, 'pipeline');
  assert.equal(parseRoute('?tab=docs&section=bogus').docSection, 'overview');
  for (const unknown of ['unknown', 'constructor', 'toString', '__proto__']) assert.equal(normalizePage(unknown), 'home');
  const search = buildSearch({ tab: 'diff', reviewSection: 'apply', sessionId: 'chg 1', worktreePath: '/tmp/wt', file: 'a b.go' });
  assert.deepEqual(parseRoute(search), { tab: 'diff', sessionId: 'chg 1', file: 'a b.go', worktreePath: '/tmp/wt', docSection: 'overview', reviewSection: 'apply' });
});

test('helpers detect tests, companions and relative time', () => {
  assert.equal(isTestPath('a/b_test.go'), true);
  assert.equal(isTestPath('a/b.spec.ts'), true);
  assert.equal(isTestPath('tests/test_x.py'), true);
  assert.equal(isTestPath('main.go'), false);
  assert.equal(companionTestPath('a/b.go'), 'a/b_test.go');
  assert.equal(companionTestPath('a/b_test.go'), '');
  assert.equal(timeAgo(Date.now() - 5 * 60 * 1000), '5m ago');
  assert.equal(timeAgo('invalid'), '');
  assert.equal(esc('<"\'&>'), '&lt;&quot;&#39;&amp;&gt;');
});

test('workflow consent lists only remote destinations and blocks unsafe starts', () => {
  const models = { scopes: { bug: { model: 'local-coder', remote_provider: false }, code: { model: 'gpt', remote_provider: true, provider_origin: 'https://api.example' }, analyze: { model: 'gpt', remote_provider: true } } };
  const choice = workflowDestinations(null, models);
  assert.deepEqual(choice.selections, { create: 'bug', test: 'code', review: 'analyze' });
  assert.deepEqual(choice.confirmed, ['code', 'analyze']);
  assert.equal(choice.destinations.length, 2);
  const custom = workflowDestinations({ roles: { creation: 'pi:x/y', testing: 'code', review: 'code' }, models: [{ scope: 'pi:x/y', remote_provider: true, model: 'y' }] }, models);
  assert.deepEqual(custom.confirmed, ['pi:x/y', 'code']);
  const unknown = workflowDestinations({ roles: { creation: 'agy:mystery', testing: 'bug', review: 'bug' } }, models);
  assert.deepEqual(unknown.confirmed, [], 'unknown metadata never supplies consent');
  const body = workflowRequest({ project_id: 'p', project_revision: 'r', revision: 1, hash: 'h' }, 'Do it', choice, { autoRepair: true });
  assert.deepEqual(body.models, choice.selections);
  assert.equal(body.auto_repair, true);
  assert.equal(body.confirm_security, false);

  const session = readySession({ targets: [{ path: 'main.go' }, { path: 'main_test.go' }] });
  assert.equal(workflowBlocker(session, true), '');
  assert.match(workflowBlocker(session, false), /trust/);
  assert.match(workflowBlocker(readySession({ targets: [{ path: 'main.go' }] }), true), /test file/);
  assert.match(workflowBlocker(readySession({ targets: [{ path: 'x_test.go' }], workflow: { status: 'running' } }), true), /already running/);
});

test('merge requests carry both checkout identities and explicit confirmation', () => {
  const body = mergeRequest({ head: 'h1', branch: 'main' }, { path: '/wt', head: 'h2', branch: 'task' }, { project_id: 'p', project_revision: 'r' });
  assert.deepEqual(body, { project_id: 'p', project_revision: 'r', head: 'h1', branch: 'main', path: '/wt', source_head: 'h2', source_branch: 'task', confirm: true, confirm_execution: true });
});
