// Explorer: folder tree on the side, tabbed read-only source viewer in main.

import { asArray, basename, dirname, esc, plural, revisionLabel } from '../core/util.js';
import { rankItems, highlight } from '../core/fuzzy.js';
import { buildTree, visibleRows } from '../core/tree.js';
import { fileIconClass, icon } from '../ui/icons.js';
import { button, badge, empty } from '../ui/dom.js';
import { findingIsOpen, findingLine, findingPath, severityBadge, sourceLabel } from './shared.js';

let treeCache = { index: null, tree: null };

function fileTree(index) {
  if (treeCache.index !== index) treeCache = { index, tree: buildTree(asArray(index && index.files)) };
  return treeCache.tree;
}

export function explorerSide(state) {
  const files = asArray(state.index && state.index.files);
  const filter = state.explorer.filter;
  const findingCounts = {};
  for (const f of asArray(state.findings)) if (findingIsOpen(f)) findingCounts[findingPath(f)] = (findingCounts[findingPath(f)] || 0) + 1;
  let rows = '';
  if (!files.length) {
    rows = state.loaded ? '<div class="side-section muted small">No indexed files. Reindex the project from Settings.</div>' : '<div class="skeleton sk-line"></div>'.repeat(8);
  } else if (filter.trim()) {
    const ranked = rankItems(files, filter, f => f.path, 200);
    rows = ranked.length ? '<div class="tree" role="listbox" aria-label="Matching files">' + ranked.map(({ item, positions }) => fileRow(item.path, flatLabel(item.path, positions), 0, state, findingCounts, true)).join('') + '</div>' : '<div class="side-section muted small">No files match “' + esc(filter) + '”.</div>';
  } else {
    const visible = visibleRows(fileTree(state.index), state.explorer.expanded);
    rows = '<div class="tree" role="tree" aria-label="Project files">' + visible.map(row => row.type === 'dir'
      ? '<button class="tree-row" type="button" role="treeitem" aria-expanded="' + row.open + '" data-action="toggle-dir" data-path="' + esc(row.path) + '" data-indent="' + row.depth + '"><span class="chev"></span>' + icon('folder', 'ficon') + '<span class="label">' + esc(row.label) + '</span><span class="meta">' + row.count + '</span></button>'
      : fileRow(row.path, esc(row.label), row.depth, state, findingCounts, false)).join('') + '</div>';
  }
  return '<div class="side-head"><span class="side-title">Explorer</span><div class="side-actions"><button class="icon-btn" type="button" data-action="collapse-tree" title="Collapse folders" aria-label="Collapse all folders">' + icon('layers') + '</button><button class="icon-btn" type="button" data-action="reindex" title="Reindex files" aria-label="Reindex files">' + icon('refresh') + '</button></div></div>' +
    '<div class="side-section"><div class="search-wrap"><span class="search-icon"></span><input class="input input-sm" type="search" data-input="explorer-filter" data-focus-key="explorer-filter" value="' + esc(filter) + '" placeholder="Filter ' + plural(files.length, 'file') + '…" aria-label="Filter files"></div></div>' +
    '<div class="side-scroll">' + rows + '</div>';
}

/** Basename first (highlighted), folder after it in a quieter tone. */
function flatLabel(path, positions) {
  const start = path.lastIndexOf('/') + 1;
  const name = highlight(path.slice(start), positions.filter(p => p >= start).map(p => p - start), esc);
  const dir = start ? ' <span class="faint xs">' + highlight(path.slice(0, start - 1), positions.filter(p => p < start - 1), esc) + '</span>' : '';
  return name + dir;
}

function fileRow(path, labelHTML, depth, state, findingCounts, flat) {
  const active = state.route.file === path;
  const issues = findingCounts[path] || 0;
  return '<button class="tree-row' + (active ? ' active' : '') + '" type="button" role="' + (flat ? 'option' : 'treeitem') + '" aria-selected="' + active + '" data-action="open-file" data-path="' + esc(path) + '" data-indent="' + depth + '" title="' + esc(path) + '"><span class="chev leaf" aria-hidden="true"></span>' + icon('file', 'ficon ' + fileIconClass(path)) + '<span class="label">' + labelHTML + '</span>' + (issues ? '<span class="status-dot issue" title="' + plural(issues, 'open issue') + '"></span>' : '') + '</button>';
}

export function explorerMain(state) {
  const explorer = state.explorer;
  const tabs = explorer.tabs;
  const current = state.route.file;
  let html = '<div class="editor">';
  if (tabs.length) {
    html += '<div class="tabs" role="tablist" aria-label="Open files">' + tabs.map(path => '<div class="tab" role="tab" aria-selected="' + (path === current) + '" data-action="open-file" data-path="' + esc(path) + '" tabindex="' + (path === current ? 0 : -1) + '" title="' + esc(path) + '">' + icon('file', 'ficon ' + fileIconClass(path)) + '<span class="tab-label">' + esc(basename(path)) + '</span><button class="tab-close" type="button" data-action="close-tab" data-path="' + esc(path) + '" aria-label="Close ' + esc(basename(path)) + '">' + icon('x') + '</button></div>').join('') + '</div>';
  }
  if (!current) {
    html += '<div class="editor-body">' + empty('Open a file', 'Pick a file in the Explorer or press ⌘P to search. Source is read-only here; changes go through a reviewed task.', 'files', button('Search files', { icon: 'search', kbd: '⌘P', data: { action: 'palette-files' } })) + '</div>';
    return html + '</div>';
  }
  const info = explorer.info && explorer.info.path === current ? explorer.info : null;
  if (!info) {
    html += '<div class="editor-body">' + (explorer.loadingPath === current ? '<div class="skeleton sk-line"></div>'.repeat(14) : empty('File unavailable', 'This file could not be read. It may have been removed or excluded from the index.', 'alert')) + '</div>';
    return html + '</div>';
  }
  const lines = String(info.content || '').split('\n');
  const findings = asArray(state.findings).filter(f => findingIsOpen(f) && findingPath(f) === info.path);
  const findingLines = new Set(findings.map(findingLine).filter(Boolean));
  html += '<div class="toolbar"><div class="toolbar-title">' + (dirname(info.path) ? '<span class="path faint">' + esc(dirname(info.path)) + '/</span>' : '') + '<span class="path">' + esc(basename(info.path)) + '</span>' + badge(info.language || 'text') + '<span class="faint xs">' + plural(lines.length, 'line') + (info.content_hash ? ' · #' + esc(revisionLabel(info.content_hash, 8)) : '') + '</span></div>' +
    '<div class="row"><label class="sr-only" for="goto-line">Go to line</label><input id="goto-line" class="input input-sm goto" type="number" min="1" max="' + lines.length + '" placeholder="Line" data-input="goto-line" data-focus-key="goto-line" aria-keyshortcuts="Control+G">' +
    button('Explain', { size: 'sm', icon: 'sparkle', disabled: Boolean(state.pending), busy: state.pending === 'Explaining…', title: 'Ask the model to explain this file (sends bounded context to the configured provider)', data: { action: 'explain-file', mutation: '' } }) +
    button('New task', { size: 'sm', variant: 'primary', icon: 'plus', title: 'Create a task targeting this file', data: { action: 'task-from-file', path: info.path } }) + '</div></div>';
  html += '<div class="editor-body" id="editor-body">' + insights(state, info, findings);
  html += '<table class="code" aria-label="Source of ' + esc(info.path) + '"><tbody>' + lines.map((line, i) => '<tr id="L' + (i + 1) + '"' + (findingLines.has(i + 1) ? ' class="finding"' : '') + '><td class="ln">' + (i + 1) + '</td><td class="src">' + esc(line) + '</td></tr>').join('') + '</tbody></table></div>';
  return html + '</div>';
}

function insights(state, info, findings) {
  let html = '';
  const ex = state.explorer.explanation;
  if (ex) {
    const list = items => asArray(items).length ? '<ul class="bullets small">' + asArray(items).map(item => '<li>' + esc(item) + '</li>').join('') + '</ul>' : '<p class="faint small">None listed.</p>';
    html += '<details class="disclosure insight" open><summary>' + icon('sparkle') + 'Explanation ' + badge('AI suggestion', 'accent') + '</summary><div class="disclosure-body"><p>' + esc(ex.summary || '') + '</p><div class="insight-grid"><div><h4>Behavior</h4>' + list(ex.behavior) + '</div><div><h4>Inputs · Outputs</h4>' + list(asArray(ex.inputs).concat(asArray(ex.outputs))) + '</div><div><h4>Errors & boundaries</h4>' + list(ex.error_behavior) + '</div></div></div></details>';
  } else if (state.explorer.analysis && state.explorer.analysis.purpose) {
    const fa = state.explorer.analysis;
    html += '<details class="disclosure insight"><summary>' + icon('book') + 'Cached analysis ' + badge('AI suggestion', 'accent') + '</summary><div class="disclosure-body stack"><p>' + esc(fa.purpose) + '</p>' + (asArray(fa.responsibilities).length ? '<div class="chips">' + fa.responsibilities.map(r => '<span class="badge">' + esc(r) + '</span>').join('') + '</div>' : '') + '</div></details>';
  }
  if (findings.length) {
    html += '<details class="disclosure insight"' + (findings.length <= 3 ? ' open' : '') + '><summary>' + icon('issues') + plural(findings.length, 'open issue') + '</summary><div class="disclosure-body"><div class="list">' + findings.map(f => '<div class="list-row"><div class="grow"><div class="title">' + esc(f.title || f.id) + '</div><div class="sub">' + esc(sourceLabel(f)) + (findingLine(f) ? ' · line ' + findingLine(f) : '') + '</div></div><div class="end">' + severityBadge(f.severity) + (findingLine(f) ? button('Go to line', { size: 'sm', variant: 'ghost', data: { action: 'goto-line', line: findingLine(f) } }) : '') + '</div></div>').join('') + '</div></div></details>';
  }
  return html;
}
