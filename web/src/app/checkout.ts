// The selected checkout: the main project ('') or one of its Git worktrees.
// It lives in the route (`wt`) so links and reloads keep it. A task page shows
// the task's own checkout; the workspace pages (Inbox, Tasks, Issues, …) keep
// the checkout chosen in the top bar, so opening a task that lives in its own
// worktree does not switch them.

import { currentRoute, navigate, useRoute } from '@/lib/router';

/** The checkout the app opened with: the main one when it opened on a task. */
function initialCheckout(): string {
  const route = currentRoute();
  return route.segments[0] === 'tasks' && route.segments[1] ? '' : route.params.get('wt') ?? '';
}

let chosen = initialCheckout();

/** The checkout chosen in the top bar. */
export function chosenCheckout(): string {
  return chosen;
}

/** Forgets a chosen checkout that was removed; the main checkout is chosen again. */
export function forgetCheckouts(paths: string[]): void {
  if (paths.includes(chosen)) chosen = '';
}

/** Opens a workspace page in the checkout chosen in the top bar. */
export function openWorkspace(path: string, params: Record<string, string | undefined> = {}): void {
  navigate(path, { ...params, wt: chosen || undefined });
}

export function useCheckout() {
  const route = useRoute();
  const worktreePath = route.params.get('wt') ?? '';
  return {
    worktreePath,
    setCheckout(path: string) {
      chosen = path;
      navigate(route.path, { ...Object.fromEntries(route.params.entries()), wt: path || undefined });
    },
  };
}
