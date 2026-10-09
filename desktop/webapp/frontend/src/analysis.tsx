import { useState } from 'react';
import { activeRun, analysisSetupModels, workspace as w, type State } from './workspace';
import { CapturedModels, ModelSelectors } from './analysis-models';
import {
  Button,
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

export function StaleAnalysisButton({ s }: { s: State }) {
  const count = s.selection?.files.filter(
    (file) =>
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
        void w.previewAnalysis('stale', s.analysisLimits || defaults, false, s.run?.plan.models)
      }
    >
      Analyze stale files ({count})
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
export function Analysis({ s }: { s: State }) {
  const limits = s.analysisLimits || defaults;
  const refresh = s.analysisRefresh || false;
  const models = analysisSetupModels(s);
  const excluded = selectionPaths(s);
  const changed = selectionChanged(s);
  const continuation =
    s.run && ['paused', 'interrupted'].includes(s.run.status) ? s.run : undefined;
  const hasRepair =
    !activeRun(s.run) && !continuation && s.selection?.recovery?.state === 'available';
  const preview = (mode: 'new' | 'repair') => {
    if (changed) {
      w.fail('Save your file selection before preparing a run.');
      return;
    }
    void w.previewAnalysis(mode, limits, refresh, models);
  };
  return (
    <div className="workspace-page">
      <Heading
        variant="intro"
        title="Analysis"
        detail={`${s.selection?.files.filter((f) => !f.reason && !excluded.includes(f.path)).length ?? '—'} eligible files selected`}
      >
        <div className="actions heading-action-group">
          <Go page="analysis-run" icon="activity">
            View run
          </Go>
          <StaleAnalysisButton s={s} />
          <Button
            tone="primary"
            icon="play"
            disabled={
              !!s.busy || (!continuation && (!s.selection || !s.models || activeRun(s.run)))
            }
            onClick={() =>
              continuation
                ? void w.previewAnalysis('resume', continuation.plan.limits)
                : preview('new')
            }
          >
            {continuation ? 'Prepare continuation' : 'Prepare analysis'}
          </Button>
          {hasRepair && (
            <Button
              icon="play"
              disabled={!!s.busy || !s.selection || !s.models || activeRun(s.run)}
              title="Repair does not change code. It re-attempts unfinished analysis work."
              onClick={() => preview('repair')}
            >
              Repair analysis
            </Button>
          )}
        </div>
        <div className="actions heading-action-group">
          <Button
            icon="sparkles"
            disabled={!!s.busy}
            title={s.busy ? 'Cannot search while another operation is running.' : undefined}
            onClick={() => void w.searchFeatures('Analysis', s.features?.goals || '')}
          >
            Search more feature suggestions
          </Button>
          <Button
            disabled={!!s.busy}
            icon="refresh"
            onClick={() => void w.act('Refresh files', () => w.refreshProject())}
          >
            Refresh
          </Button>
        </div>
      </Heading>

      {continuation && (
        <Notice>
          Your saved analysis is {human(continuation.status)}. Continue with its captured settings,
          or cancel it in Last run before starting a new analysis.
        </Notice>
      )}
      <div className="stack analysis-sections">
        <Panel
          title="Analysis setup"
          className="analysis-run-settings"
          actions={
            <Button
              icon="refresh"
              tone="ghost small"
              disabled={s.modelsLoading || !!s.busy}
              onClick={() => void w.loadAvailableModels()}
            >
              Refresh models
            </Button>
          }
        >
          <p className="analysis-settings-intro">Choose a model for each operation.</p>
          <ModelSelectors
            s={s}
            value={models}
            disabled={!!s.busy}
            onChange={(m) => w.setAnalysisSetup(m)}
          />
          <Disclosure title="Batch & request limits">
            <div className="form-grid analysis-settings-fields">
              {(
                [
                  ['batch_files', 'Files per batch', 1, 500],
                  ['budget_seconds', 'Time budget · seconds', 1, 3600],
                  ['max_attempts_per_stage', 'Attempts per stage', 1, 4],
                ] as const
              ).map(([key, label, min, max]) => (
                <label key={key}>
                  <span>{label}</span>
                  <input
                    type="number"
                    min={min}
                    max={max}
                    value={limits[key]}
                    onChange={(e) =>
                      w.set({ analysisLimits: { ...limits, [key]: Number(e.target.value) } })
                    }
                  />
                </label>
              ))}
            </div>
            <label className="checkbox-line">
              <input
                type="checkbox"
                checked={refresh}
                onChange={(e) => w.set({ analysisRefresh: e.target.checked })}
              />
              Refresh previously analyzed files
            </label>
          </Disclosure>
        </Panel>
      </div>

      {changed && (
        <Notice>
          File selection has unsaved changes. <Go page="analysis-files">Save selection in Files</Go>
        </Notice>
      )}
    </div>
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
  return (
    <div className="workspace-page analysis-files">
      <Heading
        variant="intro"
        title="Files"
        detail={`${s.selection?.files.filter((f) => !f.reason && !excluded.includes(f.path)).length ?? '—'} eligible files selected`}
      >
        <Go page="analysis" icon="activity">
          Analysis setup
        </Go>
        <Button
          icon="refresh"
          disabled={!!s.busy || changed}
          onClick={() => void w.act('Refresh files', () => w.refreshProject())}
        >
          Refresh
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
      <div className="panel table-wrap">
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
      <div className="workspace-page">
        <Heading variant="intro" title="Analysis preview">
          <Go page="analysis">Back to analysis</Go>
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
        detail="Review captured scope, request bounds and model destinations before starting."
      >
        <Go page="analysis">Back</Go>
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
            {isRepair && (
              <p className="small muted section-gap">
                Repair re-attempts unfinished analysis work. It does not modify your source code.
              </p>
            )}
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
            <p className="small muted section-gap">
              Start confirms any remote context sharing and AI Security review.
            </p>
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
  const run = s.run;
  if (!run)
    return (
      <div className="workspace-page analysis-run">
        <Heading variant="intro" title="Project analysis" />
        <Empty title="No analysis run yet">Prepare a run from Analysis in the sidebar.</Empty>
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
                ? 'Finalizing analysis…'
                : 'Waiting for the next step…';
  return (
    <div className="workspace-page analysis-run">
      <Heading
        variant="intro"
        title="Project analysis"
        detail={
          <span className="analysis-run-status">
            <StatusDot value={run.status} label="Analysis" />
            <span>{human(run.status)}</span>
          </span>
        }
      >
        {['running', 'queued'].includes(run.status) && (
          <Button icon="pause" disabled={!!s.busy} onClick={() => void w.controlRun('pause')}>
            Pause
          </Button>
        )}
        {['paused', 'interrupted'].includes(run.status) && (
          <Button
            tone="primary"
            disabled={!!s.busy}
            onClick={() => void w.previewAnalysis('resume', run.plan.limits)}
          >
            Prepare continuation
          </Button>
        )}
        {(activeRun(run) || ['paused', 'interrupted'].includes(run.status)) && (
          <Button
            disabled={!!s.busy || run.status === 'canceling'}
            onClick={() => void w.controlRun('cancel')}
          >
            Cancel run
          </Button>
        )}
      </Heading>
      {run.reason && <Notice>{run.reason}</Notice>}
      {run.features?.reason && <Notice>Feature discovery: {run.features.reason}</Notice>}
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
          <span>{elapsed} elapsed</span>
        </div>
        {activeRun(run) && (
          <div className="analysis-current-step" role="status" aria-label="Current analysis step">
            <span className="spinner" aria-hidden="true" />
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
      </Panel>
      <Disclosure title="Run details">
        <div className="stack analysis-run-details">
          <div className="grid analysis-run-categories">
            {run.sections.map((section) => (
              <Panel
                key={section.category}
                title={human(section.category)}
                className="run-result-card"
                actions={<StatusDot value={section.status} label={human(section.category)} />}
              >
                <div data-accent={section.category} className="run-result-summary">
                  <div className="metric-number">{section.finding_count ?? '—'}</div>
                  <span>{human(section.status)}</span>
                </div>
                <p className="small muted">
                  {section.finding_count === null
                    ? 'No successful evidence yet'
                    : section.category === 'performance'
                      ? 'Hypotheses · unmeasured'
                      : 'Saved findings'}
                </p>
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
            ))}
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
                <Disclosure title="Attempt details">
                  <p>
                    {run.features.attempts} of {run.plan.limits.max_attempts_per_stage} attempts
                    used
                  </p>
                </Disclosure>
              </Panel>
            )}
          </div>
          <Panel title="Captured models" className="analysis-run-models">
            <CapturedModels plan={run.plan} compact />
            <Disclosure title="Provider details">
              <CapturedModels plan={run.plan} />
            </Disclosure>
          </Panel>
          <Disclosure title="File progress">
            <div className="scroll-list">
              {run.files.map((file) => (
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
                </Disclosure>
              ))}
            </div>
          </Disclosure>
        </div>
      </Disclosure>
    </div>
  );
}
