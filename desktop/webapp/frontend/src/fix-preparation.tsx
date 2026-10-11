import { useEffect, useId, useState } from 'react';
import type { ChangeSeed } from './models';
import { workspace as w, workflowSeed, type FixPreparationContext, type State } from './workspace';
import { defaultWorkflowModels } from './change-workflow';
import { Button, Icon, Notice, Panel } from './ui';

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
  const [useWorktree, setUseWorktree] = useState(false);
  const [worktreePath, setWorktreePath] = useState('');
  const [worktreeBranch, setWorktreeBranch] = useState('');
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
      {context && (
        <p id={permissionsID} className="small muted fix-preparation-note">
          {seed.kind === 'security' && 'Security fix · '}
          {remote && 'Shares files and instructions with selected remote models · '}
          Runs isolated checks
        </p>
      )}
      {context && (
        <div className="stack section-gap">
          <label className="row small muted align-center" style={{ gap: '0.5rem' }}>
            <input
              type="checkbox"
              checked={useWorktree}
              onChange={(e) => {
                setUseWorktree(e.target.checked);
                if (e.target.checked && !worktreeBranch) {
                  const safeName = (seed.title || 'worktree-task')
                    .toLowerCase()
                    .replace(/[^a-z0-9_-]/g, '-');
                  setWorktreeBranch(`fix/${safeName}`);
                  setWorktreePath(`../${safeName}`);
                }
              }}
            />
            Launch in a new Git Worktree
          </label>
          {useWorktree && (
            <div className="row wrap gap" style={{ marginTop: '0.25rem' }}>
              <input
                type="text"
                className="field"
                placeholder="Branch name (e.g. fix/my-feature)"
                value={worktreeBranch}
                onChange={(e) => setWorktreeBranch(e.target.value)}
              />
              <input
                type="text"
                className="field"
                placeholder="Worktree path (e.g. ../my-feature)"
                value={worktreePath}
                onChange={(e) => setWorktreePath(e.target.value)}
              />
            </div>
          )}
        </div>
      )}
      <div className="actions">
        <Button
          tone="primary"
          icon="sparkles"
          disabled={
            disabled || !ready || (useWorktree && (!worktreeBranch.trim() || !worktreePath.trim()))
          }
          aria-describedby={context ? permissionsID : undefined}
          onClick={() => {
            if (!context || !ready) return;
            void (async () => {
              if (useWorktree && worktreeBranch.trim() && worktreePath.trim()) {
                await w.createGitWorktree(worktreePath.trim(), worktreeBranch.trim());
              }
              if (s.page !== 'chat') w.seedChange(prepared);
              if (useAgents) {
                await w.startChangeWorkflow(prepared, models, context);
              } else {
                await w.prepareChange(prepared, true, models.create, context);
              }
            })();
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
