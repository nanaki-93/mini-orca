import { useEffect, useState } from 'react';
import type { ChangeSeed } from './models';
import { workspace as w, workflowSeed, type FixPreparationContext, type State } from './workspace';
import { defaultWorkflowModels } from './change-workflow';
import { BulletContent, Button, Disclosure, Notice, Panel } from './ui';

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
  const modelKey = JSON.stringify(s.models?.scopes);
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
  }, [s.project?.project_id, s.project?.project_revision, modelKey, refresh]);
  const prepared = workflowSeed(seed);
  const useAgents = prepared.paths.some((path) => path.endsWith('_test.go'));
  const models = s.workflowModels || s.change?.workflow?.models || defaultWorkflowModels;
  const profiles = useAgents ? Object.values(models) : [models.create];
  const ready = context && profiles.every((profile) => context.models.scopes[profile]);
  const remote = [
    ...new Set(
      profiles.flatMap((profile) => {
        const model = context?.models.scopes[profile];
        return model?.remote_provider ? [`${model.model} · ${model.provider_origin}`] : [];
      }),
    ),
  ];
  return (
    <Panel title="Prepare fix" className="fix-preparation">
      {error ? (
        <Notice error>{error}</Notice>
      ) : !context ? (
        <p>Loading models and permissions…</p>
      ) : (
        <>
          <BulletContent title="Files in this fix" items={prepared.paths} />
          <div className="fix-permissions small">
            <p>
              Selecting {label} authorizes {seed.kind === 'security' ? 'a Security fix and ' : ''}
              project checks in an isolated copy.
              {remote.length > 0 &&
                ' It also shares these files and their instructions with the remote models below.'}
            </p>
            <BulletContent title="Remote destinations" items={remote} />
            <Disclosure title="Project checks">
              {context.trust.commands.map((argv, index) => (
                <p key={index}>
                  <code>{argv.join(' ')}</code>
                </p>
              ))}
            </Disclosure>
          </div>
        </>
      )}
      <div className="actions section-gap">
        <Button
          tone="primary"
          icon="sparkles"
          disabled={disabled || !ready}
          onClick={() => {
            if (!context || !ready) return;
            if (s.page !== 'chat') w.seedChange(prepared);
            void (useAgents
              ? w.startChangeWorkflow(prepared, models, context)
              : w.prepareChange(prepared, true, models.create, context));
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
