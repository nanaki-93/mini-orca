// The selected checkout: the main project ('') or one of its Git worktrees.
// It lives in the route (`wt`) so links and reloads keep it.

import { navigate, useRoute } from '@/lib/router';

export function useCheckout() {
  const route = useRoute();
  const worktreePath = route.params.get('wt') ?? '';
  return {
    worktreePath,
    setCheckout(path: string) {
      navigate(route.path, { ...Object.fromEntries(route.params.entries()), wt: path || undefined });
    },
  };
}
