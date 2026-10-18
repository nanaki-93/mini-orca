import { useState } from 'react';
import {
  activeRun,
  analysisSetupModels,
  canRepairAnalysis,
  recoverableFileCount,
  workspace as w,
  type State,
} from './workspace';
import { CapturedModels } from './analysis-models';
import {
  Button,
  Spinner,
  Disclosure,
  Empty,
  Go,
  Heading,
  Icon,
  KeyValues,
  Notice,
  Panel,
  StatusDot,
  human,
} from './ui';
import type { Limits } from './models';

const stages = ['semantic', 'performance', 'security_rules', 'security_ai'];
const stageNames: Record<string, string> = {
  semantic: 'Code',
  performance: 'Performance',
  security_rules: 'Security rules',
  security_ai: 'Security AI',
  feature_suggestions: 'New feature suggestions',
};
const defaults: Limits = { batch_files: 20, budget_seconds: 1800, max_attempts_per_stage: 2 };

export function StaleAnalysisButton({ s, path }: { s: State; path?: string }) {
  const count = s.selection?.files.filter(
    (file) =>
      (path === undefined || file.path === path) &&
      !file.reason &&
      !s.selection?.excluded_paths.includes(file.path) &&
      file.stages.some((stage) => stage.status === 'stale'),
  ).length;
  if (!count) return null;
  return (
    <Button
      icon="refresh"
      disabled={
        !!s.busy || !s.models || !s.selection?.editable || activeRun(s.run) || selectionChanged(s)
      }
      onClick={() =>
        void w.previewAnalysis(
          'stale',
          s.analysisLimits || defaults,
          false,
          s.analysisSetup || s.run?.plan.models,
          path,
        )
      }
    >
      {path === undefined ? `Analyze stale files (${count})` : 'Analyze stale file'}
    </Button>
  );
}

function selectionPaths(s: State) {
  return s.analysisSelectionDraft &&
    s.analysisSelectionDraft.selectionID === s.selection?.selection_id
    ? s.analysisSelectionDraft.excluded
    : s.selection?.excluded_paths || [];
}
function selectionChanged(s: State) {
  return (
    JSON.stringify([...selectionPaths(s)].sort()) !==
    JSON.stringify([...(s.selection?.excluded_paths || [])].sort())
  );
}
export function AnalysisFiles({ s }: { s: State }) {
  const [filter, setFilter] = useState('');
  const [limit, setLimit] = useState(100);
  const excluded = selectionPaths(s);
  const setExcluded = (paths: string[]) =>
    w.set({
      analysisSelectionDraft: { selectionID: s.selection!.selection_id, excluded: paths },
      preview: undefined,
    });
  const changed = selectionChanged(s);
  const edit = !!s.selection?.editable && !s.busy;
  const files = (s.selection?.files || []).filter((file) =>
    file.path.toLowerCase().includes(filter.toLowerCase()),
  );
  const canRepair = canRepairAnalysis(s);
  const recoverableCount = recoverableFileCount(s);
  return (
    <div className="workspace-page analysis-files">
      <Heading
        variant="intro"
        title="File Scope & Exclusions"
        detail={`${s.selection?.files.filter((f) => !f.reason && !excluded.includes(f.path)).length ?? '—'} eligible files selected · configure files to skip`}
      >
        <Button icon="refresh" disabled={!!s.busy || changed} onClick={() => void w.reindex()}>
          Refresh
        </Button>
        {canRepair && (
          <Button
            tone="primary"
            icon="refresh"
            disabled={!!s.busy || changed || activeRun(s.run)}
            onClick={() => void w.repairAnalysisDirect()}
          >
            {recoverableCount > 0 ? `Retry Failed (${recoverableCount})` : 'Repair Analysis'}
          </Button>
        )}
        <Button
          tone={canRepair ? 'default' : 'primary'}
          icon="play"
          disabled={!!s.busy || changed || activeRun(s.run)}
          loading={activeRun(s.run)}
          onClick={() => void w.runAnalysisDirect()}
        >
          {activeRun(s.run) ? 'Analyzing…' : 'Run Analysis'}
        </Button>
      </Heading>
      {changed && <Notice>Save your selection before starting analysis.</Notice>}
      <div className="toolbar">
        <div className="input-wrap">
          <Icon name="search" />
          <input
            aria-label="Filter analysis files"
            value={filter}
            onChange={(event) => {
              setFilter(event.target.value);
              setLimit(100);
            }}
            placeholder="Filter files…"
          />
        </div>
        <div className="actions">
          <Button
            disabled={!edit}
            onClick={() =>
              setExcluded(
                excluded.filter((path) => !files.some((f) => f.path === path && !f.reason)),
              )
            }
          >
            Include shown
          </Button>
          <Button
            disabled={!edit}
            onClick={() =>
              setExcluded([
                ...new Set([...excluded, ...files.filter((f) => !f.reason).map((f) => f.path)]),
              ])
            }
          >
            Exclude shown
          </Button>
          {changed && (
            <Button tone="primary" disabled={!edit} onClick={() => void w.saveSelection(excluded)}>
              Save selection
            </Button>
          )}
        </div>
      </div>
      {s.change && (
        <section className="context-task">
          <h2>Current task · {s.change.targets.length} of 8 paths</h2>
          {s.change.targets.map((target) => (
            <div className="studio-list-row" key={target.path}>
              <Icon name="file" />
              <code>{target.path}</code>
              <span className="muted">{target.exists ? 'Captured source' : 'New file'}</span>
            </div>
          ))}
        </section>
      )}
      <div className="panel table-wrap context-files">
        <table>
          <thead>
            <tr>
              <th>
                <span className="sr-only">Include file</span>
              </th>
              <th>File</th>
              {stages.map((stage) => (
                <th key={stage}>{stageNames[stage]}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {files.slice(0, limit).map((file) => (
              <tr key={file.path}>
                <td>
                  <input
                    aria-label={`Include ${file.path}`}
                    type="checkbox"
                    disabled={!edit || !!file.reason}
                    checked={!file.reason && !excluded.includes(file.path)}
                    onChange={(event) =>
                      setExcluded(
                        event.target.checked
                          ? excluded.filter((p) => p !== file.path)
                          : [...excluded, file.path],
                      )
                    }
                  />
                </td>
                <td className="path">
                  <button className="text-link path" onClick={() => void w.openFile(file.path)}>
                    {file.path}
                  </button>
                  {file.reason && <small className="muted block">{file.reason}</small>}
                </td>
                {stages.map((stage) => {
                  const status = file.stages?.find((item) => item.stage === stage);
                  return (
                    <td key={stage}>
                      <span title={status?.reason || file.reason}>
                        <StatusDot
                          label={stageNames[stage]}
                          value={
                            excluded.includes(file.path) || file.reason
                              ? 'excluded'
                              : status?.status
                          }
                        />
                      </span>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
        {files.length === 0 && (
          <Empty title={s.selection ? 'No matching files' : 'File selection unavailable'} />
        )}
        {files.length > limit && (
          <div className="panel-pad">
            <Button onClick={() => setLimit(limit + 100)}>
              Show more ({files.length - limit})
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
export function AnalysisPreview({ s }: { s: State }) {
  const p = s.preview;
  if (!p)
    return (
      <div className="workspace-page analysis-preview">
        <Heading variant="intro" title="Analysis preview">
          <Go page="summary">Back to overview</Go>
        </Heading>
        <Empty title="Prepare a new preview" />
      </div>
    );
  const ready = p.files.length > 0 || !!p.features?.max_model_requests;
  const isRepair = !!p.recover_incomplete;
  const isResume = !!s.resume;

  const currentSetup = s.analysisSetup && analysisSetupModels(s);
  const capturedSetup = p.models && analysisSetupModels({ ...s, analysisSetup: p.models });
  const mismatch =
    isResume &&
    currentSetup &&
    p.models &&
    (currentSetup.code !== capturedSetup?.code ||
      currentSetup.performance !== capturedSetup?.performance ||
      currentSetup.security !== capturedSetup?.security ||
      currentSetup.features !== capturedSetup?.features);

  return (
    <div className="workspace-page analysis-preview">
      <Heading
        variant="intro"
        title={isRepair ? 'Repair analysis' : isResume ? 'Continue analysis' : 'Ready to analyze'}
      >
        <Go page="summary">Back</Go>
        <Button
          tone="primary"
          icon="play"
          disabled={!ready || !!s.busy}
          onClick={() => void w.startAnalysis()}
        >
          {isRepair ? 'Start repair' : isResume ? 'Resume analysis' : 'Start analysis'}
        </Button>
      </Heading>
      <div className="grid two-columns analysis-preview-layout">
        <div className="stack">
          <Panel title={isRepair ? 'Repair scope' : 'Scope'}>
            <div className="mini-metrics">
              <div>
                <strong>{p.files.length}</strong>
                <small>{isRepair ? 'Files to repair' : 'Files'}</small>
              </div>
              <div>
                <strong>{p.expected_model_requests}</strong>
                <small>Expected requests</small>
              </div>
              <div>
                <strong>{p.max_model_requests}</strong>
                <small>Maximum requests</small>
              </div>
            </div>
            <KeyValues
              values={[
                ['Batch', `${p.limits.batch_files} files`],
                ['Time budget', `${p.limits.budget_seconds} seconds`],
                ['Attempts per stage', p.limits.max_attempts_per_stage],
                ['Existing results', p.refresh ? 'Refresh' : 'Reuse when current'],
              ]}
            />
          </Panel>
          <Panel title="Models" className="analysis-preview-models">
            <CapturedModels plan={p} />
            {isResume && (
              <p className="small muted section-gap">Resuming keeps this run's model choices.</p>
            )}
            {mismatch && (
              <Notice>
                Your current setup choices differ from this run's captured choices. Resuming uses
                the captured choices above.
              </Notice>
            )}
          </Panel>
          {p.features && (
            <Panel title="New feature suggestions">
              <p className="small muted">
                Project goals and allowed context, including root AGENTS.md. File exclusions apply.
              </p>
              {(p.features.reason || p.features.max_model_requests === 0) && (
                <Notice>{p.features.reason || 'No remaining requests for this step.'}</Notice>
              )}
              <Go page="features">Review project goals</Go>
            </Panel>
          )}
          <Panel title="Engineering specification (Docs)">
            <p className="small muted">
              Synthesizes C4 System Context, Subsystem Contracts, Runtime Scenarios, and ADRs from
              project facts and analysis evidence.
            </p>
            <Go page="docs" icon="book">
              Open Docs
            </Go>
          </Panel>
        </div>
        <Panel title="Selected files">
          {p.files.length === 0 && <Empty title="No selected files" />}
          <div className="scroll-list">
            {p.files.map((file) => (
              <div className="list-row" key={file.path}>
                <Icon name="file" />
                <span className="list-copy">
                  <strong>{file.path}</strong>
                  <small>
                    {file.stages
                      .filter((stage) => stage.eligible)
                      .map(
                        (stage) => `${stageNames[stage.stage]}${stage.cached ? ' · cached' : ''}`,
                      )
                      .join(' / ')}
                  </small>
                </span>
              </div>
            ))}
          </div>
          <Disclosure title={`${p.excluded?.length || 0} excluded files`}>
            <div className="scroll-list">
              {(p.excluded || []).map((file) => (
                <div className="list-row" key={file.path}>
                  <span className="list-copy">
                    <strong>{file.path}</strong>
                    <small>{file.reason}</small>
                  </span>
                </div>
              ))}
            </div>
          </Disclosure>
        </Panel>
      </div>
    </div>
  );
}
export function AnalysisRun({ s }: { s: State }) {
  const [tab, setTab] = useState<'overview' | 'files' | 'models'>('overview');
  const [fileQuery, setFileQuery] = useState('');
  const [onlyFailedFiles, setOnlyFailedFiles] = useState(false);
  const run = s.run;
  if (!run)
    return (
      <div className="workspace-page analysis-run">
        <Heading variant="intro" title="Last Run">
          <Go page="bugs" icon="bug">
            Issues & Ideas
          </Go>
          <Button
            tone="primary"
            icon="play"
            disabled={!!s.busy}
            onClick={() => void w.runAnalysisDirect(true)}
          >
            Run Analysis
          </Button>
        </Heading>
        <Empty title="No analysis run yet">
          Start a project analysis to inspect progress, file coverage, and findings.
        </Empty>
      </div>
    );
  const total = run.sections.reduce((sum, section) => sum + section.coverage.total, 0);
  const done = run.sections.reduce(
    (sum, section) =>
      sum +
      section.coverage.succeeded +
      section.coverage.partial +
      section.coverage.failed +
      section.coverage.skipped +
      section.coverage.unavailable,
    0,
  );
  const runningFile = run.files.find((file) =>
    file.stages.some((stage) => stage.status === 'running'),
  );
  const runningStage = runningFile?.stages.find((stage) => stage.status === 'running');
  const failedFiles = run.files.filter((file) =>
    file.stages.some((stage) =>
      ['failed', 'stale', 'partial', 'canceled', 'interrupted'].includes(stage.status),
    ),
  );
  const filteredRunFiles = run.files.filter(
    (file) =>
      (!onlyFailedFiles ||
        failedFiles.length === 0 ||
        file.stages.some((stage) =>
          ['failed', 'stale', 'partial', 'canceled', 'interrupted'].includes(stage.status),
        )) &&
      (!fileQuery.trim() || file.path.toLowerCase().includes(fileQuery.trim().toLowerCase())),
  );
  const isScopedRun = !!(
    run.plan.recover_incomplete ||
    run.plan.stale_only ||
    run.plan.retry_stale_failed
  );
  const sectionFindingCount = (category: string, runCount: number | null) => {
    const saved = s.results[category]?.saved_finding_count;
    if (isScopedRun && typeof saved === 'number') {
      return Math.max(runCount ?? 0, saved);
    }
    return runCount;
  };
  const totalFindings = run.sections.reduce(
    (sum, section) => sum + (sectionFindingCount(section.category, section.finding_count) || 0),
    0,
  );
  const canRepair = canRepairAnalysis(s);
  const recoverableCount = recoverableFileCount(s);
  const elapsed =
    run.elapsed_seconds < 60
      ? `${run.elapsed_seconds}s`
      : `${Math.floor(run.elapsed_seconds / 60)}m ${run.elapsed_seconds % 60}s`;
  const currentStep =
    run.status === 'canceling'
      ? 'Canceling analysis…'
      : run.status === 'pausing'
        ? 'Finishing the current step before pausing…'
        : runningStage
          ? `${stageNames[runningStage.stage]} · ${runningFile!.path}`
          : run.features?.status === 'running'
            ? 'Generating feature suggestions…'
            : run.status === 'queued'
              ? 'Preparing analysis…'
              : total > 0 && done === total
                ? 'Finalizing analysis & engineering docs…'
                : 'Waiting for the next step…';
  const pausedOrInterrupted = ['paused', 'interrupted'].includes(run.status);
  const docsCount =
    s.overview?.docs?.components?.length || s.overview?.analysis.components?.length || 0;
  const docsStatus = activeRun(run) ? 'running' : s.overview?.docs?.status || run.status;
  return (
    <div className="workspace-page analysis-run">
      <Heading
        variant="intro"
        title="Last Run"
        detail={
          <span className="row wrap" style={{ gap: '10px', alignItems: 'center' }}>
            <span className="analysis-run-status">
              <StatusDot value={run.status} label="Analysis" />
              <span>{human(run.status)}</span>
            </span>
            <span className="small muted">
              {run.files.length} files in scope · {totalFindings} findings · {elapsed} elapsed
            </span>
          </span>
        }
      >
        <Go page="bugs" icon="bug">
          Issues &amp; Ideas ({totalFindings})
        </Go>
        <Go page="agents" icon="sparkles" tone="ghost">
          Fix Pipeline
        </Go>
        {['running', 'queued'].includes(run.status) && (
          <Button icon="pause" disabled={!!s.busy} onClick={() => void w.controlRun('pause')}>
            Pause
          </Button>
        )}
        {pausedOrInterrupted && (
          <>
            <Button
              tone="primary"
              icon="play"
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
        )}
        {!activeRun(run) && !pausedOrInterrupted && (
          <>
            {canRepair && (
              <Button
                tone="primary"
                icon="refresh"
                disabled={!!s.busy}
                onClick={() => void w.repairAnalysisDirect()}
              >
                {failedFiles.length > 0
                  ? `Retry Failed (${failedFiles.length})`
                  : recoverableCount > 0
                    ? `Retry Failed (${recoverableCount})`
                    : 'Repair Analysis'}
              </Button>
            )}
            <Button
              tone={canRepair ? 'default' : 'primary'}
              icon="play"
              disabled={!!s.busy}
              onClick={() => void w.runAnalysisDirect(true)}
            >
              New Analysis
            </Button>
          </>
        )}
        {(activeRun(run) || pausedOrInterrupted) && (
          <Button
            disabled={!!s.busy || run.status === 'canceling'}
            onClick={() => void w.controlRun('cancel')}
          >
            Cancel run
          </Button>
        )}
      </Heading>
      {run.reason && (
        <Notice
          tone={
            ['paused', 'interrupted', 'stale', 'partial', 'failed'].includes(run.status)
              ? 'warning'
              : 'info'
          }
        >
          <div className="row between wrap" style={{ gap: '8px', alignItems: 'center' }}>
            <span>{run.reason}</span>
            {canRepair && (
              <Button
                tone="primary small"
                icon="refresh"
                disabled={!!s.busy}
                onClick={() => void w.repairAnalysisDirect()}
              >
                {failedFiles.length > 0
                  ? `Retry Failed (${failedFiles.length})`
                  : 'Repair Analysis'}
              </Button>
            )}
          </div>
        </Notice>
      )}
      {run.features?.reason && (
        <Notice
          tone={['failed', 'stale', 'partial'].includes(run.features.status) ? 'warning' : 'info'}
        >
          Feature discovery: {run.features.reason}
        </Notice>
      )}

      <div className="toolbar">
        <div className="mode-switch-group" role="tablist" aria-label="Last run views">
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'overview'}
            className={`mode-switch-btn ${tab === 'overview' ? 'active' : ''}`}
            onClick={() => setTab('overview')}
          >
            1. Overview &amp; Categories ({run.sections.length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'files'}
            className={`mode-switch-btn ${tab === 'files' ? 'active' : ''}`}
            onClick={() => setTab('files')}
          >
            2. File Coverage ({run.files.length}
            {failedFiles.length > 0 ? ` · ${failedFiles.length} failed` : ''})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'models'}
            className={`mode-switch-btn ${tab === 'models' ? 'active' : ''}`}
            onClick={() => setTab('models')}
          >
            3. Models &amp; Run Config
          </button>
        </div>
      </div>

      {tab === 'overview' && (
        <>
          <Panel className="analysis-progress">
            <div className="analysis-progress-heading">
              <div>
                <h2>File analysis</h2>
                <p className="small muted">
                  {total > 0
                    ? `${done} of ${total} file analysis steps finished`
                    : 'No file analysis steps'}
                </p>
              </div>
              {total > 0 && (
                <strong className="analysis-progress-percent" aria-hidden="true">
                  {Math.floor((done / total) * 100)}
                  <small>%</small>
                </strong>
              )}
            </div>
            {total > 0 && (
              <progress
                className="analysis-progress-bar"
                value={done}
                max={total}
                aria-label="File analysis progress"
                aria-valuetext={`${done} of ${total} file analysis steps finished`}
              />
            )}
            <div className="analysis-progress-meta small muted">
              <span>{run.window_files_completed} files completed this batch</span>
              <span>
                {failedFiles.length > 0 ? `${failedFiles.length} failed · ` : ''}
                {elapsed} elapsed
              </span>
            </div>
            {activeRun(run) && (
              <div
                className="analysis-current-step"
                role="status"
                aria-label="Current analysis step"
              >
                <Spinner />
                <p>{currentStep}</p>
              </div>
            )}
            {run.features && (
              <div
                className="analysis-feature-progress"
                role="status"
                aria-label="Feature discovery progress"
              >
                <span className="row">
                  <Icon name="sparkles" />
                  Feature discovery
                </span>
                <span className="analysis-run-status">
                  <StatusDot value={run.features.status} label="Feature discovery" />
                  <span>{human(run.features.status)}</span>
                </span>
              </div>
            )}
            <div
              className="analysis-feature-progress"
              role="status"
              aria-label="Engineering specification progress"
            >
              <span className="row">
                <Icon name="book" />
                Engineering specification (Docs)
              </span>
              <span className="analysis-run-status">
                <StatusDot value={docsStatus} label="Engineering specification" />
                <span>{human(docsStatus)}</span>
              </span>
            </div>
          </Panel>

          {failedFiles.length > 0 && (
            <Notice error>
              <div className="row between wrap" style={{ gap: '8px', alignItems: 'center' }}>
                <span>
                  {failedFiles.length} {failedFiles.length === 1 ? 'file' : 'files'} encountered
                  stage failures or incomplete steps during this run.
                </span>
                <div className="actions">
                  <Button
                    tone="primary small"
                    icon="refresh"
                    disabled={!!s.busy || activeRun(run)}
                    onClick={() => void w.repairAnalysisDirect()}
                  >
                    Retry Failed ({failedFiles.length})
                  </Button>
                  <Button
                    tone="ghost small"
                    onClick={() => {
                      setOnlyFailedFiles(true);
                      setTab('files');
                    }}
                  >
                    Inspect Failed Files ({failedFiles.length})
                  </Button>
                </div>
              </div>
            </Notice>
          )}

          <section
            className="stack analysis-run-details"
            aria-labelledby="analysis-run-details-title"
          >
            <div className="row between wrap">
              <h2 id="analysis-run-details-title">Run details</h2>
              <span className="small muted">
                Select any category below to inspect findings or start a fix
              </span>
            </div>
            <div className="grid analysis-run-categories">
              {run.sections.map((section) => {
                const count = sectionFindingCount(section.category, section.finding_count);
                return (
                  <Panel
                    key={section.category}
                    title={human(section.category)}
                    className="run-result-card"
                    actions={<StatusDot value={section.status} label={human(section.category)} />}
                  >
                    <div data-accent={section.category} className="run-result-summary">
                      <div className="metric-number">{count ?? '—'}</div>
                      <span>{human(section.status)}</span>
                    </div>
                    <p className="small muted">
                      {section.coverage.succeeded}/{section.coverage.total} files succeeded ·{' '}
                      {count === null
                        ? 'No evidence yet'
                        : section.category === 'performance'
                          ? 'Hypotheses'
                          : 'Saved findings'}
                    </p>
                    <div className="row between wrap" style={{ marginTop: '6px' }}>
                      <Go
                        page={section.category as 'bugs' | 'security' | 'performance'}
                        icon={
                          section.category === 'security'
                            ? 'shield'
                            : section.category === 'performance'
                              ? 'gauge'
                              : 'bug'
                        }
                        tone="ghost small"
                      >
                        Open {human(section.category)} →
                      </Go>
                    </div>
                    <Disclosure title="Coverage details">
                      <KeyValues
                        values={[
                          ['Completed', section.coverage.succeeded],
                          ['Partial', section.coverage.partial],
                          ['Failed', section.coverage.failed],
                          ['Unavailable', section.coverage.unavailable],
                          ['Skipped', section.coverage.skipped],
                          ['Pending', section.coverage.pending],
                        ]}
                      />
                    </Disclosure>
                  </Panel>
                );
              })}
              {run.features && (
                <Panel
                  title="New feature suggestions"
                  className="run-result-card"
                  actions={<StatusDot value={run.features.status} label="Features" />}
                >
                  <div data-accent="features" className="run-result-summary">
                    <div className="metric-number">{run.features.suggestion_count ?? '—'}</div>
                    <span>{human(run.features.status)}</span>
                  </div>
                  <p className="small muted">Advisory ideas</p>
                  <div className="row between wrap" style={{ marginTop: '6px' }}>
                    <Go page="features" icon="sparkles" tone="ghost small">
                      Open Features →
                    </Go>
                  </div>
                  <Disclosure title="Attempt details">
                    <p>
                      {run.features.attempts} of {run.plan.limits.max_attempts_per_stage} attempts
                      used
                    </p>
                  </Disclosure>
                </Panel>
              )}
              <Panel
                title="Engineering Specification"
                className="run-result-card"
                actions={<StatusDot value={docsStatus} label="Docs" />}
              >
                <div data-accent="docs" className="run-result-summary">
                  <div className="metric-number">{docsCount || '—'}</div>
                  <span>Subsystems</span>
                </div>
                <p className="small muted">C4 Context · Contracts · Scenarios · ADRs</p>
                <Go page="docs" icon="book" tone="ghost small">
                  Open Engineering Spec →
                </Go>
              </Panel>
            </div>
          </section>
        </>
      )}

      {tab === 'files' && (
        <section className="stack" aria-label="File coverage">
          <div className="toolbar">
            <div className="input-wrap" style={{ flex: 1 }}>
              <Icon name="search" />
              <input
                aria-label="Filter files in this run"
                value={fileQuery}
                onChange={(e) => setFileQuery(e.target.value)}
                placeholder="Filter analyzed files by path…"
              />
            </div>
            {failedFiles.length > 0 && (
              <div className="actions">
                <Button
                  tone={!onlyFailedFiles ? 'primary small' : 'ghost small'}
                  onClick={() => setOnlyFailedFiles(false)}
                >
                  All Files ({run.files.length})
                </Button>
                <Button
                  tone={onlyFailedFiles ? 'primary small' : 'ghost small'}
                  onClick={() => setOnlyFailedFiles(true)}
                >
                  Failed Only ({failedFiles.length})
                </Button>
                <Button
                  tone="primary small"
                  icon="refresh"
                  disabled={!!s.busy || activeRun(run)}
                  onClick={() => void w.repairAnalysisDirect()}
                >
                  Retry All Failed ({failedFiles.length})
                </Button>
              </div>
            )}
          </div>

          {failedFiles.length > 0 && (
            <Panel
              title={`Files with failures (${failedFiles.length})`}
              className="analysis-failed-files"
              actions={
                <Button
                  tone="primary small"
                  icon="refresh"
                  disabled={!!s.busy || activeRun(run)}
                  onClick={() => void w.repairAnalysisDirect()}
                >
                  Retry Failed ({failedFiles.length})
                </Button>
              }
            >
              <div className="scroll-list">
                {failedFiles.map((file) => (
                  <Disclosure
                    title={
                      <span className="row between wrap">
                        <span className="mono small analysis-file-path">{file.path}</span>
                        <span className="row wrap analysis-file-statuses">
                          {file.stages.map((stage) => (
                            <span key={stage.stage} title={stageNames[stage.stage]}>
                              <StatusDot value={stage.status} label={stageNames[stage.stage]} />
                            </span>
                          ))}
                        </span>
                      </span>
                    }
                    key={file.path}
                  >
                    {file.stages.map((stage) => (
                      <div className="list-row" key={stage.stage}>
                        <span className="list-copy">
                          <strong>{stageNames[stage.stage]}</strong>
                          <small>
                            {stage.reason ||
                              `${stage.attempts} attempts${stage.cached ? ' · cached' : ''}`}
                          </small>
                        </span>
                        <span className="analysis-run-status small">
                          <StatusDot value={stage.status} label={stageNames[stage.stage]} />
                          <span>{human(stage.status)}</span>
                        </span>
                      </div>
                    ))}
                    <div className="actions" style={{ marginTop: '8px' }}>
                      <Button
                        tone="primary small"
                        icon="refresh"
                        disabled={!!s.busy || activeRun(run)}
                        onClick={() => void w.repairAnalysisDirect(file.path)}
                      >
                        Retry File
                      </Button>
                      <Button
                        tone="ghost small"
                        icon="code"
                        onClick={() => void w.openFile(file.path)}
                      >
                        Open File
                      </Button>
                    </div>
                  </Disclosure>
                ))}
              </div>
            </Panel>
          )}

          <Panel title={`Files in this run (${filteredRunFiles.length})`}>
            {filteredRunFiles.length === 0 ? (
              <p className="small muted">No matching files recorded in this run.</p>
            ) : (
              <div className="scroll-list">
                {filteredRunFiles.map((file) => {
                  const fileHasFailure = file.stages.some((st) =>
                    ['failed', 'stale', 'partial', 'canceled', 'interrupted'].includes(st.status),
                  );
                  return (
                    <div className="list-row" key={file.path}>
                      <span className="list-copy">
                        <strong className="font-mono small">{file.path}</strong>
                      </span>
                      <span className="row wrap" style={{ gap: '8px', alignItems: 'center' }}>
                        {file.stages.map((stage) => (
                          <span
                            key={stage.stage}
                            className="row"
                            style={{ gap: '4px', fontSize: '0.75rem' }}
                            title={`${stageNames[stage.stage]}: ${human(stage.status)}${stage.reason ? ` — ${stage.reason}` : ''}`}
                          >
                            <StatusDot value={stage.status} label={stageNames[stage.stage]} />
                            <span className="muted">{stageNames[stage.stage]}</span>
                          </span>
                        ))}
                        {fileHasFailure && (
                          <Button
                            tone="primary small"
                            icon="refresh"
                            disabled={!!s.busy || activeRun(run)}
                            onClick={() => void w.repairAnalysisDirect(file.path)}
                          >
                            Retry
                          </Button>
                        )}
                        <Button
                          tone="ghost small"
                          icon="code"
                          onClick={() => void w.openFile(file.path)}
                        >
                          Open
                        </Button>
                      </span>
                    </div>
                  );
                })}
              </div>
            )}
          </Panel>
        </section>
      )}

      {tab === 'models' && (
        <section className="grid two-columns" aria-label="Models and run configuration">
          <Panel title="Captured models" className="analysis-run-models">
            <CapturedModels plan={run.plan} compact />
          </Panel>
          <Panel title="Run limits &amp; configuration">
            <KeyValues
              values={[
                ['Files in scope', run.files.length],
                ['Batch size', `${run.plan.limits.batch_files} files`],
                ['Time budget', `${run.plan.limits.budget_seconds} seconds`],
                ['Max attempts per stage', run.plan.limits.max_attempts_per_stage],
                ['Existing results', run.plan.refresh ? 'Refresh all' : 'Reuse when current'],
                ['Expected model requests', run.plan.expected_model_requests],
                ['Maximum model requests', run.plan.max_model_requests],
              ]}
            />
          </Panel>
        </section>
      )}
    </div>
  );
}
