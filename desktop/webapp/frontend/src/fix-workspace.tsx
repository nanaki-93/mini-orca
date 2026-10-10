import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { ChangeSession, ChangeSeed } from './models';
import { workspace as w, activeChangeWorkflow, canAcceptChange, type State } from './workspace';
import { WorkflowProgress } from './change-workflow';
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
      {change?.state === 'draft' && !running && (
        <Disclosure title="Regenerate fix">
          <FixPreparation
            s={s}
            seed={seed}
            disabled={blocked || !seed.message.trim()}
            label="Regenerate fix"
          />
        </Disclosure>
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
        <Go page="models" icon="layers">
          Manage models
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
      {!change?.changes.length && (
        <>
          {' '}
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
        </>
      )}
      {change?.changes.length ? (
        <ProposalReview
          key={`${change.project_id}:${change.project_revision}:${change.id}:${change.revision}:${change.hash}`}
          s={s}
          change={change}
          details={
            <>
              {' '}
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
              {details}
            </>
          }
        />
      ) : (
        <>
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
  details,
  tests = true,
}: {
  s: State;
  change: ChangeSession;
  details: ReactNode;
  tests?: boolean;
}) {
  const [tab, setTab] = useState('changes');
  const [path, setPath] = useState(change.changes[0].path);
  const [viewed, setViewed] = useState<string[]>([path]);
  const [focusDiff, setFocusDiff] = useState(false);
  const diff = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!focusDiff) return;
    diff.current?.scrollIntoView({ block: 'center' });
    diff.current?.querySelector('pre')?.focus({ preventScroll: true });
    setFocusDiff(false);
  }, [focusDiff, path]);
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
      <FixTabs
        label="Fix review"
        selected={tab}
        onSelect={setTab}
        panel="fix-review-content"
        items={[
          { id: 'changes', label: `Changes (${files.length})` },
          { id: 'checks', label: `Checks${attention ? ` (${attention} need attention)` : ''}` },
          { id: 'details', label: 'Details' },
        ]}
      />
      <div
        id="fix-review-content"
        role="tabpanel"
        aria-labelledby={`fix-review-content-tab-${['changes', 'checks', 'details'].indexOf(tab)}`}
        className="fix-tab-content"
      >
        {tab === 'changes' && (
          <>
            <div className="row between wrap fix-evidence" aria-live="polite">
              <span className="row wrap">
                <Icon name={ready ? 'circleCheck' : 'clock'} />
                {ready
                  ? 'Required checks passed · ready for your review'
                  : running
                    ? 'Generation and checks in progress'
                    : 'Resolve checks and review before applying'}
              </span>
              <span className="small muted">Revision {change.revision} · Read-only</span>
            </div>
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
                <ProposalDiff edit={edit} index={files.indexOf(edit)} total={files.length} />
              </div>
              <aside className="review-evidence" aria-label="Review evidence">
                <section>
                  <h3>Checks & evidence</h3>
                  {change.checks.length ? (
                    change.checks.map((check, i) => (
                      <div className="review-evidence-row" key={i}>
                        <Icon name={check.state === 'passed' ? 'check' : 'warning'} />
                        <span>{check.name}</span>
                        <Badge value={check.state} />
                      </div>
                    ))
                  ) : (
                    <p className="small muted">No check evidence yet.</p>
                  )}
                  {change.workflow?.review && (
                    <div className="review-evidence-row">
                      <span>Model review</span>
                      <Badge value={change.workflow.review.verdict} />
                    </div>
                  )}
                  <Button tone="ghost" onClick={() => setTab('checks')}>
                    Checks & details
                  </Button>
                </section>
                <section>
                  <h3>Review progress</h3>
                  {files.map((file) => (
                    <button
                      className="review-file-progress"
                      key={file.path}
                      onClick={() => selectFile(file.path)}
                    >
                      <Icon name={viewed.includes(file.path) ? 'check' : 'file'} />
                      <span>{file.path.split('/').at(-1)}</span>
                      <small>{viewed.includes(file.path) ? 'Viewed' : 'Next'}</small>
                    </button>
                  ))}
                  <progress aria-label="Files viewed" value={viewed.length} max={files.length} />
                </section>
                <section>
                  <h3>Captured scope</h3>
                  <p className="small muted">
                    {change.targets.length} paths · Revision {change.revision}
                  </p>
                  <p className="small muted">
                    {change.freshness === 'current'
                      ? 'Current source snapshot'
                      : human(change.freshness)}
                  </p>
                  <Button tone="ghost" onClick={() => setTab('details')}>
                    Scope & instructions
                  </Button>
                </section>
              </aside>
            </div>
          </>
        )}
        {tab === 'checks' && (
          <div className="stack">
            <ChangeChecks s={s} tests={tests} />
            <WorkflowProgress s={s} actionLabel="Apply" showCancel={false} />
          </div>
        )}
        {tab === 'details' && details}
      </div>
      {change.state === 'draft' && (
        <div className="fix-action-bar">
          <div>
            <strong>
              {viewed.length} of {files.length} files viewed
            </strong>
            <p className="small muted">
              {next
                ? 'Review each file before applying.'
                : `Apply writes only these ${files.length} ${files.length === 1 ? 'file' : 'files'}.`}
            </p>
          </div>
          {running ? (
            <Button
              disabled={!!s.busy || change.workflow?.status === 'canceling'}
              onClick={() => void w.cancelChangeWorkflow()}
            >
              Cancel workflow
            </Button>
          ) : next && ready ? (
            <Button
              tone="primary"
              icon="arrow"
              disabled={!!s.busy}
              onClick={() => {
                setTab('changes');
                selectFile(next.path);
                setFocusDiff(true);
              }}
            >
              Review next file
            </Button>
          ) : (
            <Button
              tone="primary"
              icon="check"
              disabled={!!s.busy || !ready || !!next}
              onClick={(event) => {
                // The preceding click may have advanced the last file at this same position.
                if (event.detail > 1) return;
                void w.acceptChange();
              }}
            >
              Apply {files.length} {files.length === 1 ? 'file' : 'files'}
            </Button>
          )}
        </div>
      )}
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
