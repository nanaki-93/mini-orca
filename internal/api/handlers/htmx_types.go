package handlers

// PhaseRenderData holds data for rendering phase partials.
type PhaseRenderData struct {
	FeatureRequest       string                `json:"feature_request"`
	CurrentPhaseName     string                `json:"current_phase_name"`
	TotalPhases          int                   `json:"total_phases"`
	GeneratedCode        string                `json:"generated_code,omitempty"`
	TargetFile           string                `json:"target_file,omitempty"`
	CodingPhase          bool                  `json:"coding_phase"`
	CodingPhaseData      *CodingPhaseData      `json:"coding_phase_data,omitempty"`
	TestingPhase         bool                  `json:"testing_phase"`
	TestingPhaseData     *TestingPhaseData     `json:"testing_phase_data,omitempty"`
	ReviewPhase          bool                  `json:"review_phase"`
	ReviewPhaseData      *ReviewPhaseData      `json:"review_phase_data,omitempty"`
	HumanReviewPhase     bool                  `json:"human_review_phase"`
	HumanReviewPhaseData *HumanReviewPhaseData `json:"human_review_phase_data,omitempty"`
}

// CodingPhaseData holds data for the coding phase partial.
type CodingPhaseData struct {
	CurrentUnitNumber      int        `json:"current_unit_number"`
	TotalUnits             int        `json:"total_units"`
	ProgressPercent        int        `json:"progress_percent"`
	CurrentUnitName        string     `json:"current_unit_name"`
	CurrentUnitDescription string     `json:"current_unit_description"`
	CurrentUnitFiles       []string   `json:"current_unit_files"`
	Status                 string     `json:"status"`
	CurrentFile            string     `json:"current_file"`
	FileOperations         []FileOp   `json:"file_operations"`
	NextUnits              []NextUnit `json:"next_units"`
	Language               string     `json:"language"`
	GeneratedCode          string     `json:"generated_code"`
	GeneratedLines         int        `json:"generated_lines"`
	GeneratedSize          string     `json:"generated_size"`
	EstimatedTimeRemaining string     `json:"estimated_time_remaining"`
	CanContinue            bool       `json:"can_continue"`
	CanSkip                bool       `json:"can_skip"`
}

// FileOp represents a file operation in the coding phase.
type FileOp struct {
	File   string `json:"file"`
	Status string `json:"status"`
}

// NextUnit represents a unit waiting in the queue.
type NextUnit struct {
	Name  string   `json:"name"`
	Files []string `json:"files"`
}

// TestingPhaseData holds data for the testing phase partial.
type TestingPhaseData struct {
	TotalTests  int          `json:"total_tests"`
	PassedTests int          `json:"passed_tests"`
	FailedTests int          `json:"failed_tests"`
	Progress    int          `json:"progress"`
	CurrentTest string       `json:"current_test"`
	Status      string       `json:"status"`
	TestResults []TestResult `json:"test_results"`
	Coverage    string       `json:"coverage"`
}

// TestResult represents a single test result.
type TestResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration string `json:"duration"`
	Output   string `json:"output,omitempty"`
}

// ReviewPhaseData holds data for the review phase partial.
type ReviewPhaseData struct {
	TotalChecks  int           `json:"total_checks"`
	Completed    int           `json:"completed"`
	CurrentCheck string        `json:"current_check"`
	Status       string        `json:"status"`
	Issues       []ReviewIssue `json:"issues"`
	Score        int           `json:"score"`
}

// ReviewIssue represents an issue found during review.
type ReviewIssue struct {
	Severity string `json:"severity"`
	Category string `json:"category"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// HumanReviewPhaseData holds data for the human review phase partial.
type HumanReviewPhaseData struct {
	CurrentPhase string `json:"current_phase"`
	Output       string `json:"output"`
	Approved     bool   `json:"approved"`
	Feedback     string `json:"feedback"`
}

// FileTreeRenderData holds data for rendering the file tree partial.
type FileTreeRenderData struct {
	RootItems   []FileSystemItem `json:"root_items"`
	CurrentPath string           `json:"current_path"`
	ProjectPath string           `json:"project_path"`
}

// FileSystemItem represents a file or directory in the tree.
type FileSystemItem struct {
	Name     string           `json:"name"`
	Path     string           `json:"path"`
	IsDir    bool             `json:"is_dir"`
	Children []FileSystemItem `json:"children,omitempty"`
	Size     int64            `json:"size,omitempty"`
}

// ActivityLogRenderData holds data for rendering the activity log partial.
type ActivityLogRenderData struct {
	Entries    []ActivityEntry `json:"entries"`
	Phases     []string        `json:"phases"`
	AutoScroll bool            `json:"auto_scroll"`
}

// ActivityEntry represents a single activity log entry.
type ActivityEntry struct {
	Phase     string            `json:"phase"`
	Type      string            `json:"type"`
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Action    string            `json:"action"`
	Duration  string            `json:"duration,omitempty"`
	Details   string            `json:"details,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// PhaseTrackerRenderData holds data for rendering the phase tracker partial.
type PhaseTrackerRenderData struct {
	Phases              []PhaseTrackerItem `json:"phases"`
	CurrentPhaseName    string             `json:"current_phase_name"`
	CurrentPhaseStatus  string             `json:"current_phase_status"`
	CurrentPhaseMessage string             `json:"current_phase_message"`
}

// PhaseTrackerItem represents a phase in the tracker.
type PhaseTrackerItem struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Retries    int    `json:"retries"`
	MaxRetries int    `json:"max_retries"`
	StartTime  string `json:"start_time,omitempty"`
}

// DashboardRenderData holds data for the combined dashboard partial.
type DashboardRenderData struct {
	PhaseTracker PhaseTrackerRenderData
	ActivityLog  ActivityLogRenderData
}

// RenderErrorResponse represents an error response for render endpoints.
type RenderErrorResponse struct {
	Error string `json:"error"`
}

// RenderStatusResponse represents the status response for render endpoints.
type RenderStatusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
