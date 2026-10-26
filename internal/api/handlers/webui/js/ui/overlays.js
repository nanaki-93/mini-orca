// Toasts and modal dialogs. Dialogs use the native <dialog> element, which
// traps focus, closes on Escape and restores focus to the opener.

import { esc } from '../core/util.js';
import { icon } from './icons.js';
import { $ } from './dom.js';

const TOAST_ICONS = { success: 'success', warning: 'alert', danger: 'alert', info: 'info' };

export function toast(message, tone = 'info', options = {}) {
  const region = $('toasts');
  if (!region) return;
  const node = document.createElement('div');
  node.className = 'toast toast-' + tone;
  node.setAttribute('role', tone === 'danger' ? 'alert' : 'status');
  node.innerHTML = icon(TOAST_ICONS[tone] || 'info') + '<div class="toast-msg">' + esc(message) + '</div><button class="toast-close" type="button" aria-label="Dismiss">' + icon('x') + '</button>';
  const dismiss = () => {
    if (!node.isConnected) return;
    node.classList.add('leaving');
    setTimeout(() => node.remove(), 180);
  };
  node.querySelector('.toast-close').addEventListener('click', dismiss);
  region.appendChild(node);
  while (region.children.length > 4) region.firstElementChild.remove();
  const sticky = options.sticky ?? tone === 'danger';
  if (!sticky) setTimeout(dismiss, options.duration || (tone === 'warning' ? 7000 : 4200));
}

let dialogQueue = Promise.resolve();

function openDialog({ title, body, confirmLabel = 'Confirm', cancelLabel = 'Cancel', danger = false, field = null }) {
  const dialog = $('dialog');
  $('dialog-title').textContent = title;
  $('dialog-body').textContent = body || '';
  $('dialog-body').hidden = !body;
  const confirm = $('dialog-confirm');
  confirm.textContent = confirmLabel;
  confirm.className = 'btn ' + (danger ? 'btn-danger-solid' : 'btn-primary');
  $('dialog-cancel').textContent = cancelLabel;
  const input = $('dialog-input');
  $('dialog-field').hidden = !field;
  if (field) {
    $('dialog-label').textContent = field.label || '';
    input.value = field.value || '';
    input.placeholder = field.placeholder || '';
  }
  return new Promise(resolve => {
    let settled = false;
    const finish = value => {
      if (settled) return;
      settled = true;
      cleanup();
      if (dialog.open) dialog.close();
      resolve(value);
    };
    const onSubmit = event => { event.preventDefault(); finish(field ? input.value : true); };
    const onCancel = () => finish(field ? null : false);
    const onClose = () => finish(field ? null : false);
    const form = dialog.querySelector('form');
    const cancelBtn = $('dialog-cancel');
    form.addEventListener('submit', onSubmit);
    cancelBtn.addEventListener('click', onCancel);
    dialog.addEventListener('close', onClose);
    function cleanup() {
      form.removeEventListener('submit', onSubmit);
      cancelBtn.removeEventListener('click', onCancel);
      dialog.removeEventListener('close', onClose);
    }
    dialog.showModal();
    (field ? input : danger ? cancelBtn : confirm).focus();
    if (field) input.select();
  });
}

/** Serialized so two confirmations never race for the same dialog element. */
export function confirmDialog(options) {
  const next = dialogQueue.then(() => openDialog(options));
  dialogQueue = next.catch(() => {});
  return next;
}

export function promptDialog(options) {
  const next = dialogQueue.then(() => openDialog(Object.assign({}, options, { field: { label: options.label, value: options.value, placeholder: options.placeholder } })));
  dialogQueue = next.catch(() => {});
  return next;
}

export function showShortcuts(groups) {
  const dialog = $('shortcuts');
  $('shortcuts-body').innerHTML = groups.map(group => '<section><h3>' + esc(group.title) + '</h3>' + group.items.map(([keys, label]) =>
    '<div class="sc-row"><span>' + esc(label) + '</span><span class="sc-keys">' + keys.split(' ').map(k => '<kbd class="kbd">' + esc(k) + '</kbd>').join('') + '</span></div>').join('') + '</section>').join('');
  dialog.querySelector('[data-close]').onclick = () => dialog.close();
  dialog.showModal();
}
