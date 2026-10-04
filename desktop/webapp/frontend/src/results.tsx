import { useState } from 'react';
import type * as M from './models';
import { workspace as w, type State } from './workspace';
import {
  Badge,
  Button,
  Disclosure,
  Empty,
  Go,
  Heading,
  Icon,
  InsightCard,
  KeyValues,
  Notice,
  Panel,
  Prose,
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
    kind: f.source,
    insight: f.engineering_insight,
    text: [
      ['Finding', f.message],
      ['Evidence', f.evidence],
    ],
    finding: f,
    task: f.task_spec,
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
    text: [
      ['Observed pattern', f.observed_pattern],
      ['Workload', f.workload_conditions],
      ['Recommendation', f.recommendation],
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
    text: [
      ['Observed condition', f.observed_condition],
      ['Evidence', f.evidence_kind],
      ['Preconditions & unknowns', f.preconditions_or_unknowns],
      ['Remediation', f.remediation],
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
  const [selected, setSelected] = useState<string>();
  const [limit, setLimit] = useState(100);
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
  const open = async (row: ResultRow, prepare: boolean) => {
    await w.openFile(row.path, row.symbol, row.task);
    if (prepare && w.state.file?.path === row.path && !w.state.fileStale)
      await w.navigate('assistant');
  };
  if (detail)
    return (
      <>
        <Heading
          title={detail.title}
          eyebrow={human(category)}
          detail={
            <span className="row wrap">
              <Badge value={detail.severity} />
              <Badge value={detail.confidence} tone="violet" />
              <Badge value={detail.freshness} />
            </span>
          }
        >
          <Button icon="back" onClick={() => setSelected(undefined)}>
            All findings
          </Button>
          <Button disabled={!!s.busy} onClick={() => void open(detail, false)}>
            Open source
          </Button>
          {detail.symbol && (
            <Button
              tone="primary"
              disabled={!!s.busy || detail.freshness === 'stale'}
              onClick={() => void open(detail, true)}
            >
              Prepare change
            </Button>
          )}
        </Heading>
        <div className="grid two-columns">
          <div className="stack">
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
                    <pre>{detail.task.go_test_candidate.content}</pre>
                  )}
                </Disclosure>
              </Panel>
            )}
          </div>
          <div className="stack">
            <Panel title="Source">
              <KeyValues
                values={[
                  ['File', detail.path],
                  ['Declaration', detail.symbol || 'File-level finding'],
                  ['Line', detail.line || '—'],
                  ['Source', human(detail.kind)],
                  ['State', <Badge value={detail.status} />],
                ]}
              />
              {detail.finding && (
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
                    disabled={!!s.busy || detail.status === 'fixed'}
                    onClick={() => void w.triage(detail.finding!, 'fixed')}
                  >
                    Mark fixed
                  </Button>
                </div>
              )}
            </Panel>
            <InsightCard insight={detail.insight} />
          </div>
        </div>
      </>
    );
  return (
    <>
      <Heading
        title={human(category)[0].toUpperCase() + human(category).slice(1)}
        eyebrow="Analysis results"
        detail={
          results ? (
            <span className="row">
              <Badge value={results.progress.status} />
              <span>
                {results.saved_finding_count === null
                  ? 'Count unavailable'
                  : `${results.saved_finding_count} saved findings`}
              </span>
            </span>
          ) : (
            'Saved project evidence'
          )
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
      </Heading>
      {results?.retained_files?.length ? (
        <Notice>
          Includes retained results from {results.retained_files.length} files outside this run.
        </Notice>
      ) : null}
      {category === 'security' && s.file && (
        <Panel className="section-bottom">
          <div className="row between wrap">
            <span className="mono small">{s.file.path}</span>
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
              <Badge value={s.securityReport.status} />
              {s.securityReport.reason && (
                <p className="small section-gap">{s.securityReport.reason}</p>
              )}
            </div>
          )}
        </Panel>
      )}
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
      <Panel>
        <div className="result-list">
          {filtered.slice(0, limit).map((row) => (
            <button className="result-row" key={row.key} onClick={() => setSelected(row.key)}>
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
                <small>
                  {row.path}
                  {row.line ? `:${row.line}` : ''} · {human(row.kind)}
                </small>
              </span>
              <span className="result-badges">
                <Badge value={row.severity} />
                <Badge value={row.confidence} tone="violet" />
                <Badge value={row.freshness} />
              </span>
              <Icon name="chevron" />
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
      {results && (
        <div className="grid equal-columns section-gap">
          {[
            ...(results.performance || [])
              .filter((r) => r.warning || !(r.findings || []).length)
              .map((r) => ({
                key: `p:${r.path}`,
                path: r.path,
                status: r.status,
                reason: r.warning,
              })),
            ...(results.security || [])
              .filter((r) => r.reason || !(r.findings || []).length)
              .map((r) => ({
                key: `s:${r.source}:${r.path}`,
                path: r.path,
                status: r.status,
                reason: r.reason,
              })),
          ].map((report) => (
            <Panel key={report.key} title={report.path} actions={<Badge value={report.status} />}>
              {report.reason && <Prose text={report.reason} />}
            </Panel>
          ))}
        </div>
      )}
      {!!results?.unclassified?.length && (
        <Panel title="Unclassified suggestions" className="section-gap">
          {results.unclassified.map((f) => (
            <Disclosure key={f.id} title={f.title}>
              <Prose text={f.message} />
              <Button onClick={() => void w.openFile(f.location.path, f.location.symbol)}>
                Open source
              </Button>
            </Disclosure>
          ))}
        </Panel>
      )}
    </>
  );
}
