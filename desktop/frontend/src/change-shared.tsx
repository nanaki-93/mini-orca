import { useWorkspace } from './workspace-context';
import { useState } from 'react';
import { CommitMessage } from './git-controls';
import { activeChangeWorkflow, type State } from './workspace';
import type { ChangeMutation, ChangeSession } from './models';
import { Badge, Button, Disclosure, Empty, Notice, Panel } from './ui';

export function receiptNeedsAttention(receipt?: ChangeMutation | null): boolean {
  if (!receipt) return false;
  if (receipt.state !== 'applied' && receipt.state !== 'undone') return true;
  if (receipt.commit && (receipt.commit.state !== 'committed' || !!receipt.commit.error))
    return true;
  if ((receipt.warnings?.length || 0) > 0) return true;
  if (receipt.verification?.status === 'failed') return true;
  return false;
}

export function ChangeOutcome({ s }: { s: State }) {
  const w = useWorkspace();
  const change = s.change;
  const needsAttention = receiptNeedsAttention(s.changeReceipt);
  const showReceipt =
    !!s.changeReceipt && (needsAttention || change?.id === s.changeReceipt.session_id);
  return (
    <>
      {(s.uncertain || showReceipt) && (
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
          {showReceipt && s.changeReceipt && (
            <Panel
              className="chat-receipt"
              title={
                s.changeReceipt.state === 'applied'
                  ? 'Change applied'
                  : `Change ${s.changeReceipt.state.replaceAll('_', ' ')}`
              }
              actions={
                <div className="row" style={{ gap: '8px' }}>
                  <Badge value={s.changeReceipt.state} />
                  {!needsAttention && (
                    <Button tone="ghost small" onClick={() => w.set({ changeReceipt: undefined })}>
                      Dismiss
                    </Button>
                  )}
                </div>
              }
            >
              {s.changeReceipt.commit && (
                <Notice
                  tone={
                    s.changeReceipt.commit.state !== 'committed' || !!s.changeReceipt.commit.error
                      ? 'error'
                      : 'success'
                  }
                >
                  <strong>
                    {s.changeReceipt.commit.state === 'committed'
                      ? `Committed ${s.changeReceipt.commit.oid?.slice(0, 7)} on ${s.changeReceipt.commit.branch}`
                      : s.changeReceipt.commit.state === 'failed'
                        ? 'Files applied. Commit failed.'
                        : 'Commit outcome needs inspection.'}
                  </strong>
                  <p>{s.changeReceipt.commit.error || s.changeReceipt.commit.message}</p>
                  {s.changeReceipt.commit.state === 'failed' &&
                    s.changeReceipt.state === 'applied' && (
                      <CommitRecovery
                        key={s.changeReceipt.session_id + s.changeReceipt.hash}
                        s={s}
                      />
                    )}
                  {s.changeReceipt.commit.state === 'pending' && (
                    <p>Refresh Git status and inspect the branch history before retrying.</p>
                  )}
                </Notice>
              )}
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
                    {change?.changes.some((edit) => !/\.(md|txt)$/i.test(edit.path)) && (
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
                        <Notice
                          tone={
                            s.changeReceipt.verification.status === 'failed'
                              ? 'error'
                              : s.changeReceipt.verification.status === 'passed' ||
                                  s.changeReceipt.verification.status === 'verified'
                                ? 'success'
                                : s.changeReceipt.verification.status === 'stale' ||
                                    s.changeReceipt.verification.status === 'partial'
                                  ? 'warning'
                                  : 'info'
                          }
                        >
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

function CommitRecovery({ s }: { s: State }) {
  const w = useWorkspace();
  const [message, setMessage] = useState(s.changeReceipt!.commit!.message);
  return (
    <div className="stack">
      <CommitMessage message={message} onChange={setMessage} disabled={!!s.busy} />
      <Button
        loading={s.busy === 'Retry commit'}
        disabled={!!s.busy || s.uncertain || !message.trim()}
        onClick={() => void w.retryChangeCommit(message)}
      >
        Retry commit
      </Button>
    </div>
  );
}

export function ChangeHistory({ s }: { s: State }) {
  const w = useWorkspace();
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

export function isAddedLine(kind?: string): boolean {
  return kind === 'added' || kind === 'add';
}

export function isRemovedLine(kind?: string): boolean {
  return kind === 'removed' || kind === 'remove' || kind === 'delete' || kind === 'del';
}

const SYMBOL_DECL_RE =
  /^\s*(?:export\s+)?(?:default\s+)?(?:public\s+|private\s+|protected\s+|internal\s+)?(?:async\s+|static\s+|final\s+|abstract\s+)*(?:func\s+(?:\([^)]+\)\s*)?|function\s+|def\s+|fn\s+|fun\s+|class\s+|interface\s+|struct\s+|type\s+|const\s+)([A-Za-z_][A-Za-z0-9_]*)/;

export function detectInlineSymbols(
  lines: { kind: string; old_line: number; new_line: number; text: string }[],
  newStart?: number,
  newEnd?: number,
  indexedSymbols?: { name: string; kind: string; start_line: number; end_line: number }[],
): string[] {
  const found = new Set<string>();
  if (indexedSymbols?.length && newStart !== undefined && newEnd !== undefined) {
    for (const sym of indexedSymbols) {
      if (sym.start_line <= newEnd && sym.end_line >= newStart) {
        found.add(sym.name);
      }
    }
  }
  for (const line of lines) {
    const m = line.text.match(SYMBOL_DECL_RE);
    if (m?.[1]) found.add(m[1]);
  }
  return [...found].slice(0, 4);
}

export function summarizeDiffHunk(
  lines: { kind: string; old_line: number; new_line: number; text: string }[],
  newStart: number,
  newEnd: number,
  indexedSymbols?: { name: string; kind: string; start_line: number; end_line: number }[],
): { summary: string; symbols: string[]; adds: number; dels: number } {
  const adds = lines.filter((l) => isAddedLine(l.kind)).length;
  const dels = lines.filter((l) => isRemovedLine(l.kind)).length;
  const symbols = detectInlineSymbols(lines, newStart, newEnd, indexedSymbols);
  const changedText = lines
    .filter((l) => isAddedLine(l.kind) || isRemovedLine(l.kind))
    .map((l) => l.text.trim())
    .filter(Boolean)
    .join(' ');

  const intents: string[] = [];
  if (/\b(import|from|require|use|include)\b/.test(changedText))
    intents.push('updates module imports');
  if (/\b(if|else|switch|case|match|guard)\b/.test(changedText))
    intents.push('refines branching / guard condition');
  if (/\b(err|error|throw|catch|panic|fmt\.Errorf)\b/i.test(changedText))
    intents.push('strengthens error handling');
  if (/\b(test|assert|expect|t\.Fatal|t\.Error)\b/i.test(changedText))
    intents.push('updates test verification');
  if (/\b(return|yield)\b/.test(changedText) && intents.length < 2)
    intents.push('adjusts return behavior');

  const actionClause = intents.length
    ? intents.slice(0, 2).join(' & ')
    : adds > 0 && dels === 0
      ? 'adds new implementation lines'
      : dels > 0 && adds === 0
        ? 'removes obsolete lines'
        : 'updates implementation logic';

  const symbolPrefix = symbols.length ? `In ${symbols.join(', ')}: ` : '';
  return {
    summary: `${symbolPrefix}${actionClause} (+${adds} / -${dels} lines)`,
    symbols,
    adds,
    dels,
  };
}

export function summarizeFileEdit(
  edit: ChangeSession['changes'][number],
  indexedSymbols?: { name: string; kind: string; start_line: number; end_line: number }[],
): { summary: string; symbols: string[]; adds: number; dels: number } {
  const lines = edit.diff?.lines || [];
  const adds = lines.filter((l) => isAddedLine(l.kind)).length;
  const dels = lines.filter((l) => isRemovedLine(l.kind)).length;
  const changedLines = lines.filter((l) => isAddedLine(l.kind) || isRemovedLine(l.kind));
  const newLineNums = changedLines.map((l) => l.new_line).filter((n) => n > 0);
  const minLine = newLineNums.length ? Math.min(...newLineNums) : 1;
  const maxLine = newLineNums.length ? Math.max(...newLineNums) : 1;
  const symbols = detectInlineSymbols(lines, minLine, maxLine, indexedSymbols);

  if (edit.summary?.trim()) {
    return { summary: edit.summary.trim(), symbols, adds, dels };
  }
  const hunkSummary = summarizeDiffHunk(lines, minLine, maxLine, indexedSymbols);
  return {
    summary: hunkSummary.summary,
    symbols,
    adds,
    dels,
  };
}

export function ProposalDiff({
  edit,
  index,
  total,
  symbols,
}: {
  edit: ChangeSession['changes'][number];
  index: number;
  total: number;
  symbols?: { name: string; kind: string; start_line: number; end_line: number }[];
}) {
  const info = summarizeFileEdit(edit, symbols);
  return (
    <div className="diff" aria-label={`Read-only diff for ${edit.path}`} tabIndex={0}>
      <div className="code-header">
        <div className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
          <strong className="font-mono">{edit.path}</strong>
          {info.symbols.map((sym) => (
            <Badge key={sym} value={`fn ${sym}`} tone="violet" />
          ))}
        </div>
        <span className="font-mono">
          {index + 1} / {total}
        </span>
      </div>
      <div
        className="diff-inline-summary"
        aria-label={`Inline edit summary for ${edit.path}`}
        style={{
          padding: '6px 12px',
          borderBottom: '1px solid var(--line)',
          background: 'var(--bg-soft)',
          fontSize: '0.82rem',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '8px',
          flexWrap: 'wrap',
        }}
      >
        <span>
          <strong>Edit Summary:</strong> {info.summary}
        </span>
        <span className="small font-mono muted">
          +{info.adds} / -{info.dels} lines
        </span>
      </div>
      <pre tabIndex={0} aria-label={`Changes in ${edit.path}`}>
        {edit.diff.lines.map((line, i) => {
          const kind = isAddedLine(line.kind)
            ? 'added'
            : isRemovedLine(line.kind)
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
  );
}
