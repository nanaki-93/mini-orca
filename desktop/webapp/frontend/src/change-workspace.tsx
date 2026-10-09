import { useEffect, useState } from 'react';
import {
  workspace as w,
  canAcceptChange,
  changeChecksPassed,
  activeChangeWorkflow,
  type State,
} from './workspace';
import { WorkflowModels, WorkflowProgress, defaultWorkflowModels } from './change-workflow';
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
  KeyValues,
} from './ui';
import { FixPreparation } from './fix-preparation';
import type { ChangeSession } from './models';

export function ChangeWorkspace({ s }: { s: State }) {
  const change = s.change;
  const [title, setTitle] = useState('New feature');
  const [paths, setPaths] = useState('');
  const [message, setMessage] = useState('');
  const [tests, setTests] = useState(true);
  const taskKind = change?.kind || s.changeSeed?.kind || 'feature';
  const guided = ['fix', 'performance', 'security'].includes(taskKind);
  const finding = s.changeSeed?.finding;
  const request = guided
    ? s.changeSeed?.message ||
      change?.messages.find((entry) => entry.role === 'user')?.content ||
      message
    : message;
  const taskLabel =
    taskKind === 'fix'
      ? 'Bug fix'
      : taskKind === 'performance'
        ? 'Performance fix'
        : 'Security fix';
  const explanations = change?.messages.filter((entry) => entry.role === 'assistant') || [];
  useEffect(() => {
    setTitle(change?.title || s.changeSeed?.title || 'New feature');
    setPaths(
      (change?.targets.map((target) => target.path) || s.changeSeed?.paths || []).join('\n'),
    );
    setMessage(change ? '' : s.changeSeed?.message || '');
    setTests(change?.kind !== 'instructions');
  }, [change?.id, s.changeSeed]);
  const seed = {
    title,
    paths: paths
      .split('\n')
      .map((path) => path.trim())
      .filter(Boolean),
    message: request,
    kind: taskKind,
    acceptance_criteria: change?.acceptance_criteria || s.changeSeed?.acceptance_criteria || [],
  };
  const running = activeChangeWorkflow(change);
  const models = s.workflowModels || change?.workflow?.models || defaultWorkflowModels;
  const useAgents =
    taskKind !== 'instructions' &&
    (!!change?.workflow || seed.paths.some((path) => path.endsWith('_test.go')));
  const blocked =
    !!s.busy ||
    running ||
    (!!change && (change.state !== 'draft' || change.freshness !== 'current'));
  const sourcePath = finding?.path || change?.targets[0]?.path || seed.paths[0];
  return (
    <div className={`chat-page${guided ? ' guided-fix' : ''}`}>
      <Heading
        title={guided ? taskLabel : 'Chat'}
        detail={
          guided
            ? 'Understand the finding, run the guided fix, then review each file before applying.'
            : 'Describe a change. Review the generated diff and accept when it is ready.'
        }
        variant="intro"
      >
        {guided && (
          <Go
            page={
              taskKind === 'fix' ? 'bugs' : taskKind === 'performance' ? 'performance' : 'security'
            }
            icon="back"
          >
            All findings
          </Go>
        )}
        {guided ? (
          <Button
            icon="code"
            disabled={!!s.busy || !sourcePath}
            onClick={() => void w.openFile(sourcePath, finding?.symbol || '')}
          >
            Go to file
          </Button>
        ) : (
          <Go page="editor" icon="code">
            Go to file
          </Go>
        )}
        <Button disabled={!!s.busy || running} onClick={() => w.newChange()}>
          New conversation
        </Button>
      </Heading>
      {(guided || useAgents) && (
        <Panel title="Agent models" className="fix-models">
          <WorkflowModels s={s} value={models} disabled={blocked} creationOnly={!useAgents} />
        </Panel>
      )}
      {(s.uncertain || s.changeReceipt) && (
        <section className="workspace-page chat-outcome" aria-label="Change outcome">
          {s.uncertain && (
            <Notice error>
              <div className="chat-recovery">
                <strong>Write outcome unknown.</strong>
                <p>Refresh project facts.</p>
                <div className="actions">
                  <Button onClick={() => void w.reindex()} disabled={!!s.busy}>
                    Refresh project
                  </Button>
                </div>
              </div>
            </Notice>
          )}
          {s.changeReceipt && (
            <Panel
              className="chat-receipt"
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
                  <div className="actions">
                    <Button
                      disabled={
                        !!s.busy || s.uncertain || change?.id !== s.changeReceipt.session_id
                      }
                      onClick={() => void w.verifyChange()}
                    >
                      Verify applied change
                    </Button>
                    {change?.changes.some((edit) => edit.path.endsWith('.go')) && (
                      <Button
                        disabled={
                          !!s.busy || s.uncertain || change.id !== s.changeReceipt.session_id
                        }
                        onClick={() => void w.reanalyzeChange()}
                      >
                        Reanalyze changed files
                      </Button>
                    )}
                  </div>
                  {s.changeReceipt.verification ? (
                    <div className="chat-verification" aria-label="Post-Apply verification">
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
                              <span>{check.name}</span>
                              <Badge value={check.state} />
                            </span>
                          }
                        >
                          <pre>{check.output || 'No diagnostics.'}</pre>
                        </Disclosure>
                      ))}
                    </div>
                  ) : (
                    <p className="small muted">No post-Apply verification available.</p>
                  )}
                </>
              )}
              {(s.changeReceipt.warnings || []).map((warning, i) => (
                <Notice error key={i}>
                  {warning}
                </Notice>
              ))}
              {s.changeReceipt.undo_available && (
                <div className="actions">
                  <Button disabled={!!s.busy || s.uncertain} onClick={() => void w.undoChange()}>
                    Undo proposal
                  </Button>
                </div>
              )}
            </Panel>
          )}
        </section>
      )}
      <div className="change-workspace">
        <section className="workspace-page chat-conversation" aria-label="Change conversation">
          {guided ? (
            <>
              <Panel
                title={change?.title || title}
                actions={
                  <Badge
                    value={change?.freshness === 'stale' ? 'stale' : change?.state || 'Guided fix'}
                  />
                }
              >
                <KeyValues
                  values={[
                    ['Task type', taskLabel],
                    [
                      'Location',
                      finding
                        ? `${finding.path}${finding.line ? `:${finding.line}` : ''}`
                        : change?.targets[0]?.path || seed.paths[0],
                    ],
                    ['Declaration', finding?.symbol || 'File-level finding'],
                    [
                      'Scope',
                      `${seed.paths.length} app-selected ${seed.paths.length === 1 ? 'file' : 'files'}`,
                    ],
                    ['Checks', 'Run project tests before acceptance'],
                  ]}
                />
                <BulletContent title="Acceptance criteria" items={seed.acceptance_criteria} />
                {change?.freshness === 'stale' && (
                  <Notice>
                    Source or guidance changed. Refresh the findings and start a new fix.
                  </Notice>
                )}
              </Panel>
              <Panel title="Cause" className="fix-explanation">
                <p className="small muted">
                  {finding
                    ? `Reported finding · ${finding.confidence.replaceAll('_', ' ')}`
                    : 'Saved finding context'}
                </p>
                <Prose
                  text={
                    finding?.cause ||
                    request ||
                    'The saved task has no cause recorded. Review the source and original finding before applying.'
                  }
                />
              </Panel>
              <Panel title="Proposed solution" className="fix-explanation">
                {explanations.length ? (
                  explanations.map((entry, index) => <Prose key={index} text={entry.content} />)
                ) : (
                  <Prose
                    text={
                      finding?.solution ||
                      'Review the generated explanation and file differences below. The proposed correction is subject to tests and your review.'
                    }
                  />
                )}
              </Panel>
            </>
          ) : !change ? (
            <Panel title="Task and file scope">
              <label className="block">
                Task type
                <input className="field" aria-label="Task type" value={taskKind} readOnly />
              </label>
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
          {!guided &&
            change?.messages.map((entry, i) => (
              <Panel
                key={i}
                title={entry.role === 'user' ? 'You' : 'Assistant'}
                className="chat-message"
              >
                <Prose text={entry.content} />
              </Panel>
            ))}
          <Panel title={guided ? 'Guided fix plan' : 'Describe the change'}>
            {guided ? (
              <Disclosure title="App-generated request">
                <textarea
                  aria-label="Change request"
                  className="composer"
                  value={request}
                  readOnly
                />
              </Disclosure>
            ) : (
              <>
                <label className="sr-only" htmlFor="chat-request">
                  Change request
                </label>
                <textarea
                  id="chat-request"
                  className="composer"
                  value={request}
                  onChange={(e) => setMessage(e.target.value)}
                  disabled={blocked}
                  placeholder="Describe a new feature, fix, or improvement…"
                />
              </>
            )}
            {!guided && !useAgents && (
              <label className="checkbox-line">
                <input
                  type="checkbox"
                  checked={tests}
                  disabled={blocked || change?.kind === 'instructions'}
                  onChange={(e) => setTests(e.target.checked)}
                />
                Run project tests after generation
              </label>
            )}
            <p className="small muted">
              {useAgents
                ? 'Creation, tests and agent review run automatically.'
                : 'Generation and checks run automatically. Include a _test.go path to use separate testing and review agents.'}
            </p>
            {!guided && (
              <div className="actions section-gap">
                <Button
                  tone="primary"
                  icon="sparkles"
                  disabled={
                    blocked ||
                    !request.trim() ||
                    !paths.trim() ||
                    !title.trim() ||
                    (useAgents && !s.models)
                  }
                  onClick={() =>
                    void (useAgents
                      ? w.startChangeWorkflow(seed, models)
                      : w.prepareChange(seed, tests))
                  }
                >
                  Generate changes
                </Button>
              </div>
            )}
          </Panel>
          {guided && (
            <FixPreparation
              s={s}
              seed={seed}
              disabled={blocked || !request.trim() || !paths.trim() || !title.trim()}
              label={change?.changes.length ? 'Regenerate fix' : 'Prepare fix'}
            />
          )}
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
                        {entry.workflow_status || entry.state} · revision {entry.revision}
                      </small>
                    </span>
                    <Button
                      disabled={!!s.busy || running}
                      onClick={() =>
                        void (['running', 'canceling'].includes(entry.workflow_status || '')
                          ? w.viewChange(entry.id)
                          : w.resumeChange(entry.id))
                      }
                    >
                      {['running', 'canceling'].includes(entry.workflow_status || '')
                        ? 'View workflow'
                        : 'Resume'}
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
          <WorkflowProgress s={s} />
          {change?.changes.length ? (
            <>
              <Panel
                title="Proposal diff"
                actions={<Badge value={`${change.changes.length} files`} />}
              >
                <p className="small muted">
                  Revision {change.revision} · Review each file diff before accepting.
                </p>
                {change.kind === 'performance' && (
                  <Notice>Performance unmeasured; tests do not establish a speedup.</Notice>
                )}
              </Panel>
              <ChangeFiles change={change} paths={seed.paths} />
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
                {!change.workflow && (
                  <div className="actions section-gap">
                    <Button disabled={blocked} onClick={() => void w.checkChange(guided || tests)}>
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
                )}
              </Panel>
              <Panel title="Accept this change" className="change-acceptance">
                <p className="small muted">
                  Accept applies revision {change.revision} to the {change.changes.length} displayed{' '}
                  {change.changes.length === 1 ? 'file' : 'files'}. Review each diff above.
                </p>
                <Button
                  tone="primary"
                  icon="check"
                  disabled={!!s.busy || !canAcceptChange(s)}
                  onClick={() => void w.acceptChange()}
                >
                  Accept changes
                </Button>
                {!canAcceptChange(s) && (
                  <p className="small muted">
                    Current passing checks and an approved agent review, when used, are required.
                  </p>
                )}
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
          ) : guided ? (
            <ChangeFiles change={change} paths={seed.paths} />
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

function ChangeFiles({ change, paths }: { change?: ChangeSession; paths: string[] }) {
  const [selected, setSelected] = useState('');
  const files = change?.changes.length ? change.changes.map((edit) => edit.path) : paths;
  const path = files.includes(selected) ? selected : files[0] || '';
  const edit = change?.changes.find((file) => file.path === path);
  return (
    <Panel title="Files to change" className="change-files">
      <label className="block">
        Select a file to review
        <select
          className="field"
          aria-label="Files to change"
          value={path}
          onChange={(event) => setSelected(event.target.value)}
        >
          {!files.length && <option value="">No captured files</option>}
          {files.map((file) => (
            <option key={file} value={file}>
              {file}
            </option>
          ))}
        </select>
      </label>
      {edit ? (
        <div className="diff" aria-label={`Read-only diff for ${edit.path}`} tabIndex={0}>
          <div className="code-header">
            <strong>{edit.path}</strong>
            <span>
              Read-only · {files.indexOf(path) + 1} of {files.length}
            </span>
          </div>
          <pre>
            {edit.diff.lines.map((line, i) => {
              const kind =
                line.kind === 'add' ? 'added' : line.kind === 'remove' ? 'removed' : line.kind;
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
      ) : (
        <Empty
          title={activeChangeWorkflow(change) ? 'Preparing the fix…' : 'No proposal yet'}
          detail="Run the fix to see the proposed changes for this file. Source stays unchanged until you accept."
          icon="code"
        />
      )}
    </Panel>
  );
}
