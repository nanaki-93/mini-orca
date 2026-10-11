import { useEffect, useState } from 'react';
import { workspace as w, type State } from './workspace';
import { Badge, Button, Empty, Notice, Panel } from './ui';

export function GitWorkspace({ s }: { s: State }) {
  const [selectedBranch, setSelectedBranch] = useState('');
  const [commitMsg, setCommitMsg] = useState('');
  const [newWtPath, setNewWtPath] = useState('');
  const [newWtBranch, setNewWtBranch] = useState('');

  useEffect(() => {
    void w.listGitBranches();
    void w.listGitWorktrees();
  }, [s.project?.project_revision]);

  const branches = s.gitBranches || [];
  const worktrees = s.gitWorktrees || [];
  const currentBranch = branches.find((b) => b.is_current)?.name || s.git?.branch || 'HEAD';

  return (
    <div className="workspace-page guided-fix">
      <header className="page-heading fix-heading">
        <div>
          <span className="studio-eyebrow">Git & Worktrees</span>
          <h1>Repository & Worktree Management</h1>
          <p className="results-detail-meta">
            Active branch: <strong>{currentBranch}</strong>
          </p>
        </div>
        <div className="actions">
          <Button
            tone="ghost"
            icon="refresh"
            disabled={!!s.busy}
            onClick={() => {
              void w.listGitBranches();
              void w.listGitWorktrees();
            }}
          >
            Refresh Git State
          </Button>
        </div>
      </header>

      {s.notice && <Notice>{s.notice}</Notice>}
      {s.error && <Notice error>{s.error}</Notice>}

      <div className="stack section-gap">
        <Panel title="Branch Switcher">
          <p className="small muted">Switch the active Git branch for this repository workspace.</p>
          <div className="row wrap align-center gap" style={{ marginTop: '0.5rem' }}>
            <select
              className="field"
              value={selectedBranch || currentBranch}
              onChange={(e) => setSelectedBranch(e.target.value)}
              disabled={!!s.busy}
            >
              {branches.map((b) => (
                <option key={b.name} value={b.name}>
                  {b.name} {b.is_current ? '(current)' : ''}
                </option>
              ))}
            </select>
            <Button
              tone="primary"
              disabled={!!s.busy || !selectedBranch || selectedBranch === currentBranch}
              onClick={() => {
                if (selectedBranch && selectedBranch !== currentBranch) {
                  void w.switchGitBranch(selectedBranch);
                }
              }}
            >
              Switch Branch
            </Button>
          </div>
        </Panel>

        <Panel title="Quick Commit">
          <p className="small muted">
            Stage all modified changes and commit to the current branch ({currentBranch}).
          </p>
          <div className="row wrap align-center gap" style={{ marginTop: '0.5rem' }}>
            <input
              type="text"
              className="field"
              style={{ flex: 1, minWidth: '250px' }}
              placeholder="Commit message..."
              value={commitMsg}
              onChange={(e) => setCommitMsg(e.target.value)}
              disabled={!!s.busy}
            />
            <Button
              tone="primary"
              disabled={!!s.busy || !commitMsg.trim()}
              onClick={() => {
                if (commitMsg.trim()) {
                  void w.commitGitChanges(commitMsg.trim());
                  setCommitMsg('');
                }
              }}
            >
              Commit Changes
            </Button>
          </div>
        </Panel>

        <Panel title="Worktrees & Merging">
          <p className="small muted">
            Manage separate worktrees to work on concurrent fixes or features without sharing the
            same workspace directory.
          </p>

          <div style={{ marginTop: '1rem' }}>
            <h3>Existing Worktrees</h3>
            {worktrees.length === 0 ? (
              <Empty title="No worktrees found" />
            ) : (
              <div className="stack gap" style={{ marginTop: '0.5rem' }}>
                {worktrees.map((wt) => {
                  const isCurrentWtBranch = wt.branch === currentBranch;
                  return (
                    <div
                      key={wt.path}
                      className="list-row row between align-center"
                      style={{
                        padding: '0.75rem',
                        background: 'var(--surface-2, #f5f5f5)',
                        borderRadius: '6px',
                      }}
                    >
                      <div>
                        <strong>{wt.branch || 'Detached / Bare'}</strong>
                        <p className="small muted" style={{ margin: 0 }}>
                          Path: {wt.path} {wt.is_main ? '· (Main Worktree)' : ''}
                        </p>
                      </div>
                      <div className="row wrap align-center gap">
                        {wt.is_main ? (
                          <Badge value="main" />
                        ) : isCurrentWtBranch ? (
                          <Badge value="current branch" />
                        ) : (
                          <Button
                            tone="primary"
                            disabled={!!s.busy || !wt.branch}
                            onClick={() => {
                              if (wt.branch) {
                                void w.mergeGitWorktree(wt.branch);
                              }
                            }}
                          >
                            Merge into {currentBranch}
                          </Button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          <div
            style={{
              marginTop: '1.5rem',
              borderTop: '1px solid var(--border)',
              paddingTop: '1rem',
            }}
          >
            <h3>Create New Worktree</h3>
            <div className="row wrap align-center gap" style={{ marginTop: '0.5rem' }}>
              <input
                type="text"
                className="field"
                placeholder="New branch name (e.g. fix/feature-a)"
                value={newWtBranch}
                onChange={(e) => setNewWtBranch(e.target.value)}
                disabled={!!s.busy}
              />
              <input
                type="text"
                className="field"
                placeholder="Worktree path (e.g. ../feature-a-wt)"
                value={newWtPath}
                onChange={(e) => setNewWtPath(e.target.value)}
                disabled={!!s.busy}
              />
              <Button
                tone="primary"
                disabled={!!s.busy || !newWtBranch.trim() || !newWtPath.trim()}
                onClick={() => {
                  if (newWtBranch.trim() && newWtPath.trim()) {
                    void w.createGitWorktree(newWtPath.trim(), newWtBranch.trim());
                    setNewWtBranch('');
                    setNewWtPath('');
                  }
                }}
              >
                Create Worktree
              </Button>
            </div>
          </div>
        </Panel>
      </div>
    </div>
  );
}
