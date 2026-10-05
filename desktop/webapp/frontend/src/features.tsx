import { useEffect, useState } from 'react';
import { workspace as w, type State } from './workspace';
import {
  Badge,
  Button,
  BulletContent,
  Disclosure,
  Empty,
  Go,
  Heading,
  Notice,
  Panel,
  Prose,
  StatusDot,
} from './ui';

function FeatureFailure({ s }: { s: State }) {
  const report = s.features;
  if (!s.featureGenerationRequested || !report?.failure) return null;
  const message = 'Feature search failed. Try again.';
  return (
    <Notice error>
      <p>
        {message}
        {report.suggestions.length > 0 && ' Previous ideas kept.'}
      </p>
      {report.failure !== message && (
        <Disclosure title="Error details">
          <Prose text={report.failure} />
        </Disclosure>
      )}
    </Notice>
  );
}

function FeatureEmpty({ s, filtered = false }: { s: State; filtered?: boolean }) {
  const report = s.features;
  const title = !report
    ? 'Features unavailable'
    : report.status === 'not_generated'
      ? 'No features yet'
      : report.status === 'failed'
        ? 'No saved features'
        : report.suggestions.length === 0
          ? 'No new features found'
          : filtered
            ? 'No matching features'
            : 'No active features';
  return (
    <>
      <Empty
        title={title}
        detail={
          report?.status === 'ready' && !report.suggestions.length
            ? 'Add goals and try again.'
            : undefined
        }
      />
      {!report && s.resourceErrors['feature suggestions'] && (
        <Disclosure title="Error details">
          <Prose text={s.resourceErrors['feature suggestions']} />
        </Disclosure>
      )}
    </>
  );
}

export function FeatureSummary({ s }: { s: State }) {
  const report = s.features;
  const suggestions = report?.suggestions.filter((idea) => idea.status !== 'dismissed') || [];
  return (
    <Panel
      title="New feature suggestions"
      actions={
        <div className="row">
          <StatusDot
            value={report?.freshness === 'stale' ? 'stale' : report?.status}
            label="Features"
          />
          <Go page="features" tone="ghost small">
            View all ideas
          </Go>
        </div>
      }
    >
      <div className="row wrap">{report?.freshness === 'stale' && <Badge value="stale" />}</div>
      <FeatureFailure s={s} />
      {s.featureGenerationRequested &&
        report?.freshness === 'stale' &&
        report.suggestions.length > 0 && (
          <Notice>Ideas are outdated. Generate again to update them.</Notice>
        )}
      {!report || report.status === 'not_generated' || suggestions.length === 0 ? (
        <FeatureEmpty s={s} />
      ) : (
        suggestions.slice(0, 3).map((idea) => (
          <div className="content-section" key={idea.id}>
            <div className="row between wrap">
              <h3>{idea.title}</h3>
              <Badge value={idea.status} />
            </div>
            <Prose text={idea.benefit} />
            <p className="small muted">Estimated effort: {idea.effort}</p>
            <Button
              disabled={!!s.busy || report.freshness !== 'current'}
              onClick={() => w.discussFeature(idea)}
            >
              Discuss in chat
            </Button>
          </div>
        ))
      )}
    </Panel>
  );
}

export function Features({ s }: { s: State }) {
  const report = s.features;
  const [goals, setGoals] = useState(report?.goals || '');
  const [filter, setFilter] = useState('active');
  useEffect(() => setGoals(report?.goals || ''), [report?.goals]);
  const suggestions = report?.suggestions.filter(
    (idea) =>
      filter === 'all' ||
      (filter === 'active' ? idea.status !== 'dismissed' : idea.status === filter),
  );
  return (
    <>
      <Heading title="Features">
        <Button disabled={!!s.busy} onClick={() => void w.loadFeatures()}>
          Refresh suggestions
        </Button>
      </Heading>
      <Panel title="Project goals" className="section-gap">
        <label className="block">
          Goals
          <textarea
            className="composer"
            aria-label="Project goals"
            value={goals}
            maxLength={4096}
            disabled={!!s.busy}
            onChange={(e) => setGoals(e.target.value)}
            placeholder="Audience, workflows, constraints…"
          />
        </label>
        <div className="actions section-gap">
          <Button
            disabled={!!s.busy || !report || goals === report.goals}
            onClick={() => void w.updateFeatures(goals)}
          >
            Save goals
          </Button>
          <Button
            tone="primary"
            disabled={!!s.busy || !report}
            onClick={() => void w.updateFeatures(goals, true)}
          >
            Suggest features
          </Button>
        </div>
      </Panel>
      <FeatureFailure s={s} />
      {s.featureGenerationRequested &&
        report?.freshness === 'stale' &&
        report.suggestions.length > 0 && (
          <Notice>Ideas are outdated. Generate again to update them.</Notice>
        )}
      <div className="toolbar section-gap">
        <h2>Feature suggestions</h2>
        <label>
          Show{' '}
          <select
            className="field"
            aria-label="Filter feature suggestions"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          >
            <option value="active">Active</option>
            <option value="saved">Saved</option>
            <option value="dismissed">Dismissed</option>
            <option value="all">All</option>
          </select>
        </label>
        {report && <StatusDot value={report.status} label="Features" />}
        {report?.freshness === 'stale' && <Badge value="stale" />}
      </div>
      {!report || report.status === 'not_generated' || suggestions?.length === 0 ? (
        <FeatureEmpty s={s} filtered />
      ) : (
        <div className="feature-grid">
          {suggestions?.map((idea) => (
            <Panel key={idea.id} title={idea.title} actions={<Badge value={idea.status} />}>
              <Prose text={idea.benefit} />
              <p className="small muted">Estimated effort: {idea.effort}</p>
              <Disclosure title="Why this fits the project">
                <Prose text={idea.evidence} />
                <BulletContent title="Suggested file scope" items={idea.paths} />
              </Disclosure>
              <BulletContent title="Acceptance criteria" items={idea.acceptance_criteria} />
              <div className="actions section-gap">
                <Button
                  tone="primary"
                  disabled={!!s.busy || report.freshness !== 'current'}
                  onClick={() => w.discussFeature(idea)}
                >
                  Discuss in chat
                </Button>
                <Button
                  disabled={!!s.busy}
                  onClick={() =>
                    void w.setFeatureStatus(idea.id, idea.status === 'saved' ? 'open' : 'saved')
                  }
                >
                  {idea.status === 'saved' ? 'Unsave' : 'Save idea'}
                </Button>
                <Button
                  disabled={!!s.busy}
                  onClick={() =>
                    void w.setFeatureStatus(
                      idea.id,
                      idea.status === 'dismissed' ? 'open' : 'dismissed',
                    )
                  }
                >
                  {idea.status === 'dismissed' ? 'Reopen' : 'Dismiss'}
                </Button>
              </div>
            </Panel>
          ))}
        </div>
      )}
      {report?.context_manifest && (
        <Disclosure title="Suggestion context">
          <BulletContent
            title="Included files"
            items={report.context_manifest.included?.map((file) => file.path)}
          />
        </Disclosure>
      )}
    </>
  );
}
