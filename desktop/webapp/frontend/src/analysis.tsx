import { useEffect, useState } from 'react';
import { activeRun, workspace as w, type State } from './workspace';
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
import type { Limits } from './models';

const stages = ['semantic', 'performance', 'security_rules', 'security_ai'];
const stageNames: Record<string, string> = {
  semantic: 'Code',
  performance: 'Performance',
  security_rules: 'Security rules',
  security_ai: 'Security AI',
  feature_suggestions: 'New feature suggestions',
};
const defaults: Limits = { batch_files: 20, budget_seconds: 600, max_attempts_per_stage: 2 };
export function Analysis({ s }: { s: State }) {
  const [filter, setFilter] = useState('');
  const [limit, setLimit] = useState(100);
  const [excluded, setExcluded] = useState<string[]>(s.selection?.excluded_paths || []);
  const [limits, setLimits] = useState(defaults);
  const [refresh, setRefresh] = useState(false);
  useEffect(() => setExcluded(s.selection?.excluded_paths || []), [s.selection?.selection_id]);
  const files = (s.selection?.files || []).filter((file) =>
    file.path.toLowerCase().includes(filter.toLowerCase()),
  );
  const changed =
    JSON.stringify([...excluded].sort()) !==
    JSON.stringify([...(s.selection?.excluded_paths || [])].sort());
  const edit = !!s.selection?.editable && !s.busy;
  const preview = (retry = false) => {
    if (changed) {
      w.fail('Save your file selection before preparing a run.');
      return;
    }
    void w.previewAnalysis(limits, retry ? false : refresh, retry);
  };
  return (
    <>
      <Heading
        title="Analysis"
        detail={`${s.selection?.files.filter((f) => !f.reason && !excluded.includes(f.path)).length ?? '—'} eligible files selected`}
      >
        <Button
          disabled={!!s.busy}
          icon="refresh"
          onClick={() => void w.act('Refresh files', () => w.refreshProject())}
        >
          Refresh
        </Button>
        <Button
          tone="primary"
          icon="play"
          disabled={!!s.busy || !s.selection || activeRun(s.run)}
          onClick={() => preview()}
        >
          Prepare analysis
        </Button>
      </Heading>
      {s.run && (
        <Panel>
          <div className="row between wrap">
            <div className="row">
              <Icon name="activity" />
              <strong>Last run</strong>
              <StatusDot value={s.run.status} label="Analysis" />
            </div>
            <Go page="analysis-run">View run</Go>
          </div>
        </Panel>
      )}
      <div className="actions section-gap">
        <Go page="features" icon="sparkles">
          Explore features
        </Go>
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
      <div className="grid equal-columns section-gap">
        <Panel title="Run settings">
          <Disclosure title="Batch & request limits">
            <div className="form-grid">
              {(
                [
                  ['batch_files', 'Files per batch', 1, 500],
                  ['budget_seconds', 'Time budget · seconds', 1, 3600],
                  ['max_attempts_per_stage', 'Attempts per stage', 1, 4],
                ] as const
              ).map(([key, label, min, max]) => (
                <label key={key}>
                  {label}
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
        <Panel title="Retry incomplete work">
          <Button
            disabled={!!s.busy || !s.selection || activeRun(s.run)}
            onClick={() => preview(true)}
          >
            Prepare stale & failed
          </Button>
        </Panel>
      </div>
    </>
  );
}
export function AnalysisPreview({ s }: { s: State }) {
  const p = s.preview;
  const [providers, setProviders] = useState<string[]>([]);
  const [security, setSecurity] = useState(false);
  useEffect(() => {
    setProviders([]);
    setSecurity(false);
  }, [p?.preview_id]);
  if (!p)
    return (
      <Empty title="Prepare a new preview">
        <Go page="analysis">Back to analysis</Go>
      </Empty>
    );
  const ready =
    (p.files.length > 0 || !!p.features?.max_model_requests) &&
    (p.providers || []).every(
      (provider) => !provider.remote_confirmation_required || providers.includes(provider.id),
    ) &&
    (!p.security_review_intent_required || security);
  return (
    <>
      <Heading title={s.resume ? 'Continue analysis' : 'Ready to analyze'}>
        <Go page="analysis">Back</Go>
        <Button
          tone="primary"
          icon="play"
          disabled={!ready || !!s.busy}
          onClick={() => void w.startAnalysis(providers, security)}
        >
          {s.resume ? 'Resume analysis' : 'Start analysis'}
        </Button>
      </Heading>
      <div className="grid two-columns">
        <div className="stack">
          <Panel title="Scope">
            <div className="mini-metrics">
              <div>
                <strong>{p.files.length}</strong>
                <small>Files</small>
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
          </Panel>
          <Panel title="Providers">
            {(p.providers || []).map((provider) => (
              <div className="provider-row" key={provider.id}>
                <div className="row between">
                  <strong>{provider.model.model}</strong>
                  <Badge value={provider.model.remote_provider ? 'Remote' : 'Local'} />
                </div>
                <p className="small muted">{provider.model.provider_origin}</p>
                <p className="small section-gap">
                  {provider.stages.map((stage) => stageNames[stage]).join(' · ')}
                </p>
                {provider.remote_confirmation_required && (
                  <label className="checkbox-line">
                    <input
                      type="checkbox"
                      checked={providers.includes(provider.id)}
                      onChange={(event) =>
                        setProviders(
                          event.target.checked
                            ? [...providers, provider.id]
                            : providers.filter((id) => id !== provider.id),
                        )
                      }
                    />
                    Allow selected context to this provider
                  </label>
                )}
              </div>
            ))}
            {p.security_review_intent_required && (
              <label className="checkbox-line">
                <input
                  type="checkbox"
                  checked={security}
                  onChange={(event) => setSecurity(event.target.checked)}
                />
                Include AI Security review
              </label>
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
            onClick={() => void w.previewAnalysis(run.plan.limits, false, false, true)}
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
