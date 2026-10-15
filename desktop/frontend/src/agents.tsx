import { useEffect, useState } from 'react';
import {
  activeChangeWorkflow,
  canAcceptChange,
  changeChecksPassed,
  workflowSeed,
  workflowSetupModels,
  workspace as w,
  type FindingCategory,
  type State,
} from './workspace';
import { ChangeOutcome, ProposalDiff, isAddedLine, isRemovedLine } from './change-shared';
import { findingName, findingTitle } from './finding-name';
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

export type StageId = 'create' | 'test' | 'review' | 'human_review';

interface StageSpec {
  id: StageId;
  step: string;
  shortTitle: string;
  title: string;
  role: string;
  modelKey?: keyof M.ChangeWorkflowModels;
}

export const PIPELINE_STAGES: StageSpec[] = [
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
    role: 'Inspect proposed diffs, commit, and merge.',
  },
];

export interface AgentLinkedIssue {
  key: string;
  id: string;
  kind: string;
  category: FindingCategory;
  title: string;
  name: string;
  path: string;
  line?: number;
  severity?: string;
  cause?: string;
  solution?: string;
  target: M.FindingTarget;
}

export interface AgentDashboardItem {
  key: string;
  path?: string;
  title: string;
  branch: string;
  stepNumber: 1 | 2 | 3 | 4;
  stageId: StageId;
  stageLabel: string;
  status: string;
  tone?: 'green' | 'violet' | 'amber';
  hasProblem: boolean;
  problemSummary?: string;
  mergeable?: boolean;
  commitOid?: string;
  findingTargets: M.FindingTarget[];
  linkedIssues: AgentLinkedIssue[];
}

function collectKnownIssues(s: State): AgentLinkedIssue[] {
  const map = new Map<string, AgentLinkedIssue>();
  const addSemantic = (f: M.Finding, category: FindingCategory = 'bugs') => {
    const key = `semantic:${f.id}`;
    if (map.has(key)) return;
    map.set(key, {
      key,
      id: f.id,
      kind: 'semantic',
      category: (f.category as FindingCategory) || category,
      title: findingTitle(f.title, f.message),
      name: findingName(f.title, f.message),
      path: f.location.path,
      line: f.location.start_line,
      severity: f.severity,
      cause: f.message || f.evidence || '',
      solution: f.task_spec?.acceptance_criteria.join('\n') || '',
      target: {
        project_id: f.project_id,
        kind: 'semantic',
        id: f.id,
        path: f.location.path,
      },
    });
  };
  for (const f of s.findings || []) {
    addSemantic(f, (f.category as FindingCategory) || 'bugs');
  }
  for (const cat of ['bugs', 'performance', 'security'] as const) {
    const res = s.results[cat];
    for (const f of res?.semantic || []) addSemantic(f, cat);
    for (const r of res?.performance || []) {
      for (const f of r.findings || []) {
        const key = `performance:${r.path}:${f.id}`;
        if (map.has(key)) continue;
        map.set(key, {
          key,
          id: f.id,
          kind: 'performance',
          category: 'performance',
          title: f.title,
          name: findingName(f.title, f.observed_pattern),
          path: r.path,
          line: f.start_line,
          severity: f.potential_impact,
          cause: f.observed_pattern,
          solution: f.recommendation,
          target: {
            project_id: r.project_id,
            kind: 'performance',
            id: f.id,
            path: r.path,
          },
        });
      }
    }
    for (const r of [
      ...(res?.security || []),
      ...(cat === 'security' && s.securityReport ? [s.securityReport] : []),
    ]) {
      for (const f of r.findings || []) {
        const path = f.source_anchor.path || r.path;
        const key = `security:${r.path}:${r.source}:${f.id}`;
        if (map.has(key)) continue;
        map.set(key, {
          key,
          id: f.id,
          kind: 'security',
          category: 'security',
          title: f.title,
          name: findingName(f.title, f.observed_condition),
          path,
          line: f.source_anchor.start_line,
          severity: f.severity,
          cause: f.observed_condition,
          solution: f.remediation,
          target: {
            project_id: r.project_id,
            kind: 'security',
            id: f.id,
            path,
            source: r.source,
          },
        });
      }
    }
  }
  return [...map.values()];
}

function resolveLinkedIssues(
  known: AgentLinkedIssue[],
  targets: M.FindingTarget[],
  fallbackTitle: string,
): AgentLinkedIssue[] {
  const matched: AgentLinkedIssue[] = [];
  for (const target of targets) {
    const found = known.find(
      (item) =>
        item.target.kind === target.kind &&
        item.target.id === target.id &&
        item.target.path === target.path &&
        (item.target.source || '') === (target.source || ''),
    );
    if (found) {
      matched.push(found);
    } else {
      matched.push({
        key: `${target.kind}:${target.id}`,
        id: target.id,
        kind: target.kind,
        category:
          target.kind === 'performance'
            ? 'performance'
            : target.kind === 'security'
              ? 'security'
              : 'bugs',
        title: fallbackTitle || target.id,
        name: fallbackTitle || target.id,
        path: target.path,
        target,
      });
    }
  }
  if (matched.length === 0 && fallbackTitle) {
    const normalized = fallbackTitle.trim().toLowerCase();
    const byTitle = known.find(
      (item) =>
        item.name.trim().toLowerCase() === normalized ||
        item.title.trim().toLowerCase() === normalized,
    );
    if (byTitle) matched.push(byTitle);
  }
  return matched;
}

function buildAgentItemState(params: {
  key: string;
  path?: string;
  title: string;
  branch: string;
  change?: M.ChangeSession;
  seed?: M.ChangeSeed;
  latest?: M.ChangeHistoryEntry;
  tree?: M.GitWorktree;
  agentState?: State;
  knownIssues: AgentLinkedIssue[];
}): AgentDashboardItem {
  const {
    key,
    path,
    title,
    branch,
    change: c,
    seed,
    latest,
    tree,
    agentState,
    knownIssues,
  } = params;
  const running =
    activeChangeWorkflow(c) ||
    latest?.workflow_status === 'running' ||
    latest?.workflow_status === 'canceling';
  const committed = c?.state === 'applied' || latest?.state === 'applied' || !!tree?.mergeable;
  const ready =
    (agentState ? canAcceptChange(agentState) : false) ||
    (c?.changes?.length || 0) > 0 ||
    latest?.workflow_status === 'awaiting_human_review';

  const checks = c?.checks || [];
  const checksFailed = !running && checks.length > 0 && !changeChecksPassed(c!);
  const changesRequested =
    c?.workflow?.status === 'changes_requested' ||
    c?.workflow?.review?.verdict === 'changes_requested' ||
    latest?.workflow_status === 'changes_requested';
  const workflowFailed = c?.workflow?.status === 'failed' || latest?.workflow_status === 'failed';
  const commitFailed =
    agentState?.changeReceipt?.commit?.state === 'failed' || c?.commit?.state === 'failed';
  const hasProblem =
    !!tree?.error || workflowFailed || changesRequested || checksFailed || commitFailed;

  let problemSummary: string | undefined;
  if (tree?.error) {
    problemSummary = tree.error;
  } else if (commitFailed) {
    problemSummary =
      agentState?.changeReceipt?.commit?.error || c?.commit?.error || 'Git commit failed';
  } else if (checksFailed) {
    const failedCheck = checks.find((chk) => chk.state !== 'passed');
    problemSummary =
      c?.workflow?.reason ||
      (failedCheck ? `Check failed: ${human(failedCheck.name)}` : 'Verification checks failed');
  } else if (changesRequested) {
    problemSummary = c?.workflow?.review?.summary || 'Critic requested changes';
  } else if (workflowFailed) {
    problemSummary = c?.workflow?.reason || 'Agent workflow failed · Needs attention';
  }

  let stepNumber: 1 | 2 | 3 | 4 = 1;
  let stageId: StageId = 'create';
  let stageLabel = 'Step 1/4 · Create Fix';

  if (running) {
    const runningStage = c?.workflow?.stages.find((st) => st.status === 'running')?.name;
    if (runningStage === 'test') {
      stepNumber = 2;
      stageId = 'test';
      stageLabel = 'Step 2/4 · Verify & Test';
    } else if (runningStage === 'review') {
      stepNumber = 3;
      stageId = 'review';
      stageLabel = 'Step 3/4 · Critic Review';
    } else {
      stepNumber = 1;
      stageId = 'create';
      stageLabel = 'Step 1/4 · Create Fix';
    }
  } else if (checksFailed) {
    stepNumber = 2;
    stageId = 'test';
    stageLabel = 'Step 2/4 · Checks Failed';
  } else if (changesRequested) {
    stepNumber = 3;
    stageId = 'review';
    stageLabel = 'Step 3/4 · Changes Requested';
  } else if (workflowFailed) {
    const failedTest = c?.workflow?.stages.find((st) => st.name === 'test')?.status === 'failed';
    const failedReview =
      c?.workflow?.stages.find((st) => st.name === 'review')?.status === 'failed';
    if (failedTest) {
      stepNumber = 2;
      stageId = 'test';
      stageLabel = 'Step 2/4 · Test Failed';
    } else if (failedReview) {
      stepNumber = 3;
      stageId = 'review';
      stageLabel = 'Step 3/4 · Review Failed';
    } else {
      stepNumber = 1;
      stageId = 'create';
      stageLabel = 'Step 1/4 · Workflow Failed';
    }
  } else if (tree?.mergeable) {
    stepNumber = 4;
    stageId = 'human_review';
    stageLabel = 'Step 4/4 · Ready to Merge';
  } else if (committed) {
    stepNumber = 4;
    stageId = 'human_review';
    stageLabel = 'Step 4/4 · Committed';
  } else if (ready) {
    stepNumber = 4;
    stageId = 'human_review';
    stageLabel = 'Step 4/4 · Ready to Commit';
  } else if (seed && !c) {
    stepNumber = 1;
    stageId = 'create';
    stageLabel = 'Step 1/4 · Staged';
  }

  const status = hasProblem
    ? 'failed'
    : tree?.mergeable || committed
      ? 'completed'
      : running
        ? 'running'
        : ready
          ? 'awaiting_human_review'
          : c?.workflow?.status ||
            latest?.workflow_status ||
            c?.state ||
            latest?.state ||
            'pending';

  const rawTargets = [
    ...(c?.finding_targets || []),
    ...(seed?.finding_targets || []),
    ...[...(tree?.sessions || []), ...(tree?.completed || [])].flatMap(
      (entry) => entry.finding_targets || [],
    ),
  ];
  const seenTargetKeys = new Set<string>();
  const findingTargets = rawTargets.filter((t) => {
    const k = `${t.project_id}:${t.kind}:${t.source || ''}:${t.id}:${t.path}`;
    if (seenTargetKeys.has(k)) return false;
    seenTargetKeys.add(k);
    return true;
  });
  const linkedIssues = resolveLinkedIssues(knownIssues, findingTargets, title);

  return {
    key,
    path,
    title,
    branch,
    stepNumber,
    stageId,
    stageLabel,
    status,
    tone: hasProblem
      ? 'amber'
      : tree?.mergeable || committed || ready
        ? 'green'
        : running
          ? 'violet'
          : undefined,
    hasProblem,
    problemSummary,
    mergeable: tree?.mergeable,
    commitOid: c?.commit?.oid || agentState?.changeReceipt?.commit?.oid || latest?.commit?.oid,
    findingTargets:
      findingTargets.length > 0 ? findingTargets : linkedIssues.map((iss) => iss.target),
    linkedIssues,
  };
}

export function collectAgentDashboardItems(s: State): AgentDashboardItem[] {
  const items: AgentDashboardItem[] = [];
  const knownIssues = collectKnownIssues(s);
  const mainBranch = s.repository?.branch || 'main';

  if (s.change || s.changeSeed) {
    items.push(
      buildAgentItemState({
        key: 'main',
        path: undefined,
        title: s.change?.title || s.changeSeed?.title || 'Main Workspace Agent',
        branch: mainBranch,
        change: s.change,
        seed: s.changeSeed,
        agentState: s,
        knownIssues,
      }),
    );
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
    items.push(
      buildAgentItemState({
        key: tree.path,
        path: tree.path,
        title,
        branch: tree.branch,
        change: c,
        seed,
        latest,
        tree,
        agentState: child?.state,
        knownIssues,
      }),
    );
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

  const [showNewForm, setShowNewForm] = useState(false);
  const [missionTitle, setMissionTitle] = useState(seed?.title || '');
  const [missionPaths, setMissionPaths] = useState(
    seed?.paths.join(', ') || s.file?.path || s.index?.files[0]?.path || '',
  );
  const [missionPrompt, setMissionPrompt] = useState(seed?.message || '');
  const [selectedDiffPath, setSelectedDiffPath] = useState<string>('all');

  useEffect(() => {
    if (s.selectedAgentPath && !selectedChild && !s.busy) {
      void w.ensureAgentWorktree(s.selectedAgentPath);
    }
  }, [s.selectedAgentPath, selectedChild, s.busy]);

  useEffect(() => {
    if (s.agentDetailOpen) {
      setShowNewForm(false);
    }
  }, [s.agentDetailOpen]);

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
    s.agentDetailOpen,
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
    setShowNewForm(false);
    void w.startAgentFix(prepared);
  };

  const openNewAgentForm = () => {
    setMissionTitle('');
    setMissionPaths(s.file?.path || s.index?.files[0]?.path || '');
    setMissionPrompt('');
    w.set({ agentDetailOpen: false, error: '' });
    setShowNewForm(true);
  };

  const dashboardItems = collectAgentDashboardItems(s);
  const currentAgentItem =
    dashboardItems.find((item) => (item.path || '') === (s.selectedAgentPath || '')) ||
    dashboardItems[0];
  const linkedIssues = currentAgentItem?.linkedIssues || [];

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

  // PAGE 1: NEW CUSTOM AGENT FORM
  if (!s.agentDetailOpen && showNewForm) {
    return (
      <div className="workspace-page agents-page">
        <Heading
          variant="intro"
          title="New Agent Fix"
          detail="Start an isolated agent pipeline on up to 8 files."
        >
          <Button icon="back" tone="ghost" onClick={() => setShowNewForm(false)}>
            Back to Agents Dashboard
          </Button>
        </Heading>
        <Panel title="Agent Mission Specification">
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
                rows={4}
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
                disabled={busy}
                onClick={dispatchCustomMission}
              >
                Dispatch Agent Pipeline
              </Button>
            </div>
          </div>
        </Panel>
      </div>
    );
  }

  // PAGE 2: AGENTS IN PROGRESS DASHBOARD
  if (!s.agentDetailOpen) {
    const runningCount = dashboardItems.filter((i) => i.status === 'running').length;
    const problemCount = dashboardItems.filter((i) => i.hasProblem).length;
    const readyCount = dashboardItems.filter(
      (i) => i.status === 'awaiting_human_review' || i.mergeable || i.status === 'completed',
    ).length;

    return (
      <div className="workspace-page agents-page">
        <Heading
          variant="intro"
          title="Agent Mission Control"
          detail={
            dashboardItems.length > 0
              ? `${dashboardItems.length} active ${dashboardItems.length === 1 ? 'agent' : 'agents'} · ${runningCount} running · ${readyCount} ready${problemCount > 0 ? ` · ${problemCount} need attention` : ''}`
              : 'Dashboard of agents in progress. Click an agent to inspect its step and grouped issues.'
          }
        >
          <Go page="bugs" tone="ghost" icon="bug">
            Browse Issues
          </Go>
          <Go page="git" tone="ghost" icon="git">
            Git &amp; Worktrees
          </Go>
          <Button tone="primary" icon="plus" disabled={!!s.busy} onClick={openNewAgentForm}>
            New Fix
          </Button>
        </Heading>

        {dashboardItems.length === 0 ? (
          <Empty
            title="No agents in progress"
            detail="Select one or more issues in Issues & Ideas to fix with an agent, or start a custom agent fix."
            icon="sparkles"
          >
            <Go page="bugs" tone="primary" icon="bug">
              Browse Issues
            </Go>
            <Button tone="ghost" icon="plus" onClick={openNewAgentForm}>
              New Fix
            </Button>
          </Empty>
        ) : (
          <Panel
            title={`Agents in Progress (${dashboardItems.length})`}
            actions={
              <span className="small muted">
                Click an agent to open its step and grouped issue detail
              </span>
            }
          >
            <div
              className="stack"
              style={{ gap: '12px' }}
              role="tablist"
              aria-label="Active agents"
            >
              {dashboardItems.map((item) => {
                const issueCount = item.linkedIssues.length;
                return (
                  <div
                    key={item.key}
                    role="tab"
                    tabIndex={0}
                    aria-selected={(s.selectedAgentPath || undefined) === item.path}
                    onClick={() => void w.selectAgent(item.path)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        void w.selectAgent(item.path);
                      }
                    }}
                    className="kpi-pulse-card card-kpi box-card agent-dashboard-card"
                    style={{
                      textAlign: 'left',
                      cursor: 'pointer',
                      padding: '14px 16px',
                      borderColor: item.hasProblem
                        ? 'var(--red, #e5534b)'
                        : item.status === 'running'
                          ? 'var(--accent)'
                          : undefined,
                      boxShadow: item.hasProblem ? '0 0 0 1px var(--red, #e5534b)' : undefined,
                    }}
                  >
                    <div className="row between wrap" style={{ gap: '10px' }}>
                      <div className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
                        <StatusDot value={item.status} label={item.title} />
                        <strong style={{ fontSize: '0.98rem' }}>{item.title}</strong>
                        <Badge value={item.branch} />
                        {issueCount > 0 && (
                          <Badge
                            value={`${issueCount} ${issueCount === 1 ? 'issue' : 'issues'}`}
                            tone="violet"
                          />
                        )}
                        {item.commitOid && (
                          <span className="small font-mono muted">
                            #{item.commitOid.slice(0, 7)}
                          </span>
                        )}
                      </div>
                      <div className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
                        {item.hasProblem && <Badge value="Needs Attention" tone="amber" />}
                        <Badge value={item.stageLabel} tone={item.tone} />
                      </div>
                    </div>

                    {/* Simple 4-Step Progress Strip */}
                    <div
                      className="row wrap"
                      style={{
                        gap: '6px',
                        marginTop: '10px',
                        paddingTop: '8px',
                        borderTop: '1px solid var(--line)',
                      }}
                    >
                      {PIPELINE_STAGES.map((st, idx) => {
                        const stepNum = idx + 1;
                        const isDone =
                          item.stepNumber > stepNum ||
                          (item.stepNumber === 4 &&
                            (item.mergeable || item.status === 'completed'));
                        const isCurrent = item.stepNumber === stepNum && !isDone;
                        const isProblemStep = isCurrent && item.hasProblem;
                        return (
                          <span
                            key={st.id}
                            className="small font-mono"
                            style={{
                              padding: '3px 8px',
                              borderRadius: '4px',
                              background: isProblemStep
                                ? 'rgba(229, 83, 75, 0.16)'
                                : isCurrent
                                  ? 'var(--bg-elevated)'
                                  : 'transparent',
                              border: isProblemStep
                                ? '1px solid var(--red, #e5534b)'
                                : isCurrent
                                  ? '1px solid var(--accent)'
                                  : '1px solid var(--line)',
                              color: isProblemStep
                                ? 'var(--red, #e5534b)'
                                : isDone
                                  ? 'var(--green)'
                                  : isCurrent
                                    ? 'var(--fg)'
                                    : 'var(--muted)',
                              fontWeight: isCurrent ? 600 : 400,
                            }}
                          >
                            {isProblemStep ? '⚠ ' : isDone ? '✓ ' : isCurrent ? '● ' : ''}
                            {stepNum}. {st.shortTitle}
                          </span>
                        );
                      })}
                    </div>

                    {/* Grouped Issues Preview */}
                    {item.linkedIssues.length > 0 && (
                      <div
                        className="row wrap small muted"
                        style={{ gap: '8px', marginTop: '8px' }}
                      >
                        <span>Fixing:</span>
                        {item.linkedIssues.map((iss) => (
                          <span key={iss.key} className="font-mono">
                            {iss.name} ({iss.path}
                            {iss.line ? `:${iss.line}` : ''})
                          </span>
                        ))}
                      </div>
                    )}

                    {/* Highlighted Problem Callout */}
                    {item.hasProblem && item.problemSummary && (
                      <div
                        role="alert"
                        style={{
                          marginTop: '10px',
                          padding: '8px 12px',
                          borderRadius: '6px',
                          background: 'rgba(229, 83, 75, 0.12)',
                          border: '1px solid var(--red, #e5534b)',
                          fontSize: '0.85rem',
                        }}
                      >
                        <strong>Problem: </strong>
                        <span>{item.problemSummary}</span>
                      </div>
                    )}

                    {item.mergeable && item.path && (
                      <div
                        className="row end"
                        style={{ marginTop: '10px' }}
                        onClick={(e) => e.stopPropagation()}
                      >
                        <Button
                          tone="primary small"
                          icon="git"
                          disabled={busy}
                          onClick={() => void w.mergeAgentWorktree(item.path!)}
                        >
                          Merge into {mainBranch} &amp; Close{' '}
                          {issueCount > 1 ? `${issueCount} Issues` : 'Issue'}
                        </Button>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </Panel>
        )}
      </div>
    );
  }

  // PAGE 3: AGENT & GROUPED ISSUE DETAIL PAGE
  return (
    <div className="workspace-page agents-page">
      <div className="row between wrap" style={{ marginBottom: '8px' }}>
        <Button icon="back" tone="ghost" onClick={() => w.openAgentsDashboard()}>
          All Agents ({dashboardItems.length})
        </Button>
        <div className="row wrap" style={{ gap: '8px' }}>
          {agentState.changeReceipt?.undo_available && (
            <Button
              tone="ghost"
              icon="undo"
              disabled={busy}
              onClick={() => void agentW.undoChange()}
            >
              Undo Last Change
            </Button>
          )}
          {activeStage !== 'human_review' &&
            change?.state === 'draft' &&
            changedFiles.length > 0 && (
              <Button
                tone="primary"
                icon="check"
                disabled={busy || running}
                onClick={() => void agentW.applyProposalNow(change.title)}
              >
                Accept &amp; Commit
              </Button>
            )}
          {activeStage !== 'human_review' &&
            change?.state === 'applied' &&
            isWorktreeAgent &&
            selectedChild?.worktreePath && (
              <Button
                tone="primary"
                icon="git"
                disabled={busy}
                onClick={() => void w.mergeAgentWorktree(selectedChild.worktreePath!)}
              >
                Merge into {mainBranch} &amp; Close{' '}
                {linkedIssues.length > 1 ? `${linkedIssues.length} Issues` : 'Issue'}
              </Button>
            )}
        </div>
      </div>

      <Heading
        variant="intro"
        title="Agent Mission Control"
        detail={
          <span className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
            <strong>{change?.title || seed?.title || currentAgentItem?.title || 'Agent'}</strong>
            <Badge value={`Branch: ${commitBranch}`} />
            {currentAgentItem && (
              <Badge value={currentAgentItem.stageLabel} tone={currentAgentItem.tone} />
            )}
            {commitOid && <Badge value={`#${commitOid.slice(0, 7)}`} tone="green" />}
          </span>
        }
      />

      {agentState.error && <Notice error>{agentState.error}</Notice>}
      {currentAgentItem?.hasProblem && currentAgentItem.problemSummary && (
        <Notice error>
          <div className="row between wrap" style={{ gap: '10px', alignItems: 'center' }}>
            <span>
              <strong>Problem detected: </strong>
              {currentAgentItem.problemSummary}
            </span>
            {!running && checks.length > 0 && !allChecksPassed && (
              <Button
                tone="primary small"
                icon="sparkles"
                disabled={busy}
                onClick={() => void agentW.repairChange()}
              >
                Repair Failed Checks
              </Button>
            )}
          </div>
        </Notice>
      )}
      <ChangeOutcome s={agentState} />

      {/* Grouped Issues Panel */}
      {linkedIssues.length > 0 && (
        <Panel
          title={`Grouped Issues in this Agent (${linkedIssues.length})`}
          actions={
            <span className="small muted">
              Marked Done automatically when merged into {mainBranch}
            </span>
          }
        >
          <div className="stack" style={{ gap: '8px' }}>
            {linkedIssues.map((iss) => (
              <div
                key={iss.key}
                className="list-row"
                style={{ alignItems: 'flex-start', padding: '8px 10px' }}
              >
                <div className="list-copy" style={{ flex: 1 }}>
                  <div className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
                    {iss.severity && <Badge value={iss.severity} />}
                    <strong>{iss.name}</strong>
                    <span className="small font-mono muted">
                      {iss.path}
                      {iss.line ? `:${iss.line}` : ''}
                    </span>
                  </div>
                  {iss.cause && (
                    <p className="small muted" style={{ marginTop: '4px', marginBottom: 0 }}>
                      {iss.cause}
                    </p>
                  )}
                </div>
                <Button
                  tone="ghost small"
                  onClick={() => void w.openFinding(iss.category, iss.key)}
                >
                  Issue Detail
                </Button>
              </div>
            ))}
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
                  : status === 'failed'
                    ? 'var(--red, #e5534b)'
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
                        : status === 'failed'
                          ? 'amber'
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
              <div className="box-meta" style={{ marginTop: '4px' }}>
                <span className="font-mono small">{stageModel}</span>
              </div>
            </button>
          );
        })}
      </div>

      {/* Stage Content: Render ONLY the currently selected stage */}
      {activeStage === 'create' && (
        <Panel
          title={
            change
              ? `Step 1 · Architect & Code Agent (${change.title})`
              : seed
                ? `Step 1 · Staged Fix (${seed.title})`
                : 'Step 1 · Create Fix'
          }
          actions={
            <Badge
              value={human(stageStatus('create'))}
              tone={stageStatus('create') === 'completed' ? 'green' : 'violet'}
            />
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
              <p className="small muted">
                Target files: <code>{seed?.paths.join(', ') || missionPaths}</code>
              </p>
              {userPrompt && <Prose text={userPrompt} />}
              <div className="row end">
                <Button
                  tone="primary"
                  icon="sparkles"
                  disabled={busy}
                  onClick={() => void agentW.rerunChangeWorkflow('agents')}
                >
                  Start Agent Pipeline
                </Button>
              </div>
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
              tone={allChecksPassed ? 'green' : 'amber'}
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
                          tone={check.state === 'passed' ? 'green' : 'amber'}
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
                    tone={workflow.review.verdict === 'approve' ? 'green' : 'amber'}
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
                    tone={workflow.review.verdict === 'approve' ? 'green' : 'amber'}
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
                    Merge into {mainBranch} &amp; Close{' '}
                    {linkedIssues.length > 1 ? `${linkedIssues.length} Issues` : 'Issue'}
                  </Button>
                )}
              </div>
            </div>

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
