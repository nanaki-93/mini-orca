import { useEffect, useRef, useState } from 'react';
import {
  activeChangeWorkflow,
  activeRun,
  allModelPresets,
  canAcceptChange,
  changeChecksPassed,
  findingCategory,
  isModelPresetActive,
  isTestFilePath,
  workflowSeed,
  workflowSetupModels,
  workspace as w,
  type Page,
  type State,
} from './workspace';
import { findingName, findingTitle } from './finding-name';
import { isAddedLine, isRemovedLine } from './change-shared';
import { human } from './ui';
import type * as M from './models';
import './tui.css';

type TuiPalette = 'tokyo' | 'crt' | 'amber' | 'paper';

const TUI_TABS: { key: string; page: Page; label: string }[] = [
  { key: '1', page: 'summary', label: 'Overview' },
  { key: '2', page: 'diagrams', label: 'Arch' },
  { key: '3', page: 'docs', label: 'Docs' },
  { key: '4', page: 'editor', label: 'Code/Diff' },
  { key: '5', page: 'bugs', label: 'Issues' },
  { key: '6', page: 'features', label: 'Features' },
  { key: '7', page: 'agents', label: 'Agents' },
  { key: '8', page: 'analysis-run', label: 'Last Run' },
  { key: '9', page: 'git', label: 'Git/Trees' },
  { key: '0', page: 'instructions', label: 'Rules/Models' },
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
  cause: string;
  solution: string;
  target: M.FindingTarget;
  acceptance_criteria: string[];
}

function collectTuiIssues(s: State): TuiIssueRow[] {
  const rows: TuiIssueRow[] = [];
  for (const f of s.findings || []) {
    if (f.status === 'fixed') continue;
    rows.push({
      id: `semantic:${f.id}`,
      title: findingTitle(f.title, f.message),
      name: findingName(f.title, f.message),
      path: f.location.path,
      symbol: f.location.symbol,
      line: f.location.start_line,
      severity: f.severity,
      category: 'bugs',
      cause: f.message || f.evidence || 'No cause recorded.',
      solution:
        f.task_spec?.acceptance_criteria.join('\n') ||
        'Synthesize a minimal, verified fix and unit test.',
      target: { project_id: f.project_id, kind: 'semantic', id: f.id, path: f.location.path },
      acceptance_criteria: f.task_spec?.acceptance_criteria || [],
    });
  }
  for (const cat of ['bugs', 'security', 'performance'] as const) {
    const res = s.results[cat];
    for (const f of res?.semantic || []) {
      if (f.status === 'fixed' || rows.some((r) => r.id === `semantic:${f.id}`)) continue;
      rows.push({
        id: `semantic:${f.id}`,
        title: findingTitle(f.title, f.message),
        name: findingName(f.title, f.message),
        path: f.location.path,
        symbol: f.location.symbol,
        line: f.location.start_line,
        severity: f.severity,
        category: cat,
        cause: f.message || f.evidence || 'No cause recorded.',
        solution:
          f.task_spec?.acceptance_criteria.join('\n') ||
          'Synthesize a minimal, verified fix and unit test.',
        target: { project_id: f.project_id, kind: 'semantic', id: f.id, path: f.location.path },
        acceptance_criteria: f.task_spec?.acceptance_criteria || [],
      });
    }
  }
  return rows;
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
  const [cmdInput, setCmdInput] = useState('');
  const [cmdOutput, setCmdOutput] = useState(
    'Type a command (help, analyze, test, fix, apply, undo, gui) or press 1-0 to switch views.',
  );
  const [missionTitle, setMissionTitle] = useState(s.changeSeed?.title || '');
  const [missionPaths, setMissionPaths] = useState(
    s.changeSeed?.paths.join(', ') || s.file?.path || s.index?.files[0]?.path || '',
  );
  const [missionPrompt, setMissionPrompt] = useState(s.changeSeed?.message || '');
  const cmdRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    localStorage.setItem('mini-orca:tui-palette', palette);
  }, [palette]);

  useEffect(() => {
    if (s.changeSeed) {
      setMissionTitle(s.changeSeed.title);
      setMissionPaths(s.changeSeed.paths.join(', '));
      setMissionPrompt(s.changeSeed.message);
    }
  }, [s.changeSeed?.title, s.changeSeed?.message]);

  const issues = collectTuiIssues(s);
  const clampedIdx = Math.min(selectedIdx, Math.max(0, issues.length - 1));
  const selectedIssue = issues[clampedIdx];
  const models = workflowSetupModels(s);
  const change = s.change;
  const workflow = change?.workflow;
  const running = activeChangeWorkflow(change);

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
    w.seedChange(seed, 'agents');
    setCmdOutput(`Dispatched Fix with Agent for ${issue.name} (${issue.path})`);
    void (async () => {
      try {
        const context = await w.readFixPreparation();
        if (seed.paths.some(isTestFilePath)) {
          await w.startChangeWorkflow(seed, models, context, 'agents');
        } else {
          await w.prepareChange(seed, true, models.create, context, 'agents');
        }
      } catch (err) {
        w.fail(err);
      }
    })();
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
      title: missionTitle.trim() || 'Clean-code agent mission',
      kind: 'fix',
      paths: rawPaths,
      message: missionPrompt.trim(),
      acceptance_criteria: [
        'Keep the diff minimal and limited to the requested behavior.',
        'Ensure all unit tests and static checks pass.',
      ],
    });
    w.seedChange(seed, 'agents');
    setCmdOutput(`Dispatched custom agent mission on ${seed.paths.join(', ')}`);
    void (async () => {
      try {
        const context = await w.readFixPreparation();
        if (seed.paths.some(isTestFilePath)) {
          await w.startChangeWorkflow(seed, models, context, 'agents');
        } else {
          await w.prepareChange(seed, true, models.create, context, 'agents');
        }
      } catch (err) {
        w.fail(err);
      }
    })();
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement | null)?.tagName?.toLowerCase();
      if (tag === 'input' || tag === 'textarea' || tag === 'select') {
        if (e.key === 'Escape') (e.target as HTMLElement).blur();
        return;
      }
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const tab = TUI_TABS.find((t) => t.key === e.key);
      if (tab) {
        e.preventDefault();
        void w.navigate(tab.page);
        return;
      }
      if (e.key === '/' || e.key === ':') {
        e.preventDefault();
        cmdRef.current?.focus();
      } else if (e.key === 'j') {
        e.preventDefault();
        setSelectedIdx((i) => Math.min(Math.max(0, issues.length - 1), i + 1));
      } else if (e.key === 'k') {
        e.preventDefault();
        setSelectedIdx((i) => Math.max(0, i - 1));
      } else if (e.key === 'a' && selectedIssue) {
        e.preventDefault();
        dispatchIssueToAgent(selectedIssue);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [issues.length, selectedIssue]);

  const executeCommand = (raw: string) => {
    const trimmed = raw.trim().replace(/^:/, '');
    if (!trimmed) return;
    const [verb, ...rest] = trimmed.split(/\s+/);
    const arg = rest.join(' ');
    const cmd = verb.toLowerCase();
    if (cmd === 'gui' || cmd === 'exit') {
      onExitTui();
      return;
    }
    if (cmd === 'help' || cmd === '?') {
      setCmdOutput(
        'Commands: [1-0] view | analyze | test | fix | agent <prompt> | open <file> | apply | undo | gui',
      );
      return;
    }
    if (cmd === 'analyze' || cmd === 'run') {
      setCmdOutput('Starting project analysis...');
      void w.runAnalysisDirect(false);
      return;
    }
    if (cmd === 'test' || cmd === 'scan') {
      setCmdOutput('Running project tests and checks...');
      void w.scanProject(false);
      return;
    }
    if (cmd === 'fix') {
      if (selectedIssue) {
        dispatchIssueToAgent(selectedIssue);
      } else {
        void w.navigate('agents');
        setCmdOutput('Opened Agent Mission Control.');
      }
      return;
    }
    if (cmd === 'agent' || cmd === 'agents') {
      if (arg) {
        setMissionPrompt(arg);
      }
      void w.navigate('agents');
      setCmdOutput('Switched to [7] Agent Mission Control.');
      return;
    }
    if (cmd === 'open' && arg) {
      setCmdOutput(`Opening ${arg} in Code/Diff...`);
      void w.openFile(arg);
      return;
    }
    if (cmd === 'apply') {
      setCmdOutput('Applying verified proposal...');
      void w.applyProposalNow();
      return;
    }
    if (cmd === 'undo') {
      setCmdOutput('Requesting guarded Undo...');
      void w.undoChange();
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
    setCmdOutput(`Unknown command "${trimmed}". Type "help" for available commands.`);
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
  const checks = change?.checks || [];
  const passedChecks = checks.filter((c) => c.state === 'passed').length;
  const allChecksPassed = change ? changeChecksPassed(change) : false;

  const currentTabPage: Page =
    findingCategory(s) || s.page === 'finding'
      ? 'bugs'
      : ['models', 'instructions', 'analysis-files', 'context'].includes(activePage)
        ? 'instructions'
        : activePage;

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

      {/* Numbered Tab Bar */}
      <nav className="tui-tabbar" aria-label="TUI Workspaces">
        {TUI_TABS.map((t) => {
          const isCurrent = currentTabPage === t.page;
          const badge =
            t.page === 'bugs' && issues.length > 0
              ? `(${issues.length})`
              : t.page === 'agents' && workflow
                ? `[${workflow.status}]`
                : t.page === 'editor' && changedFiles.length > 0
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
      </nav>

      {/* Notices / Errors */}
      {(s.error || s.notice || s.busy) && (
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
            {s.error ? (
              <span className="tui-c-red">[ERROR] {s.error}</span>
            ) : s.busy ? (
              <span className="tui-c-accent">[BUSY] {s.busy}…</span>
            ) : (
              <span className="tui-c-green">[OK] {s.notice}</span>
            )}
          </span>
          {s.busy && (
            <button type="button" className="tui-btn" onClick={() => void w.cancel()}>
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
                <span className="tui-box-meta">[5] TRIAGE</span>
                <div className="tui-stat tui-c-amber">{issues.length}</div>
                <div className="tui-sub">
                  {s.features?.suggestions.length || 0} feature ideas queued
                </div>
              </div>
              <div className="tui-box">
                <span className="tui-box-title">TESTS &amp; CHECKS</span>
                <span className="tui-box-meta">GATE</span>
                <div className="tui-stat tui-c-green">
                  {s.scan ? human(s.scan.status) : 'Ready'}
                </div>
                <div className="tui-sub">
                  <button
                    type="button"
                    className="tui-btn"
                    disabled={!!s.busy}
                    onClick={() => void w.scanProject(false)}
                  >
                    [Run Tests]
                  </button>
                </div>
              </div>
              <div className="tui-box focused">
                <span className="tui-box-title">AGENT PIPELINE</span>
                <span className="tui-box-meta">[7] AGENTS</span>
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
                <span className="tui-box-title">PRIORITY FINDINGS (j/k select, a = Agent Fix)</span>
                <span className="tui-box-meta">{issues.length} total</span>
                {issues.length === 0 ? (
                  <p className="tui-sub">
                    No open issues. Run <code>analyze</code> or click [Run Analysis].
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
                      {issues.slice(0, 8).map((iss, idx) => (
                        <tr
                          key={iss.id}
                          className={`tui-selectable${idx === clampedIdx ? ' tui-selected' : ''}`}
                          onClick={() => setSelectedIdx(idx)}
                        >
                          <td>
                            <span
                              className={`tui-badge ${
                                ['critical', 'high'].includes(iss.severity)
                                  ? 'tui-c-red'
                                  : 'tui-c-amber'
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
                              disabled={!!s.busy || running}
                              onClick={(e) => {
                                e.stopPropagation();
                                dispatchIssueToAgent(iss);
                              }}
                            >
                              [Fix with Agent]
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
                    [Run Analysis]
                  </button>
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
                    onClick={() => void w.navigate('agents')}
                  >
                    [7: Agent Mission Control]
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
              <span className="tui-box-title">ISSUES QUEUE (j/k move · a dispatch agent)</span>
              <span className="tui-box-meta">{issues.length} open</span>
              {issues.length === 0 ? (
                <p className="tui-sub">No open findings.</p>
              ) : (
                <table className="tui-table">
                  <thead>
                    <tr>
                      <th>SEV</th>
                      <th>CAT</th>
                      <th>TITLE</th>
                      <th>FILE</th>
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
                          <span
                            className={`tui-badge ${
                              ['critical', 'high'].includes(iss.severity)
                                ? 'tui-c-red'
                                : 'tui-c-amber'
                            }`}
                          >
                            {iss.severity}
                          </span>
                        </td>
                        <td className="tui-c-purple">{iss.category}</td>
                        <td>{iss.name}</td>
                        <td className="tui-c-muted">
                          {iss.path}
                          {iss.line ? `:${iss.line}` : ''}
                        </td>
                        <td>
                          <button
                            type="button"
                            className="tui-btn primary"
                            disabled={!!s.busy || running}
                            onClick={(e) => {
                              e.stopPropagation();
                              dispatchIssueToAgent(iss);
                            }}
                          >
                            [Fix with Agent]
                          </button>
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
                  <div>
                    <div className="tui-c-amber">ROOT CAUSE:</div>
                    <pre className="tui-pre">{selectedIssue.cause}</pre>
                  </div>
                  <div>
                    <div className="tui-c-green">CLEAN-CODE REMEDIATION:</div>
                    <pre className="tui-pre">{selectedIssue.solution}</pre>
                  </div>
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    <button
                      type="button"
                      className="tui-btn primary"
                      disabled={!!s.busy || running}
                      onClick={() => dispatchIssueToAgent(selectedIssue)}
                    >
                      [a] Fix with Agent
                    </button>
                    <button
                      type="button"
                      className="tui-btn"
                      onClick={() => void w.openFile(selectedIssue.path, selectedIssue.symbol)}
                    >
                      [Inspect File]
                    </button>
                  </div>
                </div>
              ) : (
                <p className="tui-sub">Select a finding to inspect its cause and agent fix plan.</p>
              )}
            </div>
          </div>
        )}

        {/* [7] AGENTS — TUI AGENT MISSION CONTROL */}
        {currentTabPage === 'agents' && (
          <>
            <div className="tui-box focused">
              <span className="tui-box-title">4-STAGE AUTONOMOUS CLEAN-CODE PIPELINE</span>
              <span className="tui-box-meta">
                {workflow ? `WORKFLOW: ${workflow.status.toUpperCase()}` : 'READY'}
              </span>
              <div className="tui-grid-4" style={{ marginTop: '4px' }}>
                {(
                  [
                    ['create', '1. ARCHITECT & CODE', models.create],
                    ['test', '2. VERIFY & TEST', models.test],
                    ['review', '3. CLEAN-CODE CRITIC', models.review],
                    ['human_review', '4. HUMAN PRECISION GATE', 'explicit-apply'],
                  ] as const
                ).map(([stageId, label, modelName]) => {
                  const st =
                    workflow?.stages.find((x) => x.name === stageId)?.status ||
                    (change ? 'completed' : s.changeSeed ? 'staged' : 'idle');
                  return (
                    <div key={stageId} className="tui-box">
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <strong className="tui-c-accent">{label}</strong>
                        <span
                          className={`tui-badge ${
                            st === 'completed'
                              ? 'tui-c-green'
                              : st === 'running'
                                ? 'tui-c-purple'
                                : st === 'failed'
                                  ? 'tui-c-red'
                                  : 'tui-c-muted'
                          }`}
                        >
                          {st}
                        </span>
                      </div>
                      <div className="tui-sub" style={{ marginTop: '4px' }}>
                        model: <code>{modelName}</code>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>

            <div className="tui-split-main">
              <div className="tui-box">
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
                    {workflow?.review && (
                      <div>
                        <div className="tui-c-purple">
                          CRITIC VERDICT: [{workflow.review.verdict.toUpperCase()}]
                        </div>
                        <pre className="tui-pre">{workflow.review.summary}</pre>
                      </div>
                    )}
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
                    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                      <button
                        type="button"
                        className="tui-btn primary"
                        onClick={() => void w.navigate('editor')}
                      >
                        [4: Inspect Diff &amp; Apply]
                      </button>
                      <button
                        type="button"
                        className="tui-btn"
                        disabled={!!s.busy || running}
                        onClick={() => void w.checkChange(true)}
                      >
                        [Re-run Checks]
                      </button>
                      {!allChecksPassed && checks.length > 0 && (
                        <button
                          type="button"
                          className="tui-btn"
                          disabled={!!s.busy || running}
                          onClick={() => void w.repairChange()}
                        >
                          [Self-Repair]
                        </button>
                      )}
                      {running && (
                        <button
                          type="button"
                          className="tui-btn"
                          onClick={() => void w.cancelChangeWorkflow()}
                        >
                          [Cancel Workflow]
                        </button>
                      )}
                    </div>
                  </div>
                ) : (
                  <p className="tui-sub">
                    No active agent mission. Select a finding in [5] Issues and press [a] Fix with
                    Agent, or dispatch a custom mission on the right.
                  </p>
                )}
              </div>

              <div className="tui-box">
                <span className="tui-box-title">DISPATCH PRECISION AGENT MISSION</span>
                <span className="tui-box-meta">MAX 8 FILES</span>
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
                    <div className="tui-sub">Target Files (comma-separated)</div>
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
                    disabled={!!s.busy || running}
                    onClick={dispatchCustomMission}
                  >
                    [Dispatch 4-Stage Agent Pipeline]
                  </button>
                </div>
              </div>
            </div>
          </>
        )}

        {/* [4] CODE / DIFF */}
        {currentTabPage === 'editor' && (
          <div className="tui-split-main">
            <div className="tui-box">
              <span className="tui-box-title">PROJECT FILES</span>
              <span className="tui-box-meta">{s.index?.files.length || 0} files</span>
              <table className="tui-table">
                <thead>
                  <tr>
                    <th>PATH</th>
                    <th>LANGUAGE</th>
                  </tr>
                </thead>
                <tbody>
                  {(s.index?.files || []).slice(0, 18).map((f) => (
                    <tr
                      key={f.path}
                      className={`tui-selectable${s.file?.path === f.path ? ' tui-selected' : ''}`}
                      onClick={() => void w.openFile(f.path)}
                    >
                      <td>{f.path}</td>
                      <td className="tui-c-muted">{f.language || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="tui-box focused">
              <span className="tui-box-title">
                {changedFiles.length > 0
                  ? `PROPOSED DIFF (${changedFiles.length} files)`
                  : `SOURCE: ${s.file?.path || 'none'}`}
              </span>
              <span className="tui-box-meta">READ-ONLY / VERIFIED GATE</span>
              {changedFiles.length > 0 ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  {changedFiles.map((fc) => (
                    <div key={fc.path}>
                      <div className="tui-c-accent" style={{ fontWeight: 700 }}>
                        --- a/{fc.path} +++ b/{fc.path}
                      </div>
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
                  ))}
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    <button
                      type="button"
                      className="tui-btn primary"
                      disabled={!!s.busy || !canAcceptChange(s)}
                      onClick={() => void w.applyProposalNow()}
                    >
                      [Apply Verified Proposal]
                    </button>
                    {s.changeReceipt?.undo_available && (
                      <button
                        type="button"
                        className="tui-btn"
                        disabled={!!s.busy}
                        onClick={() => void w.undoChange()}
                      >
                        [Undo Last Change]
                      </button>
                    )}
                  </div>
                </div>
              ) : (
                <pre className="tui-pre">{s.file?.content || 'Select a file on the left.'}</pre>
              )}
            </div>
          </div>
        )}

        {/* [2] ARCH, [3] DOCS, [6] FEATURES, [8] LAST RUN, [9] GIT, [0] RULES/MODELS */}
        {currentTabPage === 'diagrams' && (
          <div className="tui-box focused">
            <span className="tui-box-title">ARCHITECTURE &amp; SUBSYSTEMS</span>
            <span className="tui-box-meta">
              {s.overview?.docs?.components?.length || 0} components
            </span>
            <table className="tui-table">
              <thead>
                <tr>
                  <th>COMPONENT</th>
                  <th>LAYER</th>
                  <th>SUMMARY</th>
                </tr>
              </thead>
              <tbody>
                {(s.overview?.docs?.components || []).map((comp) => (
                  <tr key={comp.path || comp.name}>
                    <td className="tui-c-accent">{comp.name}</td>
                    <td>{comp.layer}</td>
                    <td className="tui-c-muted">{comp.summary || 'Core module'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {currentTabPage === 'docs' && (
          <div className="tui-box focused">
            <span className="tui-box-title">ENGINEERING SPECIFICATION &amp; DOCS</span>
            <span className="tui-box-meta">{s.project?.name || 'docs'}</span>
            <pre className="tui-pre">
              {s.overview?.docs?.context?.purpose ||
                s.overview?.analysis?.purpose ||
                'Run project analysis to synthesize architectural documentation.'}
            </pre>
          </div>
        )}

        {currentTabPage === 'features' && (
          <div className="tui-box focused">
            <span className="tui-box-title">FEATURE IDEAS</span>
            <span className="tui-box-meta">{s.features?.suggestions.length || 0} suggestions</span>
            <table className="tui-table">
              <thead>
                <tr>
                  <th>EFFORT</th>
                  <th>TITLE</th>
                  <th>BENEFIT</th>
                  <th>AGENT</th>
                </tr>
              </thead>
              <tbody>
                {(s.features?.suggestions || []).map((idea) => (
                  <tr key={idea.id}>
                    <td>
                      <span className="tui-badge tui-c-purple">{idea.effort || 'medium'}</span>
                    </td>
                    <td>{idea.title}</td>
                    <td className="tui-c-muted">{idea.benefit}</td>
                    <td>
                      <button
                        type="button"
                        className="tui-btn primary"
                        onClick={() => {
                          w.seedWorkflow(
                            {
                              title: idea.title,
                              kind: 'feature',
                              paths: idea.paths,
                              acceptance_criteria: idea.acceptance_criteria,
                              message: `${idea.title}\n\nBenefit: ${idea.benefit}`,
                            },
                            'agents',
                          );
                        }}
                      >
                        [Stage in Agents]
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {currentTabPage === 'analysis-run' && (
          <div className="tui-box focused">
            <span className="tui-box-title">LAST ANALYSIS RUN</span>
            <span className="tui-box-meta">{s.run?.status || 'idle'}</span>
            {s.run ? (
              <table className="tui-table">
                <thead>
                  <tr>
                    <th>PHASE</th>
                    <th>STATUS</th>
                    <th>FINDINGS</th>
                    <th>COVERAGE</th>
                  </tr>
                </thead>
                <tbody>
                  {s.run.sections.map((sec) => (
                    <tr key={sec.category}>
                      <td className="tui-c-accent">{human(sec.category)}</td>
                      <td>{sec.status}</td>
                      <td>{sec.finding_count}</td>
                      <td className="tui-c-muted">
                        {sec.coverage.succeeded}/{sec.coverage.total} files
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <p className="tui-sub">No analysis run recorded yet.</p>
            )}
          </div>
        )}

        {currentTabPage === 'git' && (
          <div className="tui-box focused">
            <span className="tui-box-title">GIT &amp; WORKTREES</span>
            <span className="tui-box-meta">{s.repository?.branch || 'main'}</span>
            <div className="tui-sub" style={{ marginBottom: '8px' }}>
              HEAD: <code>{s.repository?.head || '—'}</code> · Worktrees:{' '}
              <strong>{s.gitWorkspace?.worktrees.length || 0}</strong>
            </div>
            <table className="tui-table">
              <thead>
                <tr>
                  <th>BRANCH</th>
                  <th>PATH</th>
                  <th>STATE</th>
                </tr>
              </thead>
              <tbody>
                {(s.gitWorkspace?.worktrees || []).map((wt) => (
                  <tr key={wt.path}>
                    <td className="tui-c-accent">{wt.branch || 'detached'}</td>
                    <td>{wt.path}</td>
                    <td className="tui-c-green">{wt.current ? 'CURRENT' : 'WORKTREE'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {currentTabPage === 'instructions' && (
          <div className="tui-split-main">
            <div className="tui-box focused">
              <span className="tui-box-title">WORKSPACE TRUST &amp; AGENTS.MD</span>
              <span className="tui-box-meta">BOUNDARIES</span>
              <div style={{ display: 'flex', gap: '8px', marginBottom: '8px' }}>
                <button
                  type="button"
                  className="tui-btn primary"
                  onClick={() => void w.setWorkspaceTrust(s.workspaceTrust === false)}
                >
                  [{s.workspaceTrust !== false ? 'Revoke Trust' : 'Grant Workspace Trust'}]
                </button>
              </div>
              <pre className="tui-pre">
                {s.projectInstructions?.existing_content ||
                  s.instructionPreview?.existing_content ||
                  'AGENTS.md rules enforce surgical diffs, passing tests, and explicit Apply.'}
              </pre>
            </div>
            <div className="tui-box">
              <span className="tui-box-title">4-STAGE AGENT MODEL ROUTING</span>
              <span className="tui-box-meta">MODELS</span>
              <div className="tui-sub">
                Stage 1 (Create): <code className="tui-c-accent">{models.create}</code>
              </div>
              <div className="tui-sub">
                Stage 2 (Test): <code className="tui-c-green">{models.test}</code>
              </div>
              <div className="tui-sub">
                Stage 3 (Critic): <code className="tui-c-purple">{models.review}</code>
              </div>
            </div>
          </div>
        )}
      </main>

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
            placeholder={cmdOutput}
          />
        </form>
        <div className="tui-hints">
          <span>
            <strong className="tui-key">[1-0]</strong> view
          </span>
          <span>
            <strong className="tui-key">[j/k]</strong> select
          </span>
          <span>
            <strong className="tui-key">[a]</strong> agent fix
          </span>
          <span>
            <strong className="tui-key">[/]</strong> command
          </span>
          <span>
            <strong className="tui-key">[:gui]</strong> GUI mode
          </span>
        </div>
      </footer>
    </div>
  );
}
