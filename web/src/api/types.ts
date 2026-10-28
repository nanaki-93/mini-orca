// Wire types for the daemon's loopback API. They mirror the JSON of the Go
// DTOs in internal/app (and internal/project, internal/app/gitops); fields the
// UI does not read are omitted.

export interface ProjectIdentity {
  project_id: string;
  project_revision: string;
}

export interface GitIdentity {
  head: string;
  branch: string;
}

export interface Project extends ProjectIdentity {
  name: string;
  path: string;
  type: string;
  build_file?: string;
  file_count: number;
  source_file_count?: number;
  total_lines?: number;
  languages?: Record<string, number>;
  summary?: string;
}

export interface IndexedFile {
  path: string;
  content_hash: string;
  language: string;
  binary?: boolean;
  size_bytes?: number;
  line_count?: number;
  analysis_status?: string;
}

export interface ProjectIndex extends ProjectIdentity {
  files: IndexedFile[];
}

export interface FileInfo {
  path: string;
  content_hash: string;
  name?: string;
  language: string;
  size_bytes?: number;
  line_count?: number;
  binary?: boolean;
  content: string;
}

export interface ModelCapabilities {
  completion?: boolean;
  json_schema?: boolean;
  tools?: boolean;
  streaming?: boolean;
}

export interface Model {
  scope: string;
  profile?: string;
  model: string;
  provider_origin?: string;
  remote_provider?: boolean;
  reasoning_effort?: string;
  timeout?: string;
  context_max_tokens?: number;
  capabilities?: ModelCapabilities;
}

export interface ModelsCurrent {
  scopes: Record<string, Model>;
  /** The configuration file the daemon read; empty when it uses the built-in local defaults. */
  config_file?: string;
}

export interface ExecutionTrust extends ProjectIdentity {
  trusted: boolean;
  commands: string[][];
}

export type CheckState = 'passed' | 'failed' | 'running' | 'skipped' | 'unavailable' | 'canceled' | 'pending' | string;

export interface Check {
  name: string;
  required: boolean;
  state: CheckState;
  command?: string[];
  output?: string;
  exit_code?: number;
  duration_ms?: number;
  outcome?: string;
}

export interface DiffLine {
  kind: 'context' | 'added' | 'removed' | 'hunk' | string;
  old_line?: number;
  new_line?: number;
  text: string;
}

export interface FileDiff {
  old_path?: string;
  new_path?: string;
  lines: DiffLine[];
}

export interface FileChange {
  path: string;
  content: string;
  hash: string;
  summary?: string;
  criteria?: number[];
  diff: FileDiff;
}

export interface ChangeTarget {
  path: string;
  hash: string;
  exists: boolean;
}

export interface ChangeCommit extends Partial<GitIdentity> {
  state: string;
  message?: string;
  oid?: string;
  error?: string;
}

export interface WorkflowStage {
  name: string;
  status: string;
  reason?: string;
  started_at?: string;
  finished_at?: string;
  model?: Model;
  /** What the stage spent across attempts, and its estimated cost. */
  usage?: BudgetUsage;
  cost?: CostEstimate;
}

/** Model and tool calls and tokens an agent budget recorded. */
export interface BudgetUsage {
  model_calls: number;
  tool_calls?: number;
  input_tokens: number;
  output_tokens: number;
  unknown_usage_calls?: number;
  elapsed_millis?: number;
}

/** A cost estimate from the configured price table; `total` only when every rate is known. */
export interface CostEstimate {
  status: string;
  currency?: string;
  price_version?: string;
  known_subtotal: number;
  total?: number;
  reasons: string[];
}

export interface UsageTotal {
  runs: number;
  usage: BudgetUsage;
  cost: CostEstimate;
}

export interface UsageRow extends UsageTotal {
  session_id: string;
  title: string;
  kind?: string;
  state?: string;
  updated_at: string;
}

/** Agent usage per task in a checkout, from the agent journal. */
export interface UsageReport extends ProjectIdentity {
  currency?: string;
  price_version?: string;
  tasks: UsageRow[];
  other: UsageTotal;
  total: UsageTotal;
}

/** A record Mini-Orca set aside because it was unreadable, or skipped because a newer version wrote it. */
export interface QuarantinedFile {
  kind: 'agent_journal' | 'task' | 'analysis_run' | string;
  file: string;
  quarantined_as?: string;
  reason: string;
  remedy: string;
  at?: string;
}

export interface QuarantineReport {
  files: QuarantinedFile[];
  skipped: QuarantinedFile[];
}

/** A live update: what changed and, for workflows, the task, stage, status and spend. */
export interface DaemonEvent {
  kind: string;
  session_id?: string;
  workflow_id?: string;
  title?: string;
  stage?: string;
  status?: string;
  usage?: BudgetUsage;
  cost?: CostEstimate;
}

export interface WorkflowReview {
  proposal_hash?: string;
  verdict: 'approve' | 'changes_requested' | string;
  summary: string;
  /** Blocking issues: a review approves exactly when there are none. */
  findings: string[];
  /** Advisory suggestions that never decide the verdict. */
  notes?: string[];
}

/** A reviewer comment on one line of a proposal version. */
export interface ChangeComment {
  id: string;
  path: string;
  side: 'new' | 'old';
  line: number;
  excerpt: string;
  text: string;
  /** The proposal the comment was written on. */
  hash: string;
  /** The revision whose repair brief carried it; absent while open. */
  sent_revision?: number;
  created_at: string;
}

/** A human override of the reviewers' rejection, bound to one proposal hash. */
export interface WorkflowOverride {
  proposal_hash: string;
  reason: string;
  at: string;
}

export interface SpecialistReview {
  id: string;
  status: string;
  reason?: string;
  evidence: string;
  model?: Model;
  review?: WorkflowReview;
}

export interface WorkflowInput {
  request_id: string;
  workflow_id: string;
  status: string;
  action: string;
  expires_at: string;
}

export interface ChangeWorkflow {
  id: string;
  status: string;
  reason?: string;
  attempt?: number;
  auto_repair?: boolean;
  auto_repair_attempts?: number;
  stages: WorkflowStage[];
  review?: WorkflowReview;
  override?: WorkflowOverride;
  specialists?: SpecialistReview[];
  input?: WorkflowInput;
  started_at?: string;
  updated_at?: string;
}

export interface ChangeMessage {
  role: string;
  content: string;
}

export interface PlanEvidence {
  path: string;
  start_line?: number;
  end_line?: number;
  quote?: string;
  reason?: string;
}

export interface ChangePlan {
  problem?: string;
  approach?: string;
  evidence?: PlanEvidence[];
  assumptions?: string[];
  non_goals?: string[];
  read_paths?: string[];
  hash?: string;
  approved_hash?: string;
}

export interface FindingTarget {
  project_id: string;
  kind: string;
  id: string;
  path: string;
}

export interface UndoAvailability {
  available: boolean;
  reason: string;
}

export interface ChangeSession extends ProjectIdentity {
  id: string;
  kind: string;
  title: string;
  acceptance_criteria: string[];
  revision: number;
  hash: string;
  state: string;
  freshness: string;
  targets: ChangeTarget[];
  changes: FileChange[];
  pinned_tests?: FileChange[];
  review_diff?: FileChange[];
  messages: ChangeMessage[];
  checks: Check[];
  reviewed_hash?: string;
  review_notes?: string;
  comments?: ChangeComment[];
  /** Why the proposal is stale: the scope file that changed. */
  stale_reason?: string;
  /** The project changed since the checks ran; they must run again. */
  checks_outdated?: boolean;
  reviewed_files?: string[];
  file_review_hash?: string;
  last_reviewed_hash?: string;
  repair_attempts?: number;
  workflow?: ChangeWorkflow;
  plan?: ChangePlan;
  scope_suggestions?: string[];
  finding_targets?: FindingTarget[];
  git?: GitIdentity;
  commit?: ChangeCommit;
  worktree_path?: string;
  updated_at: string;
}

/** A change session as the UI tracks it: where it lives and its checkout identity. */
export interface LoadedSession extends ChangeSession {
  checkout: CheckoutRef;
  undo?: UndoAvailability;
}

export interface CheckoutRef extends ProjectIdentity {
  worktreePath: string;
}

export interface ChangeHistoryEntry extends ProjectIdentity {
  id: string;
  kind: string;
  title: string;
  revision: number;
  hash: string;
  state: string;
  freshness: string;
  commit?: ChangeCommit;
  updated_at: string;
  paths?: string[];
  workflow_status?: string;
  workflow_reason?: string;
  worktree_path?: string;
  finding_targets?: FindingTarget[];
}

/** A task row in lists: the history entry plus the checkout that owns it. */
export interface TaskEntry extends ChangeHistoryEntry {
  worktreePath: string;
  branch?: string;
}

export interface ChangeMutationResult extends ProjectIdentity {
  session_id: string;
  state: string;
  hash: string;
  undo_available: boolean;
  warnings: string[];
  commit?: ChangeCommit;
}

export interface SourceAnchor {
  path: string;
  start_line?: number;
  end_line?: number;
}

export interface Finding {
  id: string;
  source: string;
  confidence?: string;
  severity: string;
  title: string;
  message: string;
  rule?: string;
  location: SourceAnchor;
  evidence?: string;
  status: string;
  freshness?: string;
  category?: string;
  task_spec?: { target_path?: string; acceptance_criteria?: string[]; non_goals?: string[] };
}

export interface FindingsResponse extends ProjectIdentity {
  findings: Finding[];
}

export interface GitWorktree extends GitIdentity {
  path: string;
  current?: boolean;
  locked?: boolean;
  prunable?: boolean;
  dirty?: boolean;
  mergeable?: boolean;
  integration?: string;
  overlap_paths?: string[];
  completed?: ChangeHistoryEntry[];
  sessions?: ChangeHistoryEntry[];
  error?: string;
}

export interface GitBranch {
  name: string;
  head: string;
  worktree_path?: string;
}

export interface GitWorkspace extends GitIdentity, ProjectIdentity {
  available: boolean;
  root: string;
  dirty: boolean;
  merge_head?: string;
  operation?: string;
  conflicts: string[];
  branches?: GitBranch[];
  worktrees: GitWorktree[];
}

export interface MergePreview {
  diff?: string;
  commits?: string;
  conflicts?: string[];
  files?: string[];
  state?: string;
}

export interface DocsComponent {
  name: string;
  layer?: string;
  path?: string;
  summary?: string;
  responsibilities?: string[];
  contracts?: string[];
  dependencies?: string[];
}

export interface ProjectDocs {
  status: string;
  context?: { purpose?: string; goal_pursuit?: string[]; topology?: string; stack?: string[] };
  dependencies?: { name: string; kind?: string; version?: string; source?: string; summary?: string }[];
  components?: DocsComponent[];
  scenarios?: { title: string; trigger?: string; summary?: string; invariant?: string; steps?: { phase?: string; layer?: string; title: string; detail?: string; path?: string }[] }[];
  decisions?: { title: string; status?: string; context?: string; decision?: string; consequences?: string }[];
  runbook?: { label: string; command: string; description?: string }[];
  glossary?: { term: string; definition: string }[];
  updated_at?: string;
}

export interface AnalysisRunIdentity {
  project_id: string;
  project_revision: string;
  policy_fingerprint: string;
  provider_fingerprint: string;
  queue_id: string;
  id: string;
  generation: string;
}

export type AnalysisStageName = 'semantic' | 'performance' | 'security_rules' | 'security_ai';

export interface AnalysisStageProgress {
  stage: AnalysisStageName | string;
  status: string;
  attempts: number;
  cached: boolean;
  finding_count?: number | null;
  reason?: string;
}

export interface AnalysisRunFile {
  path: string;
  language?: string;
  stages: AnalysisStageProgress[];
}

export interface AnalysisCoverageCounts {
  total: number;
  pending: number;
  running: number;
  succeeded: number;
  partial: number;
  failed: number;
  skipped: number;
  unavailable: number;
}

export interface AnalysisSection {
  category: 'bugs' | 'performance' | 'security' | string;
  status: string;
  coverage: AnalysisCoverageCounts;
  finding_count?: number | null;
}

export interface AnalysisProvider {
  id: string;
  stages: string[];
  model: Model;
  remote_confirmation_required?: boolean;
}

/** An analysis run: what it planned, each file's stages and the totals per section. */
export interface AnalysisRunSummary {
  identity?: AnalysisRunIdentity;
  status: string;
  reason?: string;
  files?: AnalysisRunFile[];
  sections?: AnalysisSection[];
  plan?: {
    limits?: { batch_files: number; budget_seconds: number; max_attempts_per_stage: number };
    providers?: AnalysisProvider[];
    excluded?: { path: string; reason: string }[];
    refresh?: boolean;
    retry_stale_failed?: boolean;
    expected_model_requests?: number;
    max_model_requests?: number;
  };
  features?: { status: string; reason?: string; suggestion_count?: number | null } | null;
  elapsed_seconds?: number;
  window_files_completed?: number;
  created_at?: string;
  started_at?: string;
  updated_at?: string;
}

export interface Overview extends ProjectIdentity {
  docs?: ProjectDocs;
  metrics?: { type?: string; file_count?: number; source_file_count?: number; total_lines?: number; languages?: Record<string, number> };
  analysis?: { status?: string; purpose?: string; failure?: string; model?: string; generated_at?: string };
  analysis_coverage?: Record<string, number>;
  analysis_run?: AnalysisRunSummary;
  finding_counts?: { verified?: number; ai_suggestions?: number };
}

export type AgentRole = 'creation' | 'testing' | 'review';

export interface AgentSettings extends ProjectIdentity {
  configuration?: string;
  version?: number;
  state: 'configured' | 'overridden' | 'stale' | string;
  roles: Record<string, string>;
  limits?: Record<string, number>;
  ceilings?: Record<string, number>;
  skills?: { directory: string; enabled: boolean }[];
  models?: Model[];
  /** The model each role runs with; a role whose model no longer resolves has no entry. */
  role_models?: Partial<Record<AgentRole, Model>>;
}

/** A complete replacement of the project's agent settings. */
export interface AgentPreferences {
  roles: Record<AgentRole, string>;
  limits: Record<string, number>;
  disabled_skills: string[];
}

/** A model the daemon can assign: a configured scope's model or one an installed CLI lists. */
export interface ModelChoice {
  id: string;
  name: string;
  provider: string;
  source: 'configured' | 'agy' | 'pi' | string;
  location: 'local' | 'remote' | 'unknown' | string;
  model: Model;
}

export interface ModelDiscovery {
  status: 'ready' | 'unavailable' | string;
  message?: string;
}

export interface AvailableModels {
  models: ModelChoice[];
  defaults: Record<string, string>;
  pi: ModelDiscovery;
  agy: ModelDiscovery;
}

export interface DaemonStatus {
  status: string;
  version: string;
}

// ---------------------------------------------------------------- project setup

export type SetupProvider = '' | 'openai' | 'gemini' | 'vertex' | 'agy' | 'pi';

/** One model scope as setup edits it. Secrets are write-only: api_key_set says one is saved. */
export interface SetupModel {
  provider: SetupProvider | string;
  cli_path?: string;
  api_base_url?: string;
  api_key?: string;
  api_key_set?: boolean;
  clear_api_key?: boolean;
  vertex_project?: string;
  vertex_location?: string;
  model: string;
  reasoning_effort?: string;
  temperature?: number;
  max_tokens?: number;
  context_max_tokens?: number;
}

export type SetupScope = 'analyze' | 'bug' | 'code';

export interface ProjectSetup extends ProjectIdentity {
  state: 'needed' | 'skipped' | 'configured' | 'invalid' | string;
  config_file: string;
  daemon_config_file: string;
  error?: string;
  models: Record<SetupScope, SetupModel>;
  roles: Record<AgentRole, string>;
  limits: Record<string, number>;
  /** Saving replaced agent overrides from Settings → Models & agents. */
  overrides_reset?: boolean;
}

export interface SetupModelTest {
  ok: boolean;
  model?: string;
  latency_ms: number;
  reason?: string;
  detail?: string;
}

export interface AnalysisFileSelection extends ProjectIdentity {
  selection_id: string;
  excluded_paths: string[];
  files: { path: string; reason: string }[];
  editable: boolean;
}
