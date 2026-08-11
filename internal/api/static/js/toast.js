const Toast = {
    container: document.getElementById('toast-container'),
    toasts: [],
    maxToasts: 5,
    defaultDuration: 5000,

    show(message, type = 'info', title = null, duration = null) {
        if (this.toasts.length >= this.maxToasts) { this.remove(this.toasts[0].id); }
        const id = 'toast-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
        const toastDuration = duration || this.defaultDuration;
        const icons = {
            success: `<svg class="w-5 h-5 toast-icon success" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>`,
            error: `<svg class="w-5 h-5 toast-icon error" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"/></svg>`,
            warning: `<svg class="w-5 h-5 toast-icon warning" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/></svg>`,
            info: `<svg class="w-5 h-5 toast-icon info" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/></svg>`
        };
        const toastHTML = `<div id="${id}" class="toast" role="alert" aria-live="assertive"><div class="flex items-start gap-3 flex-1">${icons[type] || icons.info}<div class="toast-content min-w-0">${title ? `<div class="toast-title">${title}</div>` : ''}<div class="toast-message">${message}</div></div></div><button class="toast-close" onclick="Toast.remove('${id}')" aria-label="Close"><svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"/></svg></button><div class="toast-progress" style="animation-duration: ${toastDuration}ms;"></div></div>`;
        this.container.insertAdjacentHTML('beforeend', toastHTML);
        const toastEl = document.getElementById(id);
        const toastData = { id, element: toastEl, timer: null };
        this.toasts.push(toastData);
        toastData.timer = setTimeout(() => { this.remove(id); }, toastDuration);
        return id;
    },

    remove(id) {
        const toastIndex = this.toasts.findIndex(t => t.id === id);
        if (toastIndex === -1) return;
        const toastData = this.toasts[toastIndex];
        const toastEl = toastData.element;
        if (toastData.timer) { clearTimeout(toastData.timer); }
        toastEl.classList.add('toast-out');
        setTimeout(() => { if (toastEl.parentNode) { toastEl.parentNode.removeChild(toastEl); } }, 150);
        this.toasts.splice(toastIndex, 1);
    },

    success(message, title = 'Success', duration = null) { return this.show(message, 'success', title, duration); },
    error(message, title = 'Error', duration = null) { return this.show(message, 'error', title, duration); },
    warning(message, title = 'Warning', duration = null) { return this.show(message, 'warning', title, duration); },
    info(message, title = 'Info', duration = null) { return this.show(message, 'info', title, duration); }
};