// Settings → Models & agents: choose the model each agent role runs with and
// the budget of each run. Saving replaces the project's settings for new runs;
// reset follows the configuration (the project's or the daemon's) again. Running tasks keep what they started with.

import { useMemo, useState } from 'react';
import { Bot, RefreshCw, RotateCcw, Save } from 'lucide-react';
import type { AgentPreferences, AgentRole, AgentSettings } from '@/api/types';
import { useAgentSettings, useAvailableModels, useModels, useProject } from '@/api/hooks';
import { useActions, useBusy } from '@/app/actions';
import { useCheckout } from '@/app/checkout';
import { LIMITS, ROLES, limitErrors, modelLocation, preferencesOf, roleModelOptions, samePreferences, selectedModel } from '@/lib/agentSettings';
import { Badge, Button, Callout, Field, Input, Select, Skeleton } from '@/components/ui';

const STATE_BADGES: Record<string, { tone: 'neutral' | 'accent' | 'warning'; label: string }> = {
  configured: { tone: 'neutral', label: 'From the configuration' },
  overridden: { tone: 'accent', label: 'Project settings' },
  stale: { tone: 'warning', label: 'Out of date' },
};

export function AgentRoles() {
  const project = useProject().data;
  const { worktreePath } = useCheckout();
  const settings = useAgentSettings(worktreePath, project ?? null);
  if (!settings.data) {
    return (
      <section className="card" aria-label="Agent roles">
        <div className="card-head"><Bot size={15} aria-hidden /><span className="strong">Agent roles</span></div>
        <div className="card-body">{settings.isError ? <p className="small muted">Agent settings are unavailable for this checkout.</p> : <Skeleton lines={4} />}</div>
      </section>
    );
  }
  // A saved, reset or reloaded version starts a fresh draft.
  const data = settings.data;
  return <AgentRolesForm key={`${data.project_revision}:${data.configuration}:${data.version}:${data.state}`} settings={data} worktreePath={worktreePath} />;
}

function AgentRolesForm({ settings, worktreePath }: { settings: AgentSettings; worktreePath: string }) {
  const actions = useActions();
  const busy = useBusy();
  const scopes = useModels().data?.scopes ?? {};
  const catalog = useAvailableModels(true);
  const saved = useMemo(() => preferencesOf(settings), [settings]);
  const [draft, setDraft] = useState<AgentPreferences>(saved);
  const stale = settings.state === 'stale';
  const known = useMemo(() => {
    const models: Record<string, NonNullable<AgentSettings['role_models']>[AgentRole]> = {};
    for (const { role } of ROLES) {
      const model = settings.role_models?.[role];
      if (model) models[saved.roles[role]] = model;
    }
    return models;
  }, [settings, saved]);
  const groups = roleModelOptions(scopes, catalog.data, ROLES.map(({ role }) => draft.roles[role]), known);
  const errors = limitErrors(draft.limits, settings.ceilings);
  const dirty = !samePreferences(draft, saved);
  const valid = Object.keys(errors).length === 0;
  const badge = STATE_BADGES[settings.state] ?? STATE_BADGES.configured;
  const discoveries = [catalog.data?.claude, catalog.data?.agy, catalog.data?.pi].filter(d => d?.status === 'unavailable').map(d => d!.message).filter(Boolean);

  const setRole = (role: AgentRole, id: string) => setDraft(d => ({ ...d, roles: { ...d.roles, [role]: id } }));
  const setLimit = (key: string, value: string) => setDraft(d => ({ ...d, limits: { ...d.limits, [key]: value === '' ? Number.NaN : Number(value) } }));

  return (
    <section className="card" aria-label="Agent roles">
      <div className="card-head">
        <Bot size={15} aria-hidden />
        <span className="strong">Agent roles</span>
        <Badge tone={badge.tone}>{badge.label}</Badge>
        <span className="spacer" />
        <Button size="sm" variant="ghost" icon={RefreshCw} onClick={() => void catalog.refetch()} disabled={catalog.isFetching}>{catalog.isFetching ? 'Listing models…' : 'Refresh models'}</Button>
      </div>
      <div className="card-body stack-lg">
        {stale ? (
          <Callout tone="warning" title="The model configuration changed since these settings were saved" action={<Button size="sm" icon={RotateCcw} onClick={() => void actions.controlAgentSettings(settings, worktreePath, null)} disabled={Boolean(busy)}>Reset</Button>}>
            Analysis and agent runs wait until you reset them to the current configuration; you can then choose again.
          </Callout>
        ) : null}
        <div className="role-grid">
          {ROLES.map(({ role, label, hint }) => {
            const id = `role-${role}`;
            const location = modelLocation(selectedModel(groups, draft.roles[role]));
            return (
              <Field key={role} label={label} htmlFor={id} hint={hint}>
                <div className="row">
                  <Select id={id} value={draft.roles[role]} onChange={e => setRole(role, e.target.value)} disabled={stale}>
                    {groups.map(group => (
                      <optgroup key={group.label} label={group.label}>
                        {group.options.map(option => <option key={option.id} value={option.id}>{option.label}</option>)}
                      </optgroup>
                    ))}
                  </Select>
                  <Badge tone={location.remote ? 'warning' : location.label === 'local' ? 'success' : 'neutral'}>{location.label}</Badge>
                </div>
              </Field>
            );
          })}
        </div>
        {discoveries.length ? <p className="xs muted">{discoveries.join(' ')}</p> : null}
        <div>
          <div className="label-row">Budget per run</div>
          <div className="limit-grid">
            {LIMITS.map(({ key, label, min }) => (
              <Field key={key} label={label} htmlFor={`limit-${key}`} error={errors[key]} hint={settings.ceilings?.[key] !== undefined ? `Up to ${settings.ceilings[key].toLocaleString()}` : undefined}>
                <Input id={`limit-${key}`} type="number" inputMode="numeric" min={min} max={settings.ceilings?.[key]} step={1} value={Number.isNaN(draft.limits[key]) ? '' : String(draft.limits[key] ?? '')} invalid={Boolean(errors[key])} onChange={e => setLimit(key, e.target.value)} disabled={stale} />
              </Field>
            ))}
          </div>
        </div>
        <div className="row">
          <p className="xs muted">Remote models still ask before each run. Running tasks keep the models and budget they started with.</p>
          <span className="spacer" />
          {settings.state === 'overridden' ? <Button icon={RotateCcw} onClick={() => void actions.controlAgentSettings(settings, worktreePath, null)} disabled={Boolean(busy)}>Use the configuration</Button> : null}
          {dirty ? <Button variant="ghost" onClick={() => setDraft(saved)}>Discard</Button> : null}
          <Button variant="primary" icon={Save} onClick={() => void actions.controlAgentSettings(settings, worktreePath, draft)} disabled={stale || !dirty || !valid || Boolean(busy)}>Save</Button>
        </div>
      </div>
    </section>
  );
}
