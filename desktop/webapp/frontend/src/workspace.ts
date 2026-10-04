import { API, ApiError, current, errorMessage, native } from './api';
import type * as M from './models';

export type Page =
  | 'welcome'
  | 'summary'
  | 'project'
  | 'analysis'
  | 'analysis-preview'
  | 'analysis-run'
  | 'bugs'
  | 'performance'
  | 'security'
  | 'editor'
  | 'context'
  | 'assistant'
  | 'new-declaration'
  | 'draft'
  | 'checks'
  | 'review'
  | 'receipt'
  | 'benchmark'
  | 'terminal'
  | 'search'
  | 'models'
  | 'diagrams'
  | 'scan'
  | 'manifest';
export interface Confirmation {
  title: string;
  message: string;
  accept: string;
  details?: string[];
  destructive?: boolean;
}
export interface State {
  page: Page;
  busy: string;
  error: string;
  notice: string;
  connected: boolean;
  version: string;
  project?: M.Project;
  index?: M.ProjectIndex;
  overview?: M.Overview;
  models?: M.ModelCatalog;
  findings?: M.Finding[];
  selection?: M.Selection;
  run?: M.AnalysisRun | null;
  preview?: M.AnalysisPreview;
  resume?: M.RunIdentity;
  results: Partial<Record<string, M.SectionResults>>;
  resourceErrors: Record<string, string>;
  file?: M.FileInfo;
  symbols: M.SymbolInfo[];
  symbol: string;
  fileStale: boolean;
  fileAnalysis?: M.FileAnalysis | null;
  explanation?: M.Explanation;
  context?: M.ContextManifest;
  impact?: M.Impact;
  git?: M.GitStatus;
  securityReport?: M.SecurityReport;
  draft?: M.Draft;
  declaration: string;
  imports: string;
  dirty: boolean;
  checks?: M.Checks;
  session?: M.ChatSession;
  messages: { role: string; content: string }[];
  task?: M.TaskSpec;
  reviewed: string;
  receipt?: M.Receipt;
  uncertain: boolean;
  benchmarkCatalog?: M.BenchmarkCatalog;
  benchmark?: M.BenchmarkResult;
  scan?: M.Scan | null;
  confirmation?: Confirmation;
  terminals: M.TerminalUpdate[];
  chosenPath?: string;
}
const initial = (): State => ({
  page: 'welcome',
  busy: '',
  error: '',
  notice: '',
  connected: false,
  version: '',
  results: {},
  resourceErrors: {},
  symbols: [],
  symbol: '',
  fileStale: true,
  declaration: '',
  imports: '',
  dirty: false,
  messages: [],
  reviewed: '',
  uncertain: false,
  terminals: [],
});
const projectKey = (p?: M.ProjectIdentity) => (p ? `${p.project_id}:${p.project_revision}` : '');
const fileKey = (s: State) => `${projectKey(s.project)}:${s.file?.path}:${s.file?.content_hash}`;
export const candidateKey = (d?: M.Draft) =>
  d ? `${projectKey(d)}:${d.id}:${d.revision}:${d.hash}:${d.base_file_hash}` : '';
export const activeRun = (run?: M.AnalysisRun | null) =>
  !!run && ['queued', 'running', 'pausing', 'canceling'].includes(run.status);
export function matchesCandidate(e: M.CandidateIdentity, d: M.Draft) {
  return (
    projectKey(e) === projectKey(d) &&
    e.draft_id === d.id &&
    e.draft_revision === d.revision &&
    e.draft_hash === d.hash &&
    e.base_file_hash === d.base_file_hash &&
    e.target_path === d.target_path
  );
}
export function currentDraft(s: State) {
  return (
    !!s.draft &&
    !s.dirty &&
    !s.fileStale &&
    !s.uncertain &&
    projectKey(s.draft) === projectKey(s.project) &&
    s.draft.target_path === s.file?.path &&
    s.draft.base_file_hash === s.file?.content_hash &&
    s.draft.validation?.applicable === true
  );
}
export function canApply(s: State) {
  return (
    currentDraft(s) &&
    !!s.draft &&
    !!s.checks &&
    s.checks.applicable &&
    matchesCandidate(s.checks, s.draft) &&
    s.checks.candidate_hash === s.draft.candidate_hash &&
    s.reviewed === candidateKey(s.draft)
  );
}

export function canCancelOperation(label: string) {
  return (
    !!label &&
    ![
      'Apply change',
      'Undo change',
      'Import project',
      'Open project',
      'Choose project',
      'Start terminal',
      'Close terminal',
    ].includes(label)
  );
}

export class Workspace {
  state = initial();
  private listeners = new Set<() => void>();
  private api = new API();
  private epoch = 0;
  private fileEpoch = 0;
  private operation = 0;
  private answer?: (accepted: boolean) => void;
  private timer?: ReturnType<typeof setTimeout>;
  private stopped = false;
  private explicitProjectAction = false;
  private fileRefresh: Promise<void> = Promise.resolve();
  private resultRequests: Record<string, number> = {};
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };
  snapshot = () => this.state;
  set(patch: Partial<State>) {
    this.state = { ...this.state, ...patch };
    this.listeners.forEach((fn) => fn());
  }
  fail(error: unknown) {
    this.set({ error: errorMessage(error) });
  }
  private confirm(value: Confirmation): Promise<boolean> {
    return new Promise((resolve) => {
      this.answer = resolve;
      this.set({ confirmation: value });
    });
  }
  respond(accepted: boolean) {
    const answer = this.answer;
    this.answer = undefined;
    this.set({ confirmation: undefined });
    answer?.(accepted);
  }
  async act(label: string, action: () => Promise<void>) {
    if (this.state.busy) return;
    const operation = ++this.operation;
    this.set({ busy: label, error: '', notice: '' });
    try {
      await action();
    } catch (error) {
      if (operation !== this.operation) return;
      if (error instanceof DOMException && error.name === 'AbortError')
        this.set({ notice: 'Canceled.' });
      else this.fail(error);
      if (error instanceof ApiError && error.status === 409)
        this.set({ fileStale: true, reviewed: '', preview: undefined });
    } finally {
      if (operation === this.operation) this.set({ busy: '' });
    }
  }
  async start() {
    this.stopped = false;
    await this.connect();
    if (this.state.connected) {
      const path = localStorage.getItem('mini-orca:last-project');
      if (path && !this.explicitProjectAction) await this.openProject(path, false, false);
    }
    this.schedule();
  }
  stop() {
    this.stopped = true;
    clearTimeout(this.timer);
    this.respond(false);
    void this.api.cancelAll().catch((error) => this.fail(error));
  }
  private schedule() {
    if (this.stopped) return;
    this.timer = setTimeout(
      async () => {
        await this.poll();
        this.schedule();
      },
      activeRun(this.state.run) || this.state.scan?.status === 'running' ? 1000 : 5000,
    );
  }
  async connect() {
    try {
      const status = await this.api.get<{ version: string }>('/status');
      this.set({ connected: true, version: status.version });
    } catch (error) {
      this.set({ connected: false });
      this.fail(error);
    }
    if (this.state.connected)
      await this.resource(
        'models',
        () => this.api.get<M.ModelCatalog>('/api/models/current'),
        (models) => this.set({ models }),
      );
  }
  private async resource<T>(
    name: string,
    request: () => Promise<T>,
    commit: (value: T) => void,
    guard = () => true,
  ) {
    const epoch = this.epoch;
    try {
      const result = await request();
      if (epoch !== this.epoch || !guard()) return;
      const resourceErrors = { ...this.state.resourceErrors };
      delete resourceErrors[name];
      this.set({ resourceErrors });
      commit(result);
    } catch (error) {
      if (
        epoch === this.epoch &&
        guard() &&
        !(error instanceof DOMException && error.name === 'AbortError')
      )
        this.set({ resourceErrors: { ...this.state.resourceErrors, [name]: errorMessage(error) } });
    }
  }
  private identity(): M.ProjectIdentity {
    const p = this.state.project;
    if (!p) throw new Error('Open a project first.');
    return { project_id: p.project_id, project_revision: p.project_revision };
  }
  async chooseProject() {
    this.explicitProjectAction = true;
    await this.act('Choose project', async () => {
      const path = await native().ChooseDirectory();
      if (path) this.set({ page: 'project', chosenPath: path });
    });
  }
  async openProject(path: string, analyze: boolean, explicit = true) {
    if (explicit) this.explicitProjectAction = true;
    if (!path.trim()) return;
    await this.act(analyze ? 'Import project' : 'Open project', async () => {
      if (
        this.state.draft &&
        !(await this.confirm({
          title: 'Switch project?',
          message: 'Discard the current draft and close terminal sessions.',
          accept: 'Switch project',
          destructive: true,
        }))
      )
        return;
      if (
        !this.state.draft &&
        this.state.terminals.length &&
        !(await this.confirm({
          title: 'Switch project?',
          message: 'Terminal sessions will close.',
          accept: 'Switch project',
        }))
      )
        return;
      const confirmed = analyze
        ? await this.confirmModel('analyze', 'Import and analyze project')
        : false;
      if (analyze && confirmed === null) return;
      const project = await this.api.post<M.Project>(
        `/api/projects/${analyze ? 'import' : 'restore'}`,
        { project_path: path.trim(), ...(analyze ? { confirm_remote_provider: confirmed } : {}) },
      );
      this.epoch++;
      this.fileEpoch++;
      this.state = {
        ...initial(),
        connected: true,
        version: this.state.version,
        models: this.state.models,
        busy: this.state.busy,
        page: 'summary',
        project,
      };
      localStorage.setItem('mini-orca:last-project', project.path);
      await native().SetUnsavedDraft(false);
      this.set({});
      await this.refreshProject();
    });
  }
  async refreshProject() {
    const identity = this.identity();
    await Promise.all([
      this.resource(
        'overview',
        () => this.api.get<M.Overview>(`${current}/overview`, identity),
        (overview) => this.set({ overview }),
      ),
      this.resource(
        'index',
        () => this.api.get<M.ProjectIndex>(`${current}/index`),
        (index) => {
          if (projectKey(index) !== projectKey(this.state.project))
            throw new Error('Project revision changed. Refresh project facts.');
          this.set({ index });
        },
      ),
      this.resource(
        'findings',
        () => this.api.get<M.FindingsResponse>(`${current}/findings`, identity),
        (result) => this.set({ findings: result.findings || [] }),
      ),
      this.resource(
        'run',
        () => this.api.get<M.AnalysisRun | null>(`${current}/analysis/run`, identity),
        (run) => this.set({ run }),
      ),
      this.resource(
        'selection',
        () => this.api.get<M.Selection>(`${current}/analysis/selection`, identity),
        (selection) => this.set({ selection }),
      ),
      this.resource(
        'scan',
        () => this.api.get<M.Scan | null>(`${current}/scan`, identity),
        (scan) => this.set({ scan }),
      ),
    ]);
  }
  async reindex() {
    await this.act('Refresh project facts', async () => {
      if (
        this.state.draft &&
        !(await this.confirm({
          title: 'Refresh project?',
          message: 'Discard the current draft and refresh file identities.',
          accept: 'Refresh',
          destructive: true,
        }))
      )
        return;
      const index = await this.api.post<M.ProjectIndex>(`${current}/reindex`, {
        project_revision: this.identity().project_revision,
      });
      this.epoch++;
      this.fileEpoch++;
      this.set({
        project: { ...this.state.project!, project_revision: index.project_revision },
        index,
        draft: undefined,
        checks: undefined,
        reviewed: '',
        file: undefined,
        fileStale: true,
        results: {},
        preview: undefined,
        receipt: undefined,
        uncertain: false,
        dirty: false,
        context: undefined,
        explanation: undefined,
        benchmark: undefined,
        benchmarkCatalog: undefined,
        session: undefined,
        messages: [],
        task: undefined,
        securityReport: undefined,
      });
      await native().SetUnsavedDraft(false);
      await this.refreshProject();
    });
  }
  async navigate(page: Page) {
    this.set({ page, error: '' });
    if (['editor', 'draft', 'checks', 'review', 'assistant', 'benchmark'].includes(page))
      await this.refreshFile();
    if (['bugs', 'performance', 'security'].includes(page)) await this.loadResults(page);
    if (page === 'context' || page === 'manifest') await this.inspectContext();
  }
  private async poll() {
    if (this.state.busy || !this.state.project) return;
    const identity = this.identity();
    await this.resource(
      'connection',
      () => this.api.get<{ version: string }>('/status'),
      () => this.set({ connected: true }),
    );
    if (this.state.resourceErrors.connection) {
      this.set({ connected: false });
      return;
    }
    const old = JSON.stringify(this.state.run);
    await this.resource(
      'run',
      () => this.api.get<M.AnalysisRun | null>(`${current}/analysis/run`, identity),
      (run) => this.set({ run }),
    );
    if (JSON.stringify(this.state.run) !== old) {
      await this.refreshProject();
      if (['bugs', 'performance', 'security'].includes(this.state.page))
        await this.loadResults(this.state.page);
    }
    if (this.state.scan?.status === 'running') {
      await this.resource(
        'scan',
        () => this.api.get<M.Scan>(`${current}/scan`, identity),
        (scan) => this.set({ scan }),
      );
      if (this.state.scan?.status !== 'running') await this.refreshProject();
    }
  }
  async saveSelection(paths: string[]) {
    await this.act('Save selection', async () => {
      const selection = this.state.selection;
      if (!selection?.editable)
        throw new Error('Finish or cancel the current run before changing selection.');
      const updated = await this.api.post<M.Selection>(`${current}/analysis/selection`, {
        ...this.identity(),
        selection_id: selection.selection_id,
        excluded_paths: paths,
      });
      this.set({ selection: updated, preview: undefined });
    });
  }
  async previewAnalysis(limits: M.Limits, refresh = false, retry = false, resume = false) {
    await this.act('Prepare analysis', async () => {
      const run = resume ? this.state.run : undefined;
      const preview = await this.api.post<M.AnalysisPreview>(`${current}/analysis/preview`, {
        ...this.identity(),
        scope: 'project',
        limits: run?.plan.limits || limits,
        refresh: run?.plan.refresh || refresh,
        retry_stale_failed: run?.plan.retry_stale_failed || retry,
        ...(run ? { resume_run: run.identity } : {}),
      });
      this.set({ preview, resume: run?.identity, page: 'analysis-preview' });
    });
  }
  async startAnalysis(providerIds: string[], securityReview: boolean) {
    await this.act('Start analysis', async () => {
      const p = this.state.preview;
      if (!p || projectKey(p.identity) !== projectKey(this.state.project))
        throw new Error('Prepare a fresh analysis preview.');
      const confirmations = { provider_ids: providerIds, security_review: securityReview };
      const run = this.state.resume
        ? await this.api.post<M.AnalysisRun>(`${current}/analysis/run/control`, {
            identity: this.state.resume,
            action: 'resume',
            preview_id: p.preview_id,
            confirmations,
          })
        : await this.api.post<M.AnalysisRun>(`${current}/analysis/run`, {
            identity: p.identity,
            preview_id: p.preview_id,
            limits: p.limits,
            refresh: p.refresh,
            retry_stale_failed: p.retry_stale_failed || false,
            confirmations,
          });
      this.set({ run, preview: undefined, resume: undefined, results: {}, page: 'analysis-run' });
    });
  }
  async controlRun(action: 'pause' | 'cancel') {
    await this.act(`${action === 'pause' ? 'Pause' : 'Cancel'} analysis`, async () => {
      if (!this.state.run) return;
      const run = await this.api.post<M.AnalysisRun>(`${current}/analysis/run/control`, {
        identity: this.state.run.identity,
        action,
      });
      this.set({ run, preview: undefined });
    });
  }
  async loadResults(category: string, path = '') {
    const run = this.state.run;
    if (!run) return;
    const id = JSON.stringify(run.identity);
    const request = (this.resultRequests[category] || 0) + 1;
    this.resultRequests[category] = request;
    await this.resource(
      category,
      () =>
        this.api.get<M.SectionResults>(`${current}/analysis/results`, {
          ...run.identity,
          category,
          path,
        }),
      (result) => {
        if (JSON.stringify(result.identity) !== id)
          throw new Error('Results belong to an earlier analysis.');
        this.set({ results: { ...this.state.results, [category]: result } });
      },
      () =>
        JSON.stringify(this.state.run?.identity) === id &&
        this.resultRequests[category] === request,
    );
  }
  async triage(finding: M.Finding, status: string) {
    await this.act('Update finding', async () => {
      await this.api.request('PATCH', `${current}/findings/${encodeURIComponent(finding.id)}`, {
        project_revision: this.identity().project_revision,
        status,
      });
      await this.refreshProject();
      await this.loadResults(this.state.page);
    });
  }
  async openFile(path: string, symbol = '', task?: M.TaskSpec) {
    if (this.state.busy) return;
    if (this.state.file?.path === path) {
      if (symbol && symbol !== this.state.symbol && this.state.draft && !(await this.discard()))
        return;
      this.set({ page: 'editor', symbol: symbol || this.state.symbol, task });
      await this.refreshFile();
      return;
    }
    if (this.state.draft && !(await this.discard())) return;
    const epoch = ++this.fileEpoch;
    this.set({
      file: undefined,
      symbols: [],
      symbol,
      fileStale: true,
      fileAnalysis: undefined,
      explanation: undefined,
      context: undefined,
      impact: undefined,
      git: undefined,
      securityReport: undefined,
      session: undefined,
      messages: [],
      task,
      page: 'editor',
      error: '',
    });
    await this.resource(
      'file',
      () => this.api.get<M.FileInfo>(`${current}/files/info`, { path }),
      (file) =>
        this.set({
          file,
          fileStale:
            this.state.index?.files.find((f) => f.path === path)?.content_hash !==
            file.content_hash,
        }),
      () => epoch === this.fileEpoch,
    );
    if (epoch !== this.fileEpoch || !this.state.file) return;
    await Promise.all([
      this.resource(
        'symbols',
        () =>
          this.api.get<M.ProjectIdentity & { symbols: M.SymbolInfo[] }>(
            `${current}/files/symbols`,
            { path },
          ),
        (result) => {
          if (projectKey(result) !== projectKey(this.state.project))
            throw new Error('Symbol index changed. Refresh the project.');
          this.set({ symbols: result.symbols || [] });
        },
        () => epoch === this.fileEpoch,
      ),
      this.resource(
        'file analysis',
        () =>
          this.api.get<M.FileAnalysis | null>(`${current}/files/analysis`, {
            path,
            project_revision: this.identity().project_revision,
          }),
        (fileAnalysis) => this.set({ fileAnalysis }),
        () => epoch === this.fileEpoch,
      ),
      this.resource(
        'git',
        () => this.api.get<M.GitStatus>(`${current}/git`, { path }),
        (git) => this.set({ git }),
        () => epoch === this.fileEpoch,
      ),
    ]);
  }
  async selectSymbol(symbol: string) {
    if (this.state.busy) return;
    if (symbol !== this.state.symbol && this.state.draft && !(await this.discard())) return;
    this.fileEpoch++;
    this.set({
      symbol,
      explanation: undefined,
      context: undefined,
      impact: undefined,
      session: undefined,
      messages: [],
      task: undefined,
    });
  }
  refreshFile() {
    this.fileRefresh = this.fileRefresh.then(() => this.readFreshFile());
    return this.fileRefresh;
  }
  private async readFreshFile() {
    const old = this.state.file;
    if (!old) return;
    const epoch = this.fileEpoch;
    try {
      const file = await this.api.get<M.FileInfo>(`${current}/files/info`, { path: old.path });
      if (epoch !== this.fileEpoch) return;
      if (file.content_hash !== old.content_hash) {
        const indexed = this.state.index?.files.find((f) => f.path === file.path);
        const stale = indexed?.content_hash !== file.content_hash;
        this.set({
          file,
          fileStale: stale,
          symbols: stale ? [] : indexed?.symbols || [],
          reviewed: '',
          checks: undefined,
          benchmark: undefined,
          benchmarkCatalog: undefined,
          explanation: undefined,
          fileAnalysis: undefined,
          context: undefined,
          securityReport: undefined,
          notice: stale
            ? 'This file changed. Refresh project facts to continue.'
            : this.state.notice,
        });
      }
    } catch (error) {
      if (epoch === this.fileEpoch) {
        this.set({ fileStale: true, reviewed: '' });
        this.fail(error);
      }
    }
  }
  async inspectContext() {
    if (!this.state.file) return;
    const key = fileKey(this.state);
    const epoch = this.fileEpoch;
    const path = this.state.file.path;
    const symbol = this.state.symbol;
    const guard = () => fileKey(this.state) === key && epoch === this.fileEpoch;
    await this.resource(
      'context',
      () => this.api.get<M.ContextManifest>(`${current}/context`, { path, action: 'fix' }),
      (context) => this.set({ context }),
      guard,
    );
    if (symbol && guard())
      await this.resource(
        'impact',
        () => this.api.get<M.Impact>(`${current}/impact`, { path, symbol }),
        (impact) => this.set({ impact }),
        guard,
      );
  }
  private async confirmModel(
    scope: string,
    title: string,
    explicit = false,
  ): Promise<boolean | null> {
    const models = await this.api.get<M.ModelCatalog>('/api/models/current');
    this.set({ models });
    const model = models.scopes[scope];
    if (!model) throw new Error(`No ${scope} provider is configured.`);
    if (!model.remote_provider && !explicit) return false;
    const accepted = await this.confirm({
      title,
      message: model.remote_provider
        ? 'Send the selected context to this provider?'
        : 'Request an AI Security review of this file?',
      accept: 'Continue',
      details: [
        model.model,
        model.provider_origin,
        `Scope: ${scope}`,
        this.state.file?.path || 'Project context',
      ],
    });
    if (!accepted) return null;
    const latest = await this.api.get<M.ModelCatalog>('/api/models/current');
    if (JSON.stringify(latest.scopes[scope]) !== JSON.stringify(model))
      throw new Error('Provider configuration changed. Review the new destination and try again.');
    return model.remote_provider;
  }
  private target() {
    const file = this.state.file;
    if (!file || this.state.fileStale)
      throw new Error('Refresh the selected file before continuing.');
    return {
      ...this.identity(),
      base_file_hash: file.content_hash,
      target_path: file.path,
      target_symbol: this.state.symbol,
    };
  }
  async analyzeFile() {
    await this.act('Analyze file', async () => {
      await this.refreshFile();
      const target = this.target();
      const key = fileKey(this.state);
      const confirmed = await this.confirmModel('bug', 'Analyze file');
      if (confirmed === null) return;
      const fileAnalysis = await this.api.post<M.FileAnalysis>(`${current}/files/analysis`, {
        path: target.target_path,
        project_revision: target.project_revision,
        refresh: true,
        confirm_remote_provider: confirmed,
      });
      if (fileKey(this.state) === key) this.set({ fileAnalysis });
      await this.refreshProject();
    });
  }
  async explain() {
    await this.act('Explain declaration', async () => {
      await this.refreshFile();
      const target = this.target();
      const key = fileKey(this.state);
      if (!target.target_symbol) throw new Error('Select a declaration.');
      const confirmed = await this.confirmModel('function', 'Explain declaration');
      if (confirmed === null) return;
      const explanation = await this.api.post<M.Explanation>(`${current}/files/explanation`, {
        ...target,
        confirm_remote_provider: confirmed,
      });
      if (fileKey(this.state) === key && explanation.base_file_hash === target.base_file_hash)
        this.set({ explanation, page: 'assistant' });
    });
  }
  async securityReview(ai: boolean) {
    await this.act(ai ? 'Security review' : 'Scan source', async () => {
      await this.refreshFile();
      const t = this.target();
      const key = fileKey(this.state);
      const confirmed = ai ? await this.confirmModel('analyze', 'Security review', true) : false;
      if (confirmed === null) return;
      const securityReport = await this.api.post<M.SecurityReport>(
        `${current}/${ai ? 'security-review' : 'files/security-scan'}`,
        ai
          ? {
              project_id: t.project_id,
              project_revision: t.project_revision,
              base_file_hash: t.base_file_hash,
              path: t.target_path,
              symbol: t.target_symbol,
              confirm_remote_provider: confirmed,
            }
          : { path: t.target_path, project_revision: t.project_revision },
      );
      if (fileKey(this.state) === key) this.set({ securityReport, page: 'security' });
    });
  }
  async generate(
    message: string,
    mode: 'replace_symbol' | 'create_symbol',
    name: string,
    repair = false,
  ) {
    await this.act(repair ? 'Repair draft' : 'Prepare draft', async () => {
      if (!message.trim()) throw new Error('Describe the change.');
      await this.refreshFile();
      const target = this.target();
      const key = fileKey(this.state);
      if (
        this.state.draft &&
        (this.state.draft.target_symbol !== name || this.state.draft.mode !== mode) &&
        !(await this.discard())
      )
        return;
      const confirmed = await this.confirmModel('function', 'Prepare draft');
      if (confirmed === null) return;
      const old = this.state.session;
      const session =
        old &&
        old.target_symbol === name &&
        old.mode === mode &&
        old.base_file_hash === target.base_file_hash
          ? old
          : await this.api.post<M.ChatSession>(`${current}/chat/sessions`, {
              ...this.identity(),
              base_file_hash: target.base_file_hash,
              open_path: target.target_path,
              mode,
              target_symbol: name,
              ...(this.state.task ? { task_spec: this.state.task } : {}),
            });
      if (
        projectKey(session) !== projectKey(target) ||
        session.base_file_hash !== target.base_file_hash ||
        session.open_path !== target.target_path ||
        session.target_symbol !== name ||
        session.mode !== mode
      )
        throw new Error('Chat session does not match this declaration.');
      const proposal = await this.api.post<M.Proposal>(
        `${current}/chat/sessions/${encodeURIComponent(session.id)}/messages`,
        {
          message: message.trim(),
          parent_draft_id: this.state.draft?.id || session.latest_draft_id || '',
          confirm_remote_provider: confirmed,
          repair,
        },
      );
      if (fileKey(this.state) !== key) return;
      const d = proposal.draft;
      if (
        proposal.session_id !== session.id ||
        projectKey(d) !== projectKey(target) ||
        d.base_file_hash !== target.base_file_hash ||
        d.target_path !== target.target_path ||
        d.target_symbol !== name ||
        d.mode !== mode
      )
        throw new Error('The returned draft does not match this declaration.');
      this.set({
        session: { ...session, latest_draft_id: d.id },
        draft: d,
        declaration: d.declaration,
        imports: (d.imports || []).join('\n'),
        dirty: false,
        checks: undefined,
        reviewed: '',
        benchmark: undefined,
        benchmarkCatalog: undefined,
        receipt: undefined,
        context: proposal.context_manifest,
        messages: [
          ...this.state.messages,
          { role: 'user', content: message },
          proposal.assistant_message,
        ],
        page: 'draft',
      });
      await native().SetUnsavedDraft(true);
    });
  }
  editDraft(declaration: string, imports: string) {
    if (this.state.busy || !this.state.draft) return;
    this.set({
      declaration,
      imports,
      dirty: true,
      checks: undefined,
      reviewed: '',
      benchmark: undefined,
      benchmarkCatalog: undefined,
    });
  }
  async discard() {
    if (!this.state.draft) return true;
    if (
      !(await this.confirm({
        title: 'Discard draft?',
        message: `Discard the draft for ${this.state.draft.target_symbol}.`,
        accept: 'Discard draft',
        destructive: true,
      }))
    )
      return false;
    this.set({
      draft: undefined,
      dirty: false,
      checks: undefined,
      reviewed: '',
      benchmark: undefined,
      benchmarkCatalog: undefined,
      session: undefined,
      messages: [],
      declaration: '',
      imports: '',
    });
    await native().SetUnsavedDraft(false);
    return true;
  }
  async validate() {
    await this.act('Validate draft', async () => {
      await this.refreshFile();
      const t = this.target();
      const d = this.state.draft;
      if (!d || d.base_file_hash !== t.base_file_hash || projectKey(d) !== projectKey(t))
        throw new Error('This draft is outdated.');
      const updated = await this.api.request<M.Draft>(
        'PATCH',
        `${current}/drafts/${encodeURIComponent(d.id)}`,
        {
          project_revision: t.project_revision,
          expected_revision: d.revision,
          declaration: this.state.declaration,
          imports: this.state.imports
            .split('\n')
            .map((s) => s.trim())
            .filter(Boolean),
        },
      );
      if (
        updated.id !== d.id ||
        updated.base_file_hash !== d.base_file_hash ||
        projectKey(updated) !== projectKey(d)
      )
        throw new Error('Draft update returned a different target.');
      this.set({
        draft: updated,
        dirty: false,
        checks: undefined,
        reviewed: '',
        benchmark: undefined,
        benchmarkCatalog: undefined,
      });
      const draft = await this.api.post<M.Draft>(
        `${current}/drafts/${encodeURIComponent(updated.id)}/validate`,
        { project_revision: t.project_revision, expected_revision: updated.revision },
      );
      if (candidateKey(draft) !== candidateKey(updated))
        throw new Error('Validation belongs to an earlier draft.');
      this.set({
        draft,
        declaration: draft.declaration,
        imports: (draft.imports || []).join('\n'),
      });
    });
  }
  private draftParams() {
    if (!currentDraft(this.state)) throw new Error('Validate the current draft first.');
    const d = this.state.draft!;
    return {
      project_revision: d.project_revision,
      expected_revision: d.revision,
      expected_hash: d.hash,
    };
  }
  private async trust(): Promise<boolean> {
    const params = {
      project_revision: this.identity().project_revision,
      task_test_name: this.state.draft?.task_spec?.go_test_candidate?.name,
    };
    const trust = await this.api.get<M.ExecutionTrust>(`${current}/execution-trust`, params);
    if (trust.trusted) return true;
    if (
      !(await this.confirm({
        title: 'Allow project code to run?',
        message: 'Checks run in an isolated copy. Project code can still access this computer.',
        accept: 'Trust this project',
        details: (trust.commands || []).map((argv) => argv.join(' ')),
      }))
    )
      return false;
    await this.api.post(`${current}/execution-trust`, {
      project_revision: params.project_revision,
      confirm: true,
    });
    return true;
  }
  async runChecks(lint: boolean, tests: boolean) {
    await this.act('Run checks', async () => {
      await this.refreshFile();
      const params = this.draftParams();
      const d = this.state.draft!;
      if ((lint || tests || d.task_spec?.go_test_candidate) && !(await this.trust())) return;
      this.set({ checks: undefined, reviewed: '' });
      const checks = await this.api.post<M.Checks>(
        `${current}/drafts/${encodeURIComponent(d.id)}/checks`,
        { ...params, run_lint: lint, run_tests: tests },
      );
      if (!matchesCandidate(checks, d) || checks.candidate_hash !== d.candidate_hash)
        throw new Error('Checks do not match this draft.');
      this.set({ checks, page: 'checks' });
    });
  }
  async review() {
    await this.act('Prepare review', async () => {
      await this.refreshFile();
      this.draftParams();
      this.set({ reviewed: candidateKey(this.state.draft), page: 'review' });
    });
  }
  async apply() {
    await this.act('Apply change', async () => {
      await this.refreshFile();
      if (!canApply(this.state))
        throw new Error('Review the current draft and complete its required checks.');
      const d = this.state.draft!;
      const receipt = await this.writeSource('apply', {
        draft_id: d.id,
        draft_revision: d.revision,
        draft_hash: d.hash,
        project_id: d.project_id,
        project_revision: d.project_revision,
        base_file_hash: d.base_file_hash,
        confirm: true,
      });
      this.acceptReceipt(receipt);
      await native().SetUnsavedDraft(false);
      await this.refreshProject();
      await this.refreshFile();
    });
  }
  private async writeSource(action: 'apply' | 'undo', body: object) {
    try {
      return await this.api.post<M.Receipt>(`${current}/${action}`, body);
    } catch (error) {
      if (!(error instanceof ApiError && error.status >= 400 && error.status < 500))
        this.set({
          uncertain: true,
          reviewed: '',
          checks: undefined,
          notice:
            'The source operation could not be confirmed. Refresh the project before continuing.',
        });
      throw error;
    }
  }
  private acceptReceipt(receipt: M.Receipt) {
    this.epoch++;
    this.set({
      receipt,
      project: { ...this.state.project!, project_revision: receipt.project_revision },
      index: receipt.index,
      draft: undefined,
      dirty: false,
      checks: undefined,
      reviewed: '',
      session: undefined,
      messages: [],
      task: undefined,
      benchmarkCatalog: undefined,
      benchmark: undefined,
      results: {},
      preview: undefined,
      context: undefined,
      explanation: undefined,
      fileAnalysis: undefined,
      securityReport: undefined,
      page: 'receipt',
      notice: (receipt.warnings || []).join('\n'),
    });
  }
  async undo() {
    await this.act('Undo change', async () => {
      const r = this.state.receipt;
      if (!r?.undo_available || this.state.uncertain) throw new Error('Undo is unavailable.');
      if (
        !(await this.confirm({
          title: 'Undo this change?',
          message: r.audit?.target_path || 'Restore the source before the last Apply.',
          accept: 'Undo change',
          destructive: true,
        }))
      )
        return;
      const receipt = await this.writeSource('undo', {
        ...this.identity(),
        post_apply_hash: r.post_apply_hash,
        confirm: true,
      });
      this.acceptReceipt(receipt);
      await native().SetUnsavedDraft(false);
      await this.refreshProject();
      await this.refreshFile();
    });
  }
  async benchmarks() {
    await this.act('Find benchmarks', async () => {
      await this.refreshFile();
      const params = this.draftParams();
      const d = this.state.draft!;
      const catalog = await this.api.get<M.BenchmarkCatalog>(
        `${current}/drafts/${encodeURIComponent(d.id)}/benchmarks`,
        params,
      );
      if (!matchesCandidate(catalog, d)) throw new Error('Benchmark catalog is outdated.');
      this.set({ benchmarkCatalog: catalog, page: 'benchmark' });
    });
  }
  async compare(choice: M.BenchmarkChoice) {
    await this.act('Compare benchmark', async () => {
      await this.refreshFile();
      const params = this.draftParams();
      const d = this.state.draft!;
      const catalog = this.state.benchmarkCatalog;
      if (
        !catalog ||
        !matchesCandidate(catalog, d) ||
        !catalog.benchmarks.some((c) => c.name === choice.name && c.scope === choice.scope)
      )
        throw new Error('Refresh the benchmark catalog.');
      if (!(await this.trust())) return;
      const benchmark = await this.api.post<M.BenchmarkResult>(
        `${current}/drafts/${encodeURIComponent(d.id)}/benchmarks`,
        { ...params, benchmark: choice.name, expected_scope: choice.scope },
      );
      if (!matchesCandidate(benchmark, d)) throw new Error('Benchmark result is outdated.');
      this.set({ benchmark });
    });
  }
  async scanProject(cancel = false) {
    await this.act(cancel ? 'Cancel scan' : 'Run verified scan', async () => {
      const revision = { project_revision: this.identity().project_revision };
      if (!cancel && !(await this.trust())) return;
      const scan = cancel
        ? await this.api.request<M.Scan>(
            'DELETE',
            `${current}/scan?project_revision=${encodeURIComponent(revision.project_revision)}`,
          )
        : await this.api.post<M.Scan>(`${current}/scan`, revision);
      this.set({ scan, page: 'scan' });
    });
  }
  async openTerminal() {
    await this.act('Start terminal', async () => {
      if (!this.state.project) return;
      const terminal = await native().OpenTerminal(this.state.project.path, 100, 28);
      this.set({ terminals: [...this.state.terminals, terminal], page: 'terminal' });
    });
  }
  async closeTerminal(id: string) {
    await this.act('Close terminal', async () => {
      await native().CloseTerminal(id);
      this.set({ terminals: this.state.terminals.filter((t) => t.id !== id) });
      await this.refreshFile();
    });
  }
  async cancel() {
    if (!canCancelOperation(this.state.busy)) return;
    this.operation++;
    this.respond(false);
    await this.api.cancelAll().catch((error) => this.fail(error));
    this.set({
      busy: '',
      notice: 'Canceled. Refresh status before retrying.',
      reviewed: '',
      preview: undefined,
    });
  }
}
export const workspace = new Workspace();
