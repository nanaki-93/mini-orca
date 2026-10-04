import { useEffect, useState } from 'react';
import { workspace as w, canApplyChange, changeChecksPassed, type State } from './workspace';
import {
  Badge,
  Button,
  Disclosure,
  Empty,
  Go,
  Heading,
  Notice,
  Panel,
  Prose,
  BulletContent,
} from './ui';

export function ChangeWorkspace({ s }: { s: State }) {
  const change = s.change;
  const [title, setTitle] = useState('New feature');
  const [paths, setPaths] = useState('');
  const [message, setMessage] = useState('');
  const [tests, setTests] = useState(true);
  useEffect(() => {
    setTitle(change?.title || s.changeSeed?.title || 'New feature');
    setPaths(
      (change?.targets.map((target) => target.path) || s.changeSeed?.paths || []).join('\n'),
    );
    setMessage(change ? '' : s.changeSeed?.message || '');
    setTests(change?.kind !== 'instructions');
  }, [change?.id, s.changeSeed]);
  const submit = () =>
    void w.prepareChange(
      {
        title,
        paths: paths
          .split('\n')
          .map((path) => path.trim())
          .filter(Boolean),
        message,
        kind: change?.kind || s.changeSeed?.kind || 'feature',
        acceptance_criteria: change?.acceptance_criteria || s.changeSeed?.acceptance_criteria || [],
      },
      tests,
    );
  const blocked =
    !!s.busy || (!!change && (change.state !== 'draft' || change.freshness !== 'current'));
  return (
    <>
      <Heading
        title="Chat"
        eyebrow="Implement a change"
        detail="Describe a task, inspect the proposal, then approve its exact diff."
      >
        <Go page="editor" icon="code">
          Inspect source
        </Go>
        <Button disabled={!!s.busy} onClick={() => w.newChange()}>
          New conversation
        </Button>
      </Heading>
      {s.uncertain && (
        <Notice error>
          The write outcome needs reconciliation. Refresh project facts before continuing.{' '}
          <Button onClick={() => void w.reindex()} disabled={!!s.busy}>
            Refresh project
          </Button>
        </Notice>
      )}
      {s.changeReceipt && (
        <Panel
          title={
            s.changeReceipt.state === 'applied'
              ? 'Change applied'
              : `Change ${s.changeReceipt.state.replaceAll('_', ' ')}`
          }
          actions={<Badge value={s.changeReceipt.state} />}
        >
          <p>Applied changes still need independent verification.</p>
          {(s.changeReceipt.warnings || []).map((warning, i) => (
            <Notice error key={i}>
              {warning}
            </Notice>
          ))}
          {s.changeReceipt.undo_available && (
            <Button disabled={!!s.busy || s.uncertain} onClick={() => void w.undoChange()}>
              Undo proposal
            </Button>
          )}
        </Panel>
      )}
      <div className="change-workspace">
        <section className="stack" aria-label="Change conversation">
          {!change ? (
            <Panel title="Task and file scope">
              <label className="block">
                Task title
                <input
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  disabled={!!s.busy}
                  maxLength={200}
                />
              </label>
              <label className="block">
                Files to change (one path per line)
                <textarea
                  aria-label="Files to change"
                  value={paths}
                  onChange={(e) => setPaths(e.target.value)}
                  disabled={!!s.busy}
                  placeholder="internal/worker/process.go&#10;internal/worker/process_test.go"
                />
              </label>
              <label className="block">
                Add an existing file
                <select
                  className="field"
                  aria-label="Add an existing file"
                  value=""
                  disabled={!!s.busy}
                  onChange={(e) => {
                    if (e.target.value && !paths.split('\n').includes(e.target.value))
                      setPaths([paths, e.target.value].filter(Boolean).join('\n'));
                  }}
                >
                  <option value="">Choose a file…</option>
                  {s.index?.files
                    .filter((file) => !file.binary && /\.(go|md)$/.test(file.path))
                    .map((file) => (
                      <option key={file.path}>{file.path}</option>
                    ))}
                </select>
              </label>
              <p className="small muted">
                Choose up to eight Go or Markdown files. You can name new files. The daemon checks
                eligibility before generation.
              </p>
            </Panel>
          ) : (
            <Panel
              title={change.title}
              actions={<Badge value={change.freshness === 'stale' ? 'stale' : change.state} />}
            >
              <p className="small muted">
                Revision {change.revision} · {change.targets.length} captured paths
              </p>
              <BulletContent
                title="File scope"
                items={change.targets.map(
                  (target) => `${target.path}${target.exists ? '' : ' · new file'}`,
                )}
              />
              <BulletContent title="Acceptance criteria" items={change.acceptance_criteria} />
              {change.freshness === 'stale' && (
                <Notice>
                  This source or its instructions changed. The old proposal is available for
                  reference; start a new conversation to prepare a fresh change.
                </Notice>
              )}
            </Panel>
          )}
          {change?.messages.map((entry, i) => (
            <Panel key={i} title={entry.role === 'user' ? 'You' : 'Assistant'}>
              <Prose text={entry.content} />
            </Panel>
          ))}
          <Panel title="Describe the change">
            <label className="sr-only" htmlFor="chat-request">
              Change request
            </label>
            <textarea
              id="chat-request"
              className="composer"
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              disabled={blocked}
              placeholder="Describe a new feature, fix, or improvement…"
            />
            <label className="checkbox-line">
              <input
                type="checkbox"
                checked={tests}
                disabled={blocked || change?.kind === 'instructions'}
                onChange={(e) => setTests(e.target.checked)}
              />
              Run project tests after generation
            </label>
            <Button
              tone="primary"
              disabled={blocked || !message.trim() || !paths.trim() || !title.trim()}
              onClick={submit}
            >
              Prepare change
            </Button>
          </Panel>
          <Disclosure title="Local history">
            {!s.changeHistory ? (
              <Empty title="History unavailable" />
            ) : s.changeHistory.length === 0 ? (
              <p>No saved conversations.</p>
            ) : (
              s.changeHistory.map((entry) => (
                <div className="list-row" key={entry.id}>
                  <span className="list-copy">
                    <strong>{entry.title}</strong>
                    <small>
                      {entry.state} · revision {entry.revision}
                    </small>
                  </span>
                  <Button disabled={!!s.busy} onClick={() => void w.resumeChange(entry.id)}>
                    Resume
                  </Button>
                </div>
              ))
            )}
            <p className="small muted">
              History stays in this project. Resume checks freshness and requires new checks and
              review.
            </p>
            <Button disabled={!!s.busy} onClick={() => void w.loadChangeHistory()}>
              Refresh history
            </Button>
          </Disclosure>
        </section>
        <section className="stack" aria-label="Proposal review">
          {change?.changes.length ? (
            <>
              <Panel
                title="Proposal diff"
                actions={<Badge value={`${change.changes.length} files`} />}
              >
                <p className="small muted">
                  Read every file change before reviewing this revision.
                </p>
                {change.kind === 'performance' && (
                  <Notice>
                    Performance changes are unmeasured. Passing tests do not establish a speedup.
                  </Notice>
                )}
              </Panel>
              {change.changes.map((edit) => (
                <div
                  className="panel diff"
                  aria-label={`Read-only diff for ${edit.path}`}
                  tabIndex={0}
                  key={edit.path}
                >
                  <div className="code-header">
                    <strong>{edit.path}</strong>
                    <span>Read-only</span>
                  </div>
                  <pre>
                    {edit.diff.lines.map((line, i) => {
                      const kind =
                        line.kind === 'add'
                          ? 'added'
                          : line.kind === 'remove'
                            ? 'removed'
                            : line.kind;
                      return (
                        <span className={`diff-line ${kind}`} key={i}>
                          <span className="line-number" aria-hidden="true">
                            {line.old_line || ''}
                          </span>
                          <span className="line-number" aria-hidden="true">
                            {line.new_line || ''}
                          </span>
                          <span className="diff-sign" aria-hidden="true">
                            {kind === 'added' ? '+' : kind === 'removed' ? '−' : ' '}
                          </span>
                          <code>{line.text || ' '}</code>
                        </span>
                      );
                    })}
                  </pre>
                </div>
              ))}
              <Panel title="Check evidence">
                {change.checks.length === 0 ? (
                  <p>No current checks.</p>
                ) : (
                  change.checks.map((check, i) => (
                    <div key={i} className="section-gap">
                      <div className="row between wrap">
                        <strong>{check.name}</strong>
                        <Badge value={check.state} />
                      </div>
                      {check.output && (
                        <Disclosure title="Diagnostics">
                          <pre>{check.output}</pre>
                        </Disclosure>
                      )}
                    </div>
                  ))
                )}
                <div className="actions section-gap">
                  <Button disabled={blocked} onClick={() => void w.checkChange(tests)}>
                    Check proposal
                  </Button>
                  {change.checks.length > 0 && !changeChecksPassed(change) && (
                    <Button
                      disabled={blocked || change.repair_attempts >= 3}
                      onClick={() => void w.repairChange()}
                    >
                      Repair failed checks
                    </Button>
                  )}
                </div>
              </Panel>
              <div className="actions end">
                <Button
                  disabled={
                    blocked || !changeChecksPassed(change) || change.reviewed_hash === change.hash
                  }
                  onClick={() => void w.reviewChange()}
                >
                  Review this diff
                </Button>
                <Button
                  tone="primary"
                  disabled={!!s.busy || !canApplyChange(s)}
                  onClick={() => void w.applyChange()}
                >
                  Approve and apply
                </Button>
              </div>
              {change.reviewed_hash === change.hash && (
                <p role="status">This revision is reviewed. Apply requires confirmation.</p>
              )}
              <Disclosure title="Provider context">
                <p>
                  {change.context_manifest?.model} · {change.context_manifest?.provider_origin}
                </p>
                <BulletContent
                  title="Included files and guides"
                  items={change.context_manifest?.included?.map((file) => file.path)}
                />
              </Disclosure>
            </>
          ) : (
            <Empty
              title="No proposal yet"
              detail="Prepare a change to see its read-only diff and check evidence."
              icon="code"
            />
          )}
        </section>
      </div>
    </>
  );
}
