import { describe, expect, it } from 'vitest';
import { fuzzyScore, rankItems } from './fuzzy';
import { ancestors, buildTree, visibleRows } from './tree';
import { buildHash, parseHash } from './router';
import { escapeHtml, highlightLines } from './highlight';
import { companionTestPath, isTestPath, timeAgo } from './util';
import { parseCommits, workflowBlocker } from './workflow';
import { parseFrames } from '@/api/events';
import { createApi, query, withWorktree } from '@/api/client';

describe('fuzzy', () => {
  it('prefers basename and contiguous hits', () => {
    const files = ['internal/api/handlers/webui_handler.go', 'internal/app/handler.go', 'docs/handbook.md'];
    expect(rankItems(files, 'handler', f => f)[0].item).toBe('internal/app/handler.go');
    expect(rankItems(files, 'wuh', f => f).map(r => r.item)).toEqual(['internal/api/handlers/webui_handler.go']);
    expect(fuzzyScore('zz', 'abc')).toBeNull();
    expect(fuzzyScore('abc', 'xxabc')?.positions).toEqual([2, 3, 4]);
    expect(rankItems(files, '', f => f, 2)).toHaveLength(2);
  });
});

describe('tree', () => {
  it('compacts single-child folders and lists folders before files', () => {
    const tree = buildTree(['internal/api/a.go', 'internal/api/b.go', 'main.go'], p => p);
    expect(visibleRows(tree, new Set()).map(r => [r.type, r.label])).toEqual([['dir', 'internal/api'], ['file', 'main.go']]);
    expect(visibleRows(tree, new Set(['internal/api'])).map(r => r.label)).toEqual(['internal/api', 'a.go', 'b.go', 'main.go']);
    expect(ancestors('a/b/c.go')).toEqual(['a', 'a/b']);
  });
});

describe('router', () => {
  it('round-trips paths and params', () => {
    const hash = buildHash('/tasks/chg 1/files', { file: 'a/b.go', wt: '/tmp/wt', empty: '' });
    expect(hash).toBe('#/tasks/chg%201/files?file=a%2Fb.go&wt=%2Ftmp%2Fwt');
    const route = parseHash(hash);
    expect(route.segments).toEqual(['tasks', 'chg 1', 'files']);
    expect(route.params.get('wt')).toBe('/tmp/wt');
    expect(parseHash('').path).toBe('/');
  });
});

describe('highlight', () => {
  it('escapes markup and keeps one entry per line with balanced spans', () => {
    expect(escapeHtml('<a href="x">&</a>')).toBe('&lt;a href=&quot;x&quot;&gt;&amp;&lt;/a&gt;');
    const out = highlightLines('/* a\nb */\nconst x = "<y>";', 'javascript');
    expect(out).toHaveLength(3);
    for (const line of out) expect((line.match(/<span/g) ?? []).length).toBe((line.match(/<\/span>/g) ?? []).length);
    expect(out[2]).not.toContain('<y>');
  });
});

describe('util and workflow', () => {
  it('recognizes tests and companions', () => {
    expect(isTestPath('a/b_test.go')).toBe(true);
    expect(isTestPath('web/x.test.tsx')).toBe(true);
    expect(isTestPath('a/b.go')).toBe(false);
    expect(companionTestPath('a/b.go')).toBe('a/b_test.go');
    expect(companionTestPath('a/b_test.go')).toBe('');
    expect(timeAgo('2026-01-01T00:00:00Z', Date.parse('2026-01-01T00:05:00Z'))).toMatch(/5m/);
  });

  it('parses commits and explains workflow blockers', () => {
    expect(parseCommits('abc123 Fix thing\n\ndef456')).toEqual([{ sha: 'abc123', subject: 'Fix thing' }, { sha: 'def456', subject: '' }]);
    const s = { state: 'draft', freshness: 'current', kind: 'fix', targets: [{ path: 'a.go' }] };
    expect(workflowBlocker(s, true, isTestPath)).toMatch(/test file/);
    expect(workflowBlocker({ ...s, targets: [{ path: 'a_test.go' }] }, false, isTestPath)).toMatch(/Trust/);
    expect(workflowBlocker({ ...s, targets: [{ path: 'a_test.go' }] }, true, isTestPath)).toBe('');
    expect(workflowBlocker(null, true, isTestPath)).toMatch(/Select/);
  });
});

describe('events', () => {
  it('parses complete frames and returns the tail', () => {
    const seen: [string, string][] = [];
    const tail = parseFrames('event: update\ndata: {"kind":"workflow"}\n\n: ping\n\nevent: update\ndata: {"ki', (e, d) => seen.push([e, d]));
    expect(seen).toEqual([['update', '{"kind":"workflow"}']]);
    expect(tail).toBe('event: update\ndata: {"ki');
  });
});

describe('client', () => {
  it('builds queries and worktree params', () => {
    expect(query({ a: 1, b: '', c: undefined, d: true })).toBe('?a=1&d=true');
    expect(withWorktree('/x?a=1', '/w t')).toBe('/x?a=1&worktree_path=%2Fw%20t');
    expect(withWorktree('/x', '')).toBe('/x');
  });

  it('sends the Web UI header and surfaces daemon messages', async () => {
    const calls: RequestInit[] = [];
    const api = createApi(async (_url, init) => {
      calls.push(init!);
      return new Response(JSON.stringify({ user_message: 'Review every file first.' }), { status: 409 });
    });
    await expect(api.post('/x', { a: 1 })).rejects.toMatchObject({ message: 'Review every file first.', status: 409 });
    expect((calls[0].headers as Record<string, string>)['X-Mini-Orca-Web-UI']).toBe('1');
    expect(calls[0].body).toBe('{"a":1}');
  });
});
