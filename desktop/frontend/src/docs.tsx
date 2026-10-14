import { useMemo, useState } from 'react';
import { activeRun, workspace as w, type State } from './workspace';
import type {
  ProjectDocsComponent,
  ProjectDocsDecision,
  ProjectDocsGlossaryItem,
  ProjectDocsReport,
  ProjectDocsRunbookItem,
  ProjectDocsScenario,
} from './models';
import { Button, Empty, Heading, Icon, Panel, Prose, StatusDot, human } from './ui';

type DocsTab = 'context' | 'components' | 'scenarios' | 'decisions';

function classifyLayer(path: string, name: string): string {
  const lower = `${path} ${name}`.toLowerCase();
  if (/(desktop|frontend|ui|view|web|client)/.test(lower)) return 'Presentation Layer';
  if (/(api|handler|route|http|transport|cmd|daemon|cli)/.test(lower))
    return 'API / Transport Boundary';
  if (/(app|service|workflow|orchestrat|job|usecase)/.test(lower))
    return 'Application / Orchestration';
  if (/(script|test|eval|bench|ci|quality)/.test(lower)) return 'Tooling & Verification';
  return 'Domain & Persistence';
}

function buildFallbackDocs(s: State): ProjectDocsReport {
  const analysis = s.overview?.analysis;
  const files = s.index?.files || [];
  const rawComponents = analysis?.components || [];
  const components: ProjectDocsComponent[] = [];
  const seenPaths = new Set<string>();

  for (const raw of rawComponents) {
    const [head = '', rest = ''] = raw.split(':').map((p) => p.trim());
    const paren = head.indexOf('(');
    const path = (paren > 0 ? head.slice(0, paren) : head).trim() || 'core';
    const name =
      paren > 0 && head.endsWith(')')
        ? head.slice(paren + 1, -1).trim()
        : path.split('/').filter(Boolean).at(-1) || path;
    const layer = classifyLayer(path, name);
    const summary = rest || `Owns ${name} responsibilities within ${layer}.`;
    seenPaths.add(path);
    components.push({
      name,
      layer,
      path,
      summary,
      responsibilities: [summary],
      contracts: [
        {
          name: `${name} Boundary Contract`,
          kind: layer.includes('API') ? 'Transport & Schema' : 'Service Contract',
          summary,
        },
      ],
      dependencies: [],
    });
  }

  for (const file of files) {
    const parts = file.path.split('/');
    const dir =
      parts.length > 1 && ['internal', 'cmd', 'pkg', 'src', 'desktop'].includes(parts[0])
        ? `${parts[0]}/${parts[1]}`
        : parts[0];
    let comp = components.find((c) => c.path === dir || file.path.startsWith(c.path));
    if (!comp && components.length < 10 && !seenPaths.has(dir)) {
      seenPaths.add(dir);
      const name = dir
        .split(/[/_.-]/)
        .filter(Boolean)
        .map((w) => w[0].toUpperCase() + w.slice(1))
        .join(' ');
      const layer = classifyLayer(dir, name);
      const summary = `Subsystem at ${dir} governing ${layer.toLowerCase()} operations.`;
      comp = {
        name: name || 'Core Subsystem',
        layer,
        path: dir,
        summary,
        responsibilities: [summary],
        contracts: [],
        dependencies: [],
      };
      components.push(comp);
    }
    if (comp) {
      if (comp.responsibilities.length < 4) {
        comp.responsibilities.push(
          `Maintains ${file.path.split('/').at(-1)} (${file.language}, ${file.line_count} lines)`,
        );
      }
      for (const sym of file.symbols || []) {
        if (comp.contracts.length >= 4) break;
        comp.contracts.push({
          name: sym.name,
          kind: 'Public Contract',
          summary: `Exposed ${sym.kind} contract in ${file.path} (lines ${sym.start_line}-${sym.end_line}).`,
        });
      }
    }
  }

  for (let i = 0; i < components.length; i++) {
    if (!components[i].contracts.length) {
      components[i].contracts.push({
        name: `${components[i].name} Interface`,
        kind: 'Module Boundary',
        summary: components[i].summary,
      });
    }
    if (!components[i].dependencies.length && components.length > 1) {
      components[i].dependencies.push(components[(i + 1) % components.length].name);
    }
  }

  const flows = analysis?.flows || [];
  const entries = analysis?.entry_points || [];
  const scenarios: ProjectDocsScenario[] = (
    flows.length ? flows : [analysis?.purpose || 'Primary request admission and execution flow.']
  ).map((flow, idx) => {
    const isMermaid = /^(flowchart|graph|sequenceDiagram)\b/.test(flow.trim());
    const trigger = entries[idx] || entries[0] || s.project?.build_file || 'System Entry Trigger';
    const summary = isMermaid
      ? 'End-to-end execution path across subsystem boundaries.'
      : flow.trim();
    const first = components[0];
    const mid = components[Math.floor(components.length / 2)] || first;
    const last = components[components.length - 1] || first;
    return {
      title: isMermaid ? `Runtime Scenario ${idx + 1}` : summary.slice(0, 68),
      trigger,
      invariant: 'Boundary Validation & State Consistency',
      summary,
      diagram: isMermaid ? flow : undefined,
      steps: [
        {
          phase: 'Admission',
          layer: first?.layer || 'API / Transport Boundary',
          title: 'Boundary Admission & Input Validation',
          detail: `Receives trigger (${trigger}) and validates request preconditions.`,
          path: first?.path || files[0]?.path,
        },
        {
          phase: 'Execution',
          layer: mid?.layer || 'Application / Orchestration',
          title: 'Core Workflow Orchestration',
          detail: summary,
          path: mid?.path || files[0]?.path,
        },
        {
          phase: 'Publication',
          layer: last?.layer || 'Domain & Persistence',
          title: 'State Publication & Side-Effect Guard',
          detail: 'Commits verified state transitions and returns structured output.',
          path: last?.path || files[0]?.path,
        },
      ],
    };
  });

  const decisions: ProjectDocsDecision[] = [];
  if (analysis?.engineering_insight?.mechanism) {
    decisions.push({
      title: 'Core Architectural Mechanism',
      category: 'System Architecture',
      decision: `${analysis.engineering_insight.mechanism} — ${analysis.engineering_insight.why_it_matters_here || ''}`,
      tradeoff:
        analysis.engineering_insight.tradeoff_or_failure_mode ||
        'Requires strict boundary adherence across callers.',
    });
  }
  for (const risk of analysis?.risks || []) {
    if (decisions.length >= 4) break;
    decisions.push({
      title: `Operational Safeguard (${risk.severity.toUpperCase()})`,
      category: 'Quality & Resilience',
      decision: risk.engineering_insight?.mechanism || risk.summary,
      tradeoff:
        risk.engineering_insight?.tradeoff_or_failure_mode ||
        'Requires test verification and explicit error handling.',
    });
  }
  if (!decisions.length) {
    decisions.push({
      title: 'Layered Subsystem Isolation',
      category: 'Architecture Boundary',
      decision: `Separates responsibilities across ${Math.max(1, components.length)} bounded subsystems.`,
      tradeoff: 'Cross-cutting changes must coordinate explicit contracts across layers.',
    });
  }

  const runbook: ProjectDocsRunbookItem[] = [
    {
      label: 'Standard Verification Suite',
      command: s.project?.type === 'go' ? 'go test ./...' : 'make check',
      description: 'Executes automated unit and integration checks across the repository.',
    },
    {
      label: 'Repository Status Check',
      command: 'git status --short',
      description: 'Verifies working tree cleanliness before and after changes.',
    },
  ];

  const glossary: ProjectDocsGlossaryItem[] = components.slice(0, 6).map((c) => ({
    term: c.name,
    definition: `${c.layer} (${c.path}): ${c.summary}`,
  }));

  const stack = Object.entries(s.project?.languages || {}).map(
    ([lang, count]) => `${lang} (${count} files)`,
  );
  if (s.project?.type && s.project.type !== 'unknown') {
    stack.unshift(
      s.project.build_file ? `${s.project.type} (${s.project.build_file})` : s.project.type,
    );
  }

  const arch = analysis?.architecture || '';
  const archIsMermaid = /^(flowchart|graph|sequenceDiagram)\b/.test(arch.trim());

  return {
    schema_version: '1',
    project_id: s.project?.project_id || '',
    project_revision: s.project?.project_revision || '',
    status: analysis?.status || s.project?.ai_status || 'fresh',
    context: {
      purpose:
        analysis?.purpose ||
        s.project?.summary ||
        `${s.project?.name || 'Project'} software system.`,
      topology: archIsMermaid
        ? `Layered architecture spanning ${components.length} bounded subsystems.`
        : arch || `Layered architecture spanning ${components.length} bounded subsystems.`,
      stack: stack.length ? stack : ['Source Repository'],
      architecture_diagram: archIsMermaid ? arch : undefined,
    },
    components,
    scenarios,
    decisions,
    runbook,
    glossary,
    updated_at: s.project?.analyzed_at || '',
  };
}

function resolveFileForPath(s: State, targetPath?: string): string | undefined {
  if (!targetPath) return undefined;
  const files = s.index?.files || [];
  const exact = files.find((f) => f.path === targetPath);
  if (exact) return exact.path;
  const prefix = files.find((f) => f.path.startsWith(`${targetPath}/`));
  return prefix?.path || files[0]?.path;
}

export function DocsWorkspace({ s }: { s: State }) {
  const [tab, setTab] = useState<DocsTab>('context');
  const [query, setQuery] = useState('');
  const [selectedComponent, setSelectedComponent] = useState(0);

  const docs = useMemo(() => {
    const serverDocs = s.overview?.docs;
    if (serverDocs && serverDocs.components?.length) return serverDocs;
    return buildFallbackDocs(s);
  }, [s.overview, s.project, s.index]);

  const q = query.trim().toLowerCase();

  const filteredComponents = useMemo(
    () =>
      docs.components.filter(
        (c) =>
          !q ||
          c.name.toLowerCase().includes(q) ||
          c.layer.toLowerCase().includes(q) ||
          c.path.toLowerCase().includes(q) ||
          c.summary.toLowerCase().includes(q) ||
          c.contracts.some(
            (ct) => ct.name.toLowerCase().includes(q) || ct.summary.toLowerCase().includes(q),
          ),
      ),
    [docs.components, q],
  );

  const filteredScenarios = useMemo(
    () =>
      docs.scenarios.filter(
        (sc) =>
          !q ||
          sc.title.toLowerCase().includes(q) ||
          sc.summary.toLowerCase().includes(q) ||
          sc.trigger.toLowerCase().includes(q) ||
          sc.steps.some(
            (st) => st.title.toLowerCase().includes(q) || st.detail.toLowerCase().includes(q),
          ),
      ),
    [docs.scenarios, q],
  );

  const filteredDecisions = useMemo(
    () =>
      docs.decisions.filter(
        (d) =>
          !q ||
          d.title.toLowerCase().includes(q) ||
          d.category.toLowerCase().includes(q) ||
          d.decision.toLowerCase().includes(q) ||
          d.tradeoff.toLowerCase().includes(q),
      ),
    [docs.decisions, q],
  );

  const filteredGlossary = useMemo(
    () =>
      docs.glossary.filter(
        (g) => !q || g.term.toLowerCase().includes(q) || g.definition.toLowerCase().includes(q),
      ),
    [docs.glossary, q],
  );

  const layers = useMemo(() => {
    const map = new Map<string, { comp: ProjectDocsComponent; index: number }[]>();
    docs.components.forEach((comp, index) => {
      const list = map.get(comp.layer) || [];
      list.push({ comp, index });
      map.set(comp.layer, list);
    });
    return Array.from(map.entries());
  }, [docs.components]);

  const activeComp =
    filteredComponents[Math.min(selectedComponent, Math.max(0, filteredComponents.length - 1))] ||
    docs.components[0];

  const openPath = (path?: string) => {
    const resolved = resolveFileForPath(s, path);
    if (resolved) void w.openFile(resolved);
  };

  return (
    <div className="workspace-page docs-workspace">
      <Heading
        variant="intro"
        title="Engineering Specification"
        detail={
          <span className="row wrap" style={{ gap: '8px' }}>
            <StatusDot value={docs.status} label="Specification" />
            <span>
              C4 System Context · Subsystem Contracts · Runtime Scenarios · ADRs &amp; Runbook
            </span>
          </span>
        }
      >
        <Button
          tone="primary"
          icon="play"
          disabled={!!s.busy || activeRun(s.run)}
          loading={activeRun(s.run)}
          onClick={() => void w.runAnalysisDirect(true)}
        >
          {activeRun(s.run) ? 'Analyzing…' : 'Run Analysis'}
        </Button>
      </Heading>

      {/* 4 C4/Arc42 Specification Section Boxes */}
      <div
        className="kpi-pulse-grid box-grid-4"
        role="tablist"
        aria-label="Engineering specification sections"
      >
        <div
          className={`kpi-pulse-card card-kpi box-card clickable ${tab === 'context' ? 'active-box' : ''}`}
          role="tab"
          tabIndex={0}
          aria-selected={tab === 'context'}
          onClick={() => setTab('context')}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              setTab('context');
            }
          }}
        >
          <div className="box-head">
            <span className="box-eyebrow">
              <Icon name="layers" /> 1. System Context
            </span>
            <span className="pill">{human(docs.status)}</span>
          </div>
          <div className="kpi-pulse-value box-value">{Math.max(1, layers.length)} Layers</div>
          <div className="box-meta">
            <span>Topology &amp; Tech Stack</span>
          </div>
        </div>

        <div
          className={`kpi-pulse-card card-kpi box-card clickable ${tab === 'components' ? 'active-box' : ''}`}
          role="tab"
          tabIndex={0}
          aria-selected={tab === 'components'}
          onClick={() => setTab('components')}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              setTab('components');
            }
          }}
        >
          <div className="box-head">
            <span className="box-eyebrow">
              <Icon name="grid" /> 2. Components
            </span>
          </div>
          <div className="kpi-pulse-value box-value">{docs.components.length} Subsystems</div>
          <div className="box-meta">
            <span>Boundaries &amp; Contracts</span>
          </div>
        </div>

        <div
          className={`kpi-pulse-card card-kpi box-card clickable ${tab === 'scenarios' ? 'active-box' : ''}`}
          role="tab"
          tabIndex={0}
          aria-selected={tab === 'scenarios'}
          onClick={() => setTab('scenarios')}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              setTab('scenarios');
            }
          }}
        >
          <div className="box-head">
            <span className="box-eyebrow">
              <Icon name="activity" /> 3. Runtime Scenarios
            </span>
          </div>
          <div className="kpi-pulse-value box-value">{docs.scenarios.length} Data Flows</div>
          <div className="box-meta">
            <span>End-to-End Sequences</span>
          </div>
        </div>

        <div
          className={`kpi-pulse-card card-kpi box-card clickable ${tab === 'decisions' ? 'active-box' : ''}`}
          role="tab"
          tabIndex={0}
          aria-selected={tab === 'decisions'}
          onClick={() => setTab('decisions')}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              setTab('decisions');
            }
          }}
        >
          <div className="box-head">
            <span className="box-eyebrow">
              <Icon name="shield" /> 4. Decisions &amp; Ops
            </span>
          </div>
          <div className="kpi-pulse-value box-value">{docs.decisions.length} ADRs</div>
          <div className="box-meta">
            <span>Tradeoffs · Runbook · Glossary</span>
          </div>
        </div>
      </div>

      <div className="toolbar">
        <div className="input-wrap" style={{ flex: 1 }}>
          <Icon name="search" />
          <input
            aria-label="Filter engineering specification"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setSelectedComponent(0);
            }}
            placeholder="Filter components, contracts, runtime scenarios, ADRs, or domain terms…"
          />
        </div>
      </div>

      {/* TAB 1: SYSTEM CONTEXT & LAYERED TOPOLOGY */}
      {tab === 'context' && (
        <div className="stack">
          <div className="kpi-pulse-grid box-grid-3">
            <div className="kpi-pulse-card card-kpi box-card">
              <div className="box-head">
                <span className="box-eyebrow">System Purpose</span>
                <span className="pill">C4 Level 1</span>
              </div>
              <p style={{ fontSize: '0.88rem', lineHeight: 1.55 }}>{docs.context.purpose}</p>
            </div>

            <div className="kpi-pulse-card card-kpi box-card">
              <div className="box-head">
                <span className="box-eyebrow">Runtime Topology</span>
                <span className="pill">C4 Level 2</span>
              </div>
              <p style={{ fontSize: '0.88rem', lineHeight: 1.55 }}>{docs.context.topology}</p>
            </div>

            <div className="kpi-pulse-card card-kpi box-card">
              <div className="box-head">
                <span className="box-eyebrow">Technology &amp; Storage Stack</span>
                <span className="pill">Runtime</span>
              </div>
              <div className="row wrap" style={{ gap: '6px', marginTop: '4px' }}>
                {docs.context.stack.map((item) => (
                  <span key={item} className="pill font-mono">
                    {item}
                  </span>
                ))}
              </div>
            </div>
          </div>

          <Panel
            title="Layered Architecture Map"
            actions={
              <span className="small muted">Click any subsystem to inspect its contracts</span>
            }
          >
            <div className="arch-diagram-box">
              {layers.map(([layerName, items]) => (
                <div key={layerName} className="arch-tier">
                  <div className="arch-tier-label">{layerName}</div>
                  <div className="arch-tier-nodes">
                    {items.map(({ comp, index }) => (
                      <button
                        key={comp.path}
                        type="button"
                        className="arch-node-pill"
                        onClick={() => {
                          setSelectedComponent(index);
                          setTab('components');
                        }}
                      >
                        <Icon name="code" />
                        <strong>{comp.name}</strong>
                        <small className="muted font-mono">{comp.path}</small>
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </Panel>

          {docs.context.architecture_diagram && (
            <Panel title="System Dependency Graph">
              <Prose
                text={docs.context.architecture_diagram}
                diagramLabel="System architecture diagram"
              />
            </Panel>
          )}
        </div>
      )}

      {/* TAB 2: COMPONENTS & CONTRACTS */}
      {tab === 'components' && (
        <div className="grid two-columns docs-components-layout">
          <div className="stack">
            {filteredComponents.length === 0 && <Empty title="No matching components" />}
            {filteredComponents.map((comp, idx) => {
              const selected = activeComp?.path === comp.path;
              return (
                <div
                  key={comp.path}
                  className={`kpi-pulse-card card-kpi box-card clickable ${selected ? 'active-box' : ''}`}
                  role="button"
                  tabIndex={0}
                  onClick={() => setSelectedComponent(idx)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      setSelectedComponent(idx);
                    }
                  }}
                >
                  <div className="box-head">
                    <strong style={{ fontSize: '0.96rem' }}>{comp.name}</strong>
                    <span className="pill">{comp.layer}</span>
                  </div>
                  <p className="small muted" style={{ lineHeight: 1.5 }}>
                    {comp.summary}
                  </p>
                  <div className="box-meta">
                    <span className="font-mono">{comp.path}</span>
                    <span>{comp.contracts.length} contracts →</span>
                  </div>
                </div>
              );
            })}
          </div>

          {activeComp && (
            <Panel
              title={activeComp.name}
              className="docs-component-inspector"
              actions={
                <Button tone="primary small" icon="code" onClick={() => openPath(activeComp.path)}>
                  Open Source
                </Button>
              }
            >
              <div className="stack">
                <div className="row wrap" style={{ gap: '8px' }}>
                  <span className="pill">{activeComp.layer}</span>
                  <code className="small muted">{activeComp.path}</code>
                </div>
                <p style={{ fontSize: '0.88rem', lineHeight: 1.55 }}>{activeComp.summary}</p>

                <div>
                  <h4 className="box-eyebrow" style={{ marginBottom: '8px' }}>
                    Domain Responsibilities
                  </h4>
                  <div className="stack" style={{ gap: '6px' }}>
                    {activeComp.responsibilities.map((resp, i) => (
                      <div key={i} className="studio-list-row" style={{ cursor: 'default' }}>
                        <StatusDot value="fresh" />
                        <span className="small">{resp}</span>
                      </div>
                    ))}
                  </div>
                </div>

                <div>
                  <h4 className="box-eyebrow" style={{ marginBottom: '8px' }}>
                    Public Contracts &amp; Interfaces
                  </h4>
                  <div className="stack" style={{ gap: '8px' }}>
                    {activeComp.contracts.map((contract, i) => (
                      <div
                        key={i}
                        className="panel"
                        style={{ padding: '10px 12px', background: 'var(--raised)' }}
                      >
                        <div className="row between wrap">
                          <strong className="font-mono" style={{ fontSize: '0.82rem' }}>
                            {contract.name}
                          </strong>
                          <span className="pill">{contract.kind}</span>
                        </div>
                        <p className="small muted" style={{ marginTop: '4px' }}>
                          {contract.summary}
                        </p>
                      </div>
                    ))}
                  </div>
                </div>

                {activeComp.dependencies.length > 0 && (
                  <div>
                    <h4 className="box-eyebrow" style={{ marginBottom: '8px' }}>
                      Upstream / Downstream Collaborators
                    </h4>
                    <div className="row wrap" style={{ gap: '6px' }}>
                      {activeComp.dependencies.map((dep) => (
                        <span key={dep} className="pill">
                          → {dep}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </Panel>
          )}
        </div>
      )}

      {/* TAB 3: RUNTIME SCENARIOS & DATA FLOWS */}
      {tab === 'scenarios' && (
        <div className="stack">
          {filteredScenarios.length === 0 && <Empty title="No matching runtime scenarios" />}
          {filteredScenarios.map((scenario, idx) => (
            <Panel
              key={idx}
              title={`Scenario ${idx + 1}: ${scenario.title}`}
              actions={
                <div className="row wrap" style={{ gap: '6px' }}>
                  <span className="pill">Trigger: {scenario.trigger}</span>
                  <span className="pill">Invariant: {scenario.invariant}</span>
                </div>
              }
            >
              <div className="stack">
                <p style={{ fontSize: '0.88rem', lineHeight: 1.55 }}>{scenario.summary}</p>
                <div className="kpi-pulse-grid box-grid-3">
                  {scenario.steps.map((step, sIdx) => (
                    <div
                      key={sIdx}
                      className="kpi-pulse-card card-kpi box-card clickable"
                      role="button"
                      tabIndex={0}
                      onClick={() => openPath(step.path)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          openPath(step.path);
                        }
                      }}
                    >
                      <div className="box-head">
                        <span className="box-eyebrow">
                          Step {sIdx + 1} · {step.phase}
                        </span>
                        <span className="pill">{step.layer}</span>
                      </div>
                      <strong style={{ fontSize: '0.9rem' }}>{step.title}</strong>
                      <p className="small muted" style={{ lineHeight: 1.45 }}>
                        {step.detail}
                      </p>
                      {step.path && (
                        <div className="box-meta">
                          <span className="font-mono">{step.path}</span>
                          <Icon name="arrow" />
                        </div>
                      )}
                    </div>
                  ))}
                </div>
                {scenario.diagram && (
                  <Prose text={scenario.diagram} diagramLabel={`${scenario.title} diagram`} />
                )}
              </div>
            </Panel>
          ))}
        </div>
      )}

      {/* TAB 4: DECISIONS (ADRs), RUNBOOK & GLOSSARY */}
      {tab === 'decisions' && (
        <div className="grid two-columns">
          <div className="stack">
            <Panel title="Architectural Decision Records (ADRs)">
              <div className="stack">
                {filteredDecisions.length === 0 && <Empty title="No matching ADRs" />}
                {filteredDecisions.map((adr, idx) => (
                  <div
                    key={idx}
                    className="kpi-pulse-card card-kpi box-card"
                    style={{ background: 'var(--raised)' }}
                  >
                    <div className="box-head">
                      <span className="box-eyebrow">
                        ADR-{String(idx + 1).padStart(2, '0')} · {adr.category}
                      </span>
                    </div>
                    <strong style={{ fontSize: '0.95rem' }}>{adr.title}</strong>
                    <p className="small" style={{ lineHeight: 1.5 }}>
                      <strong>Decision:</strong> {adr.decision}
                    </p>
                    <p className="small muted" style={{ lineHeight: 1.5 }}>
                      <strong>Tradeoff / Constraint:</strong> {adr.tradeoff}
                    </p>
                  </div>
                ))}
              </div>
            </Panel>
          </div>

          <div className="stack">
            <Panel title="Operational Runbook">
              <div className="stack">
                {docs.runbook.map((item, idx) => (
                  <div
                    key={idx}
                    className="kpi-pulse-card card-kpi box-card"
                    style={{ background: 'var(--raised)' }}
                  >
                    <div className="box-head">
                      <strong>{item.label}</strong>
                      <code className="pill font-mono">{item.command}</code>
                    </div>
                    <p className="small muted">{item.description}</p>
                  </div>
                ))}
              </div>
            </Panel>

            <Panel title="Ubiquitous Domain Language (Glossary)">
              <div className="stack">
                {filteredGlossary.length === 0 && <Empty title="No matching glossary terms" />}
                {filteredGlossary.map((item, idx) => (
                  <div
                    key={idx}
                    className="panel"
                    style={{ padding: '10px 12px', background: 'var(--raised)' }}
                  >
                    <strong className="font-mono" style={{ fontSize: '0.84rem' }}>
                      {item.term}
                    </strong>
                    <p className="small muted" style={{ marginTop: '4px', lineHeight: 1.45 }}>
                      {item.definition}
                    </p>
                  </div>
                ))}
              </div>
            </Panel>
          </div>
        </div>
      )}
    </div>
  );
}
