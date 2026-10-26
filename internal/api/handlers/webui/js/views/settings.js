// Settings: appearance, checkout scope, execution trust, models, maintenance.

import { asArray, basename, esc, revisionLabel } from '../core/util.js';
import { badge, button } from '../ui/dom.js';

export const THEMES = [['system', 'System'], ['midnight', 'Midnight'], ['light', 'Light'], ['tokyo', 'Tokyo Night'], ['contrast', 'High contrast']];

export function settingsMain(state, prefs) {
  const project = state.project || {};
  const trusted = Boolean(state.trust && state.trust.trusted);
  const trees = asArray(state.gitWorkspace && state.gitWorkspace.worktrees).filter(t => !t.current && t.path);
  const scopes = Object.entries((state.models && state.models.scopes) || {});
  const commands = asArray(state.trust && state.trust.commands).map(argv => asArray(argv).join(' '));
  const select = (input, options, value) => '<select class="select" data-input="' + input + '" aria-label="' + input + '">' + options.map(([v, l]) => '<option value="' + esc(v) + '"' + (v === value ? ' selected' : '') + '>' + esc(l) + '</option>').join('') + '</select>';
  let html = '<div class="page"><div class="page-inner page-narrow"><header class="page-head"><div><h1 class="page-title">Settings</h1><p class="page-sub">Preferences are stored in this browser. Project trust is stored by the daemon.</p></div></header>';
  html += '<section class="settings-section card card-pad"><h2>Appearance</h2>' +
    row('Theme', 'System follows your OS light or dark mode.', select('pref-theme', THEMES, prefs.theme)) +
    row('Density', 'Compact fits more rows on small screens.', select('pref-density', [['comfortable', 'Comfortable'], ['compact', 'Compact']], prefs.density)) +
    row('Default diff layout', 'Press V in Review to switch at any time.', select('pref-diff', [['split', 'Split'], ['unified', 'Unified']], prefs.diffMode)) + '</section>';
  html += '<section class="settings-section card card-pad"><h2>Project</h2>' +
    row('Checkout', 'Explore and review inside the main checkout or an agent worktree.', select('checkout', [['', 'Main checkout']].concat(trees.map(t => [t.path, (t.branch || 'worktree') + ' · ' + basename(t.path)])), state.route.worktreePath)) +
    row('Location', '<span class="path">' + esc(project.path || '—') + '</span>', button('Open another project…', { size: 'sm', data: { action: 'switch-project' } })) +
    row('Revision', 'Every request is bound to this revision; stale results are refused.', '<span class="mono faint">' + esc(revisionLabel(project.project_revision, 12)) + '</span>') + '</section>';
  html += '<section class="settings-section card card-pad"><h2>Execution</h2>' +
    row('Local code checks ' + (trusted ? badge('trusted', 'success') : badge('not trusted', 'warning')), 'Allows checks, scans and the agent workflow to run these project commands in an isolated copy:' + (commands.length ? '<ul class="bullets small mono">' + commands.map(c => '<li>' + esc(c) + '</li>').join('') + '</ul>' : ''), button(trusted ? 'Revoke trust' : 'Grant trust…', { variant: trusted ? 'danger' : 'primary', size: 'sm', disabled: Boolean(state.pending), data: { action: 'toggle-trust', mutation: '' } })) + '</section>';
  html += '<section class="settings-section card card-pad"><h2>Models</h2>' + (scopes.length ? scopes.map(([scope, m]) => row('<span class="mono">' + esc(scope) + '</span>', esc(m.model || 'unavailable') + (m.provider_origin ? ' · ' + esc(m.provider_origin) : ''), m.remote_provider ? badge('remote · asks consent', 'warning') : badge('local', 'success'))).join('') : '<p class="muted small">Model catalog unavailable.</p>') + '<p class="faint xs">Change models and agent roles from the TUI (<span class="mono">:roles</span>, <span class="mono">:models</span>).</p></section>';
  html += '<section class="settings-section card card-pad"><h2>Maintenance</h2>' +
    row('Reindex files', 'Refresh deterministic file facts. No model request.', button('Reindex', { size: 'sm', disabled: Boolean(state.pending), data: { action: 'reindex', mutation: '' } })) +
    row('Refresh workspace', 'Reload everything from the daemon.', button('Refresh', { size: 'sm', data: { action: 'refresh' } })) +
    row('Daemon', state.version ? 'Version ' + esc(state.version) : 'Version unavailable', '') + '</section>';
  return html + '</div></div>';
}

function row(label, desc, control) {
  return '<div class="settings-row"><div class="grow"><div class="field-label">' + label + '</div><div class="desc">' + desc + '</div></div><div>' + control + '</div></div>';
}
