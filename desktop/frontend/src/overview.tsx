import { useWorkspace } from './workspace-context';
import { useEffect, useState } from 'react';
import {
  workspace as w,
  activeChangeWorkflow,
  analysisSetupModels,
  workflowSetupModels,
  activeRun,
  type State,
  type Page,
} from './workspace';
import { ModelIcon } from './agent-model';
import { Tests } from './tools';
import { ModelSelectors } from './analysis-models';
import { WorkflowModels } from './change-workflow';
import {
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
  Spinner,
  human,
} from './ui';

export function ProjectPage({ s, compact = false }: { s: State; compact?: boolean }) {
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
                Open project
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
                  Daemon disconnected on <code>:9090</code>.
                </p>
                <Button onClick={() => void w.act('Connect', () => w.connect())}>Reconnect</Button>
              </div>
            )}
          </div>
        </Panel>
        {s.project && !compact && (
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
                Reindex
              </Button>
              <Go page="summary">Overview</Go>
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
      {s.project && !compact && (
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
  const [showTests, setShowTests] = useState(false);
  useEffect(() => {
    if (s.scan && ['running', 'failed', 'partial'].includes(s.scan.status)) setShowTests(true);
  }, [s.scan?.status]);
  const overview = s.overview;
  const coverage = overview?.analysis_coverage;
  const percent = coverage?.total ? Math.round((100 * coverage.fresh) / coverage.total) : null;
  const features = s.features;
  const featureCount =
    features?.suggestions.filter((idea) => idea.status !== 'dismissed').length || 0;
  const currentWork = s.change || s.changeHistory?.find((entry) => entry.state === 'draft');
  const bugSection = s.run?.sections.find((value) => value.category === 'bugs');
  const secSection = s.run?.sections.find((value) => value.category === 'security');
  const perfSection = s.run?.sections.find((value) => value.category === 'performance');
  const bugCount =
    bugSection?.finding_count ??
    s.findings?.filter((f) => f.category === 'bugs' && f.status !== 'fixed').length ??
    0;
  const secCount =
    secSection?.finding_count ??
    s.findings?.filter((f) => f.category === 'security' && f.status !== 'fixed').length ??
    0;
  const perfCount =
    perfSection?.finding_count ??
    s.findings?.filter((f) => f.category === 'performance' && f.status !== 'fixed').length ??
    0;
  const totalIssues = (bugCount || 0) + (secCount || 0) + (perfCount || 0) + featureCount;
  const openWork = (id: string) => (s.change?.id === id ? w.navigate('editor') : w.viewChange(id));

  const entryFiles = (s.index?.files || []).slice(0, 6);

  return (
    <div className="summary-page">
      <div className="summary-hero">
        <header className="page-heading">
          <div>
            <h1>{s.project!.name}</h1>
            <p className="summary-path">
              {s.project!.path} · {s.project!.source_file_count ?? '—'} source files ·{' '}
              {s.project!.total_lines?.toLocaleString() ?? '—'} lines
            </p>
          </div>
          <div className="actions">
            <Button
              icon="refresh"
              disabled={!!s.busy}
              onClick={() => void w.act('Refresh overview', () => w.refreshProject())}
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
            <Button
              icon="code"
              tone="primary"
              disabled={!!s.busy}
              onClick={() => void w.navigate('editor')}
            >
              Open Code & Diff
            </Button>
          </div>
        </header>
        {overview?.analysis.failure && <p className="error-text">{overview.analysis.failure}</p>}
      </div>

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
            borderRadius: '8px',
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

      {/* 3-Card Project Pulse */}
      <div className="kpi-pulse-grid">
        <div className="kpi-pulse-card card-kpi">
          <span className="small muted uppercase">Active Proposal</span>
          <div className="kpi-pulse-value" style={{ color: 'var(--accent)' }}>
            {s.change?.changes?.length
              ? `${s.change.changes.length} ${s.change.changes.length === 1 ? 'File' : 'Files'} Changed`
              : currentWork
                ? currentWork.title
                : 'Ready to Edit'}
          </div>
          <p className="small muted">
            {s.change?.title ||
              'Edit any project file inline or generate hunk-by-hunk AI diffs in Code & Diff.'}
          </p>
          <div className="actions" style={{ marginTop: 'auto' }}>
            {currentWork ? (
              <Button
                tone="primary small"
                icon="code"
                onClick={() => void openWork(currentWork.id)}
              >
                Review Diff Hunks →
              </Button>
            ) : (
              <Button tone="primary small" icon="code" onClick={() => void w.navigate('editor')}>
                Open Editor →
              </Button>
            )}
          </div>
        </div>

        <div className="kpi-pulse-card card-kpi">
          <span className="small muted uppercase">Issues & Feature Ideas</span>
          <div className="kpi-pulse-value">
            {totalIssues}{' '}
            <span className="small muted" style={{ fontWeight: 500 }}>
              ({bugCount} Bugs · {secCount} Sec · {perfCount} Perf · {featureCount} Ideas)
            </span>
          </div>
          <p className="small muted">
            {percent !== null
              ? `${percent}% analysis coverage (${coverage?.fresh}/${coverage?.total} files)`
              : 'Run project analysis to surface bugs, security risks, and feature ideas.'}
          </p>
          <div className="actions" style={{ marginTop: 'auto' }}>
            <Button tone="ghost small" icon="bug" onClick={() => void w.navigate('bugs')}>
              Triage & Fix Issues →
            </Button>
          </div>
        </div>

        <div className="kpi-pulse-card card-kpi">
          <span className="small muted uppercase">Git & Worktrees</span>
          <div className="kpi-pulse-value" style={{ color: 'var(--green)' }}>
            {s.repository?.branch || 'main'}{' '}
            <span className="small muted" style={{ fontWeight: 500 }}>
              · {s.gitWorkspace?.worktrees.length || 0} Worktrees
            </span>
          </div>
          <p className="small muted">
            In-place edits apply with guarded Undo. Background worktrees can be merged in one click.
          </p>
          <div className="actions" style={{ marginTop: 'auto' }}>
            <Button tone="ghost small" icon="branch" onClick={() => void w.navigate('git')}>
              Manage Git & Worktrees →
            </Button>
          </div>
        </div>
      </div>

      {/* Interactive System Architecture & Entry Points */}
      <Panel
        title="System Architecture & Entry Points"
        actions={
          <div className="row" style={{ gap: '8px' }}>
            <Go page="diagrams" tone="ghost small" icon="network">
              Full Diagrams
            </Go>
            <Go page="instructions" tone="ghost small" icon="shield">
              Settings & Rules
            </Go>
          </div>
        }
      >
        <Prose
          text={
            overview?.analysis.purpose ||
            overview?.analysis.architecture ||
            s.project!.summary ||
            'Select any module or entry point below to inspect or edit it directly in Code & Diff.'
          }
        />
        {entryFiles.length > 0 && (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))',
              gap: '10px',
              marginTop: '14px',
            }}
          >
            {entryFiles.map((file) => (
              <button
                key={file.path}
                className="panel card-kpi"
                style={{
                  textAlign: 'left',
                  padding: '12px 14px',
                  cursor: 'pointer',
                  background: 'var(--raised)',
                }}
                onClick={() => void w.openFile(file.path)}
              >
                <div className="row between">
                  <strong
                    className="font-mono"
                    style={{ fontSize: '0.82rem', color: 'var(--accent)' }}
                  >
                    {file.path.split('/').at(-1)}
                  </strong>
                  <Icon name="arrow" />
                </div>
                <small className="muted font-mono" style={{ display: 'block', marginTop: '4px' }}>
                  {file.path}
                </small>
              </button>
            ))}
          </div>
        )}
      </Panel>

      <Disclosure
        title={
          <span className="row">
            <strong>Tests</strong>
            <StatusDot value={s.scan?.status} />
            <span>{s.scan ? human(s.scan.status) : 'Not run'}</span>
          </span>
        }
        open={showTests}
      >
        <Tests s={s} />
      </Disclosure>
    </div>
  );
}
export function Models({ s }: { s: State }) {
  const w = useWorkspace();
  return (
    <div className="workspace-page models-workspace">
      <Heading
        title="Models"
        detail="Model assignments for analysis, creation, testing and review."
        variant="intro"
      >
        <Button
          icon="refresh"
          disabled={!!s.busy}
          onClick={() => void w.act('Refresh models', () => w.loadAvailableModels())}
        >
          Refresh
        </Button>
      </Heading>
      {s.modelsReturn && (
        <div className="actions">
          <Go page={s.modelsReturn} icon="back">
            Back to workspace
          </Go>
        </div>
      )}
      <Panel title="Analysis models" className="analysis-model-settings">
        <ModelSelectors
          s={s}
          value={analysisSetupModels(s)}
          disabled={!!s.busy}
          onChange={(models) => w.setAnalysisSetup(models)}
        />
      </Panel>
      <Panel title="Workflow models" className="fix-models">
        <WorkflowModels
          s={s}
          value={workflowSetupModels(s)}
          disabled={!!s.busy || !s.models || activeChangeWorkflow(s.change)}
        />
      </Panel>
      <Panel title="Analysis & batch settings" className="analysis-batch-settings">
        <div className="form-grid analysis-settings-fields">
          {(
            [
              ['batch_files', 'Files per batch', 1, 500, 'Maximum files in each batch.'],
              ['budget_seconds', 'Time budget · seconds', 1, 3600, 'File-analysis time per batch.'],
              [
                'max_attempts_per_stage',
                'Attempts per stage',
                1,
                4,
                'Includes the first attempt and retries.',
              ],
            ] as const
          ).map(([key, label, min, max, hint]) => {
            const limits = s.analysisLimits || {
              batch_files: 20,
              budget_seconds: 1800,
              max_attempts_per_stage: 2,
            };
            return (
              <div
                className="analysis-setting"
                key={key}
                style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}
              >
                <label htmlFor={`settings-${key}`} style={{ fontSize: '0.85rem', fontWeight: 600 }}>
                  {label}
                </label>
                <input
                  id={`settings-${key}`}
                  type="number"
                  min={min}
                  max={max}
                  value={limits[key]}
                  style={{
                    maxWidth: '140px',
                    padding: '6px 8px',
                    borderRadius: '5px',
                    border: '1px solid var(--line)',
                    background: 'var(--canvas)',
                    color: 'var(--text)',
                  }}
                  onChange={(e) =>
                    w.set({ analysisLimits: { ...limits, [key]: Number(e.target.value) } })
                  }
                />
                <small className="muted" style={{ fontSize: '0.75rem' }}>
                  {hint}
                </small>
              </div>
            );
          })}
        </div>
        <label
          className="checkbox-line"
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: '8px',
            marginTop: '14px',
            cursor: 'pointer',
          }}
        >
          <input
            type="checkbox"
            checked={s.analysisRefresh || false}
            onChange={(e) => w.set({ analysisRefresh: e.target.checked })}
          />
          <span style={{ fontSize: '0.85rem' }}>Refresh previously analyzed files on next run</span>
        </label>
      </Panel>
      <h2>Configured profiles</h2>
      <div className="grid models-grid">
        {Object.values(s.models?.scopes || {}).map((model) => (
          <Panel
            key={model.scope}
            title={
              {
                analyze: 'Project analysis',
                bug: 'File & Security',
                function: 'Creation & explanations',
              }[model.scope] || model.scope
            }
          >
            <div className="stack model-configuration">
              <div className="row model-identity">
                <ModelIcon model={model} />
                <h3>{model.model}</h3>
              </div>
              <p className="small muted">Thinking: {model.reasoning_effort || 'Default'}</p>
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
      <Heading title="Architecture and flow">
        <Go page="summary">Back to overview</Go>
      </Heading>
      <div className="architecture-layout">
        <section className="architecture-canvas">
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
        </section>
        <aside className="architecture-notes">
          {!!s.overview?.analysis.components?.length && (
            <Panel title="Components">
              <BulletContent title="" items={s.overview.analysis.components} />
            </Panel>
          )}
          <InsightCard insight={s.overview?.analysis.engineering_insight} />
          <Panel title="Next steps">
            <BulletContent title="" items={s.overview?.analysis.next_steps} />
          </Panel>
          <Go page="editor" icon="code">
            Explore source
          </Go>
        </aside>
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
    ['summary', 'Overview', 'grid'],
    ['analysis', 'Analyze project', 'activity'],
    ['editor', 'Source', 'code'],
    ['chat', 'Chat', 'chat'],
    ['changes', 'Changes', 'branch'],
    ['bugs', 'Findings', 'bug'],
    ['history', 'History', 'clock'],
    ['analysis-files', 'Context', 'layers'],
    ['instructions', 'Instructions', 'file'],
    ['diagrams', 'Architecture', 'branch'],
    ['terminal', 'Terminal', 'terminal'],
    ['models', 'Models', 'sparkles'],
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
          data-initial-focus
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
