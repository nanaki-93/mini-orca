// Review: the lite-IDE review loop. Side panel lists changed files with review
// state; main shows one step at a time — Changes, Checks, Agents, Refine, Apply.
// Navigation never marks files reviewed, runs checks or contacts a model.

import { asArray, basename, dirname, esc, formatDuration, plural, shortHash } from '../core/util.js';
import { buildSegments, diffStats, splitRows } from '../core/diff.js';
import { checkSummary, effectiveEdits, isFileReviewed, normalizePhaseState, proposalReadiness, reviewProgress, workflowActive, REVIEW_SECTIONS } from '../core/readiness.js';
import { workflowBlocker, workflowSpecialists, workflowStages } from '../core/workflow.js';
import { fileIconClass, icon } from '../ui/icons.js';
import { badge, button, callout, empty, stateIcon, toneForState } from '../ui/dom.js';
import { taskStateBadge } from './shared.js';

const SECTION_LABELS = { changes: 'Changes', checks: 'Checks', pipeline: 'Agents', spec: 'Refine', apply: 'Apply' };

export function reviewSide(state) {
  const session = state.activeSession;
  if (!session) {
    const tasks = asArray(state.changes);
    return '<div class="side-head"><span class="side-title">Review</span></div><div class="side-scroll">' + (tasks.length ? '<div class="nav-group-label">Choose a task</div><div class="nav-list">' + tasks.slice(0, 30).map(task => '<button class="nav-item" type="button" data-action="open-task" data-id="' + esc(task.id) + '">' + icon('review') + '<span class="truncate">' + esc(task.title || task.id) + '</span></button>').join('') + '</div>' : '<div class="side-section muted small">No tasks yet.</div>') + '</div>';
  }
  const edits = effectiveEdits(session, state.review.sinceLastReview);
  const progress = reviewProgress(session);
  const filter = state.review.filter.trim().toLowerCase();
  const shown = filter ? edits.filter(edit => edit.path.toLowerCase().includes(filter)) : edits;
  let html = '<div class="side-head"><span class="side-title">Changed files <span class="count">' + edits.length + '</span></span></div>';
  html += '<div class="side-section stack"><div class="row-between small"><span class="muted">' + progress.reviewed + ' of ' + progress.total + ' reviewed</span><span class="mono faint">' + progress.percent + '%</span></div><div class="progress' + (progress.percent === 100 ? ' success' : '') + '"><span data-w="' + progress.percent + '"></span></div>' +
    (edits.length > 6 ? '<div class="search-wrap"><span class="search-icon"></span><input class="input input-sm" type="search" data-input="review-filter" data-focus-key="review-filter" value="' + esc(state.review.filter) + '" placeholder="Filter changed files" aria-label="Filter changed files"></div>' : '') + '</div>';
  html += '<div class="side-scroll"><div class="tree" role="listbox" aria-label="Changed files">' + (shown.length ? shown.map(edit => {
    const reviewed = isFileReviewed(session, edit.path);
    const stats = diffStats(edit);
    const active = state.route.file === edit.path && !state.review.allFiles;
    return '<button class="tree-row' + (active ? ' active' : '') + '" type="button" role="option" aria-selected="' + active + '" data-action="review-select" data-path="' + esc(edit.path) + '" title="' + esc(edit.path) + (reviewed ? ' — reviewed' : ' — not reviewed') + '"><span class="status-dot ' + (reviewed ? 'done' : 'pending') + '" aria-hidden="true"></span>' + icon('file', 'ficon ' + fileIconClass(edit.path)) +
      '<span class="label">' + esc(basename(edit.path)) + (dirname(edit.path) ? ' <span class="faint xs">' + esc(dirname(edit.path)) + '</span>' : '') + '</span><span class="diffstat"><span class="add">+' + stats.added + '</span><span class="del">−' + stats.removed + '</span></span></button>';
  }).join('') : '<div class="side-section faint small">' + (state.review.sinceLastReview ? 'No changes since your last review.' : 'No file edits yet.') + '</div>') + '</div></div>';
  html += '<div class="side-foot">' + gate(session) + '</div>';
  return html;
}

function gate(session) {
  const ready = proposalReadiness(session);
  const checks = checkSummary(session);
  const progress = reviewProgress(session);
  const workflow = session.workflow ? session.workflow.status : '';
  const row = (label, value, tone) => '<div class="gate-row"><span>' + label + '</span><strong class="' + (tone ? 'tone-' + tone : '') + '">' + value + '</strong></div>';
  return '<div class="gate" aria-label="Apply gate">' +
    row('Files', progress.reviewed + ' / ' + progress.total, ready.filesReviewed ? 'success' : '') +
    row('Checks', esc(checks.label), checks.tone === 'neutral' ? '' : checks.tone) +
    (workflow ? row('Agents', esc(workflow.replace(/_/g, ' ')), workflow === 'awaiting_human_review' ? 'success' : workflowActive(session) ? 'info' : 'warning') : '') +
    row('Apply', session.state === 'applied' ? 'Applied' : ready.canApply ? 'Ready' : ready.canApprove ? 'Approve first' : 'Locked', session.state === 'applied' || ready.canApply ? 'success' : '') +
    '</div>';
}

export function reviewMain(state) {
  const session = state.activeSession;
  if (!session) {
    const loading = state.route.sessionId && state.loaded;
    return '<div class="page">' + (loading ? '<div class="page-inner">' + '<div class="skeleton sk-line"></div>'.repeat(8) + '</div>' : empty('Nothing to review yet', 'Open a task to review its proposed changes, or start a new one.', 'review', button('New task', { variant: 'primary', icon: 'plus', data: { action: 'new-task' } }) + button('All tasks', { data: { action: 'open-page', page: 'tasks' } }))) + '</div>';
  }
  const section = REVIEW_SECTIONS.includes(state.route.reviewSection) ? state.route.reviewSection : 'changes';
  const ready = proposalReadiness(session);
  let html = '<div class="review-head"><div class="review-title">' + button('', { variant: 'ghost', size: 'sm', icon: 'arrowLeft', title: 'All tasks', data: { action: 'open-page', page: 'tasks' } }) + '<h1 title="' + esc(session.title || session.id) + '">' + esc(session.title || session.id) + '</h1>' + taskStateBadge(session) +
    (session._worktreePath ? badge('⎇ ' + (session.git && session.git.branch ? session.git.branch : basename(session._worktreePath)), 'accent') : '') + '</div>' +
    '<div class="row">' + primaryAction(state, ready, section) + '</div></div>';
  html += '<div class="steps" role="tablist" aria-label="Review steps">' + REVIEW_SECTIONS.map((id, i) => stepTab(id, i, session, ready, section)).join('') + '</div>';
  html += '<div class="review-body" id="review-body" role="tabpanel" aria-label="' + esc(SECTION_LABELS[section]) + '">';
  if (section === 'changes') html += changesSection(state, session, ready);
  else if (section === 'checks') html += checksSection(state, session, ready);
  else if (section === 'pipeline') html += pipelineSection(state, session);
  else if (section === 'spec') html += refineSection(state, session, ready);
  else html += applySection(state, session, ready);
  return html + '</div>';
}

function stepTab(id, index, session, ready, current) {
  const checks = checkSummary(session);
  const workflow = session.workflow ? session.workflow.status : '';
  let cls = '';
  let count = '';
  if (id === 'changes') { cls = ready.filesReviewed ? 'done' : asArray(session.changes).length ? 'current' : ''; count = String(asArray(session.changes).length); }
  if (id === 'checks') { cls = checks.failed ? 'failed' : ready.checksPassed ? 'done' : ready.filesReviewed ? 'current' : ''; }
  if (id === 'pipeline') { cls = workflow === 'awaiting_human_review' ? 'done' : workflowActive(session) ? 'current' : ['failed', 'changes_requested'].includes(workflow) ? 'failed' : ''; }
  if (id === 'apply') { cls = session.state === 'applied' ? 'done' : ready.canApprove ? 'current' : ''; }
  const num = cls === 'done' ? icon('check') : cls === 'failed' ? icon('x') : workflowActive(session) && id === 'pipeline' ? '<span class="spinner"></span>' : String(index + 1);
  return '<button class="step-tab ' + cls + '" type="button" role="tab" aria-selected="' + (id === current) + '" data-action="review-section" data-section="' + id + '" tabindex="' + (id === current ? 0 : -1) + '"><span class="step-num">' + num + '</span>' + esc(SECTION_LABELS[id]) + (count ? ' <span class="count">' + count + '</span>' : '') + '</button>';
}

function primaryAction(state, ready, section) {
  const session = state.activeSession;
  const busy = Boolean(state.pending);
  if (session.state === 'applied') return section === 'apply' ? '' : button('Undo…', { variant: 'danger', icon: 'undo', disabled: busy, data: { action: 'review-section', section: 'apply' } });
  if (workflowActive(session)) return button('View agents', { icon: 'agent', data: { action: 'review-section', section: 'pipeline' } });
  if (!asArray(session.changes).length) return button('Generate', { variant: 'primary', icon: 'sparkle', data: { action: 'review-section', section: 'pipeline' } });
  if (!ready.filesReviewed && section === 'changes') {
    const current = state.route.file;
    const canMark = current && !state.review.allFiles && !isFileReviewed(session, current) && asArray(session.changes).some(e => e.path === current);
    return canMark ? button('Mark reviewed', { variant: 'success', icon: 'check', kbd: 'M', disabled: busy || state.review.sinceLastReview, busy: state.pending === 'Saving review…', data: { action: 'review-file', path: current, mutation: '' } }) : button('Next unreviewed', { icon: 'arrowRight', kbd: 'N', data: { action: 'next-unreviewed' } });
  }
  if (!ready.filesReviewed) return button('Continue review', { variant: 'primary', icon: 'review', data: { action: 'review-section', section: 'changes' } });
  if (!ready.checksPassed) return button('Run checks', { variant: 'primary', icon: 'play', disabled: busy, busy: state.pending === 'Running checks…', data: { action: 'run-checks', mutation: '' } });
  if (ready.canApply) return button('Apply…', { variant: 'primary', icon: 'check', disabled: busy, data: { action: 'review-section', section: 'apply' } });
  if (ready.canApprove) return button('Approve review', { variant: 'primary', icon: 'check', disabled: busy, busy: state.pending === 'Approving…', data: { action: 'approve-review', mutation: '' } });
  return '';
}

// ---------------------------------------------------------------- changes

function changesSection(state, session, ready) {
  const edits = effectiveEdits(session, state.review.sinceLastReview);
  const mode = state.review.mode;
  const visible = state.review.allFiles ? edits : edits.filter(edit => edit.path === state.route.file);
  const selected = visible.length ? visible : edits.slice(0, 1);
  let html = '<div class="diff-toolbar"><div class="row">' +
    (state.review.narrow ? '' : '<div class="seg" role="group" aria-label="Diff layout"><button type="button" aria-pressed="' + (mode === 'split') + '" data-action="diff-mode" data-mode="split" title="Side by side (V)">Split</button><button type="button" aria-pressed="' + (mode === 'unified') + '" data-action="diff-mode" data-mode="unified" title="Unified (V)">Unified</button></div>') +
    '<div class="seg" role="group" aria-label="Files shown"><button type="button" aria-pressed="' + !state.review.allFiles + '" data-action="diff-scope" data-all="false">One file</button><button type="button" aria-pressed="' + state.review.allFiles + '" data-action="diff-scope" data-all="true">All files</button></div>' +
    '<div class="seg" role="group" aria-label="Comparison"><button type="button" aria-pressed="' + !state.review.sinceLastReview + '" data-action="diff-since" data-since="false">Full diff</button><button type="button" aria-pressed="' + state.review.sinceLastReview + '" data-action="diff-since" data-since="true"' + (session.last_reviewed_hash ? '' : ' disabled title="Available after a complete review"') + '>Since last review</button></div>' +
    '</div><div class="row">' + button('', { variant: 'ghost', size: 'sm', icon: 'arrowLeft', title: 'Previous file (K)', data: { action: 'file-step', step: -1 } }) + button('', { variant: 'ghost', size: 'sm', icon: 'arrowRight', title: 'Next file (J)', data: { action: 'file-step', step: 1 } }) + '</div></div>';
  if (!edits.length) {
    html += state.review.sinceLastReview ? empty('No changes since your last review', 'Switch to Full diff to inspect the whole proposal.', 'check')
      : workflowActive(session) ? empty('Agents are generating changes', 'The diff appears here as soon as the create stage finishes.', 'agent', button('View agents', { data: { action: 'review-section', section: 'pipeline' } }))
        : empty('No changes generated yet', 'Run the agent workflow or ask for a quick generation.', 'sparkle', button('Start agents', { variant: 'primary', icon: 'agent', data: { action: 'review-section', section: 'pipeline' } }) + button('Quick generate', { data: { action: 'review-section', section: 'spec' } }));
    return html;
  }
  html += '<div class="diff-files">' + selected.map(edit => diffFile(state, session, edit, ready)).join('') + '</div>';
  return html;
}

function diffFile(state, session, edit, ready) {
  const stats = diffStats(edit);
  const reviewed = isFileReviewed(session, edit.path);
  const editable = asArray(session.changes).some(e => e.path === edit.path);
  const canMark = ready.editable && editable && !reviewed && !state.review.sinceLastReview;
  const lines = asArray(edit.diff && edit.diff.lines);
  const expanded = new Set(asArray(state.review.expandedGaps[edit.path]));
  const segments = buildSegments(lines, 3, expanded);
  // Narrow screens always use the unified layout; split columns would be unreadable.
  const table = state.review.mode === 'split' && !state.review.narrow ? splitTable(edit.path, lines, segments) : unifiedTable(edit.path, lines, segments);
  const pinned = !editable;
  return '<article class="diff-file' + (reviewed ? ' reviewed' : '') + '" aria-label="' + esc(edit.path) + '"><header class="diff-file-head"><div class="diff-file-title">' + icon('file', 'ficon ' + fileIconClass(edit.path)) + (dirname(edit.path) ? '<span class="path dir">' + esc(dirname(edit.path)) + '/</span>' : '') + '<span class="path">' + esc(basename(edit.path)) + '</span><span class="diffstat"><span class="add">+' + stats.added + '</span><span class="del">−' + stats.removed + '</span></span>' +
    (pinned ? badge('pinned test') : reviewed ? badge('Reviewed', 'success', true) : badge('Not reviewed', 'warning', true)) + '</div><div class="row">' +
    button('Source', { variant: 'ghost', size: 'sm', icon: 'external', title: 'Open the current source in the Explorer', data: { action: 'open-file', path: edit.path } }) +
    (pinned ? '' : reviewed ? button('Reviewed', { size: 'sm', icon: 'check', disabled: true }) : button('Mark reviewed', { size: 'sm', variant: 'success', icon: 'check', kbd: 'M', disabled: !canMark || Boolean(state.pending), title: state.review.sinceLastReview ? 'Switch to Full diff to mark files reviewed' : 'Record that you inspected this file at the current version', data: { action: 'review-file', path: edit.path, mutation: '' } })) +
    '</div></header>' +
    (edit.summary ? '<div class="diff-summary">' + icon('sparkle') + ' <strong>Agent note</strong> <span class="faint">· AI suggestion</span><br>' + esc(edit.summary) + '</div>' : '') +
    '<div class="diff-scroll">' + (lines.length ? table : '<div class="card-body faint small">No line differences.</div>') + '</div></article>';
}

function gapRow(path, segment, cols) {
  return '<tr class="gap"><td colspan="' + cols + '"><button class="gap-btn" type="button" data-action="expand-gap" data-path="' + esc(path) + '" data-start="' + segment.start + '">' + icon('expand') + 'Show ' + plural(segment.count, 'unchanged line') + '</button></td></tr>';
}

function unifiedTable(path, lines, segments) {
  let rows = '';
  for (const segment of segments) {
    if (segment.type === 'gap') { rows += gapRow(path, segment, 4); continue; }
    for (let i = segment.start; i < segment.end; i++) {
      const line = lines[i];
      const cls = line.kind === 'added' ? 'add' : line.kind === 'removed' ? 'del' : '';
      rows += '<tr' + (cls ? ' class="' + cls + '"' : '') + '><td class="ln">' + (line.old_line || '') + '</td><td class="ln">' + (line.new_line || '') + '</td><td class="sign">' + (cls === 'add' ? '+' : cls === 'del' ? '−' : '') + '</td><td>' + esc(line.text) + '</td></tr>';
    }
  }
  return '<table class="diff unified"><tbody>' + rows + '</tbody></table>';
}

function splitTable(path, lines, segments) {
  let rows = '';
  for (const segment of segments) {
    if (segment.type === 'gap') {
      rows += '<div class="srow sgap"><button class="gap-btn" type="button" data-action="expand-gap" data-path="' + esc(path) + '" data-start="' + segment.start + '">' + icon('expand') + 'Show ' + plural(segment.count, 'unchanged line') + '</button></div>';
      continue;
    }
    for (const pair of splitRows(lines.slice(segment.start, segment.end))) {
      const l = pair.left;
      const r = pair.right;
      const lc = !l ? 'empty' : l.kind === 'removed' ? 'del' : '';
      const rc = !r ? 'empty' : r.kind === 'added' ? 'add' : '';
      rows += '<div class="srow"><span class="sln ' + lc + '">' + (l && l.old_line ? l.old_line : '') + '</span><span class="scode ' + lc + '">' + (l ? esc(l.text) : '') + '</span><span class="sln ' + rc + '">' + (r && r.new_line ? r.new_line : '') + '</span><span class="scode ' + rc + '">' + (r ? esc(r.text) : '') + '</span></div>';
    }
  }
  return '<div class="sdiff" role="table" aria-label="Side-by-side diff">' + rows + '</div>';
}

// ---------------------------------------------------------------- checks

export function phaseList(items, filter) {
  const shown = items.filter(item => filter === 'all' || normalizePhaseState(item.state) === filter);
  if (!items.length) return '<p class="faint small">No results recorded yet.</p>';
  if (!shown.length) return '<p class="faint small">No checks match this filter.</p>';
  return shown.map(item => {
    const tone = normalizePhaseState(item.state);
    return '<details class="phase"' + (tone === 'failed' && item.output ? ' open' : '') + '><summary><span class="chev"></span>' + stateIcon(tone) + '<span class="grow truncate"><strong>' + esc(item.name) + '</strong>' + (item.required === false ? ' <span class="faint xs">optional</span>' : '') + '</span>' +
      (item.meta ? '<span class="faint xs mono">' + esc(item.meta) + '</span>' : '') + badge(item.state || 'pending', toneForState(tone)) + '</summary><div class="phase-body">' +
      (item.command ? '<div class="path">$ ' + esc(item.command) + '</div>' : '') + (item.detail ? '<p class="muted small">' + esc(item.detail) + '</p>' : '') +
      (item.output ? '<pre class="pre">' + esc(item.output) + '</pre>' : '<p class="faint small">No diagnostic output recorded.</p>') + '</div></details>';
  }).join('');
}

export function phaseFilter(filter, action = 'checks-filter') {
  return '<div class="seg" role="group" aria-label="Filter results">' + ['all', 'failed', 'passed'].map(value => '<button type="button" aria-pressed="' + (filter === value) + '" data-action="' + action + '" data-filter="' + value + '">' + value[0].toUpperCase() + value.slice(1) + '</button>').join('') + '</div>';
}

function checksSection(state, session, ready) {
  const trusted = Boolean(state.trust && state.trust.trusted);
  const checks = asArray(session.checks).map(check => ({ name: check.name, state: check.state, required: check.required, command: asArray(check.command).join(' '), meta: [formatDuration(check.duration_ms), typeof check.exit_code === 'number' && check.exit_code ? 'exit ' + check.exit_code : ''].filter(Boolean).join(' · '), output: check.output }));
  const summary = checkSummary(session);
  let html = '<div class="page-inner page-narrow stack">';
  html += '<div class="row-between"><div><h2>Verification</h2><p class="muted small">Formatting, vet, lint, architecture gates and tests run in an isolated copy of the checkout.</p></div><div class="row">' + phaseFilter(state.checks.filter) +
    button(checks.length ? 'Re-run checks' : 'Run checks', { variant: 'primary', icon: 'play', disabled: !ready.editable || !asArray(session.changes).length || Boolean(state.pending), busy: state.pending === 'Running checks…', data: { action: 'run-checks', mutation: '' } }) + '</div></div>';
  if (!trusted) html += callout('warning', 'Tests and vet run project code and need execution trust. Static checks still run without it.', button('Open Settings', { size: 'sm', data: { action: 'open-page', page: 'settings' } }));
  if (summary.failed) html += callout('danger', plural(summary.failed, 'check') + ' failed. Fix the code with Refine or let agents repair it, then re-run.', button('Refine', { size: 'sm', data: { action: 'review-section', section: 'spec' } }));
  else if (ready.checksPassed) html += callout('success', 'Required checks passed for this version.' + (ready.filesReviewed ? ' Approve the review on the Apply step.' : ''), ready.filesReviewed ? button('Go to Apply', { size: 'sm', data: { action: 'review-section', section: 'apply' } }) : '');
  html += '<div>' + phaseList(checks, state.checks.filter) + '</div>';
  if (asArray(session.scope_suggestions).length) html += callout('info', 'Checks suggest widening the scope: <span class="mono">' + esc(session.scope_suggestions.join(', ')) + '</span>. Create a new task to include them.');
  return html + '</div>';
}

// ---------------------------------------------------------------- agents

function pipelineSection(state, session) {
  const workflow = session.workflow;
  const trusted = Boolean(state.trust && state.trust.trusted);
  const blocker = workflowBlocker(session, trusted);
  let html = '<div class="page-inner page-narrow stack-lg">';
  if (workflow) {
    const stages = workflowStages(workflow);
    const active = workflowActive(session);
    html += '<div class="row-between"><div><h2>Agent workflow</h2><p class="muted small">Run ' + esc(shortHash(workflow.id, 10)) + (workflow.attempt ? ' · attempt ' + workflow.attempt : '') + (workflow.auto_repair ? ' · auto-repair on' : '') + '</p></div><div class="row">' + badge(String(workflow.status || '').replace(/_/g, ' '), active ? 'info' : workflow.status === 'awaiting_human_review' ? 'success' : 'warning', active) +
      (active ? button('Cancel', { variant: 'danger', size: 'sm', icon: 'stop', disabled: Boolean(state.pending), data: { action: 'cancel-workflow', mutation: '' } }) : '') + '</div></div>';
    if (workflow.reason) html += callout(['failed', 'interrupted', 'stale', 'outdated'].includes(workflow.status) ? 'danger' : 'info', esc(workflow.reason));
    html += '<section class="card card-pad"><div class="timeline">' + stages.map(stage => {
      const waiting = stage.name === 'human_review' && ['waiting', 'pending'].includes(stage.status) && workflow.status === 'awaiting_human_review';
      const detail = waiting ? 'Waiting for your review and approval' : stage.reason || (stage.tone === 'running' ? 'In progress…' : String(stage.status).replace(/_/g, ' '));
      return '<div class="tl-item">' + (waiting ? '<span class="state-ic attention">' + icon('shield') + '</span>' : stateIcon(stage.tone)) + '<div><strong>' + esc(stage.label) + '</strong><div class="muted small">' + esc(detail) + '</div>' + (stage.model ? '<div class="faint xs mono">' + esc(stage.model) + '</div>' : '') + '</div><span class="faint xs mono">' + esc(formatDuration(stage.durationMs)) + '</span></div>';
    }).join('') + '</div></section>';
    const specialists = workflowSpecialists(workflow);
    if (specialists.length) {
      html += '<section class="stack"><h3>Specialist reviews <span class="faint xs">· AI suggestions, not approval</span></h3><div class="specialists">' + specialists.map(sp => '<div class="card card-pad stack"><div class="row-between"><strong>' + esc(sp.label) + '</strong>' + badge(sp.verdict || sp.status, toneForState(sp.tone)) + '</div>' +
        (sp.summary ? '<p class="small">' + esc(sp.summary) + '</p>' : '<p class="faint small">' + esc(sp.reason || 'No report yet.') + '</p>') +
        (sp.findings.length ? '<ul class="bullets small">' + sp.findings.slice(0, 6).map(f => '<li>' + esc(f) + '</li>').join('') + '</ul>' : '') + (sp.evidence ? '<span class="faint xs">' + esc(sp.evidence) + '</span>' : '') + '</div>').join('') + '</div></section>';
    } else if (workflow.review) {
      html += '<section class="card card-pad stack"><div class="row-between"><strong>Model review</strong>' + badge(workflow.review.verdict || 'review', workflow.review.verdict === 'approve' ? 'success' : 'warning') + '</div><p class="small">' + esc(workflow.review.summary || '') + '</p>' + (asArray(workflow.review.findings).length ? '<ul class="bullets small">' + workflow.review.findings.map(f => '<li>' + esc(f) + '</li>').join('') + '</ul>' : '') + '</section>';
    }
    const input = workflow.input;
    if (input && input.status === 'waiting') {
      const ready = proposalReadiness(session);
      html += '<section class="card card-pad stack"><div class="row-between"><strong>' + icon('shield') + ' Your decision is needed</strong><span class="faint xs">expires ' + esc(String(input.expires_at || '').slice(0, 16).replace('T', ' ')) + '</span></div><p class="muted small">Agents approved this version. Approving records your human review for this exact hash; Apply stays a separate step.</p>' +
        (ready.canApprove ? '' : callout('warning', 'Review every file and pass required checks before approving.')) +
        '<div class="row">' + button('Approve', { variant: 'success', icon: 'check', disabled: !ready.canApprove || Boolean(state.pending), data: { action: 'workflow-input', decision: 'approve', mutation: '' } }) + button('Reject', { variant: 'danger', icon: 'x', disabled: Boolean(state.pending), data: { action: 'workflow-input', decision: 'reject', mutation: '' } }) + '</div></section>';
    }
    if (asArray(workflow.harness_lessons).length) html += '<details class="disclosure"><summary>Harness lessons applied</summary><div class="disclosure-body"><ul class="bullets small">' + workflow.harness_lessons.map(l => '<li>' + esc(l) + '</li>').join('') + '</ul></div></details>';
    if (active) return html + '</div>';
    html += '<div class="divider"></div><h3>Run again</h3>';
  } else {
    html += '<div><h2>Agent workflow</h2><p class="muted small">A native ADK graph: <strong>create</strong> the change, <strong>write and run tests</strong> in an isolated workspace, then fan out to <strong>specialist reviewers</strong>. You review every file before Apply.</p></div>';
    html += '<div class="pipeline" data-pipe="4">' + [['agent', 'Create', 'Implements the task'], ['checks', 'Tests', 'Writes & runs tests'], ['review', 'Specialists', 'Correctness, architecture, test quality'], ['shield', 'You', 'Review, approve, apply']].map(([ic, label, det]) => '<div class="pipe-step"><span class="num">' + icon(ic) + '</span><span class="lbl">' + label + '</span><span class="det">' + det + '</span></div>').join('') + '</div>';
  }
  if (blocker) html += callout('warning', esc(blocker), /trust/i.test(blocker) ? button('Open Settings', { size: 'sm', data: { action: 'open-page', page: 'settings' } }) : '');
  html += '<label class="field"><span class="field-label">Instructions for the agents</span><textarea class="textarea" rows="5" data-input="workflow-message" data-focus-key="workflow-message" placeholder="What should the agents build or fix?">' + esc(state.review.workflowMessage ?? (session.plan && session.plan.approach ? session.title + '\n\n' + session.plan.approach : session.title || '')) + '</textarea></label>' +
    '<label class="checkbox"><input type="checkbox" data-input="workflow-repair"' + (state.review.autoRepair ? ' checked' : '') + '><span><strong>Allow automatic repair</strong><br><span class="muted small">Up to two retries when checks fail or a reviewer requests changes.</span></span></label>' +
    '<div class="row">' + button('Start agents', { variant: 'primary', icon: 'agent', disabled: Boolean(blocker) || Boolean(state.pending), busy: state.pending === 'Starting agents…', data: { action: 'start-workflow', mutation: '' } }) + '</div>';
  return html + '</div>';
}

// ---------------------------------------------------------------- refine

function refineSection(state, session, ready) {
  const criteria = asArray(session.acceptance_criteria);
  const targets = asArray(session.targets);
  const messages = asArray(session.messages);
  const presets = [['Fix failing checks', 'Fix the failing checks while preserving existing behavior.'], ['Add edge-case tests', 'Add unit tests for edge cases and failure paths.'], ['Simplify', 'Simplify control flow while preserving behavior and passing tests.'], ['Improve errors', 'Return errors with useful context instead of swallowing them.']];
  let html = '<div class="page-inner page-narrow stack-lg"><div><h2>Refine with one instruction</h2><p class="muted small">Quick generation sends one focused request and replaces the proposal. Previous review marks and checks are cleared for the new version.</p></div>';
  html += '<div class="stack"><textarea class="textarea" rows="5" data-input="refine-message" data-focus-key="refine-message" placeholder="Describe what to improve or fix…">' + esc(state.review.refineMessage || '') + '</textarea>' +
    '<div class="chips">' + presets.map(([label, text]) => '<button class="chip" type="button" data-action="refine-preset" data-text="' + esc(text) + '">' + icon('wand') + '<span>' + esc(label) + '</span></button>').join('') + '</div>' +
    '<div class="row">' + button(asArray(session.changes).length ? 'Update proposal' : 'Generate proposal', { variant: 'primary', icon: 'sparkle', disabled: !ready.editable || workflowActive(session) || Boolean(state.pending), busy: state.pending === 'Generating…', data: { action: 'refine', mutation: '' } }) + '</div></div>';
  html += '<section class="card"><div class="card-head"><span class="card-title">Task scope</span>' + badge(plural(targets.length, 'file')) + '</div><div class="card-body stack">' +
    (targets.length ? '<div class="chips">' + targets.map(t => '<button class="chip" type="button" data-action="open-file" data-path="' + esc(t.path) + '">' + icon('file') + '<span>' + esc(t.path) + (t.exists === false ? ' (new)' : '') + '</span></button>').join('') + '</div>' : '<p class="faint small">No target files captured.</p>') +
    '<div><div class="field-label">Acceptance criteria</div>' + (criteria.length ? '<ul class="bullets small">' + criteria.map(c => '<li>' + esc(c) + '</li>').join('') + '</ul>' : '<p class="faint small">None recorded.</p>') + '</div></div></section>';
  if (messages.length) {
    html += '<details class="disclosure"><summary>Conversation <span class="count">' + messages.length + '</span></summary><div class="disclosure-body stack">' + messages.map(m => '<div class="card card-pad stack"><div class="row-between">' + badge(m.role || 'message', m.role === 'user' ? 'info' : 'accent') + '<span class="faint xs">' + esc(String(m.created_at || '').slice(0, 19).replace('T', ' ')) + '</span></div><div class="small pre-line">' + esc(m.content || '') + '</div></div>').join('') + '</div></details>';
  }
  return html + '</div>';
}

// ---------------------------------------------------------------- apply

function applySection(state, session, ready) {
  const progress = reviewProgress(session);
  const checks = checkSummary(session);
  const busy = Boolean(state.pending);
  const destination = session._worktreePath || (state.project && state.project.path) || 'current checkout';
  let html = '<div class="page-inner page-narrow stack-lg">';
  if (session.state === 'applied') {
    html += '<div><h2>Applied</h2><p class="muted small">These changes are in ' + esc(destination) + '. Undo restores the previous files after verifying nothing else changed them.</p></div>';
    if (session.commit) html += commitInfo(session.commit);
    html += '<div class="row">' + button('Undo changes', { variant: 'danger', icon: 'undo', disabled: busy, busy: state.pending === 'Undoing…', data: { action: 'undo', mutation: '' } }) + (session._worktreePath ? button('Merge from Git', { icon: 'merge', data: { action: 'open-page', page: 'git' } }) : '') + '</div>';
    return html + '</div>';
  }
  html += '<div><h2>Ready to apply?</h2><p class="muted small">Apply writes the reviewed version to <span class="path">' + esc(destination) + '</span> with a recovery journal.</p></div>';
  const item = (done, failed, title, sub, section) => '<' + (section ? 'button type="button" data-action="review-section" data-section="' + section + '"' : 'div') + ' class="check-item' + (done ? ' done' : failed ? ' failed' : '') + '"><span class="ic">' + icon(done ? 'check' : failed ? 'x' : 'dot') + '</span><span class="grow"><strong>' + title + '</strong><div class="sub">' + sub + '</div></span>' + (section ? '<span class="chev-right"></span>' : '') + '</' + (section ? 'button' : 'div') + '>';
  html += '<div class="checklist">' + item(ready.filesReviewed, false, 'Review every changed file', progress.reviewed + ' of ' + progress.total + ' reviewed', 'changes') +
    item(ready.checksPassed, checks.failed > 0, 'Pass required checks', esc(checks.label), 'checks') +
    (session.workflow ? item(session.workflow.status === 'awaiting_human_review', ['failed', 'changes_requested', 'canceled'].includes(session.workflow.status), 'Agent workflow finished', esc(String(session.workflow.status).replace(/_/g, ' ')), 'pipeline') : '') +
    item(ready.canApply, false, 'Approve this exact version', ready.canApply ? 'Approved at #' + esc(shortHash(session.hash, 8)) : 'Records your review for hash #' + esc(shortHash(session.hash, 8)), '') + '</div>';
  if (!ready.editable) html += callout('warning', esc(ready.message));
  const gitCapable = Boolean(session.git && session.git.head);
  if (gitCapable) {
    const message = state.review.commitMessage ?? defaultCommitMessage(session);
    html += '<section class="card card-pad stack"><label class="checkbox"><input type="checkbox" data-input="commit-toggle"' + (state.review.commit ? ' checked' : '') + '><span><strong>Commit after applying</strong><br><span class="muted small">Commits only the reviewed files on <span class="mono">' + esc(session.git.branch || 'the current branch') + '</span> through a private index. No hooks, no push.</span></span></label>' +
      (state.review.commit ? '<label class="field"><span class="field-label">Commit message</span><textarea class="textarea" rows="3" data-input="commit-message" data-focus-key="commit-message">' + esc(message) + '</textarea></label>' : '') + '</section>';
  }
  html += '<div class="row">' + (ready.canApply ? button(state.review.commit && gitCapable ? 'Apply & commit' : 'Apply changes', { variant: 'primary', size: 'lg', icon: 'check', disabled: busy, busy: state.pending === 'Applying…', data: { action: 'apply', mutation: '' } })
    : button('Approve review', { variant: 'primary', size: 'lg', icon: 'check', disabled: !ready.canApprove || busy, busy: state.pending === 'Approving…', title: ready.canApprove ? 'Record approval for this version' : ready.message, data: { action: 'approve-review', mutation: '' } })) + '</div>';
  html += '<p class="faint small" role="status">' + esc(ready.message) + '</p>';
  return html + '</div>';
}

function commitInfo(commit) {
  const tone = commit.state === 'committed' ? 'success' : commit.state === 'failed' ? 'danger' : 'info';
  return callout(tone, '<strong>Commit ' + esc(commit.state) + '</strong>' + (commit.oid ? ' · <span class="mono">' + esc(shortHash(commit.oid, 10)) + '</span>' : '') + (commit.message ? '<br><span class="small">' + esc(commit.message.split('\n')[0]) + '</span>' : '') + (commit.error ? '<br><span class="small">' + esc(commit.error) + '</span>' : ''));
}

export function defaultCommitMessage(session) {
  const prefix = { fix: 'fix', feature: 'feat', refactor: 'refactor', performance: 'perf', security: 'fix' }[session.kind] || 'chore';
  return prefix + ': ' + String(session.title || session.id).replace(/^fix:\s*/i, '');
}
