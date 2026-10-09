import { useEffect, useState } from 'react';
import type { ChangeSeed } from './models';
import { workspace as w, workflowSeed, type FixPreparationContext, type State } from './workspace';
import { WorkflowModels, defaultWorkflowModels } from './change-workflow';
import { BulletContent, Button, Notice, Panel } from './ui';

export function FixPreparation({
  s,
  seed,
  disabled,
  label = 'Prepare fix',
}: {
  s: State;
  seed: ChangeSeed;
  disabled: boolean;
  label?: string;
}) {
  const [context, setContext] = useState<FixPreparationContext>();
  const [error, setError] = useState('');
  const [refresh, setRefresh] = useState(0);
  const [consent, setConsent] = useState({ key: '', remote: false, execution: false });
  useEffect(() => {
    let current = true;
    setContext(undefined);
    setError('');
    void w.readFixPreparation().then(
      (value) => {
        if (current) setContext(value);
      },
      (failure) => {
        if (current) setError(String(failure));
      },
    );
    return () => {
      current = false;
    };
  }, [s.project?.project_id, s.project?.project_revision, refresh]);
  const prepared = workflowSeed(seed);
  const useAgents = prepared.paths.some((path) => path.endsWith('_test.go'));
  const models = s.workflowModels || s.change?.workflow?.models || defaultWorkflowModels;
  const profiles = useAgents ? Object.values(models) : [models.create];
  const remote = [...new Set(profiles)].filter(
    (profile) => context?.models.scopes[profile]?.remote_provider,
  );
  const key = JSON.stringify([prepared, models, context]);
  const allowed = consent.key === key ? consent : { remote: false, execution: false };
  const ready =
    context &&
    profiles.every((p) => context.models.scopes[p]) &&
    (!remote.length || allowed.remote) &&
    (context.trust.trusted || allowed.execution);
  return (
    <Panel title="Prepare fix" className="fix-preparation">
      {error ? (
        <Notice error>{error}</Notice>
      ) : !context ? (
        <p>Loading models and permissions…</p>
      ) : (
        <>
          <WorkflowModels
            s={{ ...s, models: context.models }}
            value={models}
            disabled={disabled}
            creationOnly={!useAgents}
          />
          <BulletContent title="Files in this fix" items={prepared.paths} />
          {remote.length > 0 && (
            <>
              <BulletContent
                title="Remote destinations"
                items={remote.map(
                  (profile) =>
                    `${context.models.scopes[profile].model} · ${context.models.scopes[profile].provider_origin}`,
                )}
              />
              <label className="checkbox-line">
                <input
                  type="checkbox"
                  checked={allowed.remote}
                  disabled={disabled}
                  onChange={(event) =>
                    setConsent({ ...allowed, key, remote: event.target.checked })
                  }
                />
                Allow sending these files and applicable instructions to the selected remote models
              </label>
            </>
          )}
          {!context.trust.trusted && (
            <>
              <p className="small muted">
                Checks run in an isolated copy. Project code can still access this computer.
              </p>
              <BulletContent
                title="Test commands"
                items={context.trust.commands.map((argv) => argv.join(' '))}
              />
              <label className="checkbox-line">
                <input
                  type="checkbox"
                  checked={allowed.execution}
                  disabled={disabled}
                  onChange={(event) =>
                    setConsent({ ...allowed, key, execution: event.target.checked })
                  }
                />
                Allow project tests for this revision
              </label>
            </>
          )}
          <p className="small muted">
            {seed.kind === 'security' ? 'Prepare fix starts the Security fix workflow. ' : ''}
            Review the proposed diff and checks before applying.
          </p>
        </>
      )}
      <div className="actions section-gap">
        <Button
          tone="primary"
          icon="sparkles"
          disabled={disabled || !ready}
          onClick={() => {
            if (!context || !ready) return;
            const approval = {
              ...context,
              allowRemote: allowed.remote,
              allowExecution: allowed.execution,
            };
            if (s.page !== 'chat') w.seedChange(prepared);
            void (useAgents
              ? w.startChangeWorkflow(prepared, models, approval)
              : w.prepareChange(prepared, true, models.create, approval));
          }}
        >
          {label}
        </Button>
        <Button disabled={disabled} onClick={() => setRefresh((value) => value + 1)}>
          Refresh preparation
        </Button>
      </div>
    </Panel>
  );
}
