import { describe, expect, it } from 'vitest';
import { fuzzyScore, rankItems } from './fuzzy';
import { ancestors, buildTree, visibleRows } from './tree';
import { buildHash, parseHash } from './router';
import { escapeHtml, highlightLines } from './highlight';
import { companionTestPath, isTestPath, timeAgo } from './util';
import { changeOverlaps, parseCommits, workflowBlocker } from './workflow';
import { applyEvent, finishedRun, parseFrames } from '@/api/events';
import { QueryClient } from '@tanstack/react-query';
import { costCaveat, costLabel, formatTokens, usageLabel } from './usage';
import { createApi, documentWriteToken, query, withWorktree } from '@/api/client';

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

describe('changeOverlaps', () => {
  it('flags shared files, packages and build manifests above a task', () => {
    expect(changeOverlaps(['internal/app/a.go'], ['internal/app/b.go'])).toBe(true);
    expect(changeOverlaps(['internal/app/a.go'], ['internal/llm/c.go'])).toBe(false);
    expect(changeOverlaps(['go.mod'], ['internal/llm/c.go'])).toBe(true);
    expect(changeOverlaps(['web/package.json'], ['web/src/app.tsx'])).toBe(true);
    expect(changeOverlaps(['web/package.json'], ['internal/app/a.go'])).toBe(false);
    expect(changeOverlaps(['README.md'], ['internal/app/a.go'])).toBe(false);
  });
});

describe('usage', () => {
  it('formats tokens, calls and estimated or partial costs', () => {
    expect([formatTokens(950), formatTokens(1250), formatTokens(20_500), formatTokens(1_250_000)]).toEqual(['950', '1.3k', '21k', '1.3M']);
    expect(usageLabel({ model_calls: 1, input_tokens: 900, output_tokens: 100 })).toBe('1 call · 1.0k tokens');
    expect(usageLabel({ model_calls: 0, input_tokens: 0, output_tokens: 0 })).toBe('');
    expect(costLabel({ status: 'estimated', currency: 'USD', known_subtotal: 0.5, total: 0.5, reasons: [] })).toMatch(/0\.5/);
    expect(costLabel({ status: 'unknown', currency: 'USD', known_subtotal: 0.25, reasons: ['model_rate_unavailable'] })).toMatch(/^≥ /);
    expect(costLabel({ status: 'unknown', known_subtotal: 0, reasons: ['price_table_unavailable'] })).toBe('');
    expect(costCaveat({ status: 'unknown', known_subtotal: 0, reasons: ['price_table_unavailable'] })).toMatch(/telemetry\.prices/);
  });
});

describe('daemon events', () => {
  it('announces finished runs only', () => {
    expect(finishedRun({ kind: 'workflow', stage: 'test', status: 'running', title: 'T' })).toBeNull();
    expect(finishedRun({ kind: 'workflow', status: 'canceled', title: 'T' })).toBeNull();
    expect(finishedRun({ kind: 'workflow', status: 'awaiting_human_review', title: 'T', usage: { model_calls: 2, input_tokens: 1000, output_tokens: 500 } })).toEqual({ message: 'Agents finished “T”. It needs your review.', detail: '2 calls · 1.5k tokens', tone: 'success' });
  });

  it('reloads only the task a workflow event names', () => {
    const client = new QueryClient();
    const invalidated: unknown[] = [];
    client.invalidateQueries = (filters?: { queryKey?: unknown }) => {
      invalidated.push(filters?.queryKey);
      return Promise.resolve();
    };
    applyEvent(client, { kind: 'workflow', session_id: 'chg-1' });
    expect(invalidated).toContainEqual(['session', 'chg-1']);
    expect(invalidated).not.toContainEqual(['session']);
    expect(invalidated).toContainEqual(['usage']);
  });
});

describe('write token', () => {
  it('goes with every request that changes state, never with reads', async () => {
    const seen: Record<string, Record<string, string>> = {};
    const api = createApi(async (url, init) => {
      seen[(init?.method ?? 'GET') + ' ' + String(url)] = init?.headers as Record<string, string>;
      return new Response('{}', { status: 200 });
    }, () => 'launch-token');
    await api.get('/api/projects/current');
    await api.post('/api/projects/current/changes', {});
    expect(seen['GET /api/projects/current']['X-Mini-Orca-Token']).toBeUndefined();
    expect(seen['POST /api/projects/current/changes']['X-Mini-Orca-Token']).toBe('launch-token');
  });

  it('reads the token the daemon wrote into the document', () => {
    const doc = document.implementation.createHTMLDocument('ui');
    doc.head.innerHTML = '<meta name="mini-orca-write-token" content="__MINI_ORCA_WRITE_TOKEN__">';
    expect(documentWriteToken(doc)).toBe('');
    doc.head.innerHTML = '<meta name="mini-orca-write-token" content="abc123">';
    expect(documentWriteToken(doc)).toBe('abc123');
  });
});
