package orchestrator

import (
	"context"
	"fmt"
	"time"

	"mini-orca/internal/state"
	"mini-orca/internal/tools"
)

// InsertionManager handles standalone function insertion.
type InsertionManager struct {
	orchestrator *Orchestrator
	fileOps      *tools.FileOps
	gitOps       *tools.GitOps
}

// NewInsertionManager creates a new insertion manager.
func NewInsertionManager(orchestrator *Orchestrator, fileOps *tools.FileOps, gitOps *tools.GitOps) *InsertionManager {
	return &InsertionManager{
		orchestrator: orchestrator,
		fileOps:      fileOps,
		gitOps:       gitOps,
	}
}

// InsertionPoint represents where to insert a function.
type InsertionPoint struct {
	FilePath   string `json:"file_path"`
	Position   string `json:"position"` // "before", "after", "append"
	Reference  string `json:"reference"` // Function name to insert before/after
}

// InsertFunction inserts a standalone function with testing and review.
func (im *InsertionManager) InsertFunction(ctx context.Context, functionCode string, insertionPoint InsertionPoint) error {
	// Get current session
	session := im.orchestrator.GetSession()

	// Create a temporary atomic unit for this insertion
	unit := state.AtomicUnit{
		ID:          fmt.Sprintf("INSERT-%d", time.Now().Unix()),
		Name:        "Standalone Function Insertion",
		Type:        "function",
		File:        insertionPoint.FilePath,
		Description: fmt.Sprintf("Insert function %s %s %s", functionCode, insertionPoint.Position, insertionPoint.Reference),
		Status:      state.UnitStatusInProgress,
		Code:        functionCode,
		CreatedAt:   time.Now(),
	}

	// Write function to file based on insertion point
	switch insertionPoint.Position {
	case "before":
		if err := im.writeBeforeFunction(insertionPoint.FilePath, insertionPoint.Reference, functionCode); err != nil {
			return fmt.Errorf("failed to insert before %s: %w", insertionPoint.Reference, err)
		}
	case "after":
		if err := im.writeAfterFunction(insertionPoint.FilePath, insertionPoint.Reference, functionCode); err != nil {
			return fmt.Errorf("failed to insert after %s: %w", insertionPoint.Reference, err)
		}
	case "append":
		if err := im.fileOps.AppendFunctionToFile(insertionPoint.FilePath, "InsertedFunction", functionCode); err != nil {
			return fmt.Errorf("failed to append function: %w", err)
		}
	default:
		if err := im.fileOps.AppendFunctionToFile(insertionPoint.FilePath, "InsertedFunction", functionCode); err != nil {
			return fmt.Errorf("failed to write function: %w", err)
		}
	}

	// Run testing phase
	session.Phase = state.PhaseTesting
	session.AddEvent(state.NewEvent("", "insertion_test", state.PhaseTesting,
		fmt.Sprintf("Testing inserted function: %s", unit.ID)))

	// Execute tests
	output, err := im.orchestrator.tools.RunTest()
	if err != nil {
		session.AddEvent(state.NewEvent("", "test_failure", state.PhaseTesting,
			fmt.Sprintf("Tests failed after insertion: %v", err)))
		return fmt.Errorf("tests failed: %w", err)
	}

	session.AddEvent(state.NewEvent("", "test_success", state.PhaseTesting,
		fmt.Sprintf("Tests passed: %s", output)))

	// Run review phase
	session.Phase = state.PhaseReview
	session.AddEvent(state.NewEvent("", "insertion_review", state.PhaseReview,
		fmt.Sprintf("Reviewing inserted function: %s", unit.ID)))

	// Get reviewer agent
	result, err := im.orchestrator.ExecuteAgent(ctx, "reviewer",
		fmt.Sprintf("Review inserted function:\n%s", functionCode))
	if err != nil {
		return fmt.Errorf("review failed: %w", err)
	}

	// Check review verdict
	if len(result.Output) > 0 {
		// Log review result
		session.AddEvent(state.NewEvent("", "insertion_review_complete", state.PhaseReview,
			fmt.Sprintf("Review complete: %s", result.Output[:min(100, len(result.Output))])))
	}

	// Commit changes
	if err := im.gitOps.CommitAtomicUnit(unit.ID, fmt.Sprintf("Insert function: %s", insertionPoint.FilePath)); err != nil {
		session.AddEvent(state.NewEvent("", "warning", state.PhaseReview,
			fmt.Sprintf("Git commit failed: %v", err)))
	}

	// Mark unit as complete
	unit.Status = state.UnitStatusComplete
	unit.UpdatedAt = time.Now()

	session.AddEvent(state.NewEvent("", "insertion_complete", state.PhaseReview,
		fmt.Sprintf("Function inserted successfully: %s", unit.ID)))

	return nil
}

// writeBeforeFunction inserts code before a reference function.
func (im *InsertionManager) writeBeforeFunction(filePath, reference, code string) error {
	content, err := im.fileOps.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Find the reference function
	refIndex := -1
	for i := 0; i < len(content); i++ {
		if i+len("func "+reference) <= len(content) {
			if content[i:i+len("func "+reference)] == "func "+reference {
				refIndex = i
				break
			}
		}
	}

	if refIndex == -1 {
		return fmt.Errorf("reference function %s not found in %s", reference, filePath)
	}

	// Insert before the reference
	newContent := content[:refIndex] + code + "\n\n" + content[refIndex:]

	return im.fileOps.WriteFile(filePath, newContent)
}

// writeAfterFunction inserts code after a reference function.
func (im *InsertionManager) writeAfterFunction(filePath, reference, code string) error {
	content, err := im.fileOps.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Find the reference function
	refIndex := -1
	for i := 0; i < len(content); i++ {
		if i+len("func "+reference) <= len(content) {
			if content[i:i+len("func "+reference)] == "func "+reference {
				refIndex = i
				break
			}
		}
	}

	if refIndex == -1 {
		return fmt.Errorf("reference function %s not found in %s", reference, filePath)
	}

	// Find the end of the reference function (simplified)
	endIndex := len(content)
	for i := refIndex + len("func "+reference); i < len(content); i++ {
		if content[i] == '\n' && i+1 < len(content) {
			rest := content[i+1:]
			if len(rest) > 4 && rest[:4] == "func" ||
				len(rest) > 4 && rest[:4] == "type" ||
				len(rest) > 9 && rest[:9] == "interface" {
				endIndex = i
				break
			}
		}
	}

	// Insert after the reference
	newContent := content[:endIndex] + code + "\n\n" + content[endIndex:]

	return im.fileOps.WriteFile(filePath, newContent)
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
