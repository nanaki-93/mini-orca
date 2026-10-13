import { Fragment, useEffect, useRef, useState } from 'react';
import type * as M from './models';
import {
  workspace as w,
  activeChangeWorkflow,
  findingCategory,
  workflowSeed,
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
import { defaultWorkflowModels } from './change-workflow';
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

interface ResultRow {
  key: string;
  title: string;
  name: string;
  path: string;
  symbol: string;
  line: number;
  severity: string;
  confidence: string;
  freshness: string;
  status: string;
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
  const rows = results
    ? [
        ...(results.semantic || []).map(semantic),
        ...(results.performance || []).flatMap(performance),
        ...(results.security || []).flatMap(security),
      ]
    : (s.findings || []).filter((f) => f.category === category).map(semantic);
  if (category === 'security' && s.securityReport) rows.push(...security(s.securityReport));
  return [...new Map(rows.map((row) => [row.key, row])).values()].map((row) => ({
    ...row,
    status: s.completedFindings?.some(
      (target) =>
        target.project_id === row.target.project_id &&
        target.kind === row.target.kind &&
        target.id === row.target.id &&
        target.path === row.target.path &&
        (target.source || '') === (row.target.source || ''),
    )
      ? 'fixed'
      : row.status,
  }));
}
export function selectedResult(s: State) {
  const selected = s.selectedFinding;
  return selected && resultRows(s, selected.category).find((row) => row.key === selected.key);
}
const pageSize = 10;
export function Results({ s }: { s: State }) {
  const [categoryFilter, setCategoryFilter] = useState<'all' | FindingCategory>('all');
  const [showDone, setShowDone] = useState(false);
  const currentCategory = findingCategory(s) || 'bugs';
  const category = categoryFilter === 'all' ? currentCategory : categoryFilter;
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
  const countBugs = bugRows.filter((row) => (showDone ? true : row.status !== 'fixed')).length;
  const countSec = secRows.filter((row) => (showDone ? true : row.status !== 'fixed')).length;
  const countPerf = perfRows.filter((row) => (showDone ? true : row.status !== 'fixed')).length;
  const countAll = [...new Map(allRows.map((r) => [r.key, r])).values()].filter((row) =>
    showDone ? true : row.status !== 'fixed',
  ).length;
  const categoryCounts: Record<'all' | FindingCategory, number> = {
    all: countAll,
    bugs: countBugs,
    security: countSec,
    performance: countPerf,
  };
  const unique =
    categoryFilter === 'all'
      ? [...new Map(allRows.map((row) => [row.key, row])).values()]
      : resultRows(s, categoryFilter);
  const filtered = unique.filter(
    (row) =>
      (!showDone ? row.status !== 'fixed' : true) &&
      `${row.name} ${row.title} ${row.path} ${row.symbol}`
        .toLowerCase()
        .includes(query.toLowerCase()) &&
      (!severity || row.severity === severity),
  );
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
    kind,
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
  const triggerFix = (row: ResultRow) => {
    const preparedSeed = workflowSeed(fixSeed(row));
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
  const seed = detail ? workflowSeed(fixSeed(detail)) : undefined;
  const fixDisabled = !!s.busy || activeChangeWorkflow(s.change) || detail?.freshness === 'stale';
  const detailPanel = detail ? (
    <div className="workspace-page results-page results-detail guided-fix" data-accent={category}>
      <div className="row between wrap fix-back">
        <Button icon="back" onClick={() => void w.navigate(category)}>
          All findings
        </Button>
        {detail.finding && (
          <div className="actions finding-actions">
            <Button
              icon={detail.status === 'dismissed' ? 'undo' : 'trash'}
              tone={detail.status === 'dismissed' ? 'default' : 'danger'}
              disabled={!!s.busy}
              onClick={() =>
                void w.triage(detail.finding!, detail.status === 'dismissed' ? 'open' : 'dismissed')
              }
            >
              {detail.status === 'dismissed' ? 'Reopen' : 'Delete'}
            </Button>
            <Button
              icon="check"
              disabled={!!s.busy || detail.status === 'fixed'}
              onClick={() => void w.triage(detail.finding!, 'fixed')}
            >
              Mark as Done
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
  return (
    <div className="workspace-page results-page">
      <Heading
        variant="intro"
        title={
          categoryFilter === 'all'
            ? 'Issues'
            : `${human(categoryFilter)[0].toUpperCase() + human(categoryFilter).slice(1)} Issues`
        }
        detail={
          results ? (
            <span className="row wrap">
              <span className="results-state">
                <StatusDot value={results.progress.status} label="Analysis" />
                <span>{human(results.progress.status)}</span>
              </span>
              <span>
                {filtered.length} {filtered.length === 1 ? 'issue' : 'issues'}
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
              await w.loadResults(category);
            })
          }
        >
          Refresh
        </Button>
        <Button
          icon="play"
          tone="primary"
          disabled={!!s.busy || activeRun(s.run)}
          loading={activeRun(s.run)}
          onClick={() => void w.runAnalysisDirect()}
        >
          {activeRun(s.run) ? 'Analyzing…' : 'Run Analysis'}
        </Button>
      </Heading>
      {results?.retained_files?.length ? (
        <Notice>
          Retained results from {results.retained_files.length} files outside this run.
        </Notice>
      ) : null}
      {s.run?.status === 'stale' && (
        <Notice>Unchanged files keep saved analysis; reanalyze stale files to refresh.</Notice>
      )}
      {activeRun(s.run) && (
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
              <Spinner />
              <strong style={{ fontSize: '0.88rem' }}>Analyzing project…</strong>
              <span className="small muted font-mono">
                {s.run?.window_files_completed ?? 0} files completed
              </span>
            </div>
            <progress
              className="analysis-progress-bar"
              style={{ width: '100%', height: '5px', marginTop: '4px' }}
              value={
                s.run?.sections.reduce(
                  (sum, sec) =>
                    sum + sec.coverage.succeeded + sec.coverage.failed + sec.coverage.partial,
                  0,
                ) || 0
              }
              max={s.run?.sections.reduce((sum, sec) => sum + sec.coverage.total, 0) || 100}
            />
          </div>
          <div className="actions" style={{ flexShrink: 0 }}>
            <Button
              tone="ghost small"
              disabled={!!s.busy || s.run?.status === 'canceling'}
              onClick={() => void w.controlRun('cancel')}
            >
              Cancel
            </Button>
          </div>
        </div>
      )}
      <Panel title="Issues" className="results-findings">
        <div className="toolbar findings-toolbar">
          <div className="category-pills" style={{ display: 'inline-flex', gap: '4px' }}>
            {(['all', 'bugs', 'security', 'performance'] as const).map((cat) => (
              <Button
                key={cat}
                tone={categoryFilter === cat ? 'primary small' : 'ghost small'}
                onClick={() => {
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
                    opacity: categoryFilter === cat ? 0.9 : 0.6,
                    fontVariantNumeric: 'tabular-nums',
                  }}
                >
                  ({categoryCounts[cat]})
                </span>
              </Button>
            ))}
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
                setShowDone(e.target.checked);
                updateView({ page: 1 });
              }}
            />
            <span>Show Done</span>
          </label>
          {(query || severity) && (
            <Button
              tone="ghost small"
              icon="undo"
              onClick={() => {
                updateView({ query: '', severity: '', page: 1 });
                search.current?.focus();
              }}
            >
              Reset
            </Button>
          )}
        </div>
        <div className="findings-columns" aria-hidden="true">
          <span>Issue</span>
          <span>Location</span>
          <span>Status</span>
        </div>
        <div className="result-list">
          {filtered.slice(offset, offset + pageSize).map((row) => (
            <div
              className="result-row"
              key={row.key}
              onClick={() => w.openFinding(category, row.key)}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  w.openFinding(category, row.key);
                }
              }}
            >
              <span
                className={`finding-mark ${['high', 'critical'].includes(row.severity) ? 'red' : row.severity === 'low' ? 'blue' : ''}`}
              >
                <Icon
                  name={
                    category === 'security'
                      ? 'shield'
                      : category === 'performance'
                        ? 'gauge'
                        : 'bug'
                  }
                />
              </span>
              <span className="list-copy">
                <span className="row" style={{ gap: '8px', alignItems: 'center' }}>
                  <span
                    className={`severity-tag ${['high', 'critical'].includes(row.severity) ? 'danger' : row.severity === 'medium' ? 'warning' : 'info'}`}
                  >
                    {['high', 'critical'].includes(row.severity)
                      ? 'HIGH'
                      : row.severity === 'medium'
                        ? 'MED'
                        : 'LOW'}
                  </span>
                  <strong>{row.name}</strong>
                </span>
                <span className="finding-summary">{row.cause.split('\n')[0]}</span>
                <small>
                  <span className="path font-mono">
                    {row.path}
                    {row.line ? `:${row.line}` : ''}
                  </span>
                </small>
              </span>
              <span className="result-badges">
                {row.status === 'fixed' && <Badge value="Done" tone="green" />}
                <PriorityIcon value={row.severity} />
                <span className="results-state">
                  <StatusDot value={row.freshness} label="Freshness" />
                </span>
              </span>
              <div className="finding-inline-actions">
                <Button
                  tone="primary small"
                  disabled={!!s.busy || activeChangeWorkflow(s.change) || row.freshness === 'stale'}
                  onClick={(e) => {
                    e.stopPropagation();
                    triggerFix(row);
                  }}
                >
                  Fix
                </Button>
                <Button
                  tone="ghost small"
                  onClick={(e) => {
                    e.stopPropagation();
                    void w.openFile(row.path, row.symbol);
                  }}
                >
                  File
                </Button>
              </div>
            </div>
          ))}
        </div>
        {!filtered.length && (
          <Empty
            title={
              unique.length
                ? 'No matching issues'
                : s.run && !results
                  ? 'Results not loaded'
                  : 'No issues found'
            }
            detail={
              unique.length
                ? 'Try clearing the search or priority filter.'
                : results
                  ? `Run state: ${human(results.progress.status)}. All analyzed files healthy.`
                  : 'Run an analysis to scan the project.'
            }
          >
            {unique.length ? (
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
