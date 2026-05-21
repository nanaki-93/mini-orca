package operation

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nanaki-93/mini-orca/internal/agent"
	"github.com/nanaki-93/mini-orca/internal/state"
	"github.com/nanaki-93/mini-orca/internal/tools"
)

type StateMachine struct {
	Store        *state.Store
	Agent        *agent.Client
	Tools        *tools.Registry
	ApprovalChan chan bool // Unbuffered channel to manage human confirmation gates
}

func NewStateMachine(s *state.Store, a *agent.Client, t *tools.Registry, appChan chan bool) *StateMachine {
	return &StateMachine{
		Store:        s,
		Agent:        a,
		Tools:        t,
		ApprovalChan: appChan,
	}
}

// StartLoop initiates the main background worker routine for the orchestrator daemon
func (sm *StateMachine) StartLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("State Machine loop received cancellation signal. Terminating workers...")
			return
		default:
			// Load current state context snapshot from the persistent JSON store
			projectCtx, err := sm.Store.Load()
			if err != nil {
				log.Printf("State Machine Engine Error: Failed to load current context: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			// Direct current execution track based on state configuration models
			switch projectCtx.CurrentState {
			case state.StateAnalysis:
				log.Printf("Executing Stage 1: Running structural analysis for goal: %s", projectCtx.ProjectGoal)

				analysisMarkdown, err := sm.Agent.GenerateProjectAnalysis(projectCtx.ProjectGoal)
				if err != nil {
					log.Printf("Stage 1 execution error: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}

				projectCtx.Analysis = analysisMarkdown
				projectCtx.CurrentState = state.StateWaitingForDesign
				_ = sm.Store.Save(projectCtx)
				log.Println("Stage 1 Complete. System paused. Awaiting human confirmation via Gate 1.")

			case state.StateWaitingForDesign:
				// The engine halts execution here! It blocks on the approval channel,
				// waiting until you send a signal through your UI or terminal client.
				select {
				case <-ctx.Done():
					return
				case approved := <-sm.ApprovalChan:
					if approved {
						// Reload context to capture any manual structural edits you saved into db.json
						projectCtx, _ = sm.Store.Load()
						projectCtx.CurrentState = state.StateTechnicalDesign
						_ = sm.Store.Save(projectCtx)
						log.Println("Gate 1 approved. Transitioning to Technical Design formulation track.")
					} else {
						projectCtx.CurrentState = state.StateAnalysis // Reset track back to generation if rejected
						_ = sm.Store.Save(projectCtx)
					}
				}

			case state.StateTechnicalDesign:
				log.Println("Executing Stage 2: Formulating micro-activity task blocks...")

				tasks, err := sm.Agent.GenerateTaskBreakdown(projectCtx.Analysis)
				if err != nil {
					log.Printf("Stage 2 execution error: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}

				projectCtx.Tasks = tasks
				projectCtx.CurrentState = state.StateWaitingForTasks
				_ = sm.Store.Save(projectCtx)
				log.Println("Stage 2 Complete. Pipeline paused. Awaiting granular task sign-offs via Gate 2.")

			case state.StateWaitingForTasks:
				// Halt execution again. This gives you the opportunity to check-box tasks,
				// alter paths, tweak terminal command prompts, or eliminate tasks inside your dashboard UI.
				select {
				case <-ctx.Done():
					return
				case approved := <-sm.ApprovalChan:
					if approved {
						projectCtx, _ = sm.Store.Load()
						projectCtx.CurrentState = state.StateExecution
						_ = sm.Store.Save(projectCtx)
						log.Println("Gate 2 approved. Launching micro-execution runner...")
					} else {
						projectCtx.CurrentState = state.StateTechnicalDesign
						_ = sm.Store.Save(projectCtx)
					}
				}

			case state.StateExecution:
				log.Println("Executing Stage 3: Running micro-activities sequentially through workspace tools...")

				for i, task := range projectCtx.Tasks {
					if !task.Approved || task.Status == "success" {
						continue
					}

					projectCtx.Tasks[i].Status = "running"
					projectCtx.Tasks[i].UpdatedAt = time.Now()
					_ = sm.Store.Save(projectCtx)

					log.Printf("Executing task [%s]: %s", task.ID, task.Description)

					// Determine appropriate tool grouping configuration options
					toolType := "shell"
					if len(task.TargetFile) > 3 && task.TargetFile[len(task.TargetFile)-3:] == ".tf" {
						toolType = "terraform"
					}

					// Route commands natively to execution abstractions
					output, err := sm.Tools.RunTask(ctx, projectCtx.ProjectPath, toolType, task.Command, task.TargetFile)

					projectCtx, _ = sm.Store.Load() // Reload to capture real-time state mutations
					if err != nil {
						projectCtx.Tasks[i].Status = "failed"
						projectCtx.Tasks[i].ErrorLog = fmt.Sprintf("Error: %v. Execution Output: %s", err, output)
						projectCtx.Tasks[i].UpdatedAt = time.Now()
						_ = sm.Store.Save(projectCtx)
						log.Printf("Task execution failure encountered on task [%s]. Halting sequence pipeline.", task.ID)

						// Pivot pipeline back to task authorization queue so you can repair the error prompt
						projectCtx.CurrentState = state.StateWaitingForTasks
						_ = sm.Store.Save(projectCtx)
						break
					}

					projectCtx.Tasks[i].Status = "success"
					projectCtx.Tasks[i].UpdatedAt = time.Now()
					_ = sm.Store.Save(projectCtx)
				}

				// Verify if all steps in collection finished successfully
				allDone := true
				for _, t := range projectCtx.Tasks {
					if t.Approved && t.Status != "success" {
						allDone = false
						break
					}
				}

				if allDone {
					projectCtx.CurrentState = state.StateCompleted
					_ = sm.Store.Save(projectCtx)
					log.Println("All micro-activities executed seamlessly. Project phase generation finalized successfully.")
				}

			case state.StateCompleted:
				// Idle operational loops until a fresh operational request or reset occurs
				time.Sleep(5 * time.Second)
			}
		}
	}
}
