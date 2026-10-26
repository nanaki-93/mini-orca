// Git: repository state, agent worktrees, and review-before-merge.

import { asArray, basename, esc, plural, shortHash } from '../core/util.js';
import { buildSegments, diffStats } from '../core/diff.js';
import { parseCommits } from '../core/workflow.js';
import { icon } from '../ui/icons.js';
import { badge, button, callout, empty } from '../ui/dom.js';
import { taskStateBadge } from './shared.js';

function worktrees(state) {
  return asArray(state.gitWorkspace && state.gitWorkspace.worktrees).filter(tree => !tree.current);
}

export function gitSide(state) {
  const trees = worktrees(state);
  const ws = state.gitWorkspace;
  return '<div class="side-head"><span class="side-title">Worktrees <span class="count">' + trees.length + '</span></span></div><div class="side-scroll"><div class="nav-list">' +
    '<button class="nav-item" type="button" aria-current="' + !state.merge.path + '" data-action="git-select" data-path="">' + icon('home') + '<span class="truncate">' + esc((ws && ws.branch) || 'main checkout') + '</span>' + (ws && ws.dirty ? '<span class="count">M</span>' : '') + '</button>' +
    trees.map(tree => '<button class="nav-item" type="button" aria-current="' + (state.merge.path === tree.path) + '" data-action="git-select" data-path="' + esc(tree.path) + '" title="' + esc(tree.path) + '">' + icon('git') + '<span class="truncate">' + esc(tree.branch || basename(tree.path)) + '</span>' + (tree.mergeable ? '<span class="status-dot done" title="Mergeable"></span>' : '') + '</button>').join('') + '</div></div>';
}

export function gitMain(state) {
  const ws = state.gitWorkspace;
  let html = '<div class="page"><div class="page-inner stack-lg"><header class="page-head"><div><h1 class="page-title">Git</h1><p class="page-sub">Agent tasks can run in isolated worktrees. Review the incoming commits and diff before merging them into your branch.</p></div>' + button('Refresh', { icon: 'refresh', data: { action: 'refresh' } }) + '</header>';
  if (!ws) return html + empty('Git status unavailable', 'Open a Git repository root to use branches and worktrees.', 'git') + '</div></div>';
  if (!ws.available) return html + empty('Not a Git repository', 'Initialize Git or open the repository root to merge agent work from worktrees.', 'git') + '</div></div>';
  const conflicts = asArray(ws.conflicts);
  html += '<section class="card"><div class="card-head"><span class="card-title">' + icon('home') + 'Current checkout</span>' + (ws.dirty ? badge('modified', 'warning') : badge('clean', 'success')) + '</div><div class="card-body"><dl class="kv"><dt>Branch</dt><dd class="mono">' + esc(ws.branch || 'detached') + '</dd><dt>HEAD</dt><dd class="mono">' + esc(shortHash(ws.head, 12)) + '</dd><dt>Root</dt><dd class="path">' + esc(ws.root || '') + '</dd>' + (ws.operation ? '<dt>In progress</dt><dd>' + esc(ws.operation) + '</dd>' : '') + '</dl></div></section>';
  if (ws.merge_head || conflicts.length) {
    html += callout('danger', '<strong>Merge in progress' + (conflicts.length ? ' with ' + plural(conflicts.length, 'conflict') : '') + '.</strong> Resolve conflicts in your editor and commit, or abort to restore the branch.' + (conflicts.length ? '<div class="chips">' + conflicts.map(p => '<span class="chip"><span>' + esc(p) + '</span></span>').join('') + '</div>' : ''), button('Abort merge', { variant: 'danger', size: 'sm', disabled: Boolean(state.pending), data: { action: 'abort-merge', mutation: '' } }));
  }
  const trees = worktrees(state);
  if (!trees.length) {
    html += empty('No worktrees', 'When agents work in an isolated worktree, it appears here with its tasks, ready for a reviewed merge.', 'git');
    return html + '</div></div>';
  }
  const selected = trees.find(tree => tree.path === state.merge.path);
  if (!selected) {
    html += '<section class="card"><div class="card-head"><span class="card-title">Worktrees</span></div><div class="list">' + trees.map(tree => '<button class="list-row" type="button" data-action="git-select" data-path="' + esc(tree.path) + '">' + icon('git') + '<div class="grow"><div class="title">' + esc(tree.branch || basename(tree.path)) + '</div><div class="sub">' + esc(plural(asArray(tree.sessions).length, 'task')) + ' · ' + esc(shortHash(tree.head)) + (tree.integration ? ' · ' + esc(tree.integration) : '') + '</div></div><div class="end">' + (tree.mergeable ? badge('mergeable', 'success') : badge('not ready')) + (tree.dirty ? badge('modified', 'warning') : '') + '<span class="chev-right"></span></div></button>').join('') + '</div></section>';
    return html + '</div></div>';
  }
  html += worktreeDetail(state, selected);
  return html + '</div></div>';
}

function worktreeDetail(state, tree) {
  const sessions = asArray(tree.sessions);
  const preview = state.merge.path === tree.path ? state.merge : null;
  const trusted = Boolean(state.trust && state.trust.trusted);
  let html = '<section class="card"><div class="card-head"><span class="card-title">' + icon('git') + esc(tree.branch || basename(tree.path)) + '</span><div class="row">' + (tree.mergeable ? badge('mergeable', 'success') : badge('not mergeable')) + (tree.locked ? badge('locked', 'warning') : '') + '</div></div><div class="card-body stack"><dl class="kv"><dt>Path</dt><dd class="path">' + esc(tree.path) + '</dd><dt>HEAD</dt><dd class="mono">' + esc(shortHash(tree.head, 12)) + '</dd>' + (tree.integration ? '<dt>Integration</dt><dd>' + esc(tree.integration) + '</dd>' : '') + '</dl>' +
    (asArray(tree.overlap_paths).length ? callout('warning', 'Also changed on your branch: <span class="mono">' + esc(tree.overlap_paths.join(', ')) + '</span>. Expect conflicts.') : '') + (tree.error ? callout('danger', esc(tree.error)) : '') +
    '<div class="row">' + button('Preview merge', { icon: 'eye', disabled: Boolean(preview && preview.loading), busy: Boolean(preview && preview.loading), data: { action: 'merge-preview', path: tree.path } }) +
    button('Merge into ' + ((state.gitWorkspace && state.gitWorkspace.branch) || 'branch'), { variant: 'primary', icon: 'merge', disabled: !tree.mergeable || !trusted || !(preview && preview.preview) || Boolean(state.pending), busy: state.pending === 'Merging…', title: !(preview && preview.preview) ? 'Preview the merge first' : !trusted ? 'Grant execution trust first' : 'Merge the reviewed HEAD', data: { action: 'merge', path: tree.path, mutation: '' } }) +
    button('Open checkout', { variant: 'ghost', icon: 'external', title: 'Explore and review inside this worktree', data: { action: 'set-checkout', path: tree.path } }) + '</div></div></section>';
  if (sessions.length) html += '<section class="card"><div class="card-head"><span class="card-title">Tasks in this worktree</span></div><div class="list">' + sessions.map(s => '<button class="list-row" type="button" data-action="open-task" data-id="' + esc(s.id) + '"><div class="grow"><div class="title">' + esc(s.title || s.id) + '</div><div class="sub">' + esc(s.kind || '') + '</div></div><div class="end">' + taskStateBadge(s) + '<span class="chev-right"></span></div></button>').join('') + '</div></section>';
  if (preview && preview.preview) {
    const commits = parseCommits(preview.preview.commits);
    const files = preview.files;
    const totals = files.reduce((acc, f) => { const s = diffStats({ diff: { lines: f.lines } }); acc.add += s.added; acc.del += s.removed; return acc; }, { add: 0, del: 0 });
    html += '<section class="card"><div class="card-head"><span class="card-title">Incoming · ' + plural(commits.length, 'commit') + ' · ' + plural(files.length, 'file') + '</span><span class="diffstat"><span class="add">+' + totals.add + '</span><span class="del">−' + totals.del + '</span></span></div><div class="card-body">' +
      (commits.length ? commits.map(c => '<div class="commit"><span class="sha">' + esc(c.sha) + '</span><span>' + esc(c.subject) + '</span></div>').join('') : '<p class="faint small">No new commits — the branch is already merged.</p>') + '</div></section>';
    html += '<div class="diff-files">' + files.map(f => mergeFile(f)).join('') + '</div>';
  }
  return html;
}

function mergeFile(file) {
  const stats = diffStats({ diff: { lines: file.lines } });
  const lines = file.lines;
  const segments = buildSegments(lines.filter(l => l.kind !== 'hunk'), 3);
  const body = lines.filter(l => l.kind !== 'hunk');
  let rows = '';
  for (const segment of segments) {
    if (segment.type === 'gap') { rows += '<tr class="gap"><td colspan="4"><span class="gap-btn">' + plural(segment.count, 'unchanged line') + '</span></td></tr>'; continue; }
    for (let i = segment.start; i < segment.end; i++) {
      const line = body[i];
      const cls = line.kind === 'added' ? 'add' : line.kind === 'removed' ? 'del' : '';
      rows += '<tr' + (cls ? ' class="' + cls + '"' : '') + '><td class="ln">' + (line.old_line || '') + '</td><td class="ln">' + (line.new_line || '') + '</td><td class="sign">' + (cls === 'add' ? '+' : cls === 'del' ? '−' : '') + '</td><td>' + esc(line.text) + '</td></tr>';
    }
  }
  return '<article class="diff-file"><header class="diff-file-head"><div class="diff-file-title">' + icon('file') + '<span class="path">' + esc(file.path) + '</span><span class="diffstat"><span class="add">+' + stats.added + '</span><span class="del">−' + stats.removed + '</span></span></div></header><div class="diff-scroll">' + (file.binary ? '<div class="card-body faint small">Binary file</div>' : '<table class="diff unified"><tbody>' + rows + '</tbody></table>') + '</div></article>';
}
