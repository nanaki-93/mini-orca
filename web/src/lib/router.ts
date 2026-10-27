// A small hash router. The daemon serves one document at /ui, so routes live
// in the fragment: #/tasks/<id>/files?file=internal/x.go. The selected
// checkout (`wt`) is a global parameter that survives navigation.

import { useSyncExternalStore } from 'react';

export interface Route {
  path: string;
  segments: string[];
  params: URLSearchParams;
}

export function parseHash(hash: string): Route {
  const raw = hash.replace(/^#/, '') || '/';
  const [pathPart, search = ''] = raw.split('?');
  const path = '/' + pathPart.split('/').filter(Boolean).map(decodeURIComponent).join('/');
  return { path, segments: path.split('/').filter(Boolean), params: new URLSearchParams(search) };
}

export function buildHash(path: string, params: Record<string, string | undefined> = {}): string {
  const q = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) if (value) q.set(key, value);
  const encoded = '/' + path.split('/').filter(Boolean).map(encodeURIComponent).join('/');
  const search = q.toString();
  return '#' + encoded + (search ? '?' + search : '');
}

let cached: { hash: string; route: Route } | null = null;

function current(): Route {
  const hash = window.location.hash;
  if (!cached || cached.hash !== hash) cached = { hash, route: parseHash(hash) };
  return cached.route;
}

function subscribe(callback: () => void): () => void {
  window.addEventListener('hashchange', callback);
  return () => window.removeEventListener('hashchange', callback);
}

export function useRoute(): Route {
  return useSyncExternalStore(subscribe, current, current);
}

/** Navigates to `path`, keeping the selected checkout unless overridden. */
export function navigate(path: string, params: Record<string, string | undefined> = {}, options: { replace?: boolean } = {}): void {
  const wt = 'wt' in params ? params.wt : current().params.get('wt') ?? undefined;
  const hash = buildHash(path, { ...params, wt });
  if (hash === window.location.hash) return;
  if (options.replace) window.history.replaceState(null, '', window.location.pathname + window.location.search + hash);
  else window.location.hash = hash;
  if (options.replace) window.dispatchEvent(new HashChangeEvent('hashchange'));
}

export function setParam(key: string, value: string | undefined, options: { replace?: boolean } = { replace: true }): void {
  const route = current();
  const params: Record<string, string | undefined> = Object.fromEntries(route.params.entries());
  params[key] = value;
  navigate(route.path, params, options);
}

/**
 * The TUI opens /ui?tab=diff&session_id=…&file=…&worktree_path=…; translate
 * those links into routes once at startup.
 */
export function legacyRoute(search: string): string | null {
  const q = new URLSearchParams(search);
  if (![...q.keys()].length) return null;
  const tab = q.get('tab') ?? '';
  const session = q.get('session_id') ?? '';
  const file = q.get('file') ?? '';
  const wt = q.get('worktree_path') ?? undefined;
  if (session) {
    const sub = file || tab === 'diff' || tab === 'files' ? 'files' : tab === 'checks' ? 'checks' : '';
    return buildHash(`/tasks/${session}/${sub}`, { file: file || undefined, wt });
  }
  const pages: Record<string, string> = { files: '/code', explorer: '/code', issues: '/issues', docs: '/docs', checks: '/settings/analysis', git: '/git', settings: '/settings', tasks: '/tasks', 'new-task': '/tasks/new', home: '/', diff: '/tasks' };
  return buildHash(pages[tab] ?? (file ? '/code' : '/'), { file: file || undefined, wt });
}
