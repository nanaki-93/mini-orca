// Time limits: how long a model request or check may take before it stops.
// Settings saves them on their own; setup saves them with the models.

import { useState } from 'react';
import { Clock } from 'lucide-react';
import type { ProjectSetup, SetupTimeouts } from '@/api/types';
import { useActions, useBusy } from '@/app/actions';
import { TIMEOUTS, MAX_TIMEOUT_SECONDS, timeoutErrors } from '@/lib/setup';
import { formatDuration } from '@/lib/analysis';
import { Button, Field, Input } from '@/components/ui';

/** One number field per time limit, in seconds. */
export function TimeoutFields({ id, timeouts, onChange }: { id: string; timeouts: SetupTimeouts; onChange: (timeouts: SetupTimeouts) => void }) {
  const errors = timeoutErrors(timeouts);
  return (
    <div className="limit-grid">
      {TIMEOUTS.map(({ key, label, hint }) => (
        <Field key={key} label={`${label} (seconds)`} htmlFor={`${id}-${key}`} error={errors[key]} hint={`${hint} Now ${formatDuration(timeouts[key])}.`}>
          <Input
            id={`${id}-${key}`}
            type="number"
            inputMode="numeric"
            min={1}
            max={MAX_TIMEOUT_SECONDS}
            step={1}
            value={Number.isNaN(timeouts[key]) ? '' : String(timeouts[key])}
            invalid={Boolean(errors[key])}
            onChange={e => onChange({ ...timeouts, [key]: e.target.value === '' ? Number.NaN : Number(e.target.value) })}
          />
        </Field>
      ))}
    </div>
  );
}

/** Settings → Models & agents: edit and save the time limits alone. */
export function TimeLimits({ setup }: { setup: ProjectSetup }) {
  const actions = useActions();
  const busy = Boolean(useBusy());
  const [draft, setDraft] = useState<SetupTimeouts>(setup.timeouts);
  const changed = TIMEOUTS.some(({ key }) => draft[key] !== setup.timeouts[key]);
  const invalid = Object.keys(timeoutErrors(draft)).length > 0;
  return (
    <section className="card" aria-label="Time limits">
      <div className="card-head">
        <Clock size={15} aria-hidden />
        <span className="strong">Time limits</span>
        <span className="xs muted">A request or check that takes longer stops and is reported as timed out.</span>
      </div>
      <div className="card-body">
        <TimeoutFields id="settings-timeout" timeouts={draft} onChange={setDraft} />
      </div>
      <div className="card-foot row">
        <span className="xs muted grow">Saved in this project’s configuration and applied at once, once no analysis or agent work is running. Agent settings are kept.</span>
        <Button size="sm" variant="ghost" onClick={() => setDraft(setup.timeouts)} disabled={!changed || busy}>Discard</Button>
        <Button size="sm" variant="primary" onClick={() => void actions.saveTimeouts(setup, draft)} disabled={!changed || invalid || busy}>Save time limits</Button>
      </div>
    </section>
  );
}
