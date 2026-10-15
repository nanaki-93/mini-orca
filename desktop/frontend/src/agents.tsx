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

type StageId = 'create' | 'test' | 'review' | 'human_review';

interface StageSpec {
  id: StageId;
  step: string;
  shortTitle: string;
  title: string;
  role: string;
  modelKey?: keyof M.ChangeWorkflowModels;
}

const PIPELINE_STAGES: StageSpec[] = [
  {
    id: 'create',
    step: 'Step 1',
    shortTitle: 'Create Fix',
    title: 'Architect & Code Agent',
    role: 'Generates minimal diffs in target files.',
    modelKey: 'create',
  },
  {
    id: 'test',
    step: 'Step 2',
    shortTitle: 'Verify & Test',
    title: 'Verification & Test Agent',
    role: 'Writes unit tests and runs compiler and test checks.',
    modelKey: 'test',
  },
  {
    id: 'review',
    step: 'Step 3',
    shortTitle: 'Critic Review',
    title: 'Clean-Code Critic Agent',
    role: 'Reviews diffs against project rules and acceptance criteria.',
    modelKey: 'review',
  },
  {
    id: 'human_review',
    step: 'Step 4',
    shortTitle: 'Review & Commit',
    title: 'Human Precision Gate',
    role: 'Inspect proposed diffs and commit the fix.',
  },
];

export interface AgentDashboardItem {
  key: string;
  path?: string;
  title: string;
  branch: string;
  stageLabel: string;
  status: string;
  tone?: 'green' | 'violet' | 'amber';
  mergeable?: boolean;
  commitOid?: string;
}

export function collectAgentDashboardItems(s: State): AgentDashboardItem[] {
  const items: AgentDashboardItem[] = [];
  const mainBranch = s.repository?.branch || 'main';
  if (s.change || s.changeSeed || !s.selectedAgentPath) {
    const c = s.change;
    const running = activeChangeWorkflow(c);
    const ready = canAcceptChange(s);
    const committed = c?.state === 'applied';
    const runningStage = c?.workflow?.stages.find((st) => st.status === 'running')?.name;
    const stageLabel = running
      ? `Running · ${runningStage ? human(runningStage) : 'Pipeline'}`
      : committed
        ? 'Committed on branch'
        : ready || (c?.changes.length || 0) > 0
          ? 'Step 4 · Review & Commit'
          : s.changeSeed
            ? 'Step 1 · Staged'
            : 'New Mission';
    const status = running
      ? 'running'
      : committed
        ? 'completed'
        : ready
          ? 'awaiting_human_review'
          : c?.workflow?.status || c?.state || 'idle';
    items.push({
      key: 'main',
      path: undefined,
      title: c?.title || s.changeSeed?.title || 'Main Workspace Mission',
      branch: mainBranch,
      stageLabel,
      status,
      tone: committed || ready ? 'green' : running ? 'violet' : undefined,
      commitOid: c?.commit?.oid || s.changeReceipt?.commit?.oid,
    });
  }

  const worktrees = s.gitWorkspace?.worktrees.filter((t) => !t.current) || [];
  for (const tree of worktrees) {
    const child = s.implementations.find((impl) => impl.worktreePath === tree.path);
    const c = child?.state.change;
    const seed = child?.state.changeSeed;
    const latest = tree.completed?.[0] || tree.sessions?.[0];
    const cleanBranchTitle =
      tree.branch
        .replace(/^(codex|fix|perf|security|feature)[/-]/, '')
        .replace(/[-_]/g, ' ')
        .trim() || tree.branch;
    const title =
      c?.title ||
      seed?.title ||
      latest?.title ||
      cleanBranchTitle[0]?.toUpperCase() + cleanBranchTitle.slice(1);
    const running =
      activeChangeWorkflow(c) ||
      latest?.workflow_status === 'running' ||
      latest?.workflow_status === 'canceling';
    const committed = c?.state === 'applied' || latest?.state === 'applied' || tree.mergeable;
    const ready = (child ? canAcceptChange(child.state) : false) || (c?.changes.length || 0) > 0;
    const runningStage = c?.workflow?.stages.find((st) => st.status === 'running')?.name;
    const stageLabel = tree.mergeable
      ? 'Step 4 · Committed (Ready to Merge)'
      : running
        ? `Running · ${runningStage ? human(runningStage) : 'Pipeline'}`
        : committed
          ? 'Step 4 · Committed'
          : ready
            ? 'Step 4 · Review & Commit'
            : latest?.workflow_status === 'failed'
              ? 'Failed · Needs Attention'
              : 'Step 1 · In Progress';
    const status = tree.mergeable
      ? 'completed'
      : running
        ? 'running'
        : ready
          ? 'awaiting_human_review'
          : c?.workflow?.status || latest?.workflow_status || latest?.state || 'pending';
    items.push({
      key: tree.path,
      path: tree.path,
      title,
      branch: tree.branch,
      stageLabel,
      status,
      tone: tree.mergeable || ready ? 'green' : running ? 'violet' : 'amber',
      mergeable: tree.mergeable,
      commitOid: c?.commit?.oid || child?.state.changeReceipt?.commit?.oid || latest?.commit?.oid,
    });
  }
  return items;
}

export function AgentsWorkspace({ s }: { s: State }) {
  const selectedChild = s.selectedAgentPath
    ? s.implementations.find((impl) => impl.worktreePath === s.selectedAgentPath)
    : undefined;
  const agentW = selectedChild || w;
  const agentState = selectedChild ? selectedChild.state : s;
  const isWorktreeAgent = !!selectedChild?.worktreePath;
  const activeWorktree = s.selectedAgentPath
    ? s.gitWorkspace?.worktrees.find((t) => t.path === s.selectedAgentPath)
    : undefined;

  const change = agentState.change;
  const seed = agentState.changeSeed;
  const workflow = change?.workflow;
  const models = workflowSetupModels(s);
  const running = activeChangeWorkflow(change);
  const busy = !!s.busy || !!agentState.busy;
  const [missionTitle, setMissionTitle] = useState(seed?.title || '');
  const [missionPaths, setMissionPaths] = useState(
    seed?.paths.join(', ') || s.file?.path || s.index?.files[0]?.path || '',
  );
  const [missionPrompt, setMissionPrompt] = useState(seed?.message || '');
  const [selectedDiffPath, setSelectedDiffPath] = useState<string>('all');

  useEffect(() => {
    if (s.selectedAgentPath && !selectedChild && !s.busy) {
      void w.selectAgent(s.selectedAgentPath);
    }
  }, [s.selectedAgentPath, selectedChild, s.busy]);

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
  const readyToApply = canAcceptChange(agentState);

  const stageStatus = (id: StageId): string => {
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
        : readyToApply || changedFiles.length > 0
          ? 'awaiting_human_review'
          : 'pending';
    return 'pending';
  };

  const computeCurrentStage = (): StageId => {
    if (!change) return 'create';
    if (running) {
      const runningStage = PIPELINE_STAGES.find((st) => stageStatus(st.id) === 'running');
      return runningStage?.id || 'create';
    }
    if (checks.length > 0 && !allChecksPassed) return 'test';
    if (workflow?.status === 'changes_requested') return 'review';
    if (changedFiles.length > 0 || change.state === 'applied' || readyToApply) {
      return 'human_review';
    }
    return 'create';
  };

  const autoStage = computeCurrentStage();
  const [selectedStage, setSelectedStage] = useState<StageId>(autoStage);

  useEffect(() => {
    setSelectedStage(autoStage);
  }, [
    s.selectedAgentPath,
    autoStage,
    change?.id,
    change?.revision,
    change?.state,
    workflow?.status,
    checks.length,
    allChecksPassed,
  ]);

  useEffect(() => {
    if (seed) {
      setMissionTitle(seed.title);
      setMissionPaths(seed.paths.join(', '));
      setMissionPrompt(seed.message);
    }
  }, [seed?.title, seed?.message]);

  useEffect(() => {
    setSelectedDiffPath('all');
  }, [s.selectedAgentPath, change?.id, change?.revision]);

  useEffect(() => {
    if (s.project && !s.changeHistory) {
      void w.loadChangeHistory();
    }
    if (s.project && !s.gitWorkspace && !s.resourceErrors['Git / Worktrees']) {
      void w.loadGitWorkspace();
    }
  }, [s.project?.project_id]);

  const isStageVisitable = (id: StageId): boolean => {
    if (id === 'create') return true;
    if (!change) return false;
    const status = stageStatus(id);
    if (
      [
        'running',
        'completed',
        'failed',
        'awaiting_human_review',
        'waiting',
        'changes_requested',
      ].includes(status)
    ) {
      return true;
    }
    if (id === 'test') return checks.length > 0 || changedFiles.length > 0;
    if (id === 'review') return !!workflow?.review || allChecksPassed;
    if (id === 'human_review') return changedFiles.length > 0 || change.state === 'applied';
    return false;
  };

  const activeStage: StageId = isStageVisitable(selectedStage) ? selectedStage : autoStage;

  const dispatchCustomMission = () => {
    const rawPaths = missionPaths
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean)
      .slice(0, 8);
    if (!rawPaths.length || !missionPrompt.trim()) {
      agentW.fail('Specify at least one target file path and a fix objective.');
      return;
    }
    const prepared = workflowSeed({
      finding_targets: seed?.finding_targets,
      finding: seed?.finding,
      title: missionTitle.trim() || seed?.title || 'Agent fix mission',
      kind: seed?.kind || 'fix',
      paths: rawPaths,
      message: missionPrompt.trim(),
      acceptance_criteria: seed?.acceptance_criteria?.length
        ? seed.acceptance_criteria
        : [
            'Keep the diff minimal and limited to the requested behavior.',
            'Ensure all unit tests and static checks pass.',
          ],
    });
    if (isWorktreeAgent) {
      agentW.seedChange(prepared, 'agents');
      void (async () => {
        try {
          const context = await agentW.readFixPreparation();
          const useAgents = prepared.paths.some(isTestFilePath);
          if (useAgents) {
            await agentW.startChangeWorkflow(prepared, models, context, 'agents');
          } else {
            await agentW.prepareChange(prepared, true, models.create, context, 'agents');
          }
          await w.loadGitWorkspace();
        } catch (error) {
          agentW.fail(error);
        }
      })();
      return;
    }
    void w.startAgentFix(prepared);
  };

  const startFreshAgentMission = () => {
    w.set({
      selectedAgentPath: undefined,
      change: undefined,
      changeSeed: undefined,
      changeReceipt: undefined,
      error: '',
    });
    setMissionTitle('');
    setMissionPrompt('');
    setSelectedStage('create');
  };

  const dashboardItems = collectAgentDashboardItems(s);
  const visibleDiffFiles =
    selectedDiffPath === 'all'
      ? changedFiles
      : changedFiles.filter((f) => f.path === selectedDiffPath);

  const userPrompt =
    change?.messages.filter((m) => m.role === 'user').at(-1)?.content ||
    seed?.message ||
    missionPrompt;

  const commitOid =
    change?.commit?.oid ||
    agentState.changeReceipt?.commit?.oid ||
    activeWorktree?.completed?.[0]?.commit?.oid;
  const commitBranch =
    change?.commit?.branch || activeWorktree?.branch || agentState.repository?.branch || 'main';
  const mainBranch = s.repository?.branch || 'main';

  return (
    <div className="workspace-page agents-page">
      <Heading
        variant="intro"
        title="Agent Mission Control"
        detail="Run multiple agents in parallel worktrees, click any agent to inspect its live workflow stage, then Accept & Commit and Merge."
      >
        <Go page="bugs" tone="ghost" icon="bug">
          Browse Issues
        </Go>
        <Go page="git" tone="ghost" icon="git">
          Git &amp; Worktrees
        </Go>
        <Go page="models" tone="ghost" icon="compass">
          Configure Models
        </Go>
        {agentState.changeReceipt?.undo_available && (
          <Button tone="ghost" icon="undo" disabled={busy} onClick={() => void agentW.undoChange()}>
            Undo Last Change
          </Button>
        )}
        <Button tone="ghost" icon="plus" disabled={!!s.busy} onClick={startFreshAgentMission}>
          New Fix
        </Button>
      </Heading>

      {agentState.error && <Notice error>{agentState.error}</Notice>}
      <ChangeOutcome s={agentState} />

      {/* Multi-Agent Dashboard Strip */}
      {(dashboardItems.length > 1 ||
        (s.gitWorkspace?.worktrees.filter((t) => !t.current).length || 0) > 0) && (
        <Panel
          title={`Running & Active Agents (${dashboardItems.length})`}
          actions={
            <span className="small muted">Click any agent to inspect its 4-stage workflow</span>
          }
        >
          <div
            className="row wrap"
            style={{ gap: '8px' }}
            role="tablist"
            aria-label="Active agents"
          >
            {dashboardItems.map((item) => {
              const isSelected = (s.selectedAgentPath || undefined) === item.path;
              return (
                <button
                  key={item.key}
                  type="button"
                  role="tab"
                  aria-selected={isSelected}
                  onClick={() => void w.selectAgent(item.path)}
                  className={`kpi-pulse-card card-kpi box-card ${isSelected ? 'active' : ''}`}
                  style={{
                    flex: '1 1 240px',
                    maxWidth: '360px',
                    textAlign: 'left',
                    cursor: 'pointer',
                    padding: '10px 12px',
                    borderColor: isSelected ? 'var(--accent)' : undefined,
                    boxShadow: isSelected ? '0 0 0 1px var(--accent)' : undefined,
                  }}
                >
                  <div className="row between wrap" style={{ gap: '6px' }}>
                    <span className="row" style={{ gap: '6px', alignItems: 'center' }}>
                      <StatusDot value={item.status} label={item.title} />
                      <strong className="small">{item.title}</strong>
                    </span>
                    <Badge value={item.stageLabel} tone={item.tone} />
                  </div>
                  <div
                    className="row between wrap small muted font-mono"
                    style={{ marginTop: '6px', gap: '6px' }}
                  >
                    <span>{item.branch}</span>
                    {item.commitOid && <span>#{item.commitOid.slice(0, 7)}</span>}
                  </div>
                </button>
              );
            })}
          </div>
        </Panel>
      )}

      {/* Top Clickable 4-Stage Workflow Bar */}
      <div
        className="kpi-pulse-grid box-grid-4 agent-pipeline-grid"
        role="tablist"
        aria-label="Agent pipeline stages"
      >
        {PIPELINE_STAGES.map((stage) => {
          const status = stageStatus(stage.id);
          const visitable = isStageVisitable(stage.id);
          const isSelected = activeStage === stage.id;
          const stageModel =
            workflow?.stages.find((st) => st.name === stage.id)?.model?.model ||
            (stage.modelKey ? models[stage.modelKey] : 'Human approval');
          return (
            <button
              key={stage.id}
              type="button"
              role="tab"
              aria-selected={isSelected}
              disabled={!visitable}
              onClick={() => visitable && setSelectedStage(stage.id)}
              className={`kpi-pulse-card card-kpi box-card ${isSelected ? 'active' : ''}`}
              style={{
                textAlign: 'left',
                cursor: visitable ? 'pointer' : 'not-allowed',
                opacity: visitable ? 1 : 0.55,
                borderColor: isSelected
                  ? 'var(--accent)'
                  : status === 'running' || status === 'awaiting_human_review'
                    ? 'var(--accent)'
                    : undefined,
                boxShadow: isSelected ? '0 0 0 1px var(--accent)' : undefined,
              }}
            >
              <div className="box-head">
                <span className="box-eyebrow">
                  {stage.step} · {stage.shortTitle}
                </span>
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
            </button>
          );
        })}
      </div>

      {/* Center Stage Content: Render ONLY the currently selected stage */}
      {activeStage === 'create' && (
        <Panel
          title={
            change
              ? `Step 1 · Architect & Code Agent (${change.title})`
              : seed
                ? `Step 1 · Staged Fix (${seed.title})`
                : 'Step 1 · Start a Fix Mission'
          }
          actions={
            <div className="row wrap" style={{ gap: '6px', alignItems: 'center' }}>
              {isWorktreeAgent && activeWorktree && (
                <Badge value={`Worktree: ${activeWorktree.branch}`} tone="violet" />
              )}
              <Badge
                value={human(stageStatus('create'))}
                tone={stageStatus('create') === 'completed' ? 'green' : 'violet'}
              />
            </div>
          }
        >
          {running ? (
            <div className="stack" style={{ gap: '12px' }}>
              <div className="row wrap" style={{ gap: '10px', alignItems: 'center' }}>
                <Spinner />
                <strong>
                  Agent pipeline is running ({change?.title || seed?.title || 'Generating fix'})…
                </strong>
              </div>
              {change && (
                <p className="small muted font-mono">
                  Target files: {change.targets.map((t) => t.path).join(', ')}
                </p>
              )}
              <div className="row">
                <Button
                  tone="danger"
                  disabled={busy || workflow?.status === 'canceling'}
                  onClick={() => void agentW.cancelChangeWorkflow()}
                >
                  Cancel Agent Run
                </Button>
              </div>
            </div>
          ) : change ? (
            <div className="stack" style={{ gap: '12px' }}>
              <div className="row between wrap" style={{ gap: '8px' }}>
                <span className="small muted font-mono">
                  Target files: {change.targets.map((t) => t.path).join(', ')}
                </span>
                <span className="small font-mono">
                  <span style={{ color: 'var(--green)' }}>+{addedLines}</span> /{' '}
                  <span style={{ color: 'var(--red)' }}>-{removedLines}</span> lines ({scopeLabel})
                </span>
              </div>
              {userPrompt && <Prose text={userPrompt} />}
              {(change.acceptance_criteria || []).length > 0 && (
                <BulletContent title="Acceptance Criteria" items={change.acceptance_criteria} />
              )}
              <div className="row between wrap" style={{ marginTop: '4px' }}>
                <Button
                  tone="ghost"
                  icon="sparkles"
                  disabled={busy}
                  onClick={() => void agentW.rerunChangeWorkflow('agents')}
                >
                  Re-run Fix
                </Button>
                {changedFiles.length > 0 ? (
                  <Button
                    tone="primary"
                    icon="arrow-right"
                    onClick={() => setSelectedStage('test')}
                  >
                    Next: Verify &amp; Test
                  </Button>
                ) : (
                  <Button
                    tone="primary"
                    icon="sparkles"
                    disabled={busy}
                    onClick={() => void agentW.rerunChangeWorkflow('agents')}
                  >
                    Generate Fix Now
                  </Button>
                )}
              </div>
            </div>
          ) : (
            <div className="stack" style={{ gap: '12px' }}>
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
                  Models: <code>{models.create}</code> (Create) → <code>{models.test}</code> (Test)
                  → <code>{models.review}</code> (Critic).
                </span>
                <Button
                  tone="primary"
                  icon="sparkles"
                  disabled={busy || running}
                  onClick={dispatchCustomMission}
                >
                  Dispatch Agent Pipeline
                </Button>
              </div>

              {(agentState.changeHistory?.length || 0) > 0 && (
                <Disclosure title={`Recent Fixes (${agentState.changeHistory?.length || 0})`}>
                  <div className="stack" style={{ gap: '6px', paddingTop: '6px' }}>
                    {agentState.changeHistory!.slice(0, 8).map((entry) => (
                      <div className="list-row" key={entry.id}>
                        <div className="list-copy">
                          <strong>{entry.title}</strong>
                          <small className="muted">
                            Revision {entry.revision} · {human(entry.kind)}
                            {entry.commit?.oid ? ` · #${entry.commit.oid.slice(0, 7)}` : ''}
                          </small>
                        </div>
                        <div className="row" style={{ gap: '8px' }}>
                          <Badge value={entry.workflow_status || entry.state} />
                          <Button
                            tone="ghost small"
                            disabled={busy || running}
                            onClick={() => void agentW.viewChange(entry.id, 'agents')}
                          >
                            Open Fix
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                </Disclosure>
              )}
            </div>
          )}
        </Panel>
      )}

      {activeStage === 'test' && change && (
        <Panel
          title={`Step 2 · Verification & Test Agent (${passedChecks}/${checks.length} Passed)`}
          actions={
            <Badge
              value={
                allChecksPassed
                  ? 'All Checks Passed'
                  : checks.length
                    ? 'Checks Need Attention'
                    : 'Pending'
              }
              tone={allChecksPassed ? 'green' : 'violet'}
            />
          }
        >
          <div className="stack" style={{ gap: '12px' }}>
            {checks.length === 0 ? (
              <Empty
                title="No verification checks run yet"
                detail="Run deterministic compiler and unit test checks on the proposed changes."
                icon="check"
              />
            ) : (
              <div className="stack" style={{ gap: '6px' }}>
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

            <div className="row between wrap">
              <Button
                tone="ghost"
                icon="refresh"
                disabled={busy || running}
                onClick={() => void agentW.checkChange(true)}
              >
                Re-run Checks
              </Button>
              {running ? (
                <Button
                  tone="danger"
                  disabled={busy || workflow?.status === 'canceling'}
                  onClick={() => void agentW.cancelChangeWorkflow()}
                >
                  Cancel Agent Run
                </Button>
              ) : !allChecksPassed && checks.length > 0 ? (
                <Button
                  tone="primary"
                  icon="sparkles"
                  disabled={busy}
                  onClick={() => void agentW.repairChange()}
                >
                  Repair Failed Checks
                </Button>
              ) : (
                <Button
                  tone="primary"
                  icon="arrow-right"
                  onClick={() => setSelectedStage('review')}
                >
                  Next: Critic Review
                </Button>
              )}
            </div>
          </div>
        </Panel>
      )}

      {activeStage === 'review' && change && (
        <Panel
          title="Step 3 · Clean-Code Critic Agent"
          actions={
            <Badge
              value={workflow?.review?.verdict ? `Verdict: ${workflow.review.verdict}` : 'Reviewed'}
              tone={workflow?.review?.verdict === 'approve' ? 'green' : 'violet'}
            />
          }
        >
          <div className="stack" style={{ gap: '12px' }}>
            {workflow?.review ? (
              <div className="stack" style={{ gap: '8px' }}>
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
                Deterministic checks verified the proposal. Proceed to Step 4 to inspect the diffs
                and commit.
              </p>
            )}

            <div className="row between wrap">
              <Button tone="ghost" onClick={() => setSelectedStage('test')}>
                Back to Verify &amp; Test
              </Button>
              {workflow?.status === 'changes_requested' ? (
                <Button
                  tone="primary"
                  icon="sparkles"
                  disabled={busy || running}
                  onClick={() => void agentW.rerunChangeWorkflow('agents')}
                >
                  Re-run Fix with Critic Feedback
                </Button>
              ) : (
                <Button
                  tone="primary"
                  icon="arrow-right"
                  onClick={() => setSelectedStage('human_review')}
                >
                  Next: Review &amp; Commit
                </Button>
              )}
            </div>
          </div>
        </Panel>
      )}

      {activeStage === 'human_review' && change && (
        <Panel
          title={`Step 4 · Review & Commit (${change.title})`}
          actions={
            <div className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
              <span className="small font-mono">
                <span style={{ color: 'var(--green)' }}>+{addedLines}</span> /{' '}
                <span style={{ color: 'var(--red)' }}>-{removedLines}</span> lines ({scopeLabel})
              </span>
              <Badge
                value={
                  change.state === 'applied'
                    ? commitOid
                      ? `Committed #${commitOid.slice(0, 7)}`
                      : 'Applied'
                    : readyToApply
                      ? 'Ready to Commit'
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

            {/* Single Focused Action Bar for Step 4 */}
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
                {change.state === 'applied' && (
                  <Badge
                    value={`Branch: ${commitBranch}${commitOid ? ` (${commitOid.slice(0, 7)})` : ''}`}
                    tone="green"
                  />
                )}
              </div>
              <div className="row wrap" style={{ gap: '8px' }}>
                {change.state === 'draft' && (
                  <Button tone="ghost" icon="code" onClick={() => void agentW.navigate('editor')}>
                    Edit Hunks in Code &amp; Diff
                  </Button>
                )}
                {change.state === 'draft' && changedFiles.length > 0 && (
                  <Button
                    tone="primary"
                    icon="check"
                    disabled={busy || running}
                    onClick={() => void agentW.applyProposalNow(change.title)}
                  >
                    Accept &amp; Commit
                  </Button>
                )}
                {change.state === 'applied' && isWorktreeAgent && selectedChild?.worktreePath && (
                  <Button
                    tone="primary"
                    icon="git"
                    disabled={busy}
                    onClick={() => void w.mergeAgentWorktree(selectedChild.worktreePath!)}
                  >
                    Merge into {mainBranch} &amp; Close Issue
                  </Button>
                )}
                {change.state === 'applied' && !isWorktreeAgent && (
                  <Button tone="primary" icon="check" onClick={() => void w.navigate('bugs')}>
                    Issue Closed · Back to Issues
                  </Button>
                )}
              </div>
            </div>

            {/* File Filter Tabs + Proposed Diffs */}
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
                      const fAdds = fc.diff?.lines?.filter((l) => isAddedLine(l.kind)).length || 0;
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
                title="No diff generated yet"
                detail="Return to Step 1 to generate a fix for the target files."
                icon="code"
              />
            )}
          </div>
        </Panel>
      )}
    </div>
  );
}
