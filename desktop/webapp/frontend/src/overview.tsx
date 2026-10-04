import { useEffect, useState } from 'react';
import { workspace as w, type State, type Page } from './workspace';
import { FeatureSummary } from './features';
import {
  Badge,
  Button,
  BulletContent,
  Disclosure,
  Empty,
  Go,
  Heading,
  Icon,
  InsightCard,
  KeyValues,
  Panel,
  Prose,
} from './ui';

export function ProjectPage({ s }: { s: State }) {
  const [path, setPath] = useState(
    s.project?.path || localStorage.getItem('mini-orca:last-project') || '',
  );
  useEffect(() => {
    if (s.chosenPath) setPath(s.chosenPath);
  }, [s.chosenPath]);
  return (
    <>
      <Heading
        title={s.project ? 'Your project' : 'Open your workspace'}
        eyebrow="Mini-Orca"
        detail={s.project ? s.project.path : 'Choose a local project.'}
      />
      <div className="grid two-columns">
        <Panel title="Open project">
          <label className="field-label" htmlFor="project-path">
            Project folder
          </label>
          <div className="row">
            <input
              id="project-path"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="/path/to/project"
              spellCheck={false}
            />
            <Button disabled={!!s.busy} icon="folder" onClick={() => void w.chooseProject()}>
              Browse
            </Button>
          </div>
          <div className="actions section-gap">
            <Button
              disabled={!!s.busy || !path.trim() || !s.connected}
              tone="primary"
              onClick={() => void w.openProject(path, false)}
            >
              Open saved project
            </Button>
            <Button
              disabled={!!s.busy || !path.trim() || !s.connected}
              onClick={() => void w.openProject(path, true)}
            >
              Import & analyze
            </Button>
          </div>
          <p className="small muted section-gap">New project? Import to create its overview.</p>
          {!s.connected && (
            <div className="section-gap">
              <p className="small">
                Start the daemon with <code>go run ./cmd/daemon</code>.
              </p>
              <Button
                className="section-gap"
                onClick={() => void w.act('Connect', () => w.connect())}
              >
                Reconnect
              </Button>
            </div>
          )}
        </Panel>
        <Panel title={s.project ? 'Project facts' : 'Local by default'}>
          {s.project ? (
            <>
              <KeyValues
                values={[
                  ['Project', s.project.name],
                  ['Language', s.project.type],
                  ['Files', s.project.file_count],
                  ['Source files', s.project.source_file_count],
                  ['Lines', s.project.total_lines?.toLocaleString()],
                  [
                    'Overview',
                    <Badge value={s.overview?.analysis.status || s.project.ai_status} />,
                  ],
                ]}
              />
              <div className="actions section-gap">
                <Button disabled={!!s.busy} onClick={() => void w.reindex()} icon="refresh">
                  Refresh facts
                </Button>
                <Go page="summary">Summary</Go>
              </div>
              <Disclosure title="Project identity">
                <code>
                  {s.project.project_id}
                  <br />
                  {s.project.project_revision}
                </code>
              </Disclosure>
            </>
          ) : (
            <div className="welcome-art">
              <Icon name="layers" />
              <h2>Inspect. Draft. Review.</h2>
              <p>One change at a time.</p>
            </div>
          )}
        </Panel>
      </div>
      {s.project && (
        <div className="grid equal-columns section-gap">
          <Panel title="Languages">
            <KeyValues
              values={Object.entries(s.project.languages || {}).map(([key, value]) => [
                key,
                value.toLocaleString(),
              ])}
            />
          </Panel>
          <Panel title="Tools">
            <div className="actions">
              <Go page="scan" icon="shield">
                Verified scan
              </Go>
              <Go page="terminal" icon="terminal">
                Terminal
              </Go>
              <Go page="models" icon="sparkles">
                Models
              </Go>
            </div>
          </Panel>
        </div>
      )}
    </>
  );
}
export function Summary({ s }: { s: State }) {
  const overview = s.overview;
  const coverage = overview?.analysis_coverage;
  const percent = coverage?.total ? Math.round((100 * coverage.fresh) / coverage.total) : null;
  return (
    <>
      <Heading
        title={s.project!.name}
        eyebrow="Overview"
        detail={overview?.analysis.purpose ? undefined : s.project!.path}
      >
        <Button
          icon="refresh"
          disabled={!!s.busy}
          onClick={() => void w.act('Refresh overview', () => w.refreshProject())}
        >
          Refresh
        </Button>
        <Go page="analysis" icon="activity" tone="primary">
          Analyze project
        </Go>
      </Heading>
      <div className="overview-row">
        <Panel className="coverage-panel">
          <div className="coverage-ring">
            <svg viewBox="0 0 100 100">
              <circle className="ring-track" cx="50" cy="50" r="42" />
              <circle
                className="ring-mark"
                cx="50"
                cy="50"
                r="42"
                strokeDasharray={`${(percent || 0) * 2.64} 264`}
              />
            </svg>
            <strong>{percent === null ? '—' : `${percent}%`}</strong>
          </div>
          <div>
            <h2>Analysis coverage</h2>
            <p>
              {coverage
                ? `${coverage.fresh} of ${coverage.total} files current`
                : 'Not available yet'}
            </p>
            <div className="legend">
              <span className={coverage && coverage.stale > 0 ? 'coverage-stale' : undefined}>
                Outdated {coverage?.stale ?? '—'}
              </span>
              <span>Missing {coverage?.missing ?? '—'}</span>
              <span className={coverage && coverage.failed > 0 ? 'coverage-failed' : undefined}>
                Failed {coverage?.failed ?? '—'}
              </span>
            </div>
          </div>
        </Panel>
        <div className="metric-grid">
          {[
            ['bugs', 'Bugs', 'bug'],
            ['performance', 'Performance', 'gauge'],
            ['security', 'Security', 'shield'],
          ].map(([category, title, icon]) => {
            const section = s.run?.sections.find((value) => value.category === category);
            return (
              <button
                key={category}
                className="panel metric-card"
                data-accent={category}
                onClick={() => void w.navigate(category as Page)}
              >
                <div className="metric-label">
                  <span>{title}</span>
                  <span className="metric-icon">
                    <Icon name={icon} />
                  </span>
                </div>
                <div className="metric-number">{section?.finding_count ?? '—'}</div>
                <Badge value={section?.status || 'not_run'} />
              </button>
            );
          })}
        </div>
      </div>
      <div className="grid two-columns section-gap">
        <div className="stack">
          <Panel
            title="Project overview"
            actions={<Badge value={overview?.analysis.status || s.project!.ai_status} />}
          >
            <Prose text={overview?.analysis.purpose || s.project!.summary} />
            {overview?.analysis.failure && (
              <p className="error-text">{overview.analysis.failure}</p>
            )}
            {!!overview?.analysis.components?.length && (
              <Disclosure title="Components">
                <BulletContent title="" items={overview.analysis.components} />
              </Disclosure>
            )}
          </Panel>
          <Panel
            title="Findings"
            actions={
              <Go page="bugs" tone="ghost small">
                View all
              </Go>
            }
          >
            {s.findings?.length ? (
              s.findings.slice(0, 5).map((f) => (
                <button
                  className="list-row"
                  key={f.id}
                  onClick={() => void w.openFile(f.location.path, f.location.symbol, f.task_spec)}
                >
                  <span
                    className={`finding-mark ${['high', 'critical'].includes(f.severity) ? 'red' : f.severity === 'low' ? 'blue' : ''}`}
                  >
                    <Icon name="bug" />
                  </span>
                  <span className="list-copy">
                    <strong>{f.title}</strong>
                    <small>{f.location.path}</small>
                  </span>
                  <span className="result-badges">
                    <Badge value={f.severity} />
                    <Badge value={f.confidence} tone="violet" />
                  </span>
                </button>
              ))
            ) : (
              <Empty
                title={s.findings ? 'No saved findings' : 'Findings unavailable'}
                detail="Run an analysis or verified scan."
              />
            )}
          </Panel>
          <Panel title="Project facts">
            <div className="mini-metrics">
              <div>
                <strong>{overview?.metrics.source_file_count ?? '—'}</strong>
                <small>Source files</small>
              </div>
              <div>
                <strong>{overview?.metrics.total_lines?.toLocaleString() ?? '—'}</strong>
                <small>Lines</small>
              </div>
              <div>
                <strong>{overview?.finding_counts.verified ?? '—'}</strong>
                <small>Verified findings</small>
              </div>
            </div>
          </Panel>
        </div>
        <div className="stack">
          <InsightCard insight={overview?.analysis.engineering_insight} />
          <FeatureSummary s={s} />
          <Panel
            title="Architecture"
            actions={
              <Go page="diagrams" tone="ghost small">
                Explore
              </Go>
            }
          >
            <Prose text={overview?.analysis.architecture} diagramLabel="Architecture diagram" />
            {!overview?.analysis.architecture && (
              <p className="muted">No architecture overview saved.</p>
            )}
            {!!overview?.analysis.entry_points?.length && (
              <Disclosure title="Entry points">
                <BulletContent title="" items={overview.analysis.entry_points} />
              </Disclosure>
            )}
          </Panel>
          {!!overview?.analysis.flows?.length && (
            <Panel title="Project flows">
              {overview.analysis.flows.map((flow, i) => (
                <div className="content-section" key={i}>
                  <h3>Flow {i + 1}</h3>
                  <Prose text={flow} diagramLabel={`Flow ${i + 1} diagram`} />
                </div>
              ))}
            </Panel>
          )}
        </div>
      </div>
      {s.run && (
        <Panel className="section-gap">
          <div className="row between wrap">
            <div className="row">
              <Icon name="activity" />
              <strong>Project analysis</strong>
              <Badge value={s.run.status} />
            </div>
            <Go page="analysis-run">View run</Go>
          </div>
        </Panel>
      )}
    </>
  );
}
export function Models({ s }: { s: State }) {
  return (
    <>
      <Heading title="Models" detail="Configured destinations" eyebrow="Workspace">
        <Button
          icon="refresh"
          disabled={!!s.busy}
          onClick={() => void w.act('Refresh models', () => w.connect())}
        >
          Refresh
        </Button>
      </Heading>
      <div className="grid three-columns">
        {Object.values(s.models?.scopes || {}).map((model) => (
          <Panel
            key={model.scope}
            title={
              {
                analyze: 'Project analysis',
                bug: 'File & Security',
                function: 'Declaration edits',
              }[model.scope] || model.scope
            }
          >
            <div className="model-icon">
              <Icon name={model.remote_provider ? 'cloud' : 'laptop'} />
            </div>
            <h2>{model.model}</h2>
            <div className="section-gap">
              <Badge value={model.remote_provider ? 'Remote provider' : 'Local provider'} />
            </div>
            <KeyValues
              values={[
                ['Destination', model.provider_origin],
                ['Profile', model.profile],
                ['Reasoning', model.reasoning_effort || 'Default'],
                ['Timeout', model.timeout],
              ]}
            />
          </Panel>
        ))}
      </div>
      <p className="small muted section-gap">
        Change model configuration in config.yaml, then restart the daemon.
      </p>
      {!s.models && <Empty title="Model configuration unavailable" />}
    </>
  );
}
export function Diagrams({ s }: { s: State }) {
  return (
    <>
      <Heading title="Architecture & flows" eyebrow={s.project!.name}>
        <Go page="summary">Back to summary</Go>
      </Heading>
      <div className="stack">
        <Panel title="Architecture">
          <Prose text={s.overview?.analysis.architecture} diagramLabel="Architecture diagram" />
          {!s.overview?.analysis.architecture && (
            <p className="muted">No architecture overview saved.</p>
          )}
        </Panel>
        {(s.overview?.analysis.flows || []).map((flow, i) => (
          <Panel title={`Flow ${i + 1}`} key={i}>
            <Prose text={flow} diagramLabel={`Flow ${i + 1} diagram`} />
          </Panel>
        ))}
        {!s.overview?.analysis.flows?.length && (
          <Panel title="Project flows">
            <p className="muted">No project flows saved.</p>
          </Panel>
        )}
        <Panel title="Next steps">
          <BulletContent title="" items={s.overview?.analysis.next_steps} />
        </Panel>
      </div>
    </>
  );
}
export function SearchPage({ s }: { s: State }) {
  const [query, setQuery] = useState('');
  const [limit, setLimit] = useState(100);
  const files = (s.index?.files || []).filter((file) =>
    file.path.toLowerCase().includes(query.toLowerCase()),
  );
  const commands: [Page, string, string][] = [
    ['summary', 'Summary', 'grid'],
    ['analysis', 'Analyze project', 'activity'],
    ['editor', 'Editor', 'code'],
    ['terminal', 'Terminal', 'terminal'],
    ['models', 'Models', 'sparkles'],
    ['scan', 'Verified scan', 'shield'],
    ['project', 'Open project', 'folder'],
  ];
  return (
    <>
      <Heading title="Jump to" />
      <label htmlFor="search" className="sr-only">
        Search files and commands
      </label>
      <div className="input-wrap">
        <Icon name="search" />
        <input
          autoFocus
          id="search"
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setLimit(100);
          }}
          placeholder="Find a file or command…"
        />
      </div>
      <div className="grid two-columns section-gap">
        <Panel title={`Files · ${files.length}`}>
          {files.slice(0, limit).map((file) => (
            <button className="list-row" key={file.path} onClick={() => void w.openFile(file.path)}>
              <Icon name="file" />
              <span className="list-copy">
                <strong>{file.path.split('/').at(-1)}</strong>
                <small>{file.path}</small>
              </span>
            </button>
          ))}
          {files.length > limit && <Button onClick={() => setLimit(limit + 100)}>Show more</Button>}
        </Panel>
        <Panel title="Commands">
          {commands
            .filter(([, name]) => name.toLowerCase().includes(query.toLowerCase()))
            .map(([page, name, icon]) => (
              <button className="list-row" key={page} onClick={() => void w.navigate(page)}>
                <Icon name={icon} />
                <span>{name}</span>
              </button>
            ))}
        </Panel>
      </div>
    </>
  );
}
