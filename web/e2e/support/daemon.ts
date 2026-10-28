// A stateful fake of the daemon's loopback API, installed with Playwright
// request interception. It implements the review workflow the UI drives
// (view files, approve, apply, create tasks) with the daemon's guards: every
// mutation must carry the Web UI header and the current proposal identity.

import { test as base, expect, type Page, type Route } from '@playwright/test';
import type { ChangeHistoryEntry, ChangeSession, DaemonEvent, ProjectSetup } from '../../src/api/types';
import * as data from './fixtures';

export interface RecordedRequest {
  method: string;
  path: string;
  body: Record<string, unknown> | null;
}

/** The fake daemon's launch token, written into the document like the daemon's. */
export const WRITE_TOKEN = 'e2e-write-token';

type Handler = (body: Record<string, unknown>, match: RegExpMatchArray) => unknown;

export class FakeDaemon {
  readonly requests: RecordedRequest[] = [];
  readonly sessions = new Map<string, ChangeSession>([['chg-1', data.reviewTask()]]);
  private nextId = 100;
  private readonly overrides = new Map<string, () => unknown>();
  private readonly pending: DaemonEvent[] = [];
  private settings = data.agentSettings(0, null, null);
  private setup = data.projectSetup('configured');
  private selection = data.analysisSelection([]);

  /** Sets the project's setup state for this test; "needed" opens the wizard. */
  serveSetup(state: string): void {
    this.setup = data.projectSetup(state);
  }
  /** The analysis run the overview reports; tests set it with serveRun. */
  private run: ReturnType<typeof data.analysisRun> | undefined;

  serveRun(run: ReturnType<typeof data.analysisRun> | undefined): void {
    this.run = run;
  }

  private setRun(status: string) {
    this.run = { ...(this.run ?? data.analysisRun()), status, reason: undefined };
    return this.run;
  }
  /** Console messages a test expects, such as the browser's note on a refused request. */
  readonly expectedConsole: RegExp[] = [];

  /**
   * Adds a live update to every event-stream connection: the fake cannot keep
   * a stream open, and development mode opens the first connection twice.
   */
  push(event: DaemonEvent): void {
    this.pending.push(event);
  }

  /** Replaces a GET route's response for this test. */
  serve(path: string, respond: () => unknown): void {
    this.overrides.set(path, respond);
  }

  mutations(suffix: string): RecordedRequest[] {
    return this.requests.filter(request => request.method !== 'GET' && request.path.endsWith(suffix));
  }

  async install(page: Page): Promise<void> {
    await page.route(url => url.pathname === '/status' || url.pathname.startsWith('/api/'), route => this.handle(route));
    // The daemon writes its launch token into the document it serves; the
    // dev server cannot, so the test writes it once the document is parsed.
    await page.addInitScript(token => {
      document.addEventListener('DOMContentLoaded', () => {
        document.querySelector<HTMLMetaElement>('meta[name="mini-orca-write-token"]')?.setAttribute('content', token);
      });
    }, WRITE_TOKEN);
  }

  private readonly deleted = new Set<string>();
  private readonly extraTasks: ChangeHistoryEntry[] = [];

  /** Adds a listed task, such as a stale one, for this test. */
  addTask(entry: ChangeHistoryEntry): void {
    this.extraTasks.push(entry);
  }

  private tasks() {
    const own = [...this.sessions.values()].map(session => data.historyEntry(session));
    return [...own, ...data.otherTasks, ...this.extraTasks].filter(task => !this.deleted.has(task.id));
  }

  /** Deletes a task like the daemon: the body names the version shown; running tasks are kept. */
  private deleteTask(id: string, body: Record<string, unknown>) {
    const task = this.tasks().find(entry => entry.id === decodeURIComponent(id));
    if (!task) throw new Error('This task no longer exists; refresh the task list.');
    if (body.revision !== task.revision || body.hash !== task.hash) throw new Error('The task changed since it was loaded.');
    if (task.workflow_status === 'running') throw new Error('This task cannot be deleted yet: stop its agents before deleting it.');
    this.deleted.add(task.id);
    this.sessions.delete(task.id);
    return { deleted: task.id };
  }

  private readonly gets: [RegExp, () => unknown][] = [
    [/^\/status$/, () => ({ status: 'ok', version: '9.9.9' })],
    [/^\/api\/models\/current$/, () => data.models],
    [/^\/api\/models\/available$/, () => data.availableModels],
    [/^\/api\/projects\/current$/, () => data.project],
    [/^\/api\/projects\/current\/index$/, () => ({ ...data.project, files: data.indexedPaths.map(path => ({ path, content_hash: 'h', language: 'go', line_count: 3 })) })],
    [/^\/api\/projects\/current\/execution-trust$/, () => ({ ...data.project, trusted: true, commands: [['go', 'test', './...']] })],
    [/^\/api\/projects\/current\/git\/workspace$/, () => data.gitWorkspace()],
    [/^\/api\/projects\/current\/changes$/, () => this.tasks()],
    [/^\/api\/projects\/current\/changes\/recovery$/, () => ({ ...data.project, session_id: '', state: '', hash: '', undo_available: false, warnings: [] })],
    [/^\/api\/projects\/current\/findings$/, () => ({ ...data.project, findings: data.findings })],
    [/^\/api\/projects\/current\/overview$/, () => ({ ...data.overview, analysis_run: this.run ?? data.overview.analysis_run })],
    [/^\/api\/projects\/current\/agent-settings$/, () => this.settings],
    [/^\/api\/projects\/current\/setup$/, () => this.setup],
    [/^\/api\/projects\/current\/analysis\/selection$/, () => this.selection],
    [/^\/api\/projects\/current\/usage$/, () => data.usage(this.sessions.get('chg-1')!)],
    [/^\/api\/projects\/current\/quarantine$/, () => ({ files: [], skipped: [] })],
    [/^\/api\/projects\/current\/files\/info$/, () => ({ path: 'internal/api/handlers/webui.go', content_hash: 'h', language: 'go', line_count: 3, content: data.fileSource })],
  ];

  private readonly posts: [RegExp, Handler][] = [
    [/^\/api\/projects\/current\/changes$/, body => this.createTask(body)],
    [/^\/api\/projects\/current\/changes\/([^/]+)\/review-file$/, (body, m) => this.update(m[1], body, s => ({ ...s, reviewed_files: [...new Set([...(s.reviewed_files ?? []), String(body.path)])], file_review_hash: s.hash }))],
    [/^\/api\/projects\/current\/changes\/([^/]+)\/workflow\/input$/, (body, m) => this.update(m[1], body, s => ({ ...s, workflow: { ...s.workflow!, input: { ...s.workflow!.input!, status: String(body.decision) === 'approve' ? 'answered' : 'rejected' }, status: body.decision === 'approve' ? 'awaiting_human_review' : 'canceled' } }))],
    [/^\/api\/projects\/current\/changes\/([^/]+)\/review$/, (body, m) => this.update(m[1], body, s => this.approve(s, body))],
    [/^\/api\/projects\/current\/changes\/([^/]+)\/review-override$/, (body, m) => this.update(m[1], body, s => this.override(s, body))],
    [/^\/api\/projects\/current\/changes\/([^/]+)\/comments$/, (body, m) => this.update(m[1], body, s => this.comment(s, body))],
    [/^\/api\/projects\/current\/changes\/([^/]+)\/comments\/delete$/, (body, m) => this.update(m[1], body, s => ({ ...s, comments: (s.comments ?? []).filter(c => c.id !== body.id || c.sent_revision) }))],
    [/^\/api\/projects\/current\/changes\/([^/]+)\/messages$/, (body, m) => this.update(m[1], body, s => this.revise(s, body))],
    [/^\/api\/projects\/current\/changes\/([^/]+)\/apply$/, (body, m) => this.apply(m[1], body)],
    [/^\/api\/projects\/current\/changes\/([^/]+)\/delete$/, (body, m) => this.deleteTask(m[1], body)],
    [/^\/api\/projects\/current\/agent-settings\/control$/, body => this.controlSettings(body)],
    [/^\/api\/projects\/current\/setup$/, body => this.saveSetup(body)],
    [/^\/api\/projects\/current\/setup\/skip$/, () => (this.setup = { ...this.setup, state: 'skipped' })],
    [/^\/api\/projects\/current\/setup\/test-model$/, body => ((body.model as { model?: string }).model ? { ok: true, model: (body.model as { model: string }).model, latency_ms: 840 } : { ok: false, latency_ms: 0, reason: 'The model provider could not be reached.' })],
    [/^\/api\/projects\/current\/analysis\/selection$/, body => (this.selection = { ...data.analysisSelection(body.excluded_paths as string[]) })],
    [/^\/api\/projects\/current\/analysis\/preview$/, body => ({ identity: { ...body, queue_id: 'queue-2' }, preview_id: 'preview-1', providers: [{ id: 'p-bug', model: { model: 'local-bug-model', provider_origin: 'http://localhost:1234' } }], files: [{}, {}, {}, {}], max_model_requests: 12, security_review_intent_required: false })],
    [/^\/api\/projects\/current\/analysis\/run$/, () => this.setRun('running')],
    [/^\/api\/projects\/current\/analysis\/run\/control$/, body => this.setRun(body.action === 'pause' ? 'paused' : body.action === 'cancel' ? 'canceled' : 'running')],
    [/^\/api\/projects\/current\/git\/merge-preview$/, () => ({ commits: 'bb22 Merge preview lists overlapping files', diff: '' })],
  ];

  private async handle(route: Route): Promise<void> {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    const body = method === 'GET' ? null : ((request.postDataJSON() ?? {}) as Record<string, unknown>);
    this.requests.push({ method, path: url.pathname, body });

    if (url.pathname.endsWith('/events')) {
      const frames = this.pending.map(event => `event: update\ndata: ${JSON.stringify(event)}\n\n`).join('');
      return route.fulfill({ status: 200, contentType: 'text/event-stream', body: ': connected\n\n' + frames });
    }
    if (request.headers()['x-mini-orca-web-ui'] !== '1') {
      return json(route, 403, { user_message: 'Missing the Web UI request marker.' });
    }
    if (method !== 'GET' && request.headers()['x-mini-orca-token'] !== WRITE_TOKEN) {
      return json(route, 401, { user_message: 'Reload the Web UI to get this daemon\'s write token.' });
    }
    try {
      if (method === 'GET') {
        const override = this.overrides.get(url.pathname);
        if (override) return json(route, 200, override());
        const session = url.pathname.match(/^\/api\/projects\/current\/changes\/([^/]+)$/);
        if (session && this.sessions.has(decodeURIComponent(session[1]))) return json(route, 200, this.sessions.get(decodeURIComponent(session[1])));
        const found = this.gets.find(([pattern]) => pattern.test(url.pathname));
        return found ? json(route, 200, found[1]()) : json(route, 404, { message: `no fake for GET ${url.pathname}` });
      }
      for (const [pattern, handler] of this.posts) {
        const match = url.pathname.match(pattern);
        if (match) return json(route, 200, handler(body ?? {}, match));
      }
      return json(route, 404, { message: `no fake for ${method} ${url.pathname}` });
    } catch (err) {
      return json(route, 409, { user_message: (err as Error).message });
    }
  }

  /** Saves setup like the daemon: models, roles and budget; secrets are never echoed. */
  private saveSetup(body: Record<string, unknown>) {
    if (body.project_revision !== this.setup.project_revision) throw new Error('The project changed; reload setup.');
    const models = body.models as ProjectSetup['models'];
    const echoed = Object.fromEntries(Object.entries(models).map(([scope, model]) => [scope, { ...model, api_key: undefined, api_key_set: Boolean(model.api_key) || Boolean(model.api_key_set) }]));
    this.setup = { ...this.setup, state: 'configured', models: echoed as ProjectSetup['models'], roles: body.roles as ProjectSetup['roles'], limits: body.limits as ProjectSetup['limits'] };
    return this.setup;
  }

  /** Replaces or resets agent settings with the daemon's version and configuration guard. */
  private controlSettings(body: Record<string, unknown>) {
    const current = this.settings;
    if (body.confirm !== true || body.version !== current.version || body.configuration !== current.configuration || body.project_revision !== current.project_revision) {
      throw new Error('Agent settings changed; reload before editing.');
    }
    if (body.action === 'reset') {
      this.settings = data.agentSettings(current.version! + 1, null, null);
      return this.settings;
    }
    const values = body.values as { roles: Record<string, string>; limits: Record<string, number>; disabled_skills: string[] } | undefined;
    if (body.action !== 'replace' || !values || !Array.isArray(values.disabled_skills)) throw new Error('Replace requires complete roles, limits and disabled skills.');
    this.settings = data.agentSettings(current.version! + 1, values.roles, values.limits);
    return this.settings;
  }

  /** Applies a guarded update: the request must name the current revision and hash. */
  private update(id: string, body: Record<string, unknown>, next: (session: ChangeSession) => ChangeSession): ChangeSession {
    const session = this.sessions.get(decodeURIComponent(id));
    if (!session) throw new Error('Task not found.');
    if (body.revision !== session.revision || body.hash !== session.hash || body.project_revision !== session.project_revision) {
      throw new Error('The proposal changed since it was loaded.');
    }
    const updated = next(session);
    this.sessions.set(session.id, updated);
    return updated;
  }

  /** Approval answers a waiting input and keeps the reviewer's notes with this hash. */
  private approve(s: ChangeSession, body: Record<string, unknown>): ChangeSession {
    const workflow = s.workflow;
    if (workflow && workflow.status !== 'awaiting_human_review') throw new Error('The workflow has not reached human review.');
    const notes = typeof body.notes === 'string' ? body.notes : '';
    const input = workflow?.input?.status === 'waiting' ? { ...workflow.input, status: 'approved' } : workflow?.input;
    return { ...s, reviewed_hash: s.hash, review_notes: notes || undefined, workflow: workflow && { ...workflow, input } };
  }

  /** Only the reviewers' rejection of this exact hash can be overridden, with a reason. */
  private override(s: ChangeSession, body: Record<string, unknown>): ChangeSession {
    const workflow = s.workflow;
    if (!workflow || body.workflow_id !== workflow.id) throw new Error('The workflow changed since it was loaded.');
    if (workflow.status !== 'changes_requested' || workflow.input || workflow.review?.proposal_hash !== s.hash) throw new Error('Only a proposal the model reviewers rejected can be overridden.');
    if (typeof body.reason !== 'string' || !body.reason.trim()) throw new Error('Give a reason for overriding the reviewers.');
    const stages = workflow.stages.map(stage => (stage.name === 'human_review' ? { ...stage, status: 'waiting' } : stage));
    return { ...s, workflow: { ...workflow, status: 'awaiting_human_review', stages, override: { proposal_hash: s.hash, reason: body.reason.trim(), at: data.ago(0) } } };
  }

  /** A comment anchors to a line of the current proposal and quotes it. */
  private comment(s: ChangeSession, body: Record<string, unknown>): ChangeSession {
    const edit = s.changes.find(change => change.path === body.path);
    const line = edit?.diff.lines.find(l => (body.side === 'old' ? l.kind === 'removed' && l.old_line === body.line : l.kind !== 'removed' && l.new_line === body.line));
    if (!line || typeof body.text !== 'string' || !body.text.trim()) throw new Error('Comment on a line of the current proposal.');
    const comment = { id: `cm-${this.nextId++}`, path: String(body.path), side: body.side as 'new' | 'old', line: Number(body.line), excerpt: line.text, text: body.text.trim(), hash: s.hash, created_at: data.ago(0) };
    return { ...s, comments: [...(s.comments ?? []), comment] };
  }

  /** A revision is a new version; the open comments it carried are marked sent. */
  private revise(s: ChangeSession, body: Record<string, unknown>): ChangeSession {
    const open = (s.comments ?? []).filter(comment => !comment.sent_revision);
    if (body.include_comments && !open.length) throw new Error('There are no open line comments to send.');
    const comments = (s.comments ?? []).map(comment => (body.include_comments && !comment.sent_revision ? { ...comment, sent_revision: s.revision } : comment));
    return { ...s, revision: s.revision + 1, hash: `${s.hash}-r${s.revision + 1}`, comments, reviewed_files: [], reviewed_hash: undefined, checks: [], messages: [...s.messages, { role: 'user', content: String(body.message || 'Line comments') }, { role: 'assistant', content: 'Addressed the comments.' }] };
  }

  private apply(id: string, body: Record<string, unknown>) {
    if (body.confirm !== true) throw new Error('Apply needs explicit confirmation.');
    const updated = this.update(id, body, s => {
      if (s.reviewed_hash !== s.hash) throw new Error('Approve this version first.');
      return { ...s, state: 'applied', commit: body.commit_message ? { state: 'committed', oid: 'c0ffee12', message: String(body.commit_message) } : undefined };
    });
    return { ...data.project, session_id: updated.id, state: 'applied', hash: updated.hash, undo_available: true, warnings: [], commit: updated.commit };
  }

  private createTask(body: Record<string, unknown>): ChangeSession {
    const id = `chg-${this.nextId++}`;
    const paths = (body.paths as string[] | undefined) ?? [];
    const session: ChangeSession = {
      ...data.project,
      id,
      kind: String(body.kind ?? 'fix'),
      title: String(body.title ?? ''),
      revision: 0,
      hash: '',
      state: 'draft',
      freshness: 'current',
      updated_at: data.ago(0),
      acceptance_criteria: (body.acceptance_criteria as string[] | undefined) ?? [],
      targets: paths.map(path => ({ path, hash: '', exists: data.indexedPaths.includes(path) })),
      changes: [],
      messages: [],
      checks: [],
    };
    this.sessions.set(id, session);
    return session;
  }
}

function json(route: Route, status: number, payload: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(payload) });
}

/** Tests get a fake daemon and fail on any console error or uncaught exception. */
export const test = base.extend<{ daemon: FakeDaemon }>({
  daemon: [
    async ({ page }, use) => {
      const daemon = new FakeDaemon();
      await daemon.install(page);
      const errors: string[] = [];
      page.on('pageerror', error => errors.push(String(error)));
      page.on('console', message => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await use(daemon);
      expect(errors.filter(error => !daemon.expectedConsole.some(pattern => pattern.test(error))), 'browser console errors').toEqual([]);
    },
    { auto: true },
  ],
});

export { expect };
