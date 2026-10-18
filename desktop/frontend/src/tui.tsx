import { useEffect, useMemo, useRef, useState } from 'react';
import {
  activeChangeWorkflow,
  activeRun,
  allModelPresets,
  canAcceptChange,
  canRepairAnalysis,
  changeChecksPassed,
  evaluateBootstrapChecklist,
  findingCategory,
  isModelPresetActive,
  recoverableFileCount,
  workflowSeed,
  workflowSetupModels,
  workspace as w,
  type Page,
  type State,
} from './workspace';
import { collectAgentDashboardItems, type StageId } from './agents';
import { collectHarnessLessons, containsGuideline, insertGuidelineSection } from './instructions';
import { resolveProjectDocs } from './docs';
import { dedupeFindingItems, findingName, findingTitle } from './finding-name';
import { compareBySeverity, computeWorkspaceIssueSummary } from './results';
import { isAddedLine, isRemovedLine, summarizeFileEdit } from './change-shared';
import { Spinner, classifyNoticeTone, human } from './ui';
import type * as M from './models';
import './tui.css';

type TuiPalette = 'tokyo' | 'crt' | 'amber' | 'paper';
type TuiDocsSection = 'context' | 'components' | 'scenarios' | 'decisions';

const TUI_TABS: {
  key: string;
  page: Page;
  label: string;
  subPages: { page: Page; label: string }[];
}[] = [
  { key: '1', page: 'summary', label: 'Overview', subPages: [] },
  {
    key: '2',
    page: 'agents',
    label: 'Agent Studio',
    subPages: [
      { page: 'agents', label: 'Agents' },
      { page: 'bugs', label: 'Issues' },
      { page: 'features', label: 'Features' },
      { page: 'analysis-run', label: 'Last Run' },
    ],
  },
  {
    key: '3',
    page: 'editor',
    label: 'Code & Diff',
    subPages: [
      { page: 'editor', label: 'Code & Diff' },
      { page: 'git', label: 'Git & Worktrees' },
    ],
  },
  {
    key: '4',
    page: 'docs',
    label: 'Handbook & Arch',
    subPages: [
      { page: 'docs', label: 'Handbook / Docs' },
      { page: 'diagrams', label: 'Architecture' },
    ],
  },
  {
    key: '5',
    page: 'instructions',
    label: 'Governance & Rules',
    subPages: [],
  },
];

const PALETTES: { id: TuiPalette; label: string }[] = [
  { id: 'tokyo', label: 'Tokyo' },
  { id: 'crt', label: 'CRT Green' },
  { id: 'amber', label: 'Amber' },
  { id: 'paper', label: 'Paper' },
];

interface TuiIssueRow {
  id: string;
  title: string;
  name: string;
  path: string;
  symbol: string;
  line: number;
  severity: string;
  category: 'bugs' | 'security' | 'performance';
  status: 'open' | 'in_progress' | 'done';
  commitOid?: string;
  agent?: ReturnType<typeof collectAgentDashboardItems>[number];
  cause: string;
  solution: string;
  target: M.FindingTarget;
  acceptance_criteria: string[];
}

function collectTuiIssues(s: State, agentItems = collectAgentDashboardItems(s)): TuiIssueRow[] {
  const findCompletion = (target: M.FindingTarget) =>
    s.completedFindings?.find(
      (c) =>
        c.project_id === target.project_id &&
        c.kind === target.kind &&
        c.id === target.id &&
        c.path === target.path &&
        (c.source || '') === (target.source || ''),
    );
  const findAgent = (target: M.FindingTarget, id: string, name: string, title: string) => {
    const rowTitle = name.trim().toLowerCase();
    const fullTitle = title.trim().toLowerCase();
    return agentItems.find((item) => {
      if (
        item.findingTargets.some(
          (t) =>
            t.kind === target.kind &&
            t.id === target.id &&
            t.path === target.path &&
            (t.source || '') === (target.source || ''),
        )
      ) {
        return true;
      }
      if (item.linkedIssues.some((iss) => iss.key === id)) {
        return true;
      }
      const itemTitle = item.title.trim().toLowerCase();
      return itemTitle === rowTitle || itemTitle === fullTitle;
    });
  };

  const rows: TuiIssueRow[] = [];
  const pushSemantic = (f: M.Finding, cat: 'bugs' | 'security' | 'performance') => {
    if (f.status === 'retired' || f.status === 'dismissed') return;
    const id = `semantic:${f.id}`;
    if (rows.some((r) => r.id === id)) return;
    const target: M.FindingTarget = {
      project_id: f.project_id,
      kind: 'semantic',
      id: f.id,
      path: f.location.path,
    };
    const title = findingTitle(f.title, f.message);
    const name = findingName(f.title, f.message);
    const completion = findCompletion(target);
    const isDone = f.status === 'fixed' || !!completion;
    const agent = !isDone ? findAgent(target, id, name, title) : undefined;
    rows.push({
      id,
      title,
      name,
      path: f.location.path,
      symbol: f.location.symbol,
      line: f.location.start_line,
      severity: f.severity,
      category: cat,
      status: isDone ? 'done' : agent ? 'in_progress' : 'open',
      commitOid: completion?.commit_oid,
      agent,
      cause: f.message || f.evidence || 'No cause recorded.',
      solution:
        f.task_spec?.acceptance_criteria.join('\n') ||
        'Synthesize a minimal, verified fix and unit test.',
      target,
      acceptance_criteria: f.task_spec?.acceptance_criteria || [],
    });
  };

  for (const f of s.findings || []) {
    const cat = (f.category as 'bugs' | 'security' | 'performance') || 'bugs';
    if (!s.results[cat] || f.status === 'in_progress') {
      pushSemantic(f, cat);
    }
  }
  for (const cat of ['bugs', 'security', 'performance'] as const) {
    const res = s.results[cat];
    for (const f of res?.semantic || []) {
      pushSemantic(f, cat);
    }
    for (const rep of res?.performance || []) {
      for (const f of rep.findings || []) {
        const id = `performance:${rep.path}:${f.id}`;
        if (rows.some((r) => r.id === id)) continue;
        const target: M.FindingTarget = {
          project_id: rep.project_id,
          kind: 'performance',
          id: f.id,
          path: rep.path,
        };
        const title = f.title;
        const name = findingName(f.title, f.observed_pattern);
        const completion = findCompletion(target);
        const isDone = !!completion;
        const agent = !isDone ? findAgent(target, id, name, title) : undefined;
        rows.push({
          id,
          title,
          name,
          path: rep.path,
          symbol: f.symbol,
          line: f.start_line,
          severity: f.potential_impact,
          category: 'performance',
          status: isDone ? 'done' : agent ? 'in_progress' : 'open',
          commitOid: completion?.commit_oid,
          agent,
          cause: f.observed_pattern || 'No cause recorded.',
          solution: f.recommendation || 'Synthesize a minimal, verified fix and unit test.',
          target,
          acceptance_criteria: [],
        });
      }
    }
    const secReports = [
      ...(res?.security || []),
      ...(cat === 'security' && s.securityReport ? [s.securityReport] : []),
    ];
    for (const rep of secReports) {
      for (const f of rep.findings || []) {
        if (f.triage === 'dismissed') continue;
        const path = f.source_anchor.path || rep.path;
        const id = `security:${rep.path}:${rep.source}:${f.id}`;
        if (rows.some((r) => r.id === id)) continue;
        const target: M.FindingTarget = {
          project_id: rep.project_id,
          kind: 'security',
          id: f.id,
          path,
          source: rep.source,
        };
        const title = f.title;
        const name = findingName(f.title, f.observed_condition);
        const completion = findCompletion(target);
        const isDone = f.triage === 'fixed' || !!completion;
        const agent = !isDone ? findAgent(target, id, name, title) : undefined;
        rows.push({
          id,
          title,
          name,
          path,
          symbol: f.source_anchor.symbol,
          line: f.source_anchor.start_line,
          severity: f.severity,
          category: 'security',
          status: isDone ? 'done' : agent ? 'in_progress' : 'open',
          commitOid: completion?.commit_oid,
          agent,
          cause: f.observed_condition || 'No cause recorded.',
          solution: f.remediation || 'Synthesize a minimal, verified fix and unit test.',
          target,
          acceptance_criteria: [],
        });
      }
    }
  }
  return dedupeFindingItems(rows).sort(compareBySeverity);
}

export function TuiApp({
  s,
  activePage,
  onExitTui,
}: {
  s: State;
  activePage: Page;
  onExitTui: () => void;
}) {
  const [palette, setPalette] = useState<TuiPalette>(() => {
    const saved = localStorage.getItem('mini-orca:tui-palette') as TuiPalette | null;
    return PALETTES.some((p) => p.id === saved) ? saved! : 'tokyo';
  });
  const [selectedIdx, setSelectedIdx] = useState(0);
  const [issueView, setIssueView] = useState<'open' | 'in_progress' | 'done'>('open');
  const [issueCategory, setIssueCategory] = useState<'all' | 'bugs' | 'security' | 'performance'>(
    'all',
  );
  const [selectedIssueIds, setSelectedIssueIds] = useState<string[]>([]);
  const [showNewMissionForm, setShowNewMissionForm] = useState(false);
  const [showKeybindsModal, setShowKeybindsModal] = useState(false);
  const [docsSection, setDocsSection] = useState<TuiDocsSection>('context');
  const [rulesSection, setRulesSection] = useState<'trust' | 'models' | 'bootstrap' | 'lessons'>(
    s.instructionsTab === 'bootstrap' ? 'bootstrap' : activePage === 'models' ? 'models' : 'trust',
  );
  useEffect(() => {
    if (s.instructionsTab === 'bootstrap') {
      setRulesSection('bootstrap');
      w.set({ instructionsTab: undefined });
    } else if (activePage === 'models') {
      setRulesSection('models');
    }
  }, [s.instructionsTab, activePage]);
  const [selectedAgentStage, setSelectedAgentStage] = useState<StageId | 'all'>('all');
  const [editorMode, setEditorMode] = useState<'auto' | 'source' | 'diff'>('auto');
  const [showTuiInlineWhy, setShowTuiInlineWhy] = useState(true);
  const [graduatedTuiLessons, setGraduatedTuiLessons] = useState<string[]>([]);
  const [tuiCustomDraft, setTuiCustomDraft] = useState('');
  const [editorPrompt, setEditorPrompt] = useState('');
  const [featureGoalInput, setFeatureGoalInput] = useState('');
  const [cmdInput, setCmdInput] = useState('');
  const [cmdHistory, setCmdHistory] = useState<string[]>([]);
  const [cmdHistoryIdx, setCmdHistoryIdx] = useState(-1);
  const [cmdOutput, setCmdOutput] = useState(
    'Press [?] or type :keys for keyboard shortcuts · [1-5] switch hubs · [Tab] sub-view · [j/k] move · [Enter] select · [:gui] exit',
  );
  const selectedChild = s.selectedAgentPath
    ? s.implementations.find((impl) => impl.worktreePath === s.selectedAgentPath)
    : undefined;
  const agentW = selectedChild || w;
  const agentS = selectedChild ? selectedChild.state : s;
  const agentItems = useMemo(
    () => collectAgentDashboardItems(s),
    [
      s.change,
      s.changeSeed,
      s.changeReceipt,
      s.gitWorkspace,
      s.implementations,
      s.findings,
      s.results,
      s.repository,
    ],
  );
  const selectedAgentItem =
    agentItems.find((item) => (item.path || '') === (s.selectedAgentPath || '')) || agentItems[0];

  const [missionTitle, setMissionTitle] = useState(agentS.changeSeed?.title || '');
  const [missionPaths, setMissionPaths] = useState(
    agentS.changeSeed?.paths.join(', ') || s.file?.path || s.index?.files[0]?.path || '',
  );
  const [missionPrompt, setMissionPrompt] = useState(agentS.changeSeed?.message || '');
  const cmdRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    localStorage.setItem('mini-orca:tui-palette', palette);
  }, [palette]);

  useEffect(() => {
    setSelectedIdx(0);
  }, [activePage, s.agentDetailOpen, issueView, issueCategory]);

  useEffect(() => {
    if (agentS.changeSeed) {
      setMissionTitle(agentS.changeSeed.title);
      setMissionPaths(agentS.changeSeed.paths.join(', '));
      setMissionPrompt(agentS.changeSeed.message);
    }
  }, [agentS.changeSeed?.title, agentS.changeSeed?.message, s.selectedAgentPath]);

  useEffect(() => {
    if (!s.features && !s.resourceErrors['feature suggestions']) void w.loadFeatures();
  }, [s.project?.project_id]);

  useEffect(() => {
    if (!s.run) return;
    for (const cat of ['bugs', 'security', 'performance'] as const) {
      const current = s.results[cat];
      const section = s.run.sections.find((sec) => sec.category === cat);
      const needsLoad =
        !current ||
        JSON.stringify(current.identity) !== JSON.stringify(s.run.identity) ||
        JSON.stringify(current.progress) !== JSON.stringify(section);
      if (needsLoad && !s.resourceErrors[cat] && !s.resourceErrors[`${cat} results`]) {
        void w.loadResults(cat);
      }
    }
  }, [s.run]);

  useEffect(() => {
    if (s.run?.features && !s.resourceErrors['feature suggestions']) void w.loadFeatures();
  }, [s.run?.features?.status, s.run?.features?.report_hash]);

  const currentTabPage: Page =
    findingCategory(s) || s.page === 'finding'
      ? 'bugs'
      : ['models', 'instructions', 'analysis-files', 'context'].includes(activePage)
        ? 'instructions'
        : activePage;

  const activeHubEntry =
    TUI_TABS.find(
      (t) => t.page === currentTabPage || t.subPages.some((sp) => sp.page === currentTabPage),
    ) || TUI_TABS[0];

  const docs = useMemo(() => resolveProjectDocs(s), [s.overview, s.project, s.index]);
  const issueSummary = useMemo(
    () => computeWorkspaceIssueSummary(s),
    [s.results, s.findings, s.completedFindings, s.run, s.overview, s.features],
  );
  const allIssues = useMemo(
    () => collectTuiIssues(s, agentItems),
    [s.findings, s.results, s.completedFindings, s.securityReport, agentItems],
  );
  const allOpenIssues = useMemo(() => allIssues.filter((i) => i.status !== 'done'), [allIssues]);
  const categoryFilteredIssues = allIssues.filter(
    (i) => issueCategory === 'all' || i.category === issueCategory,
  );
  const openIssues = categoryFilteredIssues.filter((i) => i.status !== 'done');
  const inProgressIssues = categoryFilteredIssues.filter((i) => i.status === 'in_progress');
  const doneIssues = categoryFilteredIssues.filter((i) => i.status === 'done');
  const issues =
    issueView === 'in_progress' ? inProgressIssues : issueView === 'done' ? doneIssues : openIssues;
  const overviewIssues = allOpenIssues.slice(0, 8);
  const archComponents = docs.components || [];
  const editorFiles = (s.index?.files || []).slice(0, 24);
  const featureSuggestions = (s.features?.suggestions || []).filter(
    (idea) => idea.status !== 'dismissed',
  );
  const runSections = s.run?.sections || [];
  const gitWorktrees = s.gitWorkspace?.worktrees || [];
  const modelPresets = useMemo(() => allModelPresets(s), [s.modelPresets]);

  const activeListLength =
    currentTabPage === 'summary'
      ? overviewIssues.length
      : currentTabPage === 'diagrams'
        ? archComponents.length
        : currentTabPage === 'editor'
          ? editorFiles.length
          : currentTabPage === 'bugs'
            ? issues.length
            : currentTabPage === 'features'
              ? featureSuggestions.length
              : currentTabPage === 'agents' && !s.agentDetailOpen && !showNewMissionForm
                ? agentItems.length
                : currentTabPage === 'analysis-run'
                  ? runSections.length
                  : currentTabPage === 'git'
                    ? gitWorktrees.length
                    : currentTabPage === 'instructions'
                      ? modelPresets.length
                      : 0;

  const clampedIdx = Math.min(selectedIdx, Math.max(0, activeListLength - 1));
  const selectedIssue =
    currentTabPage === 'summary' ? overviewIssues[clampedIdx] : issues[clampedIdx];
  const models = workflowSetupModels(s);
  const change = agentS.change;
  const workflow = change?.workflow;
  const running = activeChangeWorkflow(change);
  const checks = change?.checks || [];
  const passedChecks = checks.filter((c) => c.state === 'passed').length;
  const allChecksPassed = change ? changeChecksPassed(change) : false;

  const toggleIssueSelection = (id: string) => {
    setSelectedIssueIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id],
    );
  };

  const dispatchIssueToAgent = (issue: TuiIssueRow) => {
    const seed = workflowSeed({
      finding_targets: [issue.target],
      title: issue.name,
      paths: [issue.path],
      kind:
        issue.category === 'security'
          ? 'security'
          : issue.category === 'performance'
            ? 'performance'
            : 'fix',
      message: `Address this finding: ${issue.title}\nLocation: ${issue.path}${issue.line ? `:${issue.line}` : ''}\nCause: ${issue.cause}\nProposed solution: ${issue.solution}`,
      acceptance_criteria: issue.acceptance_criteria,
    });
    setCmdOutput(`Dispatched Fix for ${issue.name} (${issue.path})`);
    void w.startAgentFix(seed);
  };

  const dispatchBatchIssuesToAgent = (batch: TuiIssueRow[]) => {
    if (!batch.length) return;
    if (batch.length === 1) {
      setSelectedIssueIds([]);
      dispatchIssueToAgent(batch[0]);
      return;
    }
    const paths = [...new Set(batch.map((r) => r.path).filter(Boolean))].slice(0, 7);
    const captured = batch.filter((r) => !r.path || paths.includes(r.path));
    const itemsText = captured
      .map(
        (row, idx) =>
          `${idx + 1}. ${row.title} (${row.path}${row.line ? `:${row.line}` : ''})\n   Cause: ${row.cause}\n   Proposed solution: ${row.solution}`,
      )
      .join('\n\n');
    const seed = workflowSeed({
      finding_targets: captured.map((r) => r.target),
      title: `Batch fix (${captured.length} issues)`,
      paths,
      kind: 'fix',
      message: `Address these ${captured.length} findings across the selected files:\n\n${itemsText}\n\nExplain how each changed file addresses the findings. Preserve unrelated behavior.`,
      acceptance_criteria: [...new Set(captured.flatMap((r) => r.acceptance_criteria))],
    });
    setSelectedIssueIds([]);
    setCmdOutput(`Dispatched grouped agent fix for ${captured.length} issues`);
    void w.startAgentFix(seed);
  };

  const dispatchCustomMission = () => {
    const rawPaths = missionPaths
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean)
      .slice(0, 8);
    if (!rawPaths.length || !missionPrompt.trim()) {
      w.fail('Specify at least one target file path and a clean-code objective.');
      return;
    }
    const seed = workflowSeed({
      finding_targets: agentS.changeSeed?.finding_targets,
      title: missionTitle.trim() || 'Clean-code agent mission',
      kind: 'fix',
      paths: rawPaths,
      message: missionPrompt.trim(),
      acceptance_criteria: [
        'Keep the diff minimal and limited to the requested behavior.',
        'Ensure all unit tests and static checks pass.',
      ],
    });
    setShowNewMissionForm(false);
    setCmdOutput(`Dispatched custom agent mission on ${seed.paths.join(', ')}`);
    void w.startAgentFix(seed);
  };

  const cyclePalette = () => {
    const idx = PALETTES.findIndex((p) => p.id === palette);
    const next = PALETTES[(idx + 1) % PALETTES.length];
    setPalette(next.id);
    setCmdOutput(`Switched TUI palette to ${next.label}`);
  };

  const activeInstructionDraft =
    s.instructionDraft?.content ||
    tuiCustomDraft ||
    s.projectInstructions?.existing_content ||
    s.instructionPreview?.existing_content ||
    '';

  const graduateTuiLesson = (ruleText: string, lessonId: string) => {
    const bullet = ruleText.startsWith('- ') ? ruleText : `- ${ruleText}`;
    const targetPath = s.instructionDraft?.path || s.instructionPreview?.path || 'AGENTS.md';
    const baseText = activeInstructionDraft || '# Project instructions';
    const nextDraft = containsGuideline(baseText, bullet)
      ? baseText
      : insertGuidelineSection(baseText, 'Harness² Lessons & Guardrails', bullet);
    setTuiCustomDraft(nextDraft);
    w.set({ instructionDraft: { path: targetPath, content: nextDraft } });
    setGraduatedTuiLessons((prev) => (prev.includes(lessonId) ? prev : [...prev, lessonId]));
    setCmdOutput(`Graduated Harness² lesson into AGENTS.md: ${ruleText}`);
  };

  const activateCurrentRow = () => {
    if (currentTabPage === 'summary' || currentTabPage === 'bugs') {
      if (!selectedIssue) return;
      if (selectedIssue.agent) {
        void w.selectAgent(selectedIssue.agent.path);
      } else if (selectedIssue.status !== 'done') {
        dispatchIssueToAgent(selectedIssue);
      } else {
        void w.openFile(selectedIssue.path, selectedIssue.symbol);
      }
      return;
    }
    if (currentTabPage === 'editor') {
      const file = editorFiles[clampedIdx];
      if (file) {
        setCmdOutput(`Opened ${file.path}`);
        void w.openFile(file.path);
      }
      return;
    }
    if (currentTabPage === 'features') {
      const idea = featureSuggestions[clampedIdx];
      if (idea) {
        setCmdOutput(`Dispatched feature idea "${idea.title}" to isolated Agent`);
        void w.discussFeature(idea);
      }
      return;
    }
    if (currentTabPage === 'agents' && !s.agentDetailOpen && !showNewMissionForm) {
      const item = agentItems[clampedIdx];
      if (item) {
        setCmdOutput(`Opened agent ${item.title} (${item.branch})`);
        void w.selectAgent(item.path);
      }
      return;
    }
    if (currentTabPage === 'analysis-run') {
      const sec = runSections[clampedIdx];
      if (sec) {
        void w.navigate(sec.category as Page);
      }
      return;
    }
    if (currentTabPage === 'git') {
      const wt = gitWorktrees[clampedIdx];
      if (wt && !wt.current) {
        void w.selectAgent(wt.path);
      }
      return;
    }
    if (currentTabPage === 'instructions') {
      const preset = modelPresets[clampedIdx];
      if (preset) {
        w.applyModelPreset(preset);
        setCmdOutput(`Applied model preset: ${preset.name}`);
      }
    }
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement | null)?.tagName?.toLowerCase();
      if (tag === 'input' || tag === 'textarea' || tag === 'select') {
        if (e.key === 'Escape') {
          e.preventDefault();
          (e.target as HTMLElement).blur();
        }
        return;
      }
      if (e.metaKey || e.ctrlKey || e.altKey) return;

      if (showKeybindsModal) {
        if (e.key === 'Escape' || e.key === '?' || e.key === 'Enter' || e.key === 'q') {
          e.preventDefault();
          setShowKeybindsModal(false);
        }
        return;
      }

      if (e.key === '?') {
        e.preventDefault();
        setShowKeybindsModal((prev) => !prev);
        return;
      }

      if (e.key === 'Escape' || e.key === 'Backspace') {
        if (showNewMissionForm) {
          e.preventDefault();
          setShowNewMissionForm(false);
          return;
        }
        if (currentTabPage === 'agents' && s.agentDetailOpen) {
          e.preventDefault();
          w.openAgentsDashboard();
          setCmdOutput('Returned to Agents in Progress Dashboard');
          return;
        }
      }

      const tab = TUI_TABS.find((t) => t.key === e.key);
      if (tab) {
        e.preventDefault();
        void w.navigate(tab.page);
        setCmdOutput(`Switched to [${tab.key}] ${tab.label}`);
        return;
      }

      if (e.key === 'Tab') {
        if (activeHubEntry.subPages.length > 0) {
          e.preventDefault();
          const curSubIdx = Math.max(
            0,
            activeHubEntry.subPages.findIndex((sp) => sp.page === currentTabPage),
          );
          const nextSub = activeHubEntry.subPages[(curSubIdx + 1) % activeHubEntry.subPages.length];
          if (nextSub) {
            setSelectedIdx(0);
            void w.navigate(nextSub.page);
            setCmdOutput(`Sub-view: ${nextSub.label}`);
          }
          return;
        }
        if (currentTabPage === 'instructions') {
          e.preventDefault();
          const sections: ('trust' | 'models' | 'bootstrap' | 'lessons')[] = [
            'trust',
            'models',
            'bootstrap',
            'lessons',
          ];
          const cur = sections.indexOf(rulesSection);
          const next = sections[(cur + 1) % sections.length];
          setRulesSection(next);
          setSelectedIdx(0);
          return;
        }
      }

      if (e.key === '[' || e.key === ']') {
        e.preventDefault();
        const curIdx = Math.max(
          0,
          TUI_TABS.findIndex((t) => t.key === activeHubEntry.key),
        );
        const nextIdx =
          e.key === ']'
            ? (curIdx + 1) % TUI_TABS.length
            : (curIdx - 1 + TUI_TABS.length) % TUI_TABS.length;
        const nextTab = TUI_TABS[nextIdx];
        if (nextTab) {
          void w.navigate(nextTab.page);
          setCmdOutput(`Switched to [${nextTab.key}] ${nextTab.label}`);
        }
        return;
      }

      if (e.key === '/' || e.key === ':') {
        e.preventDefault();
        cmdRef.current?.focus();
      } else if (e.key === 'j' || e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedIdx((i) => Math.min(Math.max(0, activeListLength - 1), i + 1));
      } else if (e.key === 'k' || e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedIdx((i) => Math.max(0, i - 1));
      } else if (
        (e.key === 'h' || e.key === 'ArrowLeft' || e.key === 'l' || e.key === 'ArrowRight') &&
        currentTabPage === 'bugs'
      ) {
        e.preventDefault();
        const views: ('open' | 'in_progress' | 'done')[] = ['open', 'in_progress', 'done'];
        const cur = views.indexOf(issueView);
        const next =
          e.key === 'l' || e.key === 'ArrowRight'
            ? views[(cur + 1) % views.length]
            : views[(cur - 1 + views.length) % views.length];
        setIssueView(next);
        setSelectedIdx(0);
      } else if (
        (e.key === 'h' || e.key === 'ArrowLeft' || e.key === 'l' || e.key === 'ArrowRight') &&
        currentTabPage === 'docs'
      ) {
        e.preventDefault();
        const sections: TuiDocsSection[] = ['context', 'components', 'scenarios', 'decisions'];
        const cur = sections.indexOf(docsSection);
        const next =
          e.key === 'l' || e.key === 'ArrowRight'
            ? sections[(cur + 1) % sections.length]
            : sections[(cur - 1 + sections.length) % sections.length];
        setDocsSection(next);
        setSelectedIdx(0);
      } else if (
        (e.key === 'h' || e.key === 'ArrowLeft' || e.key === 'l' || e.key === 'ArrowRight') &&
        currentTabPage === 'instructions'
      ) {
        e.preventDefault();
        const sections: ('trust' | 'models' | 'bootstrap' | 'lessons')[] = [
          'trust',
          'models',
          'bootstrap',
          'lessons',
        ];
        const cur = sections.indexOf(rulesSection);
        const next =
          e.key === 'l' || e.key === 'ArrowRight'
            ? sections[(cur + 1) % sections.length]
            : sections[(cur - 1 + sections.length) % sections.length];
        setRulesSection(next);
        setSelectedIdx(0);
      } else if (e.key === 'Enter') {
        e.preventDefault();
        activateCurrentRow();
      } else if (
        (e.key === 'x' || e.key === ' ') &&
        currentTabPage === 'bugs' &&
        selectedIssue &&
        selectedIssue.status !== 'done'
      ) {
        e.preventDefault();
        toggleIssueSelection(selectedIssue.id);
      } else if (e.key === 'd') {
        if (currentTabPage === 'bugs' && selectedIssue && selectedIssue.status !== 'done') {
          e.preventDefault();
          setCmdOutput(`Dismissed finding: ${selectedIssue.name}`);
          void w.triage(selectedIssue.target, 'dismissed');
        } else if (currentTabPage === 'features') {
          const idea = featureSuggestions[clampedIdx];
          if (idea) {
            e.preventDefault();
            setCmdOutput(`Dismissed feature idea: ${idea.title}`);
            void w.setFeatureStatus(idea.id, 'dismissed');
          }
        }
      } else if (e.key === 'a') {
        e.preventDefault();
        const chosen = openIssues.filter((i) => selectedIssueIds.includes(i.id));
        if (chosen.length > 0) {
          dispatchBatchIssuesToAgent(chosen);
        } else if (selectedIssue) {
          if (selectedIssue.agent) {
            void w.selectAgent(selectedIssue.agent.path);
          } else if (selectedIssue.status !== 'done') {
            dispatchIssueToAgent(selectedIssue);
          }
        }
      } else if (e.key === 'n') {
        e.preventDefault();
        setShowNewMissionForm(true);
        void w.navigate('agents');
        setCmdOutput('Opened Dispatch Precision Agent Mission form');
      } else if (e.key === 'c') {
        if (change && change.state !== 'applied' && allChecksPassed) {
          e.preventDefault();
          setCmdOutput('Applying and committing verified proposal...');
          void agentW.applyProposalNow();
        }
      } else if (e.key === 'm') {
        if (currentTabPage === 'git') {
          const wt = gitWorktrees[clampedIdx];
          if (wt && !wt.current && wt.mergeable) {
            e.preventDefault();
            void w.mergeAgentWorktree(wt.path);
          }
        } else if (selectedChild?.worktreePath) {
          e.preventDefault();
          void w.mergeAgentWorktree(selectedChild.worktreePath);
        }
      } else if (e.key === 'r') {
        e.preventDefault();
        if (currentTabPage === 'agents' && s.agentDetailOpen && change) {
          if (!allChecksPassed && checks.length > 0) {
            setCmdOutput('Triggering agent self-repair...');
            void agentW.repairChange();
          } else {
            setCmdOutput('Re-running deterministic verification checks...');
            void agentW.checkChange(true);
          }
        } else if (s.run && ['paused', 'interrupted'].includes(s.run.status)) {
          setCmdOutput('Resuming project analysis...');
          void w.resumeAnalysisDirect();
        } else {
          setCmdOutput('Starting project analysis...');
          void w.runAnalysisDirect(false);
        }
      } else if (e.key === 'e' && currentTabPage === 'editor') {
        e.preventDefault();
        setShowTuiInlineWhy((prev) => {
          const next = !prev;
          setCmdOutput(`Inline diff explanations ${next ? 'enabled' : 'hidden'}`);
          return next;
        });
      } else if (e.key === 'b' && currentTabPage === 'instructions') {
        e.preventDefault();
        setCmdOutput('Bootstrapping missing repository workflow pillars...');
        void w.bootstrapMissingPillars();
      } else if (e.key === 't') {
        e.preventDefault();
        if (currentTabPage === 'instructions') {
          void w.setWorkspaceTrust(s.workspaceTrust === false);
          setCmdOutput(`Workspace trust ${s.workspaceTrust === false ? 'granted' : 'revoked'}`);
        } else {
          setCmdOutput('Running project tests and checks...');
          void w.scanProject(false);
        }
      } else if (e.key === 'u') {
        e.preventDefault();
        setCmdOutput('Requesting guarded Undo...');
        void agentW.undoChange();
      } else if (e.key === 'p') {
        e.preventDefault();
        cyclePalette();
      } else if (e.key === 'g') {
        e.preventDefault();
        if (currentTabPage === 'instructions') {
          setRulesSection('lessons');
          graduateTuiLesson(
            'Propagate context.Context cancellation and add a regression test for every check repair.',
            'lesson-default-1',
          );
        } else {
          onExitTui();
        }
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [
    activeHubEntry,
    activeListLength,
    clampedIdx,
    currentTabPage,
    docsSection,
    featureSuggestions,
    issueView,
    openIssues,
    rulesSection,
    selectedIssue,
    selectedIssueIds,
    showKeybindsModal,
    showNewMissionForm,
    s.agentDetailOpen,
    s.run,
    s.workspaceTrust,
    change,
    allChecksPassed,
    checks.length,
    palette,
  ]);

  const executeCommand = (raw: string) => {
    const trimmed = raw.trim().replace(/^:/, '');
    if (!trimmed) return;
    setCmdHistory((prev) =>
      [raw.trim(), ...prev.filter((item) => item !== raw.trim())].slice(0, 30),
    );
    setCmdHistoryIdx(-1);
    const [verb, ...rest] = trimmed.split(/\s+/);
    const arg = rest.join(' ');
    const cmd = verb.toLowerCase();
    if (cmd === 'gui' || cmd === 'exit' || cmd === 'q' || cmd === 'quit') {
      onExitTui();
      return;
    }
    if (
      cmd === 'help' ||
      cmd === '?' ||
      cmd === 'keys' ||
      cmd === 'keybinds' ||
      cmd === 'keybind' ||
      cmd === 'shortcuts'
    ) {
      setShowKeybindsModal(true);
      setCmdOutput(
        'Opened Keybindings Reference ([?] or Esc to close). Commands: keys | analyze | test | fix | bootstrap | dismiss | agent <prompt> | open <file> | project <path> | apply | merge | undo | palette | gui',
      );
      return;
    }
    if (cmd === 'bootstrap') {
      setCmdOutput('Bootstrapping missing repository workflow pillars...');
      void w.bootstrapMissingPillars();
      return;
    }
    if (cmd === 'palette' || cmd === 'theme') {
      cyclePalette();
      return;
    }
    if (cmd === 'trust') {
      void w.setWorkspaceTrust(s.workspaceTrust === false);
      setCmdOutput(`Workspace trust ${s.workspaceTrust === false ? 'granted' : 'revoked'}.`);
      return;
    }
    if (cmd === 'project') {
      if (arg) {
        setCmdOutput(`Opening project at ${arg}...`);
        void w.openProject(arg, false);
      } else {
        setCmdOutput('Choosing project directory...');
        void w.chooseProject();
      }
      return;
    }
    if (cmd === 'reindex') {
      setCmdOutput('Reindexing repository...');
      void w.reindex();
      return;
    }
    if (cmd === 'analyze' || cmd === 'run') {
      setCmdOutput('Starting project analysis...');
      void w.runAnalysisDirect(false);
      return;
    }
    if (cmd === 'new-analysis' || cmd === 'reanalyze') {
      setCmdOutput('Starting fresh project analysis (preserving in-progress issues)...');
      void w.runAnalysisDirect(true);
      return;
    }
    if (cmd === 'resume') {
      setCmdOutput('Resuming paused analysis...');
      void w.resumeAnalysisDirect();
      return;
    }
    if (cmd === 'repair' || cmd === 'retry') {
      setCmdOutput('Retrying failed/stale analysis files...');
      void w.repairAnalysisDirect(arg || undefined);
      return;
    }
    if (cmd === 'pause') {
      setCmdOutput('Pausing active analysis...');
      void w.controlRun('pause');
      return;
    }
    if (cmd === 'test' || cmd === 'scan') {
      setCmdOutput('Running project tests and checks...');
      void w.scanProject(false);
      return;
    }
    if (cmd === 'dismiss' && selectedIssue && selectedIssue.status === 'open') {
      setCmdOutput(`Dismissing finding: ${selectedIssue.name}`);
      void w.triage(selectedIssue.target, 'dismissed');
      return;
    }
    if (cmd === 'fix') {
      const chosen = openIssues.filter((i) => selectedIssueIds.includes(i.id));
      if (chosen.length > 0) {
        dispatchBatchIssuesToAgent(chosen);
      } else if (selectedIssue && selectedIssue.status !== 'done') {
        dispatchIssueToAgent(selectedIssue);
      } else {
        void w.navigate('agents');
        setCmdOutput('Opened Agents in Progress Dashboard.');
      }
      return;
    }
    if (cmd === 'agent' || cmd === 'agents') {
      if (arg) {
        setMissionPrompt(arg);
        setShowNewMissionForm(true);
      }
      void w.navigate('agents');
      setCmdOutput('Switched to [2] Agent Studio.');
      return;
    }
    if (cmd === 'issues' || cmd === 'bugs') {
      void w.navigate('bugs');
      setCmdOutput('Switched to [2] Agent Studio · Issues.');
      return;
    }
    if (cmd === 'arch' || cmd === 'diagrams') {
      void w.navigate('diagrams');
      setCmdOutput('Switched to [4] Handbook & Arch · Architecture.');
      return;
    }
    if (cmd === 'docs' || cmd === 'handbook') {
      void w.navigate('docs');
      setCmdOutput('Switched to [4] Handbook & Arch · Handbook / Docs.');
      return;
    }
    if (cmd === 'git') {
      void w.navigate('git');
      setCmdOutput('Switched to [3] Code & Diff · Git & Worktrees.');
      return;
    }
    if (cmd === 'open' && arg) {
      setCmdOutput(`Opening ${arg} in Code & Diff...`);
      void w.openFile(arg);
      return;
    }
    if (cmd === 'apply' || cmd === 'commit') {
      setCmdOutput('Applying verified proposal...');
      void agentW.applyProposalNow();
      return;
    }
    if (cmd === 'repair') {
      setCmdOutput('Triggering agent self-repair...');
      void agentW.repairChange();
      return;
    }
    if (cmd === 'merge' && selectedChild?.worktreePath) {
      setCmdOutput(
        `Merging ${selectedChild.worktreePath} into ${s.repository?.branch || 'main'}...`,
      );
      void w.mergeAgentWorktree(selectedChild.worktreePath);
      return;
    }
    if (cmd === 'undo') {
      setCmdOutput('Requesting guarded Undo...');
      void agentW.undoChange();
      return;
    }
    const tabMatch = TUI_TABS.find(
      (t) => t.key === cmd || t.label.toLowerCase().includes(cmd) || t.page === cmd,
    );
    if (tabMatch) {
      void w.navigate(tabMatch.page);
      setCmdOutput(`Switched to [${tabMatch.key}] ${tabMatch.label}.`);
      return;
    }
    setCmdOutput(`Unknown command "${trimmed}". Type ":keys" or "help" for available commands.`);
  };

  // Compute diff lines & precision metrics
  const changedFiles = change?.changes || [];
  let addedLines = 0;
  let removedLines = 0;
  for (const fc of changedFiles) {
    for (const line of fc.diff?.lines || []) {
      if (isAddedLine(line.kind)) addedLines++;
      else if (isRemovedLine(line.kind)) removedLines++;
    }
  }
  const totalDelta = addedLines + removedLines;
  const scopeBadge =
    totalDelta === 0
      ? 'IDLE'
      : totalDelta <= 60
        ? 'SURGICAL'
        : totalDelta <= 200
          ? 'FOCUSED'
          : 'BROAD';

  const activeError = s.error || agentS.error;
  const activeBusy = s.busy || agentS.busy;
  const activeNotice = s.notice || agentS.notice;

  return (
    <div
      className="tui-root"
      data-tui-palette={palette}
      role="application"
      aria-label="Mini-Orca Terminal UI"
    >
      {/* Top Status Bar */}
      <header className="tui-statusbar">
        <div className="tui-statusbar-left">
          <span className="tui-brand">MINI-ORCA // TUI</span>
          <span className="tui-pill">
            <span className={`tui-dot${s.connected ? '' : ' offline'}`} />
            <strong>{s.project?.name || 'no-project'}</strong>
          </span>
          <span className="tui-pill">
            git: <strong>{s.repository?.branch || 'main'}</strong>
            {s.gitWorkspace?.dirty ? <span className="tui-c-amber">*dirty</span> : null}
          </span>
          <span className="tui-pill">
            trust:{' '}
            <strong className={s.workspaceTrust !== false ? 'tui-c-green' : 'tui-c-amber'}>
              {s.workspaceTrust !== false ? 'TRUSTED' : 'UNTRUSTED'}
            </strong>
          </span>
          <span className="tui-pill">
            preset:{' '}
            <strong className="tui-c-purple">
              {allModelPresets(s).find((p) => isModelPresetActive(s, p))?.name || 'Custom'}
            </strong>
          </span>
        </div>
        <div className="tui-statusbar-right">
          <button
            type="button"
            className="tui-btn"
            disabled={!!activeBusy}
            onClick={() => void w.reindex()}
          >
            [Reindex]
          </button>
          <button
            type="button"
            className={`tui-btn${showKeybindsModal ? ' active' : ''}`}
            aria-label="Open TUI keybindings reference"
            onClick={() => setShowKeybindsModal((prev) => !prev)}
          >
            [?] Keys
          </button>
          <div role="group" aria-label="TUI Palette" style={{ display: 'flex', gap: '4px' }}>
            {PALETTES.map((p) => (
              <button
                key={p.id}
                type="button"
                className={`tui-btn${palette === p.id ? ' active' : ''}`}
                aria-pressed={palette === p.id}
                onClick={() => setPalette(p.id)}
              >
                {p.label}
              </button>
            ))}
          </div>
          <button
            type="button"
            className="tui-btn primary"
            aria-label="Switch to GUI mode"
            onClick={onExitTui}
          >
            [GUI Mode]
          </button>
        </div>
      </header>

      {/* Single-Row Numbered 5-Hub Top Bar + Inline Sub-Views */}
      <nav
        className="tui-tabbar"
        aria-label="TUI Workspaces"
        style={{ justifyContent: 'space-between', flexWrap: 'wrap', gap: '6px' }}
      >
        <div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap' }}>
          {TUI_TABS.map((t) => {
            const isCurrent = activeHubEntry.key === t.key;
            const badge =
              t.key === '2' &&
              (workflow ||
                agentItems.length > 0 ||
                issueSummary.totalOpenIssues > 0 ||
                issueSummary.featureCount > 0)
                ? workflow
                  ? `[${workflow.status}]`
                  : `(${agentItems.length}a/${issueSummary.totalOpenIssues}i/${issueSummary.featureCount}f)`
                : t.key === '3' && changedFiles.length > 0
                  ? `(${changedFiles.length}d)`
                  : '';
            return (
              <button
                key={t.key}
                type="button"
                className="tui-tab"
                aria-current={isCurrent ? 'page' : undefined}
                onClick={() => void w.navigate(t.page)}
              >
                <span className="tui-key">[{t.key}]</span>
                <span>{t.label}</span>
                {badge && <span className="tui-c-amber">{badge}</span>}
              </button>
            );
          })}
        </div>
        <div
          aria-label="TUI Hub Sub-Views"
          style={{
            display: 'flex',
            gap: '6px',
            flexWrap: 'wrap',
            alignItems: 'center',
            paddingRight: '8px',
          }}
        >
          {(activeHubEntry.subPages.length > 0
            ? activeHubEntry.subPages
            : [
                { page: 'bugs' as Page, label: 'Issues' },
                { page: 'features' as Page, label: 'Features' },
                { page: 'analysis-run' as Page, label: 'Last Run' },
              ]
          ).map((sp) => (
            <button
              key={sp.page}
              type="button"
              className={`tui-btn${currentTabPage === sp.page ? ' primary' : ''}`}
              aria-current={currentTabPage === sp.page ? 'page' : undefined}
              onClick={() => void w.navigate(sp.page)}
            >
              [{sp.label}]
            </button>
          ))}
          {activeHubEntry.subPages.length > 0 && <span className="tui-sub">[Tab]</span>}
        </div>
      </nav>

      {/* Live Analysis Progress Banner (visible across all TUI screens when running or paused) */}
      {s.run &&
        (activeRun(s.run) || ['paused', 'interrupted'].includes(s.run.status)) &&
        (() => {
          const runTotal = s.run.sections.reduce((sum, sec) => sum + sec.coverage.total, 0);
          const runDone = s.run.sections.reduce(
            (sum, sec) =>
              sum +
              sec.coverage.succeeded +
              sec.coverage.partial +
              sec.coverage.failed +
              sec.coverage.skipped +
              sec.coverage.unavailable,
            0,
          );
          const pct = runTotal > 0 ? Math.floor((runDone / runTotal) * 100) : 0;
          const barWidth = 16;
          const filled = Math.round((pct / 100) * barWidth);
          const asciiBar = `${'█'.repeat(Math.max(0, Math.min(barWidth, filled)))}${'░'.repeat(Math.max(0, barWidth - filled))}`;
          const runningFile = s.run.files.find((f) =>
            f.stages.some((st) => st.status === 'running'),
          );
          const runningStage = runningFile?.stages.find((st) => st.status === 'running');
          const stepLabel =
            s.run.status === 'canceling'
              ? 'Canceling analysis…'
              : s.run.status === 'pausing'
                ? 'Finishing current step before pausing…'
                : runningStage && runningFile
                  ? `${runningStage.stage} · ${runningFile.path}`
                  : s.run.features?.status === 'running'
                    ? 'Generating feature suggestions…'
                    : s.run.status === 'queued'
                      ? 'Queued…'
                      : ['paused', 'interrupted'].includes(s.run.status)
                        ? s.run.reason || `Analysis ${s.run.status}`
                        : `${s.run.window_files_completed} files completed this batch`;
          return (
            <div
              role="status"
              aria-label="TUI live analysis progress"
              style={{
                padding: '6px 12px',
                borderBottom: '1px solid var(--tui-accent)',
                background: 'var(--tui-bg-alt)',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                gap: '12px',
                flexWrap: 'wrap',
              }}
            >
              <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
                {activeRun(s.run) ? (
                  <span className="tui-c-accent" style={{ fontWeight: 700 }}>
                    <Spinner /> [ANALYZING]
                  </span>
                ) : (
                  <span className="tui-c-amber" style={{ fontWeight: 700 }}>
                    [ANALYSIS {s.run.status.toUpperCase()}]
                  </span>
                )}
                <span className="tui-c-green" style={{ fontFamily: 'var(--font-mono)' }}>
                  [{asciiBar}] {pct}% ({runDone}/{runTotal} steps)
                </span>
                <span className="tui-sub">{stepLabel}</span>
                <span className="tui-c-amber">
                  {issueSummary.totalOpenIssues} issues ({issueSummary.bugCount}b/
                  {issueSummary.secCount}s/{issueSummary.perfCount}p) · {issueSummary.featureCount}{' '}
                  features · {s.run.elapsed_seconds}s
                </span>
              </div>
              <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                {['running', 'queued'].includes(s.run.status) && (
                  <button
                    type="button"
                    className="tui-btn"
                    disabled={!!activeBusy}
                    onClick={() => void w.controlRun('pause')}
                  >
                    [Pause]
                  </button>
                )}
                {['paused', 'interrupted'].includes(s.run.status) && (
                  <button
                    type="button"
                    className="tui-btn primary"
                    disabled={!!activeBusy}
                    onClick={() => void w.resumeAnalysisDirect()}
                  >
                    [Resume Analysis]
                  </button>
                )}
                <button
                  type="button"
                  className="tui-btn"
                  disabled={!!activeBusy || s.run.status === 'canceling'}
                  onClick={() => void w.controlRun('cancel')}
                >
                  [Cancel]
                </button>
                {currentTabPage !== 'analysis-run' && (
                  <button
                    type="button"
                    className="tui-btn"
                    onClick={() => void w.navigate('analysis-run')}
                  >
                    [Last Run]
                  </button>
                )}
              </div>
            </div>
          );
        })()}

      {/* Notices / Errors (Root + Active Child Worktree) */}
      {(activeError || activeNotice || activeBusy) && (
        <div
          style={{
            padding: '4px 12px',
            borderBottom: '1px solid var(--tui-border)',
            background: 'var(--tui-bg-alt)',
            display: 'flex',
            justifyContent: 'space-between',
            gap: '12px',
          }}
        >
          <span>
            {activeError ? (
              <span className="tui-c-red">[ERROR] {activeError}</span>
            ) : activeBusy ? (
              <span className="tui-c-accent">[BUSY] {activeBusy}…</span>
            ) : classifyNoticeTone(activeNotice) === 'warning' ? (
              <span className="tui-c-amber">[WARN] {activeNotice}</span>
            ) : classifyNoticeTone(activeNotice) === 'info' ? (
              <span className="tui-c-accent">[INFO] {activeNotice}</span>
            ) : (
              <span className="tui-c-green">[OK] {activeNotice}</span>
            )}
          </span>
          {activeBusy && (
            <button
              type="button"
              className="tui-btn"
              onClick={() => void (s.busy ? w : agentW).cancel()}
            >
              [Cancel]
            </button>
          )}
        </div>
      )}

      {/* Main TUI Workspace */}
      <main className="tui-workspace">
        {/* [1] OVERVIEW */}
        {currentTabPage === 'summary' && (
          <>
            <div className="tui-grid-4">
              <div className="tui-box">
                <span className="tui-box-title">SOURCE FILES</span>
                <span className="tui-box-meta">INDEXED</span>
                <div className="tui-stat">{s.project?.source_file_count ?? '—'}</div>
                <div className="tui-sub">
                  {s.overview?.docs?.components?.length ||
                    s.overview?.analysis?.components?.length ||
                    0}{' '}
                  components mapped
                </div>
              </div>
              <div className="tui-box">
                <span className="tui-box-title">OPEN ISSUES</span>
                <span className="tui-box-meta">[2] STUDIO</span>
                <div className="tui-stat tui-c-amber">{issueSummary.totalOpenIssues}</div>
                <div className="tui-sub">
                  {issueSummary.bugCount}b · {issueSummary.secCount}s · {issueSummary.perfCount}p ·{' '}
                  {issueSummary.featureCount} feature ideas
                </div>
              </div>
              <div className="tui-box">
                <span className="tui-box-title">TESTS &amp; CHECKS</span>
                <span className="tui-box-meta">[t] GATE</span>
                <div className="tui-stat tui-c-green">
                  {s.scan ? human(s.scan.status) : 'Ready'}
                </div>
                <div className="tui-sub" style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                  <button
                    type="button"
                    className="tui-btn"
                    disabled={!!s.busy}
                    onClick={() => void w.scanProject(false)}
                  >
                    [Run Tests]
                  </button>
                  <button
                    type="button"
                    className="tui-btn"
                    disabled={!!s.busy}
                    onClick={() => void w.openRepoBootstrap()}
                  >
                    [Repo Readiness ({evaluateBootstrapChecklist(s).filter((p) => p.ready).length}
                    /4)]
                  </button>
                </div>
              </div>
              <div className="tui-box focused">
                <span className="tui-box-title">AGENT PIPELINE</span>
                <span className="tui-box-meta">[2] AGENTS</span>
                <div className="tui-stat tui-c-purple">
                  {workflow ? human(workflow.status) : change ? 'Staged' : 'Idle'}
                </div>
                <div className="tui-sub">
                  <button
                    type="button"
                    className="tui-btn primary"
                    onClick={() => void w.navigate('agents')}
                  >
                    [Open Agent Mission Control]
                  </button>
                </div>
              </div>
            </div>

            <div className="tui-split-main">
              <div className="tui-box focused">
                <span className="tui-box-title">
                  PRIORITY FINDINGS (j/k move · Enter/a = Agent Fix)
                </span>
                <span className="tui-box-meta">{issueSummary.totalOpenIssues} open</span>
                {overviewIssues.length === 0 ? (
                  <p className="tui-sub">
                    No open issues. Press <code>r</code> or type <code>analyze</code> to scan.
                  </p>
                ) : (
                  <table className="tui-table">
                    <thead>
                      <tr>
                        <th>SEV</th>
                        <th>FINDING</th>
                        <th>LOCATION</th>
                        <th>AGENT</th>
                      </tr>
                    </thead>
                    <tbody>
                      {overviewIssues.map((iss, idx) => (
                        <tr
                          key={iss.id}
                          className={`tui-selectable${idx === clampedIdx ? ' tui-selected' : ''}`}
                          onClick={() => setSelectedIdx(idx)}
                        >
                          <td>
                            {idx === clampedIdx && (
                              <span className="tui-cursor-indicator" aria-hidden="true">
                                &gt;
                              </span>
                            )}
                            <span
                              className={`tui-badge ${
                                ['critical', 'high'].includes(iss.severity)
                                  ? 'tui-c-red'
                                  : iss.severity === 'medium'
                                    ? 'tui-c-amber'
                                    : 'tui-c-accent'
                              }`}
                            >
                              {iss.severity}
                            </span>
                          </td>
                          <td>{iss.name}</td>
                          <td className="tui-c-muted">
                            {iss.path}
                            {iss.line ? `:${iss.line}` : ''}
                          </td>
                          <td>
                            <button
                              type="button"
                              className="tui-btn primary"
                              disabled={!!s.busy}
                              onClick={(e) => {
                                e.stopPropagation();
                                dispatchIssueToAgent(iss);
                              }}
                            >
                              [Fix]
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>

              <div className="tui-box">
                <span className="tui-box-title">QUICK ACTIONS &amp; TOPOLOGY</span>
                <span className="tui-box-meta">{s.project?.name || 'repo'}</span>
                <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', marginBottom: '8px' }}>
                  <button
                    type="button"
                    className="tui-btn primary"
                    disabled={!!s.busy || activeRun(s.run)}
                    onClick={() => void w.runAnalysisDirect(false)}
                  >
                    [r: Run Analysis]
                  </button>
                  <button
                    type="button"
                    className="tui-btn"
                    disabled={!!s.busy}
                    onClick={() => void w.scanProject(false)}
                  >
                    [t: Run Tests]
                  </button>
                  <button
                    type="button"
                    className="tui-btn"
                    onClick={() => void w.navigate('agents')}
                  >
                    [2: Agent Mission Control]
                  </button>
                  <button
                    type="button"
                    className="tui-btn"
                    onClick={() => setShowKeybindsModal(true)}
                  >
                    [?: Keybindings]
                  </button>
                </div>
                <pre className="tui-pre">
                  {`┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  1. CREATE   │────►│   2. TEST    │────►│  3. CRITIC   │
│  ${models.create.padEnd(10).slice(0, 10)} │     │  ${models.test.padEnd(10).slice(0, 10)} │     │  ${models.review.padEnd(10).slice(0, 10)} │
└──────────────┘     └──────────────┘     └──────┬───────┘
                                                 ▼
                                        ┌────────────────┐
                                        │ 4. HUMAN GATE  │
                                        │ Explicit Apply │
                                        └────────────────┘`}
                </pre>
              </div>
            </div>
          </>
        )}

        {/* [5] ISSUES */}
        {currentTabPage === 'bugs' && (
          <div className="tui-split-main">
            <div className="tui-box focused">
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  flexWrap: 'wrap',
                  gap: '6px',
                  marginBottom: '8px',
                }}
              >
                <span className="tui-box-title">
                  ISSUES QUEUE (j/k move · h/l filter · x select · d dismiss · Enter/a fix)
                </span>
                <div
                  style={{ display: 'flex', gap: '4px', flexWrap: 'wrap', alignItems: 'center' }}
                >
                  <button
                    type="button"
                    className={`tui-btn${issueView === 'open' ? ' primary' : ''}`}
                    onClick={() => {
                      setIssueView('open');
                      setSelectedIdx(0);
                    }}
                  >
                    [Open ({openIssues.length})]
                  </button>
                  <button
                    type="button"
                    className={`tui-btn${issueView === 'in_progress' ? ' primary' : ''}`}
                    onClick={() => {
                      setIssueView('in_progress');
                      setSelectedIdx(0);
                    }}
                  >
                    [In Progress ({inProgressIssues.length})]
                  </button>
                  <button
                    type="button"
                    className={`tui-btn${issueView === 'done' ? ' primary' : ''}`}
                    onClick={() => {
                      setIssueView('done');
                      setSelectedIdx(0);
                    }}
                  >
                    [Done ({doneIssues.length})]
                  </button>
                  <span className="tui-sub" aria-hidden="true">
                    |
                  </span>
                  {(
                    [
                      ['all', `All (${issueSummary.totalOpenIssues})`],
                      ['bugs', `Bugs (${issueSummary.bugCount})`],
                      ['security', `Security (${issueSummary.secCount})`],
                      ['performance', `Performance (${issueSummary.perfCount})`],
                    ] as const
                  ).map(([cat, label]) => (
                    <button
                      key={cat}
                      type="button"
                      className={`tui-btn${issueCategory === cat ? ' active' : ''}`}
                      onClick={() => {
                        setIssueCategory(cat);
                        setSelectedIdx(0);
                      }}
                    >
                      [{label}]
                    </button>
                  ))}
                </div>
              </div>
              {selectedIssueIds.length > 0 && (
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    padding: '4px 8px',
                    marginBottom: '8px',
                    border: '1px solid var(--tui-accent)',
                  }}
                >
                  <span>
                    <strong>{selectedIssueIds.length}</strong> issue(s) selected for grouped agent
                    fix
                  </span>
                  <div style={{ display: 'flex', gap: '6px' }}>
                    <button
                      type="button"
                      className="tui-btn primary"
                      disabled={!!s.busy}
                      onClick={() =>
                        dispatchBatchIssuesToAgent(
                          openIssues.filter((i) => selectedIssueIds.includes(i.id)),
                        )
                      }
                    >
                      [Fix Selected ({selectedIssueIds.length})]
                    </button>
                    <button
                      type="button"
                      className="tui-btn"
                      onClick={() => setSelectedIssueIds([])}
                    >
                      [Clear]
                    </button>
                  </div>
                </div>
              )}
              {issues.length === 0 ? (
                <p className="tui-sub">
                  {issueView === 'in_progress'
                    ? 'No issues in progress. Select open issues and press [a] or click [Fix].'
                    : issueView === 'done'
                      ? 'No merged issues yet. Issues are marked Done when their agent branch is merged.'
                      : 'No open findings.'}
                </p>
              ) : (
                <table className="tui-table">
                  <thead>
                    <tr>
                      <th>SEL</th>
                      <th>SEV</th>
                      <th>TITLE</th>
                      <th>FILE</th>
                      <th>STEP / STATUS</th>
                      <th>ACTION</th>
                    </tr>
                  </thead>
                  <tbody>
                    {issues.map((iss, idx) => (
                      <tr
                        key={iss.id}
                        className={`tui-selectable${idx === clampedIdx ? ' tui-selected' : ''}`}
                        onClick={() => setSelectedIdx(idx)}
                      >
                        <td>
                          {idx === clampedIdx && (
                            <span className="tui-cursor-indicator" aria-hidden="true">
                              &gt;
                            </span>
                          )}
                          {iss.status !== 'done' ? (
                            <button
                              type="button"
                              className="tui-btn"
                              aria-label={`Select ${iss.name}`}
                              onClick={(e) => {
                                e.stopPropagation();
                                toggleIssueSelection(iss.id);
                              }}
                            >
                              [{selectedIssueIds.includes(iss.id) ? 'x' : ' '}]
                            </button>
                          ) : (
                            <span className="tui-c-green">[OK]</span>
                          )}
                        </td>
                        <td>
                          <span
                            className={`tui-badge ${
                              ['critical', 'high'].includes(iss.severity)
                                ? 'tui-c-red'
                                : iss.severity === 'medium'
                                  ? 'tui-c-amber'
                                  : 'tui-c-accent'
                            }`}
                          >
                            {iss.severity}
                          </span>
                        </td>
                        <td>{iss.name}</td>
                        <td className="tui-c-muted">
                          {iss.path}
                          {iss.line ? `:${iss.line}` : ''}
                        </td>
                        <td>
                          {iss.status === 'done' ? (
                            <span className="tui-badge tui-c-green">
                              {iss.commitOid ? `Done · #${iss.commitOid.slice(0, 7)}` : 'Done'}
                            </span>
                          ) : iss.agent ? (
                            <span style={{ display: 'inline-flex', gap: '4px', flexWrap: 'wrap' }}>
                              <span className="tui-badge tui-c-purple">{iss.agent.stageLabel}</span>
                              {iss.agent.hasProblem && (
                                <span className="tui-badge tui-c-red">PROBLEM</span>
                              )}
                            </span>
                          ) : (
                            <span className="tui-c-muted">open</span>
                          )}
                        </td>
                        <td style={{ display: 'flex', gap: '4px' }}>
                          {iss.agent ? (
                            <button
                              type="button"
                              className="tui-btn primary"
                              onClick={(e) => {
                                e.stopPropagation();
                                void w.selectAgent(iss.agent!.path);
                              }}
                            >
                              [Open Agent]
                            </button>
                          ) : iss.status !== 'done' ? (
                            <button
                              type="button"
                              className="tui-btn primary"
                              disabled={!!s.busy}
                              onClick={(e) => {
                                e.stopPropagation();
                                dispatchIssueToAgent(iss);
                              }}
                            >
                              [Fix]
                            </button>
                          ) : null}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>

            <div className="tui-box">
              <span className="tui-box-title">FINDING INSPECTOR</span>
              <span className="tui-box-meta">{selectedIssue?.path || 'none'}</span>
              {selectedIssue ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  <div>
                    <strong className="tui-c-accent">{selectedIssue.title}</strong>
                    <div className="tui-sub">
                      {selectedIssue.path}
                      {selectedIssue.line ? `:${selectedIssue.line}` : ''}{' '}
                      {selectedIssue.symbol ? `· ${selectedIssue.symbol}` : ''}
                    </div>
                  </div>
                  {selectedIssue.agent && (
                    <div
                      style={{
                        padding: '6px 8px',
                        border: `1px solid ${selectedIssue.agent.hasProblem ? 'var(--tui-red, #ff5f56)' : 'var(--tui-accent)'}`,
                      }}
                    >
                      <div>
                        <strong className="tui-c-purple">{selectedIssue.agent.branch}</strong> ·{' '}
                        <span>{selectedIssue.agent.stageLabel}</span>
                      </div>
                      {selectedIssue.agent.hasProblem && (
                        <div className="tui-c-red">
                          [PROBLEM] {selectedIssue.agent.problemSummary}
                        </div>
                      )}
                    </div>
                  )}
                  <div>
                    <div className="tui-c-amber">ROOT CAUSE:</div>
                    <pre className="tui-pre">{selectedIssue.cause}</pre>
                  </div>
                  <div>
                    <div className="tui-c-green">CLEAN-CODE REMEDIATION:</div>
                    <pre className="tui-pre">{selectedIssue.solution}</pre>
                  </div>
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    {selectedIssue.agent ? (
                      <button
                        type="button"
                        className="tui-btn primary"
                        onClick={() => void w.selectAgent(selectedIssue.agent!.path)}
                      >
                        [a] Open Agent Detail
                      </button>
                    ) : selectedIssue.status !== 'done' ? (
                      <button
                        type="button"
                        className="tui-btn primary"
                        disabled={!!s.busy}
                        onClick={() => dispatchIssueToAgent(selectedIssue)}
                      >
                        [a] Fix
                      </button>
                    ) : null}
                    <button
                      type="button"
                      className="tui-btn"
                      onClick={() => void w.openFile(selectedIssue.path, selectedIssue.symbol)}
                    >
                      [Inspect File]
                    </button>
                    {selectedIssue.status !== 'done' && (
                      <button
                        type="button"
                        className="tui-btn"
                        disabled={!!s.busy}
                        onClick={() => void w.triage(selectedIssue.target, 'dismissed')}
                      >
                        [d: Dismiss]
                      </button>
                    )}
                  </div>
                </div>
              ) : (
                <p className="tui-sub">Select a finding to inspect its cause and agent fix plan.</p>
              )}
            </div>
          </div>
        )}

        {/* [2] AGENT STUDIO — DASHBOARD vs AGENT & ISSUE DETAIL vs NEW MISSION */}
        {currentTabPage === 'agents' && !s.agentDetailOpen && !showNewMissionForm && (
          <div className="tui-box focused">
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: '8px',
              }}
            >
              <span className="tui-box-title">AGENTS IN PROGRESS DASHBOARD</span>
              <div style={{ display: 'flex', gap: '6px' }}>
                <button type="button" className="tui-btn" onClick={() => void w.navigate('bugs')}>
                  [Select Issues]
                </button>
                <button
                  type="button"
                  className="tui-btn primary"
                  onClick={() => setShowNewMissionForm(true)}
                >
                  [+ New Agent Fix]
                </button>
              </div>
            </div>
            {agentItems.length === 0 ? (
              <p className="tui-sub">
                No agents in progress. Select one or more issues in [Issues] and press [a], or press
                [n] for [+ New Agent Fix].
              </p>
            ) : (
              <table className="tui-table" aria-label="Agents in progress">
                <thead>
                  <tr>
                    <th>BUG(S)</th>
                    <th>4-STEP PIPELINE</th>
                    <th>DETAIL</th>
                  </tr>
                </thead>
                <tbody>
                  {agentItems.map((item, idx) => {
                    const bugTitles =
                      item.linkedIssues.length > 0
                        ? item.linkedIssues.map((iss) => iss.name).join(' · ')
                        : item.title;
                    return (
                      <tr
                        key={item.key}
                        className={`tui-selectable${idx === clampedIdx ? ' tui-selected' : ''}`}
                        onClick={() => {
                          setSelectedIdx(idx);
                          void w.selectAgent(item.path);
                        }}
                      >
                        <td>
                          {idx === clampedIdx && (
                            <span className="tui-cursor-indicator" aria-hidden="true">
                              &gt;
                            </span>
                          )}
                          <strong className="tui-c-accent">{bugTitles}</strong>
                          {item.hasProblem && (
                            <div className="tui-c-red">
                              PROBLEM: {item.problemSummary || 'Check failed'}
                            </div>
                          )}
                        </td>
                        <td>
                          <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                            {(
                              [
                                [1, 'Create Fix'],
                                [2, 'Verify & Test'],
                                [3, 'Critic Review'],
                                [4, 'Review & Commit'],
                              ] as const
                            ).map(([stepNum, shortTitle]) => {
                              const isDone =
                                item.stepNumber > stepNum ||
                                (item.stepNumber === 4 &&
                                  (item.mergeable || item.status === 'completed'));
                              const isCurrent = item.stepNumber === stepNum && !isDone;
                              const isRunningStep = isCurrent && item.status === 'running';
                              const isProblemStep = isCurrent && item.hasProblem;
                              return (
                                <span
                                  key={stepNum}
                                  className={`tui-badge ${
                                    isProblemStep
                                      ? 'tui-c-red'
                                      : isDone
                                        ? 'tui-c-green'
                                        : isCurrent
                                          ? 'tui-c-purple'
                                          : 'tui-c-muted'
                                  }`}
                                  style={{
                                    display: 'inline-flex',
                                    alignItems: 'center',
                                    gap: '4px',
                                  }}
                                >
                                  {isRunningStep ? (
                                    <Spinner />
                                  ) : isProblemStep ? (
                                    '[!] '
                                  ) : isDone ? (
                                    '[OK] '
                                  ) : (
                                    ''
                                  )}
                                  <span>
                                    {stepNum}. {shortTitle}
                                  </span>
                                </span>
                              );
                            })}
                          </div>
                        </td>
                        <td style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                          <button
                            type="button"
                            className="tui-btn primary"
                            onClick={(e) => {
                              e.stopPropagation();
                              void w.selectAgent(item.path);
                            }}
                          >
                            [Open Agent Detail]
                          </button>
                          {item.mergeable && item.path && (
                            <button
                              type="button"
                              className="tui-btn"
                              disabled={!!activeBusy}
                              onClick={(e) => {
                                e.stopPropagation();
                                void w.mergeAgentWorktree(item.path!);
                              }}
                            >
                              [Merge]
                            </button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        )}

        {currentTabPage === 'agents' && !s.agentDetailOpen && showNewMissionForm && (
          <div className="tui-box focused">
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: '8px',
              }}
            >
              <span className="tui-box-title">DISPATCH PRECISION AGENT MISSION</span>
              <button
                type="button"
                className="tui-btn"
                onClick={() => setShowNewMissionForm(false)}
              >
                [← Back to Agents Dashboard]
              </button>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <label>
                <div className="tui-sub">Mission Title</div>
                <input
                  aria-label="TUI agent mission title"
                  className="tui-input"
                  value={missionTitle}
                  onChange={(e) => setMissionTitle(e.target.value)}
                  placeholder="Enforce context cancellation on worker shutdown"
                />
              </label>
              <label>
                <div className="tui-sub">Target Files (comma-separated, max 8)</div>
                <input
                  aria-label="TUI agent target files"
                  className="tui-input"
                  value={missionPaths}
                  onChange={(e) => setMissionPaths(e.target.value)}
                  placeholder="internal/worker/worker.go, internal/worker/worker_test.go"
                />
              </label>
              <label>
                <div className="tui-sub">Clean-Code Specification</div>
                <textarea
                  aria-label="TUI agent specification"
                  className="tui-textarea"
                  rows={3}
                  value={missionPrompt}
                  onChange={(e) => setMissionPrompt(e.target.value)}
                  placeholder="Specify invariants, edge cases, and test expectations..."
                />
              </label>
              <button
                type="button"
                className="tui-btn primary"
                disabled={!!s.busy}
                onClick={dispatchCustomMission}
              >
                [Dispatch 4-Stage Agent Pipeline]
              </button>
            </div>
          </div>
        )}

        {currentTabPage === 'agents' && s.agentDetailOpen && (
          <>
            <div
              className="tui-box"
              style={{
                marginBottom: '8px',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                flexWrap: 'wrap',
                gap: '8px',
              }}
            >
              <div>
                <button type="button" className="tui-btn" onClick={() => w.openAgentsDashboard()}>
                  [← All Agents]
                </button>{' '}
                <strong className="tui-c-accent">
                  {selectedAgentItem && selectedAgentItem.linkedIssues.length > 0
                    ? selectedAgentItem.linkedIssues.map((iss) => iss.name).join(' · ')
                    : selectedAgentItem?.title || change?.title || 'Agent Detail'}
                </strong>
              </div>
              <div style={{ display: 'flex', gap: '6px', alignItems: 'center', flexWrap: 'wrap' }}>
                {selectedAgentItem?.hasProblem && (
                  <span className="tui-badge tui-c-red">
                    [PROBLEM: {selectedAgentItem.problemSummary}]
                  </span>
                )}
                {agentItems.length > 1 && (
                  <>
                    <button
                      type="button"
                      className="tui-btn"
                      onClick={() => {
                        const idx = agentItems.findIndex(
                          (item) => (item.path || '') === (s.selectedAgentPath || ''),
                        );
                        const prev = agentItems[(idx - 1 + agentItems.length) % agentItems.length];
                        if (prev) void w.selectAgent(prev.path);
                      }}
                    >
                      [← Prev Agent]
                    </button>
                    <button
                      type="button"
                      className="tui-btn"
                      onClick={() => {
                        const idx = agentItems.findIndex(
                          (item) => (item.path || '') === (s.selectedAgentPath || ''),
                        );
                        const next = agentItems[(idx + 1) % agentItems.length];
                        if (next) void w.selectAgent(next.path);
                      }}
                    >
                      [Next Agent →]
                    </button>
                  </>
                )}
              </div>
            </div>

            <div className="tui-box focused">
              <span className="tui-box-title">4-STAGE AUTONOMOUS CLEAN-CODE PIPELINE</span>
              <span className="tui-box-meta">
                {workflow ? `WORKFLOW: ${workflow.status.toUpperCase()}` : 'READY'}
              </span>
              <div className="tui-grid-4" style={{ marginTop: '4px' }}>
                {(
                  [
                    ['create', '1. CREATE FIX'],
                    ['test', '2. VERIFY & TEST'],
                    ['review', '3. CRITIC REVIEW'],
                    ['human_review', '4. REVIEW & COMMIT'],
                  ] as const
                ).map(([stageId, label]) => {
                  const st =
                    workflow?.stages.find((x) => x.name === stageId)?.status ||
                    (change ? 'completed' : agentS.changeSeed ? 'staged' : 'idle');
                  const isStageSelected = selectedAgentStage === stageId;
                  return (
                    <div
                      key={stageId}
                      className={`tui-box${isStageSelected ? ' focused' : ''}`}
                      style={{ cursor: 'pointer' }}
                      onClick={() =>
                        setSelectedAgentStage((prev) => (prev === stageId ? 'all' : stageId))
                      }
                    >
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '6px',
                        }}
                      >
                        {st === 'running' ? (
                          <Spinner />
                        ) : st === 'completed' ? (
                          <span className="tui-c-green" aria-hidden="true">
                            [OK]
                          </span>
                        ) : st === 'failed' ? (
                          <span className="tui-c-red" aria-hidden="true">
                            [!]
                          </span>
                        ) : null}
                        <strong className="tui-c-accent">{label}</strong>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>

            <div className="tui-box" style={{ marginTop: '8px' }}>
              <span className="tui-box-title">CLEAN-SOFTWARE PRECISION SCORECARD</span>
              <span className="tui-box-meta">SCOPE: {scopeBadge}</span>
              {change ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  <div>
                    <strong>Mission:</strong> {change.title}
                  </div>
                  <div className="tui-sub">
                    Files: <strong>{changedFiles.length || change.targets.length}/8 max</strong> ·
                    Delta:{' '}
                    <span className="tui-c-green">
                      +{addedLines} / -{removedLines} lines
                    </span>{' '}
                    · Self-repairs: <strong>{change.repair_attempts}/3</strong>
                  </div>
                  {(selectedAgentStage === 'all' || selectedAgentStage === 'review') &&
                    workflow?.review && (
                      <div>
                        <div className="tui-c-purple">
                          CRITIC VERDICT: [{workflow.review.verdict.toUpperCase()}]
                        </div>
                        <pre className="tui-pre">{workflow.review.summary}</pre>
                      </div>
                    )}
                  {(selectedAgentStage === 'all' || selectedAgentStage === 'test') && (
                    <div>
                      <div className="tui-c-green">
                        DETERMINISTIC CHECKS ({passedChecks}/{checks.length}):
                      </div>
                      {checks.length === 0 ? (
                        <div className="tui-sub">No check runs recorded yet.</div>
                      ) : (
                        checks.map((chk) => (
                          <div key={chk.name} className="tui-sub">
                            [{chk.state.toUpperCase()}] <strong>{chk.name}</strong>{' '}
                            {chk.output ? `— ${chk.output}` : ''}
                          </div>
                        ))
                      )}
                    </div>
                  )}
                  {(selectedAgentStage === 'all' || selectedAgentStage === 'human_review') &&
                    changedFiles.length > 0 && (
                      <div>
                        <div className="tui-c-accent">
                          PROPOSED DIFF PREVIEW ({changedFiles.length} file
                          {changedFiles.length === 1 ? '' : 's'}):
                        </div>
                        {changedFiles.map((fc) => (
                          <div key={fc.path} style={{ marginTop: '4px' }}>
                            <div className="tui-sub">
                              <strong>{fc.path}</strong>
                            </div>
                            <pre
                              className="tui-pre"
                              style={{ maxHeight: '160px', overflowY: 'auto' }}
                            >
                              {(fc.diff?.lines || []).slice(0, 40).map((ln, idx) => (
                                <span
                                  key={idx}
                                  className={
                                    isAddedLine(ln.kind)
                                      ? 'tui-diff-add'
                                      : isRemovedLine(ln.kind)
                                        ? 'tui-diff-del'
                                        : undefined
                                  }
                                >
                                  {isAddedLine(ln.kind)
                                    ? `+ ${ln.text}`
                                    : isRemovedLine(ln.kind)
                                      ? `- ${ln.text}`
                                      : `  ${ln.text}`}
                                  {'\n'}
                                </span>
                              ))}
                            </pre>
                          </div>
                        ))}
                      </div>
                    )}
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    {change.state !== 'applied' && changedFiles.length > 0 && (
                      <button
                        type="button"
                        className="tui-btn primary"
                        disabled={!!agentS.busy || running || !allChecksPassed}
                        onClick={() => void agentW.applyProposalNow()}
                      >
                        [Accept &amp; Commit]
                      </button>
                    )}
                    {change.state === 'applied' && selectedChild?.worktreePath && (
                      <button
                        type="button"
                        className="tui-btn primary"
                        disabled={!!s.busy}
                        onClick={() => void w.mergeAgentWorktree(selectedChild.worktreePath!)}
                      >
                        [Merge into {s.repository?.branch || 'main'} &amp; Close{' '}
                        {(selectedAgentItem?.linkedIssues.length || 0) > 1
                          ? `${selectedAgentItem?.linkedIssues.length} Issues`
                          : 'Issue'}
                        ]
                      </button>
                    )}
                    <button
                      type="button"
                      className="tui-btn"
                      onClick={() => void w.navigate('editor')}
                    >
                      [3: Inspect Diff &amp; Apply]
                    </button>
                    <button
                      type="button"
                      className="tui-btn"
                      disabled={!!agentS.busy || running}
                      onClick={() => void agentW.checkChange(true)}
                    >
                      [Re-run Checks]
                    </button>
                    <button
                      type="button"
                      className="tui-btn"
                      disabled={!!agentS.busy || running}
                      onClick={() => void agentW.rerunChangeWorkflow('agents')}
                    >
                      [Re-run Fix]
                    </button>
                    {((!allChecksPassed && checks.length > 0) || selectedAgentItem?.hasProblem) && (
                      <button
                        type="button"
                        className="tui-btn primary"
                        disabled={!!agentS.busy || running}
                        onClick={() => void agentW.repairChange()}
                      >
                        [Self-Repair]
                      </button>
                    )}
                    {agentS.changeReceipt?.undo_available && (
                      <button
                        type="button"
                        className="tui-btn"
                        disabled={!!agentS.busy}
                        onClick={() => void agentW.undoChange()}
                      >
                        [u: Undo]
                      </button>
                    )}
                    {running && (
                      <button
                        type="button"
                        className="tui-btn"
                        onClick={() => void agentW.cancelChangeWorkflow()}
                      >
                        [Cancel Workflow]
                      </button>
                    )}
                  </div>
                </div>
              ) : (
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: '8px' }}>
                  <p className="tui-sub">
                    Agent is staged or preparing changes. Press [Esc] or click [← All Agents] to
                    return.
                  </p>
                  <button
                    type="button"
                    className="tui-btn primary"
                    disabled={!!agentS.busy || running}
                    onClick={() => void agentW.rerunChangeWorkflow('agents')}
                  >
                    [Start Agent Pipeline]
                  </button>
                </div>
              )}
            </div>
          </>
        )}

        {/* [3] CODE & DIFF */}
        {currentTabPage === 'editor' && (
          <div className="tui-split-main">
            <div className="tui-box">
              <span className="tui-box-title">PROJECT FILES (j/k move · Enter open)</span>
              <span className="tui-box-meta">{s.index?.files.length || 0} files</span>
              <table className="tui-table">
                <thead>
                  <tr>
                    <th>PATH</th>
                    <th>LANGUAGE</th>
                  </tr>
                </thead>
                <tbody>
                  {editorFiles.map((f, idx) => (
                    <tr
                      key={f.path}
                      className={`tui-selectable${idx === clampedIdx || s.file?.path === f.path ? ' tui-selected' : ''}`}
                      onClick={() => {
                        setSelectedIdx(idx);
                        void w.openFile(f.path);
                      }}
                    >
                      <td>
                        {idx === clampedIdx && (
                          <span className="tui-cursor-indicator" aria-hidden="true">
                            &gt;
                          </span>
                        )}
                        {f.path}
                      </td>
                      <td className="tui-c-muted">{f.language || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="tui-box focused">
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  flexWrap: 'wrap',
                  gap: '6px',
                  marginBottom: '6px',
                }}
              >
                <span className="tui-box-title">
                  {(editorMode === 'diff' || (editorMode === 'auto' && changedFiles.length > 0)) &&
                  changedFiles.length > 0
                    ? `PROPOSED DIFF (${changedFiles.length} files)`
                    : `SOURCE: ${s.file?.path || 'none'}`}
                </span>
                <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                  <button
                    type="button"
                    className={`tui-btn${showTuiInlineWhy ? ' primary' : ''}`}
                    aria-label="Toggle TUI inline explanations"
                    aria-pressed={showTuiInlineWhy}
                    onClick={() => setShowTuiInlineWhy((prev) => !prev)}
                  >
                    [e: Why: {showTuiInlineWhy ? 'ON' : 'OFF'}]
                  </button>
                  <button
                    type="button"
                    className={`tui-btn${editorMode === 'source' || (editorMode === 'auto' && changedFiles.length === 0) ? ' primary' : ''}`}
                    onClick={() => setEditorMode('source')}
                  >
                    [Source]
                  </button>
                  <button
                    type="button"
                    className={`tui-btn${editorMode === 'diff' || (editorMode === 'auto' && changedFiles.length > 0) ? ' primary' : ''}`}
                    onClick={() => setEditorMode('diff')}
                  >
                    [Diff ({changedFiles.length})]
                  </button>
                </div>
              </div>
              {(editorMode === 'diff' || (editorMode === 'auto' && changedFiles.length > 0)) &&
              changedFiles.length > 0 ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  {changedFiles.map((fc) => {
                    const summaryInfo = summarizeFileEdit(fc, s.symbols);
                    return (
                      <div key={fc.path}>
                        <div className="tui-c-accent" style={{ fontWeight: 700 }}>
                          --- a/{fc.path} +++ b/{fc.path}
                        </div>
                        {showTuiInlineWhy && (
                          <div
                            className="tui-sub"
                            aria-label={`TUI inline summary for ${fc.path}`}
                            style={{
                              padding: '4px 8px',
                              border: '1px solid var(--tui-border)',
                              background: 'var(--tui-bg-alt)',
                              marginBottom: '4px',
                            }}
                          >
                            <strong className="tui-c-purple">[WHY]</strong> {summaryInfo.summary}
                          </div>
                        )}
                        <pre className="tui-pre">
                          {(fc.diff?.lines || []).map((ln, idx) => (
                            <span
                              key={idx}
                              className={
                                isAddedLine(ln.kind)
                                  ? 'tui-diff-add'
                                  : isRemovedLine(ln.kind)
                                    ? 'tui-diff-del'
                                    : undefined
                              }
                            >
                              {isAddedLine(ln.kind)
                                ? `+ ${ln.text}`
                                : isRemovedLine(ln.kind)
                                  ? `- ${ln.text}`
                                  : `  ${ln.text}`}
                              {'\n'}
                            </span>
                          ))}
                        </pre>
                      </div>
                    );
                  })}
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    <button
                      type="button"
                      className="tui-btn primary"
                      disabled={!!agentS.busy || !canAcceptChange(agentS)}
                      onClick={() => void agentW.applyProposalNow()}
                    >
                      [Apply Verified Proposal]
                    </button>
                    {agentS.changeReceipt?.undo_available && (
                      <button
                        type="button"
                        className="tui-btn"
                        disabled={!!agentS.busy}
                        onClick={() => void agentW.undoChange()}
                      >
                        [Undo Last Change]
                      </button>
                    )}
                  </div>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  <pre className="tui-pre">{s.file?.content || 'Select a file on the left.'}</pre>
                  {s.file && (
                    <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                      <input
                        aria-label="TUI file change instruction"
                        className="tui-input"
                        style={{ flex: 1 }}
                        value={editorPrompt}
                        onChange={(e) => setEditorPrompt(e.target.value)}
                        placeholder={`Ask AI to change ${s.file.path}…`}
                      />
                      <button
                        type="button"
                        className="tui-btn primary"
                        disabled={!!activeBusy || !editorPrompt.trim()}
                        onClick={() => {
                          const seed = workflowSeed({
                            title: editorPrompt.trim().slice(0, 64),
                            kind: 'fix',
                            paths: [s.file!.path],
                            message: editorPrompt.trim(),
                            acceptance_criteria: [
                              'Keep the diff minimal and preserve existing tests.',
                            ],
                          });
                          setEditorPrompt('');
                          void w.startAgentFix(seed);
                        }}
                      >
                        [Generate Diff with Agent]
                      </button>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        )}

        {/* [2] ARCH — LAYERED ARCHITECTURE & SUBSYSTEM INSPECTOR */}
        {currentTabPage === 'diagrams' && (
          <div className="tui-split-main">
            <div className="tui-box focused">
              <span className="tui-box-title">ARCHITECTURE &amp; SUBSYSTEMS (j/k navigate)</span>
              <span className="tui-box-meta">{archComponents.length} components</span>
              <table className="tui-table">
                <thead>
                  <tr>
                    <th>COMPONENT</th>
                    <th>LAYER</th>
                    <th>SUMMARY</th>
                  </tr>
                </thead>
                <tbody>
                  {archComponents.map((comp, idx) => (
                    <tr
                      key={comp.path || comp.name}
                      className={`tui-selectable${idx === clampedIdx ? ' tui-selected' : ''}`}
                      onClick={() => setSelectedIdx(idx)}
                    >
                      <td className="tui-c-accent">
                        {idx === clampedIdx && (
                          <span className="tui-cursor-indicator" aria-hidden="true">
                            &gt;
                          </span>
                        )}
                        {comp.name}
                      </td>
                      <td>{comp.layer}</td>
                      <td className="tui-c-muted">{comp.summary || 'Core module'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="tui-box">
              <span className="tui-box-title">SUBSYSTEM CONTRACT INSPECTOR</span>
              <span className="tui-box-meta">
                {archComponents[clampedIdx]?.path || 'subsystem'}
              </span>
              {archComponents[clampedIdx] ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  <div>
                    <strong className="tui-c-accent">{archComponents[clampedIdx].name}</strong> ·{' '}
                    <span className="tui-badge tui-c-purple">
                      {archComponents[clampedIdx].layer}
                    </span>
                    <div className="tui-sub">{archComponents[clampedIdx].summary}</div>
                  </div>
                  <div>
                    <div className="tui-c-green">DOMAIN RESPONSIBILITIES:</div>
                    {archComponents[clampedIdx].responsibilities.map((resp, idx) => (
                      <div key={idx} className="tui-sub">
                        • {resp}
                      </div>
                    ))}
                  </div>
                  <div>
                    <div className="tui-c-amber">PUBLIC CONTRACTS &amp; INTERFACES:</div>
                    {archComponents[clampedIdx].contracts.map((ct, idx) => (
                      <div key={idx} className="tui-sub">
                        <strong>{ct.name}</strong> [{ct.kind}] — {ct.summary}
                      </div>
                    ))}
                  </div>
                </div>
              ) : (
                <p className="tui-sub">Select a subsystem on the left to inspect its contracts.</p>
              )}
            </div>
          </div>
        )}

        {/* [3] DOCS — PURPOSE, GOAL PURSUIT, ARCHITECTURE, DEPENDENCIES & RUNBOOK */}
        {currentTabPage === 'docs' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            <div className="tui-box focused">
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  flexWrap: 'wrap',
                  gap: '6px',
                  marginBottom: '8px',
                }}
              >
                <span className="tui-box-title">
                  ENGINEERING SPECIFICATION &amp; DOCS (h/l switch section)
                </span>
                <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                  {(
                    [
                      ['context', '1. Purpose & Dependencies'],
                      ['components', `2. Components (${docs.components.length})`],
                      ['scenarios', `3. Scenarios (${docs.scenarios.length})`],
                      ['decisions', `4. ADRs & Runbook (${docs.decisions.length})`],
                    ] as const
                  ).map(([secId, label]) => (
                    <button
                      key={secId}
                      type="button"
                      className={`tui-btn${docsSection === secId ? ' primary' : ''}`}
                      onClick={() => setDocsSection(secId)}
                    >
                      [{label}]
                    </button>
                  ))}
                  <button
                    type="button"
                    className="tui-btn"
                    disabled={!!activeBusy || activeRun(s.run)}
                    onClick={() => void w.runAnalysisDirect(true)}
                  >
                    [r: Run Analysis]
                  </button>
                </div>
              </div>

              {docsSection === 'context' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  <div>
                    <div className="tui-c-accent">1. PURPOSE OF THE PROJECT:</div>
                    <pre className="tui-pre">{docs.context.purpose}</pre>
                    <div
                      style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', marginTop: '4px' }}
                    >
                      {docs.context.stack.map((st) => (
                        <span key={st} className="tui-pill">
                          {st}
                        </span>
                      ))}
                    </div>
                  </div>
                  <div>
                    <div className="tui-c-green">2. HOW IT PURSUES THE GOAL:</div>
                    {(docs.context.goal_pursuit || []).map((step, idx) => (
                      <div key={idx} className="tui-sub">
                        <strong>0{idx + 1}.</strong> {step}
                      </div>
                    ))}
                  </div>
                  <div>
                    <div className="tui-c-purple">3. ARCHITECTURE TOPOLOGY:</div>
                    <div className="tui-sub">{docs.context.topology}</div>
                  </div>
                  <div>
                    <div className="tui-c-amber">
                      4. MAIN DEPENDENCIES ({(docs.dependencies || []).length}):
                    </div>
                    <table className="tui-table">
                      <thead>
                        <tr>
                          <th>DEPENDENCY</th>
                          <th>VERSION</th>
                          <th>KIND</th>
                          <th>PURPOSE</th>
                        </tr>
                      </thead>
                      <tbody>
                        {(docs.dependencies || []).map((dep, idx) => (
                          <tr key={`${dep.name}-${idx}`}>
                            <td className="tui-c-accent">{dep.name}</td>
                            <td>{dep.version || '—'}</td>
                            <td>{dep.kind}</td>
                            <td className="tui-c-muted">{dep.purpose}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              {docsSection === 'components' && (
                <table className="tui-table">
                  <thead>
                    <tr>
                      <th>SUBSYSTEM</th>
                      <th>LAYER</th>
                      <th>PATH</th>
                      <th>CONTRACTS</th>
                    </tr>
                  </thead>
                  <tbody>
                    {docs.components.map((comp) => (
                      <tr key={comp.path}>
                        <td className="tui-c-accent">{comp.name}</td>
                        <td>{comp.layer}</td>
                        <td>{comp.path}</td>
                        <td className="tui-c-muted">
                          {comp.contracts.map((c) => c.name).join(', ')}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}

              {docsSection === 'scenarios' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  {docs.scenarios.map((sc, idx) => (
                    <div key={idx} className="tui-box">
                      <strong className="tui-c-accent">
                        Scenario {idx + 1}: {sc.title}
                      </strong>{' '}
                      <span className="tui-c-muted">[Trigger: {sc.trigger}]</span>
                      <div className="tui-sub">{sc.summary}</div>
                      {sc.steps.map((st, sIdx) => (
                        <div key={sIdx} className="tui-sub">
                          Step {sIdx + 1} ({st.phase} · {st.layer}): <strong>{st.title}</strong> —{' '}
                          {st.detail}
                        </div>
                      ))}
                    </div>
                  ))}
                </div>
              )}

              {docsSection === 'decisions' && (
                <div className="tui-split-main">
                  <div>
                    <div className="tui-c-amber">ARCHITECTURAL DECISION RECORDS (ADRs):</div>
                    {docs.decisions.map((adr, idx) => (
                      <div key={idx} className="tui-sub" style={{ marginBottom: '6px' }}>
                        <strong className="tui-c-accent">{adr.title}</strong> [{adr.category}]:{' '}
                        {adr.decision} (Tradeoff: {adr.tradeoff})
                      </div>
                    ))}
                  </div>
                  <div>
                    <div className="tui-c-green">OPERATIONAL RUNBOOK &amp; GLOSSARY:</div>
                    {docs.runbook.map((rb, idx) => (
                      <div key={idx} className="tui-sub">
                        <strong>{rb.label}</strong>: <code>{rb.command}</code> — {rb.description}
                      </div>
                    ))}
                    {docs.glossary.slice(0, 4).map((gl, idx) => (
                      <div key={idx} className="tui-sub">
                        <strong className="tui-c-purple">{gl.term}</strong>: {gl.definition}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* [6] FEATURES — GOALS, SUGGESTIONS & TRIAGE */}
        {currentTabPage === 'features' && (
          <div className="tui-split-main">
            <div className="tui-box focused">
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  flexWrap: 'wrap',
                  gap: '6px',
                  marginBottom: '8px',
                }}
              >
                <span className="tui-box-title">
                  FEATURE IDEAS (j/k move · Enter stage in Agents · d dismiss)
                </span>
                <span className="tui-box-meta">{featureSuggestions.length} suggestions</span>
              </div>
              <div
                style={{
                  display: 'flex',
                  gap: '6px',
                  marginBottom: '8px',
                  flexWrap: 'wrap',
                }}
              >
                <input
                  aria-label="TUI project goals"
                  className="tui-input"
                  style={{ flex: 1 }}
                  value={featureGoalInput || s.features?.goals || ''}
                  onChange={(e) => setFeatureGoalInput(e.target.value)}
                  placeholder="Project goals for feature suggestions…"
                />
                <button
                  type="button"
                  className="tui-btn"
                  disabled={!!activeBusy}
                  onClick={() => void w.updateFeatures(featureGoalInput || s.features?.goals || '')}
                >
                  [Save Goals]
                </button>
                <button
                  type="button"
                  className="tui-btn primary"
                  disabled={!!activeBusy}
                  onClick={() =>
                    void w.searchFeatures('Features', featureGoalInput || s.features?.goals || '')
                  }
                >
                  [Suggest Features]
                </button>
              </div>
              <table className="tui-table">
                <thead>
                  <tr>
                    <th>EFFORT</th>
                    <th>TITLE</th>
                    <th>BENEFIT</th>
                    <th>ACTIONS</th>
                  </tr>
                </thead>
                <tbody>
                  {featureSuggestions.map((idea, idx) => (
                    <tr
                      key={idea.id}
                      className={`tui-selectable${idx === clampedIdx ? ' tui-selected' : ''}`}
                      onClick={() => setSelectedIdx(idx)}
                    >
                      <td>
                        {idx === clampedIdx && (
                          <span className="tui-cursor-indicator" aria-hidden="true">
                            &gt;
                          </span>
                        )}
                        <span className="tui-badge tui-c-purple">{idea.effort || 'medium'}</span>
                      </td>
                      <td>{idea.title}</td>
                      <td className="tui-c-muted">{idea.benefit}</td>
                      <td style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                        <button
                          type="button"
                          className="tui-btn primary"
                          onClick={() => {
                            void w.discussFeature(idea);
                          }}
                        >
                          [Fix]
                        </button>
                        <button
                          type="button"
                          className="tui-btn"
                          onClick={() =>
                            void w.setFeatureStatus(
                              idea.id,
                              idea.status === 'saved' ? 'open' : 'saved',
                            )
                          }
                        >
                          [{idea.status === 'saved' ? 'Saved *' : 'Save'}]
                        </button>
                        <button
                          type="button"
                          className="tui-btn"
                          onClick={() => void w.setFeatureStatus(idea.id, 'dismissed')}
                        >
                          [Dismiss]
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="tui-box">
              <span className="tui-box-title">FEATURE IDEA INSPECTOR</span>
              <span className="tui-box-meta">
                {featureSuggestions[clampedIdx]?.paths.join(', ') || 'feature'}
              </span>
              {featureSuggestions[clampedIdx] ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  <div>
                    <strong className="tui-c-accent">{featureSuggestions[clampedIdx].title}</strong>
                    <div className="tui-sub">{featureSuggestions[clampedIdx].benefit}</div>
                  </div>
                  {featureSuggestions[clampedIdx].evidence && (
                    <div>
                      <div className="tui-c-amber">ARCHITECTURAL RATIONALE:</div>
                      <pre className="tui-pre">{featureSuggestions[clampedIdx].evidence}</pre>
                    </div>
                  )}
                  {(featureSuggestions[clampedIdx].acceptance_criteria || []).length > 0 && (
                    <div>
                      <div className="tui-c-green">ACCEPTANCE CRITERIA:</div>
                      {featureSuggestions[clampedIdx].acceptance_criteria.map((ac, i) => (
                        <div key={i} className="tui-sub">
                          • {ac}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              ) : (
                <p className="tui-sub">
                  Select a feature idea or click [Suggest Features] to generate ideas.
                </p>
              )}
            </div>
          </div>
        )}

        {currentTabPage === 'analysis-run' && (
          <div className="tui-box focused">
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                flexWrap: 'wrap',
                gap: '6px',
                marginBottom: '8px',
              }}
            >
              <span className="tui-box-title">
                LAST ANALYSIS RUN (j/k move · Enter open category · r run/resume)
              </span>
              <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                {s.run && ['paused', 'interrupted'].includes(s.run.status) && (
                  <button
                    type="button"
                    className="tui-btn primary"
                    disabled={!!activeBusy}
                    onClick={() => void w.resumeAnalysisDirect()}
                  >
                    [Resume Analysis]
                  </button>
                )}
                {!activeRun(s.run) &&
                  !(s.run && ['paused', 'interrupted'].includes(s.run.status)) &&
                  canRepairAnalysis(s) && (
                    <button
                      type="button"
                      className="tui-btn primary"
                      disabled={!!activeBusy}
                      onClick={() => void w.repairAnalysisDirect()}
                    >
                      {recoverableFileCount(s) > 0
                        ? `[Retry Failed (${recoverableFileCount(s)})]`
                        : '[Repair Analysis]'}
                    </button>
                  )}
                {activeRun(s.run) && (
                  <>
                    <button
                      type="button"
                      className="tui-btn"
                      disabled={!!activeBusy}
                      onClick={() => void w.controlRun('pause')}
                    >
                      [Pause]
                    </button>
                    <button
                      type="button"
                      className="tui-btn"
                      disabled={!!activeBusy}
                      onClick={() => void w.controlRun('cancel')}
                    >
                      [Cancel]
                    </button>
                  </>
                )}
                <button
                  type="button"
                  className="tui-btn"
                  disabled={!!activeBusy || activeRun(s.run)}
                  onClick={() => void w.runAnalysisDirect(true)}
                >
                  [New Analysis]
                </button>
              </div>
            </div>
            {s.run ? (
              <>
                <div
                  className="tui-sub"
                  style={{
                    marginBottom: '8px',
                    display: 'flex',
                    justifyContent: 'space-between',
                    flexWrap: 'wrap',
                    gap: '8px',
                  }}
                >
                  <span>
                    Status: <strong className="tui-c-accent">{human(s.run.status)}</strong> · Files
                    in scope: <strong>{s.run.files.length}</strong> · Elapsed:{' '}
                    <strong>{s.run.elapsed_seconds}s</strong>
                  </span>
                  <span className="tui-c-amber">
                    Open Issues: <strong>{issueSummary.totalOpenIssues}</strong> (
                    {issueSummary.bugCount}b/{issueSummary.secCount}s/{issueSummary.perfCount}p) ·
                    Feature Ideas: <strong>{issueSummary.featureCount}</strong>
                  </span>
                </div>
                <table className="tui-table">
                  <thead>
                    <tr>
                      <th>PHASE</th>
                      <th>STATUS</th>
                      <th>OPEN / SAVED</th>
                      <th>COVERAGE</th>
                    </tr>
                  </thead>
                  <tbody>
                    {runSections.map((sec, idx) => {
                      const openForCat =
                        sec.category === 'bugs'
                          ? issueSummary.bugCount
                          : sec.category === 'security'
                            ? issueSummary.secCount
                            : issueSummary.perfCount;
                      return (
                        <tr
                          key={sec.category}
                          className={`tui-selectable${idx === clampedIdx ? ' tui-selected' : ''}`}
                          onClick={() => {
                            setSelectedIdx(idx);
                            void w.navigate(sec.category as Page);
                          }}
                        >
                          <td className="tui-c-accent">
                            {idx === clampedIdx && (
                              <span className="tui-cursor-indicator" aria-hidden="true">
                                &gt;
                              </span>
                            )}
                            {human(sec.category)}
                          </td>
                          <td>{sec.status}</td>
                          <td>
                            {openForCat} open
                            {typeof sec.finding_count === 'number'
                              ? ` (${sec.finding_count} in run)`
                              : ''}
                          </td>
                          <td className="tui-c-muted">
                            {sec.coverage.succeeded}/{sec.coverage.total} files
                          </td>
                        </tr>
                      );
                    })}
                    {s.run.features && (
                      <tr className="tui-selectable" onClick={() => void w.navigate('features')}>
                        <td className="tui-c-accent">Feature Discovery</td>
                        <td>{s.run.features.status}</td>
                        <td>{issueSummary.featureCount} ideas</td>
                        <td className="tui-c-muted">
                          {s.run.features.attempts}/{s.run.plan.limits.max_attempts_per_stage}{' '}
                          attempts
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </>
            ) : (
              <p className="tui-sub">No analysis run recorded yet.</p>
            )}
          </div>
        )}

        {currentTabPage === 'git' && (
          <div className="tui-box focused">
            <span className="tui-box-title">
              GIT &amp; WORKTREES (j/k move · Enter open · m merge)
            </span>
            <span className="tui-box-meta">{s.repository?.branch || 'main'}</span>
            <div className="tui-sub" style={{ marginBottom: '8px' }}>
              HEAD: <code>{s.repository?.head || '—'}</code> · Worktrees:{' '}
              <strong>{gitWorktrees.filter((wt) => !wt.current).length}</strong>
            </div>
            <table className="tui-table">
              <thead>
                <tr>
                  <th>BRANCH</th>
                  <th>PATH</th>
                  <th>STATE</th>
                  <th>ACTIONS</th>
                </tr>
              </thead>
              <tbody>
                {gitWorktrees.map((wt, idx) => (
                  <tr
                    key={wt.path}
                    className={`tui-selectable${idx === clampedIdx ? ' tui-selected' : ''}`}
                    onClick={() => setSelectedIdx(idx)}
                  >
                    <td className="tui-c-accent">
                      {idx === clampedIdx && (
                        <span className="tui-cursor-indicator" aria-hidden="true">
                          &gt;
                        </span>
                      )}
                      {wt.branch || 'detached'}
                    </td>
                    <td>{wt.path}</td>
                    <td>
                      <span
                        className={`tui-badge ${
                          wt.current || wt.mergeable
                            ? 'tui-c-green'
                            : wt.dirty || wt.locked || wt.prunable
                              ? 'tui-c-amber'
                              : 'tui-c-accent'
                        }`}
                      >
                        {wt.current ? 'CURRENT' : wt.mergeable ? 'READY TO MERGE' : 'IN PROGRESS'}
                      </span>
                    </td>
                    <td style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                      {!wt.current && (
                        <>
                          <button
                            type="button"
                            className="tui-btn"
                            disabled={!!s.busy}
                            onClick={() => void w.selectAgent(wt.path)}
                          >
                            [Open in Agents]
                          </button>
                          {wt.mergeable && (
                            <button
                              type="button"
                              className="tui-btn primary"
                              disabled={!!s.busy}
                              onClick={() => void w.mergeAgentWorktree(wt.path)}
                            >
                              [Merge]
                            </button>
                          )}
                          <button
                            type="button"
                            className="tui-btn"
                            disabled={!!s.busy || wt.dirty || wt.locked}
                            onClick={() => void w.discardWorktree(wt)}
                          >
                            [Discard]
                          </button>
                        </>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {currentTabPage === 'instructions' &&
          (() => {
            const bootstrapPillars = evaluateBootstrapChecklist(s);
            const readyCount = bootstrapPillars.filter((p) => p.ready).length;
            const missingCount = bootstrapPillars.length - readyCount;
            const defaultLessonRule =
              'Propagate context.Context cancellation and add a regression test for every check repair.';
            const dynamicLessons = collectHarnessLessons(s);
            const allTuiLessons = [
              defaultLessonRule,
              ...dynamicLessons.filter(
                (l) => l.trim().toLowerCase() !== defaultLessonRule.toLowerCase(),
              ),
            ];
            return (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    flexWrap: 'wrap',
                    gap: '6px',
                  }}
                >
                  <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                    <button
                      type="button"
                      className={`tui-btn${rulesSection === 'trust' ? ' primary' : ''}`}
                      onClick={() => setRulesSection('trust')}
                    >
                      [1. Trust &amp; Rules]
                    </button>
                    <button
                      type="button"
                      className={`tui-btn${rulesSection === 'models' ? ' primary' : ''}`}
                      onClick={() => setRulesSection('models')}
                    >
                      [2. Model Routing]
                    </button>
                    <button
                      type="button"
                      className={`tui-btn${rulesSection === 'bootstrap' ? ' primary' : ''}`}
                      onClick={() => setRulesSection('bootstrap')}
                    >
                      [3. Repo Bootstrap ({readyCount}/4)]
                    </button>
                    <button
                      type="button"
                      className={`tui-btn${rulesSection === 'lessons' ? ' primary' : ''}`}
                      onClick={() => setRulesSection('lessons')}
                    >
                      [4. Harness² Inbox]
                    </button>
                  </div>
                  <span className="tui-sub">h/l or [Tab] switch section</span>
                </div>

                {rulesSection === 'lessons' ? (
                  <div className="tui-box focused">
                    <div
                      style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        flexWrap: 'wrap',
                        gap: '6px',
                        marginBottom: '8px',
                      }}
                    >
                      <span className="tui-box-title">
                        HARNESS² LESSON INBOX (g graduate lesson to AGENTS.md)
                      </span>
                      <span className="tui-badge tui-c-purple">DISCOVERED INVARIANTS</span>
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                      {allTuiLessons.map((lessonRule, idx) => {
                        const lessonId = idx === 0 ? 'lesson-default-1' : `lesson-dyn-${idx}`;
                        const isGraduated =
                          graduatedTuiLessons.includes(lessonId) ||
                          containsGuideline(activeInstructionDraft, lessonRule);
                        return (
                          <div
                            key={lessonId}
                            style={{
                              display: 'flex',
                              justifyContent: 'space-between',
                              alignItems: 'center',
                              gap: '8px',
                              padding: '6px 8px',
                              border: '1px solid var(--tui-border)',
                              flexWrap: 'wrap',
                            }}
                          >
                            <div>
                              <strong className="tui-c-accent">
                                {idx === 0
                                  ? 'Verification & Self-Repair Invariant'
                                  : `Harness² Lesson #${idx}`}
                              </strong>
                              <div className="tui-sub">Rule: {lessonRule}</div>
                            </div>
                            <button
                              type="button"
                              className="tui-btn primary"
                              disabled={isGraduated}
                              onClick={() => graduateTuiLesson(lessonRule, lessonId)}
                            >
                              [{isGraduated ? 'Graduated [OK]' : 'g: Graduate to AGENTS.md'}]
                            </button>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                ) : rulesSection === 'bootstrap' ? (
                  <div className="tui-box focused">
                    <div
                      style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        flexWrap: 'wrap',
                        gap: '6px',
                        marginBottom: '8px',
                      }}
                    >
                      <span className="tui-box-title">
                        STEP-BY-STEP ENVIRONMENT BOOTSTRAP CHECKLIST (b bootstrap missing)
                      </span>
                      <div style={{ display: 'flex', gap: '6px', alignItems: 'center' }}>
                        <span
                          className={`tui-badge ${readyCount === 4 ? 'tui-c-green' : 'tui-c-amber'}`}
                        >
                          {readyCount}/4 PILLARS READY
                        </span>
                        {missingCount > 0 && (
                          <button
                            type="button"
                            className="tui-btn primary"
                            disabled={!!activeBusy}
                            onClick={() => void w.bootstrapMissingPillars()}
                          >
                            [b: Bootstrap Missing ({missingCount})]
                          </button>
                        )}
                        <button
                          type="button"
                          className="tui-btn"
                          disabled={!!activeBusy}
                          onClick={() => void w.runBootstrapToolchain()}
                        >
                          [Run mise trust &amp;&amp; mise install]
                        </button>
                      </div>
                    </div>
                    {s.bootstrapExec && s.bootstrapExec.steps.length > 0 && (
                      <div
                        style={{
                          display: 'flex',
                          gap: '6px',
                          flexWrap: 'wrap',
                          marginBottom: '8px',
                        }}
                      >
                        {s.bootstrapExec.steps.map((step) => (
                          <span
                            key={step.command}
                            className={`tui-badge ${
                              step.state === 'passed'
                                ? 'tui-c-green'
                                : step.state === 'failed'
                                  ? 'tui-c-red'
                                  : 'tui-c-amber'
                            }`}
                          >
                            {step.command}: {step.state}
                          </span>
                        ))}
                      </div>
                    )}
                    <table className="tui-table" aria-label="TUI environment bootstrap checklist">
                      <thead>
                        <tr>
                          <th>PILLAR</th>
                          <th>TARGET FILE</th>
                          <th>STATUS</th>
                          <th>ACTION</th>
                        </tr>
                      </thead>
                      <tbody>
                        {bootstrapPillars.map((pillar) => (
                          <tr key={pillar.id}>
                            <td>
                              <strong className="tui-c-accent">{pillar.title}</strong>
                              <div className="tui-sub">{pillar.summary}</div>
                            </td>
                            <td>
                              <code>{pillar.detectedPath || pillar.targetPath}</code>
                            </td>
                            <td>
                              <span
                                className={`tui-badge ${pillar.ready ? 'tui-c-green' : 'tui-c-amber'}`}
                              >
                                {pillar.statusLabel}
                              </span>
                            </td>
                            <td style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                              {!pillar.ready && (
                                <button
                                  type="button"
                                  className="tui-btn primary"
                                  disabled={!!activeBusy}
                                  onClick={() => void w.bootstrapPillar(pillar.id)}
                                >
                                  [Bootstrap {pillar.targetPath}]
                                </button>
                              )}
                              {pillar.ready && pillar.detectedPath && (
                                <button
                                  type="button"
                                  className="tui-btn"
                                  disabled={!!activeBusy}
                                  onClick={() => void w.openFile(pillar.detectedPath!)}
                                >
                                  [Inspect {pillar.detectedPath}]
                                </button>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : rulesSection === 'models' ? (
                  <div className="tui-box focused">
                    <span className="tui-box-title">
                      4-STAGE AGENT MODEL ROUTING (j/k preset · Enter apply)
                    </span>
                    <span className="tui-box-meta">MODELS</span>
                    <div className="tui-sub">
                      Stage 1 (Create): <code className="tui-c-accent">{models.create}</code>
                    </div>
                    <div className="tui-sub">
                      Stage 2 (Test): <code className="tui-c-green">{models.test}</code>
                    </div>
                    <div className="tui-sub" style={{ marginBottom: '8px' }}>
                      Stage 3 (Critic): <code className="tui-c-purple">{models.review}</code>
                    </div>
                    <table className="tui-table" aria-label="Model presets">
                      <thead>
                        <tr>
                          <th>PRESET</th>
                          <th>DESCRIPTION</th>
                          <th>STATUS</th>
                        </tr>
                      </thead>
                      <tbody>
                        {modelPresets.map((preset, idx) => {
                          const active = isModelPresetActive(s, preset);
                          return (
                            <tr
                              key={preset.id}
                              className={`tui-selectable${idx === clampedIdx ? ' tui-selected' : ''}`}
                              onClick={() => {
                                setSelectedIdx(idx);
                                w.applyModelPreset(preset);
                              }}
                            >
                              <td>
                                {idx === clampedIdx && (
                                  <span className="tui-cursor-indicator" aria-hidden="true">
                                    &gt;
                                  </span>
                                )}
                                <strong>{preset.name}</strong>
                              </td>
                              <td className="tui-c-muted">{preset.description}</td>
                              <td>
                                {active ? (
                                  <span className="tui-badge tui-c-green">ACTIVE</span>
                                ) : (
                                  <span className="tui-c-muted">Enter to apply</span>
                                )}
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <div className="tui-box focused">
                    <span className="tui-box-title">
                      WORKSPACE TRUST &amp; AGENTS.MD (t toggle trust)
                    </span>
                    <span className="tui-box-meta">BOUNDARIES</span>
                    <div
                      style={{
                        display: 'flex',
                        gap: '8px',
                        marginBottom: '8px',
                        flexWrap: 'wrap',
                      }}
                    >
                      <button
                        type="button"
                        className="tui-btn primary"
                        onClick={() => void w.setWorkspaceTrust(s.workspaceTrust === false)}
                      >
                        [{s.workspaceTrust !== false ? 'Revoke Trust' : 'Grant Workspace Trust'}]
                      </button>
                      <button
                        type="button"
                        className="tui-btn"
                        onClick={() => void w.openProjectInstructions()}
                      >
                        [Reload AGENTS.md]
                      </button>
                    </div>
                    <pre className="tui-pre">
                      {activeInstructionDraft ||
                        'AGENTS.md rules enforce surgical diffs, passing tests, and explicit Apply.'}
                    </pre>
                  </div>
                )}
              </div>
            );
          })()}
      </main>

      {/* Interactive Keybindings & Commands Reference Modal */}
      {showKeybindsModal && (
        <div
          className="tui-modal-backdrop"
          onClick={() => setShowKeybindsModal(false)}
          role="presentation"
        >
          <div
            className="tui-modal"
            role="dialog"
            aria-modal="true"
            aria-label="TUI Keybindings Reference"
            onClick={(e) => e.stopPropagation()}
          >
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                borderBottom: '1px solid var(--tui-border)',
                paddingBottom: '8px',
              }}
            >
              <div>
                <strong className="tui-c-accent">
                  MINI-ORCA TUI // KEYBOARD SHORTCUTS &amp; COMMANDS REFERENCE
                </strong>
                <div className="tui-sub">
                  100% keyboard-driven navigation — press [?] or [Esc] to close
                </div>
              </div>
              <button
                type="button"
                className="tui-btn primary"
                aria-label="Close keybindings reference"
                onClick={() => setShowKeybindsModal(false)}
              >
                [Esc / Close]
              </button>
            </div>

            <div className="tui-keybind-grid">
              <div className="tui-box">
                <span className="tui-box-title">GLOBAL NAVIGATION</span>
                <table className="tui-table">
                  <tbody>
                    <tr>
                      <td>
                        <strong className="tui-key">[1]–[5]</strong>
                      </td>
                      <td>Jump to TUI workspace hub</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">[Tab]</strong>
                      </td>
                      <td>Cycle sub-views inside active hub</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">[ / ]</strong>
                      </td>
                      <td>Previous / Next workspace hub</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">j / k / ↓ / ↑</strong>
                      </td>
                      <td>Move row cursor down / up</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">h / l / ← / →</strong>
                      </td>
                      <td>Cycle Open / In Progress / Done</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">Enter</strong>
                      </td>
                      <td>Open / Execute highlighted item</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">Esc / Backspace</strong>
                      </td>
                      <td>Back to All Agents / Close modal</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">?</strong>
                      </td>
                      <td>Toggle Keybindings Reference</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">/ or :</strong>
                      </td>
                      <td>Focus orca&gt; command prompt</td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div className="tui-box">
                <span className="tui-box-title">WORKFLOW &amp; ACTIONS</span>
                <table className="tui-table">
                  <tbody>
                    <tr>
                      <td>
                        <strong className="tui-key">a</strong>
                      </td>
                      <td>Dispatch Fix / Open Agent</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">e</strong>
                      </td>
                      <td>Toggle inline diff explanations</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">x / Space</strong>
                      </td>
                      <td>Toggle issue for batch agent fix</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">n</strong>
                      </td>
                      <td>New custom agent mission form</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">c</strong>
                      </td>
                      <td>Accept &amp; Commit verified proposal</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">m</strong>
                      </td>
                      <td>Merge agent worktree into main</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">r</strong>
                      </td>
                      <td>Run/Resume analysis or Self-Repair</td>
                    </tr>
                    <tr>
                      <td>
                        <strong className="tui-key">t / b / g</strong>
                      </td>
                      <td>Tests/Trust · Bootstrap · Graduate lesson / GUI</td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div className="tui-box">
                <span className="tui-box-title">ORCA&gt; COLON COMMANDS</span>
                <table className="tui-table">
                  <tbody>
                    <tr>
                      <td>
                        <code className="tui-c-green">:keys</code> /{' '}
                        <code className="tui-c-green">:help</code>
                      </td>
                      <td>Show this keybind reference</td>
                    </tr>
                    <tr>
                      <td>
                        <code className="tui-c-green">:analyze</code> /{' '}
                        <code className="tui-c-green">:resume</code>
                      </td>
                      <td>Run or resume project analysis</td>
                    </tr>
                    <tr>
                      <td>
                        <code className="tui-c-green">:test</code>
                      </td>
                      <td>Run verification test suite</td>
                    </tr>
                    <tr>
                      <td>
                        <code className="tui-c-green">:fix</code>
                      </td>
                      <td>Dispatch selected issue(s) to agent</td>
                    </tr>
                    <tr>
                      <td>
                        <code className="tui-c-green">:agent &lt;prompt&gt;</code>
                      </td>
                      <td>Stage custom agent mission</td>
                    </tr>
                    <tr>
                      <td>
                        <code className="tui-c-green">:open &lt;path&gt;</code>
                      </td>
                      <td>Open file in [3] Code &amp; Diff</td>
                    </tr>
                    <tr>
                      <td>
                        <code className="tui-c-green">:apply</code> /{' '}
                        <code className="tui-c-green">:merge</code>
                      </td>
                      <td>Commit proposal or merge branch</td>
                    </tr>
                    <tr>
                      <td>
                        <code className="tui-c-green">:undo</code> /{' '}
                        <code className="tui-c-green">:gui</code>
                      </td>
                      <td>Guarded Undo or exit to GUI</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Bottom Command Prompt Bar */}
      <footer className="tui-cmdbar">
        <form
          className="tui-cmd-form"
          onSubmit={(e) => {
            e.preventDefault();
            executeCommand(cmdInput);
            setCmdInput('');
          }}
        >
          <span className="tui-cmd-prompt">orca&gt;</span>
          <input
            ref={cmdRef}
            aria-label="TUI command prompt"
            className="tui-cmd-input"
            value={cmdInput}
            onChange={(e) => setCmdInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'ArrowUp' && cmdHistory.length > 0) {
                e.preventDefault();
                const nextIdx = Math.min(cmdHistory.length - 1, cmdHistoryIdx + 1);
                setCmdHistoryIdx(nextIdx);
                setCmdInput(cmdHistory[nextIdx] || '');
              } else if (e.key === 'ArrowDown' && cmdHistoryIdx >= 0) {
                e.preventDefault();
                const nextIdx = cmdHistoryIdx - 1;
                setCmdHistoryIdx(nextIdx);
                setCmdInput(nextIdx >= 0 ? cmdHistory[nextIdx] || '' : '');
              }
            }}
            placeholder={cmdOutput}
          />
        </form>
        <div className="tui-hints">
          <span>
            <strong className="tui-key">[1-5 / Tab]</strong> hub / sub-view
          </span>
          <span>
            <strong className="tui-key">[j/k]</strong> move
          </span>
          <span>
            <strong className="tui-key">[Enter]</strong> open
          </span>
          <span>
            <strong className="tui-key">[e]</strong> why
          </span>
          <span>
            <strong className="tui-key">[a]</strong> agent fix
          </span>
          <span>
            <strong className="tui-key">[?]</strong> keys
          </span>
          <span>
            <strong className="tui-key">[:gui]</strong> GUI mode
          </span>
        </div>
      </footer>
    </div>
  );
}
