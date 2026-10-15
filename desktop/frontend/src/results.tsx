import { Fragment, useEffect, useRef, useState } from 'react';
import type * as M from './models';
import {
  workspace as w,
  activeChangeWorkflow,
  findingCategory,
  inProgressFindingTargets,
  isTestFilePath,
  workflowSeed,
  workflowSetupModels,
  activeRun,
  type FindingCategory,
  type FindingListView,
  type State,
} from './workspace';
import { FixPreparation } from './fix-preparation';
import { FixHeading } from './fix-workspace';
import { SolutionExcerpt } from './fix-solution';
import { StaleAnalysisButton } from './analysis';
import { findingName, findingTitle } from './finding-name';
import {
  Badge,
  Button,
  Disclosure,
  Empty,
  Go,
  Heading,
  Icon,
  InsightCard,
  Notice,
  Panel,
  Prose,
  Spinner,
  PriorityIcon,
  StatusDot,
  human,
} from './ui';
import { collectAgentDashboardItems, type AgentDashboardItem } from './agents';

interface ResultRow {
  key: string;
  category?: FindingCategory;
  featureId?: string;
  title: string;
  name: string;
  path: string;
  symbol: string;
  line: number;
  severity: string;
  confidence: string;
  freshness: string;
  status: string;
  commitOid?: string;
  verification?: string;
  kind: string;
  insight?: M.Insight;
  text: [string, string][];
  finding?: M.Finding;
  task?: M.TaskSpec;
  cause: string;
  solution: string;
  target: M.FindingTarget;
}

function isClosedStatus(status: string): boolean {
  return status === 'fixed' || status === 'dismissed' || status === 'retired';
}

function findAgentForRow(s: State, row: ResultRow): AgentDashboardItem | undefined {
  const items = collectAgentDashboardItems(s);
  const rowTitle = row.name.trim().toLowerCase();
  const fullTitle = row.title.trim().toLowerCase();
  return items.find((item) => {
    if (
      item.findingTargets.some(
        (t) =>
          t.kind === row.target.kind &&
          t.id === row.target.id &&
          t.path === row.target.path &&
          (t.source || '') === (row.target.source || ''),
      )
    ) {
      return true;
    }
    if (item.linkedIssues.some((issue) => issue.key === row.key)) {
      return true;
    }
    const itemTitle = item.title.trim().toLowerCase();
    return itemTitle === rowTitle || itemTitle === fullTitle;
  });
}
function semantic(f: M.Finding): ResultRow {
  return {
    key: `semantic:${f.id}`,
    target: { project_id: f.project_id, kind: 'semantic', id: f.id, path: f.location.path },
    title: findingTitle(f.title, f.message),
    name: findingName(f.title, f.message),
    path: f.location.path,
    symbol: f.location.symbol,
    line: f.location.start_line,
    severity: f.severity,
    confidence: f.confidence,
    freshness: f.freshness,
    status: f.status,
    kind: ['suggested', 'ai_suggestion'].includes(f.confidence) ? 'AI analysis' : f.source,
    insight: f.engineering_insight,
    text: [],
    finding: f,
    task: f.task_spec,
    cause: [...new Set([f.message, f.evidence].filter(Boolean))].join('\n\n'),
    solution:
      f.task_spec?.acceptance_criteria.join('\n') ||
      'No solution yet. Prepare a fix to review proposed changes.',
  };
}
function performance(report: M.PerformanceReport): ResultRow[] {
  return (report.findings || []).map((f) => ({
    key: `performance:${report.path}:${f.id}`,
    target: { project_id: report.project_id, kind: 'performance', id: f.id, path: report.path },
    title: f.title,
    name: findingName(f.title, f.observed_pattern),
    path: report.path,
    symbol: f.symbol,
    line: f.start_line,
    severity: f.potential_impact,
    confidence: f.confidence,
    freshness: report.status,
    status: report.status,
    kind: 'Performance hypothesis',
    insight: f.engineering_insight,
    cause: f.observed_pattern,
    solution: f.recommendation,
    text: [
      ['Workload', f.workload_conditions],
      ['Tradeoff', f.tradeoff],
      ['Verification', f.verification_plan],
    ],
  }));
}
function security(report: M.SecurityReport): ResultRow[] {
  return (report.findings || []).map((f) => ({
    key: `security:${report.path}:${report.source}:${f.id}`,
    target: {
      project_id: report.project_id,
      kind: 'security',
      id: f.id,
      path: f.source_anchor.path || report.path,
      source: report.source,
    },
    title: f.title,
    name: findingName(f.title, f.observed_condition),
    path: f.source_anchor.path || report.path,
    symbol: f.source_anchor.symbol,
    line: f.source_anchor.start_line,
    severity: f.severity,
    confidence: f.confidence,
    freshness: report.status,
    status: f.verification_state || f.triage,
    verification: f.verification_state,
    kind: report.source || 'Security finding',
    insight: f.engineering_insight,
    cause: f.observed_condition,
    solution: f.remediation,
    text: [
      ['Evidence', f.evidence_kind],
      ['Preconditions & unknowns', f.preconditions_or_unknowns],
      ['Verification', f.verification_idea],
      ['Rule', f.rule],
      ['CWE', f.cwe || ''],
      ['Reference', f.reference || ''],
    ],
  }));
}
function resultRows(s: State, category: FindingCategory): ResultRow[] {
  const results = s.results[category];
  const activeTargets = inProgressFindingTargets(s);
  const retainedSemantic = (s.findings || [])
    .filter(
      (f) =>
        f.category === category &&
        (f.status === 'in_progress' || activeTargets.some((t) => t.id === f.id)),
    )
    .map(semantic);
  const rows = results
    ? [
        ...(results.semantic || []).map(semantic),
        ...retainedSemantic,
        ...(results.performance || []).flatMap(performance),
        ...(results.security || []).flatMap(security),
      ]
    : (s.findings || []).filter((f) => f.category === category).map(semantic);
  if (category === 'security' && s.securityReport) rows.push(...security(s.securityReport));
  return [...new Map(rows.map((row) => [row.key, row])).values()].map((row) => {
    const completion = s.completedFindings?.find(
      (target) =>
        target.project_id === row.target.project_id &&
        target.kind === row.target.kind &&
        target.id === row.target.id &&
        target.path === row.target.path &&
        (target.source || '') === (row.target.source || ''),
    );
    return {
      ...row,
      category,
      commitOid: completion?.commit_oid,
      status: completion ? 'fixed' : row.status,
    };
  });
}
export function selectedResult(s: State) {
  const selected = s.selectedFinding;
  if (!selected) return undefined;
  return (
    resultRows(s, selected.category).find((row) => row.key === selected.key) ||
    resultRows(s, 'bugs').find((row) => row.key === selected.key) ||
    resultRows(s, 'performance').find((row) => row.key === selected.key) ||
    resultRows(s, 'security').find((row) => row.key === selected.key)
  );
}
const pageSize = 10;
export function Results({ s }: { s: State }) {
  const [categoryFilter, setCategoryFilter] = useState<'all' | FindingCategory | 'features'>(() =>
    s.page === 'features'
      ? 'features'
      : findingCategory(s) && s.page !== 'bugs'
        ? findingCategory(s)!
        : 'all',
  );
  useEffect(() => {
    if (s.page === 'features') {
      setCategoryFilter('features');
    } else if (s.page === 'bugs' && categoryFilter === 'features') {
      setCategoryFilter('all');
    } else if (s.page === 'security' || s.page === 'performance') {
      setCategoryFilter(s.page);
    }
  }, [s.page]);
  const [showDone, setShowDone] = useState(false);
  const [inProgressOnly, setInProgressOnly] = useState(false);
  const [sortBy, setSortBy] = useState<'default' | 'severity' | 'path' | 'category'>('default');
  const [goals, setGoals] = useState(s.features?.goals || '');
  useEffect(() => {
    setGoals(s.features?.goals || '');
  }, [s.features?.goals]);
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
  const currentCategory = findingCategory(s) || 'bugs';
  const category: FindingCategory =
    categoryFilter === 'all' || categoryFilter === 'features' ? currentCategory : categoryFilter;
  const kind =
    category === 'performance' ? 'performance' : category === 'security' ? 'security' : 'fix';
  const results = s.results[category];
  const listView = s.findingViews[category] || { query: '', severity: '', page: 1 };
  const { query, severity } = listView;
  const search = useRef<HTMLInputElement>(null);
  const updateView = (patch: Partial<FindingListView>) =>
    w.set({ findingViews: { ...s.findingViews, [category]: { ...listView, ...patch } } });
  const allRows = [
    ...resultRows(s, 'bugs'),
    ...resultRows(s, 'performance'),
    ...resultRows(s, 'security'),
  ];
  const bugRows = resultRows(s, 'bugs');
  const secRows = resultRows(s, 'security');
  const perfRows = resultRows(s, 'performance');
  const countBugs = bugRows.filter((row) => (showDone ? true : !isClosedStatus(row.status))).length;
  const countSec = secRows.filter((row) => (showDone ? true : !isClosedStatus(row.status))).length;
  const countPerf = perfRows.filter((row) =>
    showDone ? true : !isClosedStatus(row.status),
  ).length;
  const uniqueAllRows = [...new Map(allRows.map((r) => [r.key, r])).values()];
  const countAll = uniqueAllRows.filter((row) =>
    showDone ? true : !isClosedStatus(row.status),
  ).length;
  const inProgressCount = uniqueAllRows.filter(
    (row) => !isClosedStatus(row.status) && !!findAgentForRow(s, row),
  ).length;
  const closedCount = uniqueAllRows.filter((row) => isClosedStatus(row.status)).length;
  const categoryCounts: Record<'all' | FindingCategory, number> = {
    all: countAll,
    bugs: countBugs,
    security: countSec,
    performance: countPerf,
  };
  const unique =
    categoryFilter === 'all' || categoryFilter === 'features'
      ? uniqueAllRows
      : resultRows(s, categoryFilter);
  const severityRank: Record<string, number> = {
    critical: 0,
    high: 1,
    medium: 2,
    low: 3,
    info: 4,
  };
  const filtered = unique
    .filter(
      (row) =>
        (inProgressOnly
          ? !isClosedStatus(row.status) && !!findAgentForRow(s, row)
          : !showDone
            ? !isClosedStatus(row.status)
            : true) &&
        `${row.name} ${row.title} ${row.path} ${row.symbol}`
          .toLowerCase()
          .includes(query.toLowerCase()) &&
        (!severity || row.severity === severity),
    )
    .sort((a, b) => {
      if (sortBy === 'severity') {
        return (
          (severityRank[a.severity] ?? 9) - (severityRank[b.severity] ?? 9) ||
          a.path.localeCompare(b.path)
        );
      }
      if (sortBy === 'path') {
        return a.path.localeCompare(b.path) || a.line - b.line;
      }
      if (sortBy === 'category') {
        return (
          (a.category || '').localeCompare(b.category || '') ||
          (severityRank[a.severity] ?? 9) - (severityRank[b.severity] ?? 9)
        );
      }
      return 0;
    });
  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize));
  const page = Math.min(listView.page, pageCount);
  const offset = (page - 1) * pageSize;
  const windowStart = Math.max(2, Math.min(page - 1, pageCount - 3));
  const pageNumbers = [
    ...new Set([
      1,
      ...Array.from(
        { length: Math.min(3, Math.max(0, pageCount - 2)) },
        (_, index) => windowStart + index,
      ),
      pageCount,
    ]),
  ];
  const priorities = [...new Set([...unique.map((row) => row.severity), severity])]
    .filter(Boolean)
    .sort((a, b) => {
      const order = ['critical', 'high', 'medium', 'low', 'info'];
      return (
        (order.indexOf(a) < 0 ? order.length : order.indexOf(a)) -
          (order.indexOf(b) < 0 ? order.length : order.indexOf(b)) || a.localeCompare(b)
      );
    });
  const clearSearch = () => {
    updateView({ query: '', page: 1 });
    search.current?.focus();
  };
  useEffect(() => {
    if (listView.page !== page) updateView({ page });
  }, [listView.page, page]);
  const detail = s.page === 'finding' ? selectedResult(s) : undefined;
  const fixSeed = (row: ResultRow): M.ChangeSeed => ({
    finding_targets: [row.target],
    title: row.name,
    paths: [row.path],
    kind:
      row.category === 'performance'
        ? 'performance'
        : row.category === 'security'
          ? 'security'
          : kind,
    message: `Address this finding: ${row.title}\nLocation: ${row.path}${row.line ? `:${row.line}` : ''}${row.symbol ? ` (${row.symbol})` : ''}\nCause: ${row.cause}\nProposed solution: ${row.solution}\n${row.text.map(([label, text]) => `${label}: ${text}`).join('\n')}\nExplain only the proposed solution, including how each changed file addresses the finding. The cause is shown separately. Report unresolved failures or limitations. Preserve unrelated behavior.`,
    acceptance_criteria: row.task?.acceptance_criteria || [],
    finding: {
      path: row.path,
      symbol: row.symbol,
      line: row.line,
      cause: row.cause,
      solution: row.solution,
      confidence: row.confidence,
    },
  });
  const triggerFix = (row: ResultRow, targetPage: 'agents' | 'changes' = 'agents') => {
    const preparedSeed = workflowSeed(fixSeed(row));
    if (targetPage === 'agents') {
      void w.startAgentFix(preparedSeed);
      return;
    }
    w.seedChange(preparedSeed, targetPage);
    void (async () => {
      try {
        const context = await w.readFixPreparation();
        const useAgents = preparedSeed.paths.some(isTestFilePath);
        const models = workflowSetupModels(s);
        if (useAgents) {
          await w.startChangeWorkflow(preparedSeed, models, context, targetPage);
        } else {
          await w.prepareChange(preparedSeed, true, models.create, context, targetPage);
        }
      } catch (error) {
        w.fail(error);
      }
    })();
  };
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const toggleRowSelection = (key: string) => {
    setSelectedKeys((prev) =>
      prev.includes(key) ? prev.filter((item) => item !== key) : [...prev, key],
    );
  };
  const selectedRows = unique.filter((row) => selectedKeys.includes(row.key));
  const batchFixSeed = (rows: ResultRow[]): M.ChangeSeed => {
    if (rows.length === 1) return fixSeed(rows[0]);
    const hasTest = rows.some((r) => isTestFilePath(r.path));
    const maxPaths = hasTest ? 8 : 7;
    const paths = [...new Set(rows.map((r) => r.path).filter(Boolean))].slice(0, maxPaths);
    const capturedRows = rows.filter((r) => !r.path || paths.includes(r.path));
    const acceptance_criteria = [
      ...new Set(capturedRows.flatMap((r) => r.task?.acceptance_criteria || [])),
    ];
    const itemsText = capturedRows
      .map(
        (row, idx) =>
          `${idx + 1}. ${row.title} (${row.path}${row.line ? `:${row.line}` : ''}${row.symbol ? ` · ${row.symbol}` : ''})\n   Cause: ${row.cause}\n   Proposed solution: ${row.solution}`,
      )
      .join('\n\n');
    return {
      finding_targets: capturedRows.map((r) => r.target),
      title: `Batch fix (${capturedRows.length} issues)`,
      paths,
      kind,
      message: `Address these ${capturedRows.length} findings across the selected files:\n\n${itemsText}\n\nExplain how each changed file addresses the findings. Preserve unrelated behavior.`,
      acceptance_criteria,
      finding: {
        path: capturedRows[0].path,
        symbol: capturedRows[0].symbol,
        line: capturedRows[0].line,
        cause: capturedRows.map((r) => `${r.name}: ${r.cause}`).join('\n\n'),
        solution: capturedRows.map((r) => `${r.name}: ${r.solution}`).join('\n\n'),
        confidence: capturedRows[0].confidence,
      },
    };
  };
  const triggerBatchFix = (rows: ResultRow[]) => {
    if (!rows.length) return;
    const preparedSeed = workflowSeed(batchFixSeed(rows));
    setSelectedKeys([]);
    void w.startAgentFix(preparedSeed);
  };
  const seed = detail ? workflowSeed(fixSeed(detail)) : undefined;
  const fixDisabled = !!s.busy || activeChangeWorkflow(s.change) || detail?.freshness === 'stale';
  const detailPanel = detail ? (
    <div className="workspace-page results-page results-detail guided-fix" data-accent={category}>
      <div className="row between wrap fix-back">
        <Button icon="back" onClick={() => void w.navigate(category)}>
          All findings
        </Button>
        {!detail.featureId && (
          <div className="actions finding-actions">
            <Button
              icon={detail.status === 'dismissed' ? 'undo' : 'trash'}
              tone={detail.status === 'dismissed' ? 'default' : 'danger'}
              disabled={!!s.busy}
              onClick={() =>
                void w.triage(
                  detail.finding || detail.target,
                  detail.status === 'dismissed' ? 'open' : 'dismissed',
                )
              }
            >
              {detail.status === 'dismissed' ? 'Reopen' : 'Delete'}
            </Button>
          </div>
        )}
      </div>
      <FixHeading
        title={detail.name}
        path={detail.path}
        symbol={detail.symbol}
        line={detail.line}
        kind={kind}
        metadata={
          <>
            <span className="finding-priority">
              <PriorityIcon value={detail.severity} />
              <span>{human(detail.severity)}</span>
            </span>
            {detail.status !== detail.freshness && (
              <span aria-label="Finding state">
                <Badge
                  value={detail.status === 'fixed' ? 'Done' : detail.status}
                  tone={detail.status === 'fixed' ? 'green' : undefined}
                />
                {detail.status === 'fixed' && detail.verification && (
                  <Badge value={detail.verification} />
                )}
              </span>
            )}
            <span className="results-state">
              <StatusDot value={detail.freshness} label="Freshness" />
            </span>
          </>
        }
      >
        {detail.freshness === 'stale' && <StaleAnalysisButton s={s} path={detail.path} />}
        <Button
          icon="code"
          disabled={!!s.busy}
          onClick={() => void w.openFile(detail.path, detail.symbol)}
        >
          Go to file
        </Button>
      </FixHeading>
      {detail.freshness === 'stale' && (
        <Notice>This finding is stale. Refresh analysis before preparing a fix.</Notice>
      )}
      {category === 'performance' && (
        <Notice>Performance unmeasured; tests do not establish a runtime speedup.</Notice>
      )}
      <Panel title="Cause" className="fix-explanation fix-cause">
        {detail.name !== detail.title && detail.title !== detail.cause && (
          <Prose text={detail.title} />
        )}
        <Prose text={detail.cause || 'No cause recorded.'} />
        <p className="small muted">
          {human(detail.kind)} · Confidence: {human(detail.confidence)}
        </p>
      </Panel>
      <Panel title="Proposed solution" className="fix-explanation fix-solution">
        <SolutionExcerpt text={detail.solution || 'No remediation supplied.'} />
      </Panel>
      {seed && detail.path && (
        <FixPreparation key={detail.key} s={s} seed={seed} disabled={fixDisabled} />
      )}
      <div className="stack results-detail-layout">
        {detail.text
          .filter(([, content]) => content)
          .map(([title, content]) => (
            <Panel title={title} key={title}>
              <Prose text={content} />
            </Panel>
          ))}
        {detail.task && (
          <Panel title="Acceptance criteria">
            <ul>
              {detail.task.acceptance_criteria.map((item, i) => (
                <li key={i}>{item}</li>
              ))}
            </ul>
            <Disclosure title="Scope">
              <ul>
                {detail.task.non_goals?.map((item, i) => (
                  <li key={i}>{item}</li>
                ))}
              </ul>
              {detail.task.go_test_candidate && (
                <pre tabIndex={0}>{detail.task.go_test_candidate.content}</pre>
              )}
            </Disclosure>
          </Panel>
        )}
        <InsightCard insight={detail.insight} />
      </div>
    </div>
  ) : null;
  if (s.page === 'finding')
    return (
      detailPanel || (
        <div className="workspace-page results-page">
          <Empty
            title="Finding unavailable"
            detail="This finding is no longer in the loaded results."
          >
            <Go page={category} icon="back">
              All findings
            </Go>
          </Empty>
        </div>
      )
    );
  const reports = [
    ...(results?.performance || [])
      .filter((r) => r.warning || !(r.findings || []).length)
      .map((r) => ({ key: `p:${r.path}`, path: r.path, status: r.status, reason: r.warning })),
    ...[
      ...(results?.security || []),
      ...(category === 'security' && s.securityReport ? [s.securityReport] : []),
    ]
      .filter((r) => r.reason || !(r.findings || []).length)
      .map((r) => ({
        key: `s:${r.source}:${r.path}`,
        path: r.path,
        status: r.status,
        reason: r.reason,
      })),
  ];
  const featureRows: ResultRow[] = (s.features?.suggestions || [])
    .filter((idea) => (showDone ? true : idea.status !== 'dismissed'))
    .map((idea) => ({
      key: `feature:${idea.id}`,
      featureId: idea.id,
      target: {
        project_id: s.project?.project_id || '',
        kind: 'semantic',
        id: idea.id,
        path: idea.paths[0] || '',
      },
      title: idea.title,
      name: idea.title,
      path: idea.paths.join(', '),
      symbol: '',
      line: 0,
      severity: idea.effort || 'medium',
      confidence: 'ai_suggestion',
      freshness: idea.freshness || 'fresh',
      status: idea.status,
      kind: 'Feature idea',
      text: [['Evidence', idea.evidence]],
      cause: idea.benefit,
      solution: idea.acceptance_criteria.join('\n'),
    }));
  const [selectedInspectorKey, setSelectedInspectorKey] = useState<string>('');
  const showFeatures = categoryFilter === 'features' || s.page === 'features';
  const pageRows = showFeatures ? featureRows : filtered.slice(offset, offset + pageSize);
  const inspectorRow =
    (showFeatures
      ? featureRows.find((r) => r.key === selectedInspectorKey) || featureRows[0]
      : filtered.find((r) => r.key === selectedInspectorKey) || filtered[0]) || undefined;
  useEffect(() => {
    if (s.page === 'finding') return;
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement | null)?.tagName?.toLowerCase();
      if (
        tag === 'input' ||
        tag === 'textarea' ||
        tag === 'select' ||
        e.metaKey ||
        e.ctrlKey ||
        e.altKey
      ) {
        return;
      }
      if (!pageRows.length) return;
      const currentIndex = Math.max(
        0,
        pageRows.findIndex((r) => r.key === (inspectorRow?.key || '')),
      );
      if (e.key === 'j') {
        e.preventDefault();
        const next = pageRows[Math.min(pageRows.length - 1, currentIndex + 1)];
        if (next) setSelectedInspectorKey(next.key);
      } else if (e.key === 'k') {
        e.preventDefault();
        const prev = pageRows[Math.max(0, currentIndex - 1)];
        if (prev) setSelectedInspectorKey(prev.key);
      } else if (e.key === 'x' && !showFeatures && inspectorRow) {
        e.preventDefault();
        toggleRowSelection(inspectorRow.key);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [s.page, pageRows, inspectorRow, showFeatures]);
  const allPageSelected =
    !showFeatures && pageRows.length > 0 && pageRows.every((r) => selectedKeys.includes(r.key));
  const pausedOrInterrupted = !!s.run && ['paused', 'interrupted'].includes(s.run.status);
  const runTotal = s.run?.sections.reduce((sum, sec) => sum + sec.coverage.total, 0) || 0;
  const runDone =
    s.run?.sections.reduce(
      (sum, sec) =>
        sum +
        sec.coverage.succeeded +
        sec.coverage.partial +
        sec.coverage.failed +
        sec.coverage.skipped +
        sec.coverage.unavailable,
      0,
    ) || 0;
  const runStatus = results?.progress.status || s.run?.status;

  return (
    <div className="workspace-page results-page">
      <Heading
        variant="intro"
        title="Issues & Feature Ideas"
        detail={
          runStatus ? (
            <span className="row wrap">
              <span className="results-state">
                <StatusDot value={runStatus} label="Analysis" />
                <span>{human(runStatus)}</span>
              </span>
              <span>
                {filtered.length} {filtered.length === 1 ? 'issue' : 'issues'} ·{' '}
                {featureRows.length} feature ideas
              </span>
            </span>
          ) : undefined
        }
      >
        <Button
          icon="refresh"
          disabled={!!s.busy}
          onClick={() =>
            void w.act('Refresh results', async () => {
              await w.refreshProject();
              await Promise.all([
                w.loadResults('bugs'),
                w.loadResults('security'),
                w.loadResults('performance'),
              ]);
              await w.loadFeatures();
            })
          }
        >
          Refresh
        </Button>
        {s.run && (
          <Button
            icon="activity"
            disabled={!!s.busy}
            onClick={() => void w.navigate('analysis-run')}
          >
            Last Run
          </Button>
        )}
        {pausedOrInterrupted ? (
          <>
            <Button
              icon="play"
              tone="primary"
              disabled={!!s.busy}
              onClick={() => void w.resumeAnalysisDirect()}
            >
              Resume Analysis
            </Button>
            <Button
              icon="refresh"
              disabled={!!s.busy}
              onClick={() => void w.runAnalysisDirect(true)}
            >
              New Analysis
            </Button>
          </>
        ) : (
          <Button
            icon="play"
            tone="primary"
            disabled={!!s.busy || activeRun(s.run)}
            loading={activeRun(s.run)}
            onClick={() => void w.runAnalysisDirect(true)}
          >
            {activeRun(s.run) ? 'Analyzing…' : 'Run Analysis'}
          </Button>
        )}
      </Heading>
      {results?.retained_files?.length ? (
        <Notice>
          Retained results from {results.retained_files.length} files outside this run.
        </Notice>
      ) : null}
      {s.run?.status === 'stale' && (
        <Notice>Unchanged files keep saved analysis; reanalyze stale files to refresh.</Notice>
      )}
      {(activeRun(s.run) || pausedOrInterrupted) && (
        <div
          className="analysis-live-banner"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: '16px',
            padding: '12px 18px',
            background: 'var(--panel)',
            border: '1px solid var(--line)',
            borderLeft: '4px solid var(--accent)',
            borderRadius: '6px',
            marginBottom: '16px',
          }}
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', flex: 1 }}>
            <div className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
              {activeRun(s.run) ? (
                <>
                  <Spinner />
                  <strong style={{ fontSize: '0.88rem' }}>Analyzing project…</strong>
                </>
              ) : (
                <>
                  <StatusDot value={s.run?.status} label="Analysis" />
                  <strong style={{ fontSize: '0.88rem' }}>
                    Analysis {human(s.run?.status || '')}
                  </strong>
                </>
              )}
              <span className="small muted font-mono">
                {runTotal > 0 ? `${runDone} of ${runTotal} steps · ` : ''}
                {s.run?.window_files_completed ?? 0} files completed this batch
              </span>
              {s.run?.reason && <span className="small muted">· {s.run.reason}</span>}
            </div>
            <progress
              className="analysis-progress-bar"
              style={{ width: '100%', height: '5px', marginTop: '4px' }}
              value={runDone}
              max={runTotal || 100}
            />
          </div>
          <div className="actions" style={{ flexShrink: 0 }}>
            {pausedOrInterrupted ? (
              <>
                <Button
                  tone="primary small"
                  icon="play"
                  disabled={!!s.busy}
                  onClick={() => void w.resumeAnalysisDirect()}
                >
                  Resume Analysis
                </Button>
                <Button
                  tone="ghost small"
                  icon="refresh"
                  disabled={!!s.busy}
                  onClick={() => void w.runAnalysisDirect(true)}
                >
                  New Analysis
                </Button>
                <Button
                  tone="ghost small"
                  disabled={!!s.busy}
                  onClick={() => void w.navigate('analysis-run')}
                >
                  Last Run
                </Button>
              </>
            ) : (
              <>
                <Button
                  tone="ghost small"
                  disabled={!!s.busy}
                  onClick={() => void w.navigate('analysis-run')}
                >
                  Last Run
                </Button>
                {['running', 'queued'].includes(s.run?.status || '') && (
                  <Button
                    tone="ghost small"
                    disabled={!!s.busy}
                    onClick={() => void w.controlRun('pause')}
                  >
                    Pause
                  </Button>
                )}
                <Button
                  tone="ghost small"
                  disabled={!!s.busy || s.run?.status === 'canceling'}
                  onClick={() => void w.controlRun('cancel')}
                >
                  Cancel
                </Button>
              </>
            )}
          </div>
        </div>
      )}
      <div className="issue-split-grid">
        <Panel title="Issues & Ideas" className="results-findings">
          <div className="toolbar findings-toolbar">
            {!showFeatures && pageRows.length > 0 && (
              <label
                className="checkbox-filter select-all-issues"
                title="Select or deselect all issues on this page"
              >
                <input
                  type="checkbox"
                  aria-label="Select all issues on page"
                  checked={allPageSelected}
                  onChange={(e) => {
                    const pageKeys = pageRows.map((r) => r.key);
                    if (e.target.checked) {
                      setSelectedKeys((prev) => [...new Set([...prev, ...pageKeys])]);
                    } else {
                      setSelectedKeys((prev) => prev.filter((k) => !pageKeys.includes(k)));
                    }
                  }}
                />
                <span>All</span>
              </label>
            )}
            <div
              className="category-pills"
              style={{ display: 'inline-flex', gap: '4px', flexWrap: 'wrap' }}
            >
              {(['all', 'bugs', 'security', 'performance'] as const).map((cat) => (
                <Button
                  key={cat}
                  tone={
                    !showFeatures && !inProgressOnly && categoryFilter === cat
                      ? 'primary small'
                      : 'ghost small'
                  }
                  onClick={() => {
                    if (s.page === 'features') void w.navigate('bugs');
                    setInProgressOnly(false);
                    setCategoryFilter(cat);
                    updateView({ page: 1 });
                  }}
                >
                  <span>
                    {cat === 'all' ? 'All' : human(cat)[0].toUpperCase() + human(cat).slice(1)}
                  </span>
                  <span
                    style={{
                      marginLeft: '4px',
                      fontSize: '11px',
                      opacity: !inProgressOnly && categoryFilter === cat ? 0.9 : 0.6,
                      fontVariantNumeric: 'tabular-nums',
                    }}
                  >
                    ({categoryCounts[cat]})
                  </span>
                </Button>
              ))}
              <Button
                tone={!showFeatures && inProgressOnly ? 'primary small' : 'ghost small'}
                aria-pressed={!showFeatures && inProgressOnly}
                onClick={() => {
                  if (s.page === 'features') void w.navigate('bugs');
                  setCategoryFilter('all');
                  setShowDone(false);
                  setInProgressOnly((prev) => !prev);
                  updateView({ page: 1 });
                }}
              >
                <span>In Progress ({inProgressCount})</span>
              </Button>
              <Button
                tone={showFeatures ? 'primary small' : 'ghost small'}
                onClick={() => {
                  setInProgressOnly(false);
                  setCategoryFilter('features');
                }}
              >
                <span>Features ({featureRows.length})</span>
              </Button>
            </div>
            <div className="input-wrap findings-search">
              <Icon name="search" />
              <input
                ref={search}
                aria-label="Filter issues"
                value={query}
                onChange={(e) => updateView({ query: e.target.value, page: 1 })}
                onKeyDown={(e) => {
                  if (e.key === 'Escape' && query) {
                    e.preventDefault();
                    clearSearch();
                  }
                }}
                placeholder="Filter issues, files, symbols…"
              />
              {query && (
                <Button
                  className="findings-search-clear"
                  tone="ghost"
                  icon="close"
                  aria-label="Clear issue search"
                  title="Clear search (Esc)"
                  onClick={clearSearch}
                />
              )}
            </div>
            <div className="priority-filter" data-active={!!severity}>
              <span className="priority-filter-icon" aria-hidden="true">
                {severity ? <PriorityIcon value={severity} /> : <Icon name="filter" />}
              </span>
              <select
                aria-label="Issue priority"
                value={severity}
                onChange={(e) => updateView({ severity: e.target.value, page: 1 })}
              >
                <option value="">All priorities</option>
                {priorities.map((value) => (
                  <option key={value} value={value}>
                    {human(value)[0].toUpperCase() + human(value).slice(1)}
                  </option>
                ))}
              </select>
              <Icon name="down" />
            </div>
            {!showFeatures && (
              <div className="priority-filter" data-active={sortBy !== 'default'}>
                <span className="priority-filter-icon" aria-hidden="true">
                  <Icon name="layers" />
                </span>
                <select
                  aria-label="Sort issues"
                  value={sortBy}
                  onChange={(e) =>
                    setSortBy(e.target.value as 'default' | 'severity' | 'path' | 'category')
                  }
                >
                  <option value="default">Sort: Default</option>
                  <option value="severity">Sort: Severity</option>
                  <option value="path">Sort: File Path</option>
                  <option value="category">Sort: Category</option>
                </select>
                <Icon name="down" />
              </div>
            )}
            <label
              className="checkbox-filter"
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '6px',
                fontSize: '12px',
                cursor: 'pointer',
                userSelect: 'none',
                marginLeft: '8px',
              }}
            >
              <input
                type="checkbox"
                checked={showDone}
                onChange={(e) => {
                  if (e.target.checked) setInProgressOnly(false);
                  setShowDone(e.target.checked);
                  updateView({ page: 1 });
                }}
              />
              <span>Show Done{closedCount > 0 ? ` (${closedCount})` : ''}</span>
            </label>
          </div>
          {!showFeatures && selectedRows.length > 0 && (
            <div
              className="batch-findings-bar row between wrap"
              role="region"
              aria-label="Batch issue actions"
              style={{
                padding: '8px 12px',
                marginBottom: '8px',
                borderRadius: '6px',
                border: '1px solid var(--accent)',
                background: 'var(--panel)',
                gap: '8px',
              }}
            >
              <span className="small">
                <strong>{selectedRows.length}</strong> selected (
                {[...new Set(selectedRows.map((r) => r.path))].slice(0, 8).length} files)
              </span>
              <div className="actions" style={{ gap: '6px' }}>
                <Button
                  tone="primary small"
                  icon="sparkles"
                  disabled={!!s.busy || selectedRows.some((r) => r.freshness === 'stale')}
                  onClick={() => triggerBatchFix(selectedRows)}
                >
                  Fix Selected ({selectedRows.length})
                </Button>
                <Button
                  tone="danger small"
                  icon="trash"
                  disabled={!!s.busy}
                  onClick={() => {
                    const targets = selectedRows.map((r) => r.finding || r.target);
                    setSelectedKeys([]);
                    void w.batchTriage(targets, 'dismissed');
                  }}
                >
                  Dismiss
                </Button>
                <Button tone="ghost small" onClick={() => setSelectedKeys([])}>
                  Clear
                </Button>
              </div>
            </div>
          )}
          {showFeatures && (
            <Disclosure title="Project goals & Feature discovery">
              <div className="stack" style={{ gap: '8px', padding: '8px 0' }}>
                <textarea
                  aria-label="Project goals"
                  rows={2}
                  value={goals}
                  onChange={(e) => setGoals(e.target.value)}
                  placeholder="Describe project goals or constraints to guide feature discovery…"
                />
                <div className="actions">
                  <Button
                    tone="ghost small"
                    disabled={!!s.busy}
                    onClick={() => void w.updateFeatures(goals)}
                  >
                    Save goals
                  </Button>
                  <Button
                    tone="primary small"
                    icon="sparkles"
                    disabled={!!s.busy}
                    onClick={() => void w.searchFeatures('Features', goals)}
                  >
                    Suggest features
                  </Button>
                </div>
              </div>
            </Disclosure>
          )}
          <div className="result-list">
            {pageRows.map((row) => {
              const rowCategory = row.category || category;
              const rowAgent = !showFeatures ? findAgentForRow(s, row) : undefined;
              return (
                <div
                  className={`result-row ${inspectorRow?.key === row.key ? 'active' : ''}`}
                  key={row.key}
                  onClick={() => {
                    setSelectedInspectorKey(row.key);
                  }}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      setSelectedInspectorKey(row.key);
                    }
                  }}
                >
                  {!showFeatures && (
                    <input
                      type="checkbox"
                      aria-label={`Select ${row.name}`}
                      checked={selectedKeys.includes(row.key)}
                      onClick={(e) => e.stopPropagation()}
                      onChange={() => toggleRowSelection(row.key)}
                      style={{ cursor: 'pointer', flexShrink: 0 }}
                    />
                  )}
                  <span
                    className={`finding-mark ${['high', 'critical'].includes(row.severity) ? 'red' : row.severity === 'low' ? 'blue' : ''}`}
                  >
                    <Icon
                      name={
                        showFeatures
                          ? 'sparkles'
                          : rowCategory === 'security'
                            ? 'shield'
                            : rowCategory === 'performance'
                              ? 'gauge'
                              : 'bug'
                      }
                    />
                  </span>
                  <span className="list-copy">
                    <span className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
                      <span
                        className={`severity-tag ${['high', 'critical'].includes(row.severity) ? 'danger' : row.severity === 'medium' ? 'warning' : 'info'}`}
                      >
                        {showFeatures
                          ? 'IDEA'
                          : ['high', 'critical'].includes(row.severity)
                            ? 'HIGH'
                            : row.severity === 'medium'
                              ? 'MED'
                              : 'LOW'}
                      </span>
                      <strong>{row.name}</strong>
                      {row.status === 'fixed' && (
                        <Badge
                          value={row.commitOid ? `Done · #${row.commitOid.slice(0, 7)}` : 'Done'}
                          tone="green"
                        />
                      )}
                      {row.status === 'dismissed' && <Badge value="Dismissed" />}
                      {row.status !== 'fixed' && rowAgent && (
                        <>
                          <Badge
                            value={
                              rowAgent.status === 'committed'
                                ? `Committed in ${rowAgent.branch}`
                                : `Step ${rowAgent.stepNumber}/4 · ${rowAgent.stageLabel}`
                            }
                            tone={
                              rowAgent.status === 'committed'
                                ? 'green'
                                : rowAgent.status === 'running'
                                  ? 'violet'
                                  : 'amber'
                            }
                          />
                          {rowAgent.hasProblem && <Badge value="Needs Attention" tone="red" />}
                        </>
                      )}
                    </span>
                    <small>
                      <span className="path font-mono">
                        {row.path}
                        {row.line ? `:${row.line}` : ''}
                      </span>
                    </small>
                  </span>
                  <div className="finding-inline-actions" style={{ display: 'flex', gap: '6px' }}>
                    {!showFeatures &&
                    rowAgent &&
                    ['running', 'review', 'committed'].includes(rowAgent.status) ? (
                      <Button
                        tone="primary small"
                        icon="sparkles"
                        onClick={(e) => {
                          e.stopPropagation();
                          void w.selectAgent(rowAgent.path);
                        }}
                      >
                        Open Agent
                      </Button>
                    ) : (
                      <Button
                        tone="primary small"
                        icon="sparkles"
                        disabled={!!s.busy || row.freshness === 'stale'}
                        onClick={(e) => {
                          e.stopPropagation();
                          if (showFeatures) {
                            const idea = s.features?.suggestions.find(
                              (item) => `feature:${item.id}` === row.key,
                            );
                            if (idea) w.discussFeature(idea, false);
                          } else {
                            triggerFix(row, 'agents');
                          }
                        }}
                      >
                        Fix with Agent
                      </Button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
          {!showFeatures && !filtered.length && (
            <Empty
              title={
                inProgressOnly
                  ? 'No issues in progress'
                  : unique.length
                    ? 'No matching issues'
                    : s.run && !results
                      ? 'Results not loaded'
                      : 'No issues found'
              }
              detail={
                inProgressOnly
                  ? 'Select one or more open issues and click Fix with Agent to dispatch an agent.'
                  : unique.length
                    ? 'Try clearing the search or priority filter.'
                    : results
                      ? `Run state: ${human(results.progress.status)}. All analyzed files healthy.`
                      : 'Run an analysis to scan the project.'
              }
            >
              {inProgressOnly ? (
                <Button tone="ghost" icon="undo" onClick={() => setInProgressOnly(false)}>
                  Show Open Issues
                </Button>
              ) : unique.length ? (
                <Button tone="ghost" icon="undo" onClick={clearSearch}>
                  Clear filters
                </Button>
              ) : (
                <Button tone="primary" icon="play" onClick={() => void w.runAnalysisDirect()}>
                  Run analysis
                </Button>
              )}
            </Empty>
          )}
          <nav className="findings-pagination" aria-label="Findings pagination">
            <div className="pagination-summary small muted" role="status">
              Page {page} of {pageCount}
            </div>
            <div className="pagination-pages">
              <Button
                tone="ghost"
                icon="chevronLeft"
                aria-label="Previous page"
                title="Previous page"
                disabled={page === 1}
                onClick={() => updateView({ page: page - 1 })}
              />
              {pageNumbers.map((number, index) => (
                <Fragment key={number}>
                  {index > 0 && number > pageNumbers[index - 1] + 1 && (
                    <span className="pagination-gap" aria-hidden="true">
                      …
                    </span>
                  )}
                  <Button
                    tone="ghost"
                    aria-label={`Page ${number}`}
                    aria-current={number === page ? 'page' : undefined}
                    onClick={() => updateView({ page: number })}
                  >
                    {number}
                  </Button>
                </Fragment>
              ))}
              <Button
                tone="ghost"
                icon="chevron"
                aria-label="Next page"
                title="Next page"
                disabled={page === pageCount}
                onClick={() => updateView({ page: page + 1 })}
              />
            </div>
          </nav>
        </Panel>

        <Panel
          title="Selected Issue Inspector"
          className="results-inspector-panel"
          actions={<span className="small muted font-mono">j/k navigate · x select</span>}
        >
          {inspectorRow ? (
            <div className="stack">
              <div className="row between wrap">
                <div className="row wrap" style={{ gap: '6px' }}>
                  <Badge value={inspectorRow.kind} tone="violet" />
                  {inspectorRow.status && (
                    <Badge
                      value={
                        inspectorRow.status === 'fixed'
                          ? inspectorRow.commitOid
                            ? `Done · #${inspectorRow.commitOid.slice(0, 7)}`
                            : 'Done'
                          : inspectorRow.status
                      }
                      tone={inspectorRow.status === 'fixed' ? 'green' : undefined}
                    />
                  )}
                </div>
                {!inspectorRow.featureId && (
                  <div className="actions finding-actions">
                    <Button
                      tone={inspectorRow.status === 'dismissed' ? 'ghost small' : 'danger small'}
                      icon={inspectorRow.status === 'dismissed' ? 'undo' : 'trash'}
                      disabled={!!s.busy}
                      onClick={() =>
                        void w.triage(
                          inspectorRow.finding || inspectorRow.target,
                          inspectorRow.status === 'dismissed' ? 'open' : 'dismissed',
                        )
                      }
                    >
                      {inspectorRow.status === 'dismissed' ? 'Reopen' : 'Delete'}
                    </Button>
                  </div>
                )}
                {inspectorRow.featureId && (
                  <div className="actions finding-actions">
                    <Button
                      tone="ghost small"
                      icon="check"
                      disabled={!!s.busy || inspectorRow.status === 'saved'}
                      onClick={() => void w.setFeatureStatus(inspectorRow.featureId!, 'saved')}
                    >
                      Save idea
                    </Button>
                    <Button
                      tone="danger small"
                      icon="trash"
                      disabled={!!s.busy || inspectorRow.status === 'dismissed'}
                      onClick={() => void w.setFeatureStatus(inspectorRow.featureId!, 'dismissed')}
                    >
                      Dismiss
                    </Button>
                  </div>
                )}
              </div>
              <div>
                <h3 style={{ marginTop: '4px' }}>{inspectorRow.name}</h3>
                <p className="small muted font-mono" style={{ marginTop: '4px' }}>
                  {inspectorRow.path}
                  {inspectorRow.line ? `:${inspectorRow.line}` : ''}
                </p>
              </div>
              {(() => {
                const inspectorAgent = !showFeatures ? findAgentForRow(s, inspectorRow) : undefined;
                if (!inspectorAgent) return null;
                return (
                  <div
                    className="row between wrap"
                    style={{
                      padding: '10px 12px',
                      borderRadius: '8px',
                      border: `1px solid ${inspectorAgent.hasProblem ? 'var(--red, #e5534b)' : 'var(--accent)'}`,
                      background: 'var(--bg-elevated)',
                      gap: '8px',
                    }}
                  >
                    <div className="stack" style={{ gap: '2px' }}>
                      <div className="row wrap" style={{ gap: '6px', alignItems: 'center' }}>
                        <Badge value={inspectorAgent.branch} tone="violet" />
                        <span className="small">
                          <strong>
                            Step {inspectorAgent.stepNumber}/4 · {inspectorAgent.stageLabel}
                          </strong>
                        </span>
                        {inspectorAgent.hasProblem && <Badge value="Needs Attention" tone="red" />}
                        {inspectorAgent.commitOid && (
                          <span className="small font-mono muted">
                            #{inspectorAgent.commitOid.slice(0, 7)}
                          </span>
                        )}
                      </div>
                      <span className="small muted">
                        {inspectorAgent.hasProblem
                          ? inspectorAgent.problemSummary
                          : inspectorAgent.status === 'committed'
                            ? `Committed on worktree branch ${inspectorAgent.branch}. Merge into ${s.repository?.branch || 'main'} to mark Done.`
                            : 'An agent is working on this issue.'}
                      </span>
                    </div>
                    <div className="actions" style={{ gap: '6px' }}>
                      <Button
                        tone="ghost small"
                        icon="sparkles"
                        onClick={() => void w.selectAgent(inspectorAgent.path)}
                      >
                        Open Agent
                      </Button>
                      {inspectorAgent.mergeable && inspectorAgent.path && (
                        <Button
                          tone="primary small"
                          icon="check"
                          disabled={!!s.busy}
                          onClick={() => void w.mergeAgentWorktree(inspectorAgent.path!)}
                        >
                          Merge &amp; Close
                        </Button>
                      )}
                    </div>
                  </div>
                );
              })()}
              <div>
                <strong className="small uppercase muted">Cause / Context</strong>
                <Prose text={inspectorRow.cause || 'No cause recorded.'} />
              </div>
              <div>
                <strong className="small uppercase muted">Proposed Fix / Criteria</strong>
                <Prose text={inspectorRow.solution || 'Prepare a fix to generate diff hunks.'} />
              </div>
              {(inspectorRow.text.some(([, content]) => content) || inspectorRow.insight) && (
                <Disclosure title="Extended Evidence & Insight">
                  <div className="stack" style={{ gap: '8px', paddingTop: '6px' }}>
                    {inspectorRow.text
                      .filter(([, content]) => content)
                      .map(([title, content]) => (
                        <div key={title}>
                          <strong className="small uppercase muted">{title}</strong>
                          <Prose text={content} />
                        </div>
                      ))}
                    <InsightCard insight={inspectorRow.insight} />
                  </div>
                </Disclosure>
              )}
              <div className="actions">
                {!showFeatures ? (
                  <Button
                    tone="primary"
                    icon="sparkles"
                    disabled={!!s.busy}
                    onClick={() => triggerFix(inspectorRow, 'agents')}
                  >
                    Fix with Agent
                  </Button>
                ) : (
                  <Button
                    tone="primary"
                    icon="sparkles"
                    disabled={!!s.busy}
                    onClick={() => {
                      const idea = s.features?.suggestions.find(
                        (item) => `feature:${item.id}` === inspectorRow.key,
                      );
                      if (idea) w.discussFeature(idea, false);
                    }}
                  >
                    Generate &amp; Review Diff
                  </Button>
                )}
                {inspectorRow.path && (
                  <Button
                    tone="ghost"
                    icon="code"
                    onClick={() =>
                      void w.openFile(inspectorRow.path.split(',')[0].trim(), inspectorRow.symbol)
                    }
                  >
                    Open File
                  </Button>
                )}
                {!showFeatures && (
                  <Button
                    tone="ghost"
                    onClick={() =>
                      void w.openFinding(inspectorRow.category || category, inspectorRow.key)
                    }
                  >
                    Full Details
                  </Button>
                )}
              </div>
            </div>
          ) : (
            <Empty
              title="Select an issue or feature idea"
              detail="Click any row on the left to inspect its cause and generate a diff in Code & Diff."
              icon="bug"
            />
          )}
        </Panel>
      </div>
      <Disclosure
        title={
          <>
            <span>Analysis details</span>
            {reports.length > 0 && (
              <span className="small muted">
                {' · '}
                {reports.length} report notices (
                {[...new Set(reports.map((r) => human(r.status)))].join(', ')})
              </span>
            )}
            {!!results?.unclassified?.length && (
              <span className="small muted">
                {' '}
                · {results.unclassified.length} unclassified suggestions
              </span>
            )}
          </>
        }
      >
        {!reports.length && !results?.unclassified?.length && (
          <p className="muted">No additional report diagnostics are loaded.</p>
        )}
        {reports.map((report, index) => (
          <div className="section-gap" key={`${report.key}:${index}`}>
            <div className="row wrap">
              <span className="path font-mono">{report.path}</span>
              <span className="results-state">
                <StatusDot value={report.status} />
                <span>{human(report.status)}</span>
              </span>
            </div>
            {report.reason && <Prose text={report.reason} />}
          </div>
        ))}
        {!!results?.unclassified?.length && (
          <Panel title="Unclassified suggestions">
            {results.unclassified.map((f) => (
              <Disclosure key={f.id} title={f.title}>
                <Prose text={f.message} />
                <Button onClick={() => void w.openFile(f.location.path, f.location.symbol)}>
                  Go to file
                </Button>
              </Disclosure>
            ))}
          </Panel>
        )}
      </Disclosure>
    </div>
  );
}
