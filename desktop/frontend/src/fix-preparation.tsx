import { useWorkspace } from './workspace-context';
import { WorktreePreparation } from './git-controls';
import { useEffect, useId, useState } from 'react';
import type { ChangeSeed } from './models';
import {
  effectiveWorkflowModel,
  workflowSeed,
  workflowSetupModels,
  type FixPreparationContext,
  type State,
} from './workspace';
import { Button, Icon, Loading, Notice, Panel } from './ui';

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
  const w = useWorkspace();
  const [context, setContext] = useState<FixPreparationContext>();
  const [error, setError] = useState('');
  const [choosingWorkspace, setChoosingWorkspace] = useState(false);
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
  const models = workflowSetupModels(s);
  const profiles = useAgents ? Object.values(models) : [models.create];
  const modelContext = context && {
    models: context.models,
    availableModels: context.availableModels || s.availableModels,
  };
  const ready =
    modelContext && profiles.every((profile) => effectiveWorkflowModel(modelContext, profile));
  const remote = profiles.some(
    (profile) => modelContext && effectiveWorkflowModel(modelContext, profile)?.remote_provider,
  );
  const begin = (workspace?: { branch: string; path: string }) => {
    if (!context || !ready) return;
    setChoosingWorkspace(false);
    if (workspace) {
      void w.prepareFixInWorktree(prepared, models, context, workspace.branch, workspace.path);
      return;
    }
    if (s.page !== 'chat') w.seedChange(prepared);
    void (useAgents
      ? w.startChangeWorkflow(prepared, models, context)
      : w.prepareChange(prepared, true, models.create, context));
  };
  return (
    <Panel className="fix-preparation">
      {choosingWorkspace && (
        <WorktreePreparation
          s={{ ...s, changeSeed: prepared }}
          onClose={() => setChoosingWorkspace(false)}
          onPrepare={begin}
          remote={remote}
        />
      )}
      {error ? (
        <Notice error>{error}</Notice>
      ) : !context ? (
        <Loading label="Loading models and permissions…" />
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
      {context && (
        <p id={permissionsID} className="small muted fix-preparation-note">
          {seed.kind === 'security' && 'Security fix · '}
          {remote && 'Shares files and instructions with selected remote models · '}
          Runs isolated checks
        </p>
      )}
      <div className="actions">
        <Button
          tone="primary"
          icon="sparkles"
          disabled={disabled || !ready}
          aria-describedby={context ? permissionsID : undefined}
          onClick={() => {
            if (!context || !ready) return;
            if (context.repository.available && !w.worktreePath) setChoosingWorkspace(true);
            else begin();
          }}
        >
          {label}
        </Button>
      </div>
      {(error || (context && !ready)) && (
        <Button icon="refresh" disabled={disabled} onClick={() => setRefresh((value) => value + 1)}>
          Refresh preparation
        </Button>
      )}
    </Panel>
  );
}
