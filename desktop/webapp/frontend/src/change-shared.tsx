import { workspace as w, activeChangeWorkflow, changeChecksPassed, type State } from './workspace';
import type { ChangeSession } from './models';
import { Badge, Button, Disclosure, Empty, Notice, Panel } from './ui';

export function ChangeOutcome({ s }: { s: State }) {
  const change = s.change;
  return (
    <>
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
    </>
  );
}

export function ChangeHistory({ s }: { s: State }) {
  const running = activeChangeWorkflow(s.change);
  return (
    <Panel className="chat-history" title="Saved conversations">
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
    </Panel>
  );
}

export function ChangeChecks({ s, tests = true }: { s: State; tests?: boolean }) {
  const change = s.change;
  if (!change) return null;
  const blocked =
    !!s.busy ||
    activeChangeWorkflow(change) ||
    change.state !== 'draft' ||
    change.freshness !== 'current';
  return (
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
      )}
    </Panel>
  );
}

export function ProposalDiff({
  edit,
  index,
  total,
}: {
  edit: ChangeSession['changes'][number];
  index: number;
  total: number;
}) {
  return (
    <div className="diff" aria-label={`Read-only diff for ${edit.path}`} tabIndex={0}>
      <div className="code-header">
        <strong>{edit.path}</strong>
        <span>
          Read-only · {index + 1} of {total}
        </span>
      </div>
      <pre tabIndex={0} aria-label={`Changes in ${edit.path}`}>
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
  );
}
