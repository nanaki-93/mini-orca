import { useId, useState } from 'react';
import type { State } from './workspace';
import { Button, Disclosure, Overlay } from './ui';

export function CommitMessage({
  message,
  onChange,
  disabled,
}: {
  message: string;
  onChange: (message: string) => void;
  disabled: boolean;
}) {
  const messageID = useId();
  return (
    <Disclosure title="Commit message">
      <label htmlFor={messageID}>Commit message</label>
      <textarea
        id={messageID}
        value={message}
        maxLength={4096}
        rows={2}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
      />
    </Disclosure>
  );
}

export function WorktreePreparation({
  s,
  onClose,
  onPrepare,
  remote,
}: {
  s: State;
  onClose: () => void;
  onPrepare: (workspace?: { branch: string; path: string }) => void;
  remote: boolean;
}) {
  const seed = s.changeSeed;
  const task = seed?.kind === 'feature' ? 'feature' : 'fix';
  const title = seed?.title || 'fix';
  const slug =
    title
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 48) || 'fix';
  //add a random suffix to the slug to avoid conflicts with existing worktrees
  const randomSuffix = Math.random().toString(36).substring(2, 8);
  const [mode, setMode] = useState('current');
  const [branch, setBranch] = useState(`codex/${slug}-${randomSuffix}`);
  const [path, setPath] = useState(`${s.project!.path}/.worktrees/${slug}-${randomSuffix}`);
  return (
    <Overlay title={`Prepare ${task} workspace`} onClose={onClose}>
      <form
        className="stack"
        onSubmit={(event) => {
          event.preventDefault();
          onPrepare(mode === 'new' ? { branch: branch.trim(), path: path.trim() } : undefined);
        }}
      >
        <fieldset className="git-workspace-options">
          <legend>Where should this {task} be prepared?</legend>
          <label>
            <input
              type="radio"
              name="fix-workspace"
              checked={mode === 'current'}
              onChange={() => setMode('current')}
            />
            Current workspace · {s.repository?.branch || 'Detached HEAD'}
          </label>
          <label>
            <input
              type="radio"
              name="fix-workspace"
              checked={mode === 'new'}
              onChange={() => setMode('new')}
            />
            New Git worktree
          </label>
        </fieldset>
        {mode === 'new' && (
          <>
            <label>
              New branch name
              <input
                value={branch}
                maxLength={240}
                onChange={(event) => setBranch(event.target.value)}
              />
            </label>
            <label>
              Worktree path
              <input value={path} onChange={(event) => setPath(event.target.value)} />
            </label>
            <p className="small muted">
              Starts from the current commit. Local uncommitted changes stay here. Choose a new
              directory inside this project's .worktrees folder. Your current project and branch
              stay open while the implementation runs separately.
            </p>
            <p className="small muted">
              Trust this project for local execution. Creating a worktree can run configured
              checkout filters.
            </p>
          </>
        )}
        <p className="small muted">
          {remote && 'Shares selected files and instructions with the selected remote models. '}
          Prepares the selected files with the configured models and runs isolated checks. No fix is
          applied until you review it.
        </p>
        <div className="actions end">
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button
            type="submit"
            tone="primary"
            disabled={
              !!s.busy ||
              (mode === 'new' &&
                (!branch.trim() ||
                  !path.trim() ||
                  !s.repository?.head ||
                  !s.repository.at_root ||
                  !s.repository.branch))
            }
          >
            {mode === 'new' ? 'Create worktree & prepare' : 'Prepare in current workspace'}
          </Button>
        </div>
      </form>
    </Overlay>
  );
}
