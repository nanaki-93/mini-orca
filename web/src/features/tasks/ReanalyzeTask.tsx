// Re-analyzes a stale task: a model checks whether its issue is still present
// in the current files, then the task is refreshed against them or deleted.

import { ScanSearch } from 'lucide-react';
import type { LoadedSession } from '@/api/types';
import { useActions, useBusy } from '@/app/actions';
import { Button, Tip } from '@/components/ui';

export function ReanalyzeTask({ session, variant = 'primary', size }: { session: LoadedSession; variant?: 'primary' | 'secondary'; size?: 'sm' }) {
  const actions = useActions();
  const busy = useBusy();
  return (
    <Tip label="Check whether the issue is still present in the current files: a valid task is refreshed, the rest deleted">
      <span>
        <Button variant={variant} size={size} icon={ScanSearch} onClick={() => void actions.reanalyze(session)} disabled={Boolean(busy)} loading={busy === 'Re-analyzing…'}>
          Re-analyze
        </Button>
      </span>
    </Tip>
  );
}
