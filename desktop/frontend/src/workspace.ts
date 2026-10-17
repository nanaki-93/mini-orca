import { API, ApiError, current, errorMessage, native } from './api';
import type * as M from './models';

export type Page =
  | 'features'
  | 'instructions'
  | 'chat'
  | 'changes'
  | 'history'
  | 'git'
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
  | 'finding'
  | 'editor'
  | 'context'
  | 'terminal'
  | 'search'
  | 'models'
  | 'diagrams'
  | 'docs'
  | 'agents';
export type FindingCategory = 'bugs' | 'performance' | 'security';
export interface FindingListView {
  query: string;
  severity: string;
  page: number;
}
export function findingCategory(s: Pick<State, 'page' | 'selectedFinding'>) {
  if (s.page === 'finding') return s.selectedFinding?.category;
  if (s.page === 'bugs' || s.page === 'performance' || s.page === 'security') return s.page;
}
export interface Confirmation {
  title: string;
  message: string;
  accept: string;
  details?: string[];
  destructive?: boolean;
}
export interface FixPreparationContext {
  models: M.ModelCatalog;
  availableModels?: M.AvailableModels;
  trust: M.ExecutionTrust;
  repository: M.GitRepository;
}
export interface State {
  implementations: Workspace[];
  selectedAgentPath?: string;
  agentDetailOpen?: boolean;
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
  selectedFinding?: { category: FindingCategory; key: string };
  findingViews: Partial<Record<FindingCategory, FindingListView>>;
  findings?: M.Finding[];
  completedFindings?: M.FindingCompletion[];
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
  repository?: M.GitRepository;
  gitWorkspace?: M.GitWorkspace;
  gitPreview?: M.GitMergePreview;
  gitResult?: M.GitMutation;
  securityReport?: M.SecurityReport;
  uncertain: boolean;
  scan?: M.Scan | null;
  confirmation?: Confirmation;
  terminals: M.TerminalUpdate[];
  chosenPath?: string;
  change?: M.ChangeSession;
  changeSeed?: M.ChangeSeed;
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
  modelPresets?: ModelPreset[];
  workspaceTrust?: boolean;
  bootstrapExec?: M.BootstrapExecResult;
  instructionsTab?: 'rules' | 'guidelines' | 'effective' | 'bootstrap';
}
export interface ModelPreset {
  id: string;
  name: string;
  description: string;
  builtIn?: boolean;
  analysis: M.AnalysisModels;
  workflow: M.ChangeWorkflowModels;
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
export function workflowSetupModels(s: State): M.ChangeWorkflowModels {
  const setup = s.workflowModels ||
    s.change?.workflow?.models || { create: 'function', test: 'bug', review: 'analyze' };
  const resolve = (id: string) => s.availableModels?.defaults[id] || id;
  return {
    create: resolve(setup.create),
    test: resolve(setup.test),
    review: resolve(setup.review),
  };
}
function builtInModelPresets(s: State): ModelPreset[] {
  const resolve = (id: string) => s.availableModels?.defaults[id] || id;
  const fastModel =
    s.availableModels?.models.find((m) => m.location === 'local' || !m.model.remote_provider)?.id ||
    resolve('function');
  const deepModel = resolve('analyze');
  const bugModel = resolve('bug');
  const fnModel = resolve('function');
  return [
    {
      id: 'preset-fast-local',
      name: 'Fast Local / Speed',
      description: 'Routes all analysis and workflow stages to your fastest or local model.',
      builtIn: true,
      analysis: {
        code: fastModel,
        performance: fastModel,
        security: fastModel,
        features: fastModel,
      },
      workflow: {
        create: fastModel,
        test: fastModel,
        review: fastModel,
      },
    },
    {
      id: 'preset-deep-audit',
      name: 'Deep Audit',
      description:
        'Routes every analysis, creation, test, and review stage to the deep analysis model.',
      builtIn: true,
      analysis: {
        code: deepModel,
        performance: deepModel,
        security: deepModel,
        features: deepModel,
      },
      workflow: {
        create: deepModel,
        test: deepModel,
        review: deepModel,
      },
    },
    {
      id: 'preset-hybrid-roles',
      name: 'Hybrid Multi-Role',
      description:
        'Balanced routing across specialized creation, bug/security, and deep review profiles.',
      builtIn: true,
      analysis: {
        code: bugModel,
        performance: deepModel,
        security: bugModel,
        features: deepModel,
      },
      workflow: {
        create: fnModel,
        test: bugModel,
        review: deepModel,
      },
    },
  ];
}
export function allModelPresets(s: State): ModelPreset[] {
  return [...builtInModelPresets(s), ...(s.modelPresets || [])];
}
export function isModelPresetActive(s: State, preset: ModelPreset): boolean {
  const currentAnalysis = analysisSetupModels(s);
  const currentWorkflow = workflowSetupModels(s);
  const targetAnalysis = preset.analysis;
  const targetWorkflow = preset.workflow;
  return (
    currentAnalysis.code === targetAnalysis.code &&
    currentAnalysis.performance ===
      (targetAnalysis.performance || targetAnalysis.review || targetAnalysis.code) &&
    currentAnalysis.security ===
      (targetAnalysis.security || targetAnalysis.review || targetAnalysis.code) &&
    currentAnalysis.features === targetAnalysis.features &&
    currentWorkflow.create === targetWorkflow.create &&
    currentWorkflow.test === targetWorkflow.test &&
    currentWorkflow.review === targetWorkflow.review
  );
}
export function effectiveWorkflowModel(
  context: { models?: M.ModelCatalog; availableModels?: M.AvailableModels },
  profile: string,
): M.Model | undefined {
  const resolved = context.availableModels?.defaults[profile] || profile;
  return (
    context.models?.scopes[profile] ||
    context.availableModels?.models.find(
      (choice) => choice.id === profile || choice.id === resolved,
    )?.model
  );
}
export interface BootstrapChecklistPillar {
  id: 'entrypoint' | 'toolchain' | 'contract' | 'precommit';
  step: number;
  title: string;
  shortTitle: string;
  targetPath: string;
  detectedPath?: string;
  ready: boolean;
  statusLabel: string;
  summary: string;
  templateContent: string;
}

function stackBootstrapTemplates(projectType: string, projectName: string) {
  const kind = (projectType || 'go').toLowerCase();
  const isNode = ['typescript', 'javascript', 'node', 'react', 'nextjs'].some((k) =>
    kind.includes(k),
  );
  const isPython = kind.includes('python');
  const isJvm = ['kotlin', 'java', 'gradle', 'maven'].some((k) => kind.includes(k));

  const justfile = isNode
    ? `# Standardized repository entrypoint for ${projectName}
set shell := ["sh", "-cu"]

default:
    @just --list

# Install dependencies and pre-commit hooks
setup:
    npm install
    command -v lefthook >/dev/null 2>&1 && lefthook install || true

# Start local development server
dev:
    npm run dev

# Run the automated test suite
test:
    npm test

# Run a single named test
test-one name:
    npx vitest run -t "{{name}}"

# Run linters, typechecks, and tests
check:
    npm run check
    npm test
`
    : isPython
      ? `# Standardized repository entrypoint for ${projectName}
set shell := ["sh", "-cu"]

default:
    @just --list

# Install dependencies and pre-commit hooks
setup:
    python3 -m pip install -e .
    command -v lefthook >/dev/null 2>&1 && lefthook install || true

# Start local development workflow
dev:
    python3 -m unittest

# Run the automated test suite
test:
    python3 -m unittest discover -v

# Run a single test module or case
test-one name:
    python3 -m unittest {{name}} -v

# Run static checks and tests
check:
    python3 -m compileall -q .
    python3 -m unittest discover
`
      : isJvm
        ? `# Standardized repository entrypoint for ${projectName}
set shell := ["sh", "-cu"]

default:
    @just --list

# Bootstrap dependencies and pre-commit hooks
setup:
    ./gradlew classes
    command -v lefthook >/dev/null 2>&1 && lefthook install || true

# Start local development server
dev:
    ./gradlew run

# Run the automated test suite
test:
    ./gradlew test

# Run a single test class or method
test-one name:
    ./gradlew test --tests "*{{name}}*"

# Run linters (ktlint/check) and tests
check:
    ./gradlew check
`
        : `# Standardized repository entrypoint for ${projectName}
set shell := ["sh", "-cu"]

default:
    @just --list

# Bootstrap dependencies and pre-commit hooks
setup:
    go mod download
    command -v lefthook >/dev/null 2>&1 && lefthook install || true

# Start local development mode
dev:
    go test ./...

# Run the automated test suite with race detector
test:
    go test -race ./...

# Run a single named Go test
test-one name:
    go test ./... -run "{{name}}" -v

# Run formatting, static analysis, and tests
check:
    test -z "$(gofmt -l .)"
    go vet ./...
    go test -race ./...
`;

  const miseToml = isNode
    ? `# Pinned runtime and linter toolchain for ${projectName}
[tools]
node = "22"
just = "1.36.0"
lefthook = "1.10.10"
gitleaks = "8.24.0"
`
    : isPython
      ? `# Pinned runtime and linter toolchain for ${projectName}
[tools]
python = "3.12"
just = "1.36.0"
lefthook = "1.10.10"
gitleaks = "8.24.0"
`
      : isJvm
        ? `# Pinned runtime and linter toolchain for ${projectName}
[tools]
java = "temurin-21"
kotlin = "2.1.0"
ktlint = "1.5.0"
just = "1.36.0"
lefthook = "1.10.10"
gitleaks = "8.24.0"
`
        : `# Pinned runtime and linter toolchain for ${projectName}
[tools]
go = "1.22"
golangci-lint = "1.63.4"
just = "1.36.0"
lefthook = "1.10.10"
gitleaks = "8.24.0"
`;

  const agentsMd = `# ${projectName} Agent Contract

Keep this contract under 150 lines so human developers and coding agents share identical operational rules.

## 1. Quick Commands & Single-Test Execution

- Bootstrap & entrypoints (\`justfile\`): \`just setup\`, \`just dev\`, \`just test\`, \`just check\`.
- Run a single test: \`just test-one TestName\`.
- Always run \`just check\` before committing changes.

## 2. Architecture Boundaries

- Entrypoints and HTTP/UI handlers must never import database or storage packages directly; always go through service interfaces.
- Domain and indexing packages must remain pure and never import transport or presentation layers.
- Keep diffs surgical and scoped to the requested change, its callers, and its regression tests.

## 3. Error Handling & Concurrency Conventions

- Handle all errors explicitly with actionable context (e.g., \`fmt.Errorf("action: %w", err)\`). Never swallow errors or silently return fallback values.
- Propagate cancellation (\`context.Context\` / \`AbortSignal\`) across all I/O, child processes, and model calls.
- Guard shared state with explicit synchronization and ensure no background workers leak after a task or test finishes.
`;

  const lefthookYml = `# Fast parallel pre-commit gates (<2s on staged files)
pre-commit:
  parallel: true
  commands:
    whitespace:
      glob: "*.{go,ts,tsx,js,py,kt,java,md,yml,yaml,json,toml}"
      run: git diff --cached --check -- {staged_files}
    gitleaks:
      run: |
        if command -v gitleaks >/dev/null 2>&1; then
          gitleaks protect --staged --no-banner --redact
        fi
`;

  return { justfile, miseToml, agentsMd, lefthookYml };
}

export function evaluateBootstrapChecklist(s: State): BootstrapChecklistPillar[] {
  const files = s.index?.files || [];
  const projectName = s.project?.name || 'Project';
  const projectType = s.project?.type || 'go';
  const templates = stackBootstrapTemplates(projectType, projectName);

  const findFile = (candidates: string[]) =>
    files.find((f) => candidates.some((c) => f.path.toLowerCase() === c.toLowerCase()));

  const entrypointFile = findFile(['justfile', 'Taskfile.yml', 'Taskfile.yaml']);
  const toolchainFile = findFile(['.mise.toml', 'mise.toml', 'devbox.json']);
  const contractFile = findFile(['AGENTS.md', '.cursorrules']);
  const precommitFile = findFile(['lefthook.yml', '.lefthook.yml', 'lefthook.yaml']);

  const previewIsRootAgents =
    s.instructionPreview?.path === 'AGENTS.md' && s.instructionPreview.exists;
  const projectInstIsRootAgents =
    s.projectInstructions?.path === 'AGENTS.md' && s.projectInstructions.exists;
  const hasContract = !!contractFile || !!previewIsRootAgents || !!projectInstIsRootAgents;
  const contractLines = previewIsRootAgents
    ? s.instructionPreview!.existing_content.split('\n').length
    : projectInstIsRootAgents
      ? s.projectInstructions!.existing_content.split('\n').length
      : contractFile?.line_count || 0;
  const contractReady = hasContract && contractLines > 0 && contractLines <= 150;

  return [
    {
      id: 'entrypoint',
      step: 1,
      title: '1. Standardize the Repo Entrypoint',
      shortTitle: 'Repo Entrypoint',
      targetPath: entrypointFile?.path || 'justfile',
      detectedPath: entrypointFile?.path,
      ready: !!entrypointFile,
      statusLabel: entrypointFile ? `Ready (${entrypointFile.path})` : 'Missing',
      summary:
        'Add a justfile or Taskfile.yml with setup, dev, test, and check targets so Step 2 verification runs your exact checks.',
      templateContent: templates.justfile,
    },
    {
      id: 'toolchain',
      step: 2,
      title: '2. Pin the Toolchain',
      shortTitle: 'Pinned Toolchain',
      targetPath: toolchainFile?.path || '.mise.toml',
      detectedPath: toolchainFile?.path,
      ready: !!toolchainFile,
      statusLabel: toolchainFile ? `Ready (${toolchainFile.path})` : 'Missing',
      summary:
        'Add a .mise.toml or devbox.json pinning exact language runtimes and linters (golangci-lint, ktlint, gitleaks, lefthook).',
      templateContent: templates.miseToml,
    },
    {
      id: 'contract',
      step: 3,
      title: '3. Write the Agent Contract (<150 lines)',
      shortTitle: 'Agent Contract (<150L)',
      targetPath: 'AGENTS.md',
      detectedPath: hasContract ? contractFile?.path || 'AGENTS.md' : undefined,
      ready: contractReady,
      statusLabel: !hasContract
        ? 'Missing'
        : contractReady
          ? `Ready (${contractLines} / 150 lines)`
          : `Over limit (${contractLines} / 150 lines)`,
      summary:
        'Keep root AGENTS.md under 150 lines with exact build/single-test commands, architecture boundaries, and error/concurrency rules.',
      templateContent: templates.agentsMd,
    },
    {
      id: 'precommit',
      step: 4,
      title: '4. Enforce Fast Pre-Commit Gates',
      shortTitle: 'Pre-Commit Gates',
      targetPath: precommitFile?.path || 'lefthook.yml',
      detectedPath: precommitFile?.path,
      ready: !!precommitFile,
      statusLabel: precommitFile ? `Ready (${precommitFile.path})` : 'Missing',
      summary:
        'Configure lefthook.yml to run formatters, linters, and secret scanners (gitleaks) in parallel on staged files (<2s).',
      templateContent: templates.lefthookYml,
    },
  ];
}

const changePage = (kind: string): Page =>
  ['fix', 'performance', 'security'].includes(kind) ? 'changes' : 'chat';
function readStoredJson<T>(key: string): T | undefined {
  if (typeof localStorage === 'undefined') return undefined;
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return undefined;
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? (parsed as T) : undefined;
  } catch {
    return undefined;
  }
}
function readStoredBool(key: string): boolean | undefined {
  if (typeof localStorage === 'undefined') return undefined;
  const raw = localStorage.getItem(key);
  if (raw === 'true') return true;
  if (raw === 'false') return false;
  return undefined;
}
function writeStoredJson(key: string, value: unknown) {
  if (typeof localStorage === 'undefined') return;
  try {
    if (value === undefined) localStorage.removeItem(key);
    else localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Ignore storage quota or restricted environment errors.
  }
}
const initial = (): State => ({
  implementations: [],
  page: 'welcome',
  busy: '',
  error: '',
  notice: '',
  connected: false,
  version: '',
  results: {},
  findingViews: {},
  resourceErrors: {},
  symbols: [],
  symbol: '',
  fileStale: true,
  uncertain: false,
  terminals: [],
  featureGenerationRequested: false,
  analysisSetup: readStoredJson<M.AnalysisModels>('mini-orca:analysis-models'),
  workflowModels: readStoredJson<M.ChangeWorkflowModels>('mini-orca:workflow-models'),
  analysisLimits: readStoredJson<M.Limits>('mini-orca:analysis-limits'),
  analysisRefresh: readStoredBool('mini-orca:analysis-refresh'),
  modelPresets: readStoredJson<ModelPreset[]>('mini-orca:model-presets') || [],
  workspaceTrust:
    typeof localStorage !== 'undefined' &&
    localStorage.getItem('mini-orca:workspace-trust') === 'true',
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
      'Retry commit',
      'Update Git workspace',
      'Discard worktree',
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

function canApplyChange(s: State) {
  return canAcceptChange(s) && s.change!.reviewed_hash === s.change!.hash;
}

export const activeChangeWorkflow = (change?: M.ChangeSession) =>
  !!change?.workflow && ['running', 'canceling'].includes(change.workflow.status);

export function isTestFilePath(path: string): boolean {
  const lower = path.replace(/\\/g, '/').toLowerCase();
  const parts = lower.split('/');
  const base = parts[parts.length - 1] || '';
  if (
    lower.endsWith('_test.go') ||
    lower.endsWith('_test.py') ||
    (base.startsWith('test_') && base.endsWith('.py'))
  ) {
    return true;
  }
  for (const suffix of [
    '.test.ts',
    '.spec.ts',
    '.test.tsx',
    '.spec.tsx',
    '.test.js',
    '.spec.js',
    '.test.jsx',
    '.spec.jsx',
    '.test.mjs',
    '.spec.mjs',
    '.test.cjs',
    '.spec.cjs',
    'test.kt',
    'tests.kt',
    'spec.kt',
    'test.java',
    'tests.java',
    '_test.rs',
    '_test.rb',
    '_spec.rb',
    'test.php',
    'test.cs',
    'tests.cs',
    'test.swift',
    'tests.swift',
    '_test.c',
    '_test.cpp',
    '_test.cc',
  ]) {
    if (base.endsWith(suffix)) return true;
  }
  return parts
    .slice(0, -1)
    .some((part) => ['test', 'tests', '__tests__', 'spec', 'specs'].includes(part));
}

function inferTestPath(path: string): string | undefined {
  if (isTestFilePath(path)) return undefined;
  if (path.endsWith('.go')) return path.replace(/\.go$/, '_test.go');
  if (/\.(ts|tsx|js|jsx|mjs|cjs)$/.test(path))
    return path.replace(/\.(ts|tsx|js|jsx|mjs|cjs)$/, '.test.$1');
  if (path.endsWith('.py')) return path.replace(/\.py$/, '_test.py');
  if (path.endsWith('.rs')) return path.replace(/\.rs$/, '_test.rs');
  if (path.endsWith('.kt')) return path.replace(/\.kt$/, 'Test.kt');
  if (path.endsWith('.java')) return path.replace(/\.java$/, 'Test.java');
  if (path.endsWith('.rb')) return path.replace(/\.rb$/, '_spec.rb');
  if (path.endsWith('.php')) return path.replace(/\.php$/, 'Test.php');
  if (path.endsWith('.cs')) return path.replace(/\.cs$/, 'Tests.cs');
  if (path.endsWith('.swift')) return path.replace(/\.swift$/, 'Tests.swift');
  if (/\.(c|cpp|cc)$/.test(path)) return path.replace(/\.(c|cpp|cc)$/, '_test.$1');
  return undefined;
}

export function workflowSeed(seed: M.ChangeSeed): M.ChangeSeed {
  const paths = [...new Set(seed.paths.filter(Boolean))].slice(0, 8);
  if (paths.length < 8 && !paths.some(isTestFilePath)) {
    for (const path of paths) {
      const testPath = inferTestPath(path);
      if (testPath) {
        paths.push(testPath);
        break;
      }
    }
  }
  const seenTargets = new Set<string>();
  const finding_targets = seed.finding_targets
    ?.filter((t) => {
      const key = `${t.project_id}:${t.kind}:${t.source || ''}:${t.id}:${t.path}`;
      if (!paths.includes(t.path) || seenTargets.has(key)) return false;
      seenTargets.add(key);
      return true;
    })
    .slice(0, 16);
  return { ...seed, paths, ...(finding_targets ? { finding_targets } : {}) };
}
function workflowReviewable(change: M.ChangeSession) {
  const flow = change.workflow;
  return (
    !flow ||
    (flow.status === 'awaiting_human_review' &&
      flow.review?.verdict === 'approve' &&
      flow.review.proposal_hash === change.hash)
  );
}
export function worktreeDefaultTarget(title: string, rootPath?: string) {
  const slug =
    title
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 32) || 'fix';
  const suffix = Math.random().toString(36).slice(2, 8);
  const base = (rootPath || '').trim().replace(/\/+$/, '');
  return {
    branch: `codex/${slug}-${suffix}`,
    path: base ? `${base}/.worktrees/${slug}-${suffix}` : `.worktrees/${slug}-${suffix}`,
  };
}

export function inProgressFindingTargets(s: State): M.FindingTarget[] {
  const seen = new Set<string>();
  const targets: M.FindingTarget[] = [];
  const add = (list?: M.FindingTarget[]) => {
    for (const t of list || []) {
      const key = `${t.kind}:${t.id}:${t.path}:${t.source || ''}`;
      if (!seen.has(key)) {
        seen.add(key);
        targets.push(t);
      }
    }
  };
  if (s.change?.state === 'draft') add(s.change.finding_targets);
  for (const entry of s.changeHistory || []) {
    if (entry.state === 'draft') add(entry.finding_targets);
  }
  for (const tree of s.gitWorkspace?.worktrees || []) {
    if (tree.current || tree.prunable) continue;
    for (const entry of [...(tree.sessions || []), ...(tree.completed || [])]) {
      if (entry.state === 'draft' || entry.state === 'applied') add(entry.finding_targets);
    }
  }
  return targets;
}

export class Workspace {
  state = initial();
  private listeners = new Set<() => void>();
  private api: API;
  constructor(
    readonly worktreePath?: string,
    private parent?: Workspace,
  ) {
    this.api = new API(worktreePath);
  }
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
    if ('analysisSetup' in patch) writeStoredJson('mini-orca:analysis-models', patch.analysisSetup);
    if ('workflowModels' in patch)
      writeStoredJson('mini-orca:workflow-models', patch.workflowModels);
    if ('analysisLimits' in patch)
      writeStoredJson('mini-orca:analysis-limits', patch.analysisLimits);
    if ('analysisRefresh' in patch)
      writeStoredJson('mini-orca:analysis-refresh', patch.analysisRefresh);
    if ('modelPresets' in patch) writeStoredJson('mini-orca:model-presets', patch.modelPresets);
    this.state = { ...this.state, ...patch };
    this.listeners.forEach((fn) => fn());
  }
  fail(error: unknown) {
    this.set({ error: errorMessage(error) });
  }
  private confirm(value: Confirmation): Promise<boolean> {
    if (this.parent) return this.parent.confirm(value);
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
      this.connectEvents();
    }
    this.schedule();
  }
  stop() {
    this.epoch++;
    this.operation++;
    this.fileEpoch++;
    for (const child of this.state.implementations) child.stop();
    this.stopped = true;
    clearTimeout(this.timer);
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = undefined;
    }
    this.respond(false);
    void this.api.cancelAll().catch((error) => this.fail(error));
  }
  private eventSource?: EventSource;
  private connectEvents() {
    if (
      typeof EventSource === 'undefined' ||
      this.stopped ||
      (typeof window !== 'undefined' &&
        (window as { fixture?: { enableSSE?: boolean } }).fixture &&
        !(window as { fixture?: { enableSSE?: boolean } }).fixture?.enableSSE)
    ) {
      return;
    }
    if (this.eventSource) this.eventSource.close();
    const url = this.worktreePath
      ? `${current}/events?worktree_path=${encodeURIComponent(this.worktreePath)}`
      : `${current}/events`;
    try {
      const es = new EventSource(url);
      es.addEventListener('update', () => {
        this.handleServerEvent();
      });
      this.eventSource = es;
    } catch {
      // Fallback to timer polling when SSE is unavailable.
    }
  }
  handleServerEvent() {
    if (this.stopped) return;
    clearTimeout(this.timer);
    void this.poll().finally(() => {
      this.schedule();
    });
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
      await Promise.all([
        this.resource(
          'models',
          () => this.api.get<M.ModelCatalog>('/api/models/current'),
          (models) => this.set({ models }),
        ),
        this.resource(
          'availableModels',
          () => this.api.get<M.AvailableModels>('/api/models/available'),
          (availableModels) => this.set({ availableModels }),
        ),
      ]);
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
      if (name in this.state.resourceErrors) {
        const resourceErrors = { ...this.state.resourceErrors };
        delete resourceErrors[name];
        this.set({ resourceErrors });
      }
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
      for (const child of this.state.implementations) child.stop();
      const defaults = initial();
      this.state = {
        ...defaults,
        connected: true,
        version: this.state.version,
        models: this.state.models,
        availableModels: this.state.availableModels,
        analysisSetup: this.state.analysisSetup ?? defaults.analysisSetup,
        workflowModels: this.state.workflowModels ?? defaults.workflowModels,
        analysisLimits: this.state.analysisLimits ?? defaults.analysisLimits,
        analysisRefresh: this.state.analysisRefresh ?? defaults.analysisRefresh,
        modelPresets: this.state.modelPresets ?? defaults.modelPresets,
        workspaceTrust: this.state.workspaceTrust ?? defaults.workspaceTrust,
        busy: this.state.busy,
        page: 'summary',
        project,
      };
      localStorage.setItem('mini-orca:last-project', project.path);

      this.set({});
      await this.refreshProject();
    });
  }
  async refreshRepository() {
    if (!this.state.project) return;
    await this.resource(
      'Git',
      () => this.api.get<M.GitRepository>(`${current}/git/repository`, this.identity()),
      (repository) => {
        if (projectKey(repository) !== projectKey(this.state.project))
          throw new Error('Git status belongs to another project.');
        const change = this.state.change;
        const stale =
          change?.state === 'draft' &&
          change.git &&
          (change.git.head !== repository.head || change.git.branch !== repository.branch);
        this.set({
          repository,
          ...(stale ? { change: { ...change, freshness: 'stale', reviewed_hash: '' } } : {}),
        });
      },
    );
  }
  async loadGitWorkspace() {
    await this.resource(
      'Git / Worktrees',
      () => this.api.get<M.GitWorkspace>(`${current}/git/workspace`, this.identity()),
      (gitWorkspace) => {
        if (projectKey(gitWorkspace) !== projectKey(this.state.project))
          throw new Error('Git workspace changed. Refresh it.');
        for (const child of this.state.implementations)
          if (!gitWorkspace.worktrees.some((tree) => tree.path === child.worktreePath))
            this.closeWorktree(child.worktreePath!);
        const preview = this.state.gitPreview;
        this.set({
          gitWorkspace,
          repository: gitWorkspace,
          gitPreview:
            preview?.target.head === gitWorkspace.head &&
            preview.target.branch === gitWorkspace.branch
              ? preview
              : undefined,
        });
      },
    );
  }
  private async readGitMergePreview(tree: M.GitWorktree, repository: M.GitWorkspace) {
    const preview = await this.api.post<M.GitMergePreview>(`${current}/git/merge-preview`, {
      ...this.identity(),
      head: repository.head,
      branch: repository.branch,
      path: tree.path,
      source_head: tree.head,
    });
    if (
      preview.target.head !== repository.head ||
      preview.target.branch !== repository.branch ||
      preview.source.head !== tree.head ||
      preview.source.branch !== tree.branch ||
      preview.path !== tree.path
    )
      throw new Error('The merge preview changed. Refresh Git status.');
    return preview;
  }
  async gitAction(action: 'merge' | 'abort-merge', tree?: M.GitWorktree) {
    await this.act('Update Git workspace', async () => {
      const repository = this.state.gitWorkspace;
      if (!repository || this.state.resourceErrors['Git / Worktrees'] || this.state.uncertain)
        throw new Error('Refresh Git status first.');
      const epoch = this.epoch,
        navigation = this.navigation;
      const preview =
        action === 'merge' && tree
          ? await this.readGitMergePreview(tree, repository)
          : this.state.gitPreview;
      if (epoch !== this.epoch || navigation !== this.navigation) return;
      if (
        action === 'merge' &&
        (!preview ||
          preview.target.head !== repository.head ||
          preview.target.branch !== repository.branch ||
          !preview.commits.trim())
      )
        throw new Error('No current changes to merge. Refresh Git status.');
      if (
        action === 'abort-merge' &&
        !(await this.confirm({
          title: 'Abort this merge?',
          message:
            'Discard conflict-resolution edits and restore the state before this merge. Open terminal sessions will close.',
          accept: 'Abort merge',
          destructive: true,
        }))
      )
        return;
      if (epoch !== this.epoch || navigation !== this.navigation) return;
      await this.api.post(`${current}/execution-trust`, {
        project_revision: repository.project_revision,
        confirm: true,
      });
      if (epoch !== this.epoch || navigation !== this.navigation) return;
      for (const terminal of this.state.terminals) await native().CloseTerminal(terminal.id);
      this.set({ terminals: [] });
      if (epoch !== this.epoch || navigation !== this.navigation) return;
      let result: M.GitMutation;
      try {
        result = await this.api.post<M.GitMutation>(`${current}/git/${action}`, {
          ...this.identity(),
          head: repository.head,
          branch: repository.branch,
          confirm: true,
          confirm_execution: true,
          ...(action === 'merge'
            ? { path: preview!.path, source_head: preview!.source.head }
            : { merge_head: repository.merge_head }),
        });
      } catch (error) {
        if (!(error instanceof ApiError))
          this.set({ uncertain: true, gitPreview: undefined, gitResult: undefined });
        throw error;
      }
      if (epoch !== this.epoch) return;
      this.epoch++;
      this.fileEpoch++;
      if (result.removed_worktree) this.closeWorktree(result.removed_worktree);
      this.set({
        project: {
          ...this.state.project!,
          project_revision: result.index?.project_revision || repository.project_revision,
        },
        index: result.index,
        repository: result.repository,
        gitResult: result,
        gitPreview: undefined,
        change: undefined,
        changeSeed: undefined,
        changeReceipt: undefined,
        results: {},
        preview: undefined,
        fileStale: true,
        uncertain: !result.index,
      });
      await this.refreshProject();
      await this.loadGitWorkspace();
    });
  }
  async discardWorktree(tree: M.GitWorktree) {
    await this.act('Discard worktree', async () => {
      const repository = this.state.gitWorkspace;
      if (!repository || this.state.resourceErrors['Git / Worktrees'] || this.state.uncertain)
        throw new Error('Refresh Git status first.');
      const epoch = this.epoch,
        navigation = this.navigation;
      if (
        !(await this.confirm({
          title: `Discard ${tree.branch}?`,
          message: `Delete the worktree at ${tree.path} and its local task history without merging. Its implementation will close. The branch and committed changes are kept. Trust this project for local execution; approved project checks may run.`,
          accept: 'Discard change',
          destructive: true,
        }))
      )
        return;
      if (epoch !== this.epoch || navigation !== this.navigation) return;
      await this.api.post(`${current}/execution-trust`, {
        project_revision: repository.project_revision,
        confirm: true,
      });
      if (epoch !== this.epoch || navigation !== this.navigation) return;
      let result: M.GitMutation;
      try {
        result = await this.api.post<M.GitMutation>(`${current}/git/discard-worktree`, {
          ...this.identity(),
          head: repository.head,
          branch: repository.branch,
          path: tree.path,
          source_head: tree.head,
          source_branch: tree.branch,
          confirm: true,
          confirm_execution: true,
        });
      } catch (error) {
        if (epoch === this.epoch) {
          if (error instanceof ApiError) await this.loadGitWorkspace();
          else this.set({ uncertain: true, gitPreview: undefined, gitResult: undefined });
        }
        throw error;
      }
      if (epoch !== this.epoch) return;
      if (result.removed_worktree) this.closeWorktree(result.removed_worktree);
      this.set({ gitResult: result, gitPreview: undefined });
      await this.loadGitWorkspace();
    });
  }
  async prepareFixInWorktree(
    seed: M.ChangeSeed,
    models: M.ChangeWorkflowModels,
    preparation: FixPreparationContext,
    branch: string,
    path: string,
    tests = true,
  ) {
    let created: string | undefined;
    const originalProject = projectKey(this.state.project);
    const originalEpoch = this.epoch;
    await this.act('Create worktree', async () => {
      const epoch = this.epoch;
      const profiles = seed.paths.some(isTestFilePath) ? Object.values(models) : [models.create];
      const latest = await this.checkedFixPreparation(profiles, preparation);
      if (!latest || epoch !== this.epoch) return;
      await this.api.post(`${current}/execution-trust`, {
        project_revision: latest.trust.project_revision,
        confirm: true,
      });
      const result = await this.api.post<M.GitMutation>(`${current}/git/worktrees`, {
        ...this.identity(),
        head: preparation.repository.head,
        branch: preparation.repository.branch,
        target_branch: branch,
        path,
        confirm: true,
        confirm_execution: true,
      });
      if (epoch !== this.epoch) return;
      this.set({ gitResult: result });
      if (result.state !== 'created' || result.error || !result.path)
        throw new Error(
          `${result.error || 'Worktree creation failed.'}${result.path ? ` Path: ${result.path}` : ''}`,
        );
      created = result.path;
      await this.loadGitWorkspace();
    });
    if (!created || projectKey(this.state.project) !== originalProject || this.state.busy) return;
    const child = await this.ensureAgentWorktree(created);
    if (!child || projectKey(this.state.project) !== originalProject) return;
    child.seedChange(seed, 'agents');
    this.set({
      selectedAgentPath: created,
      agentDetailOpen: true,
      page: 'agents',
      error: '',
    });
    let fresh: FixPreparationContext;
    try {
      fresh = await child.readFixPreparation();
      if (
        Object.values(models).some(
          (profile) =>
            JSON.stringify(effectiveWorkflowModel(fresh, profile)) !==
            JSON.stringify(effectiveWorkflowModel(preparation, profile)),
        )
      )
        throw new Error(
          'Models changed. Review preparation in the new worktree before continuing.',
        );
    } catch (error) {
      child.fail(error);
      return;
    }
    if (originalEpoch !== this.epoch) return;
    if (seed.paths.some(isTestFilePath))
      await child.startChangeWorkflow(seed, models, fresh, 'agents');
    else await child.prepareChange(seed, tests, models.create, fresh, 'agents');
  }
  async ensureAgentWorktree(path: string): Promise<Workspace | undefined> {
    let child = this.state.implementations.find((item) => item.worktreePath === path);
    if (child) return child;
    const epoch = this.epoch;
    child = new Workspace(path, this);
    const project = await child.api.get<M.Project>(current);
    if (epoch !== this.epoch) return undefined;
    child.set({
      project,
      connected: true,
      models: this.state.models,
      availableModels: this.state.availableModels,
      workflowModels: this.state.workflowModels,
      workspaceTrust: this.state.workspaceTrust,
      page: 'agents',
    });
    await child.refreshProject();
    if (epoch !== this.epoch) return undefined;
    if (!child.state.change && child.state.changeHistory?.length) {
      await child.viewChange(child.state.changeHistory[0].id, 'agents');
    }
    this.set({ implementations: [...this.state.implementations, child] });
    child.subscribe(() => this.set({}));
    child.connectEvents();
    child.schedule();
    return child;
  }
  openAgentsDashboard() {
    this.set({ agentDetailOpen: false, page: 'agents', error: '' });
  }
  async selectAgent(worktreePath?: string) {
    if (!worktreePath) {
      this.set({
        selectedAgentPath: undefined,
        agentDetailOpen: true,
        page: 'agents',
        error: '',
      });
      return;
    }
    await this.act('Open agent workspace', async () => {
      const child = await this.ensureAgentWorktree(worktreePath);
      if (!child) return;
      if (!child.state.change && child.state.changeHistory?.length) {
        await child.viewChange(child.state.changeHistory[0].id, 'agents');
      }
      this.set({
        selectedAgentPath: worktreePath,
        agentDetailOpen: true,
        page: 'agents',
        error: '',
      });
    });
  }
  async startAgentFix(rawSeed: M.ChangeSeed, options?: { preferWorktree?: boolean }) {
    const preparedSeed = workflowSeed(rawSeed);
    const useWorktree = options?.preferWorktree ?? true;
    try {
      const context = await this.readFixPreparation();
      const models = workflowSetupModels(this.state);
      const canCreateWorktree =
        useWorktree &&
        !this.worktreePath &&
        context.repository.available &&
        context.repository.at_root &&
        !context.repository.conflicts?.length &&
        !!context.repository.branch &&
        !!context.repository.head &&
        !context.repository.merge_head &&
        !context.repository.operation;
      if (canCreateWorktree) {
        const target = worktreeDefaultTarget(
          preparedSeed.title,
          context.repository.root || this.state.project?.path,
        );
        let child: Workspace | undefined;
        let fresh: FixPreparationContext | undefined;
        await this.act('Create worktree', async () => {
          const epoch = this.epoch;
          const profiles = preparedSeed.paths.some(isTestFilePath)
            ? Object.values(models)
            : [models.create];
          const latest = await this.checkedFixPreparation(profiles, context);
          if (!latest || epoch !== this.epoch) return;
          await this.api.post(`${current}/execution-trust`, {
            project_revision: latest.trust.project_revision,
            confirm: true,
          });
          const result = await this.api.post<M.GitMutation>(`${current}/git/worktrees`, {
            ...this.identity(),
            head: context.repository.head,
            branch: context.repository.branch,
            target_branch: target.branch,
            path: target.path,
            confirm: true,
            confirm_execution: true,
          });
          if (epoch !== this.epoch) return;
          this.set({ gitResult: result });
          if (result.state !== 'created' || result.error || !result.path)
            throw new Error(
              `${result.error || 'Worktree creation failed.'}${result.path ? ` Path: ${result.path}` : ''}`,
            );
          await this.loadGitWorkspace();
          child = await this.ensureAgentWorktree(result.path);
          if (!child || epoch !== this.epoch) return;
          fresh = await child.readFixPreparation();
          child.seedChange(preparedSeed, 'agents');
          this.set({
            ...(this.state.change ? {} : { changeSeed: undefined }),
            selectedAgentPath: result.path,
            agentDetailOpen: false,
            page: 'agents',
            error: '',
          });
        });
        if (!child || !fresh) return;
        const activeChild = child;
        const activeFresh = fresh;
        void (async () => {
          try {
            if (preparedSeed.paths.some(isTestFilePath)) {
              await activeChild.startChangeWorkflow(preparedSeed, models, activeFresh, 'agents');
            } else {
              await activeChild.prepareChange(
                preparedSeed,
                true,
                models.create,
                activeFresh,
                'agents',
              );
            }
            await this.loadGitWorkspace();
          } catch (error) {
            activeChild.fail(error);
          }
        })();
        return;
      }
      this.seedChange(preparedSeed, 'agents');
      this.set({ selectedAgentPath: undefined, agentDetailOpen: false, page: 'agents' });
      const useAgents = preparedSeed.paths.some(isTestFilePath);
      if (useAgents) {
        await this.startChangeWorkflow(preparedSeed, models, context, 'agents');
      } else {
        await this.prepareChange(preparedSeed, true, models.create, context, 'agents');
      }
    } catch (error) {
      this.fail(error);
    }
  }
  async mergeAgentWorktree(worktreePath: string) {
    if (!this.state.gitWorkspace) await this.loadGitWorkspace();
    const tree = this.state.gitWorkspace?.worktrees.find((item) => item.path === worktreePath);
    if (!tree) {
      this.fail(new Error('Worktree not found. Refresh Git status.'));
      return;
    }
    await this.gitAction('merge', tree);
    if (this.state.run) {
      await Promise.all(
        (['bugs', 'security', 'performance'] as const).map((cat) => this.loadResults(cat)),
      );
    }
  }
  private closeWorktree(path: string) {
    const child = this.state.implementations.find((item) => item.worktreePath === path);
    if (child) child.stop();
    this.set({
      implementations: this.state.implementations.filter((item) => item !== child),
      ...(this.state.selectedAgentPath === path
        ? { selectedAgentPath: undefined, agentDetailOpen: false }
        : {}),
    });
  }
  async refreshProject() {
    const identity = this.identity();
    await Promise.all([
      this.refreshRepository(),
      ...(!this.worktreePath ? [this.loadGitWorkspace()] : []),
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
        (result) =>
          this.set({ findings: result.findings || [], completedFindings: result.completed || [] }),
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
    if (
      this.parent &&
      !['chat', 'changes', 'history', 'instructions', 'editor', 'context', 'models'].includes(page)
    ) {
      await this.parent.navigate(page);
      return;
    }
    this.navigation++;
    this.set({
      page,
      ...(page === 'agents' ? { agentDetailOpen: false } : {}),
      error: '',
      modelsReturn:
        page === 'models' && this.state.page !== 'models'
          ? this.state.page
          : this.state.modelsReturn,
      selectedFinding: [
        'finding',
        'models',
        'search',
        'project',
        'terminal',
        'analysis-preview',
      ].includes(page)
        ? this.state.selectedFinding
        : undefined,
    });
    if (['analysis', 'models'].includes(page) && !this.state.availableModels)
      await this.loadAvailableModels();
    if (page === 'git') await this.loadGitWorkspace();
    if (page === 'editor') await this.refreshFile();
    if (['chat', 'changes', 'history', 'agents'].includes(page)) await this.loadChangeHistory();
    if (page === 'features' || page === 'summary') await this.loadFeatures();
    if (page === 'instructions')
      await this.loadInstructions(this.state.instructionPreview?.path || 'AGENTS.md');
    const category = findingCategory(this.state);
    if (category) await this.loadResults(category);
    if (page === 'context') await this.inspectContext();
  }
  openFinding(category: FindingCategory, key: string) {
    this.navigation++;
    this.set({ page: 'finding', selectedFinding: { category, key }, error: '' });
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
    if (!activeChangeWorkflow(this.state.change)) await this.refreshRepository();
    const old = JSON.stringify(this.state.run);
    await this.resource(
      'run',
      () => this.api.get<M.AnalysisRun | null>(`${current}/analysis/run`, identity),
      (run) => this.set({ run }),
    );
    if (JSON.stringify(this.state.run) !== old) {
      await this.refreshProject();
      const category = findingCategory(this.state);
      if (category) {
        await Promise.all([
          this.loadResults('bugs'),
          this.loadResults('security'),
          this.loadResults('performance'),
        ]);
      }
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
  applyModelPreset(preset: ModelPreset) {
    this.set({
      analysisSetup: { ...preset.analysis },
      workflowModels: { ...preset.workflow },
      preview: undefined,
      resume: undefined,
      page: this.state.page === 'search' ? 'models' : this.state.page,
      notice: `Applied model preset "${preset.name}".`,
    });
  }
  saveCustomModelPreset(name: string, description = 'Custom saved model routing preset.') {
    const trimmed = name.trim();
    if (!trimmed) return;
    const analysis = analysisSetupModels(this.state);
    const workflow = workflowSetupModels(this.state);
    const existing = (this.state.modelPresets || []).filter(
      (p) => p.name.toLowerCase() !== trimmed.toLowerCase(),
    );
    const preset: ModelPreset = {
      id: `custom-${Date.now()}`,
      name: trimmed,
      description,
      analysis,
      workflow,
    };
    this.set({
      modelPresets: [...existing, preset],
      notice: `Saved model preset "${trimmed}".`,
    });
  }
  deleteCustomModelPreset(id: string) {
    const next = (this.state.modelPresets || []).filter((p) => p.id !== id);
    this.set({ modelPresets: next });
  }
  async previewAnalysis(
    mode: 'new' | 'repair' | 'resume' | 'stale',
    limits: M.Limits,
    refresh = false,
    models?: M.AnalysisModels,
    stalePath?: string,
    start = false,
    clearResults = false,
  ) {
    await this.act(start ? 'Execute analysis' : 'Preview analysis', async () => {
      const epoch = this.epoch;
      const operation = this.operation;
      const navigation = this.navigation;
      const setup = this.state.analysisSetup ? { ...this.state.analysisSetup } : undefined;
      const selection_id = this.state.selection?.selection_id;
      const run = mode === 'resume' ? this.state.run : undefined;
      const preview = await this.api.post<M.AnalysisPreview>(`${current}/analysis/preview`, {
        ...this.identity(),
        scope: 'project',
        include_features: run ? !!run.plan.features : mode === 'new',
        models: run ? run.plan.models : models,
        limits: run?.plan.limits || limits,
        refresh: run ? run.plan.refresh : refresh || clearResults,
        clear_results: run ? !!run.plan.clear_results : clearResults,
        retry_stale_failed: run ? !!run.plan.retry_stale_failed : false,
        recover_incomplete: mode === 'repair',
        stale_only: mode === 'stale',
        stale_path: stalePath,
        ...(run ? { resume_run: run.identity } : {}),
      });
      if (
        epoch !== this.epoch ||
        operation !== this.operation ||
        (start && navigation !== this.navigation) ||
        selection_id !== this.state.selection?.selection_id ||
        JSON.stringify(setup) !== JSON.stringify(this.state.analysisSetup)
      )
        return;
      this.set({
        preview,
        resume: run?.identity,
        ...(start ? {} : { page: 'analysis-preview' as Page }),
      });
      if (start) await this.dispatchAnalysis(false);
    });
  }
  async resumeAnalysisDirect() {
    const run = this.state.run;
    if (!run) return;
    await this.previewAnalysis(
      'resume',
      run.plan.limits,
      run.plan.refresh,
      run.plan.models,
      run.plan.stale_path,
      true,
    );
  }
  async runAnalysisDirect(clearResults = true) {
    const limits = this.state.analysisLimits || {
      batch_files: 20,
      budget_seconds: 1800,
      max_attempts_per_stage: 2,
    };
    const refresh = clearResults || this.state.analysisRefresh || false;
    const models = analysisSetupModels(this.state);
    await this.previewAnalysis('new', limits, refresh, models, undefined, true, clearResults);
  }
  async startAnalysis() {
    await this.act('Start analysis', async () => {
      await this.dispatchAnalysis(true);
    });
  }
  private async dispatchAnalysis(confirm: boolean) {
    const p = this.state.preview;
    if (!p || projectKey(p.identity) !== projectKey(this.state.project))
      throw new Error('Prepare a fresh analysis preview.');
    if (!p.files.length && !p.features?.max_model_requests)
      throw new Error('No analysis work is available for the selected files.');
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
    if (confirm && (remote.length || p.security_review_intent_required)) {
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
    const clearing = !!p.clear_results && !this.state.resume;
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
          clear_results: p.clear_results || false,
          include_features: !!p.features,
          models: p.models,
          retry_stale_failed: p.retry_stale_failed || false,
          stale_only: p.stale_only || false,
          stale_path: p.stale_path,
          recover_incomplete: p.recover_incomplete || false,
          confirmations,
        });
    if (epoch !== this.epoch || operation !== this.operation) return;
    if (projectKey(run.identity) !== projectKey(p.identity))
      throw new Error('Analysis belongs to another project revision.');
    const activeTargets = clearing ? inProgressFindingTargets(this.state) : [];
    const retainedFindings = clearing
      ? (this.state.findings || [])
          .filter((f) => f.status === 'in_progress' || activeTargets.some((t) => t.id === f.id))
          .map((f) => (f.status === 'open' ? { ...f, status: 'in_progress' } : f))
      : this.state.findings;
    this.set({
      run,
      preview: undefined,
      resume: undefined,
      results: {},
      ...(clearing
        ? {
            findings: retainedFindings,
            features: this.state.features
              ? { ...this.state.features, suggestions: [], generations: [] }
              : undefined,
          }
        : {}),
      page: [
        'summary',
        'docs',
        'bugs',
        'security',
        'performance',
        'finding',
        'analysis-run',
      ].includes(this.state.page)
        ? this.state.page
        : 'bugs',
      featureGenerationRequested: this.state.featureGenerationRequested || !!p.features,
    });
    if (clearing) await this.refreshProject();
    await this.loadFeatures();
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
  async triage(finding: M.Finding | M.FindingTarget, status: string) {
    await this.act('Update finding', async () => {
      const target = 'kind' in finding ? finding : undefined;
      await this.api.request('PATCH', `${current}/findings/${encodeURIComponent(finding.id)}`, {
        project_revision: this.identity().project_revision,
        status,
        ...(target
          ? {
              kind: target.kind,
              path: target.path,
              ...(target.source ? { source: target.source } : {}),
            }
          : {}),
      });
      await this.refreshProject();
      for (const cat of ['bugs', 'security', 'performance'] as const) {
        if (this.state.results[cat] || findingCategory(this.state) === cat) {
          await this.loadResults(cat);
        }
      }
    });
  }
  async batchTriage(findings: (M.Finding | M.FindingTarget)[], status: string) {
    if (!findings.length) return;
    await this.act('Update findings', async () => {
      const project_revision = this.identity().project_revision;
      for (const finding of findings) {
        const target = 'kind' in finding ? finding : undefined;
        await this.api.request('PATCH', `${current}/findings/${encodeURIComponent(finding.id)}`, {
          project_revision,
          status,
          ...(target
            ? {
                kind: target.kind,
                path: target.path,
                ...(target.source ? { source: target.source } : {}),
              }
            : {}),
        });
      }
      await this.refreshProject();
      for (const cat of ['bugs', 'security', 'performance'] as const) {
        if (this.state.results[cat]) await this.loadResults(cat);
      }
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
      ...(symbol && !this.state.fileStale
        ? [
            this.resource(
              'impact',
              () => this.api.get<M.Impact>(`${current}/impact`, { path, symbol }),
              (impact) => this.set({ impact }),
              () => epoch === this.fileEpoch,
            ),
          ]
        : []),
    ]);
  }
  async selectSymbol(symbol: string) {
    if (this.state.busy) return;
    const epoch = ++this.fileEpoch;
    const path = this.state.file?.path;
    this.set({
      symbol,
      explanation: undefined,
      context: undefined,
      impact: undefined,
    });
    if (symbol && path && !this.state.fileStale) {
      const key = fileKey(this.state);
      await this.resource(
        'impact',
        () => this.api.get<M.Impact>(`${current}/impact`, { path, symbol }),
        (impact) => this.set({ impact }),
        () => fileKey(this.state) === key && epoch === this.fileEpoch,
      );
    }
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
    if (this.state.workspaceTrust || this.parent?.state.workspaceTrust)
      return model.remote_provider;
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
      if (fileKey(this.state) === key)
        this.set({ securityReport, page: 'security', selectedFinding: undefined });
    });
  }
  async setWorkspaceTrust(trusted: boolean) {
    if (typeof localStorage !== 'undefined')
      localStorage.setItem('mini-orca:workspace-trust', String(trusted));
    this.set({
      workspaceTrust: trusted,
      notice: trusted
        ? 'Consolidated Workspace Trust enabled.'
        : 'Consolidated Workspace Trust disabled; explicit confirmations restored.',
    });
    for (const child of this.state.implementations) {
      child.set({ workspaceTrust: trusted });
    }
    if (trusted && this.state.project) {
      try {
        await this.api.post(`${current}/execution-trust`, {
          project_revision: this.identity().project_revision,
          confirm: true,
        });
      } catch {
        // Ignore if backend trust endpoint is unavailable until next check.
      }
    }
  }
  private async trust(): Promise<boolean> {
    const params = {
      project_revision: this.identity().project_revision,
    };
    const trust = await this.api.get<M.ExecutionTrust>(`${current}/execution-trust`, params);
    if (trust.trusted) return true;
    if (
      !this.state.workspaceTrust &&
      !this.parent?.state.workspaceTrust &&
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
    await this.act(cancel ? 'Cancel tests' : 'Run tests', async () => {
      this.set({ page: 'summary' });
      const revision = { project_revision: this.identity().project_revision };
      if (!cancel && !(await this.trust())) return;
      const scan = cancel
        ? await this.api.request<M.Scan>(
            'DELETE',
            `${current}/scan?project_revision=${encodeURIComponent(revision.project_revision)}`,
          )
        : await this.api.post<M.Scan>(`${current}/scan`, revision);
      this.set({ scan });
    });
  }
  seedChange(seed: M.ChangeSeed, targetPage?: Page) {
    if (this.state.busy || activeChangeWorkflow(this.state.change)) return;
    this.set({
      changeSeed: seed,
      change: undefined,
      changeReceipt: undefined,
      page: targetPage || changePage(seed.kind),
      error: '',
    });
    void this.loadChangeHistory();
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
  async discussFeature(idea: M.FeatureSuggestion) {
    const seed = workflowSeed({
      title: idea.title,
      kind: 'feature',
      paths: idea.paths,
      acceptance_criteria: idea.acceptance_criteria,
      message: `${idea.title}\n\nBenefit: ${idea.benefit}\nContext: ${idea.evidence}\n\nAcceptance criteria:\n${idea.acceptance_criteria.map((item) => `- ${item}`).join('\n')}`,
    });
    await this.startAgentFix(seed);
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
      this.set({ change, changeSeed: undefined, changeReceipt: undefined, page: 'changes' });
      await this.requestChangeChecks(false);
    });
  }
  newChange() {
    if (this.state.busy || activeChangeWorkflow(this.state.change)) return;
    this.set({
      page: 'chat',
      change: undefined,
      changeSeed: undefined,
      changeReceipt: undefined,
    });
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
  async viewChange(id: string, targetPage?: Page) {
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
        this.set({ change, changeSeed: undefined, page: targetPage || changePage(change.kind) });
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
        const wasRunning = activeChangeWorkflow(this.state.change);
        if (next.revision >= (this.state.change?.revision || 0)) this.set({ change: next });
        if (wasRunning && !activeChangeWorkflow(next) && this.parent) {
          void this.parent.loadGitWorkspace();
        }
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
    const choices = selected.every((profile) => catalog.scopes[profile])
      ? undefined
      : await this.api.get<M.AvailableModels>('/api/models/available');
    if (choices) this.set({ availableModels: choices });
    const resolve = (profile: string) =>
      effectiveWorkflowModel({ models: catalog, availableModels: choices }, profile);
    if (selected.some((profile) => !resolve(profile)))
      throw new Error('Choose a configured model for every workflow stage.');
    const remote = [...new Set(selected.filter((profile) => resolve(profile)?.remote_provider))];
    if (remote.length || seed.kind === 'security') {
      const accepted = await this.confirm({
        title: seed.kind === 'security' ? 'Start security workflow?' : 'Send workflow context?',
        message:
          'Create, write and run tests, then request a model review for the captured files. Human review is required before Apply.',
        accept: 'Start workflow',
        details: [
          ...Object.entries(models).map(([stage, profile]) => {
            const model = resolve(profile)!;
            return `${stage}: ${model.model} · ${profile} · ${model.provider_origin}`;
          }),
          ...seed.paths,
        ],
      });
      if (!accepted) return null;
      const [latestCatalog, latestChoices] = await Promise.all([
        this.api.get<M.ModelCatalog>('/api/models/current'),
        choices
          ? this.api.get<M.AvailableModels>('/api/models/available')
          : Promise.resolve(undefined),
      ]);
      if (
        selected.some(
          (profile) =>
            JSON.stringify(resolve(profile)) !==
            JSON.stringify(
              effectiveWorkflowModel(
                { models: latestCatalog, availableModels: latestChoices },
                profile,
              ),
            ),
        )
      )
        throw new Error('Provider configuration changed. Review the new destinations.');
    }
    return remote;
  }
  async readFixPreparation(): Promise<FixPreparationContext> {
    const epoch = this.epoch;
    const [models, trust, repository] = await Promise.all([
      this.api.get<M.ModelCatalog>('/api/models/current'),
      this.api.get<M.ExecutionTrust>(`${current}/execution-trust`, {
        project_revision: this.identity().project_revision,
      }),
      this.api.get<M.GitRepository>(`${current}/git/repository`, this.identity()),
    ]);
    const setup = workflowSetupModels(this.state);
    const availableModels = Object.values(setup).every((profile) =>
      effectiveWorkflowModel({ models, availableModels: this.state.availableModels }, profile),
    )
      ? this.state.availableModels
      : await this.api.get<M.AvailableModels>('/api/models/available');
    if (epoch !== this.epoch || projectKey(trust) !== projectKey(this.state.project))
      throw new Error('Project changed. Refresh fix preparation.');
    this.set({ models, ...(availableModels ? { availableModels } : {}), repository });
    return { models, availableModels, trust, repository };
  }
  private async checkedFixPreparation(profiles: string[], preparation: FixPreparationContext) {
    const epoch = this.epoch,
      operation = this.operation;
    const latest = await this.readFixPreparation();
    if (epoch !== this.epoch || operation !== this.operation) return null;
    if (profiles.some((profile) => !latest.models.scopes[profile]) && !latest.availableModels) {
      latest.availableModels = await this.api.get<M.AvailableModels>('/api/models/available');
      if (epoch !== this.epoch || operation !== this.operation) return null;
      this.set({ availableModels: latest.availableModels });
    }
    if (
      projectKey(latest.trust) !== projectKey(preparation.trust) ||
      latest.repository.head !== preparation.repository.head ||
      latest.repository.branch !== preparation.repository.branch ||
      JSON.stringify(latest.trust.commands) !== JSON.stringify(preparation.trust.commands) ||
      profiles.some((profile) => {
        const currentModel = effectiveWorkflowModel(latest, profile);
        const preparedModel = effectiveWorkflowModel(preparation, profile);
        return !currentModel || JSON.stringify(currentModel) !== JSON.stringify(preparedModel);
      })
    )
      throw new Error('Preparation changed. Review the current models and permissions.');
    return latest;
  }
  private async authorizeFix(profiles: string[], preparation: FixPreparationContext) {
    const epoch = this.epoch;
    const latest = await this.checkedFixPreparation(profiles, preparation);
    if (!latest) return null;
    const remote = [
      ...new Set(profiles.filter((p) => effectiveWorkflowModel(latest, p)?.remote_provider)),
    ];
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
    targetPage?: Page,
  ) {
    await this.act('Start workflow', async () => {
      if (activeChangeWorkflow(this.state.change))
        throw new Error('Finish or cancel the active workflow.');
      if (!seed.paths.some(isTestFilePath))
        throw new Error(
          'Include a test path (e.g. _test.go, .test.ts, test_*.py) in Files to change.',
        );
      const epoch = this.epoch,
        operation = this.operation;
      const profiles = preparation
        ? await this.authorizeFix(Object.values(models), preparation)
        : await this.confirmWorkflow(seed, models);
      if (profiles === null || (!preparation && !(await this.trust()))) return;
      if (epoch !== this.epoch || operation !== this.operation) return;
      let change = this.state.change;
      if (
        !change ||
        change.state !== 'draft' ||
        change.freshness !== 'current' ||
        change.messages.length > 36
      ) {
        change = await this.api.post<M.ChangeSession>(`${current}/changes`, {
          ...this.identity(),
          kind: seed.kind,
          title: seed.title,
          paths: seed.paths,
          acceptance_criteria: seed.acceptance_criteria,
          finding_targets: seed.finding_targets,
        });
        if (epoch !== this.epoch || operation !== this.operation) return;
        if (projectKey(change) !== projectKey(this.state.project))
          throw new Error('Conversation belongs to another project revision.');
        this.set({ change, page: targetPage || changePage(change.kind) });
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
            auto_repair: true,
          },
        );
        if (epoch !== this.epoch || operation !== this.operation || this.state.change?.id !== id)
          return;
        if (next.id !== id || projectKey(next) !== projectKey(change) || !next.workflow)
          throw new Error('Workflow identity does not match this conversation.');
        this.set({
          change: next,
          changeReceipt: undefined,
          workflowModels: models,
          page: targetPage || changePage(next.kind),
        });
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
  async rerunChangeWorkflow(targetPage?: Page, retryInstructions?: string) {
    const change = this.state.change;
    const page = targetPage || this.state.page;
    if (!change && !this.worktreePath && page === 'agents' && this.state.changeSeed) {
      await this.startAgentFix(this.state.changeSeed);
      return;
    }
    const rawBase =
      change?.messages.find(
        (m) =>
          m.role === 'user' &&
          !m.content.startsWith('Write meaningful regression and boundary tests'),
      )?.content ||
      this.state.changeSeed?.message ||
      (change ? `Fix ${change.title}` : '');
    const baseMessage = rawBase
      .replace(
        /\n\n(?:Critic review feedback|Previous check failures to resolve|Previous workflow failure to resolve|Additional retry instructions):[\s\S]*$/,
        '',
      )
      .trim();
    const review = change?.workflow?.review;
    const criticFeedback =
      review?.verdict === 'changes_requested'
        ? `\n\nCritic review feedback:\n${review.summary}${
            review.findings?.length ? '\n' + review.findings.map((f) => `- ${f}`).join('\n') : ''
          }`
        : '';
    const failedChecks = (change?.checks || []).filter(
      (c) => c.state !== 'passed' && c.state !== 'skipped',
    );
    const checkFeedback = failedChecks.length
      ? `\n\nPrevious check failures to resolve:\n${failedChecks
          .map((c) => {
            const raw = (c.output || c.state).trim();
            const snippet = raw.slice(-1500);
            return `${c.name} (${c.state}):\n${snippet}`;
          })
          .join('\n')}`
      : change?.workflow?.status === 'failed' && change.workflow.reason?.trim()
        ? `\n\nPrevious workflow failure to resolve:\n${change.workflow.reason.trim()}`
        : '';
    const noteFeedback = retryInstructions?.trim()
      ? `\n\nAdditional retry instructions:\n${retryInstructions.trim()}`
      : '';
    const seed: M.ChangeSeed | undefined = change
      ? workflowSeed({
          title: change.title,
          kind: change.kind,
          paths: change.targets.map((t) => t.path),
          message: (baseMessage + criticFeedback + checkFeedback + noteFeedback).slice(0, 8000),
          acceptance_criteria: change.acceptance_criteria,
          finding_targets: change.finding_targets,
        })
      : this.state.changeSeed
        ? workflowSeed({
            ...this.state.changeSeed,
            message: (this.state.changeSeed.message + noteFeedback).slice(0, 8000),
          })
        : undefined;
    if (!seed) return;
    try {
      const models = workflowSetupModels(this.state);
      const context = await this.readFixPreparation();
      const hasTestTarget = change
        ? change.targets.some((t) => isTestFilePath(t.path))
        : seed.paths.some(isTestFilePath);
      if (hasTestTarget) {
        await this.startChangeWorkflow(seed, models, context, page);
      } else {
        await this.prepareChange(seed, true, models.create, context, page);
      }
    } catch (error) {
      this.fail(error);
    }
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
    targetPage?: Page,
  ) {
    await this.act('Prepare change', async () => {
      if (!seed.message.trim()) throw new Error('Describe the change.');
      if (preparation && !(await this.authorizeFix([profile], preparation))) return;
      if (
        !this.state.change ||
        this.state.change.state !== 'draft' ||
        this.state.change.freshness !== 'current' ||
        this.state.change.messages.length >= 32
      ) {
        const identity = this.identity();
        const epoch = this.epoch;
        const operation = this.operation;
        const change = await this.api.post<M.ChangeSession>(`${current}/changes`, {
          ...identity,
          kind: seed.kind,
          title: seed.title,
          paths: seed.paths,
          acceptance_criteria: seed.acceptance_criteria,
          finding_targets: seed.finding_targets,
        });
        if (epoch !== this.epoch || operation !== this.operation) return;
        if (projectKey(change) !== projectKey(this.state.project))
          throw new Error('Conversation belongs to another project revision.');
        this.set({ change, page: targetPage || changePage(change.kind) });
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
  async repairChange(message = 'Repair failed checks.') {
    await this.act('Repair proposal', async () => {
      const runTests =
        this.state.change?.check_options?.run_tests ||
        !!this.state.change?.workflow ||
        (this.state.change?.targets || []).some((t) => isTestFilePath(t.path));
      if (this.state.change && this.state.change.checks.length === 0) {
        if (!(await this.requestChangeChecks(runTests))) return;
        if (this.state.change && changeChecksPassed(this.state.change)) return;
      }
      if (await this.requestChangeMessage(message, true, workflowSetupModels(this.state).create))
        await this.requestChangeChecks(runTests);
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
      this.set({ change, changeSeed: undefined, page: changePage(change.kind) });
    });
  }
  async acceptChange(commitMessage?: string) {
    await this.act('Apply change', async () => {
      if (
        this.state.change &&
        this.state.change.state === 'draft' &&
        this.state.change.freshness === 'current' &&
        this.state.change.changes.length > 0 &&
        !activeChangeWorkflow(this.state.change) &&
        !canAcceptChange(this.state)
      ) {
        if (!(await this.requestChangeChecks(!!this.state.change.check_options?.run_tests))) return;
      }
      let change = this.state.change;
      if (
        !change ||
        !['editor', 'chat', 'changes', 'agents', 'finding'].includes(this.state.page) ||
        !canAcceptChange(this.state)
      )
        throw new Error('The displayed proposal needs current passing checks and model review.');
      if (!this.state.repository || this.state.resourceErrors.Git) {
        await this.refreshRepository();
      }
      const epoch = this.epoch;
      const operation = this.operation;
      const navigation = this.navigation;
      const repository = this.state.repository;
      const message = (commitMessage ?? change.title).trim();
      if (!repository || this.state.resourceErrors.Git)
        throw new Error('Refresh Git status before applying.');
      if (repository.available && !message) throw new Error('Enter a commit message.');
      // One explicit acceptance records review and applies only the displayed revision.
      let identity = this.changeIdentity(change);
      let reviewed: M.ChangeSession;
      try {
        reviewed = await this.api.post<M.ChangeSession>(
          `${current}/changes/${encodeURIComponent(change.id)}/review`,
          identity,
        );
      } catch (error) {
        if (
          error instanceof ApiError &&
          error.status === 400 &&
          (await this.requestChangeChecks(!!change.check_options?.run_tests)) &&
          this.state.change &&
          canAcceptChange(this.state)
        ) {
          change = this.state.change;
          identity = this.changeIdentity(change);
          reviewed = await this.api.post<M.ChangeSession>(
            `${current}/changes/${encodeURIComponent(change.id)}/review`,
            identity,
          );
        } else {
          throw error;
        }
      }
      if (
        epoch !== this.epoch ||
        operation !== this.operation ||
        navigation !== this.navigation ||
        !['editor', 'chat', 'changes', 'agents', 'finding'].includes(this.state.page) ||
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
      const receipt = await this.writeChangeSource(change.id, 'apply', {
        ...identity,
        ...(repository.available
          ? { head: repository.head, branch: repository.branch, commit_message: message }
          : {}),
      });
      await this.acceptChangeMutation(receipt);
    });
  }
  async applyProposalNow(commitMessage?: string) {
    await this.acceptChange(commitMessage);
  }
  async updateProposalFileContent(path: string, content: string, note = 'Updated diff hunks.') {
    await this.act('Update proposal', async () => {
      const change = this.state.change;
      if (!change || change.state !== 'draft' || change.freshness !== 'current')
        throw new Error('Start a fresh proposal before updating hunks.');
      const identity = this.changeIdentity(change);
      const updatedChanges = change.changes
        .map((edit) =>
          edit.path === path ? { path, content } : { path: edit.path, content: edit.content },
        )
        .filter((edit) => {
          const target = change.targets.find((t) => t.path === edit.path);
          return !target || edit.content !== target.content;
        });
      if (!updatedChanges.length) {
        this.set({
          change: undefined,
          changeSeed: undefined,
          notice: 'All hunks rejected; proposal cleared.',
        });
        return;
      }
      const next = await this.api.post<M.ChangeSession>(
        `${current}/changes/${encodeURIComponent(change.id)}/messages`,
        {
          ...identity,
          message: note,
          changes: updatedChanges,
        },
      );
      this.set({ change: next, changeReceipt: undefined });
      await this.requestChangeChecks(false);
    });
  }
  private async applyDirectFile(path: string, content: string) {
    if (activeChangeWorkflow(this.state.change))
      throw new Error('Finish or cancel the active workflow first.');
    const isInstructions = path === 'AGENTS.md' || path.endsWith('/AGENTS.md');
    const change = await this.api.post<M.ChangeSession>(`${current}/instructions/proposal`, {
      ...this.identity(),
      kind: isInstructions ? 'instructions' : 'feature',
      title: `Save ${path}`,
      paths: [path],
      acceptance_criteria: [`Apply direct edits to ${path}.`],
      content,
    });
    this.set({ change, changeSeed: undefined, changeReceipt: undefined });
    const checked = await this.api.post<M.ChangeSession>(
      `${current}/changes/${encodeURIComponent(change.id)}/checks`,
      { ...this.changeIdentity(change), run_tests: false },
    );
    const reviewed = await this.api.post<M.ChangeSession>(
      `${current}/changes/${encodeURIComponent(checked.id)}/review`,
      this.changeIdentity(checked),
    );
    this.set({ change: reviewed });
    const repository = this.state.repository;
    const receipt = await this.writeChangeSource(reviewed.id, 'apply', {
      ...this.changeIdentity(reviewed),
      ...(repository?.available
        ? { head: repository.head, branch: repository.branch, commit_message: `Update ${path}` }
        : {}),
    });
    await this.acceptChangeMutation(receipt);
    if (this.state.file?.path === path) await this.refreshFile();
    if (isInstructions) {
      const preview = await this.api
        .get<M.InstructionPreview>(`${current}/instructions`, {
          path,
          project_revision: this.identity().project_revision,
        })
        .catch(() => undefined);
      if (preview) {
        this.set({
          instructionPreview: preview,
          ...(path === 'AGENTS.md' ? { projectInstructions: preview } : {}),
        });
      }
    }
  }
  async saveDirectFile(path: string, content: string) {
    await this.act('Save file', async () => {
      await this.applyDirectFile(path, content);
      this.set({ notice: `Saved ${path} (Undo available).` });
    });
  }
  private async executeBootstrapHooks() {
    const execRes = await this.api
      .post<M.BootstrapExecResult>(`${current}/bootstrap/exec`, {
        project_revision: this.identity().project_revision,
        confirm: true,
      })
      .catch(() => undefined);
    if (execRes) this.set({ bootstrapExec: execRes });
    return execRes;
  }
  async runBootstrapToolchain() {
    await this.act('Run mise trust && mise install', async () => {
      const execRes = await this.executeBootstrapHooks();
      const summary = execRes?.steps.map((s) => `${s.command} (${s.state})`).join(' · ');
      this.set({
        notice: summary
          ? `Toolchain synced: ${summary}.`
          : 'No .mise.toml or lefthook.yml found in this repository.',
      });
    });
  }
  async bootstrapPillar(pillarId: BootstrapChecklistPillar['id']) {
    await this.act('Bootstrap repository pillar', async () => {
      const pillar = evaluateBootstrapChecklist(this.state).find((item) => item.id === pillarId);
      if (!pillar) return;
      await this.applyDirectFile(pillar.targetPath, pillar.templateContent);
      const execRes =
        pillar.id === 'toolchain' || pillar.id === 'precommit'
          ? await this.executeBootstrapHooks()
          : undefined;
      const hookNote = execRes?.steps.length
        ? ` · ${execRes.steps.map((s) => `${s.command}: ${s.state}`).join(', ')}`
        : '';
      this.set({ notice: `Bootstrapped ${pillar.targetPath}${hookNote} (Undo available).` });
    });
  }
  async bootstrapMissingPillars() {
    await this.act('Bootstrap repository workflow', async () => {
      const missing = evaluateBootstrapChecklist(this.state).filter((item) => !item.ready);
      if (!missing.length) {
        await this.executeBootstrapHooks();
        this.set({ notice: 'All 4 repository bootstrap pillars are already ready.' });
        return;
      }
      for (const pillar of missing) {
        await this.applyDirectFile(pillar.targetPath, pillar.templateContent);
      }
      const execRes = await this.executeBootstrapHooks();
      const hookNote = execRes?.steps.length
        ? ` · ${execRes.steps.map((s) => `${s.command}: ${s.state}`).join(', ')}`
        : '';
      this.set({
        notice: `Bootstrapped ${missing.map((p) => p.targetPath).join(', ')}${hookNote} (Undo available).`,
      });
    });
  }
  async openRepoBootstrap() {
    this.set({ instructionsTab: 'bootstrap' });
    await this.navigate('instructions');
  }
  async retryChangeCommit(message: string) {
    await this.act('Retry commit', async () => {
      const receipt = this.state.changeReceipt;
      if (!receipt || receipt.commit?.state !== 'failed' || this.state.uncertain)
        throw new Error('Refresh commit recovery first.');
      const result = await this.writeChangeSource(receipt.session_id, 'commit', {
        ...this.identity(),
        revision: this.state.change?.revision || 0,
        hash: receipt.hash,
        commit_message: message,
      });
      await this.acceptChangeMutation(result);
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
              ? receipt.commit?.state === 'committed'
                ? 'Restore the previous files as uncommitted changes. The Git commit stays in history.'
                : 'Undo the latest unchanged proposal.'
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
        .filter((edit) => !/\.(md|txt)$/i.test(edit.path))
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
  private async writeChangeSource(
    id: string,
    action: 'apply' | 'undo' | 'commit',
    identity: object,
  ) {
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
        ? {
            ...this.state.change,
            state: receipt.state,
            commit: receipt.commit || this.state.change.commit,
            reviewed_hash: '',
          }
        : undefined,
      fileStale: true,
      results: {},
      preview: undefined,
      uncertain: !receipt.index,
    });
    await this.refreshProject();
    await this.loadChangeHistory();
    if (this.parent) {
      await this.parent.loadGitWorkspace();
    } else if (this.state.run) {
      await Promise.all(
        (['bugs', 'security', 'performance'] as const).map((cat) => this.loadResults(cat)),
      );
    }
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
