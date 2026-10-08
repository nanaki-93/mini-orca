import * as fs from 'fs';

const path = 'desktop/webapp/frontend/src/analysis.tsx';
let code = fs.readFileSync(path, 'utf8');

// 1. Add CapturedModels component
const capturedModelsCode = `
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
        const provider = plan.providers?.find((p) => p.stages.some((s) => stages.includes(s)));
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
`;

code = code.replace(
  'function ModelSelectors',
  capturedModelsCode + '\nfunction ModelSelectors'
);

// 2. In Analysis: calculate hasRepair
const hasRepairCode = `  const hasRepair =
    !activeRun(s.run) &&
    !['paused', 'interrupted'].includes(s.run?.status || '') &&
    s.selection?.files.some(
      (f) =>
        !f.reason &&
        !excluded.includes(f.path) &&
        f.stages?.some((stage) =>
          ['stale', 'failed', 'partial', 'missing', 'canceled', 'unavailable', 'interrupted'].includes(stage.status),
        ),
    );
  const preview = (retry = false, recover = false) => {
    if (changed) {
      w.fail('Save your file selection before preparing a run.');
      return;
    }
    void w.previewAnalysis(limits, retry ? false : refresh, retry, false, models, recover);
  };`;

code = code.replace(
  /  const preview = \(retry = false\) => \{[\s\S]*?  \};/,
  hasRepairCode
);

// 3. In Analysis: replace the structure of Analysis
const oldAnalysisTop = `<Heading
        title="Analysis"
        detail={\`\${s.selection?.files.filter((f) => !f.reason && !excluded.includes(f.path)).length ?? '—'} eligible files selected\`}
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
          disabled={!!s.busy || !s.selection || !s.models || activeRun(s.run)}
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
      </div>`;

const newAnalysisTop = `<Heading
        title="Analysis"
        detail={\`\${s.selection?.files.filter((f) => !f.reason && !excluded.includes(f.path)).length ?? '—'} eligible files selected\`}
      >
        <Button
          disabled={!!s.busy}
          icon="refresh"
          onClick={() => void w.act('Refresh files', () => w.refreshProject())}
        >
          Refresh
        </Button>
      </Heading>

      <div className="grid equal-columns section-gap">
        <Panel title="Run settings">
          <ModelSelectors s={s} value={models} disabled={!!s.busy} onChange={setModels} />
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
        {s.run && (
          <Panel title="Last run">
            <div className="row between wrap">
              <div className="row">
                <Icon name="activity" />
                <StatusDot value={s.run.status} label="Analysis" />
              </div>
              <Go page="analysis-run">View run</Go>
            </div>
            <div className="section-gap">
              <CapturedModels plan={s.run.plan} />
            </div>
          </Panel>
        )}
      </div>

      <div className="actions section-gap">
        <Button
          tone="primary"
          icon="play"
          disabled={!!s.busy || !s.selection || !s.models || activeRun(s.run)}
          onClick={() => preview()}
        >
          Prepare analysis
        </Button>
        {hasRepair && (
          <Button
            icon="play"
            disabled={!!s.busy || !s.selection || !s.models || activeRun(s.run)}
            onClick={() => preview(false, true)}
          >
            Repair analysis
          </Button>
        )}
        <Button
          icon="sparkles"
          disabled={!!s.busy}
          onClick={() => void w.searchFeatures('Analysis', s.features?.goals || '')}
        >
          Search more feature suggestions
        </Button>
        <Go page="features" icon="arrow-right">
          Explore features
        </Go>
      </div>`;

code = code.replace(oldAnalysisTop, newAnalysisTop);

// 4. In Analysis: remove the old Run settings and Retry panels at the bottom
const bottomPanels = `      <div className="grid equal-columns section-gap">
        <Panel title="Run settings">
          <ModelSelectors s={s} value={models} disabled={!!s.busy} onChange={setModels} />
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
      </div>`;
code = code.replace(bottomPanels, '');

// 5. In AnalysisPreview: Replace ModelSelectors
const oldPreviewModels = `          <Panel title="Models">
            <ModelSelectors
              s={s}
              value={p.models || modelDefaults}
              disabled={!!s.busy || !!s.resume}
              onChange={(models) =>
                void w.previewAnalysis(p.limits, p.refresh, !!p.retry_stale_failed, false, models)
              }
            />
            {s.resume && <p className="small muted">Resuming keeps this run's model choices.</p>}
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
              </div>
            ))}
            <p className="small muted">
              Start confirms any remote context sharing and AI Security review.
            </p>
          </Panel>`;

const newPreviewModels = `          <Panel title="Models">
            <CapturedModels plan={p} />
            {s.resume && <p className="small muted section-gap">Resuming keeps this run's model choices.</p>}
            <p className="small muted section-gap">
              Start confirms any remote context sharing and AI Security review.
            </p>
          </Panel>`;

code = code.replace(oldPreviewModels, newPreviewModels);

// 6. In AnalysisRun: Add CapturedModels Panel
const oldAnalysisRunMiddle = `      <Panel>
        <div className="row between wrap">
          <h2>{run.window_files_completed} files completed this batch</h2>
          <span className="muted small">{run.elapsed_seconds}s elapsed</span>
        </div>`;

const newAnalysisRunMiddle = `      <Panel title="Captured models" className="section-gap">
        <CapturedModels plan={run.plan} />
      </Panel>
      <Panel>
        <div className="row between wrap">
          <h2>{run.window_files_completed} files completed this batch</h2>
          <span className="muted small">{run.elapsed_seconds}s elapsed</span>
        </div>`;

code = code.replace(oldAnalysisRunMiddle, newAnalysisRunMiddle);

fs.writeFileSync(path, code);
