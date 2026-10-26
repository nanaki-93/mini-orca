// Pure helpers shared by every view. Nothing here touches the DOM or network.

const ESCAPES = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };

/** Escapes text for safe interpolation into HTML text and attribute values. */
export function esc(value) {
  return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ESCAPES[ch]);
}

export function plural(count, singular, pluralForm) {
  return count + ' ' + (count === 1 ? singular : (pluralForm || singular + 's'));
}

export function basename(path) {
  const value = String(path || '');
  const index = value.lastIndexOf('/');
  return index >= 0 ? value.slice(index + 1) : value;
}

export function dirname(path) {
  const value = String(path || '');
  const index = value.lastIndexOf('/');
  return index > 0 ? value.slice(0, index) : '';
}

export function shortHash(value, length = 7) {
  return value ? String(value).slice(0, length) : '';
}

/** Shortens a project revision for display, dropping the digest algorithm prefix. */
export function revisionLabel(value, length = 10) {
  return shortHash(String(value || '').replace(/^[a-z0-9]+:/i, ''), length);
}

export function asArray(value) {
  return Array.isArray(value) ? value : [];
}

export function unique(values) {
  return [...new Set(values.filter(Boolean))];
}

/** Formats a past timestamp as a compact relative label ("3m ago"). */
export function timeAgo(value, now = Date.now()) {
  const time = typeof value === 'number' ? value : Date.parse(value || '');
  if (!Number.isFinite(time) || time <= 0) return '';
  const seconds = Math.max(0, Math.round((now - time) / 1000));
  if (seconds < 45) return 'just now';
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return minutes + 'm ago';
  const hours = Math.round(minutes / 60);
  if (hours < 24) return hours + 'h ago';
  const days = Math.round(hours / 24);
  return days < 30 ? days + 'd ago' : new Date(time).toISOString().slice(0, 10);
}

export function formatDuration(ms) {
  if (!Number.isFinite(ms) || ms <= 0) return '';
  if (ms < 1000) return ms + ' ms';
  const seconds = ms / 1000;
  if (seconds < 60) return seconds.toFixed(seconds < 10 ? 1 : 0) + ' s';
  const minutes = Math.floor(seconds / 60);
  return minutes + 'm ' + Math.round(seconds % 60) + 's';
}

export function durationBetween(start, end) {
  const a = Date.parse(start || '');
  const b = Date.parse(end || '');
  return Number.isFinite(a) && Number.isFinite(b) && b >= a ? b - a : 0;
}

export function debounce(fn, wait) {
  let timer = null;
  const wrapped = (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => { timer = null; fn(...args); }, wait);
  };
  wrapped.cancel = () => { clearTimeout(timer); timer = null; };
  return wrapped;
}

export function isTestPath(path) {
  const value = String(path || '');
  return /_test\.go$/.test(value) || /\.(test|spec)\.[cm]?[jt]sx?$/.test(value) || /(^|\/)test_[^/]+\.py$/.test(value);
}

/** Returns the companion Go test path for a source file, or '' when none applies. */
export function companionTestPath(path) {
  const value = String(path || '');
  return value.endsWith('.go') && !value.endsWith('_test.go') ? value.slice(0, -3) + '_test.go' : '';
}

export function severityRank(severity) {
  return { critical: 0, high: 1, medium: 2, low: 3, info: 4 }[String(severity || '').toLowerCase()] ?? 5;
}

export function severityTone(severity) {
  const rank = severityRank(severity);
  return rank <= 1 ? 'danger' : rank === 2 ? 'warning' : 'neutral';
}
