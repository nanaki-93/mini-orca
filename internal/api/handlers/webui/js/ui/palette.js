// Command palette: one fuzzy search across commands, files and tasks.
// `>` limits results to commands and `#` to tasks. Recently used commands
// rank first when the query is empty.

import { esc } from '../core/util.js';
import { rankItems, highlight } from '../core/fuzzy.js';
import { icon } from './icons.js';
import { $ } from './dom.js';

const RECENT_KEY = 'mini_orca_recent_commands';

function readRecent() {
  try { return JSON.parse(localStorage.getItem(RECENT_KEY) || '[]').slice(0, 8); } catch (_) { return []; }
}

function writeRecent(id) {
  try { localStorage.setItem(RECENT_KEY, JSON.stringify([id].concat(readRecent().filter(x => x !== id)).slice(0, 8))); } catch (_) { /* storage unavailable */ }
}

export function createPalette(provider) {
  const dialog = $('palette');
  const input = $('palette-input');
  const list = $('palette-list');
  let results = [];
  let index = 0;

  function collect(raw) {
    const text = raw.trim();
    let groups = ['Commands', 'Tasks', 'Files'];
    let q = text;
    if (text.startsWith('>')) { groups = ['Commands']; q = text.slice(1); }
    else if (text.startsWith('#')) { groups = ['Tasks']; q = text.slice(1); }
    const all = provider().filter(item => groups.includes(item.group) && (item.enabled === undefined || item.enabled));
    if (!q.trim()) {
      const recent = readRecent();
      const commands = all.filter(item => item.group === 'Commands');
      commands.forEach(item => { item.boost = recent.includes(item.id) ? 100 - recent.indexOf(item.id) : 0; });
      const ordered = commands.slice().sort((a, b) => (b.boost || 0) - (a.boost || 0));
      const tasks = all.filter(item => item.group === 'Tasks').slice(0, 5);
      return ordered.slice(0, 14).concat(tasks).map(item => ({ item, positions: [] }));
    }
    const limits = { Commands: 12, Tasks: 8, Files: 30 };
    return groups.flatMap(group => rankItems(all.filter(item => item.group === group), q, item => item.label, limits[group]));
  }

  function render() {
    results = collect(input.value);
    index = Math.min(index, Math.max(0, results.length - 1));
    let html = '';
    let lastGroup = '';
    results.forEach((result, i) => {
      const item = result.item;
      if (item.group !== lastGroup) {
        html += '<div class="palette-group" role="presentation">' + esc(item.group) + '</div>';
        lastGroup = item.group;
      }
      html += '<button class="palette-item" type="button" role="option" id="pal-' + i + '" data-index="' + i + '" aria-selected="' + (i === index) + '" tabindex="-1">' +
        icon(item.icon || 'dot') + '<span class="p-label">' + highlight(item.label, result.positions, esc) + '</span>' +
        (item.hint ? '<span class="p-hint">' + esc(item.hint) + '</span>' : '') +
        (item.kbd ? '<kbd class="kbd kbd-sm">' + esc(item.kbd) + '</kbd>' : '') + '</button>';
    });
    list.innerHTML = html || '<div class="empty" role="status"><p>No matches. Try a file name, a task title or “>” for commands.</p></div>';
    sync();
  }

  function sync() {
    list.querySelectorAll('.palette-item').forEach(el => el.setAttribute('aria-selected', String(Number(el.dataset.index) === index)));
    const active = list.querySelector('#pal-' + index);
    if (active) {
      input.setAttribute('aria-activedescendant', active.id);
      active.scrollIntoView({ block: 'nearest' });
    } else {
      input.removeAttribute('aria-activedescendant');
    }
  }

  function run(i) {
    const result = results[i];
    if (!result) return;
    close();
    if (result.item.group === 'Commands') writeRecent(result.item.id);
    result.item.run();
  }

  function open(prefill = '') {
    if (dialog.open) return;
    input.value = prefill;
    index = 0;
    render();
    dialog.showModal();
    input.focus();
  }

  function close() {
    if (dialog.open) dialog.close();
  }

  input.addEventListener('input', () => { index = 0; render(); });
  input.addEventListener('keydown', event => {
    if (event.key === 'ArrowDown' || (event.key === 'n' && event.ctrlKey)) { event.preventDefault(); index = results.length ? (index + 1) % results.length : 0; sync(); }
    else if (event.key === 'ArrowUp' || (event.key === 'p' && event.ctrlKey)) { event.preventDefault(); index = results.length ? (index - 1 + results.length) % results.length : 0; sync(); }
    else if (event.key === 'Enter') { event.preventDefault(); run(index); }
    else if (event.key === 'Tab') { event.preventDefault(); }
  });
  list.addEventListener('click', event => {
    const item = event.target.closest('.palette-item');
    if (item) run(Number(item.dataset.index));
  });
  list.addEventListener('mousemove', event => {
    const item = event.target.closest('.palette-item');
    if (item && Number(item.dataset.index) !== index) { index = Number(item.dataset.index); sync(); }
  });
  dialog.addEventListener('click', event => { if (event.target === dialog) close(); });

  return { open, close, isOpen: () => dialog.open, toggle: () => (dialog.open ? close() : open()) };
}
