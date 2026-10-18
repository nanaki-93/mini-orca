package app

import (
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	maxChangePaths    = 64
	maxChangeBytes    = 256 * 1024
	maxChangeMessages = 40
)

type ChangeTarget struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
	Mode    uint32 `json:"mode"`
}

type ChangeEdit struct {
	Path    string              `json:"path"`
	Content string              `json:"content"`
	Hash    string              `json:"hash"`
	Summary string              `json:"summary,omitempty"`
	Diff    project.UnifiedDiff `json:"diff"`
}

// ChangeSession is private project-local history, unlike source-free findings
// and audits. Restored check/review evidence is cleared before reuse.
type ChangeSession struct {
	SchemaVersion      int                     `json:"schema_version"`
	ID                 string                  `json:"id"`
	ProjectID          string                  `json:"project_id"`
	ProjectRevision    string                  `json:"project_revision"`
	Kind               string                  `json:"kind"`
	Title              string                  `json:"title"`
	AcceptanceCriteria []string                `json:"acceptance_criteria"`
	FindingTargets     []project.FindingTarget `json:"finding_targets,omitempty"`
	Targets            []ChangeTarget          `json:"targets"`
	Instructions       map[string]string       `json:"instructions"`
	PolicyFingerprint  string                  `json:"policy_fingerprint"`
	WorkspaceHash      string                  `json:"workspace_hash"`
	Git                *GitIdentity            `json:"git,omitempty"`
	Commit             *ChangeCommit           `json:"commit,omitempty"`
	Revision           int64                   `json:"revision"`
	Hash               string                  `json:"hash"`
	State              string                  `json:"state"`
	Freshness          string                  `json:"freshness"`
	Messages           []ChangeMessage         `json:"messages"`
	Changes            []ChangeEdit            `json:"changes"`
	PinnedTests        []ChangeEdit            `json:"pinned_tests,omitempty"`
	ContextManifest    project.ContextManifest `json:"context_manifest"`
	Checks             []CheckResult           `json:"checks"`
	CheckOptions       CheckOptions            `json:"check_options"`
	ReviewedHash       string                  `json:"reviewed_hash,omitempty"`
	RepairAttempts     int                     `json:"repair_attempts"`
	Workflow           *ChangeWorkflow         `json:"workflow,omitempty"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

// Listing summaries keeps a full history within the native response bound;
// source and conversation contents are returned only for the chosen session.
type ChangeHistoryEntry struct {
	ID              string                  `json:"id"`
	ProjectID       string                  `json:"project_id"`
	ProjectRevision string                  `json:"project_revision"`
	Kind            string                  `json:"kind"`
	Title           string                  `json:"title"`
	Revision        int64                   `json:"revision"`
	Hash            string                  `json:"hash"`
	State           string                  `json:"state"`
	Freshness       string                  `json:"freshness"`
	WorkflowStatus  string                  `json:"workflow_status,omitempty"`
	WorkflowReason  string                  `json:"workflow_reason,omitempty"`
	Commit          *ChangeCommit           `json:"commit,omitempty"`
	FindingTargets  []project.FindingTarget `json:"finding_targets,omitempty"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type ChangeCreateRequest struct {
	ProjectID          string                  `json:"project_id"`
	ProjectRevision    string                  `json:"project_revision"`
	Kind               string                  `json:"kind"`
	Title              string                  `json:"title"`
	Paths              []string                `json:"paths"`
	AcceptanceCriteria []string                `json:"acceptance_criteria"`
	FindingTargets     []project.FindingTarget `json:"finding_targets,omitempty"`
}

type ChangeIdentity struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	Revision        int64  `json:"revision"`
	Hash            string `json:"hash"`
}

type ChangeEditInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Summary string `json:"summary,omitempty"`
}

type ChangeMessageRequest struct {
	ChangeIdentity
	Profile               string            `json:"profile,omitempty"`
	Message               string            `json:"message"`
	ConfirmRemoteProvider bool              `json:"confirm_remote_provider"`
	Repair                bool              `json:"repair,omitempty"`
	Changes               []ChangeEditInput `json:"changes,omitempty"`
}

type InstructionProposalRequest struct {
	ChangeCreateRequest
	Content string `json:"content"`
}

type ChangeCheckRequest struct {
	ChangeIdentity
	CheckOptions
}

type ChangeApplyRequest struct {
	ChangeIdentity
	GitIdentity
	Confirm       bool   `json:"confirm"`
	CommitMessage string `json:"commit_message,omitempty"`
}

type ChangeMutationResult struct {
	SessionID       string                `json:"session_id"`
	ProjectID       string                `json:"project_id"`
	ProjectRevision string                `json:"project_revision"`
	State           string                `json:"state"`
	Hash            string                `json:"hash"`
	UndoAvailable   bool                  `json:"undo_available"`
	Index           *project.ProjectIndex `json:"index,omitempty"`
	Warnings        []string              `json:"warnings"`
	Verification    *ChangeVerification   `json:"verification,omitempty"`
	Commit          *ChangeCommit         `json:"commit,omitempty"`
}
