import { useWorkspace } from './workspace-context';
import { CommitMessage } from './git-controls';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { ChangeSession, ChangeSeed } from './models';
import { activeChangeWorkflow, canAcceptChange, workflowSeed, type State } from './workspace';
import { WorkflowProgress, defaultWorkflowModels } from './change-workflow';
import { ChangeChecks, ChangeOutcome, ProposalDiff } from './change-shared';
import { FixPreparation } from './fix-preparation';
import { FixSolution } from './fix-solution';
import { ProjectGuidance } from './project-guidance';
import { findingName } from './finding-name';
import {
  Badge,
  BulletContent,
  Button,
  Disclosure,
  Empty,
  Go,
  Icon,
  Notice,
  Panel,
  Prose,
  human,
} from './ui';

export function FixHeading({
  title,
  path,
  symbol,
  line,
  kind,
  metadata,
  children,
}: {
  title: string;
  path?: string;
  symbol?: string;
  line?: number;
  kind: ChangeSeed['kind'];
  metadata?: ReactNode;
  children?: ReactNode;
}) {
  const label =
    kind === 'performance' ? 'Performance fix' : kind === 'security' ? 'Security fix' : 'Bug fix';
  return (
    <header className="page-heading fix-heading">
      <div className="fix-identity">
        <span className="fix-kind-icon" role="img" aria-label={label} title={label}>
          <Icon name={kind === 'performance' ? 'gauge' : kind === 'security' ? 'shield' : 'bug'} />
        </span>
        <div>
          <h1>{title}</h1>
          <p className="results-detail-meta">
            {path && (
              <span className="path">
                {path}
                {line ? `:${line}` : ''}
              </span>
            )}
            <span className="row wrap">
              {symbol && <span>{symbol}</span>}
              {metadata}
            </span>
          </p>
        </div>
      </div>
      <div className="actions">{children}</div>
    </header>
  );
}

export function FixWorkspace({ s }: { s: State }) {
  const w = useWorkspace();
  const change = s.change;
  const finding = s.changeSeed?.finding;
  const kind = change?.kind || s.changeSeed?.kind || 'fix';
  const category = kind === 'fix' ? 'bugs' : kind === 'performance' ? 'performance' : 'security';
  const seed: ChangeSeed = {
    title: change?.title || s.changeSeed?.title || 'Fix',
    paths: change?.targets.map((target) => target.path) || s.changeSeed?.paths || [],
    message:
      s.changeSeed?.message ||
      change?.messages.find((entry) => entry.role === 'user')?.content ||
      '',
    kind,
    acceptance_criteria: change?.acceptance_criteria || s.changeSeed?.acceptance_criteria || [],
    finding,
  };
  const sourcePath = finding?.path || seed.paths[0];
  const running = activeChangeWorkflow(change);
  const blocked =
    !!s.busy ||
    running ||
    (!!change && (change.state !== 'draft' || change.freshness !== 'current'));

  const resetAndRetry = () => {
    const preparedSeed = workflowSeed(seed);
    w.seedChange(preparedSeed);
    void (async () => {
      try {
        const context = await w.readFixPreparation();
        const useAgents = preparedSeed.paths.some((path) => path.endsWith('_test.go'));
        const models = s.workflowModels || s.change?.workflow?.models || defaultWorkflowModels;
        if (useAgents) {
          await w.startChangeWorkflow(preparedSeed, models, context);
        } else {
          await w.prepareChange(preparedSeed, true, models.create, context);
        }
      } catch (error) {
        w.fail(error);
      }
    })();
  };

  const retryWithDiagnostics = () => {
    const failed = (change?.checks || []).filter((c) => c.state !== 'passed');
    const diag = failed.length
      ? `Repair failed checks using diagnostics:\n${failed
          .map((c) => `[${c.name}] exit code ${c.exit_code}:\n${c.output}`)
          .join('\n\n')}`
      : 'Repair failed checks and re-verify.';
    void w.repairChange(diag);
  };

  const earlierExplanations =
    change?.messages.filter((entry) => entry.role === 'assistant').slice(0, -1) || [];
  const details = (
    <div className="stack fix-details">
      <ProjectGuidance s={s} />
      <Panel title="Scope">
        <BulletContent title="Captured files" items={seed.paths} />
        <BulletContent title="Acceptance criteria" items={seed.acceptance_criteria} />
      </Panel>
      {earlierExplanations.length > 0 && (
        <Disclosure title="Earlier explanations">
          {earlierExplanations.map((entry, index) => (
            <Prose key={index} text={entry.content} />
          ))}
        </Disclosure>
      )}
      {change?.context_manifest && (
        <Panel className="chat-provider-context">
          <Disclosure title="Provider context">
            <p>
              {change.context_manifest.model} · {change.context_manifest.provider_origin}
            </p>
            <BulletContent
              title="Included files and guides"
              items={change.context_manifest.included?.map((file) => file.path)}
            />
          </Disclosure>
        </Panel>
      )}
      <Go page="history" icon="clock">
        Open history
      </Go>
      <div className="actions">
        <Button disabled={!!s.busy || running} onClick={() => w.newChange()}>
          New conversation
        </Button>
      </div>
    </div>
  );
  return (
    <div className="workspace-page guided-fix" data-accent={category}>
      <div className="row between wrap fix-back">
        <Go page={category} icon="back">
          All findings
        </Go>
      </div>
      <FixHeading
        title={findingName(seed.title, finding?.cause)}
        path={sourcePath}
        symbol={finding?.symbol}
        line={finding?.line}
        kind={kind}
        metadata={
          <Badge value={change?.freshness === 'stale' ? 'stale' : change?.state || 'draft'} />
        }
      >
        <Button
          icon="code"
          disabled={!!s.busy || !sourcePath}
          onClick={() => void w.openFile(sourcePath, finding?.symbol || '')}
        >
          Go to file
        </Button>
      </FixHeading>
      {change?.freshness === 'stale' && (
        <Notice>Source or guidance changed. Refresh the findings and start a new fix.</Notice>
      )}
      {kind === 'performance' && (
        <Notice>Performance unmeasured; tests do not establish a speedup.</Notice>
      )}
      <ChangeOutcome s={s} />
      {change?.changes.length ? (
        <ProposalReview
          key={`${change.project_id}:${change.project_revision}:${change.id}:${change.revision}:${change.hash}`}
          s={s}
          change={change}
          seed={seed}
          onResetAndRetry={resetAndRetry}
          onRetryWithDiagnostics={retryWithDiagnostics}
          details={details}
        />
      ) : (
        <>
          <Panel title="Cause" className="fix-explanation fix-cause">
            <Prose
              text={
                finding?.cause ||
                seed.message ||
                'No cause was saved. Review the original finding and source.'
              }
            />
            <p className="small muted">
              {finding
                ? `Reported finding · ${human(finding.confidence)}`
                : 'Saved finding context'}
            </p>
          </Panel>
          <FixSolution s={s} />
          {running ? (
            <WorkflowProgress s={s} showOutcome={false} />
          ) : (
            <FixPreparation
              s={s}
              seed={seed}
              disabled={blocked || !seed.message.trim() || !seed.paths.length}
            />
          )}
          <Disclosure title="Details">{details}</Disclosure>
          {running && (
            <Empty
              title="Preparing the fix…"
              detail="Source stays unchanged until you apply the reviewed proposal."
              icon="code"
            />
          )}
        </>
      )}
    </div>
  );
}

export function ProposalReview({
  s,
  change,
  seed,
  onResetAndRetry,
  onRetryWithDiagnostics,
  details,
  tests = true,
}: {
  s: State;
  change: ChangeSession;
  seed?: ChangeSeed;
  onResetAndRetry?: () => void;
  onRetryWithDiagnostics?: () => void;
  details?: ReactNode;
  tests?: boolean;
}) {
  const w = useWorkspace();
  const targetSeed: ChangeSeed = seed || {
    title: change.title,
    paths: change.targets.map((target) => target.path),
    kind: change.kind,
    message: change.messages.find((m) => m.role === 'user')?.content || '',
    acceptance_criteria: change.acceptance_criteria,
  };
  const [tab, setTab] = useState('changes');
  const [showDiff, setShowDiff] = useState(false);
  const [commitMessage, setCommitMessage] = useState(change.title);
  const [path, setPath] = useState(change.changes[0].path);
  const [viewed, setViewed] = useState<string[]>([path]);
  const [focusDiff, setFocusDiff] = useState(false);
  const diff = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!focusDiff || !showDiff) return;
    diff.current?.scrollIntoView({ block: 'center' });
    diff.current?.querySelector('pre')?.focus({ preventScroll: true });
    setFocusDiff(false);
  }, [focusDiff, showDiff, path]);
  const files = change.changes;
  const edit = files.find((file) => file.path === path) || files[0];
  const next = files.find((file) => !viewed.includes(file.path));
  const running = activeChangeWorkflow(change);
  const ready = canAcceptChange(s);
  const attention = change.checks.filter((check) => check.state !== 'passed').length;
  const selectFile = (path: string) => {
    setPath(path);
    setViewed((previous) => (previous.includes(path) ? previous : [...previous, path]));
  };
  return (
    <section className="fix-review chat-review" aria-label="Proposal review">
      {attention > 0 && (
        <div className="retry-banner">
          <div className="retry-info">
            <strong>
              <Icon name="alert" />{' '}
              {attention === 1 ? '1 check needs attention' : `${attention} checks need attention`}
            </strong>
            <small>
              Resolve failure using test diagnostics as input, or reset and retry from base.
            </small>
          </div>
          <div className="retry-actions">
            {onRetryWithDiagnostics && (
              <Button
                tone="warning small"
                icon="refresh"
                disabled={!!s.busy || running}
                title="Re-run fix using error diagnostics as input, without resetting workspace"
                onClick={() => void onRetryWithDiagnostics()}
              >
                Retry with Diagnostics
              </Button>
            )}
            {onResetAndRetry && (
              <Button
                tone="danger small"
                icon="trash"
                disabled={!!s.busy || running}
                title="Clean edit files and retry from base"
                onClick={() => void onResetAndRetry()}
              >
                Reset & Retry
              </Button>
            )}
          </div>
        </div>
      )}

      <div className="implementation-container">
        <aside className="finding-context-card">
          <Panel title="Target Finding" className="fix-explanation fix-cause">
            <div className="context-block">
              <h4
                className="small muted uppercase"
                style={{ marginBottom: '6px', fontSize: '0.72rem', letterSpacing: '0.05em' }}
              >
                Cause
              </h4>
              <Prose
                text={
                  targetSeed.finding?.cause ||
                  targetSeed.message ||
                  'No cause recorded. Review the original finding and source.'
                }
              />
              <p className="small muted" style={{ marginTop: '8px' }}>
                {targetSeed.finding
                  ? `Reported finding · ${human(targetSeed.finding.confidence)}`
                  : 'Finding context'}
              </p>
            </div>
            {targetSeed.acceptance_criteria.length > 0 && (
              <div
                className="context-block"
                style={{
                  marginTop: '14px',
                  paddingTop: '12px',
                  borderTop: '1px solid var(--line)',
                }}
              >
                <h4
                  className="small muted uppercase"
                  style={{ marginBottom: '6px', fontSize: '0.72rem', letterSpacing: '0.05em' }}
                >
                  Acceptance criteria
                </h4>
                <BulletContent title="Acceptance criteria" items={targetSeed.acceptance_criteria} />
              </div>
            )}
          </Panel>
          <FixSolution s={s} />
          {details && <Disclosure title="Details & Guidance">{details}</Disclosure>}
        </aside>

        <main className="implementation-main-card">
          <FixTabs
            label="Fix steps"
            selected={tab}
            onSelect={setTab}
            panel="fix-review-content"
            items={[
              { id: 'finding', label: '1. Finding' },
              { id: 'changes', label: `2. Changes (${files.length})` },
              { id: 'checks', label: `3. Checks${attention ? ` (${attention} pending)` : ''}` },
              { id: 'commit', label: '4. Commit' },
            ]}
          />
          <div
            id="fix-review-content"
            role="tabpanel"
            aria-labelledby={`fix-review-content-tab-${['finding', 'changes', 'checks', 'commit'].indexOf(tab)}`}
            className="fix-tab-content"
          >
            {tab === 'finding' && (
              <div className="stack fix-step-finding" style={{ padding: '18px 22px' }}>
                <Panel title="Cause" className="fix-explanation fix-cause">
                  <Prose
                    text={
                      targetSeed.finding?.cause ||
                      targetSeed.message ||
                      'No cause recorded. Review the original finding and source.'
                    }
                  />
                  <p className="small muted">
                    {targetSeed.finding
                      ? `Reported finding · ${human(targetSeed.finding.confidence)}`
                      : 'Finding context'}
                  </p>
                </Panel>
                <FixSolution s={s} />
                {targetSeed.acceptance_criteria.length > 0 && (
                  <Panel title="Acceptance criteria">
                    <BulletContent
                      title="Acceptance criteria"
                      items={targetSeed.acceptance_criteria}
                    />
                  </Panel>
                )}
                {details && <Disclosure title="More details">{details}</Disclosure>}
              </div>
            )}
            {tab === 'changes' && (
              <>
                <div className="row between wrap fix-evidence" aria-live="polite">
                  <span className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
                    <Icon name={ready ? 'circleCheck' : 'clock'} />
                    <strong>
                      {ready
                        ? 'Checks passed · ready for review'
                        : running
                          ? 'Checks in progress'
                          : 'Resolve checks before applying'}
                    </strong>
                  </span>
                  <div className="row" style={{ gap: '8px', alignItems: 'center' }}>
                    <Button
                      tone={showDiff ? 'ghost small' : 'primary small'}
                      icon={showDiff ? 'close' : 'code'}
                      onClick={() => setShowDiff((v) => !v)}
                    >
                      {showDiff ? 'Hide diff' : 'Check diff'}
                    </Button>
                  </div>
                </div>
                {!showDiff ? (
                  <div className="proposal-files-summary">
                    <table className="dense-table">
                      <thead>
                        <tr>
                          <th>File</th>
                          <th>Changes</th>
                          <th>Status</th>
                          <th></th>
                        </tr>
                      </thead>
                      <tbody>
                        {files.map((file) => {
                          const adds =
                            file.diff?.lines?.filter((l) => l.kind === 'add').length || 0;
                          const dels =
                            file.diff?.lines?.filter((l) => l.kind === 'delete').length || 0;
                          const isVisited = viewed.includes(file.path);
                          return (
                            <tr key={file.path}>
                              <td>
                                <span className="font-mono">{file.path}</span>
                              </td>
                              <td>
                                <span className="diff-stats font-mono">
                                  <span className="diff-stat add">+{adds}</span>{' '}
                                  <span className="diff-stat del">-{dels}</span>
                                </span>
                              </td>
                              <td>
                                {isVisited ? (
                                  <Badge value="Reviewed" tone="green" />
                                ) : (
                                  <Badge value="Pending" />
                                )}
                              </td>
                              <td style={{ textAlign: 'right' }}>
                                <Button
                                  tone="ghost small"
                                  onClick={() => {
                                    selectFile(file.path);
                                    setShowDiff(true);
                                    setFocusDiff(true);
                                  }}
                                >
                                  Diff
                                </Button>
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <>
                    <h2 className="sr-only">Proposal diff</h2>
                    <FixTabs
                      label="Files to change"
                      selected={edit.path}
                      onSelect={selectFile}
                      panel="fix-file-diff"
                      items={files.map((file) => ({
                        id: file.path,
                        label: file.path,
                        visited: viewed.includes(file.path),
                      }))}
                    />
                    <div className="studio-review-body">
                      <div
                        ref={diff}
                        id="fix-file-diff"
                        role="tabpanel"
                        aria-labelledby={`fix-file-diff-tab-${files.indexOf(edit)}`}
                        className="change-files"
                      >
                        <ProposalDiff
                          edit={edit}
                          index={files.indexOf(edit)}
                          total={files.length}
                        />
                      </div>
                    </div>
                  </>
                )}
              </>
            )}
            {tab === 'checks' && (
              <div className="stack fix-step-checks">
                <ChangeChecks s={s} tests={tests} />
                <WorkflowProgress s={s} actionLabel="Apply" showCancel={false} />
              </div>
            )}
            {tab === 'commit' && (
              <div className="stack fix-step-commit">
                <Panel title="Review & Commit">
                  <p className="small muted">
                    {viewed.length} of {files.length} files reviewed. Ready to commit to{' '}
                    {s.repository?.branch || 'the current branch'}.
                  </p>
                  {s.repository?.available && (
                    <CommitMessage
                      message={commitMessage}
                      onChange={setCommitMessage}
                      disabled={!!s.busy}
                    />
                  )}
                </Panel>
              </div>
            )}
          </div>
          {change.state === 'draft' && (
            <div className="fix-action-bar">
              <div className="row" style={{ gap: '10px', alignItems: 'center' }}>
                {onRetryWithDiagnostics && attention > 0 && (
                  <Button
                    tone="warning small"
                    icon="refresh"
                    disabled={!!s.busy || running}
                    title="Re-run fix using error diagnostics as input, without resetting workspace"
                    onClick={() => void onRetryWithDiagnostics()}
                  >
                    Retry with Diagnostics
                  </Button>
                )}
                {onResetAndRetry && (
                  <Button
                    tone="ghost small"
                    icon="trash"
                    disabled={!!s.busy || running}
                    title="Clean edit files and retry from base"
                    onClick={() => void onResetAndRetry()}
                  >
                    Reset & Retry
                  </Button>
                )}
                <span className="small muted font-mono">
                  {viewed.length}/{files.length} reviewed
                </span>
              </div>
              {running ? (
                <Button
                  disabled={!!s.busy || change.workflow?.status === 'canceling'}
                  onClick={() => void w.cancelChangeWorkflow()}
                >
                  Cancel workflow
                </Button>
              ) : tab === 'finding' ? (
                <Button tone="primary" icon="arrow" onClick={() => setTab('changes')}>
                  Review changes
                </Button>
              ) : tab === 'changes' ? (
                next && ready ? (
                  <Button
                    tone="primary"
                    icon="arrow"
                    disabled={!!s.busy}
                    onClick={() => {
                      selectFile(next.path);
                      setShowDiff(true);
                      setFocusDiff(true);
                    }}
                  >
                    Review next file
                  </Button>
                ) : (
                  <Button tone="primary" icon="arrow" onClick={() => setTab('checks')}>
                    Verify checks
                  </Button>
                )
              ) : tab === 'checks' ? (
                <Button tone="primary" icon="arrow" onClick={() => setTab('commit')}>
                  Proceed to commit
                </Button>
              ) : (
                <Button
                  tone="primary"
                  icon="check"
                  loading={s.busy === 'Apply change'}
                  disabled={
                    !!s.busy ||
                    !ready ||
                    !s.repository ||
                    !!s.resourceErrors.Git ||
                    (s.repository.available &&
                      (!s.repository.at_root ||
                        !s.repository.branch ||
                        !s.repository.head ||
                        !commitMessage.trim()))
                  }
                  onClick={(event) => {
                    if (event.detail > 1) return;
                    void w.acceptChange(commitMessage);
                  }}
                >
                  {s.repository?.available
                    ? 'Commit to branch'
                    : `Apply ${files.length} ${files.length === 1 ? 'file' : 'files'}`}
                </Button>
              )}
            </div>
          )}
        </main>
      </div>
    </section>
  );
}

function FixTabs({
  label,
  items,
  selected,
  onSelect,
  panel,
}: {
  label: string;
  items: { id: string; label: string; visited?: boolean }[];
  selected: string;
  onSelect: (id: string) => void;
  panel: string;
}) {
  return (
    <div className="fix-tabs" role="tablist" aria-label={label}>
      {items.map((item, index) => (
        <button
          key={item.id}
          id={`${panel}-tab-${index}`}
          role="tab"
          aria-label={item.label}
          title={item.label}
          aria-selected={item.id === selected}
          aria-controls={panel}
          tabIndex={item.id === selected ? 0 : -1}
          onClick={() => onSelect(item.id)}
          onKeyDown={(event) => {
            const target =
              event.key === 'ArrowRight'
                ? (index + 1) % items.length
                : event.key === 'ArrowLeft'
                  ? (index + items.length - 1) % items.length
                  : event.key === 'Home'
                    ? 0
                    : event.key === 'End'
                      ? items.length - 1
                      : -1;
            if (target < 0) return;
            event.preventDefault();
            onSelect(items[target].id);
            (event.currentTarget.parentElement?.children[target] as HTMLElement)?.focus();
          }}
        >
          <span>{item.label}</span>
          {item.visited && <Icon name="check" />}
        </button>
      ))}
    </div>
  );
}
