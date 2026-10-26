// Small DOM helpers. Views render escaped HTML strings; dynamic geometry is
// applied through the CSSOM afterwards because the CSP forbids inline styles.

import { esc } from '../core/util.js';
import { icon } from './icons.js';

export const $ = id => document.getElementById(id);

/** Sets innerHTML and then applies data-driven sizes (data-w → width %, data-p → --p). */
export function setHTML(element, html) {
  if (!element) return;
  element.innerHTML = html;
  applyGeometry(element);
}

export function applyGeometry(root) {
  root.querySelectorAll('[data-w]').forEach(el => { el.style.width = Math.max(0, Math.min(100, Number(el.dataset.w) || 0)) + '%'; });
  root.querySelectorAll('[data-p]').forEach(el => { el.style.setProperty('--p', String(Math.max(0, Math.min(100, Number(el.dataset.p) || 0)))); });
  root.querySelectorAll('[data-pipe]').forEach(el => { el.style.setProperty('--n', String(Number(el.dataset.pipe) || 4)); });
}

/** Returns attribute markup from an object, escaping values and skipping false/null. */
export function attrs(map) {
  return Object.entries(map).filter(([, v]) => v !== false && v !== null && v !== undefined).map(([k, v]) => (v === true ? ' ' + k : ' ' + k + '="' + esc(v) + '"')).join('');
}

export function button(label, options = {}) {
  const cls = ['btn'].concat(options.variant ? ['btn-' + options.variant] : [], options.size ? ['btn-' + options.size] : [], options.class ? [options.class] : []).join(' ');
  const data = Object.fromEntries(Object.entries(options.data || {}).map(([k, v]) => ['data-' + k, v]));
  return '<button' + attrs(Object.assign({ type: 'button', class: cls, title: options.title, 'aria-label': !label && options.title ? options.title : null, disabled: options.disabled, 'aria-pressed': options.pressed, 'aria-busy': options.busy ? 'true' : null, 'aria-keyshortcuts': options.shortcut }, data)) + '>' +
    (options.icon ? icon(options.icon) : '') + (label ? '<span>' + esc(label) + '</span>' : '') + (options.kbd ? '<kbd class="kbd">' + esc(options.kbd) + '</kbd>' : '') + '</button>';
}

export function badge(text, tone = '', dot = false) {
  return '<span class="badge' + (tone ? ' badge-' + tone : '') + '">' + (dot ? '<span class="dot"></span>' : '') + esc(text) + '</span>';
}

export function empty(title, body, iconName = 'info', actions = '') {
  return '<div class="empty"><div class="empty-icon">' + icon(iconName) + '</div><h3>' + esc(title) + '</h3>' + (body ? '<p>' + esc(body) + '</p>' : '') + (actions ? '<div class="row">' + actions + '</div>' : '') + '</div>';
}

export function callout(tone, text, actions = '') {
  const name = tone === 'danger' ? 'alert' : tone === 'warning' ? 'alert' : tone === 'success' ? 'success' : 'info';
  return '<div class="callout callout-' + tone + '">' + icon(name) + '<div class="grow">' + text + '</div>' + (actions ? '<div class="row">' + actions + '</div>' : '') + '</div>';
}

export function stateIcon(tone) {
  if (tone === 'running') return '<span class="state-ic running"><span class="spinner"></span></span>';
  const name = tone === 'passed' ? 'check' : tone === 'failed' ? 'x' : 'dot';
  return '<span class="state-ic ' + esc(tone) + '">' + icon(name) + '</span>';
}

export function toneForState(tone) {
  return { passed: 'success', failed: 'danger', running: 'info' }[tone] || '';
}

export function isTyping(event) {
  const target = event.target;
  if (!target || !target.tagName) return false;
  const tag = target.tagName.toLowerCase();
  return tag === 'input' || tag === 'textarea' || tag === 'select' || target.isContentEditable;
}

/** Preserves focus and caret of an input across a re-render of its container. */
export function preserveFocus(container, render) {
  const active = document.activeElement;
  const key = active && container.contains(active) ? active.dataset.focusKey || active.id : '';
  const start = active && 'selectionStart' in active ? active.selectionStart : null;
  const end = active && 'selectionEnd' in active ? active.selectionEnd : null;
  render();
  if (!key) return;
  const next = container.querySelector('[data-focus-key="' + CSS.escape(key) + '"]') || document.getElementById(key);
  if (next && next !== document.activeElement) {
    next.focus({ preventScroll: true });
    if (start !== null && 'setSelectionRange' in next) {
      try { next.setSelectionRange(start, end); } catch (_) { /* not a text input */ }
    }
  }
}
