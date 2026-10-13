import { useState } from 'react';
import { workspace as w, type State } from './workspace';
import { Button, Empty, Heading, Loading, Notice, Overlay, Panel } from './ui';

export function GitWorkspace({ s }: { s: State }) {
  const [pending, setPending] = useState('');
  const run = (key: string, action: () => Promise<void>) => {
    if (w.state.busy) return;
    setPending(key);
    void action().finally(() => setPending(''));
  };
  const repository = s.gitWorkspace;
  const worktrees = repository?.worktrees.filter((tree) => tree.mergeable) || [];
  const failed = repository?.worktrees.filter((tree) => tree.error) || [];
  const busy = !!s.busy;
  const blocked = busy || !!s.resourceErrors['Git / Worktrees'] || s.uncertain;
  return (
    <div className="workspace-page git-workspace">
      <Heading title="Git / Worktrees" variant="intro">
        <Button
          icon="refresh"
          disabled={busy}
          loading={s.busy === 'Refresh Git'}
          onClick={() => void w.act('Refresh Git', () => w.loadGitWorkspace())}
        >
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
        s.resourceErrors['Git / Worktrees'] ? (
          <Empty title="Git status unavailable" />
        ) : (
          <Loading label="Loading Git status…" />
        )
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
              <p>Resolve merge conflicts or abort merge.</p>
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
          {failed.length > 0 && (
            <Notice error>
              <strong>Some worktree statuses are unavailable. Refresh Git to retry.</strong>
              {failed.map((tree) => (
                <p key={tree.path}>
                  {tree.branch || tree.path}: {tree.error}
                </p>
              ))}
            </Notice>
          )}
          <Panel title="Worktrees">
            {worktrees.length === 0 ? (
              <Empty
                title="No worktrees ready to merge"
                detail={
                  repository.dirty
                    ? 'Commit or move local changes in the current branch, then refresh Git.'
                    : undefined
                }
              />
            ) : (
              <div className="git-worktrees">
                {worktrees.map((tree) => (
                  <article className="git-worktree" key={tree.path} aria-label={tree.branch}>
                    <h3>{tree.branch}</h3>
                    <div className="git-worktree-actions">
                      <Button disabled={blocked} onClick={() => void w.openWorktree(tree.path)}>
                        View implementation
                      </Button>
                      <Button
                        tone="danger"
                        disabled={blocked}
                        loading={pending === `discard:${tree.path}`}
                        onClick={() => run(`discard:${tree.path}`, () => w.discardWorktree(tree))}
                      >
                        Discard change
                      </Button>
                      <Button
                        tone="primary"
                        disabled={blocked}
                        loading={pending === `merge:${tree.path}`}
                        onClick={() => run(`merge:${tree.path}`, () => w.gitAction('merge', tree))}
                      >
                        Merge into current branch
                      </Button>
                    </div>
                  </article>
                ))}
              </div>
            )}
          </Panel>
        </>
      )}
      {s.gitPreview && (
        <Overlay title="Review worktree merge" onClose={() => w.set({ gitPreview: undefined })}>
          <p>
            <strong>{s.gitPreview.source.branch}</strong> into{' '}
            <strong>{s.gitPreview.target.branch}</strong>
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
              loading={s.busy === 'Update Git workspace'}
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
