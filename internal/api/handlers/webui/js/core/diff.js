// Diff shaping for review. The daemon sends every line of a changed file;
// review focuses on changed lines with a few lines of context and lets the
// reviewer expand hidden ranges explicitly.

import { asArray } from './util.js';

export function diffStats(edit) {
  let added = 0;
  let removed = 0;
  for (const line of asArray(edit && edit.diff && edit.diff.lines)) {
    if (line.kind === 'added') added++;
    else if (line.kind === 'removed') removed++;
  }
  return { added, removed };
}

/**
 * Groups diff lines into visible hunks and collapsed gaps.
 * Each segment is {type:'lines', start, end} or {type:'gap', start, end, count}
 * where start/end index into `lines` (end exclusive). Gaps listed in `expanded`
 * (keyed by their start index) stay visible.
 */
export function buildSegments(lines, context = 3, expanded = new Set()) {
  const list = asArray(lines);
  if (!list.length) return [];
  const keep = new Array(list.length).fill(false);
  let anyChange = false;
  list.forEach((line, index) => {
    if (line.kind === 'added' || line.kind === 'removed') {
      anyChange = true;
      for (let k = Math.max(0, index - context); k <= Math.min(list.length - 1, index + context); k++) keep[k] = true;
    }
  });
  if (!anyChange) return [{ type: 'gap', start: 0, end: list.length, count: list.length }];
  const segments = [];
  let index = 0;
  while (index < list.length) {
    const visible = keep[index];
    let end = index;
    while (end < list.length && keep[end] === visible) end++;
    if (visible) segments.push({ type: 'lines', start: index, end });
    else if (expanded.has(index) || end - index <= 2) segments.push({ type: 'lines', start: index, end });
    else segments.push({ type: 'gap', start: index, end, count: end - index });
    index = end;
  }
  return mergeAdjacent(segments);
}

function mergeAdjacent(segments) {
  const out = [];
  for (const segment of segments) {
    const last = out[out.length - 1];
    if (last && last.type === 'lines' && segment.type === 'lines') last.end = segment.end;
    else out.push({ ...segment });
  }
  return out;
}

/** Counts change blocks (runs of added/removed lines) for hunk navigation. */
export function hunkStarts(lines) {
  const starts = [];
  asArray(lines).forEach((line, index, list) => {
    const changed = line.kind === 'added' || line.kind === 'removed';
    const prev = list[index - 1];
    const prevChanged = prev && (prev.kind === 'added' || prev.kind === 'removed');
    if (changed && !prevChanged) starts.push(index);
  });
  return starts;
}

/** Pairs removed/added runs side by side for split view. */
export function splitRows(lines) {
  const list = asArray(lines);
  const rows = [];
  let i = 0;
  while (i < list.length) {
    if (list[i].kind !== 'added' && list[i].kind !== 'removed') {
      rows.push({ left: list[i], right: list[i] });
      i++;
      continue;
    }
    const removed = [];
    const added = [];
    while (i < list.length && list[i].kind === 'removed') removed.push(list[i++]);
    while (i < list.length && list[i].kind === 'added') added.push(list[i++]);
    for (let k = 0; k < Math.max(removed.length, added.length); k++) rows.push({ left: removed[k] || null, right: added[k] || null });
  }
  return rows;
}

/**
 * Parses `git diff` text (as returned by merge preview) into per-file line
 * lists with the same shape as proposal diffs.
 */
export function parseGitDiff(text) {
  const files = [];
  let current = null;
  let oldLine = 0;
  let newLine = 0;
  for (const raw of String(text || '').split('\n')) {
    if (raw.startsWith('diff --git ')) {
      const match = /^diff --git a\/(.+?) b\/(.+)$/.exec(raw);
      current = { path: match ? match[2] : raw.slice(11), lines: [], binary: false };
      files.push(current);
      continue;
    }
    if (!current) continue;
    if (raw.startsWith('+++ ') || raw.startsWith('--- ') || raw.startsWith('index ') || raw.startsWith('new file') || raw.startsWith('deleted file') || raw.startsWith('similarity') || raw.startsWith('rename ')) continue;
    if (raw.startsWith('Binary files')) { current.binary = true; continue; }
    const hunk = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)$/.exec(raw);
    if (hunk) {
      oldLine = Number(hunk[1]);
      newLine = Number(hunk[2]);
      current.lines.push({ kind: 'hunk', text: raw });
      continue;
    }
    if (raw.startsWith('+')) current.lines.push({ kind: 'added', new_line: newLine++, text: raw.slice(1) });
    else if (raw.startsWith('-')) current.lines.push({ kind: 'removed', old_line: oldLine++, text: raw.slice(1) });
    else if (raw.startsWith(' ')) current.lines.push({ kind: 'context', old_line: oldLine++, new_line: newLine++, text: raw.slice(1) });
  }
  return files;
}
