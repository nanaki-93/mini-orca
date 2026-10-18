import { useMemo, useState } from 'react';
import { activeRun, workspace as w, type State } from './workspace';
import type {
  ProjectDocsComponent,
  ProjectDocsDecision,
  ProjectDocsDependency,
  ProjectDocsGlossaryItem,
  ProjectDocsReport,
  ProjectDocsRunbookItem,
  ProjectDocsScenario,
} from './models';
import { Badge, Button, Disclosure, Empty, Heading, Icon, Panel, Prose, StatusDot } from './ui';

type DocsTab = 'context' | 'components' | 'scenarios' | 'decisions' | 'handbook';

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
  const topology = archIsMermaid
    ? `Layered architecture spanning ${components.length} bounded subsystems.`
    : arch || `Layered architecture spanning ${components.length} bounded subsystems.`;

  const goalPursuit: string[] = [];
  if (analysis?.engineering_insight?.mechanism) {
    const why = analysis.engineering_insight.why_it_matters_here;
    goalPursuit.push(
      why
        ? `${analysis.engineering_insight.mechanism} — ${why}`
        : analysis.engineering_insight.mechanism,
    );
  }
  for (const flow of flows) {
    if (goalPursuit.length >= 4) break;
    if (!/^(flowchart|graph|sequenceDiagram)\b/.test(flow.trim())) {
      goalPursuit.push(flow.trim());
    }
  }
  if (entries.length && goalPursuit.length < 4) {
    goalPursuit.push(`Admits execution through entry points: ${entries.slice(0, 3).join(', ')}.`);
  }
  if (!goalPursuit.length) {
    goalPursuit.push(topology);
  }

  const dependencies: ProjectDocsDependency[] = [];
  const seenDeps = new Set<string>();
  const addDep = (dep: ProjectDocsDependency) => {
    const key = dep.name.toLowerCase();
    if (!key || seenDeps.has(key) || dependencies.length >= 10) return;
    seenDeps.add(key);
    dependencies.push(dep);
  };

  if (s.project?.type && s.project.type !== 'unknown') {
    addDep({
      name: s.project.type,
      kind: 'Runtime / Toolchain',
      source: s.project.build_file || 'project manifest',
      purpose: `Primary language runtime and build system (${s.project.build_file || s.project.type}).`,
    });
  }
  for (const file of files) {
    for (const imp of file.imports || []) {
      if (dependencies.length >= 8) break;
      addDep({
        name: imp,
        kind:
          imp.includes('.') || imp.startsWith('@')
            ? 'External Package'
            : 'Standard / Internal Package',
        source: file.path,
        purpose: `Imported by ${file.path} for ${classifyLayer(file.path, imp).toLowerCase()}.`,
      });
    }
  }
  for (const comp of components) {
    for (const dep of comp.dependencies || []) {
      addDep({
        name: dep,
        kind: 'Internal Subsystem',
        source: comp.path || comp.name,
        purpose: `Collaborates with ${comp.name} (${comp.layer}).`,
      });
    }
  }

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
      goal_pursuit: goalPursuit,
      topology,
      stack: stack.length ? stack : ['Source Repository'],
      architecture_diagram: archIsMermaid ? arch : undefined,
    },
    dependencies,
    components,
    scenarios,
    decisions,
    runbook,
    glossary,
    updated_at: s.project?.analyzed_at || '',
  };
}

export function resolveProjectDocs(s: State): ProjectDocsReport {
  const fallback = buildFallbackDocs(s);
  const serverDocs = s.overview?.docs;
  if (!serverDocs || !serverDocs.components?.length) return fallback;
  return {
    ...serverDocs,
    context: {
      ...serverDocs.context,
      goal_pursuit: serverDocs.context.goal_pursuit?.length
        ? serverDocs.context.goal_pursuit
        : fallback.context.goal_pursuit,
    },
    dependencies: serverDocs.dependencies?.length ? serverDocs.dependencies : fallback.dependencies,
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

export function countComponentFindings(s: State, compPath: string): number {
  if (!compPath) return 0;
  const norm = compPath.replace(/\/+$/, '');
  const matches = (path?: string) =>
    !!path && (path === norm || path.startsWith(`${norm}/`) || norm.startsWith(`${path}/`));
  const seen = new Set<string>();
  for (const f of s.findings || []) {
    if (matches(f.location?.path)) seen.add(`sem:${f.id}`);
  }
  for (const cat of ['bugs', 'performance', 'security'] as const) {
    const res = s.results?.[cat];
    for (const f of res?.semantic || []) {
      if (matches(f.location?.path)) seen.add(`sem:${f.id}`);
    }
    for (const r of res?.performance || []) {
      if (matches(r.path)) {
        for (const f of r.findings || []) seen.add(`perf:${r.path}:${f.id}`);
      }
    }
    for (const r of res?.security || []) {
      for (const f of r.findings || []) {
        const p = f.source_anchor?.path || r.path;
        if (matches(p)) seen.add(`sec:${p}:${f.id}`);
      }
    }
  }
  return seen.size;
}

export function formatArchitectureMarkdown(
  docs: ProjectDocsReport,
  projectName = 'Project',
): string {
  const lines: string[] = [
    `# ${projectName} Architecture & Engineering Handbook`,
    '',
    `> Generated by Mini-Orca Engineering Specification · Schema v${docs.schema_version || '1'}`,
    '',
    '## 1. System Purpose & Goal Pursuit',
    '',
    docs.context.purpose,
    '',
    `**Stack:** ${(docs.context.stack || []).join(', ')}`,
    `**Topology:** ${docs.context.topology}`,
    '',
  ];
  if (docs.context.goal_pursuit?.length) {
    lines.push('### How It Pursues the Goal', '');
    docs.context.goal_pursuit.forEach((step, idx) => {
      lines.push(`${idx + 1}. ${step}`);
    });
    lines.push('');
  }
  if (docs.context.architecture_diagram) {
    lines.push('```mermaid', docs.context.architecture_diagram.trim(), '```', '');
  }
  lines.push('## 2. Subsystems & Public Contracts', '');
  for (const comp of docs.components) {
    lines.push(
      `### ${comp.name} (\`${comp.path}\`)`,
      `- **Layer:** ${comp.layer}`,
      `- **Summary:** ${comp.summary}`,
    );
    if (comp.dependencies?.length) {
      lines.push(`- **Dependencies:** ${comp.dependencies.join(', ')}`);
    }
    if (comp.contracts?.length) {
      lines.push('- **Contracts:**');
      for (const ct of comp.contracts) {
        lines.push(`  - \`${ct.name}\` (${ct.kind}): ${ct.summary}`);
      }
    }
    lines.push('');
  }
  lines.push('## 3. Runtime Scenarios', '');
  docs.scenarios.forEach((sc, idx) => {
    lines.push(
      `### Scenario ${idx + 1}: ${sc.title}`,
      `- **Trigger:** \`${sc.trigger}\``,
      '',
      sc.summary,
      '',
    );
    sc.steps.forEach((st, sIdx) => {
      lines.push(`${sIdx + 1}. **[${st.phase} · ${st.layer}] ${st.title}** — ${st.detail}`);
    });
    lines.push('');
  });
  lines.push('## 4. Architectural Decision Records (ADRs)', '');
  for (const adr of docs.decisions) {
    lines.push(
      `### ${adr.title} (${adr.category})`,
      `- **Decision:** ${adr.decision}`,
      `- **Tradeoff:** ${adr.tradeoff}`,
      '',
    );
  }
  if (docs.runbook?.length) {
    lines.push('## 5. Operational Runbook', '');
    for (const item of docs.runbook) {
      lines.push(`- **${item.label}** (\`${item.command}\`): ${item.description}`);
    }
    lines.push('');
  }
  return lines.join('\n');
}

export function DocsWorkspace({ s }: { s: State }) {
  const [tab, setTab] = useState<DocsTab>('context');
  const [query, setQuery] = useState('');
  const [selectedComponent, setSelectedComponent] = useState(0);
  const [handbookSection, setHandbookSection] = useState<
    'purpose' | 'topology' | 'components' | 'scenarios' | 'decisions'
  >('purpose');

  const docs = useMemo(() => resolveProjectDocs(s), [s.overview, s.project, s.index]);

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

  const filteredDependencies = useMemo(
    () =>
      (docs.dependencies || []).filter(
        (d) =>
          !q ||
          d.name.toLowerCase().includes(q) ||
          d.kind.toLowerCase().includes(q) ||
          d.source.toLowerCase().includes(q) ||
          d.purpose.toLowerCase().includes(q),
      ),
    [docs.dependencies, q],
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

  const entryPoints = s.overview?.analysis.entry_points || [];
  const goalPursuit = docs.context.goal_pursuit || [];

  const syncArchitectureDoc = () => {
    const md = formatArchitectureMarkdown(docs, s.project?.name || 'Project');
    void w.saveDirectFile('docs/ARCHITECTURE.md', md);
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
              Purpose · Goal Pursuit · Architecture · Main Dependencies · Runtime &amp; Ops
            </span>
          </span>
        }
      >
        <Button tone="ghost" icon="file" disabled={!!s.busy} onClick={syncArchitectureDoc}>
          Sync to docs/ARCHITECTURE.md
        </Button>
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

      <div className="toolbar">
        <div
          className="mode-switch-group"
          role="tablist"
          aria-label="Engineering specification sections"
        >
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'context'}
            className={`mode-switch-btn ${tab === 'context' ? 'active' : ''}`}
            onClick={() => setTab('context')}
          >
            1. Overview &amp; Architecture ({Math.max(1, layers.length)})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'components'}
            className={`mode-switch-btn ${tab === 'components' ? 'active' : ''}`}
            onClick={() => setTab('components')}
          >
            2. Components ({docs.components.length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'scenarios'}
            className={`mode-switch-btn ${tab === 'scenarios' ? 'active' : ''}`}
            onClick={() => setTab('scenarios')}
          >
            3. Runtime Scenarios ({docs.scenarios.length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'decisions'}
            className={`mode-switch-btn ${tab === 'decisions' ? 'active' : ''}`}
            onClick={() => setTab('decisions')}
          >
            4. Decisions &amp; Ops ({docs.decisions.length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'handbook'}
            className={`mode-switch-btn ${tab === 'handbook' ? 'active' : ''}`}
            onClick={() => setTab('handbook')}
          >
            5. Handbook Reader
          </button>
        </div>

        <div className="input-wrap" style={{ flex: 1 }}>
          <Icon name="search" />
          <input
            aria-label="Filter engineering specification"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setSelectedComponent(0);
            }}
            placeholder="Filter purpose, architecture, dependencies, contracts, scenarios, or ADRs…"
          />
        </div>
      </div>

      {/* TAB 1: PURPOSE, HOW IT PURSUES THE GOAL, ARCHITECTURE, DEPENDENCIES & OTHER INFO */}
      {tab === 'context' && (
        <div className="stack">
          <div className="grid two-columns">
            <Panel title="Purpose of the Project">
              <div className="stack" style={{ gap: '10px' }}>
                <p style={{ fontSize: '0.92rem', lineHeight: 1.55 }}>{docs.context.purpose}</p>
                <div className="row wrap" style={{ gap: '6px' }}>
                  {docs.context.stack.map((item) => (
                    <span key={item} className="pill font-mono">
                      {item}
                    </span>
                  ))}
                </div>
                {entryPoints.length > 0 && (
                  <div className="row wrap" style={{ gap: '6px', marginTop: '2px' }}>
                    <span className="small muted">Entry points:</span>
                    {entryPoints.map((ep) => (
                      <button
                        key={ep}
                        type="button"
                        className="pill font-mono"
                        onClick={() => openPath(ep)}
                      >
                        {ep}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </Panel>

            <Panel title="How It Pursues the Goal">
              <div className="stack" style={{ gap: '8px' }}>
                {goalPursuit.map((step, idx) => (
                  <div
                    key={idx}
                    className="studio-list-row"
                    style={{ cursor: 'default', alignItems: 'flex-start' }}
                  >
                    <span className="pill font-mono" style={{ flexShrink: 0 }}>
                      0{idx + 1}
                    </span>
                    <span className="small" style={{ lineHeight: 1.5 }}>
                      {step}
                    </span>
                  </div>
                ))}
              </div>
            </Panel>
          </div>

          <Panel
            title="Layered Architecture Map"
            actions={
              <span className="small muted">
                {docs.context.topology} · Click any subsystem to inspect its contracts
              </span>
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
            <Disclosure title="System Dependency Graph">
              <Prose
                text={docs.context.architecture_diagram}
                diagramLabel="System architecture diagram"
              />
            </Disclosure>
          )}

          <div className="grid two-columns">
            <Panel title={`Main Dependencies (${filteredDependencies.length})`}>
              <div className="stack" style={{ gap: '8px' }}>
                {filteredDependencies.length === 0 && <Empty title="No matching dependencies" />}
                {filteredDependencies.map((dep, idx) => (
                  <div
                    key={`${dep.name}-${idx}`}
                    className="panel"
                    style={{ padding: '10px 12px', background: 'var(--raised)' }}
                  >
                    <div className="row between wrap" style={{ gap: '8px' }}>
                      <span className="row wrap" style={{ gap: '6px' }}>
                        <strong className="font-mono" style={{ fontSize: '0.84rem' }}>
                          {dep.name}
                        </strong>
                        {dep.version && <code className="pill font-mono">{dep.version}</code>}
                      </span>
                      <span className="pill">{dep.kind}</span>
                    </div>
                    <p className="small muted" style={{ marginTop: '4px', lineHeight: 1.45 }}>
                      {dep.purpose}
                    </p>
                  </div>
                ))}
              </div>
            </Panel>

            <Panel title="Other Project Info &amp; Operations">
              <div className="stack" style={{ gap: '8px' }}>
                {docs.runbook.slice(0, 3).map((item, idx) => (
                  <div
                    key={idx}
                    className="panel"
                    style={{ padding: '10px 12px', background: 'var(--raised)' }}
                  >
                    <div className="row between wrap">
                      <strong>{item.label}</strong>
                      <code className="pill font-mono">{item.command}</code>
                    </div>
                    <p className="small muted" style={{ marginTop: '4px' }}>
                      {item.description}
                    </p>
                  </div>
                ))}
                <div className="row wrap" style={{ gap: '8px', marginTop: '4px' }}>
                  <Button tone="ghost small" onClick={() => setTab('components')}>
                    Inspect {docs.components.length} Subsystems →
                  </Button>
                  <Button tone="ghost small" onClick={() => setTab('scenarios')}>
                    Inspect {docs.scenarios.length} Scenarios →
                  </Button>
                  <Button tone="ghost small" onClick={() => setTab('decisions')}>
                    Inspect {docs.decisions.length} ADRs &amp; Glossary →
                  </Button>
                </div>
              </div>
            </Panel>
          </div>
        </div>
      )}

      {/* TAB 2: COMPONENTS & CONTRACTS */}
      {tab === 'components' && (
        <div className="grid two-columns docs-components-layout">
          <Panel title={`Subsystems (${filteredComponents.length})`}>
            <div className="stack" style={{ gap: '6px' }}>
              {filteredComponents.length === 0 && <Empty title="No matching components" />}
              {filteredComponents.map((comp, idx) => {
                const selected = activeComp?.path === comp.path;
                const issueCount = countComponentFindings(s, comp.path);
                return (
                  <button
                    key={comp.path}
                    type="button"
                    className={`studio-list-row ${selected ? 'active' : ''}`}
                    style={{
                      border: selected ? '1px solid var(--accent)' : '1px solid var(--line)',
                      background: selected ? 'var(--accent-soft)' : 'var(--raised)',
                    }}
                    onClick={() => setSelectedComponent(idx)}
                  >
                    <span className="studio-row-copy">
                      <strong>{comp.name}</strong>
                      <small className="font-mono muted">{comp.path}</small>
                    </span>
                    <span className="row wrap" style={{ gap: '4px' }}>
                      <Badge
                        value={issueCount === 0 ? 'Healthy' : `${issueCount} issues`}
                        tone={issueCount === 0 ? 'green' : 'amber'}
                      />
                      <span className="pill">{comp.layer}</span>
                    </span>
                  </button>
                );
              })}
            </div>
          </Panel>

          {activeComp && (
            <Panel
              title={activeComp.name}
              className="docs-component-inspector"
              actions={
                <div className="row wrap" style={{ gap: '6px' }}>
                  {(() => {
                    const count = countComponentFindings(s, activeComp.path);
                    return (
                      <Badge
                        value={count === 0 ? '0 open issues' : `${count} open issues`}
                        tone={count === 0 ? 'green' : 'amber'}
                      />
                    );
                  })()}
                  <Button
                    tone="primary small"
                    icon="code"
                    onClick={() => openPath(activeComp.path)}
                  >
                    Open Source
                  </Button>
                </div>
              }
            >
              <div className="stack">
                <div className="row wrap" style={{ gap: '8px' }}>
                  <span className="pill">{activeComp.layer}</span>
                  <code className="small muted">{activeComp.path}</code>
                </div>
                <p style={{ fontSize: '0.88rem', lineHeight: 1.55 }}>{activeComp.summary}</p>

                {(() => {
                  const outbound = activeComp.dependencies || [];
                  const inbound = docs.components
                    .filter(
                      (c) =>
                        c.path !== activeComp.path &&
                        (c.dependencies || []).some(
                          (d) =>
                            d.toLowerCase() === activeComp.name.toLowerCase() ||
                            d.toLowerCase() === activeComp.path.toLowerCase(),
                        ),
                    )
                    .map((c) => c.name);
                  if (!outbound.length && !inbound.length) return null;
                  return (
                    <div>
                      <h4 className="box-eyebrow" style={{ marginBottom: '8px' }}>
                        Subsystem Dependency Graph
                      </h4>
                      <div className="row wrap" style={{ gap: '6px' }}>
                        {outbound.map((dep) => (
                          <Badge key={`out-${dep}`} value={`Calls → ${dep}`} tone="violet" />
                        ))}
                        {inbound.map((caller) => (
                          <Badge
                            key={`in-${caller}`}
                            value={`Called by ← ${caller}`}
                            tone="green"
                          />
                        ))}
                      </div>
                    </div>
                  );
                })()}

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
              actions={<span className="pill">Trigger: {scenario.trigger}</span>}
            >
              <div className="stack">
                <p style={{ fontSize: '0.88rem', lineHeight: 1.55 }}>{scenario.summary}</p>
                <div className="kpi-pulse-grid box-grid-3">
                  {scenario.steps.map((step, sIdx) => (
                    <div
                      key={sIdx}
                      className="panel box-card clickable"
                      style={{ padding: '12px 14px', background: 'var(--raised)' }}
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
                      <strong style={{ fontSize: '0.88rem' }}>{step.title}</strong>
                      <p className="small muted" style={{ lineHeight: 1.45 }}>
                        {step.detail}
                      </p>
                    </div>
                  ))}
                </div>
                {scenario.diagram && (
                  <Disclosure title="Sequence / Flow Diagram">
                    <Prose text={scenario.diagram} diagramLabel={`${scenario.title} diagram`} />
                  </Disclosure>
                )}
              </div>
            </Panel>
          ))}
        </div>
      )}

      {/* TAB 4: DECISIONS (ADRs), RUNBOOK & GLOSSARY */}
      {tab === 'decisions' && (
        <div className="grid two-columns">
          <Panel title="Architectural Decision Records (ADRs)">
            <div className="stack">
              {filteredDecisions.length === 0 && <Empty title="No matching ADRs" />}
              {filteredDecisions.map((adr, idx) => (
                <div
                  key={idx}
                  className="panel"
                  style={{ padding: '12px 14px', background: 'var(--raised)' }}
                >
                  <div className="row between wrap" style={{ marginBottom: '4px' }}>
                    <strong>{adr.title}</strong>
                    <span className="pill">{adr.category}</span>
                  </div>
                  <p className="small" style={{ lineHeight: 1.5 }}>
                    {adr.decision}
                  </p>
                  <p className="small muted" style={{ lineHeight: 1.45, marginTop: '4px' }}>
                    Tradeoff: {adr.tradeoff}
                  </p>
                </div>
              ))}
            </div>
          </Panel>

          <div className="stack">
            <Panel title="Operational Runbook">
              <div className="stack">
                {docs.runbook.map((item, idx) => (
                  <div
                    key={idx}
                    className="panel"
                    style={{ padding: '10px 12px', background: 'var(--raised)' }}
                  >
                    <div className="row between wrap">
                      <strong>{item.label}</strong>
                      <code className="pill font-mono">{item.command}</code>
                    </div>
                    <p className="small muted" style={{ marginTop: '4px' }}>
                      {item.description}
                    </p>
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

      {/* TAB 5: UNIFIED HANDBOOK READER MODE */}
      {tab === 'handbook' && (
        <div
          className="grid"
          style={{ gridTemplateColumns: '240px 1fr', gap: '16px', alignItems: 'start' }}
        >
          <Panel title="Handbook Contents">
            <nav className="stack" style={{ gap: '6px' }} aria-label="Handbook table of contents">
              {(
                [
                  { id: 'purpose', label: '1. System Purpose & Stack' },
                  { id: 'topology', label: `2. Layered Architecture (${layers.length})` },
                  { id: 'components', label: `3. Subsystem Contracts (${docs.components.length})` },
                  { id: 'scenarios', label: `4. Runtime Scenarios (${docs.scenarios.length})` },
                  { id: 'decisions', label: `5. ADRs & Runbook (${docs.decisions.length})` },
                ] as const
              ).map((sec) => (
                <button
                  key={sec.id}
                  type="button"
                  className={`studio-list-row ${handbookSection === sec.id ? 'active' : ''}`}
                  style={{
                    border:
                      handbookSection === sec.id
                        ? '1px solid var(--accent)'
                        : '1px solid var(--line)',
                    background: handbookSection === sec.id ? 'var(--accent-soft)' : 'var(--raised)',
                  }}
                  onClick={() => setHandbookSection(sec.id)}
                >
                  <strong className="small">{sec.label}</strong>
                </button>
              ))}
            </nav>
          </Panel>

          <Panel
            title={`${s.project?.name || 'Project'} Engineering Handbook`}
            actions={<Badge value={`Schema v${docs.schema_version || '1'}`} tone="violet" />}
          >
            <div className="stack" style={{ gap: '14px', maxWidth: '78ch', lineHeight: 1.6 }}>
              <Prose text={formatArchitectureMarkdown(docs, s.project?.name || 'Project')} />
            </div>
          </Panel>
        </div>
      )}
    </div>
  );
}
