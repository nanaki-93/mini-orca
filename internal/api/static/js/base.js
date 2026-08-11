// ─── HTMX Batch Requests ─────────────────────────────────────────────
const HTMXBatcher = {
    pending: new Map(),
    batchWindow: 50,

    addToBatch(key, config) {
        if (!this.pending.has(key)) {
            this.pending.set(key, []);
            setTimeout(() => this.processBatch(key), this.batchWindow);
        }
        this.pending.get(key).push(config);
    },

    processBatch(key) {
        const configs = this.pending.get(key);
        if (configs && configs.length > 0) {
            const latest = configs[configs.length - 1];
            if (latest.elt && latest.elt.requestConfig) {
                htmx.invokeCallAction(latest.elt, 'htmx:configRequest', latest);
            }
        }
        this.pending.delete(key);
    }
};

function debounceHTMX(element, delay = 300) {
    let timeout;
    element.addEventListener('input', function() {
        clearTimeout(timeout);
        timeout = setTimeout(() => {
            htmx.trigger(element, 'do-search');
        }, delay);
    });
    element.addEventListener('do-search', function() {
        htmx.trigger(element.parentElement, 'htmx:configRequest', {
            elt: element,
            requestConfig: { verb: 'GET', target: element }
        });
        htmx.ajax('GET', element.closest('[hx-get]').getAttribute('hx-get'), {
            target: element.closest('[hx-target]').getAttribute('hx-target'),
            swap: 'innerHTML',
            values: new FormData(element.closest('form'))
        });
    });
}
    const ScreenReader = {
    announce(message, priority = 'polite') {
    // Console fallback for development
    if (priority === 'assertive') {
    console.log('[ALERT]', message);
} else {
    console.log('[INFO]', message);
}
},
    announceStatus(message) { this.announce(message, 'polite'); },
    announceAlert(message) { this.announce(message, 'assertive'); }
};

    const FocusManager = {
    lastFocusedElement: null,
    saveFocus() { this.lastFocusedElement = document.activeElement; },
    restoreFocus() {
    if (this.lastFocusedElement && document.body.contains(this.lastFocusedElement)) {
    this.lastFocusedElement.focus();
    this.lastFocusedElement = null;
}
},
    trapFocus(container) {
    const focusableElements = container.querySelectorAll(
    'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
    );
    const firstFocusable = focusableElements[0];
    const lastFocusable = focusableElements[focusableElements.length - 1];

    function handleKeyDown(e) {
    if (e.key === 'Tab') {
    if (e.shiftKey) {
    if (document.activeElement === firstFocusable) { e.preventDefault(); lastFocusable.focus(); }
} else {
    if (document.activeElement === lastFocusable) { e.preventDefault(); firstFocusable.focus(); }
}
}
    if (e.key === 'Escape') { container.dispatchEvent(new CustomEvent('close-focus-trap')); }
}
    container.addEventListener('keydown', handleKeyDown);
    container.addEventListener('close-focus-trap', () => { container.removeEventListener('keydown', handleKeyDown); });
    if (firstFocusable) { firstFocusable.focus(); }
    return () => container.removeEventListener('keydown', handleKeyDown);
},
    focusElement(element) {
    if (element) { element.focus(); element.scrollIntoView({ behavior: 'smooth', block: 'nearest' }); }
}
};