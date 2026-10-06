import { useEffect, useState } from 'react';
import { workspace as w, type State } from './workspace';
import {
  Badge,
  Button,
  BulletContent,
  Disclosure,
  Empty,
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
  const staleActiveCount =
    report?.suggestions.filter((idea) => idea.status !== 'dismissed' && idea.freshness === 'stale')
      .length || 0;

  const lastGen = report?.generations?.find((g) => g.id === report.last_generation);
  const addedCount = lastGen
    ? report?.suggestions.filter((idea) => idea.generation_id === lastGen.id).length || 0
    : 0;

  return (
    <div className="workspace-page features-page">
      <Heading
        title="Features"
        detail="Goal-aware advisory ideas. Discuss an idea in Chat before preparing a change."
        variant="intro"
      >
        <Button disabled={!!s.busy} onClick={() => void w.loadFeatures()}>
          Refresh suggestions
        </Button>
      </Heading>
      <Panel title="Project goals">
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
            onClick={() => void w.searchFeatures('Features', goals)}
          >
            {report?.suggestions?.length ? 'Search more feature suggestions' : 'Suggest features'}
          </Button>
        </div>
      </Panel>
      <FeatureFailure s={s} />
      {s.featureGenerationRequested && report?.status === 'ready' && (
        <Notice>
          {addedCount > 0 ? `Added ${addedCount} new suggestions.` : 'No new suggestions found.'}
        </Notice>
      )}
      {staleActiveCount > 0 && (
        <Notice>
          {staleActiveCount} active {staleActiveCount === 1 ? 'idea is' : 'ideas are'} outdated.
          Search again to update {staleActiveCount === 1 ? 'it' : 'them'}.
        </Notice>
      )}
      <div className="toolbar features-toolbar">
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
            <Panel
              key={idea.id}
              title={idea.title}
              actions={
                <div className="actions">
                  {idea.freshness === 'stale' && <Badge value="stale" />}
                  <Badge value={idea.status} />
                </div>
              }
            >
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
                  disabled={!!s.busy || idea.freshness === 'stale'}
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
          {lastGen?.model_summary && (
            <p className="small muted">
              Generated by {lastGen.model_summary.profile} with {lastGen.model_summary.model} ·{' '}
              {lastGen.model_summary.provider_origin}
            </p>
          )}
          <BulletContent
            title="Included files"
            items={report.context_manifest.included?.map((file) => file.path)}
          />
        </Disclosure>
      )}
    </div>
  );
}
