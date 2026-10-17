import { useWorkspace } from './workspace-context';
import { useEffect, useState } from 'react';
import {
  workspace as w,
  activeChangeWorkflow,
  analysisSetupModels,
  workflowSetupModels,
  allModelPresets,
  isModelPresetActive,
  activeRun,
  evaluateBootstrapChecklist,
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
  const draftChange = s.change?.state === 'draft' ? s.change : undefined;
  const currentWork = draftChange || s.changeHistory?.find((entry) => entry.state === 'draft');
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

  const phases = s.scan?.phases || [];
  const passedPhases = phases.filter((p) => p.state === 'passed' || p.state === 'completed').length;
  const scanRunning = s.scan?.status === 'running';
  const testValue = scanRunning
    ? 'Running…'
    : phases.length > 0
      ? `${passedPhases} / ${phases.length} Passed`
      : s.scan
        ? human(s.scan.status)
        : 'Not run';
  const testMeta =
    phases.length > 0
      ? phases.map((p) => `${p.name}: ${human(p.state)}`).join(' · ')
      : 'parser · vet · test';

  const docs = overview?.docs;
  const componentCount =
    docs?.components?.length ||
    overview?.analysis.components?.length ||
    Math.max(1, new Set((s.index?.files || []).map((f) => f.path.split('/')[0])).size);
  const scenarioCount =
    docs?.scenarios?.length || Math.max(1, overview?.analysis.flows?.length || 1);
  const docsStatus = docs?.status || overview?.analysis.status || s.project?.ai_status || 'fresh';
  const bootstrapPillars = evaluateBootstrapChecklist(s);
  const bootstrapReadyCount = bootstrapPillars.filter((p) => p.ready).length;

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
              icon="shield"
              tone="ghost"
              disabled={!!s.busy}
              onClick={() => void w.openRepoBootstrap()}
            >
              Repo Readiness ({bootstrapReadyCount}/4 Pillars)
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
          </div>
        </header>
        {overview?.analysis.failure && <p className="error-text">{overview.analysis.failure}</p>}
      </div>

      {(activeRun(s.run) || pausedOrInterrupted) && (
        <div className="analysis-live-banner">
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
          {!pausedOrInterrupted && (
            <div className="actions" style={{ flexShrink: 0 }}>
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
            </div>
          )}
        </div>
      )}

      {/* 4-Box Project Pulse */}
      <div className="kpi-pulse-grid box-grid-4">
        <div
          className="kpi-pulse-card card-kpi box-card clickable"
          role="button"
          tabIndex={0}
          aria-expanded={showTests}
          onClick={() => setShowTests((prev) => !prev)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              setShowTests((prev) => !prev);
            }
          }}
        >
          <div className="box-head">
            <span className="box-eyebrow">
              <StatusDot value={s.scan?.status || 'fresh'} /> Tests &amp; Checks
            </span>
            <Button
              tone={scanRunning ? 'ghost small' : 'primary small'}
              icon={scanRunning ? 'stop' : 'play'}
              aria-label={scanRunning ? 'Cancel Tests' : 'Run Tests'}
              disabled={!s.project || !!s.busy}
              onClick={(e) => {
                e.stopPropagation();
                void w.scanProject(scanRunning);
              }}
            >
              {scanRunning ? 'Stop' : 'Run'}
            </Button>
          </div>
          <div
            className="kpi-pulse-value box-value"
            style={{ color: s.scan?.status === 'failed' ? 'var(--red)' : 'var(--green)' }}
          >
            {testValue}
          </div>
          <div className="box-meta">
            <span>{testMeta}</span>
            <span>{showTests ? '▲' : '▼'}</span>
          </div>
        </div>

        <div
          className="kpi-pulse-card card-kpi box-card clickable"
          role="button"
          tabIndex={0}
          onClick={() => void w.navigate('docs')}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              void w.navigate('docs');
            }
          }}
        >
          <div className="box-head">
            <span className="box-eyebrow">
              <Icon name="book" /> System Docs
            </span>
            <span className="pill">{human(docsStatus)}</span>
          </div>
          <div className="kpi-pulse-value box-value">{componentCount} Subsystems</div>
          <div className="box-meta">
            <span>{scenarioCount} Scenarios</span>
            <Icon name="arrow" />
          </div>
        </div>

        <div
          className="kpi-pulse-card card-kpi box-card clickable"
          role="button"
          tabIndex={0}
          onClick={() => void w.navigate('bugs')}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              void w.navigate('bugs');
            }
          }}
        >
          <div className="box-head">
            <span className="box-eyebrow">
              <Icon name="bug" /> Issues &amp; Ideas
            </span>
            {percent !== null && <span className="pill">{percent}% analyzed</span>}
          </div>
          <div className="kpi-pulse-value box-value">{totalIssues} Open</div>
          <div className="box-meta">
            <span>
              {bugCount} Bugs · {secCount} Sec · {perfCount} Perf
            </span>
            <Icon name="arrow" />
          </div>
        </div>

        <div
          className="kpi-pulse-card card-kpi box-card clickable"
          role="button"
          tabIndex={0}
          onClick={() => (currentWork ? void openWork(currentWork.id) : void w.navigate('editor'))}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              if (currentWork) void openWork(currentWork.id);
              else void w.navigate('editor');
            }
          }}
        >
          <div className="box-head">
            <span className="box-eyebrow">
              <Icon name="code" /> Active Proposal
            </span>
            <span className="pill">{s.repository?.branch || 'main'}</span>
          </div>
          <div className="kpi-pulse-value box-value" style={{ color: 'var(--accent)' }}>
            {draftChange?.changes?.length
              ? `${draftChange.changes.length} ${draftChange.changes.length === 1 ? 'File' : 'Files'} Changed`
              : currentWork
                ? currentWork.title
                : 'Ready to Edit'}
          </div>
          <div className="box-meta">
            <span>Open Code &amp; Diff</span>
            <Icon name="arrow" />
          </div>
        </div>
      </div>

      {showTests && (
        <Panel
          title="Verification Phases"
          actions={
            <Button tone="ghost small" icon="close" onClick={() => setShowTests(false)}>
              Hide
            </Button>
          }
        >
          <Tests s={s} />
        </Panel>
      )}

      {/* Concise System Summary — detailed architecture & entry points live on the dedicated Architecture page */}
      <Panel
        title="System Architecture & Entry Points"
        actions={
          <div className="actions">
            <Go page="diagrams" tone="ghost small" icon="layers">
              Architecture →
            </Go>
            <Go page="docs" tone="ghost small" icon="book">
              Engineering Spec →
            </Go>
          </div>
        }
      >
        <Prose
          text={
            overview?.analysis.purpose ||
            s.project!.summary ||
            'Open Architecture or Engineering Spec to inspect subsystem boundaries, diagrams, and entry points.'
          }
        />
      </Panel>
    </div>
  );
}
export function Models({ s }: { s: State }) {
  const w = useWorkspace();
  const [modelTab, setModelTab] = useState<'routing' | 'presets'>('routing');
  const [presetName, setPresetName] = useState('');
  const presets = allModelPresets(s);
  return (
    <div className="workspace-page models-workspace">
      <Heading
        title="Models"
        detail="Model assignments for analysis, creation, testing and review."
        variant="intro"
      >
        <Go page="analysis-files" tone="ghost" icon="folder">
          File Scope
        </Go>
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
      <div className="toolbar">
        <div className="mode-switch-group" role="tablist" aria-label="Model routing views">
          <button
            type="button"
            role="tab"
            aria-selected={modelTab === 'routing'}
            className={`mode-switch-btn ${modelTab === 'routing' ? 'active' : ''}`}
            onClick={() => setModelTab('routing')}
          >
            1. Model Routing
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={modelTab === 'presets'}
            className={`mode-switch-btn ${modelTab === 'presets' ? 'active' : ''}`}
            onClick={() => setModelTab('presets')}
          >
            2. Presets &amp; Batch Limits ({presets.length})
          </button>
        </div>
      </div>
      {modelTab === 'routing' && (
        <div className="grid two-columns">
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
        </div>
      )}
      {modelTab === 'presets' && (
        <>
          <Panel title="Saved model presets" className="model-presets-panel">
            <div className="stack">
              <div className="grid models-grid">
                {presets.map((preset) => {
                  const active = isModelPresetActive(s, preset);
                  return (
                    <div
                      key={preset.id}
                      className={`panel model-preset-card ${active ? 'active-preset' : ''}`}
                    >
                      <div>
                        <div
                          className="row"
                          style={{ justifyContent: 'space-between', alignItems: 'center' }}
                        >
                          <strong>{preset.name}</strong>
                          {active && <span className="pill">Active</span>}
                        </div>
                        <p className="small muted" style={{ marginTop: '4px' }}>
                          {preset.description}
                        </p>
                      </div>
                      <div className="actions">
                        <Button
                          tone={active ? undefined : 'primary'}
                          disabled={!!s.busy}
                          onClick={() => w.applyModelPreset(preset)}
                        >
                          {active ? 'Reapply preset' : 'Apply preset'}
                        </Button>
                        {!preset.builtIn && (
                          <Button
                            disabled={!!s.busy}
                            onClick={() => w.deleteCustomModelPreset(preset.id)}
                          >
                            Delete
                          </Button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
              <div className="row" style={{ gap: '8px', alignItems: 'center', marginTop: '4px' }}>
                <input
                  aria-label="Custom preset name"
                  placeholder="Save current routing as preset (e.g. Offline Ollama)…"
                  value={presetName}
                  onChange={(e) => setPresetName(e.target.value)}
                  style={{ maxWidth: '320px' }}
                />
                <Button
                  disabled={!!s.busy || !presetName.trim()}
                  onClick={() => {
                    w.saveCustomModelPreset(presetName);
                    setPresetName('');
                  }}
                >
                  Save preset
                </Button>
              </div>
            </div>
          </Panel>
          <Panel title="Analysis & batch settings" className="analysis-batch-settings">
            <div className="form-grid analysis-settings-fields">
              {(
                [
                  ['batch_files', 'Files per batch', 1, 500, 'Maximum files in each batch.'],
                  [
                    'budget_seconds',
                    'Time budget · seconds',
                    1,
                    3600,
                    'File-analysis time per batch.',
                  ],
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
                    <label
                      htmlFor={`settings-${key}`}
                      style={{ fontSize: '0.85rem', fontWeight: 600 }}
                    >
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
              <span style={{ fontSize: '0.85rem' }}>
                Refresh previously analyzed files on next run
              </span>
            </label>
          </Panel>
          <Disclosure title="Configured profiles & daemon config">
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
            <p className="small muted" style={{ marginTop: '8px' }}>
              Edit config.yaml, then restart the daemon.
            </p>
          </Disclosure>
        </>
      )}
      {!s.models ? (
        <Empty title="Model configuration unavailable" />
      ) : (
        Object.keys(s.models.scopes || {}).length === 0 && <Empty title="No configured models" />
      )}
    </div>
  );
}
export function Diagrams({ s }: { s: State }) {
  const [tab, setTab] = useState<'topology' | 'flows'>('topology');
  const entryFiles = (s.index?.files || []).slice(0, 6);
  const flows = s.overview?.analysis.flows || [];
  return (
    <div className="workspace-page">
      <Heading
        title="Architecture and flow"
        detail="System topology, core components, and repository entry points."
        variant="intro"
      >
        <Go page="docs" icon="book">
          Engineering Spec
        </Go>
        <Go page="editor" icon="code">
          Explore source
        </Go>
      </Heading>
      <div className="toolbar">
        <div className="mode-switch-group" role="tablist" aria-label="Architecture views">
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'topology'}
            className={`mode-switch-btn ${tab === 'topology' ? 'active' : ''}`}
            onClick={() => setTab('topology')}
          >
            1. Topology &amp; Entry Points
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'flows'}
            className={`mode-switch-btn ${tab === 'flows' ? 'active' : ''}`}
            onClick={() => setTab('flows')}
          >
            2. Runtime Flows ({flows.length})
          </button>
        </div>
      </div>
      {tab === 'topology' ? (
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
            {entryFiles.length > 0 && (
              <Panel title="Quick Entry Files">
                <div className="row wrap" style={{ gap: '8px' }}>
                  {entryFiles.map((file) => (
                    <Button
                      key={file.path}
                      tone="ghost small"
                      icon="file"
                      onClick={() => void w.openFile(file.path)}
                    >
                      <span className="font-mono">{file.path}</span>
                    </Button>
                  ))}
                </div>
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
          </aside>
        </div>
      ) : (
        <section className="stack" aria-label="Runtime flows">
          {flows.length === 0 ? (
            <Empty
              title="No runtime flows saved"
              detail="Run project analysis to generate runtime execution flow diagrams."
            />
          ) : (
            flows.map((flow, i) => (
              <Panel title={`Flow ${i + 1}`} key={i}>
                <Prose text={flow} diagramLabel={`Flow ${i + 1} diagram`} />
              </Panel>
            ))
          )}
        </section>
      )}
    </div>
  );
}
export function SearchPage({ s }: { s: State }) {
  const [query, setQuery] = useState('');
  const [limit, setLimit] = useState(100);
  const [activeIndex, setActiveIndex] = useState(0);
  const files = (s.index?.files || []).filter((file) =>
    file.path.toLowerCase().includes(query.toLowerCase()),
  );
  const commands: [Page, string, string][] = [
    ['summary', 'Overview', 'grid'],
    ['diagrams', 'Architecture', 'layers'],
    ['docs', 'Docs', 'book'],
    ['editor', 'Code & Diff', 'code'],
    ['bugs', 'Issues & Ideas', 'bug'],
    ['features', 'Features', 'sparkles'],
    ['agents', 'Agents', 'sparkles'],
    ['analysis-run', 'Last Run', 'activity'],
    ['git', 'Git & Worktrees', 'branch'],
    ['instructions', 'Settings & Rules', 'shield'],
    ['models', 'Model Routing · Models', 'compass'],
    ['analysis-files', 'File Scope & Exclusions', 'layers'],
    ['terminal', 'Terminal', 'terminal'],
    ['project', 'Open project', 'folder'],
  ];
  const visibleFiles = files.slice(0, limit);
  const visibleCommands = commands.filter(([, name]) =>
    name.toLowerCase().includes(query.toLowerCase()),
  );
  const visiblePresets = allModelPresets(s).filter((preset) =>
    `Preset: ${preset.name}`.toLowerCase().includes(query.toLowerCase()),
  );
  const totalItems = visibleFiles.length + visibleCommands.length + visiblePresets.length;
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
            setActiveIndex(0);
          }}
          onKeyDown={(event) => {
            if (!totalItems) return;
            if (event.key === 'ArrowDown') {
              event.preventDefault();
              setActiveIndex((i) => (i + 1) % totalItems);
            } else if (event.key === 'ArrowUp') {
              event.preventDefault();
              setActiveIndex((i) => (i - 1 + totalItems) % totalItems);
            } else if (event.key === 'Enter') {
              event.preventDefault();
              const idx = Math.min(activeIndex, totalItems - 1);
              if (idx < visibleFiles.length) {
                void w.openFile(visibleFiles[idx].path);
              } else if (idx < visibleFiles.length + visibleCommands.length) {
                const cmd = visibleCommands[idx - visibleFiles.length];
                if (cmd) void w.navigate(cmd[0]);
              } else {
                const preset = visiblePresets[idx - visibleFiles.length - visibleCommands.length];
                if (preset) w.applyModelPreset(preset);
              }
            }
          }}
          placeholder="Find a file or command…"
        />
      </div>
      <div className="grid two-columns search-regions">
        <Panel title={`Files · ${files.length}`}>
          {!files.length && <p className="muted">No matching files.</p>}
          {visibleFiles.map((file, index) => (
            <button
              className={`list-row ${activeIndex === index ? 'active' : ''}`}
              aria-selected={activeIndex === index}
              key={file.path}
              onClick={() => void w.openFile(file.path)}
            >
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
          {!visibleCommands.length && !visiblePresets.length && (
            <p className="muted">No matching commands.</p>
          )}
          {visibleCommands.map(([page, name, icon], index) => {
            const itemIndex = visibleFiles.length + index;
            return (
              <button
                className={`list-row ${activeIndex === itemIndex ? 'active' : ''}`}
                aria-selected={activeIndex === itemIndex}
                key={page}
                onClick={() => void w.navigate(page)}
              >
                <Icon name={icon} />
                <span>{name}</span>
              </button>
            );
          })}
          {visiblePresets.map((preset, index) => {
            const itemIndex = visibleFiles.length + visibleCommands.length + index;
            return (
              <button
                className={`list-row ${activeIndex === itemIndex ? 'active' : ''}`}
                aria-selected={activeIndex === itemIndex}
                key={preset.id}
                onClick={() => w.applyModelPreset(preset)}
              >
                <Icon name="sparkles" />
                <span>Preset: {preset.name}</span>
              </button>
            );
          })}
        </Panel>
      </div>
    </div>
  );
}
