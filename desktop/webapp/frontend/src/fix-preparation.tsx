import { useEffect, useId, useState } from 'react';
import type { ChangeSeed } from './models';
import { workspace as w, workflowSeed, type FixPreparationContext, type State } from './workspace';
import { defaultWorkflowModels } from './change-workflow';
import { Button, Disclosure, Icon, Notice, Panel } from './ui';

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
  const permissionsID = useId();
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
  const remote = profiles.some((profile) => context?.models.scopes[profile]?.remote_provider);
  return (
    <Panel className="fix-preparation">
      {error ? (
        <Notice error>{error}</Notice>
      ) : !context ? (
        <p>Loading models and permissions…</p>
      ) : (
        <>
          <h2 className="fix-scope-title">Files in this fix</h2>
          <ul className="fix-file-list">
            {prepared.paths.map((path) => (
              <li key={path}>
                <Icon name="file" />
                <span className="path">{path}</span>
              </li>
            ))}
          </ul>
        </>
      )}
      <div className="actions">
        <Button
          tone="primary"
          icon="sparkles"
          disabled={disabled || !ready}
          aria-describedby={context ? permissionsID : undefined}
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
      </div>
      <div className="fix-checks">
        <Disclosure title="Permissions & checks">
          {context && (
            <span id={permissionsID} className="small muted">
              {seed.kind === 'security' && 'Security fix · '}
              {remote && 'Shares files and instructions with selected remote models · '}
              Runs isolated checks
            </span>
          )}
          {context && (
            <Disclosure title="Project checks">
              {context.trust.commands.map((argv, index) => (
                <p key={index}>
                  <code>{argv.join(' ')}</code>
                </p>
              ))}
            </Disclosure>
          )}
          <Button
            icon="refresh"
            disabled={disabled}
            onClick={() => setRefresh((value) => value + 1)}
          >
            Refresh preparation
          </Button>
        </Disclosure>
      </div>
    </Panel>
  );
}
