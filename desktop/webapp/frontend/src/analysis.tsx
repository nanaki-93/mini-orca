import { useEffect, useState } from 'react';
import { activeRun, analysisSetupModels, workspace as w, type State } from './workspace';
import {
  Badge,
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
import type { AnalysisModels, Limits } from './models';

const stages = ['semantic', 'performance', 'security_rules', 'security_ai'];
const stageNames: Record<string, string> = {
  semantic: 'Code',
  performance: 'Performance',
  security_rules: 'Security rules',
  security_ai: 'Security AI',
  feature_suggestions: 'New feature suggestions',
};
const defaults: Limits = { batch_files: 20, budget_seconds: 600, max_attempts_per_stage: 2 };

export function CapturedModels({ plan }: { plan?: import('./models').AnalysisPreview }) {
  if (!plan?.models) return <p className="small muted">Models unavailable</p>;
  return (
    <div className="grid three-columns">
      {(
        [
          ['code', 'Code', ['semantic']],
          ['review', 'Performance & Security', ['performance', 'security_rules', 'security_ai']],
          ['features', 'Feature discovery', ['feature_suggestions']],
        ] as const
      ).map(([key, label, stages]) => {
        const profile = plan.models![key];
        const provider = plan.providers?.find((p) =>
          p.stages.some((s) => (stages as readonly string[]).includes(s)),
        );
        const spec = provider?.model;
        return (
          <div key={key}>
            <strong className="block">{label}</strong>
            {spec ? (
              <>
                <div className="row between wrap">
                  <span>{spec.model}</span>
                  <Badge value={spec.remote_provider ? 'Remote' : 'Local'} />
                </div>
                <div className="small muted">{spec.provider_origin}</div>
                <div className="small muted">Profile: {profile}</div>
              </>
            ) : (
              <div className="small muted">
                <div className="row between wrap">
                  <span>Unavailable</span>
                </div>
                <div className="small muted">Profile: {profile || 'legacy'}</div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function ModelSelectors({
  s,
  value,
  disabled,
  onChange,
  className = '',
}: {
  s: State;
  value: AnalysisModels;
  disabled: boolean;
  onChange: (models: AnalysisModels) => void;
  className?: string;
}) {
  const choices = Object.entries(s.models?.scopes || {});
  return (
    <div className={`form-grid ${className}`}>
      {(
        [
          ['code', 'Code analysis model'],
          ['review', 'Performance & Security model'],
          ['features', 'Feature discovery model'],
        ] as const
      ).map(([key, label]) => (
        <label key={key}>
          <span>{label}</span>
          <select
            className="field"
            aria-label={label}
            value={value[key]}
            disabled={disabled || !choices.length}
            onChange={(event) =>
              onChange({ ...value, [key]: event.target.value as AnalysisModels[typeof key] })
            }
          >
            {!choices.length && <option value={value[key]}>Models unavailable</option>}
            {choices.map(([profile, model]) => (
              <option key={profile} value={profile}>
                {model.model} · {profile} · {model.remote_provider ? 'Remote' : 'Local'}
              </option>
            ))}
          </select>
        </label>
      ))}
    </div>
  );
}

export function Analysis({ s }: { s: State }) {
  const [filter, setFilter] = useState('');
  const [limit, setLimit] = useState(100);
  const [excluded, setExcluded] = useState<string[]>(s.selection?.excluded_paths || []);
  const [limits, setLimits] = useState(defaults);
  const [refresh, setRefresh] = useState(false);
  const models = analysisSetupModels(s);
  useEffect(() => setExcluded(s.selection?.excluded_paths || []), [s.selection?.selection_id]);
  const files = (s.selection?.files || []).filter((file) =>
    file.path.toLowerCase().includes(filter.toLowerCase()),
  );
  const changed =
    JSON.stringify([...excluded].sort()) !==
    JSON.stringify([...(s.selection?.excluded_paths || [])].sort());
  const edit = !!s.selection?.editable && !s.busy;
  const hasRepair =
    !activeRun(s.run) &&
    !['paused', 'interrupted'].includes(s.run?.status || '') &&
    s.selection?.recovery?.state === 'available';
  const preview = (mode: 'new' | 'repair') => {
    if (changed) {
      w.fail('Save your file selection before preparing a run.');
      return;
    }
    void w.previewAnalysis(mode, limits, refresh, models);
  };
  return (
    <>
      <Heading
        variant="intro"
        title="Analysis"
        detail={`${s.selection?.files.filter((f) => !f.reason && !excluded.includes(f.path)).length ?? '—'} eligible files selected`}
      >
        <div className="actions heading-action-group">
          <Button
            tone="primary"
            icon="play"
            disabled={!!s.busy || !s.selection || !s.models || activeRun(s.run)}
            onClick={() => preview('new')}
          >
            Prepare analysis
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
          <Go page="features" icon="arrow-right">
            Explore features
          </Go>
          <Button
            disabled={!!s.busy}
            icon="refresh"
            onClick={() => void w.act('Refresh files', () => w.refreshProject())}
          >
            Refresh
          </Button>
        </div>
      </Heading>

      <div className="stack analysis-sections section-gap">
        <Panel title="Run settings" className="analysis-run-settings">
          <ModelSelectors
            className="analysis-settings-fields"
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
                    onChange={(e) => setLimits({ ...limits, [key]: Number(e.target.value) })}
                  />
                </label>
              ))}
            </div>
            <label className="checkbox-line">
              <input
                type="checkbox"
                checked={refresh}
                onChange={(e) => setRefresh(e.target.checked)}
              />
              Refresh previously analyzed files
            </label>
          </Disclosure>
        </Panel>
        {s.run && (
          <Panel
            title="Last run"
            className="analysis-last-run"
            actions={
              <div className="row wrap">
                <span className="row">
                  <Icon name="activity" />
                  <StatusDot value={s.run.status} label="Analysis" />
                  <span>{human(s.run.status)}</span>
                </span>
                <Go page="analysis-run">View run</Go>
              </div>
            }
          >
            <CapturedModels plan={s.run.plan} />
          </Panel>
        )}
      </div>

      <div className="toolbar section-gap">
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
    </>
  );
}
export function AnalysisPreview({ s }: { s: State }) {
  const p = s.preview;
  if (!p)
    return (
      <Empty title="Prepare a new preview">
        <Go page="analysis">Back to analysis</Go>
      </Empty>
    );
  const ready = p.files.length > 0 || !!p.features?.max_model_requests;
  const isRepair = !!p.recover_incomplete;
  const isResume = !!s.resume;

  const currentSetup = w.snapshot().analysisSetup;
  const mismatch =
    isResume &&
    currentSetup &&
    p.models &&
    (currentSetup.code !== p.models.code ||
      currentSetup.review !== p.models.review ||
      currentSetup.features !== p.models.features);

  return (
    <>
      <Heading
        title={isRepair ? 'Repair analysis' : isResume ? 'Continue analysis' : 'Ready to analyze'}
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
      <div className="grid two-columns">
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
                ['Existing results', p.refresh ? 'Refresh' : 'Reuse when current'],
              ]}
            />
            {isRepair && (
              <p className="small muted section-gap">
                Repair re-attempts unfinished analysis work. It does not modify your source code.
              </p>
            )}
          </Panel>
          <Panel title="Models">
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
    </>
  );
}
export function AnalysisRun({ s }: { s: State }) {
  const run = s.run;
  if (!run)
    return (
      <Empty title="No analysis run yet">
        <Go page="analysis" tone="primary">
          Prepare analysis
        </Go>
      </Empty>
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
  const currentStep =
    run.status === 'canceling'
      ? 'Canceling analysis…'
      : run.status === 'pausing'
        ? 'Finishing the current step before pausing…'
        : runningStage
          ? `${stageNames[runningStage.stage]} · ${runningFile!.path}`
          : run.features?.status === 'running'
            ? 'Generating feature suggestions…'
            : 'Preparing analysis…';
  return (
    <>
      <Heading title="Project analysis" detail={<StatusDot value={run.status} label="Analysis" />}>
        <Go page="analysis">Files</Go>
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
      <Panel title="Captured models" className="section-gap">
        <CapturedModels plan={run.plan} />
      </Panel>
      <Panel>
        <div className="row between wrap">
          <h2>{run.window_files_completed} files completed this batch</h2>
          <span className="muted small">{run.elapsed_seconds}s elapsed</span>
        </div>
        {activeRun(run) && (
          <div role="status" aria-label="Current analysis step" className="section-gap">
            <p className="row wrap">
              <span className="spinner" aria-hidden="true" />
              {currentStep}
            </p>
            {runningStage && run.features?.status === 'running' && run.status === 'running' && (
              <p className="small muted">Generating feature suggestions…</p>
            )}
          </div>
        )}
        <progress value={done} max={Math.max(total, 1)} aria-label="Analysis progress" />
        <div className="small muted">
          {done} of {total} category work units finished
        </div>
      </Panel>
      {run.features && (
        <Panel
          title="New feature suggestions"
          className="section-gap"
          actions={<StatusDot value={run.features.status} label="Features" />}
        >
          <div className="metric-number">{run.features.suggestion_count ?? '—'}</div>
          <p className="small muted">
            Advisory ideas · {run.features.attempts} of {run.plan.limits.max_attempts_per_stage}{' '}
            attempts used
          </p>
          {run.features.reason && <Notice>{run.features.reason}</Notice>}
          <Go page="features">Open feature suggestions</Go>
        </Panel>
      )}
      <div className="grid three-columns section-gap">
        {run.sections.map((section) => (
          <Panel
            key={section.category}
            title={human(section.category)}
            actions={<StatusDot value={section.status} label={human(section.category)} />}
          >
            <div className="metric-number">{section.finding_count ?? '—'}</div>
            <p className="small muted">
              {section.finding_count === null ? 'No successful evidence yet' : 'Saved findings'}
            </p>
            <KeyValues
              values={[
                ['Completed', section.coverage.succeeded],
                ['Partial', section.coverage.partial],
                ['Failed', section.coverage.failed],
                ['Unavailable', section.coverage.unavailable],
              ]}
            />
            <Go page={section.category as 'bugs' | 'performance' | 'security'}>Open results</Go>
          </Panel>
        ))}
      </div>
      <Panel title="File progress" className="section-gap">
        <div className="scroll-list">
          {run.files.map((file) => (
            <Disclosure
              title={
                <span className="row between wrap">
                  <span className="mono small">{file.path}</span>
                  <span className="row wrap">
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
                  <StatusDot value={stage.status} label={stageNames[stage.stage]} />
                </div>
              ))}
            </Disclosure>
          ))}
        </div>
      </Panel>
    </>
  );
}
