import { useEffect, useState } from 'react';
import { workspace as w, type State } from './workspace';
import type * as M from './models';
import { Button, Empty, Heading, Icon, Loading, Notice, Overlay, Panel } from './ui';

function getWorktreeInfo(tree: M.GitWorktree, s: State) {
  const child = s.implementations?.find((item) => item.worktreePath === tree.path);
  const childChange = child?.state?.change || child?.state?.changeSeed;
  if (childChange) {
    return {
      title: childChange.title,
      kind: childChange.kind || 'fix',
    };
  }
  const lastCompleted = tree.completed?.[0];
  if (lastCompleted) {
    return {
      title: lastCompleted.title,
      kind: lastCompleted.kind || 'fix',
    };
  }
  let kind = 'fix';
  if (tree.branch.includes('security') || tree.branch.includes('sec-')) kind = 'security';
  else if (tree.branch.includes('perf') || tree.branch.includes('performance'))
    kind = 'performance';
  else if (tree.branch.includes('feature')) kind = 'feature';

  const cleanTitle = tree.branch
    .replace(/^(fix|perf|security|feature)[/-]/, '')
    .replace(/[-_]/g, ' ')
    .trim();
  const title = cleanTitle ? cleanTitle[0].toUpperCase() + cleanTitle.slice(1) : tree.branch;
  return { title, kind };
}

function kindIcon(kind: string) {
  switch (kind) {
    case 'security':
      return 'shield';
    case 'performance':
      return 'gauge';
    case 'feature':
      return 'sparkles';
    default:
      return 'bug';
  }
}

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
  useEffect(() => {
    if (!repository && !busy && !s.resourceErrors['Git / Worktrees']) {
      void w.loadGitWorkspace();
    }
  }, [repository, busy, s.resourceErrors]);
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
          <div className="git-split-grid">
            <Panel title={`Current Branch (${repository.branch || 'Detached HEAD'})`}>
              <div className="stack">
                <div className="row between wrap">
                  <span className="small muted font-mono">HEAD: {repository.head || '—'}</span>
                  <span
                    className={`badge ${repository.dirty ? 'amber' : 'green'}`}
                    style={{ fontWeight: 600 }}
                  >
                    {repository.dirty ? 'Uncommitted changes' : 'Working tree clean'}
                  </span>
                </div>
                <p className="small muted">
                  In-place proposals and direct file saves apply directly to{' '}
                  <code>{repository.branch || 'current branch'}</code> with guarded Undo.
                </p>
                <div className="actions">
                  <Button
                    tone="primary small"
                    icon="code"
                    onClick={() => void w.navigate('editor')}
                  >
                    Open Code & Diff
                  </Button>
                  {s.changeReceipt?.undo_available && (
                    <Button
                      tone="ghost small"
                      icon="undo"
                      disabled={busy}
                      onClick={() => void w.undoChange()}
                    >
                      Undo Last Change
                    </Button>
                  )}
                </div>
              </div>
            </Panel>

            <Panel title="Worktrees">
              {worktrees.length === 0 ? (
                <Empty
                  title="No worktrees ready to merge"
                  detail={
                    repository.dirty
                      ? 'Commit or move local changes in the current branch, then refresh Git.'
                      : 'Check the Worktree toggle in the Code & Diff prompt bar to run tasks in isolated worktrees.'
                  }
                />
              ) : (
                <div className="git-worktrees">
                  {worktrees.map((tree) => {
                    const info = getWorktreeInfo(tree, s);
                    const icon = kindIcon(info.kind);
                    return (
                      <article className="git-worktree" key={tree.path} aria-label={tree.branch}>
                        <span
                          className={`finding-mark ${info.kind === 'security' ? 'red' : info.kind === 'performance' ? 'blue' : ''}`}
                        >
                          <Icon name={icon} />
                        </span>
                        <div className="list-copy">
                          <div className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
                            <strong>{info.title}</strong>
                            <span
                              className={`severity-tag ${info.kind === 'security' ? 'danger' : info.kind === 'performance' ? 'warning' : 'info'}`}
                            >
                              {info.kind.toUpperCase()}
                            </span>
                          </div>
                          <small>
                            <span className="path font-mono">{tree.branch}</span>
                            <span className="muted"> · {tree.path}</span>
                          </small>
                        </div>
                        <div className="git-worktree-actions">
                          <Button
                            tone="ghost small"
                            disabled={blocked}
                            onClick={() => void w.openWorktree(tree.path)}
                          >
                            View implementation
                          </Button>
                          <Button
                            tone="danger small"
                            disabled={blocked}
                            loading={pending === `discard:${tree.path}`}
                            onClick={() =>
                              run(`discard:${tree.path}`, () => w.discardWorktree(tree))
                            }
                          >
                            Discard
                          </Button>
                          <Button
                            tone="primary small"
                            disabled={blocked}
                            loading={pending === `merge:${tree.path}`}
                            onClick={() =>
                              run(`merge:${tree.path}`, () => w.gitAction('merge', tree))
                            }
                          >
                            Merge
                          </Button>
                        </div>
                      </article>
                    );
                  })}
                </div>
              )}
            </Panel>
          </div>
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
