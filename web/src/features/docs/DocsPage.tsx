// The generated handbook: purpose and topology, components by layer,
// dependencies, runbook, scenarios, decisions and glossary.

import { BookOpen, Boxes, Compass, GitFork, ListOrdered, Network, Terminal, TextQuote } from 'lucide-react';
import { useOverview, useProject } from '@/api/hooks';
import type { DocsComponent, ProjectDocs } from '@/api/types';
import { useActions, useBusy } from '@/app/actions';
import { useCheckout } from '@/app/checkout';
import { navigate, useRoute } from '@/lib/router';
import { timeAgo } from '@/lib/util';
import { Badge, Button, Callout, EmptyState, Skeleton, cx } from '@/components/ui';
import { useFeedback } from '@/components/overlays';

const SECTIONS = [
  { id: 'overview', label: 'Overview', icon: Compass },
  { id: 'components', label: 'Components', icon: Boxes },
  { id: 'dependencies', label: 'Dependencies', icon: GitFork },
  { id: 'scenarios', label: 'Scenarios', icon: Network },
  { id: 'decisions', label: 'Decisions', icon: ListOrdered },
  { id: 'runbook', label: 'Runbook', icon: Terminal },
  { id: 'glossary', label: 'Glossary', icon: TextQuote },
] as const;

export function DocsPage({ section }: { section: string }) {
  const { data, isLoading } = useOverview();
  const project = useProject().data;
  const { worktreePath } = useCheckout();
  const actions = useActions();
  const busy = useBusy();
  const docs = data?.docs;
  const current = SECTIONS.find(s => s.id === section)?.id ?? 'overview';

  return (
    <div className="docs">
      <nav className="docs-nav" aria-label="Docs sections">
        <div className="panel-title">Handbook</div>
        {SECTIONS.map(s => (
          <a key={s.id} href={`#/docs/${s.id}`} className={cx('docs-link', s.id === current && 'is-active')} onClick={e => { e.preventDefault(); navigate(`/docs/${s.id}`); }}>
            <s.icon size={14} aria-hidden />
            {s.label}
            <span className="spacer" />
            <span className="xs faint">{countOf(docs, s.id)}</span>
          </a>
        ))}
        {docs?.updated_at ? <div className="xs faint docs-updated">Updated {timeAgo(docs.updated_at)}</div> : null}
      </nav>
      <div className="docs-body page-scroll">
        <div className="page-inner page-narrow">
          {isLoading ? (
            <Skeleton lines={10} />
          ) : !docs ? (
            <EmptyState icon={BookOpen} title="No handbook yet" action={project ? <Button variant="primary" onClick={() => void actions.runAnalysis(project, worktreePath)} disabled={Boolean(busy)}>Analyze project</Button> : undefined}>
              Project analysis writes this handbook from the source: purpose, components, dependencies and how to run things.
            </EmptyState>
          ) : (
            <>
              {docs.status && docs.status !== 'fresh' && docs.status !== 'completed' ? (
                <div className="docs-notice">
                  <Callout tone="warning" title={DOCS_STATUS_TITLES[docs.status] ?? 'The handbook is incomplete'}>
                    Sections below come from indexed facts only. Run project analysis to write the full handbook.
                  </Callout>
                </div>
              ) : null}
              <DocsSection docs={docs} section={current} />
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function countOf(docs: ProjectDocs | undefined, id: string): string {
  if (!docs || id === 'overview') return '';
  const list = (docs as unknown as Record<string, unknown[]>)[id];
  return Array.isArray(list) && list.length ? String(list.length) : '';
}

function DocsSection({ docs, section }: { docs: ProjectDocs; section: string }) {
  const { toast } = useFeedback();
  switch (section) {
    case 'components':
      return <Components components={docs.components ?? []} />;
    case 'dependencies':
      return (
        <article className="doc">
          <h1>Dependencies</h1>
          <table className="table">
            <thead><tr><th>Name</th><th>Kind</th><th>Version</th><th>Source</th></tr></thead>
            <tbody>{(docs.dependencies ?? []).map(d => (<tr key={d.name}><td><div className="strong-row">{d.name}</div>{d.summary ? <div className="xs muted">{d.summary}</div> : null}</td><td className="small">{d.kind}</td><td className="mono xs">{d.version}</td><td className="mono xs muted">{d.source}</td></tr>))}</tbody>
          </table>
        </article>
      );
    case 'scenarios':
      return (
        <article className="doc">
          <h1>Scenarios</h1>
          {(docs.scenarios ?? []).length ? (docs.scenarios ?? []).map(s => (
            <section key={s.title} className="doc-card">
              <h2>{s.title}</h2>
              {s.summary ? <p>{s.summary}</p> : null}
              {s.invariant ? <p className="small"><span className="muted">Invariant: </span>{s.invariant}</p> : null}
              {s.steps?.length ? <ol className="steps">{s.steps.map((step, i) => (<li key={i}><span className="strong">{step.title}</span>{step.detail ? <span className="muted"> — {step.detail}</span> : null}{step.path ? <button type="button" className="link-btn mono xs" onClick={() => navigate('/code', { file: step.path })}>{step.path}</button> : null}</li>))}</ol> : null}
            </section>
          )) : <p className="muted">No scenarios were documented yet.</p>}
        </article>
      );
    case 'decisions':
      return (
        <article className="doc">
          <h1>Decisions</h1>
          {(docs.decisions ?? []).length ? (docs.decisions ?? []).map(d => (
            <section key={d.title} className="doc-card">
              <div className="row"><h2 className="grow">{d.title}</h2>{d.status ? <Badge>{d.status}</Badge> : null}</div>
              {d.context ? <p><span className="muted">Context: </span>{d.context}</p> : null}
              {d.decision ? <p><span className="muted">Decision: </span>{d.decision}</p> : null}
              {d.consequences ? <p><span className="muted">Consequences: </span>{d.consequences}</p> : null}
            </section>
          )) : <p className="muted">No decisions were recorded yet.</p>}
        </article>
      );
    case 'runbook':
      return (
        <article className="doc">
          <h1>Runbook</h1>
          <div className="stack">
            {(docs.runbook ?? []).map(r => (
              <div key={r.label} className="runbook-item">
                <div className="grow">
                  <div className="strong">{r.label}</div>
                  {r.description ? <div className="small muted">{r.description}</div> : null}
                  <code className="command">{r.command}</code>
                </div>
                <Button size="sm" variant="ghost" onClick={() => { void navigator.clipboard?.writeText(r.command); toast('Command copied.', 'success'); }}>Copy</Button>
              </div>
            ))}
          </div>
        </article>
      );
    case 'glossary':
      return (
        <article className="doc">
          <h1>Glossary</h1>
          <dl className="glossary">{(docs.glossary ?? []).map(g => (<div key={g.term}><dt>{g.term}</dt><dd>{g.definition}</dd></div>))}</dl>
        </article>
      );
    default:
      return (
        <article className="doc">
          <h1>Overview</h1>
          {docs.context?.purpose ? <p className="lead">{docs.context.purpose}</p> : null}
          <div className="facts">
            {docs.context?.topology ? <div><div className="xs muted">Topology</div><div>{docs.context.topology}</div></div> : null}
            {docs.context?.stack?.length ? <div><div className="xs muted">Stack</div><div>{docs.context.stack.join(', ')}</div></div> : null}
          </div>
          {docs.context?.goal_pursuit?.length ? (<><h2>Goals</h2><ul>{docs.context.goal_pursuit.map((g, i) => <li key={i}>{g}</li>)}</ul></>) : null}
          <h2>Layers</h2>
          <LayerMap components={docs.components ?? []} />
        </article>
      );
  }
}

function groupByLayer(components: DocsComponent[]): [string, DocsComponent[]][] {
  const map = new Map<string, DocsComponent[]>();
  for (const c of components) map.set(c.layer || 'Other', [...(map.get(c.layer || 'Other') ?? []), c]);
  return [...map.entries()];
}

function LayerMap({ components }: { components: DocsComponent[] }) {
  if (!components.length) return <p className="muted">No components documented yet.</p>;
  return (
    <div className="layers">
      {groupByLayer(components).map(([layer, list]) => (
        <div key={layer} className="layer">
          <div className="layer-name">{layer}</div>
          <div className="layer-items">
            {list.map(c => (
              <button key={c.name} type="button" className="layer-item" onClick={() => navigate('/docs/components', { name: c.name })} title={c.summary}>{c.name}</button>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

function Components({ components }: { components: DocsComponent[] }) {
  const route = useRoute();
  const focus = route.params.get('name');
  return (
    <article className="doc">
      <h1>Components</h1>
      {groupByLayer(components).map(([layer, list]) => (
        <section key={layer}>
          <h2>{layer}</h2>
          <div className="component-grid">
            {list.map(c => (
              <div key={c.name} className={cx('doc-card', c.name === focus && 'is-focus')}>
                <div className="row"><span className="strong grow">{c.name}</span>{c.path ? <span className="mono xs faint">{c.path}</span> : null}</div>
                {c.summary ? <p className="small muted">{c.summary}</p> : null}
                {c.responsibilities?.length ? <ul className="small">{c.responsibilities.slice(0, 4).map((r, i) => <li key={i}>{r}</li>)}</ul> : null}
              </div>
            ))}
          </div>
        </section>
      ))}
    </article>
  );
}

const DOCS_STATUS_TITLES: Record<string, string> = {
  failed: 'AI analysis did not complete',
  stale: 'The handbook is out of date',
  partial: 'The handbook is partial',
  missing: 'No handbook yet',
  running: 'Analysis is writing the handbook',
};
