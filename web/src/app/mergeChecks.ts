// The checks of the last merge: after a merge, the daemon runs the tests and
// lint of the subprojects it changed on the merged result. They can take
// minutes, so they run in the background and the Branches page shows them.

import { useSyncExternalStore } from 'react';
import { api, errorMessage } from '@/api/client';
import type { GitMutationResult, MergeChecks } from '@/api/types';

export interface MergeChecksState {
  /** The branch that was merged. */
  branch: string;
  status: 'running' | 'done' | 'error';
  result?: MergeChecks;
  error?: string;
}

let state: MergeChecksState | null = null;
const listeners = new Set<() => void>();

function set(next: MergeChecksState | null) {
  state = next;
  listeners.forEach(fn => fn());
}

export function useMergeChecks(): MergeChecksState | null {
  return useSyncExternalStore(
    fn => {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
    () => state,
  );
}

export function dismissMergeChecks() {
  set(null);
}

/** What a merge result names to check: the commits before and after it, under the merged revision. */
export function mergeToCheck(result: GitMutationResult | undefined) {
  if (result?.state !== 'merged' || !result.previous_head || !result.repository?.head || !result.index || result.previous_head === result.repository.head) return null;
  return { project_id: result.index.project_id, project_revision: result.index.project_revision, from: result.previous_head, to: result.repository.head };
}

/**
 * Runs the checks of a merge and reports the outcome; a newer merge replaces
 * the state of an earlier one still running.
 */
export async function runMergeChecks(branch: string, request: NonNullable<ReturnType<typeof mergeToCheck>>, report: (result: MergeChecks) => void) {
  const mine: MergeChecksState = { branch, status: 'running' };
  set(mine);
  try {
    const result = await api.post<MergeChecks>('/api/projects/current/git/merge-checks', request);
    if (state === mine) set({ branch, status: 'done', result });
    report(result);
  } catch (err) {
    if (state === mine) set({ branch, status: 'error', error: errorMessage(err) });
  }
}
