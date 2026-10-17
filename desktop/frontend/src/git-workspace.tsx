import { useEffect, useState } from 'react';
import { activeChangeWorkflow, workspace as w, type State } from './workspace';
import type * as M from './models';
import { Badge, Button, Empty, Heading, Icon, Loading, Notice, Panel } from './ui';

function getWorktreeInfo(tree: M.GitWorktree, s: State) {
  const child = s.implementations?.find((item) => item.worktreePath === tree.path);
  const childChange = child?.state?.change || child?.state?.changeSeed;
  const latestSession = tree.completed?.[0] || tree.sessions?.[0];
  const running =
    activeChangeWorkflow(child?.state?.change) ||
    latestSession?.workflow_status === 'running' ||
    latestSession?.workflow_status === 'canceling';
  const statusLabel = tree.mergeable
    ? 'Ready to Merge'
    : running
      ? 'Agent Running'
      : latestSession?.state === 'draft'
        ? 'Draft / Awaiting Commit'
        : tree.dirty
          ? 'Uncommitted Changes'
          : 'In Progress';
  const statusTone: 'green' | 'violet' | 'amber' | undefined = tree.mergeable
    ? 'green'
    : running
      ? 'violet'
      : 'amber';
  if (childChange) {
    return {
      title: childChange.title,
      kind: childChange.kind || 'fix',
      statusLabel,
      statusTone,
      commitOid:
        child?.state?.change?.commit?.oid ||
        child?.state?.changeReceipt?.commit?.oid ||
        latestSession?.commit?.oid,
    };
  }
  if (latestSession) {
    return {
      title: latestSession.title,
      kind: latestSession.kind || 'fix',
      statusLabel,
      statusTone,
      commitOid: latestSession.commit?.oid,
    };
  }
  let kind = 'fix';
  if (tree.branch.includes('security') || tree.branch.includes('sec-')) kind = 'security';
  else if (tree.branch.includes('perf') || tree.branch.includes('performance'))
    kind = 'performance';
  else if (tree.branch.includes('feature')) kind = 'feature';

  const cleanTitle = tree.branch
    .replace(/^(codex|fix|perf|security|feature)[/-]/, '')
    .replace(/[-_]/g, ' ')
    .trim();
  const title = cleanTitle ? cleanTitle[0].toUpperCase() + cleanTitle.slice(1) : tree.branch;
  return { title, kind, statusLabel, statusTone, commitOid: undefined };
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
  const [gitTab, setGitTab] = useState<'worktrees' | 'branch'>('worktrees');
  const run = (key: string, action: () => Promise<void>) => {
    if (w.state.busy) return;
    setPending(key);
    void action().finally(() => setPending(''));
  };
  const repository = s.gitWorkspace;
  const worktrees = repository?.worktrees.filter((tree) => !tree.current) || [];
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
            <Notice
              tone={
                s.gitResult.error ||
                s.gitResult.state === 'conflicts' ||
                s.gitResult.state === 'failed'
                  ? 'error'
                  : s.gitResult.warnings.length > 0
                    ? 'warning'
                    : 'success'
              }
            >
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
          <div className="toolbar">
            <div className="mode-switch-group" role="tablist" aria-label="Git workspace views">
              <button
                type="button"
                role="tab"
                aria-selected={gitTab === 'worktrees'}
                className={`mode-switch-btn ${gitTab === 'worktrees' ? 'active' : ''}`}
                onClick={() => setGitTab('worktrees')}
              >
                1. Worktrees &amp; Merge ({worktrees.length})
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={gitTab === 'branch'}
                className={`mode-switch-btn ${gitTab === 'branch' ? 'active' : ''}`}
                onClick={() => setGitTab('branch')}
              >
                2. Current Branch &amp; Working Tree ({repository.branch || 'Detached'})
              </button>
            </div>
          </div>
          <div className="git-split-grid">
            {gitTab === 'worktrees' ? (
              <Panel title={`Agent Worktrees (${worktrees.length})`}>
                {worktrees.length === 0 ? (
                  <Empty
                    title="No active worktrees"
                    detail={
                      repository.dirty
                        ? 'Click "Fix" on any issue to launch an isolated worktree from HEAD. Commit or stash local changes before merging a completed worktree back.'
                        : 'Click "Fix" on any issue to launch an isolated agent worktree.'
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
                              <Badge value={info.statusLabel} tone={info.statusTone} />
                              {info.commitOid && (
                                <Badge value={info.commitOid.slice(0, 7)} tone="green" />
                              )}
                            </div>
                            <small>
                              <span className="path font-mono">{tree.branch}</span>
                              <span className="muted"> · {tree.path}</span>
                            </small>
                          </div>
                          <div className="git-worktree-actions">
                            <Button
                              tone="ghost small"
                              icon="sparkles"
                              disabled={blocked}
                              onClick={() => void w.selectAgent(tree.path)}
                            >
                              Open in Agents
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
                            {tree.mergeable && (
                              <Button
                                tone="primary small"
                                disabled={blocked}
                                loading={pending === `merge:${tree.path}`}
                                onClick={() =>
                                  run(`merge:${tree.path}`, () => w.gitAction('merge', tree))
                                }
                              >
                                Merge into {repository.branch || 'branch'}
                              </Button>
                            )}
                          </div>
                        </article>
                      );
                    })}
                  </div>
                )}
              </Panel>
            ) : (
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
                    Agent fixes run in isolated Git worktrees and merge into{' '}
                    <code>{repository.branch || 'current branch'}</code> after you Accept &amp;
                    Commit.
                  </p>
                  <div className="actions">
                    <Button
                      tone="primary small"
                      icon="sparkles"
                      onClick={() => void w.navigate('agents')}
                    >
                      Open Agent Dashboard
                    </Button>
                    <Button
                      tone="ghost small"
                      icon="code"
                      onClick={() => void w.navigate('editor')}
                    >
                      Open Code &amp; Diff
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
            )}
          </div>
        </>
      )}
    </div>
  );
}
