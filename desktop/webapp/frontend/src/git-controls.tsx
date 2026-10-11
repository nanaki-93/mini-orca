import { useId, useState } from 'react';
import type { State } from './workspace';
import { Button, Overlay } from './ui';

export function CommitDialog({
  s,
  title,
  message: initialMessage,
  onClose,
  onCommit,
}: {
  s: State;
  title: string;
  message: string;
  onClose: () => void;
  onCommit: (message: string) => void;
}) {
  const [message, setMessage] = useState(initialMessage);
  const messageID = useId();
  return (
    <Overlay title={title} onClose={onClose}>
      <form
        className="stack"
        onSubmit={(event) => {
          event.preventDefault();
          if (message.trim()) onCommit(message.trim());
        }}
      >
        <p>
          {title === 'Retry commit'
            ? 'Commit the already applied files'
            : 'Apply the reviewed files and commit them'}{' '}
          on{' '}
          <strong>
            {title === 'Retry commit' ? s.changeReceipt?.commit?.branch : s.repository?.branch}
          </strong>
          .
        </p>
        <div className="stack compact-stack">
          <label htmlFor={messageID}>Commit message</label>
          <textarea
            id={messageID}
            data-initial-focus
            value={message}
            maxLength={4096}
            rows={3}
            onChange={(event) => setMessage(event.target.value)}
          />
        </div>
        <p className="small muted">
          Only the reviewed files are included. Hooks and signing are skipped. Nothing is pushed.
        </p>
        <div className="actions end">
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" tone="primary" disabled={!!s.busy || !message.trim()}>
            {title}
          </Button>
        </div>
      </form>
    </Overlay>
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
  const title = seed?.title || 'fix';
  const slug =
    title
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 48) || 'fix';
  const [mode, setMode] = useState('current');
  const [branch, setBranch] = useState(`codex/${slug}`);
  const [path, setPath] = useState(`${s.project!.path}-${slug}`);
  return (
    <Overlay title="Prepare fix workspace" onClose={onClose}>
      <form
        className="stack"
        onSubmit={(event) => {
          event.preventDefault();
          onPrepare(mode === 'new' ? { branch: branch.trim(), path: path.trim() } : undefined);
        }}
      >
        <fieldset className="git-workspace-options">
          <legend>Where should this fix be prepared?</legend>
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
              directory whose parent already exists.
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
