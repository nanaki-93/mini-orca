import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';

const html = readFileSync(new URL('./webui.html', import.meta.url), 'utf8');
const source = html.split('<script>')[1].split('</script>')[0];

// Run the shipped script with a minimal DOM, replacing only startup I/O.
// Browser checks cover layout and native focus; these cases cover action boundaries.
function harness() {
  const elements = new Map();
  const requests = [];
  let activeElement;
  class Element {
    constructor(id, attributes = {}) {
      this.id = id;
      this.attributes = attributes;
      this.value = '';
      this.textContent = id;
      this.style = {};
      this.listeners = {};
      this.disabled = false;
      this.isConnected = true;
      const classes = new Set((attributes.class || '').split(' '));
      this.classList = {
        add: value => classes.add(value), remove: value => classes.delete(value),
        contains: value => classes.has(value),
        toggle(value, on = !classes.has(value)) { if (on) classes.add(value); else classes.delete(value); return on; }
      };
    }
    setAttribute(key, value) { this.attributes[key] = value; }
    removeAttribute(key) { delete this.attributes[key]; }
    getAttribute(key) { return this.attributes[key] ?? null; }
    addEventListener(name, fn) { this.listeners[name] = fn; }
    focus() { activeElement = this; }
    scrollIntoView() {}
    querySelectorAll() { return []; }
    click() { if (!this.disabled) this.listeners.click?.({ target: this }); }
  }
  for (const match of html.split('<script>')[0].matchAll(/<[^>]*\bid="([^"]+)"[^>]*>/g)) {
    const attributes = Object.fromEntries([...match[0].matchAll(/([\w-]+)="([^"]*)"/g)].map(m => [m[1], m[2]]));
    elements.set(match[1], new Element(match[1], attributes));
  }
  const document = {
    getElementById: id => elements.get(id) || null,
    get activeElement() { return activeElement; },
    documentElement: new Element('html'),
    addEventListener() {},
    querySelectorAll(selector) {
      if (selector.startsWith('[data-mutation]')) return [...elements.values()].filter(el => 'data-mutation' in el.attributes);
      return [];
    }
  };
  const startup = '  setTab(state.tab);\n  loadAll();\n  connectSSE();';
  assert.ok(source.includes(startup), 'startup instrumentation must match');
  const script = source.replace(startup, `
    loadAll = async function() {};
    globalThis.testUI = { state, proposalReadiness, applyActiveProposal, approveProposalReview,
      runAction, renderReviewActions, loadChangeSession, submitSpecTask, effectiveEdits, reviewSingleFile,
      openCommandPalette, closeCommandPalette, selectCommand, setTab, normalizePage };
  `);
  const context = {
    document, URLSearchParams,
    window: { location: { search: '' }, history: { replaceState() {} }, matchMedia: () => ({ matches: false }), confirm: () => true },
    localStorage: { getItem: () => null, setItem() {} },
    MutationObserver: class { observe() {} },
    setTimeout,
    fetch: async (path, options) => {
      requests.push({ path, options });
      return { ok: true, status: 200, text: async () => JSON.stringify({ state: 'applied' }) };
    }
  };
  runInNewContext(script, context, { filename: 'webui.html' });
  return { ...context.testUI, elements, requests, context };
}

function readySession(overrides = {}) {
  return { id: 'change-1', project_id: 'project-1', project_revision: 'revision-1', revision: 2,
    hash: 'hash-1', state: 'reviewed', freshness: 'current', changes: [{ path: 'main.go' }],
    reviewed_files: ['main.go'], file_review_hash: 'hash-1', reviewed_hash: 'hash-1',
    checks: [{ name: 'tests', required: true, state: 'passed' }], ...overrides };
}

test('Apply is unavailable for absent, stale, unreviewed or unchecked proposals', () => {
  const ui = harness();
  for (const session of [null, readySession({ freshness: 'stale' }), readySession({ freshness: undefined }),
    readySession({ changes: [] }), readySession({ reviewed_files: [] }), readySession({ file_review_hash: 'old' }),
    readySession({ reviewed_hash: 'old' }), readySession({ checks: [] }),
    readySession({ checks: [{ required: true, state: 'failed' }] }),
    readySession({ checks: [{ required: true, state: 'canceled' }] }),
    readySession({ workflow: { status: 'running' } }), readySession({ state: 'applied' })]) {
    assert.equal(ui.proposalReadiness(session).canApply, false, JSON.stringify(session));
  }
  assert.equal(ui.proposalReadiness(readySession()).canApply, true);
  assert.equal(ui.proposalReadiness(readySession({ checks: [{ required: false, state: 'skipped' }] })).canApply, true);
});

test('comparison view and pinned tests do not change full-proposal review authority', () => {
  const ui = harness();
  ui.state.sinceLastReview = true;
  ui.state.selectedFile = 'main.go';
  const session = readySession({ changes: [{ path: 'main.go' }, { path: 'other.go' }], review_diff: [{ path: 'main.go' }] });
  assert.equal(ui.proposalReadiness(session).canApply, false);
  assert.equal(ui.proposalReadiness(readySession({ pinned_tests: [{ path: 'main_test.go' }] })).canApply, true);
});

test('Apply sends only one explicit apply request with captured identity', async () => {
  const ui = harness();
  ui.state.activeSession = readySession({ _worktreePath: '/tmp/task checkout' });
  await ui.applyActiveProposal();
  assert.equal(ui.requests.length, 1);
  assert.match(ui.requests[0].path, /\/apply\?worktree_path=%2Ftmp%2Ftask%20checkout$/);
  assert.deepEqual(JSON.parse(ui.requests[0].options.body), {
    project_id: 'project-1', project_revision: 'revision-1', revision: 2, hash: 'hash-1', confirm: true
  });
});

test('Apply and approval never mark unreviewed files implicitly', async () => {
  const ui = harness();
  ui.state.activeSession = readySession({ reviewed_files: [] });
  await ui.applyActiveProposal();
  await ui.approveProposalReview();
  assert.equal(ui.requests.length, 0);
  ui.state.activeSession = readySession({ reviewed_hash: '' });
  await ui.approveProposalReview();
  assert.equal(ui.requests.length, 1);
  assert.match(ui.requests[0].path, /\/review$/);
});

test('canceling Apply sends no request', async () => {
  const ui = harness();
  ui.state.activeSession = readySession();
  ui.context.window.confirm = () => false;
  await ui.applyActiveProposal();
  assert.equal(ui.requests.length, 0);
});

test('pending actions prevent duplicate submissions and restore controls after failure', async () => {
  const ui = harness();
  const button = ui.elements.get('generate-spec-btn');
  let release;
  let calls = 0;
  const action = () => { calls++; return new Promise(resolve => { release = resolve; }); };
  const pending = ui.runAction(button, action, 'Generating…');
  assert.equal(button.disabled, true);
  await ui.runAction(button, action);
  assert.equal(calls, 1);
  release();
  await pending;
  assert.equal(button.disabled, false);
  await assert.rejects(ui.runAction(button, async () => { throw new Error('failure'); }), /failure/);
  assert.equal(button.disabled, false);
  assert.equal(ui.state.pendingAction, false);
});

test('unavailable proposal selection clears previous action authority', async () => {
  const ui = harness();
  ui.state.project = { project_id: 'project-1', project_revision: 'revision-1' };
  ui.state.activeSession = readySession();
  ui.context.fetch = async () => { throw new Error('Unavailable'); };
  await ui.loadChangeSession('missing');
  assert.equal(ui.state.activeSession, null);
  assert.equal(ui.elements.get('apply-btn').disabled, true);
});

test('over-limit task scope is rejected before any request', async () => {
  const ui = harness();
  ui.state.project = { project_id: 'project-1' };
  ui.elements.get('new-task-title').value = 'Fix handler';
  ui.elements.get('new-task-paths').value = Array.from({ length: 9 }, (_, i) => `file${i}.go`).join(',');
  await ui.submitSpecTask(true);
  assert.equal(ui.requests.length, 0);
  assert.match(ui.elements.get('status-banner-text').textContent, /1–8/);
});

test('command palette supports arrow selection, file search, Escape and focus restoration', () => {
  const ui = harness();
  const trigger = ui.elements.get('cmd-palette-btn');
  const input = ui.elements.get('cmd-input');
  trigger.focus();
  ui.openCommandPalette();
  assert.equal(ui.elements.get('app-shell').inert, true);
  input.listeners.keydown({ key: 'ArrowDown', preventDefault() {} });
  assert.equal(input.getAttribute('aria-activedescendant'), 'command-1');
  ui.state.index = { files: [{ path: 'internal/server.go' }] };
  input.value = 'server.go';
  input.listeners.input();
  assert.match(ui.elements.get('cmd-list').innerHTML, /Open file: internal\/server.go/);
  input.listeners.keydown({ key: 'Escape', preventDefault() {} });
  assert.equal(ui.elements.get('app-shell').inert, false);
  assert.equal(ui.context.document.activeElement, trigger);
});


test('empty comparison stays empty and cannot mark full files reviewed', async () => {
  const ui = harness();
  ui.state.sinceLastReview = true;
  ui.state.activeSession = readySession({ review_diff: [], reviewed_files: [] });
  assert.equal(ui.effectiveEdits(ui.state.activeSession).length, 0);
  await ui.reviewSingleFile('main.go');
  assert.equal(ui.requests.length, 0);
});

test('a late proposal response cannot replace the latest selection', async () => {
  const ui = harness();
  ui.state.project = { project_id: 'project-1', project_revision: 'revision-1' };
  ui.state.mainProject = ui.state.project;
  const responses = [];
  ui.context.fetch = () => new Promise(resolve => responses.push(resolve));
  const first = ui.loadChangeSession('first');
  await Promise.resolve();
  const second = ui.loadChangeSession('second');
  await Promise.resolve();
  responses[1]({ ok: true, status: 200, text: async () => JSON.stringify(readySession({ id: 'second' })) });
  await second;
  responses[0]({ ok: true, status: 200, text: async () => JSON.stringify(readySession({ id: 'first' })) });
  await first;
  assert.equal(ui.state.activeSession.id, 'second');
});

// Page changes are passive: selecting a section never runs checks or a model.
test('focused pages hide unrelated content and preserve legacy TUI routes', () => {
  const ui = harness();
  const pages = ['home', 'files', 'tasks', 'new-task', 'issues', 'docs', 'checks', 'settings', 'diff'];
  for (const page of pages) {
    ui.setTab(page);
    assert.equal(ui.state.tab, page);
    for (const other of pages) assert.equal(ui.elements.get('view-' + other).hidden, page !== other);
  }
  ui.setTab('understand');
  assert.equal(ui.state.tab, 'docs');
  ui.setTab('build');
  assert.equal(ui.state.tab, 'new-task');
  ui.setTab('unknown');
  assert.equal(ui.state.tab, 'home');
  assert.equal(ui.requests.length, 0);
});
