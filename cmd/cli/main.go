package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type MicroTask struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type ProjectContext struct {
	CurrentState string      `json:"current_state"`
	ProjectGoal  string      `json:"project_goal"`
	Analysis     string      `json:"analysis"`
	Tasks        []MicroTask `json:"tasks"`
}

const daemonURL = "http://127.0.0.1:8080/api/session"

func main() {
	fmt.Println("=== Local Agent Orchestrator CLI ===")

	for {
		// 1. Fetch current status from running daemon
		resp, err := http.Get(daemonURL)
		if err != nil {
			fmt.Println("Error: Cannot connect to daemon. Is it running? Checking again in 3s...")
			time.Sleep(3 * time.Second)
			continue
		}

		var ctx ProjectContext
		json.NewDecoder(resp.Body).Decode(&ctx)
		resp.Body.Close()

		// 2. Render UI based on current State Engine status
		fmt.Printf("\n[Current State]: %s\n", ctx.CurrentState)

		switch ctx.CurrentState {
		case "PROJECT_ANALYSIS":
			fmt.Println("Agent is currently analyzing your goal. Waiting...")
			time.Sleep(2 * time.Second)

		case "WAITING_FOR_DESIGN_APPROVAL":
			fmt.Println("\n--- Proposing Architectural Solution ---")
			fmt.Println(ctx.Analysis)
			fmt.Println("----------------------------------------")

			if askApproval("Do you approve this high-level architecture design?") {
				sendApproval(true)
			} else {
				sendApproval(false)
				fmt.Println("Design rejected. Returning to analysis pool.")
			}

		case "TECHNICAL_DESIGN":
			fmt.Println("Agent is breaking design down into structured task definitions...")
			time.Sleep(2 * time.Second)

		case "WAITING_FOR_TASK_APPROVAL":
			fmt.Println("\n--- Generated Micro-Task List ---")
			for _, t := range ctx.Tasks {
				fmt.Printf("[%s] - %s (Status: %s)\n", t.ID, t.Description, t.Status)
			}
			fmt.Println("---------------------------------")

			if askApproval("Do you authorize the execution of these sequential actions?") {
				sendApproval(true)
			} else {
				sendApproval(false)
				fmt.Println("Task block execution halted.")
			}

		case "MICRO_EXECUTION":
			fmt.Println("Executing micro-actions. Check daemon console logs for outputs...")
			time.Sleep(3 * time.Second)

		case "COMPLETED":
			fmt.Println("🎉 Project Phase completed successfully! Enter a new goal in state.json to restart.")
			return
		}
	}
}

func askApproval(prompt string) bool {
	for {
		fmt.Printf("\n%s (y/n): ", prompt)
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))
		if input == "y" || input == "yes" {
			return true
		}
		if input == "n" || input == "no" {
			return false
		}
	}
}

func sendApproval(approve bool) {
	payload := fmt.Sprintf(`{"approved": %t}`, approve)
	url := "http://127.0.0.1:8080/api/session/approve"
	_, err := http.Post(url, "application/json", bytes.NewBufferString(payload))
	if err != nil {
		fmt.Errorf("Failed to transmit confirmation signal: %v", err)
	}
}
