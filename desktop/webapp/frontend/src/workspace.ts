import { API, ApiError, current, errorMessage, native } from './api';
import type * as M from './models';

export type Page =
  | 'features'
  | 'instructions'
  | 'chat'
  | 'changes'
  | 'history'
  | 'welcome'
  | 'summary'
  | 'project'
  | 'analysis'
  | 'analysis-files'
  | 'analysis-preview'
  | 'analysis-run'
  | 'bugs'
  | 'performance'
  | 'security'
  | 'editor'
  | 'context'
  | 'terminal'
  | 'search'
  | 'models'
  | 'diagrams'
  | 'scan';
export interface Confirmation {
  title: string;
  message: string;
  accept: string;
  details?: string[];
  destructive?: boolean;
}
export interface FixPreparationContext {
  models: M.ModelCatalog;
  trust: M.ExecutionTrust;
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
  availableModels?: M.AvailableModels;
  modelsLoading?: boolean;
  modelsReturn?: Page;
  selectedFinding?: { category: string; key: string };
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
  uncertain: boolean;
  scan?: M.Scan | null;
  confirmation?: Confirmation;
  terminals: M.TerminalUpdate[];
  chosenPath?: string;
  change?: M.ChangeSession;
  changeSeed?: M.ChangeSeed;
  chatDraft?: ChatDraft;
  changeHistory?: M.ChangeHistoryEntry[];
  changeReceipt?: M.ChangeMutation | null;
  features?: M.FeatureReport;
  // Saved history does not authorize generation notices in a new project session.
  featureGenerationRequested: boolean;
  instructionPreview?: M.InstructionPreview;
  projectInstructions?: M.InstructionPreview;
  analysisSetup?: M.AnalysisModels;
  analysisLimits?: M.Limits;
  analysisRefresh?: boolean;
  analysisSelectionDraft?: { selectionID: string; excluded: string[] };
  workflowModels?: M.ChangeWorkflowModels;
}
export interface ChatDraft {
  sessionID?: string;
  seed?: M.ChangeSeed;
  title: string;
  paths: string;
  message: string;
  tests: boolean;
}
export type AnalysisAssignments = Required<
  Pick<M.AnalysisModels, 'code' | 'performance' | 'security' | 'features'>
>;
export function analysisSetupModels(s: State): AnalysisAssignments {
  const setup = s.analysisSetup ||
    s.run?.plan?.models || { code: 'bug', review: 'analyze', features: 'analyze' };
  const resolve = (id: string) => s.availableModels?.defaults[id] || id;
  return {
    code: resolve(setup.code),
    performance: resolve(setup.performance || setup.review || 'analyze'),
    security: resolve(setup.security || setup.review || 'analyze'),
    features: resolve(setup.features),
  };
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
  uncertain: false,
  terminals: [],
  featureGenerationRequested: false,
});
const projectKey = (p?: M.ProjectIdentity) => (p ? `${p.project_id}:${p.project_revision}` : '');
const fileKey = (s: State) => `${projectKey(s.project)}:${s.file?.path}:${s.file?.content_hash}`;
export const activeRun = (run?: M.AnalysisRun | null) =>
  !!run && ['queued', 'running', 'pausing', 'canceling'].includes(run.status);
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
      'Start workflow',
    ].includes(label)
  );
}

export function changeChecksPassed(change: M.ChangeSession) {
  return (
    change.checks.length > 0 &&
    change.checks.every((check) => !check.required || check.state === 'passed')
  );
}
export function canAcceptChange(s: State) {
  const change = s.change;
  return (
    !!change &&
    !s.uncertain &&
    change.state === 'draft' &&
    change.freshness === 'current' &&
    projectKey(change) === projectKey(s.project) &&
    changeChecksPassed(change) &&
    change.changes.length > 0 &&
    workflowReviewable(change)
  );
}

export function canApplyChange(s: State) {
  return canAcceptChange(s) && s.change!.reviewed_hash === s.change!.hash;
}

export const activeChangeWorkflow = (change?: M.ChangeSession) =>
  !!change?.workflow && ['running', 'canceling'].includes(change.workflow.status);
export function workflowSeed(seed: M.ChangeSeed): M.ChangeSeed {
  const paths = [...seed.paths];
  const source = paths.find((path) => path.endsWith('.go') && !path.endsWith('_test.go'));
  if (source && paths.length < 8 && !paths.some((path) => path.endsWith('_test.go')))
    paths.push(source.replace(/\.go$/, '_test.go'));
  return { ...seed, paths };
}
export function workflowReviewable(change: M.ChangeSession) {
  const flow = change.workflow;
  return (
    !flow ||
    (flow.status === 'awaiting_human_review' &&
      flow.review?.verdict === 'approve' &&
      flow.review.proposal_hash === change.hash)
  );
}

export class Workspace {
  state = initial();
  private listeners = new Set<() => void>();
  private api = new API();
  private epoch = 0;
  private fileEpoch = 0;
  private operation = 0;
  private navigation = 0;
  private answer?: (accepted: boolean) => void;
  private timer?: ReturnType<typeof setTimeout>;
  private stopped = false;
  private explicitProjectAction = false;
  private fileRefresh: Promise<void> = Promise.resolve();
  private resultRequests: Record<string, number> = {};
  private featureRequest = 0;
  private instructionRequest = 0;
  private projectInstructionRequest = 0;
  private modelCatalogRequest = 0;
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
        this.set({ fileStale: true, preview: undefined });
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
      activeChangeWorkflow(this.state.change) ||
        activeRun(this.state.run) ||
        this.state.scan?.status === 'running'
        ? 1000
        : 5000,
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

      this.set({});
      await this.refreshProject();
    });
  }
  async refreshProject() {
    const identity = this.identity();
    await Promise.all([
      this.loadFeatures(),
      this.loadChangeHistory(),
      this.loadProjectInstructions(),
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
      const index = await this.api.post<M.ProjectIndex>(`${current}/reindex`, {
        project_revision: this.identity().project_revision,
      });
      this.epoch++;
      this.fileEpoch++;
      this.set({
        project: { ...this.state.project!, project_revision: index.project_revision },
        index,
        file: undefined,
        fileStale: true,
        change: this.state.change
          ? { ...this.state.change, freshness: 'stale', reviewed_hash: '' }
          : undefined,
        results: {},
        preview: undefined,
        uncertain: false,
        context: undefined,
        explanation: undefined,
        securityReport: undefined,
      });

      await this.refreshProject();
    });
  }
  async navigate(page: Page) {
    this.navigation++;
    this.set({
      page,
      error: '',
      modelsReturn:
        page === 'models' && this.state.page !== 'models'
          ? this.state.page
          : this.state.modelsReturn,
      selectedFinding:
        page === 'models' || this.state.page === 'models' ? this.state.selectedFinding : undefined,
    });
    if (['analysis', 'models'].includes(page) && !this.state.availableModels)
      await this.loadAvailableModels();
    if (page === 'editor') await this.refreshFile();
    if (['chat', 'changes', 'history'].includes(page)) await this.loadChangeHistory();
    if (page === 'features' || page === 'summary') await this.loadFeatures();
    if (page === 'instructions')
      await this.loadInstructions(this.state.instructionPreview?.path || 'AGENTS.md');
    if (['bugs', 'performance', 'security'].includes(page)) await this.loadResults(page);
    if (page === 'context') await this.inspectContext();
  }
  async loadAvailableModels() {
    const request = ++this.modelCatalogRequest;
    const epoch = this.epoch;
    this.set({ modelsLoading: true });
    await Promise.all([
      this.resource(
        'availableModels',
        () => this.api.get<M.AvailableModels>('/api/models/available'),
        (availableModels) => this.set({ availableModels }),
        () => request === this.modelCatalogRequest,
      ),
      this.resource(
        'models',
        () => this.api.get<M.ModelCatalog>('/api/models/current'),
        (models) => this.set({ models }),
        () => request === this.modelCatalogRequest,
      ),
    ]);
    if (epoch === this.epoch && request === this.modelCatalogRequest)
      this.set({ modelsLoading: false });
  }
  private async poll() {
    if (activeChangeWorkflow(this.state.change)) await this.refreshChangeWorkflow();
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
      this.set({ selection: updated, analysisSelectionDraft: undefined, preview: undefined });
    });
  }
  setAnalysisSetup(setup: M.AnalysisModels) {
    this.set({ analysisSetup: setup, preview: undefined, resume: undefined });
  }
  async previewAnalysis(
    mode: 'new' | 'repair' | 'resume' | 'stale',
    limits: M.Limits,
    refresh = false,
    models?: M.AnalysisModels,
  ) {
    await this.act('Prepare analysis', async () => {
      const epoch = this.epoch;
      const setup = this.state.analysisSetup ? { ...this.state.analysisSetup } : undefined;
      const selection_id = this.state.selection?.selection_id;
      const run = mode === 'resume' ? this.state.run : undefined;
      const preview = await this.api.post<M.AnalysisPreview>(`${current}/analysis/preview`, {
        ...this.identity(),
        scope: 'project',
        include_features: run ? !!run.plan.features : mode === 'new',
        models: run ? run.plan.models : models,
        limits: run?.plan.limits || limits,
        refresh: run?.plan.refresh || refresh,
        recover_incomplete: mode === 'repair',
        stale_only: mode === 'stale',
        ...(run ? { resume_run: run.identity } : {}),
      });
      if (
        epoch !== this.epoch ||
        selection_id !== this.state.selection?.selection_id ||
        JSON.stringify(setup) !== JSON.stringify(this.state.analysisSetup)
      )
        return;
      this.set({ preview, resume: run?.identity, page: 'analysis-preview' });
    });
  }
  async startAnalysis() {
    await this.act('Start analysis', async () => {
      const p = this.state.preview;
      if (!p || projectKey(p.identity) !== projectKey(this.state.project))
        throw new Error('Prepare a fresh analysis preview.');
      const epoch = this.epoch,
        operation = this.operation;
      const remote = p.providers.filter(
        (provider) =>
          provider.remote_confirmation_required &&
          (p.files.some((file) =>
            file.stages.some(
              (stage) =>
                stage.max_model_requests > 0 &&
                (stage.provider_id === provider.id || provider.stages.includes(stage.stage)),
            ),
          ) ||
            (p.features?.provider_id === provider.id && p.features.max_model_requests > 0)),
      );
      if (remote.length || p.security_review_intent_required) {
        const accepted = await this.confirm({
          title: this.state.resume ? 'Resume analysis' : 'Start analysis',
          message: remote.length
            ? `Send selected context to ${remote.length === 1 ? 'this remote provider' : 'these remote providers'}${p.security_review_intent_required ? ' and include AI Security review' : ''}?`
            : 'Include AI Security review for the selected files?',
          accept: 'Start',
          details: [
            `${p.files.length} selected files`,
            ...remote.map(
              (provider) => `${provider.model.model} · ${provider.model.provider_origin}`,
            ),
            ...(p.features?.max_model_requests
              ? ['Feature discovery includes saved goals and allowed project context.']
              : []),
          ],
        });
        if (!accepted) return;
      }
      if (
        epoch !== this.epoch ||
        operation !== this.operation ||
        this.state.preview?.preview_id !== p.preview_id
      )
        return;
      const confirmations = {
        provider_ids: remote.map((provider) => provider.id),
        security_review: p.security_review_intent_required,
      };
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
            include_features: !!p.features,
            models: p.models,
            retry_stale_failed: p.retry_stale_failed || false,
            stale_only: p.stale_only || false,
            recover_incomplete: p.recover_incomplete || false,
            confirmations,
          });
      this.set({
        run,
        preview: undefined,
        resume: undefined,
        results: {},
        page: 'analysis-run',
        featureGenerationRequested: this.state.featureGenerationRequested || !!p.features,
      });
      await this.loadFeatures();
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
  async openFile(path: string, symbol = '') {
    if (this.state.busy) return;
    if (this.state.file?.path === path) {
      this.set({ page: 'editor', symbol: symbol || this.state.symbol });
      await this.refreshFile();
      return;
    }
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
    this.fileEpoch++;
    this.set({
      symbol,
      explanation: undefined,
      context: undefined,
      impact: undefined,
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
        this.set({ fileStale: true });
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
    paths?: string[],
  ): Promise<boolean | null> {
    const models = await this.api.get<M.ModelCatalog>('/api/models/current');
    this.set({ models });
    const choices = models.scopes[scope]
      ? undefined
      : await this.api.get<M.AvailableModels>('/api/models/available');
    const model =
      models.scopes[scope] || choices?.models.find((choice) => choice.id === scope)?.model;
    if (!model)
      throw new Error('The selected model is unavailable. Refresh models and choose again.');
    if (!model.remote_provider && !explicit) return false;
    const accepted = await this.confirm({
      title,
      message: model.remote_provider
        ? 'Send the selected context to this provider?'
        : 'Request an AI Security review of this file?',
      accept: 'Continue',
      details: [
        ...(models.scopes[scope] ? [`Scope: ${scope}`] : []),
        model.model,
        model.provider_origin,
        ...(paths || [this.state.file?.path || 'Project context']),
      ],
    });
    if (!accepted) return null;
    const latest = choices
      ? (await this.api.get<M.AvailableModels>('/api/models/available')).models.find(
          (choice) => choice.id === scope,
        )?.model
      : (await this.api.get<M.ModelCatalog>('/api/models/current')).scopes[scope];
    if (JSON.stringify(latest) !== JSON.stringify(model))
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
        this.set({ explanation });
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
  private async trust(): Promise<boolean> {
    const params = {
      project_revision: this.identity().project_revision,
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
  seedChange(seed: M.ChangeSeed) {
    if (this.state.busy || activeChangeWorkflow(this.state.change)) return;
    this.set({
      changeSeed: seed,
      change: undefined,
      changeReceipt: undefined,
      page: 'chat',
      error: '',
    });
    void this.loadChangeHistory();
  }
  seedWorkflow(seed: M.ChangeSeed) {
    this.seedChange(workflowSeed(seed));
  }
  async loadFeatures() {
    if (!this.state.project) return;
    const request = ++this.featureRequest;
    await this.resource(
      'feature suggestions',
      () => this.api.get<M.FeatureReport>(`${current}/features`, this.identity()),
      (features) => this.set({ features }),
      () => request === this.featureRequest,
    );
  }
  async updateFeatures(goals: string) {
    await this.act('Save project goals', async () => {
      const report = this.state.features;
      if (!report) throw new Error('Load feature suggestions first.');
      const epoch = this.epoch,
        operation = this.operation;
      const request = ++this.featureRequest;
      try {
        const features = await this.api.post<M.FeatureReport>(`${current}/features/goals`, {
          ...this.identity(),
          expected_hash: report.hash,
          goals,
        });
        if (epoch === this.epoch && operation === this.operation && request === this.featureRequest)
          this.set({ features });
      } catch (error) {
        if (epoch === this.epoch && operation === this.operation) await this.loadFeatures();
        throw error;
      }
    });
  }
  async searchFeatures(origin: 'Features' | 'Analysis', goals: string) {
    await this.act('Search more feature suggestions', async () => {
      const report = this.state.features;
      if (!report) throw new Error('Load feature suggestions first.');
      const epoch = this.epoch,
        operation = this.operation;
      const request = ++this.featureRequest;
      const setup = analysisSetupModels(this.state);
      const profile = setup.features;
      const titles = report.suggestions.map((s) => s.title);
      const details = [`Origin: ${origin}`];
      if (this.state.models?.scopes[profile]) details.push(`Profile: ${profile}`);
      if (titles.length) {
        details.push(`Existing idea titles: ${titles.length}`);
        details.push(...titles.map((t) => `- ${t}`));
      }

      const confirmed = await this.confirmModel(
        profile,
        'Search more feature suggestions',
        false,
        details,
      );
      if (confirmed === null || epoch !== this.epoch || operation !== this.operation) return;
      this.set({ featureGenerationRequested: true });
      try {
        const payload: any = {
          ...this.identity(),
          expected_hash: report.hash,
          goals,
          confirm_remote_provider: confirmed,
          profile,
        };
        if (origin === 'Analysis') {
          payload.analysis_selection_id = this.state.selection?.selection_id;
        }
        const features = await this.api.post<M.FeatureReport>(
          `${current}/features/generate`,
          payload,
        );
        if (epoch === this.epoch && operation === this.operation && request === this.featureRequest)
          this.set({ features });
      } catch (error) {
        if (epoch === this.epoch && operation === this.operation) await this.loadFeatures();
        throw error;
      }
    });
  }
  async setFeatureStatus(id: string, status: M.FeatureSuggestion['status']) {
    await this.act('Save suggestion status', async () => {
      const report = this.state.features;
      if (!report) return;
      const epoch = this.epoch,
        operation = this.operation,
        request = ++this.featureRequest;
      const features = await this.api.request<M.FeatureReport>(
        'PATCH',
        `${current}/features/${encodeURIComponent(id)}`,
        {
          ...this.identity(),
          expected_hash: report.hash,
          goals: report.goals,
          status,
        },
      );
      if (epoch === this.epoch && operation === this.operation && request === this.featureRequest)
        this.set({ features });
    });
  }
  discussFeature(idea: M.FeatureSuggestion, workflow = false) {
    const seed = {
      title: idea.title,
      kind: 'feature',
      paths: idea.paths,
      acceptance_criteria: idea.acceptance_criteria,
      message: `${idea.title}\n\nBenefit: ${idea.benefit}\nContext: ${idea.evidence}\n\nAcceptance criteria:\n${idea.acceptance_criteria.map((item) => `- ${item}`).join('\n')}`,
    };
    if (workflow) this.seedWorkflow(seed);
    else this.seedChange(seed);
  }
  async loadInstructions(path: string) {
    if (!this.state.project) return false;
    const request = ++this.instructionRequest;
    this.set({ instructionPreview: undefined });
    let loaded = false;
    await this.resource(
      'project instructions',
      () =>
        this.api.get<M.InstructionPreview>(`${current}/instructions`, { ...this.identity(), path }),
      (instructionPreview) => {
        this.set({ instructionPreview });
        loaded = true;
      },
      () => request === this.instructionRequest,
    );
    return loaded;
  }
  async loadProjectInstructions() {
    if (!this.state.project) return;
    const request = ++this.projectInstructionRequest;
    const identity = this.identity();
    this.set({ projectInstructions: undefined });
    await this.resource(
      'default instructions',
      () =>
        this.api.get<M.InstructionPreview>(`${current}/instructions`, {
          ...identity,
          path: 'AGENTS.md',
        }),
      (projectInstructions) => {
        if (projectKey(projectInstructions) !== projectKey(identity))
          throw new Error('Project instructions belong to another revision. Refresh the project.');
        this.set({ projectInstructions });
      },
      () =>
        request === this.projectInstructionRequest &&
        projectKey(this.state.project) === projectKey(identity),
    );
  }
  async openProjectInstructions() {
    this.navigation++;
    this.set({ page: 'instructions', error: '' });
    await this.loadInstructions('AGENTS.md');
  }
  clearInstructionScope() {
    this.instructionRequest++;
    this.set({ instructionPreview: undefined });
  }
  async proposeInstructions(content: string) {
    await this.act('Preview instruction diff', async () => {
      if (activeChangeWorkflow(this.state.change))
        throw new Error('Finish or cancel the active workflow first.');
      const preview = this.state.instructionPreview;
      if (!preview || projectKey(preview) !== projectKey(this.state.project))
        throw new Error('Load the current instruction scope first.');
      const epoch = this.epoch,
        operation = this.operation;
      const change = await this.api.post<M.ChangeSession>(`${current}/instructions/proposal`, {
        ...this.identity(),
        kind: 'instructions',
        title: `Instructions for ${preview.path}`,
        paths: [preview.path],
        acceptance_criteria: [
          'Preserve existing rules and apply the requested instruction updates.',
        ],
        content,
      });
      if (
        epoch !== this.epoch ||
        operation !== this.operation ||
        this.state.instructionPreview !== preview
      )
        return;
      if (
        projectKey(change) !== projectKey(preview) ||
        change.targets.length !== 1 ||
        change.targets[0].path !== preview.path
      )
        throw new Error('Instruction proposal belongs to another scope.');
      this.set({ change, changeSeed: undefined, changeReceipt: undefined, page: 'chat' });
      await this.requestChangeChecks(false);
    });
  }
  newChange() {
    if (this.state.busy || activeChangeWorkflow(this.state.change)) return;
    this.set({
      change: undefined,
      changeSeed: undefined,
      changeReceipt: undefined,
      chatDraft: undefined,
    });
  }
  setChatDraft(chatDraft: ChatDraft) {
    this.set({ chatDraft });
  }
  async loadChangeHistory() {
    if (!this.state.project) return;
    const identity = this.identity();
    await Promise.all([
      this.resource(
        'change history',
        () => this.api.get<M.ChangeHistoryEntry[]>(`${current}/changes`, identity),
        (changeHistory) => this.set({ changeHistory }),
      ),
      this.resource(
        'change recovery',
        () => this.api.get<M.ChangeMutation | null>(`${current}/changes/recovery`, identity),
        (changeReceipt) => {
          const previous = this.state.changeReceipt;
          if (
            changeReceipt &&
            previous?.session_id === changeReceipt.session_id &&
            previous.hash === changeReceipt.hash
          )
            changeReceipt = {
              ...changeReceipt,
              warnings: [
                ...new Set([...(previous.warnings || []), ...(changeReceipt.warnings || [])]),
              ],
            };
          this.set({ changeReceipt });
        },
      ),
    ]);
    if (!this.state.change && projectKey(this.state.project) === projectKey(identity)) {
      const active = this.state.changeHistory?.find((entry) =>
        ['running', 'canceling'].includes(entry.workflow_status || ''),
      );
      if (active) await this.viewChange(active.id);
    }
  }
  private changeIdentity(change = this.state.change) {
    if (!change) throw new Error('Choose a change conversation.');
    return {
      project_id: change.project_id,
      project_revision: change.project_revision,
      revision: change.revision,
      hash: change.hash,
    };
  }
  setWorkflowModels(workflowModels: M.ChangeWorkflowModels) {
    this.set({ workflowModels });
  }
  async viewChange(id: string) {
    const epoch = this.epoch;
    const previous = this.state.change?.id,
      operation = this.operation;
    await this.resource(
      'change workflow',
      () =>
        this.api.get<M.ChangeSession>(
          `${current}/changes/${encodeURIComponent(id)}`,
          this.identity(),
        ),
      (change) => {
        if (change.id !== id || projectKey(change) !== projectKey(this.state.project))
          throw new Error('Workflow belongs to another project revision.');
        this.set({ change, changeSeed: undefined, page: 'chat' });
      },
      () =>
        epoch === this.epoch &&
        operation === this.operation &&
        previous === this.state.change?.id &&
        !activeChangeWorkflow(this.state.change),
    );
  }
  private async refreshChangeWorkflow() {
    const change = this.state.change;
    if (!change?.workflow) return;
    await this.resource(
      'change workflow',
      () =>
        this.api.get<M.ChangeSession>(
          `${current}/changes/${encodeURIComponent(change.id)}`,
          this.identity(),
        ),
      (next) => {
        if (
          next.id !== change.id ||
          projectKey(next) !== projectKey(change) ||
          next.workflow?.id !== change.workflow?.id
        )
          throw new Error('Workflow identity changed.');
        if (next.revision >= (this.state.change?.revision || 0)) this.set({ change: next });
      },
      () =>
        this.state.change?.id === change.id &&
        this.state.change?.workflow?.id === change.workflow?.id,
    );
  }
  private async confirmWorkflow(seed: M.ChangeSeed, models: M.ChangeWorkflowModels) {
    const catalog = await this.api.get<M.ModelCatalog>('/api/models/current');
    this.set({ models: catalog });
    const selected = [models.create, models.test, models.review];
    if (selected.some((profile) => !catalog.scopes[profile]))
      throw new Error('Choose a configured model for every workflow stage.');
    const remote = [
      ...new Set(selected.filter((profile) => catalog.scopes[profile].remote_provider)),
    ];
    if (remote.length || seed.kind === 'security') {
      const accepted = await this.confirm({
        title: seed.kind === 'security' ? 'Start security workflow?' : 'Send workflow context?',
        message:
          'Create, write and run tests, then request a model review for the captured files. Human review is required before Apply.',
        accept: 'Start workflow',
        details: [
          ...Object.entries(models).map(
            ([stage, profile]) =>
              `${stage}: ${catalog.scopes[profile].model} · ${profile} · ${catalog.scopes[profile].provider_origin}`,
          ),
          ...seed.paths,
        ],
      });
      if (!accepted) return null;
      const latest = await this.api.get<M.ModelCatalog>('/api/models/current');
      if (
        selected.some(
          (profile) =>
            JSON.stringify(catalog.scopes[profile]) !== JSON.stringify(latest.scopes[profile]),
        )
      )
        throw new Error('Provider configuration changed. Review the new destinations.');
    }
    return remote;
  }
  async readFixPreparation(): Promise<FixPreparationContext> {
    const epoch = this.epoch;
    const [models, trust] = await Promise.all([
      this.api.get<M.ModelCatalog>('/api/models/current'),
      this.api.get<M.ExecutionTrust>(`${current}/execution-trust`, {
        project_revision: this.identity().project_revision,
      }),
    ]);
    if (epoch !== this.epoch || projectKey(trust) !== projectKey(this.state.project))
      throw new Error('Project changed. Refresh fix preparation.');
    this.set({ models });
    return { models, trust };
  }
  private async checkedFixPreparation(profiles: string[], preparation: FixPreparationContext) {
    const epoch = this.epoch,
      operation = this.operation;
    const latest = await this.readFixPreparation();
    if (epoch !== this.epoch || operation !== this.operation) return null;
    if (
      projectKey(latest.trust) !== projectKey(preparation.trust) ||
      JSON.stringify(latest.trust.commands) !== JSON.stringify(preparation.trust.commands) ||
      profiles.some(
        (profile) =>
          !latest.models.scopes[profile] ||
          JSON.stringify(latest.models.scopes[profile]) !==
            JSON.stringify(preparation.models.scopes[profile]),
      )
    )
      throw new Error('Preparation changed. Review the current models and permissions.');
    return latest;
  }
  private async authorizeFix(profiles: string[], preparation: FixPreparationContext) {
    const epoch = this.epoch;
    const latest = await this.checkedFixPreparation(profiles, preparation);
    if (!latest) return null;
    const remote = [...new Set(profiles.filter((p) => latest.models.scopes[p].remote_provider))];
    if (!latest.trust.trusted) {
      if (epoch !== this.epoch) return null;
      await this.api.post(`${current}/execution-trust`, {
        project_revision: latest.trust.project_revision,
        confirm: true,
      });
    }
    return remote;
  }
  async startChangeWorkflow(
    seed: M.ChangeSeed,
    models: M.ChangeWorkflowModels,
    preparation?: FixPreparationContext,
  ) {
    await this.act('Start workflow', async () => {
      if (activeChangeWorkflow(this.state.change))
        throw new Error('Finish or cancel the active workflow.');
      if (!seed.paths.some((path) => path.endsWith('_test.go')))
        throw new Error('Include a Go test path (_test.go) in Files to change.');
      const epoch = this.epoch,
        operation = this.operation;
      const profiles = preparation
        ? await this.authorizeFix(Object.values(models), preparation)
        : await this.confirmWorkflow(seed, models);
      if (profiles === null || (!preparation && !(await this.trust()))) return;
      if (epoch !== this.epoch || operation !== this.operation) return;
      let change = this.state.change;
      if (!change) {
        change = await this.api.post<M.ChangeSession>(`${current}/changes`, {
          ...this.identity(),
          kind: seed.kind,
          title: seed.title,
          paths: seed.paths,
          acceptance_criteria: seed.acceptance_criteria,
        });
        if (epoch !== this.epoch || operation !== this.operation) return;
        if (projectKey(change) !== projectKey(this.state.project))
          throw new Error('Conversation belongs to another project revision.');
        this.set({ change, page: 'chat' });
      }
      const id = change.id;
      try {
        const next = await this.api.post<M.ChangeSession>(
          `${current}/changes/${encodeURIComponent(id)}/workflow`,
          {
            ...this.changeIdentity(change),
            message: seed.message,
            models,
            confirmed_profiles: profiles,
            confirm_security: seed.kind === 'security',
          },
        );
        if (epoch !== this.epoch || operation !== this.operation || this.state.change?.id !== id)
          return;
        if (next.id !== id || projectKey(next) !== projectKey(change) || !next.workflow)
          throw new Error('Workflow identity does not match this conversation.');
        this.set({ change: next, changeReceipt: undefined, workflowModels: models, page: 'chat' });
      } catch (error) {
        if (epoch === this.epoch && this.state.change?.id === id) await this.viewChange(id);
        throw error;
      }
    });
  }
  async cancelChangeWorkflow() {
    await this.act('Cancel workflow', async () => {
      const change = this.state.change;
      if (!activeChangeWorkflow(change) || !change?.workflow) return;
      const epoch = this.epoch;
      const next = await this.api.post<M.ChangeSession>(
        `${current}/changes/${encodeURIComponent(change.id)}/workflow/cancel`,
        {
          ...this.identity(),
          workflow_id: change.workflow.id,
        },
      );
      if (epoch === this.epoch && this.state.change?.workflow?.id === change.workflow.id)
        this.set({ change: next });
    });
  }
  private async requestChangeMessage(
    message: string,
    repair = false,
    profile = 'function',
    preparation?: FixPreparationContext,
  ) {
    const change = this.state.change;
    if (!change || change.freshness !== 'current' || change.state !== 'draft')
      throw new Error('Start a fresh conversation for this source.');
    const identity = this.changeIdentity(change);
    const epoch = this.epoch;
    const operation = this.operation;
    const confirmed = preparation
      ? ((await this.authorizeFix([profile], preparation))?.includes(profile) ?? null)
      : await this.confirmModel(
          profile,
          repair ? 'Repair proposal' : 'Prepare proposal',
          change.kind === 'security',
          change.targets.map((target) => target.path),
        );
    if (confirmed === null) return false;
    if (epoch !== this.epoch || operation !== this.operation) return false;
    const next = await this.api.post<M.ChangeSession>(
      `${current}/changes/${encodeURIComponent(change.id)}/messages`,
      { ...identity, message, repair, profile, confirm_remote_provider: confirmed },
    );
    if (
      epoch !== this.epoch ||
      operation !== this.operation ||
      this.state.change?.id !== change.id ||
      this.state.change.hash !== change.hash
    )
      return false;
    if (
      projectKey(next) !== projectKey(change) ||
      next.id !== change.id ||
      next.revision <= change.revision
    )
      throw new Error('Proposal identity does not match this conversation.');
    this.set({ change: next, changeReceipt: undefined });
    return true;
  }
  private async requestChangeChecks(tests: boolean) {
    const change = this.state.change;
    if (!change) return false;
    const identity = this.changeIdentity(change);
    const epoch = this.epoch;
    const operation = this.operation;
    if (
      (tests || change.check_options?.run_tests || change.check_options?.run_lint) &&
      !(await this.trust())
    )
      return false;
    this.set({ change: { ...change, checks: [], reviewed_hash: '' } });
    const checked = await this.api.post<M.ChangeSession>(
      `${current}/changes/${encodeURIComponent(change.id)}/checks`,
      { ...identity, run_tests: tests },
    );
    if (
      epoch !== this.epoch ||
      operation !== this.operation ||
      this.state.change?.id !== change.id ||
      this.state.change.hash !== change.hash
    )
      return false;
    if (checked.hash !== change.hash || checked.revision !== change.revision)
      throw new Error('Check evidence does not match this proposal.');
    this.set({ change: checked });
    return true;
  }
  async prepareChange(
    seed: M.ChangeSeed,
    tests: boolean,
    profile = 'function',
    preparation?: FixPreparationContext,
  ) {
    await this.act('Prepare change', async () => {
      if (!seed.message.trim()) throw new Error('Describe the change.');
      if (preparation && !(await this.authorizeFix([profile], preparation))) return;
      if (!this.state.change) {
        const identity = this.identity();
        const epoch = this.epoch;
        const operation = this.operation;
        const change = await this.api.post<M.ChangeSession>(`${current}/changes`, {
          ...identity,
          kind: seed.kind,
          title: seed.title,
          paths: seed.paths,
          acceptance_criteria: seed.acceptance_criteria,
        });
        if (epoch !== this.epoch || operation !== this.operation) return;
        if (projectKey(change) !== projectKey(this.state.project))
          throw new Error('Conversation belongs to another project revision.');
        this.set({ change, page: 'chat' });
      }
      if (!(await this.requestChangeMessage(seed.message, false, profile, preparation))) return;
      if (!(await this.requestChangeChecks(tests))) return;
      for (
        let attempt = 0;
        attempt < 3 &&
        this.state.change &&
        !changeChecksPassed(this.state.change) &&
        this.state.change.repair_attempts < 3;
        attempt++
      ) {
        if (
          this.state.change.checks.some(
            (check) => check.name === 'existing test baseline' && check.state !== 'passed',
          )
        )
          break;
        if (!(await this.requestChangeMessage('Repair failed checks.', true, profile, preparation)))
          return;
        if (!(await this.requestChangeChecks(tests))) return;
      }
      this.set({
        notice:
          this.state.change && changeChecksPassed(this.state.change)
            ? 'Changes are ready for your review.'
            : 'Checks need attention. Review the diagnostics before retrying.',
      });
    });
  }
  async checkChange(tests: boolean) {
    await this.act('Check proposal', async () => {
      await this.requestChangeChecks(tests);
    });
  }
  async repairChange() {
    await this.act('Repair proposal', async () => {
      if (await this.requestChangeMessage('Repair failed checks.', true))
        await this.requestChangeChecks(!!this.state.change?.check_options?.run_tests);
    });
  }
  async resumeChange(id: string) {
    await this.act('Resume conversation', async () => {
      if (activeChangeWorkflow(this.state.change))
        throw new Error('Finish or cancel the active workflow first.');
      const epoch = this.epoch;
      const operation = this.operation;
      const change = await this.api.post<M.ChangeSession>(
        `${current}/changes/${encodeURIComponent(id)}/resume`,
        { ...this.identity(), revision: 0, hash: '' },
      );
      if (epoch !== this.epoch || operation !== this.operation) return;
      if (change.project_id !== this.state.project?.project_id)
        throw new Error('History belongs to another project.');
      this.set({ change, changeSeed: undefined, page: 'chat' });
    });
  }
  async acceptChange() {
    await this.act('Apply change', async () => {
      const change = this.state.change;
      if (!change || !['chat', 'changes'].includes(this.state.page) || !canAcceptChange(this.state))
        throw new Error('The displayed proposal needs current passing checks and model review.');
      const epoch = this.epoch;
      const operation = this.operation;
      const navigation = this.navigation;
      const identity = this.changeIdentity(change);
      // One explicit acceptance records review and applies only the displayed revision.
      const reviewed = await this.api.post<M.ChangeSession>(
        `${current}/changes/${encodeURIComponent(change.id)}/review`,
        identity,
      );
      if (
        epoch !== this.epoch ||
        operation !== this.operation ||
        navigation !== this.navigation ||
        !['chat', 'changes'].includes(this.state.page) ||
        this.state.change?.id !== change.id ||
        this.state.change.revision !== change.revision ||
        this.state.change.hash !== change.hash
      )
        return;
      if (
        reviewed.id !== change.id ||
        reviewed.revision !== change.revision ||
        reviewed.hash !== change.hash ||
        projectKey(reviewed) !== projectKey(change) ||
        reviewed.reviewed_hash !== change.hash ||
        !canAcceptChange(this.state)
      )
        throw new Error(
          'The reviewed proposal changed. Inspect the current diff before accepting.',
        );
      this.set({ change: reviewed });
      if (!canApplyChange(this.state)) throw new Error('The proposal is no longer ready to apply.');
      const receipt = await this.writeChangeSource(change.id, 'apply', identity);
      await this.acceptChangeMutation(receipt);
    });
  }
  async undoChange() {
    await this.act('Undo change', async () => {
      const receipt = this.state.changeReceipt;
      if (!receipt?.undo_available || this.state.uncertain)
        throw new Error('Refresh the project and recovery status before Undo.');
      if (
        !(await this.confirm({
          title: 'Restore the previous files?',
          message:
            receipt.state === 'applied'
              ? 'Undo the latest unchanged proposal.'
              : 'Recover the interrupted grouped change.',
          accept: 'Undo proposal',
          destructive: true,
        }))
      )
        return;
      const result = await this.writeChangeSource(receipt.session_id, 'undo', {
        ...this.identity(),
        revision: this.state.change?.revision || 0,
        hash: receipt.hash,
      });
      await this.acceptChangeMutation(result);
    });
  }
  async verifyChange() {
    await this.act('Verify applied change', async () => {
      const receipt = this.state.changeReceipt;
      const change = this.state.change;
      if (
        !receipt ||
        receipt.state !== 'applied' ||
        !change ||
        change.id !== receipt.session_id ||
        this.state.uncertain
      )
        throw new Error('Resume the applied conversation and refresh its recovery status first.');
      const epoch = this.epoch,
        operation = this.operation;
      if (change.changes.some((edit) => edit.path.endsWith('.go')) && !(await this.trust())) return;
      if (epoch !== this.epoch || operation !== this.operation) return;
      const verification = await this.api.post<M.ChangeVerification>(
        `${current}/changes/${encodeURIComponent(change.id)}/verify`,
        {
          ...this.identity(),
          revision: change.revision,
          hash: receipt.hash,
        },
      );
      if (
        epoch !== this.epoch ||
        operation !== this.operation ||
        this.state.changeReceipt?.hash !== receipt.hash
      )
        return;
      if (
        verification.session_id !== change.id ||
        verification.proposal_hash !== receipt.hash ||
        projectKey(verification) !== projectKey(this.state.project)
      )
        throw new Error('Verification belongs to another applied change.');
      this.set({ changeReceipt: { ...receipt, verification } });
    });
  }
  async reanalyzeChange() {
    await this.act('Reanalyze changed files', async () => {
      const change = this.state.change,
        receipt = this.state.changeReceipt;
      if (
        !change ||
        !receipt ||
        receipt.state !== 'applied' ||
        receipt.session_id !== change.id ||
        this.state.uncertain
      )
        throw new Error('Choose the latest applied conversation first.');
      const identity = this.identity(),
        epoch = this.epoch,
        operation = this.operation;
      const paths = change.changes
        .filter((edit) => edit.path.endsWith('.go'))
        .map((edit) => edit.path);
      const confirmed = await this.confirmModel('bug', 'Reanalyze changed files', false, paths);
      if (confirmed === null || epoch !== this.epoch || operation !== this.operation) return;
      let completed = 0;
      for (const path of paths) {
        if (epoch !== this.epoch || operation !== this.operation) return;
        const analysis = await this.api.post<M.FileAnalysis>(`${current}/files/analysis`, {
          path,
          project_revision: identity.project_revision,
          refresh: true,
          confirm_remote_provider: confirmed,
        });
        if (epoch !== this.epoch || operation !== this.operation) return;
        if (
          projectKey(analysis) !== projectKey(identity) ||
          analysis.path !== path ||
          analysis.content_hash !== change.changes.find((edit) => edit.path === path)?.hash
        )
          throw new Error('Reanalysis belongs to an earlier source revision.');
        if (!['success', 'fresh'].includes(analysis.status))
          throw new Error(analysis.failure || `Reanalysis is ${analysis.status} for ${path}.`);
        completed++;
      }
      await this.refreshProject();
      if (epoch === this.epoch && operation === this.operation)
        this.set({
          notice: `Reanalyzed ${completed} changed files. Original findings retain their status.`,
        });
    });
  }
  private async writeChangeSource(id: string, action: 'apply' | 'undo', identity: object) {
    try {
      return await this.api.post<M.ChangeMutation>(
        `${current}/changes/${encodeURIComponent(id)}/${action}`,
        { ...identity, confirm: true },
      );
    } catch (error) {
      if (!(error instanceof ApiError))
        this.set({
          uncertain: true,
          change: this.state.change ? { ...this.state.change, reviewed_hash: '' } : undefined,
        });
      throw error;
    }
  }
  private async acceptChangeMutation(receipt: M.ChangeMutation) {
    if (receipt.project_id !== this.state.project?.project_id)
      throw new Error('Mutation belongs to another project.');
    this.epoch++;
    this.fileEpoch++;
    this.set({
      project: { ...this.state.project!, project_revision: receipt.project_revision },
      index: receipt.index,
      changeReceipt: receipt,
      change: this.state.change
        ? { ...this.state.change, state: receipt.state, reviewed_hash: '' }
        : undefined,
      fileStale: true,
      results: {},
      preview: undefined,
      uncertain: !receipt.index,
    });
    await this.refreshProject();
    await this.loadChangeHistory();
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
      preview: undefined,
    });
  }
}
export const workspace = new Workspace();
