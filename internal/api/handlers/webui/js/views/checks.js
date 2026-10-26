// Checks: the workspace Go scan and project analysis coverage.

import { asArray, esc, plural, timeAgo } from '../core/util.js';
import { normalizePhaseState } from '../core/readiness.js';
import { badge, button, callout } from '../ui/dom.js';
import { phaseFilter, phaseList } from './review.js';

const CATEGORY_LABELS = { bugs: 'Bugs', performance: 'Performance', security: 'Security' };

export function checksMain(state) {
  const scan = state.scan;
  const trusted = Boolean(state.trust && state.trust.trusted);
  const scanRunning = scan && normalizePhaseState(scan.status) === 'running';
  const run = state.overview && state.overview.analysis_run;
  const runActive = run && ['running', 'queued', 'pausing'].includes(run.status);
  let html = '<div class="page"><div class="page-inner page-narrow stack-lg"><header class="page-head"><div><h1 class="page-title">Checks</h1><p class="page-sub">Verified results from tools stay separate from model analysis. Nothing here runs until you start it.</p></div>' + phaseFilter(state.checks.filter) + '</header>';

  html += '<section class="stack"><div class="row-between"><div><h2>Go scan</h2><p class="muted small">Parser, vet and tests in a temporary copy of the workspace' + (scan && scan.updated_at ? ' · ' + esc(timeAgo(scan.updated_at)) : '') + '.</p></div><div class="row">' + (scan ? badge(scan.status || 'recorded', { passed: 'success', failed: 'danger', running: 'info' }[normalizePhaseState(scan.status)] || '', scanRunning) : '') +
    (scanRunning ? button('Cancel', { variant: 'danger', size: 'sm', disabled: Boolean(state.pending), data: { action: 'cancel-scan', mutation: '' } }) : button(scan ? 'Run again' : 'Run Go scan', { variant: 'primary', icon: 'play', disabled: !trusted || Boolean(state.pending), busy: state.pending === 'Starting scan…', title: trusted ? 'Run parser, vet and tests' : 'Grant execution trust first', data: { action: 'run-scan', mutation: '' } })) + '</div></div>';
  if (!trusted) html += callout('warning', 'The scan runs project code. Grant execution trust in Settings to enable it.', button('Open Settings', { size: 'sm', data: { action: 'open-page', page: 'settings' } }));
  const phases = asArray(scan && scan.phases).map(p => ({ name: p.name, state: p.state, command: asArray(p.command).join(' '), meta: typeof p.exit_code === 'number' && p.exit_code ? 'exit ' + p.exit_code : '', output: p.output }));
  html += '<div>' + phaseList(phases, state.checks.filter) + '</div></section>';

  html += '<section class="stack"><div class="row-between"><div><h2>Project analysis</h2><p class="muted small">Model analysis of bugs, performance and security across indexed files. Results are AI suggestions until verified.</p></div><div class="row">' + (run ? badge(run.status, { passed: 'success', failed: 'danger', running: 'info' }[normalizePhaseState(run.status)] || '', runActive) : '') +
    (runActive ? button('Pause', { size: 'sm', icon: 'pause', disabled: Boolean(state.pending), data: { action: 'analysis-control', op: 'pause', mutation: '' } }) + button('Cancel', { size: 'sm', variant: 'danger', disabled: Boolean(state.pending), data: { action: 'analysis-control', op: 'cancel', mutation: '' } })
      : button(run ? 'Run again' : 'Run analysis', { icon: 'play', disabled: Boolean(state.pending), busy: state.pending === 'Preparing analysis…', data: { action: 'run-analysis', mutation: '' } })) + '</div></div>';
  if (run && run.reason) html += callout(normalizePhaseState(run.status) === 'failed' ? 'danger' : 'info', esc(run.reason));
  const sections = asArray(run && run.sections);
  if (sections.length) {
    html += '<div class="card"><div class="list">' + sections.map(sec => {
      const cov = sec.coverage || {};
      const done = (cov.succeeded || 0) + (cov.partial || 0);
      const pct = cov.total ? Math.round((done / cov.total) * 100) : 0;
      const tone = (cov.failed || 0) > 0 ? 'danger' : pct === 100 ? 'success' : '';
      return '<div class="list-row"><div class="grow stack"><div class="row-between"><strong>' + esc(CATEGORY_LABELS[sec.category] || sec.category) + '</strong><span class="faint xs mono">' + done + '/' + (cov.total || 0) + (cov.failed ? ' · ' + cov.failed + ' failed' : '') + (typeof sec.finding_count === 'number' ? ' · ' + plural(sec.finding_count, 'finding') : '') + '</span></div><div class="progress ' + tone + '"><span data-w="' + pct + '"></span></div>' + (sec.reason ? '<span class="faint xs">' + esc(sec.reason) + '</span>' : '') + '</div>' + badge(sec.status, { passed: 'success', failed: 'danger', running: 'info' }[normalizePhaseState(sec.status)] || '') + '</div>';
    }).join('') + '</div></div>';
  } else {
    html += '<p class="faint small">No analysis run recorded for this revision.</p>';
  }
  const coverage = state.overview && state.overview.analysis_coverage;
  if (coverage && coverage.total) html += '<p class="faint xs">File coverage: ' + coverage.fresh + ' fresh · ' + coverage.stale + ' stale · ' + coverage.missing + ' missing' + (coverage.failed ? ' · ' + coverage.failed + ' failed' : '') + ' of ' + coverage.total + '.</p>';
  html += '</section>';
  return html + '</div></div>';
}
