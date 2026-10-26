// Same-origin daemon client. Every request carries the Web UI marker header the
// daemon's local request policy requires for browser-originated calls.

export class ApiError extends Error {
  constructor(message, status, data) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.data = data;
  }
}

export function withWorktree(path, worktreePath) {
  if (!worktreePath) return path;
  return path + (path.includes('?') ? '&' : '?') + 'worktree_path=' + encodeURIComponent(worktreePath);
}

export function query(params) {
  const q = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) if (value !== undefined && value !== null && value !== '') q.set(key, value);
  const text = q.toString();
  return text ? '?' + text : '';
}

export function createApi(fetchImpl) {
  async function request(path, options = {}) {
    const opts = { method: options.method || 'GET', headers: Object.assign({ 'X-Mini-Orca-Web-UI': '1' }, options.headers) };
    if (options.body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = typeof options.body === 'string' ? options.body : JSON.stringify(options.body);
    }
    if (options.signal) opts.signal = options.signal;
    const response = await fetchImpl(path, opts);
    if (response.status === 204) return null;
    const text = await response.text();
    let data = null;
    if (text) {
      try { data = JSON.parse(text); } catch (_) { data = { error: text }; }
    }
    if (!response.ok) {
      const message = (data && (data.user_message || data.message || data.error)) || ('HTTP ' + response.status);
      throw new ApiError(message, response.status, data);
    }
    return data;
  }
  return {
    get: (path, options) => request(path, options),
    post: (path, body, options) => request(path, Object.assign({}, options, { method: 'POST', body: body === undefined ? {} : body })),
    patch: (path, body) => request(path, { method: 'PATCH', body }),
    del: path => request(path, { method: 'DELETE' })
  };
}
