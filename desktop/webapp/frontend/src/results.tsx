import { useEffect, useRef, useState } from 'react';
import type * as M from './models';
import { workspace as w, activeChangeWorkflow, workflowSeed, type State } from './workspace';
import { FixPreparation } from './fix-preparation';
import { SolutionExcerpt } from './fix-solution';
import { StaleAnalysisButton } from './analysis';
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
  StatusDot,
  human,
} from './ui';

interface ResultRow {
  key: string;
  title: string;
  path: string;
  symbol: string;
  line: number;
  severity: string;
  confidence: string;
  freshness: string;
  status: string;
  kind: string;
  insight?: M.Insight;
  text: [string, string][];
  finding?: M.Finding;
  task?: M.TaskSpec;
  cause: string;
  solution: string;
}
function semantic(f: M.Finding): ResultRow {
  return {
    key: `semantic:${f.id}`,
    title: f.title || f.message,
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
      'No solution yet. Prepare a fix to review the proposed changes.',
  };
}
function performance(report: M.PerformanceReport): ResultRow[] {
  return (report.findings || []).map((f) => ({
    key: `performance:${report.path}:${f.id}`,
    title: f.title,
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
    title: f.title,
    path: f.source_anchor.path || report.path,
    symbol: f.source_anchor.symbol,
    line: f.source_anchor.start_line,
    severity: f.severity,
    confidence: f.confidence,
    freshness: report.status,
    status: f.verification_state || f.triage,
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
export function Results({ s }: { s: State }) {
  const category = s.page;
  const results = s.results[category];
  const [query, setQuery] = useState('');
  const [severity, setSeverity] = useState('');
  const selected = s.selectedFinding?.category === category ? s.selectedFinding.key : undefined;
  const setSelected = (key?: string) =>
    w.set({ selectedFinding: key ? { category, key } : undefined });
  const [limit, setLimit] = useState(100);
  const view = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (selected) {
      view.current?.focus();
      view.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }
  }, [selected]);
  const rows = results
    ? [
        ...(results.semantic || []).map(semantic),
        ...(results.performance || []).flatMap(performance),
        ...(results.security || []).flatMap(security),
      ]
    : (s.findings || []).filter((f) => f.category === category).map(semantic);
  if (category === 'security' && s.securityReport) rows.push(...security(s.securityReport));
  const unique = [...new Map(rows.map((row) => [row.key, row])).values()];
  const filtered = unique.filter(
    (row) =>
      `${row.title} ${row.path} ${row.symbol}`.toLowerCase().includes(query.toLowerCase()) &&
      (!severity || row.severity === severity),
  );
  const detail = unique.find((row) => row.key === selected);
  const fixSeed = (row: ResultRow): M.ChangeSeed => ({
    title: row.title.slice(0, 200),
    paths: [row.path],
    kind: category === 'performance' ? 'performance' : category === 'security' ? 'security' : 'fix',
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
  const seed = detail ? workflowSeed(fixSeed(detail)) : undefined;
  const fixDisabled = !!s.busy || activeChangeWorkflow(s.change) || detail?.freshness === 'stale';
  const detailPanel = detail ? (
    <div ref={view} tabIndex={-1} className="workspace-page results-page results-detail guided-fix">
      <div className="row between wrap fix-back">
        <Button icon="back" onClick={() => setSelected(undefined)}>
          All findings
        </Button>
        <span className="small muted">Prepare a fix</span>
        <Go page="models" icon="layers">
          Manage models
        </Go>
      </div>
      <Heading
        variant="intro"
        title={detail.title}
        detail={
          <span className="results-detail-meta">
            <span className="path">
              {detail.path}
              {detail.line ? `:${detail.line}` : ''}
            </span>
            <span>{detail.symbol || 'File-level finding'}</span>
            <span className="row wrap">
              <Badge value={detail.severity} />
              <span aria-label="Finding state">
                <Badge value={detail.status} />
              </span>
              <span className="results-state">
                <StatusDot value={detail.freshness} label="Freshness" />
                <span>{human(detail.freshness)}</span>
              </span>
            </span>
          </span>
        }
      >
        <StaleAnalysisButton s={s} />
        <Button
          icon="code"
          disabled={!!s.busy}
          onClick={() => void w.openFile(detail.path, detail.symbol)}
        >
          Go to file
        </Button>
      </Heading>
      {detail.freshness === 'stale' && (
        <Notice>This finding is stale. Refresh its analysis before preparing a fix.</Notice>
      )}
      {category === 'performance' && (
        <Notice>Performance unmeasured; tests do not establish a speedup.</Notice>
      )}
      <Panel title="Cause" className="fix-explanation fix-cause">
        <Prose
          text={detail.cause || 'No cause was saved. Review the evidence before preparing a fix.'}
        />
        <p className="small muted">
          {human(detail.kind)} · Reported confidence: {human(detail.confidence)}
        </p>
      </Panel>
      <Panel title="Proposed solution" className="fix-explanation fix-solution">
        <SolutionExcerpt
          text={
            detail.solution ||
            'No remediation was supplied. Review the generated solution before Apply.'
          }
        />
      </Panel>
      {seed && detail.path && (
        <FixPreparation key={detail.key} s={s} seed={seed} disabled={fixDisabled} />
      )}
      <Disclosure title="Details">
        <div className="stack results-detail-layout">
          {detail.finding && (
            <Panel title="Finding actions">
              <div className="actions section-gap">
                <Button
                  disabled={!!s.busy}
                  onClick={() =>
                    void w.triage(
                      detail.finding!,
                      detail.status === 'dismissed' ? 'open' : 'dismissed',
                    )
                  }
                >
                  {detail.status === 'dismissed' ? 'Reopen' : 'Dismiss'}
                </Button>
                <Button
                  icon="check"
                  disabled={!!s.busy || detail.status === 'fixed'}
                  onClick={() => void w.triage(detail.finding!, 'fixed')}
                >
                  Mark as fixed
                </Button>
              </div>
            </Panel>
          )}
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
      </Disclosure>
    </div>
  ) : null;
  const reports = [
    ...(results?.performance || [])
      .filter((r) => r.warning || !(r.findings || []).length)
      .map((r) => ({
        key: `p:${r.path}`,
        path: r.path,
        status: r.status,
        reason: r.warning,
      })),
    ...(results?.security || [])
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
        title={human(category)[0].toUpperCase() + human(category).slice(1)}
        detail={
          results ? (
            <span className="row wrap">
              <span className="results-state">
                <StatusDot value={results.progress.status} label="Analysis" />
                <span>{human(results.progress.status)}</span>
              </span>
              <span>
                {results.saved_finding_count === null
                  ? 'Count unavailable'
                  : `${results.saved_finding_count} saved findings`}
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
        {category === 'bugs' && (
          <Go page="scan" icon="shield">
            Verified scan
          </Go>
        )}
        <Go page="analysis" tone="primary">
          Analyze project
        </Go>
        <StaleAnalysisButton s={s} />
      </Heading>
      {results?.retained_files?.length ? (
        <Notice>
          Includes retained results from {results.retained_files.length} files outside this run.
        </Notice>
      ) : null}
      {s.run?.status === 'stale' && (
        <Notice>
          Files are checked individually. Unchanged files keep their saved analysis; reanalyze the
          stale files to refresh them.
        </Notice>
      )}
      {category === 'security' && s.file && (
        <Panel title="Selected source" className="results-source">
          <div className="row between wrap">
            <span className="path results-source-path">{s.file.path}</span>
            <div className="actions">
              <Button
                disabled={!!s.busy || s.fileStale}
                onClick={() => void w.securityReview(false)}
              >
                Scan source
              </Button>
              <Button
                disabled={!!s.busy || s.fileStale}
                onClick={() => void w.securityReview(true)}
              >
                AI Security review
              </Button>
            </div>
          </div>
          {s.securityReport && (
            <div className="section-gap">
              <span className="results-state">
                <StatusDot value={s.securityReport.status} label="Security" />
                <span>{human(s.securityReport.status)}</span>
              </span>
              {s.securityReport.reason && (
                <p className="small section-gap">{s.securityReport.reason}</p>
              )}
            </div>
          )}
        </Panel>
      )}
      <Panel title="Findings" className="results-findings">
        <div className="toolbar">
          <div className="input-wrap">
            <Icon name="search" />
            <input
              aria-label="Filter findings"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setLimit(100);
              }}
              placeholder="Filter findings…"
            />
          </div>
          <select
            className="field compact"
            aria-label="Finding severity"
            value={severity}
            onChange={(e) => setSeverity(e.target.value)}
          >
            <option value="">All impact levels</option>
            {[...new Set(unique.map((row) => row.severity))].filter(Boolean).map((value) => (
              <option key={value} value={value}>
                {human(value)}
              </option>
            ))}
          </select>
        </div>
        <div className="findings-columns" aria-hidden="true">
          <span>Finding</span>
          <span>Impact / freshness</span>
          <span />
        </div>
        <div className="result-list">
          {filtered.slice(0, limit).map((row) => (
            <button
              className="result-row"
              key={row.key}
              aria-expanded={row.key === selected}
              onClick={() => setSelected(row.key === selected ? undefined : row.key)}
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
                <strong>{row.title}</strong>
                <span className="finding-summary">{row.cause.split('\n')[0]}</span>
                <small>
                  <span className="path">
                    {row.path}
                    {row.line ? `:${row.line}` : ''}
                  </span>
                </small>
              </span>
              <span className="result-badges">
                <Badge value={row.severity} />
                <span className="results-state">
                  <StatusDot value={row.freshness} label="Freshness" hideSuccess />
                  <span>{human(row.freshness)}</span>
                </span>
              </span>
              <span className="finding-open">
                <Icon name="chevron" />
              </span>
            </button>
          ))}
        </div>
        {!filtered.length && (
          <Empty
            title={
              unique.length
                ? 'No matching findings'
                : s.run && !results
                  ? 'Results not loaded'
                  : 'No saved findings'
            }
            detail={results ? `Run state: ${human(results.progress.status)}` : undefined}
          >
            <Go page="analysis">Open analysis</Go>
          </Empty>
        )}
        {filtered.length > limit && (
          <Button className="section-gap" onClick={() => setLimit(limit + 100)}>
            Show more
          </Button>
        )}
      </Panel>
      {detailPanel}
      {!!reports.length && (
        <section aria-label="Report summaries" className="grid equal-columns results-reports">
          {reports.map((report) => (
            <Panel
              key={report.key}
              title={report.path}
              actions={
                <span className="results-state">
                  <StatusDot value={report.status} />
                  <span>{human(report.status)}</span>
                </span>
              }
            >
              {report.reason && <Prose text={report.reason} />}
            </Panel>
          ))}
        </section>
      )}
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
    </div>
  );
}
