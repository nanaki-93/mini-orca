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
} from './ui';

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
      <Heading
        title="Features"
        eyebrow="Project analysis"
        detail="AI suggestions for new capabilities, grounded in your project goals."
      >
        <Button disabled={!!s.busy} onClick={() => void w.loadFeatures()}>
          Refresh suggestions
        </Button>
      </Heading>
      <Notice>
        Suggestions are advisory. Their effort and benefits are estimates; implement and verify each
        idea through Chat.
      </Notice>
      <Panel title="Project goals" className="section-gap">
        <label className="block">
          What should this project help users do?
          <textarea
            className="composer"
            aria-label="Project goals"
            value={goals}
            maxLength={4096}
            disabled={!!s.busy}
            onChange={(e) => setGoals(e.target.value)}
            placeholder="Audience, important workflows, and constraints…"
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
      {report?.failure && <Notice error>{report.failure}</Notice>}
      {report?.freshness === 'stale' && (
        <Notice>
          These ideas use older project context or goals. Generate again before preparing an
          implementation.
        </Notice>
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
        {report && <Badge value={report.status} />}
      </div>
      {!report ? (
        <Empty title="Suggestions unavailable" />
      ) : report.status === 'not_generated' ? (
        <Empty
          title="No suggestions generated"
          detail="Add goals, then request ideas for this project."
        />
      ) : suggestions?.length === 0 ? (
        <Empty
          title="No matching suggestions"
          detail="Change the filter or generate a new set of ideas."
        />
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
