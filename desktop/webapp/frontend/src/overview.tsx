import { StaleAnalysisButton } from './analysis';
import { useEffect, useState } from 'react';
import { workspace as w, activeChangeWorkflow, type State, type Page } from './workspace';
import { WorkflowModels, defaultWorkflowModels } from './change-workflow';
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
  StatusDot,
} from './ui';

export function ProjectPage({ s }: { s: State }) {
  const [path, setPath] = useState(
    s.project?.path || localStorage.getItem('mini-orca:last-project') || '',
  );
  useEffect(() => {
    if (s.chosenPath) setPath(s.chosenPath);
  }, [s.chosenPath]);
  return (
    <div className="workspace-page project-workspace">
      <Heading
        title={s.project ? 'Project' : 'Open project'}
        detail={s.project && <span className="project-current-path">{s.project.path}</span>}
        variant="intro"
      />
      <div className={s.project ? 'grid two-columns' : 'stack'}>
        <Panel title="Open project" className="project-opening">
          <div className="stack">
            <div>
              <label className="field-label" htmlFor="project-path">
                Project folder
              </label>
              <div className="row project-folder">
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
            </div>
            <div className="actions">
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
            {!s.connected && (
              <div className="stack">
                <p className="small">
                  Start the daemon with <code>go run ./cmd/daemon</code>.
                </p>
                <Button onClick={() => void w.act('Connect', () => w.connect())}>Reconnect</Button>
              </div>
            )}
          </div>
        </Panel>
        {s.project && (
          <Panel title="Project facts" className="project-facts">
            <KeyValues
              values={[
                ['Project', s.project.name],
                ['Language', s.project.type],
                ['Files', s.project.file_count],
                ['Source files', s.project.source_file_count],
                ['Lines', s.project.total_lines?.toLocaleString()],
                [
                  'Overview',
                  <StatusDot value={s.overview?.analysis.status || s.project.ai_status} />,
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
          </Panel>
        )}
      </div>
      {s.project && (
        <div className="grid equal-columns">
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
    </div>
  );
}
export function Summary({ s }: { s: State }) {
  const overview = s.overview;
  const coverage = overview?.analysis_coverage;
  const percent = coverage?.total ? Math.round((100 * coverage.fresh) / coverage.total) : null;
  const features = s.features;
  const metrics: {
    page: Page;
    title: string;
    icon: string;
    count?: number | null;
    status?: string;
    action?: string;
  }[] = [
    ...(
      [
        ['bugs', 'Bugs', 'bug'],
        ['performance', 'Performance', 'gauge'],
        ['security', 'Security', 'shield'],
      ] as const
    ).map(([page, title, icon]) => {
      const section = s.run?.sections.find((value) => value.category === page);
      return {
        page,
        title,
        icon,
        count: section?.finding_count,
        status: section?.status || 'not_run',
      };
    }),
    {
      page: 'features' as const,
      title: 'Features',
      icon: 'sparkles',
      count:
        features && (features.status === 'ready' || features.suggestions.length > 0)
          ? features.suggestions.filter((idea) => idea.status !== 'dismissed').length
          : undefined,
      status:
        features?.status === 'failed'
          ? 'failed'
          : features?.freshness === 'stale' ||
              features?.suggestions.some(
                (idea) => idea.status !== 'dismissed' && idea.freshness === 'stale',
              )
            ? 'stale'
            : features?.status,
    },
    {
      page: 'diagrams',
      title: 'Architecture and Flow',
      icon: 'branch',
      status: overview?.analysis.status || s.project!.ai_status,
      action: 'Explore',
    },
    {
      page: s.run ? 'analysis-run' : 'analysis',
      title: 'Project Analysis',
      icon: 'activity',
      status: s.run?.status || (s.run === null ? 'not_run' : 'unavailable'),
      action: s.run ? 'View run' : 'Prepare analysis',
    },
  ];
  return (
    <div className="summary-page">
      <div className="summary-hero">
        <Heading title={s.project!.name} detail={s.project!.path}>
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
          <StaleAnalysisButton s={s} />
        </Heading>
        <dl className="summary-facts" aria-label="Project facts">
          <div>
            <dt>Project type</dt>
            <dd className="summary-project-type">{s.project!.type}</dd>
          </div>
          <div>
            <dt>Source files</dt>
            <dd>{overview?.metrics.source_file_count ?? '—'}</dd>
          </div>
          <div>
            <dt>Lines</dt>
            <dd>{overview?.metrics.total_lines?.toLocaleString() ?? '—'}</dd>
          </div>
          <div>
            <dt>Verified findings</dt>
            <dd>{overview?.finding_counts.verified ?? '—'}</dd>
          </div>
        </dl>
      </div>
      <div className="summary-dashboard">
        <Panel className="coverage-panel">
          <div className="coverage-ring">
            <svg viewBox="0 0 100 100" aria-hidden="true">
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
          <div className="coverage-copy">
            <h2>Analysis coverage</h2>
            <p>
              {coverage
                ? `${coverage.fresh} of ${coverage.total} files current`
                : 'Not available yet'}
            </p>
          </div>
          <div className="legend">
            <span className={coverage && coverage.stale > 0 ? 'coverage-stale' : undefined}>
              Outdated <strong>{coverage?.stale ?? '—'}</strong>
            </span>
            <span>
              Missing <strong>{coverage?.missing ?? '—'}</strong>
            </span>
            <span className={coverage && coverage.failed > 0 ? 'coverage-failed' : undefined}>
              Failed <strong>{coverage?.failed ?? '—'}</strong>
            </span>
          </div>
        </Panel>
        <div className="metric-grid">
          {metrics.map(({ page, title, icon, count, status, action }) => (
            <button
              key={page}
              className="panel metric-card"
              data-accent={page}
              onClick={() => void w.navigate(page)}
            >
              <div className="metric-label">
                <span className="row">
                  <span className="metric-icon">
                    <Icon name={icon} />
                  </span>
                  <span>{title}</span>
                </span>
                <StatusDot value={status} label={title} />
              </div>
              {action ? (
                <div className="metric-action">
                  <span>{action}</span>
                  <Icon name="arrow" />
                </div>
              ) : (
                <div className="metric-value">
                  <span className="metric-number">{count ?? '—'}</span>
                  <Icon name="arrow" />
                </div>
              )}
            </button>
          ))}
        </div>
      </div>
      <div className="summary-details">
        <Panel
          title="Project overview"
          actions={
            <StatusDot value={overview?.analysis.status || s.project!.ai_status} label="Overview" />
          }
        >
          <Prose text={overview?.analysis.purpose || s.project!.summary} />
          {overview?.analysis.failure && <p className="error-text">{overview.analysis.failure}</p>}
          {!!overview?.analysis.components?.length && (
            <Disclosure title="Components">
              <BulletContent title="" items={overview.analysis.components} />
            </Disclosure>
          )}
        </Panel>
        <InsightCard insight={overview?.analysis.engineering_insight} />
      </div>
    </div>
  );
}
export function Models({ s }: { s: State }) {
  return (
    <div className="workspace-page models-workspace">
      <Heading
        title="Models"
        detail="Choose workflow models and inspect configured provider destinations."
        variant="intro"
      >
        <Button
          icon="refresh"
          disabled={!!s.busy}
          onClick={() => void w.act('Refresh models', () => w.connect())}
        >
          Refresh
        </Button>
      </Heading>
      <Panel title="Workflow models" className="fix-models">
        <p className="small muted">
          Creation, testing and review choices for Chat and fixes. A running workflow keeps its
          captured models.
        </p>
        <WorkflowModels
          s={s}
          value={s.workflowModels || s.change?.workflow?.models || defaultWorkflowModels}
          disabled={!!s.busy || !s.models || activeChangeWorkflow(s.change)}
        />
      </Panel>
      <h2>Configured profiles</h2>
      <p className="small muted">
        Current daemon configuration, not captured run choices or provider health.
      </p>
      <div className="grid models-grid">
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
            <div className="stack model-configuration">
              <div className="row model-identity">
                <div className="model-icon">
                  <Icon name={model.remote_provider ? 'cloud' : 'laptop'} />
                </div>
                <h3>{model.model}</h3>
              </div>
              <div>
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
            </div>
          </Panel>
        ))}
      </div>
      <Disclosure title="Configuration">
        <p className="small muted">Edit config.yaml, then restart the daemon.</p>
      </Disclosure>
      {!s.models ? (
        <Empty title="Model configuration unavailable" />
      ) : (
        Object.keys(s.models.scopes || {}).length === 0 && <Empty title="No configured models" />
      )}
    </div>
  );
}
export function Diagrams({ s }: { s: State }) {
  return (
    <>
      <Heading title="Architecture and Flow">
        <Go page="summary">Back to summary</Go>
      </Heading>
      <div className="stack">
        <Panel title="Architecture">
          <Prose text={s.overview?.analysis.architecture} diagramLabel="Architecture diagram" />
          {!s.overview?.analysis.architecture && (
            <p className="muted">No architecture overview saved.</p>
          )}
          {!!s.overview?.analysis.entry_points?.length && (
            <BulletContent title="Entry points" items={s.overview.analysis.entry_points} />
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
    <div className="workspace-page search-workspace">
      <Heading title="Jump to" variant="intro" />
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
      <div className="grid two-columns search-regions">
        <Panel title={`Files · ${files.length}`}>
          {!files.length && <p className="muted">No matching files.</p>}
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
          {!commands.some(([, name]) => name.toLowerCase().includes(query.toLowerCase())) && (
            <p className="muted">No matching commands.</p>
          )}
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
    </div>
  );
}
