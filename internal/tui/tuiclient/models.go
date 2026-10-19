package tuiclient

// ProjectIdentity identifies a specific project and its current content revision.
type ProjectIdentity struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
}

// Project holds repository metadata and summary metrics returned by the daemon.
type Project struct {
	ProjectIdentity
	Name            string         `json:"name"`
	Path            string         `json:"path"`
	Type            string         `json:"type"`
	BuildFile       string         `json:"build_file"`
	FileCount       int            `json:"file_count"`
	SourceFileCount int            `json:"source_file_count"`
	TotalLines      int            `json:"total_lines"`
	Languages       map[string]int `json:"languages"`
	Files           []string       `json:"files"`
	Summary         string         `json:"summary"`
	AIStatus        string         `json:"ai_status"`
	AnalyzedAt      string         `json:"analyzed_at"`
}

// SymbolInfo describes an indexed symbol declaration in a source file.
type SymbolInfo struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Signature    string `json:"signature"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	Confidence   string `json:"confidence"`
	AtomicTarget bool   `json:"atomic_target"`
}

// IndexedFile holds indexed metadata for a single repository file.
type IndexedFile struct {
	Path           string       `json:"path"`
	ContentHash    string       `json:"content_hash"`
	Language       string       `json:"language"`
	Binary         bool         `json:"binary"`
	SizeBytes      int          `json:"size_bytes"`
	LineCount      int          `json:"line_count"`
	Imports        []string     `json:"imports"`
	Symbols        []SymbolInfo `json:"symbols"`
	AnalysisStatus string       `json:"analysis_status"`
}

// ProjectIndex holds the repository's indexed file list.
type ProjectIndex struct {
	ProjectIdentity
	Files []IndexedFile `json:"files"`
}

// FileInfo contains the full contents and metadata of an opened repository file.
type FileInfo struct {
	Path        string `json:"path"`
	ContentHash string `json:"content_hash"`
	Name        string `json:"name"`
	Language    string `json:"language"`
	SizeBytes   int    `json:"size_bytes"`
	LineCount   int    `json:"line_count"`
	Binary      bool   `json:"binary"`
	Content     string `json:"content"`
}

// ProjectDocsComponent describes an architectural layer or component in the handbook.
type ProjectDocsComponent struct {
	Name             string   `json:"name"`
	Layer            string   `json:"layer"`
	Path             string   `json:"path"`
	Summary          string   `json:"summary"`
	Responsibilities []string `json:"responsibilities"`
	Dependencies     []string `json:"dependencies"`
}

// ProjectDocsScenarioStep describes a single step in an execution scenario.
type ProjectDocsScenarioStep struct {
	Phase  string `json:"phase"`
	Layer  string `json:"layer"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Path   string `json:"path,omitempty"`
}

// ProjectDocsScenario describes an end-to-end runtime scenario in the handbook.
type ProjectDocsScenario struct {
	Title     string                    `json:"title"`
	Trigger   string                    `json:"trigger"`
	Invariant string                    `json:"invariant"`
	Summary   string                    `json:"summary"`
	Diagram   string                    `json:"diagram,omitempty"`
	Steps     []ProjectDocsScenarioStep `json:"steps"`
}

// ProjectDocsDecision describes an architectural decision record (ADR).
type ProjectDocsDecision struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Decision string `json:"decision"`
	Tradeoff string `json:"tradeoff"`
}

// ProjectDocsDependency describes a library or tool dependency in the handbook.
type ProjectDocsDependency struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Kind    string `json:"kind"`
	Source  string `json:"source"`
	Purpose string `json:"purpose"`
}

// ProjectDocsContext describes the high-level purpose and topology of a project.
type ProjectDocsContext struct {
	Purpose             string   `json:"purpose"`
	GoalPursuit         []string `json:"goal_pursuit,omitempty"`
	Topology            string   `json:"topology"`
	Stack               []string `json:"stack"`
	ArchitectureDiagram string   `json:"architecture_diagram,omitempty"`
}

// ProjectDocsReport contains the structured engineering handbook for a project.
type ProjectDocsReport struct {
	ProjectIdentity
	SchemaVersion string                  `json:"schema_version"`
	Status        string                  `json:"status"`
	Context       ProjectDocsContext      `json:"context"`
	Dependencies  []ProjectDocsDependency `json:"dependencies,omitempty"`
	Components    []ProjectDocsComponent  `json:"components"`
	Scenarios     []ProjectDocsScenario   `json:"scenarios"`
	Decisions     []ProjectDocsDecision   `json:"decisions"`
	UpdatedAt     string                  `json:"updated_at"`
}

// OverviewMetrics summarizes repository file and language counts.
type OverviewMetrics struct {
	Type            string         `json:"type"`
	FileCount       int            `json:"file_count"`
	SourceFileCount int            `json:"source_file_count"`
	TotalLines      int            `json:"total_lines"`
	Languages       map[string]int `json:"languages"`
}

// OverviewAnalysis summarizes high-level project architecture and risks.
type OverviewAnalysis struct {
	Purpose      string   `json:"purpose"`
	Architecture string   `json:"architecture"`
	Components   []string `json:"components"`
	EntryPoints  []string `json:"entry_points"`
	Flows        []string `json:"flows"`
	NextSteps    []string `json:"next_steps"`
	Status       string   `json:"status"`
	Failure      string   `json:"failure,omitempty"`
}

// FindingCounts tracks verified static findings versus AI suggestions.
type FindingCounts struct {
	Verified      int `json:"verified"`
	AISuggestions int `json:"ai_suggestions"`
}

// Overview holds the project dashboard overview returned by the daemon.
type Overview struct {
	ProjectIdentity
	Docs             *ProjectDocsReport `json:"docs,omitempty"`
	Metrics          OverviewMetrics    `json:"metrics"`
	Analysis         OverviewAnalysis   `json:"analysis"`
	AnalysisCoverage map[string]int     `json:"analysis_coverage"`
	FindingCounts    FindingCounts      `json:"finding_counts"`
	AnalysisRun      *AnalysisRun       `json:"analysis_run,omitempty"`
}

// Model describes a configured LLM scope binding and its effective runtime parameters.
type Model struct {
	Scope            string   `json:"scope"`
	Profile          string   `json:"profile"`
	Model            string   `json:"model"`
	ProviderOrigin   string   `json:"provider_origin"`
	RemoteProvider   bool     `json:"remote_provider"`
	ReasoningEffort  string   `json:"reasoning_effort,omitempty"`
	Temperature      *float32 `json:"temperature,omitempty"`
	MaxTokens        *int     `json:"max_tokens,omitempty"`
	ContextMaxTokens int      `json:"context_max_tokens,omitempty"`
	Timeout          string   `json:"timeout"`
	MaxRetries       int      `json:"max_retries,omitempty"`
}

// ModelCatalog maps model scopes (analyze, bug, function) to configured models.
type ModelCatalog struct {
	Scopes map[string]Model `json:"scopes"`
}

// ModelChoice describes an assignable model from /api/models/available.
type ModelChoice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Source   string `json:"source"`
	Location string `json:"location"`
	Model    Model  `json:"model"`
}

// ModelDiscovery describes CLI model discovery status for Pi or Agy.
type ModelDiscovery struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// AvailableModels holds the discovered and configured model catalog from /api/models/available.
type AvailableModels struct {
	Models   []ModelChoice     `json:"models"`
	Defaults map[string]string `json:"defaults"`
	Pi       ModelDiscovery    `json:"pi"`
	Agy      ModelDiscovery    `json:"agy"`
}

// ModelPreset represents a named 3-scope model configuration preset in the TUI.
type ModelPreset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Analyze  string `json:"analyze"`
	Bug      string `json:"bug"`
	Function string `json:"function"`
}

// TaskSpec describes the target and acceptance criteria for fixing a finding.
type TaskSpec struct {
	SchemaVersion      string   `json:"schema_version"`
	TargetPath         string   `json:"target_path"`
	TargetSymbol       string   `json:"target_symbol"`
	TargetSignature    string   `json:"target_signature"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	NonGoals           []string `json:"non_goals"`
}

// SourceAnchor locates a finding within a file and symbol range.
type SourceAnchor struct {
	Path      string `json:"path"`
	Symbol    string `json:"symbol"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// Finding represents a semantic or static analysis finding.
type Finding struct {
	ProjectIdentity
	ID         string       `json:"id"`
	Source     string       `json:"source"`
	Confidence string       `json:"confidence"`
	Severity   string       `json:"severity"`
	Title      string       `json:"title"`
	Message    string       `json:"message"`
	Rule       string       `json:"rule"`
	FileHash   string       `json:"file_hash"`
	Location   SourceAnchor `json:"location"`
	Evidence   string       `json:"evidence"`
	Status     string       `json:"status"`
	Freshness  string       `json:"freshness"`
	Category   string       `json:"category"`
	TaskSpec   *TaskSpec    `json:"task_spec,omitempty"`
}

// FindingTarget identifies a finding across semantic, performance, or security reports.
type FindingTarget struct {
	ProjectID string `json:"project_id"`
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Path      string `json:"path"`
	Source    string `json:"source,omitempty"`
}

// FindingCompletion records a finding closed by a committed change or merged worktree.
type FindingCompletion struct {
	FindingTarget
	CommitOID string `json:"commit_oid"`
}

// FindingsResponse wraps the daemon's /api/projects/current/findings payload.
type FindingsResponse struct {
	ProjectIdentity
	Findings  []Finding           `json:"findings"`
	Completed []FindingCompletion `json:"completed,omitempty"`
}

// Coverage tracks per-section file progress during a unified analysis run.
type Coverage struct {
	Total       int `json:"total"`
	Pending     int `json:"pending"`
	Running     int `json:"running"`
	Succeeded   int `json:"succeeded"`
	Partial     int `json:"partial"`
	Failed      int `json:"failed"`
	Skipped     int `json:"skipped"`
	Unavailable int `json:"unavailable"`
}

// Section describes progress and finding counts for one analysis category.
type Section struct {
	Category     string   `json:"category"`
	Status       string   `json:"status"`
	Coverage     Coverage `json:"coverage"`
	FindingCount *int     `json:"finding_count"`
	Reason       string   `json:"reason,omitempty"`
}

// RunStage tracks a single stage's status for a file in an analysis run.
type RunStage struct {
	Stage        string `json:"stage"`
	Status       string `json:"status"`
	Attempts     int    `json:"attempts"`
	Cached       bool   `json:"cached"`
	FindingCount *int   `json:"finding_count"`
	Reason       string `json:"reason,omitempty"`
}

// RunFile tracks per-file stage outcomes in an analysis run.
type RunFile struct {
	Path        string     `json:"path"`
	ContentHash string     `json:"content_hash"`
	Language    string     `json:"language"`
	Stages      []RunStage `json:"stages"`
}

// RunIdentity uniquely identifies an analysis run and its queue generation.
type RunIdentity struct {
	ProjectIdentity
	PolicyFingerprint   string `json:"policy_fingerprint"`
	ProviderFingerprint string `json:"provider_fingerprint"`
	QueueID             string `json:"queue_id"`
	ID                  string `json:"id"`
	Generation          string `json:"generation"`
}

// AnalysisRunConfig holds user-editable parameters for project analysis runs.
type AnalysisRunConfig struct {
	BatchFiles          int  `json:"batch_files"`
	BudgetSeconds       int  `json:"budget_seconds"`
	MaxAttemptsPerStage int  `json:"max_attempts_per_stage"`
	IncludeFeatures     bool `json:"include_features"`
	Refresh             bool `json:"refresh"`
	ClearResults        bool `json:"clear_results"`
}

// AnalysisPreview holds a pre-dispatch plan for unified project analysis.
type AnalysisPreview struct {
	PreviewID                    string         `json:"preview_id"`
	Scope                        string         `json:"scope"`
	Refresh                      bool           `json:"refresh"`
	ClearResults                 bool           `json:"clear_results,omitempty"`
	IncludeFeatures              *bool          `json:"include_features,omitempty"`
	RetryStaleFailed             bool           `json:"retry_stale_failed,omitempty"`
	RecoverIncomplete            bool           `json:"recover_incomplete,omitempty"`
	StaleOnly                    bool           `json:"stale_only,omitempty"`
	StalePath                    string         `json:"stale_path,omitempty"`
	Limits                       map[string]int `json:"limits,omitempty"`
	ExpectedModelRequests        int            `json:"expected_model_requests"`
	MaxModelRequests             int            `json:"max_model_requests"`
	SecurityReviewIntentRequired bool           `json:"security_review_intent_required"`
}

// AnalysisRunFeatures tracks feature-discovery status inside a unified run.
type AnalysisRunFeatures struct {
	Status          string `json:"status"`
	Attempts        int    `json:"attempts"`
	SuggestionCount *int   `json:"suggestion_count"`
	ReportHash      string `json:"report_hash,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// AnalysisRun represents the state of a unified multi-stage analysis job.
type AnalysisRun struct {
	Features             *AnalysisRunFeatures `json:"features,omitempty"`
	Identity             RunIdentity          `json:"identity"`
	Plan                 AnalysisPreview      `json:"plan"`
	Status               string               `json:"status"`
	Files                []RunFile            `json:"files"`
	Sections             []Section            `json:"sections"`
	ElapsedSeconds       int                  `json:"elapsed_seconds"`
	WindowFilesCompleted int                  `json:"window_files_completed"`
	Reason               string               `json:"reason,omitempty"`
}

// Selection holds the user's included/excluded file selection for project analysis.
type Selection struct {
	ProjectIdentity
	SelectionID   string   `json:"selection_id"`
	ExcludedPaths []string `json:"excluded_paths"`
	Editable      bool     `json:"editable"`
}

// PerformanceFinding describes a single performance issue in a file report.
type PerformanceFinding struct {
	ID              string `json:"id"`
	Category        string `json:"category"`
	PotentialImpact string `json:"potential_impact"`
	Confidence      string `json:"confidence"`
	Title           string `json:"title"`
	ObservedPattern string `json:"observed_pattern"`
	Recommendation  string `json:"recommendation"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	Symbol          string `json:"symbol"`
}

// PerformanceReport holds performance findings for a single repository file.
type PerformanceReport struct {
	ProjectIdentity
	Path        string               `json:"path"`
	ContentHash string               `json:"content_hash"`
	Status      string               `json:"status"`
	Findings    []PerformanceFinding `json:"findings"`
}

// SecurityFinding describes a single security finding in a security report.
type SecurityFinding struct {
	ID                string       `json:"id"`
	Rule              string       `json:"rule"`
	Title             string       `json:"title"`
	SourceAnchor      SourceAnchor `json:"source_anchor"`
	Severity          string       `json:"severity"`
	Confidence        string       `json:"confidence"`
	ObservedCondition string       `json:"observed_condition"`
	Remediation       string       `json:"remediation"`
	Triage            string       `json:"triage"`
}

// SecurityReport holds security findings for a file or project scope.
type SecurityReport struct {
	ProjectIdentity
	Path        string            `json:"path"`
	ContentHash string            `json:"content_hash"`
	Status      string            `json:"status"`
	Source      string            `json:"source"`
	Findings    []SecurityFinding `json:"findings"`
}

// SectionResults holds the daemon's category-specific analysis results.
type SectionResults struct {
	Identity          RunIdentity         `json:"identity"`
	Progress          Section             `json:"progress"`
	SavedFindingCount *int                `json:"saved_finding_count"`
	Semantic          []Finding           `json:"semantic"`
	Performance       []PerformanceReport `json:"performance"`
	Security          []SecurityReport    `json:"security"`
}

// Check represents a single verification check phase (build, vet, test, etc.).
type Check struct {
	Name     string   `json:"name"`
	Required bool     `json:"required"`
	State    string   `json:"state"`
	Command  []string `json:"command"`
	Output   string   `json:"output"`
	ExitCode int      `json:"exit_code"`
}

// ExecutionTrust holds the current project's execution trust state and allowed commands.
type ExecutionTrust struct {
	ProjectIdentity
	Trusted  bool       `json:"trusted"`
	Commands [][]string `json:"commands"`
}

// Scan represents a deterministic project scan and test execution result.
type Scan struct {
	ProjectIdentity
	Status string  `json:"status"`
	Phases []Check `json:"phases"`
}

// GitIdentity captures the branch name and HEAD commit OID.
type GitIdentity struct {
	Head   string `json:"head"`
	Branch string `json:"branch"`
}

// GitRepository describes the current checkout's Git state.
type GitRepository struct {
	GitIdentity
	ProjectIdentity
	Available bool     `json:"available"`
	Root      string   `json:"root"`
	AtRoot    bool     `json:"at_root"`
	Dirty     bool     `json:"dirty"`
	MergeHead string   `json:"merge_head,omitempty"`
	Operation string   `json:"operation,omitempty"`
	Conflicts []string `json:"conflicts"`
}

// GitWorktree describes an active Git worktree (main checkout or isolated agent worktree).
type GitWorktree struct {
	GitIdentity
	Path      string               `json:"path"`
	Current   bool                 `json:"current"`
	Locked    bool                 `json:"locked"`
	Prunable  bool                 `json:"prunable"`
	Dirty     bool                 `json:"dirty,omitempty"`
	Mergeable bool                 `json:"mergeable"`
	Completed []ChangeHistoryEntry `json:"completed"`
	Sessions  []ChangeHistoryEntry `json:"sessions,omitempty"`
	Error     string               `json:"error,omitempty"`
}

// GitWorkspace describes all branches and worktrees in the repository.
type GitWorkspace struct {
	GitRepository
	Worktrees []GitWorktree `json:"worktrees"`
}

// DiffLine represents a single line in a unified diff.
type DiffLine struct {
	Kind    string `json:"kind"`
	OldLine int    `json:"old_line"`
	NewLine int    `json:"new_line"`
	Text    string `json:"text"`
}

// FileDiff holds the diff lines for a modified file in a change proposal.
type FileDiff struct {
	OldPath string     `json:"old_path"`
	NewPath string     `json:"new_path"`
	Lines   []DiffLine `json:"lines"`
}

// FileChange describes a proposed file modification in a ChangeSession.
type FileChange struct {
	Path    string   `json:"path"`
	Content string   `json:"content"`
	Hash    string   `json:"hash"`
	Summary string   `json:"summary,omitempty"`
	Diff    FileDiff `json:"diff"`
}

// ChangeTarget captures the baseline file hash and content before a proposal.
type ChangeTarget struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
}

// ChangeCommit records a Git commit created from an applied change session.
type ChangeCommit struct {
	GitIdentity
	State   string `json:"state"`
	Message string `json:"message"`
	OID     string `json:"oid,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ChangeWorkflowStage represents one stage in the 4-stage change workflow.
type ChangeWorkflowStage struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Model  *Model `json:"model,omitempty"`
}

// ChangeWorkflowReview holds the critic review verdict for a checked proposal.
type ChangeWorkflowReview struct {
	ProposalHash string   `json:"proposal_hash"`
	Verdict      string   `json:"verdict"`
	Summary      string   `json:"summary"`
	Findings     []string `json:"findings"`
}

// ChangeWorkflow tracks the 4-stage automated agent pipeline on a ChangeSession.
type ChangeWorkflow struct {
	ID                 string                `json:"id"`
	Status             string                `json:"status"`
	Reason             string                `json:"reason,omitempty"`
	AutoRepair         bool                  `json:"auto_repair,omitempty"`
	AutoRepairAttempts int                   `json:"auto_repair_attempts,omitempty"`
	ProbeSummary       string                `json:"probe_summary,omitempty"`
	HarnessLessons     []string              `json:"harness_lessons,omitempty"`
	Stages             []ChangeWorkflowStage `json:"stages"`
	Review             *ChangeWorkflowReview `json:"review,omitempty"`
	StartedAt          string                `json:"started_at"`
	UpdatedAt          string                `json:"updated_at"`
}

// ChangeMessage represents a prompt or model response in a change conversation.
type ChangeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChangeSession holds a guarded multi-file change proposal and its verification state.
type ChangeSession struct {
	ProjectIdentity
	FindingTargets     []FindingTarget `json:"finding_targets,omitempty"`
	Git                *GitIdentity    `json:"git,omitempty"`
	Commit             *ChangeCommit   `json:"commit,omitempty"`
	ID                 string          `json:"id"`
	Kind               string          `json:"kind"`
	Title              string          `json:"title"`
	AcceptanceCriteria []string        `json:"acceptance_criteria"`
	Revision           int             `json:"revision"`
	Hash               string          `json:"hash"`
	State              string          `json:"state"`
	Freshness          string          `json:"freshness"`
	Targets            []ChangeTarget  `json:"targets"`
	Changes            []FileChange    `json:"changes"`
	Messages           []ChangeMessage `json:"messages"`
	Checks             []Check         `json:"checks"`
	ReviewedHash       string          `json:"reviewed_hash,omitempty"`
	RepairAttempts     int             `json:"repair_attempts"`
	Workflow           *ChangeWorkflow `json:"workflow,omitempty"`
	UpdatedAt          string          `json:"updated_at"`
}

// ChangeHistoryEntry summarizes a persisted change proposal in history or a worktree.
type ChangeHistoryEntry struct {
	ProjectIdentity
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	Title          string          `json:"title"`
	Revision       int             `json:"revision"`
	Hash           string          `json:"hash"`
	State          string          `json:"state"`
	Freshness      string          `json:"freshness"`
	FindingTargets []FindingTarget `json:"finding_targets,omitempty"`
	Commit         *ChangeCommit   `json:"commit,omitempty"`
	UpdatedAt      string          `json:"updated_at"`
	Paths          []string        `json:"paths,omitempty"`
	WorkflowStatus string          `json:"workflow_status,omitempty"`
	WorkflowReason string          `json:"workflow_reason,omitempty"`
}

// ChangeMutation captures the receipt returned by Apply, Commit, or Undo.
type ChangeMutation struct {
	ProjectIdentity
	Commit        *ChangeCommit `json:"commit,omitempty"`
	SessionID     string        `json:"session_id"`
	State         string        `json:"state"`
	Hash          string        `json:"hash"`
	UndoAvailable bool          `json:"undo_available"`
	Warnings      []string      `json:"warnings"`
}

// ChangeSeed holds the parameters used to start a new change or agent mission.
type ChangeSeed struct {
	FindingTargets     []FindingTarget `json:"finding_targets,omitempty"`
	Title              string          `json:"title"`
	Message            string          `json:"message"`
	Kind               string          `json:"kind"`
	Paths              []string        `json:"paths"`
	AcceptanceCriteria []string        `json:"acceptance_criteria"`
}

// FeatureSuggestion represents a goal-aligned feature idea generated by analysis.
type FeatureSuggestion struct {
	ID                 string   `json:"id"`
	GenerationID       string   `json:"generation_id,omitempty"`
	Freshness          string   `json:"freshness,omitempty"`
	Title              string   `json:"title"`
	Benefit            string   `json:"benefit"`
	Evidence           string   `json:"evidence"`
	Paths              []string `json:"paths"`
	Effort             string   `json:"effort"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Status             string   `json:"status"`
}

// FeatureReport holds the repository's feature goals and generated feature ideas.
type FeatureReport struct {
	ProjectIdentity
	SchemaVersion int                 `json:"schema_version"`
	Hash          string              `json:"hash"`
	Goals         string              `json:"goals"`
	Status        string              `json:"status"`
	Freshness     string              `json:"freshness"`
	Failure       string              `json:"failure,omitempty"`
	Suggestions   []FeatureSuggestion `json:"suggestions"`
	WorkspaceHash string              `json:"workspace_hash"`
	UpdatedAt     string              `json:"updated_at"`
}

// InstructionPreset describes a recommended AGENTS.md rule preset.
type InstructionPreset struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Content  string   `json:"content"`
	Category string   `json:"category,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

// InstructionPreview holds existing and effective AGENTS.md instructions for a scope.
type InstructionPreview struct {
	ProjectIdentity
	Path            string              `json:"path"`
	Exists          bool                `json:"exists"`
	ExistingContent string              `json:"existing_content"`
	Presets         []InstructionPreset `json:"presets"`
}
