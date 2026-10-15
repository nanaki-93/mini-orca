import { useEffect, useState } from 'react';
import {
  activeChangeWorkflow,
  canAcceptChange,
  changeChecksPassed,
  isTestFilePath,
  workflowSeed,
  workflowSetupModels,
  workspace as w,
  type State,
} from './workspace';
import { ChangeOutcome, ProposalDiff, isAddedLine, isRemovedLine } from './change-shared';
import {
  Badge,
  BulletContent,
  Button,
  Disclosure,
  Empty,
  Go,
  Heading,
  Notice,
  Panel,
  Prose,
  Spinner,
  StatusDot,
  human,
} from './ui';
import type * as M from './models';

interface StageSpec {
  id: 'create' | 'test' | 'review' | 'human_review';
  step: string;
  title: string;
  role: string;
  modelKey?: keyof M.ChangeWorkflowModels;
}

const PIPELINE_STAGES: StageSpec[] = [
  {
    id: 'create',
    step: 'Step 1',
    title: 'Architect & Code Agent',
    role: 'Generates minimal diffs in target files.',
    modelKey: 'create',
  },
  {
    id: 'test',
    step: 'Step 2',
    title: 'Verification & Test Agent',
    role: 'Writes unit tests and runs compiler and test checks.',
    modelKey: 'test',
  },
  {
    id: 'review',
    step: 'Step 3',
    title: 'Clean-Code Critic Agent',
    role: 'Reviews diffs against project rules and acceptance criteria.',
    modelKey: 'review',
  },
  {
    id: 'human_review',
    step: 'Step 4',
    title: 'Human Precision Gate',
    role: 'Inspect the diff below and click Accept & Apply Fix.',
  },
];

export function AgentsWorkspace({ s }: { s: State }) {
  const change = s.change;
  const seed = s.changeSeed;
  const workflow = change?.workflow;
  const models = workflowSetupModels(s);
  const running = activeChangeWorkflow(change);
  const [missionTitle, setMissionTitle] = useState(seed?.title || '');
  const [missionPaths, setMissionPaths] = useState(
    seed?.paths.join(', ') || s.file?.path || s.index?.files[0]?.path || '',
  );
  const [missionPrompt, setMissionPrompt] = useState(seed?.message || '');
  const [selectedDiffPath, setSelectedDiffPath] = useState<string>('all');

  useEffect(() => {
    if (seed) {
      setMissionTitle(seed.title);
      setMissionPaths(seed.paths.join(', '));
      setMissionPrompt(seed.message);
    }
  }, [seed?.title, seed?.message]);

  useEffect(() => {
    setSelectedDiffPath('all');
  }, [change?.id, change?.revision]);

  useEffect(() => {
    if (s.project && !s.changeHistory) {
      void w.loadChangeHistory();
    }
  }, [s.project?.project_id]);

  const changedFiles = change?.changes || [];
  let addedLines = 0;
  let removedLines = 0;
  for (const fileChange of changedFiles) {
    for (const line of fileChange.diff?.lines || []) {
      if (isAddedLine(line.kind)) addedLines++;
      else if (isRemovedLine(line.kind)) removedLines++;
    }
  }
  const totalDelta = addedLines + removedLines;
  const scopeLabel =
    totalDelta === 0
      ? 'No diff yet'
      : totalDelta <= 60
        ? 'Surgical'
        : totalDelta <= 200
          ? 'Focused'
          : 'Broad';

  const checks = change?.checks || [];
  const passedChecks = checks.filter((c) => c.state === 'passed').length;
  const allChecksPassed = change ? changeChecksPassed(change) : false;
  const reviewVerdict = workflow?.review?.verdict;
  const readyToApply = canAcceptChange(s);

  const stageStatus = (id: StageSpec['id']): string => {
    const found = workflow?.stages.find((st) => st.name === id);
    if (found) return found.status;
    if (!change) return seed ? 'staged' : 'idle';
    if (id === 'create') return changedFiles.length > 0 ? 'completed' : 'pending';
    if (id === 'test')
      return checks.length > 0 ? (allChecksPassed ? 'completed' : 'failed') : 'pending';
    if (id === 'review') return workflow?.review ? 'completed' : 'pending';
    if (id === 'human_review')
      return change.state === 'applied'
        ? 'completed'
        : readyToApply
          ? 'awaiting_human_review'
          : 'pending';
    return 'pending';
  };

  const dispatchCustomMission = () => {
    const rawPaths = missionPaths
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean)
      .slice(0, 8);
    if (!rawPaths.length || !missionPrompt.trim()) {
      w.fail('Specify at least one target file path and a fix objective.');
      return;
    }
    const prepared = workflowSeed({
      title: missionTitle.trim() || 'Agent fix mission',
      kind: 'fix',
      paths: rawPaths,
      message: missionPrompt.trim(),
      acceptance_criteria: [
        'Keep the diff minimal and limited to the requested behavior.',
        'Ensure all unit tests and static checks pass.',
      ],
    });
    w.seedChange(prepared, 'agents');
    void (async () => {
      try {
        const context = await w.readFixPreparation();
        const useAgents = prepared.paths.some(isTestFilePath);
        if (useAgents) {
          await w.startChangeWorkflow(prepared, models, context, 'agents');
        } else {
          await w.prepareChange(prepared, true, models.create, context, 'agents');
        }
      } catch (error) {
        w.fail(error);
      }
    })();
  };

  const visibleDiffFiles =
    selectedDiffPath === 'all'
      ? changedFiles
      : changedFiles.filter((f) => f.path === selectedDiffPath);

  const missionFormContent = (
    <div className="stack" style={{ gap: '10px' }}>
      <div className="grid" style={{ gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
        <label className="block">
          <span className="small muted">Fix Title</span>
          <input
            aria-label="Agent mission title"
            className="field"
            value={missionTitle}
            onChange={(e) => setMissionTitle(e.target.value)}
            placeholder="e.g., Enforce bounded retry in worker queue"
          />
        </label>
        <label className="block">
          <span className="small muted">Target Files (comma-separated, max 8)</span>
          <input
            aria-label="Agent target files"
            className="field font-mono"
            value={missionPaths}
            onChange={(e) => setMissionPaths(e.target.value)}
            placeholder="internal/worker/worker.go, internal/worker/worker_test.go"
          />
        </label>
      </div>
      <label className="block">
        <span className="small muted">Fix Instructions &amp; Constraints</span>
        <textarea
          aria-label="Agent mission specification"
          rows={3}
          value={missionPrompt}
          onChange={(e) => setMissionPrompt(e.target.value)}
          placeholder="Describe the exact behavior to fix, invariants to preserve, and edge cases to test…"
        />
      </label>
      <div className="row between wrap">
        <span className="small muted">
          Models: <code>{models.create}</code> (Create) → <code>{models.test}</code> (Test) →{' '}
          <code>{models.review}</code> (Critic).
        </span>
        <Button
          tone="primary"
          icon="sparkles"
          disabled={!!s.busy || running}
          onClick={dispatchCustomMission}
        >
          Dispatch Agent Pipeline
        </Button>
      </div>
    </div>
  );

  return (
    <div className="workspace-page agents-page">
      <Heading
        variant="intro"
        title="Agent Mission Control"
        detail="Step-by-step fix pipeline: generate diff, run tests, verify with critic, and apply."
      >
        <Go page="bugs" tone="ghost" icon="bug">
          Browse Issues
        </Go>
        <Go page="models" tone="ghost" icon="compass">
          Configure Models
        </Go>
        {s.changeReceipt?.undo_available && (
          <Button tone="ghost" icon="undo" disabled={!!s.busy} onClick={() => void w.undoChange()}>
            Undo Last Change
          </Button>
        )}
        {running && (
          <Button
            tone="danger"
            disabled={!!s.busy || workflow?.status === 'canceling'}
            onClick={() => void w.cancelChangeWorkflow()}
          >
            Cancel Agent Run
          </Button>
        )}
        {changedFiles.length > 0 && (
          <Button tone="ghost" icon="code" onClick={() => void w.navigate('editor')}>
            Open in Code &amp; Diff ({changedFiles.length})
          </Button>
        )}
        {change && change.state === 'draft' && changedFiles.length > 0 && (
          <Button
            tone="primary"
            icon="check"
            disabled={!!s.busy || running || !readyToApply}
            onClick={() => void w.applyProposalNow(change.title)}
          >
            Accept &amp; Apply Fix
          </Button>
        )}
      </Heading>

      <ChangeOutcome s={s} />

      {running && (
        <div
          className="analysis-live-banner"
          role="status"
          aria-label="Active agent pipeline"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: '16px',
            padding: '12px 18px',
            background: 'var(--panel)',
            border: '1px solid var(--accent)',
            borderLeft: '4px solid var(--accent)',
            borderRadius: '6px',
          }}
        >
          <div className="row wrap" style={{ gap: '10px', alignItems: 'center' }}>
            <Spinner />
            <strong>
              Agent pipeline running: {change?.title || seed?.title || 'Generating & verifying fix'}
            </strong>
            <Badge value={workflow?.status || 'running'} tone="violet" />
          </div>
          <Button
            tone="ghost small"
            disabled={!!s.busy || workflow?.status === 'canceling'}
            onClick={() => void w.cancelChangeWorkflow()}
          >
            Cancel
          </Button>
        </div>
      )}

      {/* 4-Step Fix Progress Bar */}
      <div
        className="kpi-pulse-grid box-grid-4 agent-pipeline-grid"
        role="list"
        aria-label="Agent pipeline stages"
      >
        {PIPELINE_STAGES.map((stage) => {
          const status = stageStatus(stage.id);
          const stageModel =
            workflow?.stages.find((st) => st.name === stage.id)?.model?.model ||
            (stage.modelKey ? models[stage.modelKey] : 'Human approval');
          return (
            <div
              key={stage.id}
              role="listitem"
              className="kpi-pulse-card card-kpi box-card"
              style={{
                borderColor:
                  status === 'running' || status === 'awaiting_human_review'
                    ? 'var(--accent)'
                    : undefined,
              }}
            >
              <div className="box-head">
                <span className="box-eyebrow">{stage.step}</span>
                <span className="row" style={{ gap: '6px', alignItems: 'center' }}>
                  <StatusDot value={status} label={stage.title} />
                  <Badge
                    value={human(status)}
                    tone={
                      status === 'completed'
                        ? 'green'
                        : status === 'running' || status === 'awaiting_human_review'
                          ? 'violet'
                          : undefined
                    }
                  />
                </span>
              </div>
              <div className="box-value" style={{ fontSize: '0.96rem', marginTop: '4px' }}>
                {stage.title}
              </div>
              <p className="small muted" style={{ margin: '4px 0' }}>
                {stage.role}
              </p>
              <div className="box-meta">
                <span className="font-mono small">{stageModel}</span>
              </div>
            </div>
          );
        })}
      </div>

      {/* Active Fix Step: Diff Review, Critic Gate & Direct Apply */}
      {change ? (
        <div className="stack" style={{ gap: '16px' }}>
          <Panel
            title={`Proposed Fix · ${change.title}`}
            actions={
              <div className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
                <span className="small font-mono">
                  <span style={{ color: 'var(--green)' }}>+{addedLines}</span> /{' '}
                  <span style={{ color: 'var(--red)' }}>-{removedLines}</span> lines ({scopeLabel})
                </span>
                <Badge
                  value={
                    change.state === 'applied'
                      ? 'Applied'
                      : readyToApply
                        ? 'Ready to Apply'
                        : human(workflow?.status || change.state)
                  }
                  tone={change.state === 'applied' || readyToApply ? 'green' : 'violet'}
                />
              </div>
            }
          >
            <div className="stack" style={{ gap: '12px' }}>
              {workflow?.reason && (
                <Notice error={workflow.status === 'failed'}>{workflow.reason}</Notice>
              )}

              {/* Primary Fix Actions Bar */}
              <div
                className="proposal-action-banner"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  flexWrap: 'wrap',
                  gap: '10px',
                }}
              >
                <div className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
                  <span className="small">
                    <strong>{changedFiles.length}</strong>{' '}
                    {changedFiles.length === 1 ? 'changed file' : 'changed files'} ·{' '}
                    <strong>
                      {passedChecks}/{checks.length}
                    </strong>{' '}
                    checks passed
                  </span>
                  {workflow?.review && (
                    <Badge
                      value={`Critic: ${workflow.review.verdict}`}
                      tone={workflow.review.verdict === 'approve' ? 'green' : 'violet'}
                    />
                  )}
                </div>
                <div className="row wrap" style={{ gap: '8px' }}>
                  {change.state === 'draft' && (
                    <>
                      <Button
                        tone="primary"
                        icon="check"
                        disabled={!!s.busy || running || !readyToApply}
                        onClick={() => void w.applyProposalNow(change.title)}
                      >
                        Accept &amp; Apply Fix
                      </Button>
                      {!readyToApply && !running && (
                        <Button
                          tone="ghost"
                          icon="sparkles"
                          disabled={!!s.busy}
                          onClick={() => void w.rerunChangeWorkflow('agents')}
                        >
                          Re-run Fix
                        </Button>
                      )}
                      <Button
                        tone="ghost"
                        icon="refresh"
                        disabled={!!s.busy || running}
                        onClick={() => void w.checkChange(true)}
                      >
                        Verify Checks
                      </Button>
                      {!allChecksPassed && checks.length > 0 && (
                        <Button
                          tone="ghost"
                          icon="sparkles"
                          disabled={!!s.busy || running}
                          onClick={() => void w.repairChange()}
                        >
                          Repair Checks
                        </Button>
                      )}
                    </>
                  )}
                  <Button tone="ghost" icon="code" onClick={() => void w.navigate('editor')}>
                    Edit Hunks in Code &amp; Diff
                  </Button>
                  {change.state === 'draft' && (
                    <Button
                      tone="ghost"
                      icon="close"
                      disabled={!!s.busy || running}
                      onClick={() => w.newChange()}
                    >
                      Discard
                    </Button>
                  )}
                </div>
              </div>

              {/* File Filter Tabs + Inline Proposed Diffs */}
              {changedFiles.length > 0 ? (
                <div className="stack" style={{ gap: '10px' }}>
                  {changedFiles.length > 1 && (
                    <div className="row wrap" style={{ gap: '6px' }} role="tablist">
                      <Button
                        tone={selectedDiffPath === 'all' ? 'primary small' : 'ghost small'}
                        onClick={() => setSelectedDiffPath('all')}
                      >
                        All files ({changedFiles.length})
                      </Button>
                      {changedFiles.map((fc) => {
                        const fAdds =
                          fc.diff?.lines?.filter((l) => isAddedLine(l.kind)).length || 0;
                        const fDels =
                          fc.diff?.lines?.filter((l) => isRemovedLine(l.kind)).length || 0;
                        return (
                          <Button
                            key={fc.path}
                            tone={selectedDiffPath === fc.path ? 'primary small' : 'ghost small'}
                            icon="file"
                            onClick={() => setSelectedDiffPath(fc.path)}
                          >
                            <span className="font-mono">{fc.path.split('/').at(-1)}</span>{' '}
                            <span className="font-mono small">
                              +{fAdds}/-{fDels}
                            </span>
                          </Button>
                        );
                      })}
                    </div>
                  )}
                  <div className="stack" style={{ gap: '12px' }}>
                    {visibleDiffFiles.map((edit, idx) => (
                      <ProposalDiff
                        key={edit.path}
                        edit={edit}
                        index={idx}
                        total={visibleDiffFiles.length}
                      />
                    ))}
                  </div>
                </div>
              ) : (
                <Empty
                  title={running ? 'Generating proposed diff…' : 'No diff generated yet'}
                  detail={
                    running
                      ? 'The agent pipeline is currently creating and testing the change.'
                      : 'Click Re-run Fix to generate a fresh diff for the target files.'
                  }
                  icon="code"
                >
                  {!running && (
                    <Button
                      tone="primary"
                      icon="sparkles"
                      disabled={!!s.busy}
                      onClick={() => void w.rerunChangeWorkflow('agents')}
                    >
                      Generate Fix Now
                    </Button>
                  )}
                </Empty>
              )}
            </div>
          </Panel>

          {/* Clean-Software Precision Gate: Critic Review & Checks */}
          <Panel
            title="Clean-Software Precision Gate"
            actions={
              <Badge
                value={
                  allChecksPassed
                    ? reviewVerdict === 'approve'
                      ? 'Verified & Approved'
                      : 'Checks Passed'
                    : checks.length
                      ? `${passedChecks}/${checks.length} Checks`
                      : 'Awaiting Gate'
                }
                tone={allChecksPassed ? 'green' : 'violet'}
              />
            }
          >
            <div className="stack" style={{ gap: '12px' }}>
              {workflow?.review ? (
                <div className="stack" style={{ gap: '6px' }}>
                  <div className="row between wrap">
                    <strong className="small uppercase muted">Critic Agent Verdict</strong>
                    <Badge
                      value={workflow.review.verdict}
                      tone={workflow.review.verdict === 'approve' ? 'green' : 'violet'}
                    />
                  </div>
                  <Prose text={workflow.review.summary} />
                  {(workflow.review.findings || []).length > 0 && (
                    <BulletContent
                      title="Critic Observations"
                      items={workflow.review.findings || []}
                    />
                  )}
                </div>
              ) : (
                <p className="small muted">
                  Critic Agent review will appear here once Step 3 completes.
                </p>
              )}

              <div>
                <strong className="small uppercase muted">
                  Deterministic Checks ({passedChecks}/{checks.length})
                </strong>
                {checks.length === 0 ? (
                  <p className="small muted" style={{ marginTop: '4px' }}>
                    No compiler or test checks recorded yet.
                  </p>
                ) : (
                  <div className="stack" style={{ gap: '6px', marginTop: '6px' }}>
                    {checks.map((check) => (
                      <Disclosure
                        key={check.name}
                        title={
                          <span className="row between wrap" style={{ width: '100%' }}>
                            <span>
                              <strong>{human(check.name)}</strong>{' '}
                              {check.command?.length ? (
                                <code className="small muted">{check.command.join(' ')}</code>
                              ) : null}
                            </span>
                            <Badge
                              value={check.state}
                              tone={check.state === 'passed' ? 'green' : undefined}
                            />
                          </span>
                        }
                      >
                        <pre className="small font-mono" style={{ overflowX: 'auto' }}>
                          {check.output || `Exit code ${check.exit_code}`}
                        </pre>
                      </Disclosure>
                    ))}
                  </div>
                )}
              </div>

              {(change.acceptance_criteria || []).length > 0 && (
                <Disclosure title={`Acceptance Criteria (${change.acceptance_criteria.length})`}>
                  <ul className="compact-list" style={{ marginTop: '6px' }}>
                    {change.acceptance_criteria.map((criterion, idx) => (
                      <li key={idx}>{criterion}</li>
                    ))}
                  </ul>
                </Disclosure>
              )}
            </div>
          </Panel>
        </div>
      ) : seed ? (
        <Panel
          title={`Staged Fix · ${seed.title}`}
          actions={<Badge value="Ready to Run" tone="violet" />}
        >
          <div className="stack">
            <p className="small muted font-mono">Target files: {seed.paths.join(', ')}</p>
            <Prose text={seed.message} />
            <div className="actions">
              <Button
                tone="primary"
                icon="sparkles"
                disabled={!!s.busy}
                onClick={dispatchCustomMission}
              >
                Start Fix Pipeline
              </Button>
              <Button tone="ghost" icon="code" onClick={() => void w.navigate('editor')}>
                Open in Code &amp; Diff
              </Button>
            </div>
          </div>
        </Panel>
      ) : null}

      {/* Custom Fix Mission Form: collapsed when an active fix is already open */}
      {change ? (
        <Disclosure title="Start a New Custom Fix Mission">{missionFormContent}</Disclosure>
      ) : (
        <Panel
          title="Start a Fix Mission"
          actions={
            <span className="small muted">Auto-includes unit test target when eligible</span>
          }
        >
          {missionFormContent}
        </Panel>
      )}

      {/* Parallel Worktree Agents & Mission History */}
      <Disclosure
        title={`Worktree Agent Fleet (${s.implementations.length}) & Recent Fixes (${s.changeHistory?.length || 0})`}
      >
        <div className="stack" style={{ gap: '12px', paddingTop: '8px' }}>
          {s.implementations.length > 0 && (
            <Panel title="Isolated Worktree Agents">
              <div className="stack" style={{ gap: '6px' }}>
                {s.implementations.map((child) => (
                  <div className="list-row" key={child.worktreePath}>
                    <div className="list-copy">
                      <strong>{child.state.change?.title || child.state.project?.name}</strong>
                      <small className="path font-mono">{child.worktreePath}</small>
                    </div>
                    <Badge
                      value={
                        child.state.error
                          ? 'failed'
                          : child.state.busy ||
                            child.state.change?.workflow?.status ||
                            child.state.change?.state ||
                            'saved'
                      }
                    />
                    <Button
                      tone="ghost small"
                      onClick={() => void w.openWorktree(child.worktreePath!)}
                    >
                      Inspect Worktree Agent
                    </Button>
                  </div>
                ))}
              </div>
            </Panel>
          )}
          <Panel title="Recent Fixes">
            {!s.changeHistory?.length ? (
              <Empty
                title="No saved fixes yet"
                detail="Start a fix from Issues & Ideas or launch a mission above."
              />
            ) : (
              <div className="stack" style={{ gap: '6px' }}>
                {s.changeHistory.slice(0, 8).map((entry) => (
                  <div className="list-row" key={entry.id}>
                    <div className="list-copy">
                      <strong>{entry.title}</strong>
                      <small className="muted">
                        Revision {entry.revision} · {human(entry.kind)}
                      </small>
                    </div>
                    <div className="row" style={{ gap: '8px' }}>
                      <Badge value={entry.workflow_status || entry.state} />
                      <Button
                        tone="ghost small"
                        disabled={!!s.busy || running}
                        onClick={() => void w.resumeChange(entry.id)}
                      >
                        Open Fix
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </Panel>
        </div>
      </Disclosure>
    </div>
  );
}
