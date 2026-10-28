// Agent overrides saved for an earlier model configuration hold up all agent
// work until they are reset; this says so where the work is started.

import { RotateCcw, TriangleAlert } from 'lucide-react';
import { useAgentSettings, useProject } from '@/api/hooks';
import { useActions, useBusy } from '@/app/actions';
import { useCheckout } from '@/app/checkout';
import { Button, Callout } from '@/components/ui';

export function AgentSettingsNotice({ note }: { note?: string }) {
  const project = useProject().data;
  const { worktreePath } = useCheckout();
  const settings = useAgentSettings(worktreePath, project ?? null).data;
  const actions = useActions();
  const busy = Boolean(useBusy());
  if (settings?.state !== 'stale') return null;
  return (
    <Callout
      tone="warning"
      icon={TriangleAlert}
      title="Agent settings are out of date"
      action={<Button size="sm" icon={RotateCcw} onClick={() => void actions.controlAgentSettings(settings, worktreePath, null)} disabled={busy}>Reset</Button>}
    >
      {note ?? 'The roles and budget saved in Settings → Models & agents belong to an earlier model configuration, so analysis and agents wait until they are reset to the current configuration.'}
    </Callout>
  );
}
