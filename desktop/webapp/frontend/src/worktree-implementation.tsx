import { useSyncExternalStore } from 'react';
import { workspace, canCancelOperation, type State, type Workspace } from './workspace';
import { WorkspaceContext } from './workspace-context';
import { ChangeWorkspace } from './change-workspace';
import { ChangeHistory } from './change-shared';
import { Instructions } from './instructions';
import { Editor, Context } from './editor';
import { Models } from './overview';
import { Badge, Button, Modal, Notice, Panel } from './ui';

export function Implementations({ s }: { s: State }) {
  if (!s.implementations.length) return null;
  return (
    <Panel title="Worktree implementations">
      {s.implementations.map((child) => (
        <div className="list-row" key={child.worktreePath}>
          <div className="list-copy">
            <strong>{child.state.change?.title || child.state.project?.name}</strong>
            <small className="path">{child.worktreePath}</small>
          </div>
          <Badge
            value={
              child.state.error
                ? 'failed'
                : child.state.busy ||
                  child.state.change?.workflow?.status ||
                  child.state.change?.state ||
                  'saved'
            }
          />
          <Button onClick={() => void workspace.openWorktree(child.worktreePath!)}>
            View implementation
          </Button>
        </div>
      ))}
    </Panel>
  );
}

export function WorktreeImplementation({ child }: { child: Workspace }) {
  const s = useSyncExternalStore(child.subscribe, child.snapshot);
  return (
    <WorkspaceContext.Provider value={child}>
      <div className="workspace-page">
        <Panel title={`Implementation · ${s.repository?.branch || s.project?.name}`}>
          <p className="path">{child.worktreePath}</p>
          <p className="small muted">
            Review and Apply affect this worktree. The project's current branch stays open.
          </p>
          <Button onClick={() => void workspace.navigate('changes')}>All implementations</Button>
        </Panel>
        {s.busy && (
          <Notice>
            {s.busy}…{' '}
            {canCancelOperation(s.busy) && (
              <Button onClick={() => void child.cancel()}>Cancel</Button>
            )}
          </Notice>
        )}
        {s.error && <Notice error>{s.error}</Notice>}
        {s.notice && <Notice>{s.notice}</Notice>}
        {Object.entries(s.resourceErrors).map(([key, error]) => (
          <Notice error key={key}>
            {key}: {error}
          </Notice>
        ))}
        {s.page === 'history' ? (
          <ChangeHistory s={s} />
        ) : s.page === 'instructions' ? (
          <Instructions s={s} />
        ) : s.page === 'editor' ? (
          <Editor s={s} />
        ) : s.page === 'context' ? (
          <Context s={s} />
        ) : s.page === 'models' ? (
          <Models s={s} />
        ) : (
          <ChangeWorkspace s={s} />
        )}
        {s.confirmation && <Modal key={s.confirmation.title} />}
      </div>
    </WorkspaceContext.Provider>
  );
}
