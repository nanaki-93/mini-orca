import { useId, useState } from 'react';
import { workspace as w, type State } from './workspace';
import { Badge, Button, Empty, Heading, Notice, Overlay, Panel } from './ui';

export function GitWorkspace({ s }: { s: State }) {
  const repository = s.gitWorkspace;
  const [branch, setBranch] = useState('');
  const branchID = useId();
  const busy = !!s.busy;
  const blocked = busy || !!s.resourceErrors['Git / Worktrees'] || s.uncertain;
  return (
    <div className="workspace-page git-workspace">
      <Heading
        title="Git / Worktrees"
        detail="Switch branches, review completed fixes, and merge isolated work."
        variant="intro"
      >
        <Button icon="refresh" disabled={busy} onClick={() => void w.loadGitWorkspace()}>
          Refresh Git
        </Button>
      </Heading>
      {s.uncertain && (
        <Notice error>
          <strong>Git outcome unknown.</strong>
          <p>Git may have changed files. Refresh project facts before another action.</p>
          <Button disabled={busy} onClick={() => void w.reindex().then(() => w.loadGitWorkspace())}>
            Refresh project
          </Button>
        </Notice>
      )}
      {!repository ? (
        <Empty title="Loading Git status…" />
      ) : !repository.available ? (
        <Empty
          title="This project is not a Git repository"
          detail="Initialize Git and create an initial commit, then refresh Git status."
        />
      ) : (
        <>
          {(!repository.at_root || !repository.head || !repository.branch) && (
            <Notice error>
              {!repository.at_root
                ? 'Open the repository root to use Git actions.'
                : !repository.head
                  ? 'Create an initial commit before using Git workflows.'
                  : 'Detached HEAD. Check out a local branch using Git before preparing changes.'}
            </Notice>
          )}
          {s.gitResult && (
            <Notice error={!!s.gitResult.error || s.gitResult.state === 'conflicts'}>
              <strong>
                {s.gitResult.state === 'conflicts'
                  ? 'Merge conflicts need review.'
                  : `Git operation: ${s.gitResult.state}`}
              </strong>
              {s.gitResult.error && <p>{s.gitResult.error}</p>}
              {s.gitResult.removed_worktree && (
                <p>Removed worktree: {s.gitResult.removed_worktree}. The branch is kept.</p>
              )}
              {s.gitResult.warnings.map((warning) => (
                <p key={warning}>{warning}</p>
              ))}
            </Notice>
          )}
          {repository.merge_head && (
            <Panel title="Merge in progress" className="git-conflicts">
              <p>
                Review these files and resolve the merge using your Git tools. You can also abort
                and restore the state before this merge.
              </p>
              {repository.conflicts.length ? (
                <ul>
                  {repository.conflicts.map((path) => (
                    <li className="path" key={path}>
                      {path}
                    </li>
                  ))}
                </ul>
              ) : (
                <p>Conflicts are resolved in the index. Complete the merge using Git.</p>
              )}
              <Button tone="danger" disabled={busy} onClick={() => void w.gitAction('abort-merge')}>
                Abort merge
              </Button>
            </Panel>
          )}
          {repository.operation && (
            <Notice error>
              Finish or abort the current Git operation ({repository.operation}) using your Git
              tools.
            </Notice>
          )}
          <Panel title="Current branch">
            <div className="git-branch-row">
              <div>
                <strong className="git-branch-name">{repository.branch || 'Detached HEAD'}</strong>
                <p className="path small muted">{repository.root}</p>
              </div>
              <Badge value={repository.dirty ? 'Uncommitted changes' : 'Clean'} />
            </div>
            <div className="actions git-switcher">
              <div className="git-branch-control">
                <label htmlFor={branchID}>Switch to branch</label>
                <select
                  id={branchID}
                  className="field"
                  value={branch}
                  onChange={(event) => setBranch(event.target.value)}
                >
                  <option value="">Choose a local branch</option>
                  {repository.branches.map((item) => (
                    <option
                      key={item.name}
                      value={item.name}
                      disabled={
                        item.name === repository.branch ||
                        (!!item.worktree_path && item.worktree_path !== repository.root)
                      }
                    >
                      {item.name}
                      {item.name === repository.branch
                        ? ' · current'
                        : item.worktree_path
                          ? ' · in another worktree'
                          : ''}
                    </option>
                  ))}
                </select>
              </div>
              <Button
                disabled={
                  blocked ||
                  repository.dirty ||
                  !branch ||
                  branch === repository.branch ||
                  !!repository.merge_head ||
                  !!repository.operation ||
                  !repository.at_root ||
                  !repository.head
                }
                onClick={() => void w.gitAction('switch', branch)}
              >
                Switch branch
              </Button>
            </div>
            {repository.dirty && (
              <p className="small muted">
                Commit or move your local changes before switching or merging.
              </p>
            )}
          </Panel>
          <Panel title="Worktrees">
            <p className="small muted">
              To create a worktree, select Prepare fix on a finding or Generate changes in a new
              feature conversation.
            </p>
            <div className="git-worktrees">
              {repository.worktrees.map((tree) => (
                <article className="git-worktree" key={tree.path}>
                  <div className="git-branch-row">
                    <div>
                      <h3>{tree.branch || 'Detached HEAD'}</h3>
                      <p className="path small">{tree.path}</p>
                    </div>
                    <Badge
                      value={
                        tree.current
                          ? 'Current'
                          : tree.locked
                            ? 'Locked'
                            : tree.prunable
                              ? 'Unavailable'
                              : tree.dirty === undefined
                                ? 'Status unavailable'
                                : tree.dirty
                                  ? 'Uncommitted changes'
                                  : 'Clean'
                      }
                    />
                  </div>
                  {tree.error && <Notice error>{tree.error}</Notice>}
                  <p>
                    {tree.error
                      ? 'Completed fix history may be unavailable.'
                      : `${tree.completed.length} completed ${tree.completed.length === 1 ? 'fix' : 'fixes'}`}
                  </p>
                  {tree.completed.length > 0 && (
                    <ul>
                      {tree.completed.map((fix) => (
                        <li key={fix.id}>{fix.title}</li>
                      ))}
                    </ul>
                  )}
                  {!tree.current && (
                    <Button
                      disabled={blocked || tree.prunable}
                      onClick={() => void w.openWorktree(tree.path)}
                    >
                      View implementations
                    </Button>
                  )}
                  {!tree.current && (
                    <Button
                      disabled={blocked || tree.prunable}
                      onClick={() => void w.openProject(tree.path, false)}
                    >
                      Open project here
                    </Button>
                  )}
                  {!tree.current && (
                    <Button
                      icon="branch"
                      disabled={
                        blocked ||
                        !!tree.error ||
                        tree.dirty !== false ||
                        tree.locked ||
                        tree.prunable ||
                        !tree.branch ||
                        tree.head === repository.head ||
                        repository.dirty ||
                        !!repository.merge_head ||
                        !!repository.operation ||
                        !repository.at_root ||
                        !repository.branch
                      }
                      onClick={() => void w.previewGitMerge(tree)}
                    >
                      Merge into Current Branch
                    </Button>
                  )}
                </article>
              ))}
            </div>
          </Panel>
        </>
      )}
      {s.gitPreview && (
        <Overlay title="Review worktree merge" onClose={() => w.set({ gitPreview: undefined })}>
          <p>
            <strong>{s.gitPreview.source.branch}</strong> into{' '}
            <strong>{s.gitPreview.target.branch}</strong>
          </p>
          <p className="small muted">
            Incoming changes since the common ancestor. Git will merge these with your current
            branch. After a successful merge, the source worktree and its local task history are
            deleted, and its implementation closes. The branch is kept.
          </p>
          <h3>Incoming commits</h3>
          <pre tabIndex={0}>{s.gitPreview.commits || 'No new commits.'}</pre>
          <h3>Changes to review</h3>
          <pre className="git-merge-diff" tabIndex={0}>
            {s.gitPreview.diff || 'No file differences.'}
          </pre>
          <div className="actions end">
            <Button disabled={busy} onClick={() => w.set({ gitPreview: undefined })}>
              Cancel
            </Button>
            <Button
              tone="primary"
              disabled={blocked || !s.gitPreview.commits.trim()}
              onClick={() => void w.gitAction('merge')}
            >
              Merge into current branch
            </Button>
          </div>
        </Overlay>
      )}
    </div>
  );
}
