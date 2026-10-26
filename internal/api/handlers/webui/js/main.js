// Mini-Orca Web UI entry point: routing, rendering, events and shortcuts.

import { createApi } from './app/api.js';
import { createActions, createInitialState, emptyDraft } from './app/actions.js';
import { buildSearch, normalizePage, PAGES, parseRoute } from './core/route.js';
import { asArray, basename, debounce, esc } from './core/util.js';
import { ancestors } from './core/tree.js';
import { checkSummary, effectiveEdits, isFileReviewed, nextUnreviewedPath, normalizePhaseState, proposalReadiness, reviewProgress, workflowActive, REVIEW_SECTIONS } from './core/readiness.js';
import { icon } from './ui/icons.js';
import { $, applyGeometry, isTyping, preserveFocus } from './ui/dom.js';
import { confirmDialog, promptDialog, showShortcuts, toast } from './ui/overlays.js';
import { createPalette } from './ui/palette.js';
import { homeMain } from './views/home.js';
import { explorerMain, explorerSide } from './views/explorer.js';
import { newTaskMain, tasksMain } from './views/tasks.js';
import { reviewMain, reviewSide } from './views/review.js';
import { issuesMain, issuesSide } from './views/issues.js';
import { docsMain, docsSide } from './views/docs.js';
import { checksMain } from './views/checks.js';
import { gitMain, gitSide } from './views/git.js';
import { settingsMain, THEMES } from './views/settings.js';
import { findingLine, findingPath, findingIsOpen } from './views/shared.js';

// ---------------------------------------------------------------- preferences

const PREFS_KEY = 'mini_orca_prefs';
const LEGACY_THEMES = { navy: 'midnight', obsidian: 'midnight', catppuccin: 'midnight', nord: 'midnight', dracula: 'midnight', crt: 'contrast', amber: 'contrast', paper: 'light' };

function loadPrefs() {
  const prefs = { theme: 'system', density: 'comfortable', diffMode: 'split', sideWidth: 288, sideOpen: true };
  try {
    Object.assign(prefs, JSON.parse(localStorage.getItem(PREFS_KEY) || '{}'));
    const legacy = localStorage.getItem('mini_orca_theme');
    if (legacy && !localStorage.getItem(PREFS_KEY)) prefs.theme = LEGACY_THEMES[legacy] || (THEMES.some(([id]) => id === legacy) ? legacy : 'system');
  } catch (_) { /* storage unavailable: defaults apply */ }
  if (!THEMES.some(([id]) => id === prefs.theme)) prefs.theme = 'system';
  return prefs;
}

function savePrefs() {
  try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch (_) { /* best effort */ }
}

function applyPrefs() {
  const root = document.documentElement;
  root.dataset.theme = prefs.theme === 'midnight' ? 'midnight' : prefs.theme;
  root.dataset.density = prefs.density;
  root.style.setProperty('--side-w', Math.max(200, Math.min(520, prefs.sideWidth)) + 'px');
}

const prefs = loadPrefs();
applyPrefs();
const mobileQuery = window.matchMedia('(max-width: 900px)');
// On narrow screens the side panel is a drawer that starts closed each visit.
let drawerOpen = false;
const sideOpen = () => (mobileQuery.matches ? drawerOpen : prefs.sideOpen);
if (mobileQuery.addEventListener) mobileQuery.addEventListener('change', () => { drawerOpen = false; render(); });
const narrowQuery = window.matchMedia('(max-width: 760px)');
if (narrowQuery.addEventListener) narrowQuery.addEventListener('change', () => { state.review.narrow = narrowQuery.matches; render(); });

// ---------------------------------------------------------------- state

const state = createInitialState(parseRoute(window.location.search));
state.review.mode = prefs.diffMode === 'unified' ? 'unified' : 'split';
state.review.narrow = window.matchMedia('(max-width: 760px)').matches;
const api = createApi((input, init) => window.fetch(input, init));
let renderQueued = false;
const notify = () => {
  if (renderQueued) return;
  renderQueued = true;
  requestAnimationFrame(() => { renderQueued = false; render(); });
};
const actions = createActions({ state, api, ui: { confirm: confirmDialog, prompt: promptDialog, toast }, notify });

// ---------------------------------------------------------------- pages

const PAGE_DEFS = {
  home: { icon: 'home', key: '1', main: homeMain },
  files: { icon: 'files', key: '2', side: explorerSide, main: explorerMain },
  tasks: { icon: 'tasks', key: '3', main: tasksMain },
  'new-task': { nav: 'tasks', main: newTaskMain },
  diff: { icon: 'review', key: '4', side: reviewSide, main: reviewMain },
  issues: { icon: 'issues', key: '5', side: issuesSide, main: issuesMain },
  docs: { icon: 'docs', key: '6', side: docsSide, main: docsMain },
  checks: { icon: 'checks', key: '7', main: checksMain },
  git: { icon: 'git', key: '8', side: gitSide, main: gitMain },
  settings: { icon: 'settings', key: '9', bottom: true, main: s => settingsMain(s, prefs) }
};
const NAV_ORDER = ['home', 'files', 'tasks', 'diff', 'issues', 'docs', 'checks', 'git', 'settings'];

function syncURL(mode = 'replace') {
  const search = buildSearch({ tab: state.route.tab, docSection: state.route.docSection, reviewSection: state.route.reviewSection, worktreePath: state.route.worktreePath, sessionId: state.route.sessionId, file: state.route.file });
  if (search === window.location.search) return;
  window.history[mode === 'push' ? 'pushState' : 'replaceState'](null, '', '/ui' + search);
}

function go(page, patch = {}, mode = 'push') {
  const previous = state.route.tab;
  Object.assign(state.route, patch, { tab: normalizePage(page) });
  syncURL(mode);
  if (mobileQuery.matches) drawerOpen = false;
  render();
  if (previous !== state.route.tab) $('main').focus({ preventScroll: true });
}

async function openTask(id, section = 'changes', file = '') {
  if (state.pending) return;
  const same = state.activeSession && state.activeSession.id === id;
  go('diff', { sessionId: id, reviewSection: section, file });
  if (!same) await actions.loadSession(id);
  else if (!file) selectFirstReviewFile();
  syncURL();
}

function selectFirstReviewFile() {
  const session = state.activeSession;
  if (!session) return;
  const edits = effectiveEdits(session, state.review.sinceLastReview);
  if (!edits.some(edit => edit.path === state.route.file)) state.route.file = nextUnreviewedPath(session, edits, '') || (edits[0] ? edits[0].path : '');
}

async function openSource(path, line) {
  if (!path) return;
  ancestors(path).forEach(dir => state.explorer.expanded.add(dir));
  state.explorer.pendingLine = Number(line) || 0;
  go('files', { file: path });
  if (!(state.explorer.info && state.explorer.info.path === path)) await actions.openFile(path);
  notify();
}

window.addEventListener('popstate', async () => {
  const route = parseRoute(window.location.search);
  const checkoutChanged = route.worktreePath !== state.route.worktreePath;
  const sessionChanged = route.sessionId !== state.route.sessionId;
  const fileChanged = route.file !== state.route.file;
  Object.assign(state.route, route);
  render();
  if (checkoutChanged) { await actions.setCheckout(route.worktreePath); return; }
  if (sessionChanged && route.sessionId) await actions.loadSession(route.sessionId);
  if (route.tab === 'files' && route.file && fileChanged) await actions.openFile(route.file);
});

// ---------------------------------------------------------------- rendering

const regionCache = new WeakMap();
const SCROLLERS = '.page, .review-body, .editor-body, .side-scroll, .split > *';

function renderRegion(element, html, scrollKey) {
  const cached = regionCache.get(element);
  if (cached && cached.html === html) return false;
  const scrolls = cached && cached.scrollKey === scrollKey ? [...element.querySelectorAll(SCROLLERS)].map(el => el.scrollTop) : null;
  preserveFocus(element, () => { element.innerHTML = html; });
  applyGeometry(element);
  if (scrolls) element.querySelectorAll(SCROLLERS).forEach((el, i) => { if (scrolls[i]) el.scrollTop = scrolls[i]; });
  regionCache.set(element, { html, scrollKey });
  return true;
}

function render() {
  const page = state.route.tab;
  const def = PAGE_DEFS[page] || PAGE_DEFS.home;
  renderChrome(page, def);
  const app = $('app');
  if (def.side) {
    app.dataset.side = sideOpen() ? 'open' : 'closed';
    renderRegion($('side-content'), def.side(state), page);
  } else {
    app.dataset.side = 'none';
    renderRegion($('side-content'), '', page);
  }
  $('side-toggle').disabled = !def.side;
  $('side-toggle').setAttribute('aria-expanded', String(Boolean(def.side) && sideOpen()));
  const changed = renderRegion($('main'), def.main(state), page + '|' + (page === 'diff' ? state.route.reviewSection + '|' + (state.review.allFiles ? '*' : state.route.file) + '|' + (state.activeSession ? state.activeSession.id : '') : page === 'files' ? state.route.file : page === 'docs' ? state.route.docSection : ''));
  if (changed) afterMainRender(page);
}

function afterMainRender(page) {
  if (page === 'files' && state.explorer.pendingLine && state.explorer.info && state.explorer.info.path === state.route.file) {
    revealLine(state.explorer.pendingLine);
    state.explorer.pendingLine = 0;
  }
  if (page === 'files') {
    const active = $('side-content').querySelector('.tree-row.active');
    if (active) active.scrollIntoView({ block: 'nearest' });
  }
}

function revealLine(line) {
  const row = document.getElementById('L' + line);
  if (!row) return;
  document.querySelectorAll('.code tr.hl').forEach(el => el.classList.remove('hl'));
  row.classList.add('hl');
  row.scrollIntoView({ block: 'center' });
}

function renderChrome(page, def) {
  const project = state.project || {};
  const session = state.activeSession;
  const ws = state.gitWorkspace;
  const branch = state.route.worktreePath ? ((session && session.git && session.git.branch) || basename(state.route.worktreePath)) : ((ws && ws.branch) || 'main');
  $('crumb-project').textContent = project.name || 'Workspace';
  $('crumb-project').title = project.path || '';
  $('crumb-branch').textContent = branch + (state.route.worktreePath ? ' (worktree)' : '');
  $('crumb-page').textContent = PAGES[page] || 'Home';
  document.title = (PAGES[page] || 'Home') + ' · ' + (project.name || 'Mini-Orca');

  const openIssues = asArray(state.findings).filter(findingIsOpen).length;
  const progress = session ? reviewProgress(session) : null;
  const badges = {
    diff: session && workflowActive(session) ? '<span class="activity-badge dot" aria-hidden="true"></span>' : progress && progress.total - progress.reviewed > 0 && session.state !== 'applied' ? '<span class="activity-badge warn">' + (progress.total - progress.reviewed) + '</span>' : '',
    issues: openIssues ? '<span class="activity-badge">' + (openIssues > 99 ? '99+' : openIssues) + '</span>' : '',
    git: ws && asArray(ws.conflicts).length ? '<span class="activity-badge warn">!</span>' : ''
  };
  const current = def.nav || page;
  const item = id => {
    const d = PAGE_DEFS[id];
    return '<button class="activity-btn" type="button" data-action="open-page" data-page="' + id + '" title="' + esc(PAGES[id]) + ' (' + d.key + ')" aria-label="' + esc(PAGES[id]) + '"' + (current === id ? ' aria-current="page"' : '') + ' aria-keyshortcuts="' + d.key + '">' + icon(d.icon) + (badges[id] || '') + '</button>';
  };
  renderRegion($('activity'), NAV_ORDER.filter(id => !PAGE_DEFS[id].bottom).map(item).join('') + '<span class="activity-spacer"></span>' + NAV_ORDER.filter(id => PAGE_DEFS[id].bottom).map(item).join(''), 'nav');

  const pill = $('proposal-pill');
  if (session) {
    pill.hidden = false;
    const done = progress.total > 0 && progress.reviewed === progress.total;
    pill.innerHTML = '<span class="ring' + (done ? ' done' : '') + '" data-p="' + progress.percent + '"></span><span class="pill-title">' + esc(session.title || session.id) + '</span><span class="faint">' + progress.reviewed + '/' + progress.total + '</span>';
    pill.title = 'Open review: ' + (session.title || session.id);
    applyGeometry(pill);
  } else {
    pill.hidden = true;
  }

  const live = $('live-dot');
  live.dataset.state = state.connection;
  live.setAttribute('aria-label', state.connection === 'live' ? 'Live updates connected' : state.connection === 'offline' ? 'Live updates disconnected' : 'Connecting to live updates');
  live.title = live.getAttribute('aria-label');

  $('status-branch').innerHTML = icon('git') + '<span>' + esc(branch) + (ws && ws.dirty && !state.route.worktreePath ? ' •' : '') + '</span>';
  const trusted = Boolean(state.trust && state.trust.trusted);
  $('status-trust').innerHTML = icon(trusted ? 'lock' : 'unlock') + '<span class="' + (trusted ? '' : 'tone-warning') + '">' + (trusted ? 'Checks trusted' : 'Checks need trust') + '</span>';
  $('status-trust').title = trusted ? 'Project code may run in an isolated check workspace' : 'Grant execution trust in Settings to run checks';
  $('status-activity').innerHTML = activityLabel();
  const review = $('status-review');
  if (session) {
    review.hidden = false;
    const checks = checkSummary(session);
    review.innerHTML = icon('review') + '<span>' + progress.reviewed + '/' + progress.total + ' reviewed · ' + esc(checks.label) + '</span>';
  } else {
    review.hidden = true;
  }
  $('status-version').textContent = state.version ? 'daemon ' + state.version : '';
  [...document.querySelectorAll('[data-mutation]')].forEach(el => { if (state.pending) el.disabled = true; });
}

function activityLabel() {
  if (state.pending) return '<span class="spinner"></span>' + esc(state.pending);
  const session = state.activeSession;
  if (session && workflowActive(session)) {
    const stage = asArray(session.workflow.stages).find(s => normalizePhaseState(s.status) === 'running');
    return '<span class="spinner"></span>Agents · ' + esc(stage ? stage.name.replace('_', ' ') : session.workflow.status);
  }
  if (state.scan && normalizePhaseState(state.scan.status) === 'running') return '<span class="spinner"></span>Go scan running';
  const run = state.overview && state.overview.analysis_run;
  if (run && run.status === 'running') return '<span class="spinner"></span>Analysis running';
  if (!state.loaded) return '<span class="spinner"></span>Loading workspace';
  return '';
}

// ---------------------------------------------------------------- side panel

function setSide(open) {
  if (mobileQuery.matches) drawerOpen = open;
  else {
    prefs.sideOpen = open;
    savePrefs();
  }
  const def = PAGE_DEFS[state.route.tab];
  $('app').dataset.side = def && def.side ? (open ? 'open' : 'closed') : 'none';
  $('side-toggle').setAttribute('aria-expanded', String(open));
}

function toggleSide() {
  const open = !sideOpen();
  setSide(open);
  if (open) {
    const target = $('side-content').querySelector('input, button');
    if (target) target.focus();
  }
}

$('side-toggle').innerHTML = icon('panel');
$('help-trigger').innerHTML = icon('keyboard');
$('side-toggle').addEventListener('click', toggleSide);
$('side-scrim').addEventListener('click', () => setSide(false));
$('help-trigger').addEventListener('click', openShortcuts);
$('palette-trigger').addEventListener('click', () => palette.open());

(function resizer() {
  const handle = $('side-resizer');
  let startX = 0;
  let startW = 0;
  const move = event => {
    prefs.sideWidth = Math.max(200, Math.min(520, startW + event.clientX - startX));
    applyPrefs();
  };
  const up = () => {
    handle.classList.remove('dragging');
    window.removeEventListener('pointermove', move);
    window.removeEventListener('pointerup', up);
    savePrefs();
  };
  handle.addEventListener('pointerdown', event => {
    startX = event.clientX;
    startW = prefs.sideWidth;
    handle.classList.add('dragging');
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up);
  });
  handle.addEventListener('keydown', event => {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
    event.preventDefault();
    prefs.sideWidth = Math.max(200, Math.min(520, prefs.sideWidth + (event.key === 'ArrowRight' ? 16 : -16)));
    applyPrefs();
    savePrefs();
  });
})();

// ---------------------------------------------------------------- review helpers

function reviewEdits() {
  return effectiveEdits(state.activeSession, state.review.sinceLastReview);
}

function stepFile(delta) {
  const edits = reviewEdits();
  if (!edits.length) return;
  const index = edits.findIndex(edit => edit.path === state.route.file);
  const next = edits[(index + delta + edits.length) % edits.length];
  Object.assign(state.route, { file: next.path, reviewSection: 'changes' });
  state.review.allFiles = false;
  syncURL();
  render();
  scrollReviewTop();
}

function nextUnreviewed() {
  const session = state.activeSession;
  const next = nextUnreviewedPath(session, reviewEdits(), state.route.file);
  if (!next) {
    toast(proposalReadiness(session).filesReviewed ? 'All files reviewed. Continue with checks.' : 'No other unreviewed files.', 'info');
    if (proposalReadiness(session).filesReviewed) go('diff', { reviewSection: 'checks' });
    return;
  }
  Object.assign(state.route, { file: next, reviewSection: 'changes' });
  state.review.allFiles = false;
  syncURL();
  render();
  scrollReviewTop();
}

function scrollReviewTop() {
  const body = $('review-body');
  if (body) body.scrollTop = 0;
}

async function markCurrentReviewed() {
  const session = state.activeSession;
  if (!session || state.review.allFiles || state.route.reviewSection !== 'changes') return;
  const path = state.route.file;
  if (path && !isFileReviewed(session, path)) {
    await actions.reviewFile(path);
    scrollReviewTop();
  }
}

// ---------------------------------------------------------------- click actions

const clickHandlers = {
  'open-page': el => {
    const page = el.dataset.page || 'home';
    if (page === 'diff') return clickHandlers['open-review'](el);
    go(page);
  },
  'new-task': () => {
    if (state.route.tab !== 'new-task') state.draft = state.draft.title || state.draft.paths.length ? state.draft : emptyDraft();
    go('new-task');
  },
  palette: () => palette.open(),
  'palette-files': () => palette.open(''),
  shortcuts: () => openShortcuts(),
  refresh: () => actions.loadAll(),
  'open-review': el => {
    const section = REVIEW_SECTIONS.includes(el.dataset.section) ? el.dataset.section : state.route.reviewSection || 'changes';
    if (state.activeSession) {
      go('diff', { reviewSection: section, file: el.dataset.file || state.route.file });
      if (!el.dataset.file) selectFirstReviewFile();
      if (el.dataset.file) state.review.allFiles = false;
      syncURL();
      render();
    } else if (state.route.sessionId) {
      openTask(state.route.sessionId, section, el.dataset.file || '');
    } else {
      go('diff');
    }
  },
  'open-task': el => openTask(el.dataset.id),
  'review-section': el => { go('diff', { reviewSection: el.dataset.section }); if (el.dataset.section === 'changes') { selectFirstReviewFile(); syncURL(); render(); } },
  'review-select': el => { state.review.allFiles = false; Object.assign(state.route, { file: el.dataset.path, reviewSection: 'changes' }); syncURL(); render(); scrollReviewTop(); },
  'review-file': el => actions.reviewFile(el.dataset.path).then(scrollReviewTop),
  'next-unreviewed': () => nextUnreviewed(),
  'file-step': el => stepFile(Number(el.dataset.step) || 1),
  'diff-mode': el => { state.review.mode = el.dataset.mode; prefs.diffMode = state.review.mode; savePrefs(); render(); },
  'diff-scope': el => { state.review.allFiles = el.dataset.all === 'true'; if (!state.review.allFiles) selectFirstReviewFile(); render(); },
  'diff-since': el => { state.review.sinceLastReview = el.dataset.since === 'true'; selectFirstReviewFile(); render(); },
  'expand-gap': el => {
    const path = el.dataset.path;
    const list = state.review.expandedGaps[path] || [];
    state.review.expandedGaps[path] = list.concat(Number(el.dataset.start));
    render();
  },
  'run-checks': () => actions.runChecks(),
  'approve-review': () => actions.approveReview(),
  apply: () => actions.applyProposal({ commit: state.review.commit, message: state.review.commitMessage ?? defaultCommit() }),
  undo: () => actions.undoProposal(),
  refine: async () => { if (await actions.refine(state.review.refineMessage)) { state.review.refineMessage = ''; go('diff', { reviewSection: 'changes' }); selectFirstReviewFile(); render(); } },
  'refine-preset': el => { state.review.refineMessage = el.dataset.text; render(); const box = document.querySelector('[data-input="refine-message"]'); if (box) box.focus(); },
  'start-workflow': async () => {
    const message = state.review.workflowMessage ?? (document.querySelector('[data-input="workflow-message"]') || {}).value;
    if (await actions.startWorkflow({ message, autoRepair: state.review.autoRepair })) state.review.workflowMessage = null;
  },
  'cancel-workflow': () => actions.cancelWorkflow(),
  'workflow-input': el => actions.respondWorkflowInput(el.dataset.decision),
  'checks-filter': el => { state.checks.filter = el.dataset.filter; render(); },
  'draft-step': el => actions.setDraftStep(Number(el.dataset.step)),
  'draft-save': async () => { const created = await actions.createTask(false); if (created) go('diff', { sessionId: created.id, reviewSection: 'pipeline', file: '' }); },
  'draft-generate': async () => { const created = await actions.createTask(true); if (created) go('diff', { sessionId: created.id, reviewSection: state.activeSession && asArray(state.activeSession.changes).length ? 'changes' : 'pipeline', file: '' }); },
  'draft-toggle-path': el => actions.toggleDraftPath(el.dataset.path),
  'draft-add-manual': () => addManualPath(),
  'toggle-dir': el => { const set = state.explorer.expanded; if (set.has(el.dataset.path)) set.delete(el.dataset.path); else set.add(el.dataset.path); render(); },
  'collapse-tree': () => { state.explorer.expanded.clear(); render(); },
  reindex: () => actions.reindex(),
  'open-file': el => openSource(el.dataset.path, el.dataset.line),
  'close-tab': async el => { const next = actions.closeTab(el.dataset.path); syncURL(); if (next) await actions.openFile(next); },
  'explain-file': () => actions.explainFile(),
  'task-from-file': el => { actions.draftFromFile(el.dataset.path); go('new-task'); },
  'goto-line': el => revealLine(Number(el.dataset.line)),
  'issues-filter': el => { state.issues[el.dataset.key] = el.dataset.value; state.issues.selected = ''; render(); },
  'issue-select': el => { state.issues.selected = el.dataset.id; render(); },
  'issue-fix': el => { const finding = asArray(state.findings).find(f => f.id === el.dataset.id); if (finding) { actions.draftFromFinding(finding); go('new-task'); } },
  'issue-triage': async el => { const finding = asArray(state.findings).find(f => f.id === el.dataset.id); await actions.triageFinding(finding, el.dataset.status); render(); },
  'doc-section': el => go('docs', { docSection: el.dataset.section }),
  'run-analysis': () => actions.runAnalysis(),
  'analysis-control': el => actions.controlAnalysis(el.dataset.op),
  'run-scan': () => actions.runScan(),
  'cancel-scan': () => actions.cancelScan(),
  'git-select': el => { state.merge = { path: el.dataset.path, preview: state.merge.path === el.dataset.path ? state.merge.preview : null, files: state.merge.path === el.dataset.path ? state.merge.files : [], loading: false }; go('git'); },
  'merge-preview': el => actions.previewMerge(el.dataset.path),
  merge: el => actions.mergeWorktree(el.dataset.path),
  'abort-merge': () => actions.abortMerge(),
  'set-checkout': el => { actions.setCheckout(el.dataset.path).then(() => go('home')); },
  'toggle-trust': () => actions.toggleTrust(),
  'switch-project': () => actions.switchProject()
};

function defaultCommit() {
  const session = state.activeSession;
  if (!session) return '';
  const prefix = { fix: 'fix', feature: 'feat', refactor: 'refactor', performance: 'perf', security: 'fix' }[session.kind] || 'chore';
  return prefix + ': ' + String(session.title || session.id).replace(/^fix:\s*/i, '');
}

function addManualPath() {
  const input = document.querySelector('[data-input="draft-manual-path"]');
  const path = input ? input.value.trim().replace(/^\.\//, '') : '';
  if (!path) return;
  if (!state.draft.paths.includes(path)) actions.toggleDraftPath(path);
  if (input) input.value = '';
}

document.addEventListener('click', event => {
  const el = event.target.closest('[data-action]');
  if (!el || el.disabled || el.getAttribute('aria-disabled') === 'true') return;
  const handler = clickHandlers[el.dataset.action];
  if (!handler) return;
  event.preventDefault();
  Promise.resolve(handler(el)).catch(err => toast(err.message || String(err), 'danger'));
});

// ---------------------------------------------------------------- inputs

const rerenderSoon = debounce(() => render(), 90);
const inputHandlers = {
  'explorer-filter': v => { state.explorer.filter = v; rerenderSoon(); },
  'tasks-filter': v => { state.tasks.q = v; rerenderSoon(); },
  'issues-q': v => { state.issues.q = v; rerenderSoon(); },
  'review-filter': v => { state.review.filter = v; rerenderSoon(); },
  'draft-title': v => { state.draft.title = v; if (state.draft.errorField === 'title' && v.trim()) { state.draft.error = ''; state.draft.errorField = ''; rerenderSoon(); } },
  'draft-spec': v => { state.draft.spec = v; },
  'draft-criteria': v => { state.draft.criteria = v; },
  'draft-kind': v => { state.draft.kind = v; },
  'draft-path-query': v => { state.draft.pathQuery = v; rerenderSoon(); },
  'draft-mode': v => { state.draft.mode = v; render(); },
  'draft-auto-repair': (v, el) => { state.draft.autoRepair = el.checked; },
  'workflow-message': v => { state.review.workflowMessage = v; },
  'workflow-repair': (v, el) => { state.review.autoRepair = el.checked; },
  'refine-message': v => { state.review.refineMessage = v; },
  'commit-toggle': (v, el) => { state.review.commit = el.checked; if (state.review.commitMessage === null) state.review.commitMessage = defaultCommit(); render(); },
  'commit-message': v => { state.review.commitMessage = v; },
  'pref-theme': v => { prefs.theme = v; savePrefs(); applyPrefs(); },
  'pref-density': v => { prefs.density = v; savePrefs(); applyPrefs(); },
  'pref-diff': v => { prefs.diffMode = v; state.review.mode = v; savePrefs(); },
  checkout: v => { actions.setCheckout(v); }
};

function onInput(event) {
  const el = event.target.closest('[data-input]');
  if (!el) return;
  const handler = inputHandlers[el.dataset.input];
  const textual = el.tagName === 'TEXTAREA' || (el.tagName === 'INPUT' && !['checkbox', 'radio'].includes(el.type));
  if (handler && (event.type === 'input' ? textual : !textual)) handler(el.value, el);
}
document.addEventListener('input', onInput);
document.addEventListener('change', onInput);

document.addEventListener('keydown', event => {
  const el = event.target;
  if (!el || !el.dataset) return;
  if (el.dataset.input === 'goto-line' && event.key === 'Enter') { event.preventDefault(); revealLine(Number(el.value)); }
  if (el.dataset.input === 'draft-manual-path' && event.key === 'Enter') { event.preventDefault(); addManualPath(); }
  if (el.dataset.input === 'explorer-filter' && event.key === 'Enter') {
    const first = $('side-content').querySelector('[data-action="open-file"]');
    if (first) { event.preventDefault(); first.click(); }
  }
  if (['draft-title'].includes(el.dataset.input) && event.key === 'Enter') { event.preventDefault(); actions.setDraftStep(2); }
  if (['refine-message', 'workflow-message', 'commit-message'].includes(el.dataset.input) && event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
    event.preventDefault();
    const action = { 'refine-message': 'refine', 'workflow-message': 'start-workflow', 'commit-message': 'apply' }[el.dataset.input];
    const button = document.querySelector('[data-action="' + action + '"]');
    if (button && !button.disabled) button.click();
  }
});

// ---------------------------------------------------------------- shortcuts

const SHORTCUTS = [
  { title: 'Navigate', items: [['⌘K', 'Search everything'], ['⌘P', 'Open a file'], ['1 … 9', 'Switch page'], ['⌘B', 'Toggle side panel'], ['C', 'New task'], ['/', 'Focus filter'], ['?', 'Show shortcuts']] },
  { title: 'Review', items: [['J K', 'Next / previous file'], ['N', 'Next unreviewed file'], ['M', 'Mark file reviewed'], ['V', 'Split / unified'], ['A', 'One file / all files'], ['[ ]', 'Previous / next step']] },
  { title: 'Explorer', items: [['⌘G', 'Go to line'], ['Enter', 'Open first match (in filter)']] },
  { title: 'Forms', items: [['⌘Enter', 'Submit instruction'], ['Esc', 'Close dialog']] }
];

function openShortcuts() {
  showShortcuts(SHORTCUTS);
}

document.addEventListener('keydown', event => {
  const mod = event.metaKey || event.ctrlKey;
  const key = event.key.toLowerCase();
  if (mod && key === 'k') { event.preventDefault(); palette.toggle(); return; }
  if (mod && key === 'p') { event.preventDefault(); palette.open(event.shiftKey ? '>' : ''); return; }
  if (mod && key === 'b') { event.preventDefault(); toggleSide(); return; }
  if (mod && key === 'g' && state.route.tab === 'files') { event.preventDefault(); const input = $('goto-line'); if (input) input.focus(); return; }
  if (document.querySelector('dialog[open]')) return;
  if (event.key === 'Escape' && mobileQuery.matches && drawerOpen) { setSide(false); $('side-toggle').focus(); return; }
  if (mod || event.altKey || isTyping(event)) return;
  if (event.key === '?') { event.preventDefault(); openShortcuts(); return; }
  if (event.key === '/') {
    const filter = $('side-content').querySelector('input[type="search"]') || $('main').querySelector('input[type="search"]');
    if (filter) { event.preventDefault(); filter.focus(); filter.select(); }
    return;
  }
  const page = NAV_ORDER.find(id => PAGE_DEFS[id].key === event.key);
  if (page) { event.preventDefault(); page === 'diff' ? clickHandlers['open-review']({ dataset: {} }) : go(page); return; }
  if (key === 'c' && !event.shiftKey) { event.preventDefault(); clickHandlers['new-task'](); return; }
  if (state.route.tab === 'diff' && state.activeSession) {
    const sections = REVIEW_SECTIONS;
    const index = sections.indexOf(state.route.reviewSection);
    if (event.key === ']' || event.key === '[') { event.preventDefault(); go('diff', { reviewSection: sections[(index + (event.key === ']' ? 1 : -1) + sections.length) % sections.length] }); return; }
    if (state.route.reviewSection !== 'changes') return;
    if (key === 'j') { event.preventDefault(); stepFile(1); }
    else if (key === 'k') { event.preventDefault(); stepFile(-1); }
    else if (key === 'n') { event.preventDefault(); nextUnreviewed(); }
    else if (key === 'm') { event.preventDefault(); markCurrentReviewed(); }
    else if (key === 'v') { event.preventDefault(); state.review.mode = state.review.mode === 'split' ? 'unified' : 'split'; prefs.diffMode = state.review.mode; savePrefs(); render(); }
    else if (key === 'a') { event.preventDefault(); state.review.allFiles = !state.review.allFiles; if (!state.review.allFiles) selectFirstReviewFile(); render(); }
  }
});

// Keyboard reachability for row-like elements that are not buttons (editor tabs).
document.addEventListener('keydown', event => {
  if ((event.key === 'Enter' || event.key === ' ') && event.target.matches && event.target.matches('[role="tab"][data-action]:not(button)')) {
    event.preventDefault();
    event.target.click();
  }
});

// ---------------------------------------------------------------- palette

const palette = createPalette(() => {
  const items = [];
  const cmd = (id, label, run, opts = {}) => items.push(Object.assign({ id, label, run, group: 'Commands', icon: opts.icon || 'bolt' }, opts));
  NAV_ORDER.forEach(id => cmd('go-' + id, 'Go to ' + PAGES[id], () => (id === 'diff' ? clickHandlers['open-review']({ dataset: {} }) : go(id)), { icon: PAGE_DEFS[id].icon, kbd: PAGE_DEFS[id].key }));
  cmd('new-task', 'New task', () => clickHandlers['new-task'](), { icon: 'plus', kbd: 'C' });
  cmd('refresh', 'Refresh workspace', () => actions.loadAll(), { icon: 'refresh' });
  cmd('reindex', 'Reindex project files', () => actions.reindex(), { icon: 'refresh', hint: 'No model request' });
  cmd('scan', 'Run Go scan', () => actions.runScan(), { icon: 'play', hint: 'Runs project code in a copy' });
  cmd('analysis', 'Run project analysis & generate docs', () => actions.runAnalysis(), { icon: 'sparkle' });
  cmd('trust', state.trust && state.trust.trusted ? 'Revoke execution trust' : 'Grant execution trust', () => actions.toggleTrust(), { icon: 'shield' });
  cmd('switch-project', 'Open another project…', () => actions.switchProject(), { icon: 'folder' });
  cmd('toggle-side', 'Toggle side panel', toggleSide, { icon: 'panel', kbd: '⌘B' });
  cmd('shortcuts', 'Keyboard shortcuts', openShortcuts, { icon: 'keyboard', kbd: '?' });
  THEMES.forEach(([id, label]) => cmd('theme-' + id, 'Theme: ' + label, () => { prefs.theme = id; savePrefs(); applyPrefs(); render(); }, { icon: 'eye' }));
  const session = state.activeSession;
  if (session) {
    const ready = proposalReadiness(session);
    cmd('review-next', 'Review: next unreviewed file', () => { go('diff', { reviewSection: 'changes' }); nextUnreviewed(); }, { icon: 'review', kbd: 'N' });
    cmd('review-checks', 'Review: run checks', () => actions.runChecks(), { icon: 'play', enabled: ready.editable && asArray(session.changes).length > 0 });
    cmd('review-approve', 'Review: approve this version', () => actions.approveReview(), { icon: 'check', enabled: ready.canApprove && !ready.canApply });
    cmd('review-apply', 'Review: apply changes…', () => go('diff', { reviewSection: 'apply' }), { icon: 'check', enabled: ready.canApply });
    cmd('review-agents', 'Review: agent workflow', () => go('diff', { reviewSection: 'pipeline' }), { icon: 'agent' });
    if (workflowActive(session)) cmd('review-cancel', 'Review: cancel agent workflow', () => actions.cancelWorkflow(), { icon: 'stop' });
  }
  asArray(state.changes).forEach(task => items.push({ id: 'task-' + task.id, group: 'Tasks', label: task.title || task.id, hint: task.state + (task._branch ? ' · ⎇ ' + task._branch : ''), icon: 'review', run: () => openTask(task.id) }));
  asArray(state.index && state.index.files).forEach(file => items.push({ id: 'file-' + file.path, group: 'Files', label: file.path, hint: file.language || '', icon: 'file', run: () => openSource(file.path) }));
  asArray(state.findings).filter(findingIsOpen).slice(0, 200).forEach(f => items.push({ id: 'issue-' + f.id, group: 'Tasks', label: 'Issue: ' + (f.title || f.id), hint: findingPath(f) + (findingLine(f) ? ':' + findingLine(f) : ''), icon: 'issues', run: () => { state.issues.selected = f.id; go('issues'); } }));
  return items;
});

// ---------------------------------------------------------------- live updates

const refreshFromEvents = debounce(() => { if (!state.pending) actions.loadAll(); }, 400);

function connectEvents() {
  if (typeof EventSource === 'undefined') { state.connection = 'offline'; return; }
  const source = new EventSource('/api/projects/current/events');
  source.addEventListener('open', () => { state.connection = 'live'; notify(); });
  source.addEventListener('update', () => refreshFromEvents());
  source.addEventListener('error', () => { state.connection = source.readyState === EventSource.CLOSED ? 'offline' : 'connecting'; notify(); });
}

// Fallback polling only while background work runs and live updates are down.
setInterval(() => {
  if (state.connection === 'live' || state.pending || document.hidden) return;
  const busy = (state.activeSession && workflowActive(state.activeSession)) || (state.scan && normalizePhaseState(state.scan.status) === 'running') || (state.overview && state.overview.analysis_run && state.overview.analysis_run.status === 'running');
  if (busy) actions.loadAll();
}, 5000);

// ---------------------------------------------------------------- boot

if (state.route.tab === 'files' && state.route.file) ancestors(state.route.file).forEach(dir => state.explorer.expanded.add(dir));
render();
syncURL();
actions.loadAll().then(() => {
  if (state.route.tab === 'diff') { selectFirstReviewFile(); syncURL(); render(); }
});
connectEvents();
