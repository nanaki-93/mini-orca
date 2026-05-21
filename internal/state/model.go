package state

import "time"

type WorkflowState string

const (
	StateAnalysis         WorkflowState = "PROJECT_ANALYSIS"
	StateWaitingForDesign WorkflowState = "WAITING_FOR_DESIGN_APPROVAL"
	StateTechnicalDesign  WorkflowState = "TECHNICAL_DESIGN"
	StateWaitingForTasks  WorkflowState = "WAITING_FOR_TASK_APPROVAL"
	StateExecution        WorkflowState = "MICRO_EXECUTION"
	StateCompleted        WorkflowState = "COMPLETED"
)

type MicroTask struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	TargetFile  string    `json:"target_file"`
	Command     string    `json:"command"` // Command to run or verify (e.g., "go test ./...")
	Content     string    `json:"content"` // Code content to write (if applicable)
	Approved    bool      `json:"approved"`
	Status      string    `json:"status"` // "pending", "running", "success", "failed"
	ErrorLog    string    `json:"error_log,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProjectContext struct {
	SessionID    string        `json:"session_id"`
	ProjectPath  string        `json:"project_path"` // Target folder on your Mac
	CurrentState WorkflowState `json:"current_state"`
	ProjectGoal  string        `json:"project_goal"`
	Analysis     string        `json:"analysis"` // Strategy document markdown from Step 1
	Tasks        []MicroTask   `json:"tasks"`    // Granular steps from Step 2
	LastUpdated  time.Time     `json:"last_updated"`
}
