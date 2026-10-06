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
    <div className="chat-page">
      <Heading
        title="Chat"
        detail="Capture file scope, describe a change, then review the checked proposal before applying."
        variant="intro"
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
          Write outcome unknown. Refresh project facts.{' '}
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
          {s.changeReceipt.state === 'applied' && (
            <>
              <p className="small muted">Acceptance criteria still need review.</p>
              <div className="actions section-gap">
                <Button
                  disabled={!!s.busy || s.uncertain || change?.id !== s.changeReceipt.session_id}
                  onClick={() => void w.verifyChange()}
                >
                  Verify applied change
                </Button>
                {change?.changes.some((edit) => edit.path.endsWith('.go')) && (
                  <Button
                    disabled={!!s.busy || s.uncertain || change.id !== s.changeReceipt.session_id}
                    onClick={() => void w.reanalyzeChange()}
                  >
                    Reanalyze changed files
                  </Button>
                )}
              </div>
              {s.changeReceipt.verification && (
                <div className="section-gap" aria-label="Post-Apply verification">
                  <div className="row wrap">
                    <strong>Verification</strong>
                    <Badge value={s.changeReceipt.verification.status} />
                  </div>
                  {s.changeReceipt.verification.reason && (
                    <Notice error={s.changeReceipt.verification.status === 'failed'}>
                      {s.changeReceipt.verification.reason}
                    </Notice>
                  )}
                  {s.changeReceipt.verification.checks.map((check, i) => (
                    <Disclosure
                      key={i}
                      title={
                        <span className="row wrap">
                          {check.name}
                          <Badge value={check.state} />
                        </span>
                      }
                    >
                      <pre>{check.output || 'No diagnostics.'}</pre>
                    </Disclosure>
                  ))}
                </div>
              )}
            </>
          )}
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
        <section className="workspace-page chat-conversation" aria-label="Change conversation">
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
                  className="chat-path-input"
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
              <p className="small muted">Up to 8 Go/Markdown files, including new files.</p>
            </Panel>
          ) : (
            <Panel
              title={change.title}
              actions={<Badge value={change.freshness === 'stale' ? 'stale' : change.state} />}
            >
              <p className="small muted">
                Revision {change.revision} · {change.targets.length} captured paths
              </p>
              <div className="chat-file-scope">
                <BulletContent
                  title="File scope"
                  items={change.targets.map(
                    (target) => `${target.path}${target.exists ? '' : ' · new file'}`,
                  )}
                />
              </div>
              <BulletContent title="Acceptance criteria" items={change.acceptance_criteria} />
              {change.freshness === 'stale' && (
                <Notice>Source or guidance changed. Start a new conversation.</Notice>
              )}
            </Panel>
          )}
          {change?.messages.map((entry, i) => (
            <Panel
              key={i}
              title={entry.role === 'user' ? 'You' : 'Assistant'}
              className="chat-message"
            >
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
            <div className="actions">
              <Button
                tone="primary"
                disabled={blocked || !message.trim() || !paths.trim() || !title.trim()}
                onClick={submit}
              >
                Prepare change
              </Button>
            </div>
          </Panel>
          <Panel className="chat-history">
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
              <div className="actions section-gap">
                <Button disabled={!!s.busy} onClick={() => void w.loadChangeHistory()}>
                  Refresh history
                </Button>
              </div>
            </Disclosure>
          </Panel>
        </section>
        <section className="workspace-page chat-review" aria-label="Proposal review">
          {change?.changes.length ? (
            <>
              <Panel
                title="Proposal diff"
                actions={<Badge value={`${change.changes.length} files`} />}
              >
                <p className="small muted">
                  Revision {change.revision} · Read every file diff before reviewing this proposal.
                </p>
                {change.kind === 'performance' && (
                  <Notice>Performance unmeasured; tests do not establish a speedup.</Notice>
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
                    <div key={i} className="chat-check">
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
              <Panel title="Review and apply">
                <div className="actions">
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
                {change.reviewed_hash === change.hash && <p role="status">Revision reviewed.</p>}
              </Panel>
              <Panel className="chat-provider-context">
                <Disclosure title="Provider context">
                  <p>
                    {change.context_manifest?.model} · {change.context_manifest?.provider_origin}
                  </p>
                  <BulletContent
                    title="Included files and guides"
                    items={change.context_manifest?.included?.map((file) => file.path)}
                  />
                </Disclosure>
              </Panel>
            </>
          ) : (
            <Panel>
              <Empty title="No proposal yet" icon="code" />
            </Panel>
          )}
        </section>
      </div>
    </div>
  );
}
