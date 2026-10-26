// Docs: the generated engineering handbook, one section at a time, with every
// path linked into the Explorer.

import { asArray, esc, plural, timeAgo } from '../core/util.js';
import { DOC_SECTIONS } from '../core/route.js';
import { icon } from '../ui/icons.js';
import { badge, button, callout, empty } from '../ui/dom.js';

const LABELS = { overview: 'Overview', architecture: 'Architecture', components: 'Components', scenarios: 'Scenarios', decisions: 'Decisions', dependencies: 'Dependencies', runbook: 'Runbook', glossary: 'Glossary' };
const ICONS = { overview: 'home', architecture: 'layers', components: 'box', scenarios: 'flow', decisions: 'scale', dependencies: 'git', runbook: 'terminal', glossary: 'book' };

function docsOf(state) {
  return (state.overview && state.overview.docs) || null;
}

function countFor(docs, section) {
  if (!docs) return 0;
  return { components: asArray(docs.components).length, scenarios: asArray(docs.scenarios).length, decisions: asArray(docs.decisions).length, dependencies: asArray(docs.dependencies).length, runbook: asArray(docs.runbook).length, glossary: asArray(docs.glossary).length }[section] || 0;
}

export function docsSide(state) {
  const docs = docsOf(state);
  return '<div class="side-head"><span class="side-title">Handbook</span></div><div class="side-scroll"><div class="nav-list">' + DOC_SECTIONS.map(section => {
    const count = countFor(docs, section);
    return '<button class="nav-item" type="button" aria-current="' + (state.route.docSection === section) + '" data-action="doc-section" data-section="' + section + '">' + icon(ICONS[section]) + '<span>' + LABELS[section] + '</span>' + (count ? '<span class="count">' + count + '</span>' : '') + '</button>';
  }).join('') + '</div></div><div class="side-foot stack">' + (docs ? '<span class="faint xs">' + esc(docs.status || '') + (docs.updated_at ? ' · updated ' + esc(timeAgo(docs.updated_at)) : '') + '</span>' : '') + button(docs ? 'Regenerate' : 'Generate docs', { size: 'sm', icon: 'sparkle', disabled: Boolean(state.pending), data: { action: 'run-analysis', mutation: '' } }) + '</div>';
}

function pathChip(path) {
  return path ? '<button class="chip" type="button" data-action="open-file" data-path="' + esc(path) + '">' + icon('file') + '<span>' + esc(path) + '</span></button>' : '';
}

export function docsMain(state) {
  const docs = docsOf(state);
  const analysis = (state.overview && state.overview.analysis) || {};
  const section = state.route.docSection || 'overview';
  let html = '<div class="page"><div class="page-inner"><article class="doc stack-lg">';
  if (!docs && section !== 'overview') {
    html += empty('Docs not generated yet', 'A project analysis builds a handbook from the repository: architecture, components, runtime scenarios, decisions and a runbook.', 'docs', button('Run analysis', { variant: 'primary', icon: 'play', disabled: Boolean(state.pending), data: { action: 'run-analysis', mutation: '' } }));
    return html + '</article></div></div>';
  }
  if (docs && docs.status && !['fresh', 'completed'].includes(docs.status)) html += callout('warning', 'These docs are <strong>' + esc(docs.status) + '</strong>: the source changed since they were generated. Regenerate for an accurate picture.');
  const ctx = (docs && docs.context) || {};
  if (section === 'overview') {
    const metrics = (state.overview && state.overview.metrics) || {};
    const languages = Object.entries(metrics.languages || {}).sort((a, b) => b[1] - a[1]);
    html += '<header><p class="eyebrow">Overview</p><h2>' + esc((state.project && state.project.name) || 'Project') + '</h2>' + (ctx.purpose || analysis.purpose ? '<p class="lead">' + esc(ctx.purpose || analysis.purpose) + '</p>' : '<p class="muted">Run a project analysis to describe what this project does and how it pursues its goal.</p>') + '</header>';
    if (asArray(ctx.goal_pursuit).length) html += '<section class="stack"><h3>How it pursues its goal</h3><ol class="steps-list">' + ctx.goal_pursuit.map(step => '<li><span>' + esc(step) + '</span></li>').join('') + '</ol></section>';
    html += '<section class="card card-pad"><dl class="kv">' + (metrics.type ? '<dt>Project type</dt><dd>' + esc(metrics.type) + (metrics.build_file ? ' · <span class="mono">' + esc(metrics.build_file) + '</span>' : '') + '</dd>' : '') +
      (metrics.file_count ? '<dt>Files</dt><dd>' + esc(plural(metrics.source_file_count || metrics.file_count, 'source file')) + ' · ' + esc((metrics.total_lines || 0).toLocaleString()) + ' lines</dd>' : '') +
      (languages.length ? '<dt>Languages</dt><dd>' + languages.slice(0, 6).map(([lang, n]) => esc(lang) + ' <span class="faint">' + n + '</span>').join(' · ') + '</dd>' : '') +
      (asArray(ctx.stack).length ? '<dt>Stack</dt><dd>' + ctx.stack.map(esc).join(' · ') + '</dd>' : '') + '</dl></section>';
    if (!docs) html += '<div class="row">' + button('Generate docs', { variant: 'primary', icon: 'sparkle', disabled: Boolean(state.pending), data: { action: 'run-analysis', mutation: '' } }) + '</div>';
  } else if (section === 'architecture') {
    html += '<header><p class="eyebrow">Architecture</p><h2>System topology</h2>' + (ctx.topology ? '<p class="lead">' + esc(ctx.topology) + '</p>' : '') + '</header>';
    const diagram = ctx.architecture_diagram || analysis.architecture || '';
    html += diagram ? '<section class="stack"><div class="row-between"><h3>Diagram source</h3>' + badge(ctx.architecture_diagram ? 'derived' : 'AI suggestion', ctx.architecture_diagram ? 'info' : 'accent') + '</div><pre class="pre">' + esc(diagram) + '</pre><p class="faint xs">Mermaid source — paste it into any Mermaid renderer to view.</p></section>' : '<p class="muted">No architecture diagram recorded.</p>';
    if (asArray(analysis.entry_points).length) html += '<section class="stack"><h3>Entry points</h3><div class="chips">' + analysis.entry_points.map(pathChip).join('') + '</div></section>';
  } else if (section === 'components') {
    const items = asArray(docs.components);
    html += '<header><p class="eyebrow">Components</p><h2>' + plural(items.length, 'component') + '</h2></header>';
    html += items.length ? '<div class="doc-grid">' + items.map(c => '<section class="card card-pad stack"><div class="row-between"><strong>' + esc(c.name) + '</strong>' + (c.layer ? badge(c.layer, 'accent') : '') + '</div>' + (c.summary ? '<p class="small">' + esc(c.summary) + '</p>' : '') +
      (asArray(c.responsibilities).length ? '<ul class="bullets small">' + c.responsibilities.map(r => '<li>' + esc(r) + '</li>').join('') + '</ul>' : '') +
      (asArray(c.contracts).length ? '<div class="stack"><span class="field-label">Contracts</span>' + c.contracts.map(k => '<div class="small"><span class="mono">' + esc(k.name) + '</span> <span class="faint">' + esc(k.kind || '') + '</span> — ' + esc(k.summary || '') + '</div>').join('') + '</div>' : '') +
      (asArray(c.dependencies).length ? '<div class="faint xs">Depends on: ' + c.dependencies.map(esc).join(', ') + '</div>' : '') + (c.path ? '<div>' + pathChip(c.path) + '</div>' : '') + '</section>').join('') + '</div>' : '<p class="muted">No components recorded.</p>';
  } else if (section === 'scenarios') {
    const items = asArray(docs.scenarios);
    html += '<header><p class="eyebrow">Scenarios</p><h2>Runtime flows</h2></header>';
    html += items.length ? items.map(sc => '<section class="card card-pad stack"><div class="row-between"><h3>' + esc(sc.title) + '</h3>' + (sc.trigger ? badge(sc.trigger) : '') + '</div>' + (sc.summary ? '<p class="small">' + esc(sc.summary) + '</p>' : '') +
      (asArray(sc.steps).length ? '<ol class="steps-list">' + sc.steps.map(st => '<li><div><strong>' + esc(st.title || st.phase) + '</strong>' + (st.layer ? ' <span class="faint xs">' + esc(st.layer) + '</span>' : '') + (st.detail ? '<div class="muted small">' + esc(st.detail) + '</div>' : '') + (st.path ? '<div>' + pathChip(st.path) + '</div>' : '') + '</div></li>').join('') + '</ol>' : '') +
      (sc.invariant ? callout('info', '<strong>Invariant</strong> · ' + esc(sc.invariant)) : '') + '</section>').join('') : '<p class="muted">No scenarios recorded.</p>';
  } else if (section === 'decisions') {
    const items = asArray(docs.decisions);
    html += '<header><p class="eyebrow">Decisions</p><h2>Architecture decision records</h2></header>';
    html += items.length ? items.map((d, i) => '<section class="card card-pad stack"><div class="row-between"><h3>ADR-' + String(i + 1).padStart(3, '0') + ' · ' + esc(d.title) + '</h3>' + (d.category ? badge(d.category) : '') + '</div><dl class="kv"><dt>Decision</dt><dd>' + esc(d.decision || '') + '</dd><dt>Trade-off</dt><dd>' + esc(d.tradeoff || '') + '</dd></dl></section>').join('') : '<p class="muted">No decisions recorded.</p>';
  } else if (section === 'dependencies') {
    const items = asArray(docs.dependencies);
    html += '<header><p class="eyebrow">Dependencies</p><h2>' + plural(items.length, 'dependency', 'dependencies') + '</h2></header>';
    html += items.length ? '<div class="card"><table class="table"><thead><tr><th>Name</th><th>Kind</th><th>Version</th><th>Purpose</th></tr></thead><tbody>' + items.map(d => '<tr><td class="mono">' + esc(d.name) + '</td><td>' + esc(d.kind || '') + '</td><td class="mono faint">' + esc(d.version || '') + '</td><td>' + esc(d.summary || '') + '</td></tr>').join('') + '</tbody></table></div>' : '<p class="muted">No dependencies recorded.</p>';
  } else if (section === 'runbook') {
    const items = asArray(docs.runbook);
    html += '<header><p class="eyebrow">Runbook</p><h2>Operational commands</h2><p class="muted small">Shown for reference. Mini-Orca never runs these for you.</p></header>';
    html += items.length ? items.map(r => '<section class="card card-pad stack"><strong>' + esc(r.label) + '</strong>' + (r.description ? '<p class="muted small">' + esc(r.description) + '</p>' : '') + '<pre class="pre">' + esc(r.command) + '</pre></section>').join('') : '<p class="muted">No runbook entries recorded.</p>';
  } else if (section === 'glossary') {
    const items = asArray(docs.glossary);
    html += '<header><p class="eyebrow">Glossary</p><h2>Domain terms</h2></header>';
    html += items.length ? '<div class="card"><table class="table"><tbody>' + items.map(g => '<tr><td><strong>' + esc(g.term) + '</strong></td><td>' + esc(g.definition) + '</td></tr>').join('') + '</tbody></table></div>' : '<p class="muted">No glossary terms recorded.</p>';
  }
  return html + '</article></div></div>';
}
