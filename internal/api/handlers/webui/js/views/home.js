// Home: a calm dashboard with the single next action first.

import { asArray, esc, plural, revisionLabel, timeAgo } from '../core/util.js';
import { checkSummary, nextAction, proposalReadiness, reviewProgress, workflowActive, normalizePhaseState } from '../core/readiness.js';
import { icon } from '../ui/icons.js';
import { badge, button, empty } from '../ui/dom.js';
import { taskStateBadge } from './shared.js';

const NEXT_ICONS = { warning: 'review', success: 'check', danger: 'alert', info: 'agent', neutral: 'sparkle' };

export function homeMain(state) {
  if (!state.loaded) return '<div class="page"><div class="page-inner">' + skeleton() + '</div></div>';
  const project = state.project || {};
  const files = asArray(state.index && state.index.files);
  const findings = asArray(state.findings).filter(f => !['dismissed', 'fixed', 'retired'].includes(String(f.status || 'open')));
  const severe = findings.filter(f => ['critical', 'high'].includes(String(f.severity).toLowerCase())).length;
  const scan = state.scan;
  const scanTone = scan ? normalizePhaseState(scan.status) : 'idle';
  const next = nextAction(state);
  const branch = (state.gitWorkspace && state.gitWorkspace.branch) || '';
  const openTasks = asArray(state.changes).filter(c => !['applied', 'undone', 'reverted'].includes(c.state));
  const docs = state.overview && state.overview.docs;

  let html = '<div class="page"><div class="page-inner">';
  html += '<header class="hero"><div><p class="eyebrow">Workspace</p><h1 class="page-title">' + esc(project.name || 'Your workspace') + '</h1>' +
    '<div class="hero-meta">' +
    (project.path ? '<span title="' + esc(project.path) + '">' + icon('folder') + '<span class="path">' + esc(project.path) + '</span></span>' : '') +
    (branch ? '<span>' + icon('git') + esc(branch) + (state.gitWorkspace && state.gitWorkspace.dirty ? ' · modified' : '') + '</span>' : '') +
    (project.project_revision ? '<span>' + icon('commit') + '<span class="mono">#' + esc(revisionLabel(project.project_revision)) + '</span></span>' : '') +
    '</div></div><div class="row">' + button('New task', { variant: 'primary', icon: 'plus', kbd: 'C', data: { action: 'new-task' } }) + button('Search', { icon: 'search', kbd: '⌘K', data: { action: 'palette' } }) + '</div></header>';

  html += '<section class="next-card" data-tone="' + esc(next.tone) + '" aria-label="Next step"><div class="next-icon">' + icon(NEXT_ICONS[next.tone] || 'sparkle') + '</div>' +
    '<div class="grow"><p class="eyebrow">Next step</p><div class="next-title">' + esc(next.title) + '</div><p class="muted small">' + esc(next.detail) + '</p></div>' +
    button(next.label, { variant: 'primary', icon: 'arrowRight', data: { action: next.action, page: next.page || '', section: next.section || '' } }) + '</section>';

  html += '<section class="stats" aria-label="Workspace summary">' +
    stat('files', 'Indexed files', String(files.length), files.length ? plural(Object.keys(languageCounts(files)).length, 'language') : 'Reindex from Settings', 'files') +
    stat('tasks', 'Open tasks', String(openTasks.length), openTasks.length ? plural(openTasks.filter(c => c.workflow_status === 'running').length, 'running workflow') : 'Create one from any file or issue', 'tasks') +
    stat('issues', 'Open issues', String(findings.length), severe ? severe + ' high or critical' : findings.length ? 'No high-severity issues' : 'Nothing to triage', 'issues', severe ? 'tone-danger' : '') +
    stat('checks', 'Go scan', scan ? String(scan.status || 'recorded') : 'Not run', scan && scan.updated_at ? timeAgo(scan.updated_at) : 'Runs in an isolated copy', 'checks', scanTone === 'failed' ? 'tone-danger' : scanTone === 'passed' ? 'tone-success' : '') +
    '</section>';

  html += '<div class="grid-2"><div class="stack">' + activeTaskCard(state) + recentTasks(state) + '</div><div class="stack">' + docsCard(docs, state) + quickLinks() + '</div></div>';
  return html + '</div></div>';
}

function stat(id, label, value, sub, page, tone = '') {
  return '<button class="stat" type="button" data-action="open-page" data-page="' + page + '"><span class="stat-label">' + icon(page === 'checks' ? 'checks' : page) + esc(label) + '</span><span class="stat-value ' + tone + '" id="home-stat-' + id + '">' + esc(value) + '</span><span class="stat-sub">' + esc(sub) + '</span></button>';
}

function languageCounts(files) {
  const counts = {};
  for (const file of files) counts[file.language || 'text'] = (counts[file.language || 'text'] || 0) + 1;
  return counts;
}

export function pipelineSteps(session) {
  const ready = proposalReadiness(session);
  const progress = reviewProgress(session);
  const checks = checkSummary(session);
  const hasEdits = asArray(session.changes).length > 0;
  const running = workflowActive(session);
  return [
    { section: 'spec', label: 'Generate', detail: running ? 'Agents running' : hasEdits ? 'Proposal ready' : 'Not generated', cls: hasEdits ? 'done' : 'current' },
    { section: 'changes', label: 'Review', detail: progress.reviewed + '/' + progress.total + ' files', cls: ready.filesReviewed ? 'done' : hasEdits ? 'current' : '' },
    { section: 'checks', label: 'Checks', detail: checks.label, cls: checks.failed ? 'failed' : ready.checksPassed ? 'done' : ready.filesReviewed ? 'current' : '' },
    { section: 'apply', label: 'Apply', detail: session.state === 'applied' ? 'Applied' : ready.canApply ? 'Approved' : ready.canApprove ? 'Ready to approve' : 'Locked', cls: session.state === 'applied' ? 'done' : ready.canApprove ? 'current' : '' }
  ];
}

function activeTaskCard(state) {
  const session = state.activeSession;
  if (!session) {
    return '<section class="card"><div class="card-head"><span class="card-title">Active task</span></div>' + empty('No active task', 'Shape a focused change across 1–8 files, review the diff and apply it when checks pass.', 'tasks', button('New task', { variant: 'primary', icon: 'plus', data: { action: 'new-task' } })) + '</section>';
  }
  const steps = pipelineSteps(session);
  const progress = reviewProgress(session);
  return '<section class="card" id="home-active-task" aria-label="Active task"><div class="card-head"><span class="card-title truncate">' + icon('review') + '<span class="truncate">' + esc(session.title || session.id) + '</span></span>' + taskStateBadge(session) + '</div>' +
    '<div class="card-body stack"><p class="muted small">' + esc(proposalReadiness(session).message) + '</p>' +
    '<div class="pipeline" data-pipe="4">' + steps.map((step, i) => '<button class="pipe-step ' + step.cls + '" type="button" data-action="open-review" data-section="' + step.section + '"><span class="num">' + (step.cls === 'done' ? icon('check') : '') + 'Step ' + (i + 1) + '</span><span class="lbl">' + esc(step.label) + '</span><span class="det">' + esc(step.detail) + '</span></button>').join('') + '</div>' +
    '<div class="progress' + (progress.percent === 100 ? ' success' : '') + '" role="progressbar" aria-label="Files reviewed" aria-valuenow="' + progress.percent + '" aria-valuemin="0" aria-valuemax="100"><span data-w="' + progress.percent + '"></span></div>' +
    (asArray(session.changes).length ? '<div class="chips">' + asArray(session.changes).slice(0, 6).map(edit => '<button class="chip" type="button" data-action="open-review" data-section="changes" data-file="' + esc(edit.path) + '">' + icon('file') + '<span>' + esc(edit.path) + '</span></button>').join('') + '</div>' : '') +
    '</div></section>';
}

function recentTasks(state) {
  const tasks = asArray(state.changes).slice(0, 5);
  const body = tasks.length ? '<div class="list">' + tasks.map(task => '<button class="list-row" type="button" data-action="open-task" data-id="' + esc(task.id) + '"><div class="grow"><div class="title">' + esc(task.title || task.id) + '</div><div class="sub">' + esc([task.kind, task._branch ? '⎇ ' + task._branch : '', timeAgo(task.updated_at)].filter(Boolean).join(' · ')) + '</div></div><div class="end">' + taskStateBadge(task) + '<span class="chev-right"></span></div></button>').join('') + '</div>'
    : '<div class="card-body muted small">Tasks you create appear here.</div>';
  return '<section class="card"><div class="card-head"><span class="card-title">Recent tasks</span>' + button('View all', { variant: 'ghost', size: 'sm', data: { action: 'open-page', page: 'tasks' } }) + '</div>' + body + '</section>';
}

function docsCard(docs, state) {
  const purpose = (docs && docs.context && docs.context.purpose) || (state.overview && state.overview.analysis && state.overview.analysis.purpose) || '';
  const status = docs ? docs.status : '';
  return '<section class="card"><div class="card-head"><span class="card-title">' + icon('docs') + 'Project docs</span>' + (status ? badge(status, status === 'fresh' || status === 'completed' ? 'success' : 'warning') : badge('Not generated')) + '</div><div class="card-body stack">' +
    (purpose ? '<p class="small">' + esc(purpose.length > 280 ? purpose.slice(0, 277) + '…' : purpose) + '</p>' : '<p class="muted small">Run a project analysis to generate an engineering handbook: architecture, components, scenarios and decisions.</p>') +
    '<div class="row">' + button('Open docs', { size: 'sm', data: { action: 'open-page', page: 'docs' } }) + (docs ? '' : button('Run analysis', { size: 'sm', variant: 'ghost', icon: 'play', data: { action: 'run-analysis' } })) + '</div></div></section>';
}

function quickLinks() {
  const links = [['issues', 'Triage issues', 'Turn findings into fix tasks'], ['git', 'Branches & worktrees', 'Preview and merge agent work'], ['checks', 'Checks & analysis', 'Go scan and analysis coverage']];
  return '<section class="card"><div class="list">' + links.map(([page, title, sub]) => '<button class="list-row" type="button" data-action="open-page" data-page="' + page + '">' + icon(page) + '<div class="grow"><div class="title">' + esc(title) + '</div><div class="sub">' + esc(sub) + '</div></div><span class="chev-right"></span></button>').join('') + '</div></section>';
}

function skeleton() {
  return '<div class="stack-lg" aria-busy="true" aria-label="Loading workspace"><div class="skeleton sk-title"></div><div class="skeleton sk-block"></div><div class="stats">' + '<div class="skeleton sk-stat"></div>'.repeat(4) + '</div></div>';
}
