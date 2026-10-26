// Issues: filters on the side, findings list with a detail pane.

import { asArray, esc, plural, severityRank, severityTone } from '../core/util.js';
import { rankItems } from '../core/fuzzy.js';
import { icon } from '../ui/icons.js';
import { badge, button, callout, empty } from '../ui/dom.js';
import { findingIsOpen, findingLine, findingPath, severityBadge, sourceLabel } from './shared.js';

const CATEGORIES = [['all', 'All categories'], ['bugs', 'Bugs'], ['security', 'Security'], ['performance', 'Performance']];
const STATUSES = [['open', 'Open'], ['dismissed', 'Dismissed'], ['fixed', 'Fixed'], ['all', 'All']];
const SEVERITIES = [['all', 'Any severity'], ['critical', 'Critical'], ['high', 'High'], ['medium', 'Medium'], ['low', 'Low']];

export function filteredFindings(state) {
  const f = state.issues;
  const list = asArray(state.findings).filter(finding => {
    const status = String(finding.status || 'open');
    if (f.status === 'open' && !findingIsOpen(finding)) return false;
    if (f.status !== 'open' && f.status !== 'all' && status !== f.status) return false;
    if (f.category !== 'all' && String(finding.category || 'bugs') !== f.category) return false;
    if (f.severity !== 'all' && String(finding.severity || '').toLowerCase() !== f.severity) return false;
    return true;
  });
  const ranked = f.q.trim() ? rankItems(list, f.q, x => (x.title || '') + ' ' + findingPath(x) + ' ' + (x.message || ''), 500).map(r => r.item) : list;
  return f.q.trim() ? ranked : ranked.sort((a, b) => severityRank(a.severity) - severityRank(b.severity) || findingPath(a).localeCompare(findingPath(b)));
}

export function issuesSide(state) {
  const all = asArray(state.findings);
  const count = pred => all.filter(pred).length;
  const group = (label, key, options, counter) => '<div class="nav-group-label">' + label + '</div><div class="nav-list">' + options.map(([value, text]) => '<button class="nav-item" type="button" aria-current="' + (state.issues[key] === value) + '" data-action="issues-filter" data-key="' + key + '" data-value="' + value + '"><span>' + esc(text) + '</span>' + (counter ? '<span class="count">' + counter(value) + '</span>' : '') + '</button>').join('') + '</div>';
  return '<div class="side-head"><span class="side-title">Issues</span></div><div class="side-scroll">' +
    group('Status', 'status', STATUSES, value => count(f => value === 'all' || (value === 'open' ? findingIsOpen(f) : String(f.status || 'open') === value))) +
    group('Category', 'category', CATEGORIES, value => count(f => findingIsOpen(f) && (value === 'all' || String(f.category || 'bugs') === value))) +
    group('Severity', 'severity', SEVERITIES, value => count(f => findingIsOpen(f) && (value === 'all' || String(f.severity || '').toLowerCase() === value))) + '</div>';
}

export function issuesMain(state) {
  const list = filteredFindings(state);
  const selected = list.find(f => f.id === state.issues.selected) || list[0] || null;
  let left = '<div class="page-inner"><header class="page-head"><div><h1 class="page-title">Issues</h1><p class="page-sub">Verified tool findings and AI suggestions stay labeled separately. Turn any finding into a focused fix task.</p></div>' + button('Run analysis', { icon: 'play', disabled: Boolean(state.pending), data: { action: 'run-analysis', mutation: '' } }) + '</header>' +
    '<div class="search-wrap"><span class="search-icon"></span><input class="input" type="search" data-input="issues-q" data-focus-key="issues-q" value="' + esc(state.issues.q) + '" placeholder="Search ' + plural(list.length, 'issue') + '…" aria-label="Search issues"></div><div class="divider"></div>';
  if (!asArray(state.findings).length) {
    left += empty('No findings yet', 'Run a Go scan or a project analysis to surface bugs, performance and security issues.', 'issues', button('Run Go scan', { data: { action: 'open-page', page: 'checks' } }) + button('Run analysis', { variant: 'primary', data: { action: 'run-analysis', mutation: '' } }));
    return '<div class="page">' + left + '</div></div>';
  }
  left += list.length ? '<div class="card"><div class="list" role="listbox" aria-label="Issues">' + list.map(f => '<button class="list-row' + (selected && f.id === selected.id ? ' active' : '') + '" type="button" role="option" aria-selected="' + Boolean(selected && f.id === selected.id) + '" data-action="issue-select" data-id="' + esc(f.id) + '"><span class="sev ' + severityTone(f.severity) + '" aria-hidden="true"></span><div class="grow"><div class="title">' + esc(f.title || f.id) + '</div><div class="sub mono">' + esc(findingPath(f) || 'workspace') + (findingLine(f) ? ':' + findingLine(f) : '') + '</div></div><div class="end">' + (f.confidence === 'suggested' ? badge('AI', 'accent') : '') + severityBadge(f.severity) + '</div></button>').join('') + '</div></div>'
    : empty('No issues match', 'Adjust the filters on the left.', 'filter');
  left += '</div>';
  return '<div class="split"><div>' + left + '</div><aside class="split-detail" aria-label="Issue detail">' + (selected ? issueDetail(state, selected) : empty('Select an issue', '', 'issues')) + '</aside></div>';
}

function issueDetail(state, f) {
  const path = findingPath(f);
  const line = findingLine(f);
  const open = findingIsOpen(f);
  const insight = f.engineering_insight;
  let html = '<div class="page-inner stack-lg"><div class="stack"><div class="row">' + severityBadge(f.severity) + badge(f.category || 'bugs') + badge(sourceLabel(f), f.confidence === 'suggested' ? 'accent' : 'info') + (f.freshness === 'stale' ? badge('stale', 'warning') : '') + '</div><h2>' + esc(f.title || f.id) + '</h2>' +
    (path ? '<button class="chip" type="button" data-action="open-file" data-path="' + esc(path) + '" data-line="' + (line || '') + '">' + icon('file') + '<span>' + esc(path) + (line ? ':' + line : '') + '</span></button>' : '') + '</div>';
  if (f.message) html += '<p>' + esc(f.message) + '</p>';
  if (f.evidence) html += '<div><div class="field-label">Evidence</div><pre class="pre">' + esc(f.evidence) + '</pre></div>';
  if (f.rule) html += '<dl class="kv"><dt>Rule</dt><dd class="mono">' + esc(f.rule) + '</dd>' + (f.originating_analysis ? '<dt>Origin</dt><dd>' + esc(f.originating_analysis) + '</dd>' : '') + '</dl>';
  if (insight) {
    html += '<section class="card card-pad stack"><strong>' + icon('sparkle') + ' Engineering insight <span class="faint xs">· AI suggestion</span></strong><dl class="kv">' +
      (insight.mechanism ? '<dt>Mechanism</dt><dd>' + esc(insight.mechanism) + '</dd>' : '') + (insight.why_it_matters_here ? '<dt>Why here</dt><dd>' + esc(insight.why_it_matters_here) + '</dd>' : '') +
      (insight.tradeoff_or_failure_mode ? '<dt>Trade-off</dt><dd>' + esc(insight.tradeoff_or_failure_mode) + '</dd>' : '') + (insight.transferable_lesson ? '<dt>Lesson</dt><dd>' + esc(insight.transferable_lesson) + '</dd>' : '') + '</dl></section>';
  }
  if (f.task_spec && asArray(f.task_spec.acceptance_criteria).length) html += '<div><div class="field-label">Suggested acceptance criteria</div><ul class="bullets small">' + f.task_spec.acceptance_criteria.map(c => '<li>' + esc(c) + '</li>').join('') + '</ul></div>';
  if (f.freshness === 'stale') html += callout('warning', 'The file changed since this finding was recorded. Re-run analysis to confirm it still applies.');
  html += '<div class="row">' + button('Create fix task', { variant: 'primary', icon: 'plus', data: { action: 'issue-fix', id: f.id } }) + (path ? button('Open file', { icon: 'files', data: { action: 'open-file', path, line: line || '' } }) : '') +
    button(open ? 'Dismiss' : 'Reopen', { variant: 'ghost', disabled: Boolean(state.pending), data: { action: 'issue-triage', id: f.id, status: open ? 'dismissed' : 'open', mutation: '' } }) + '</div>';
  return html + '</div>';
}
