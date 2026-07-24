package handlers

import (
	"encoding/json"
	"net/http"

	"mini-orca/internal/orchestrator"
	"mini-orca/internal/tools"
)

// InsertionHandler handles function insertion API endpoints.
type InsertionHandler struct {
	insertionMgr *orchestrator.InsertionManager
	fileOps      *tools.FileOps
}

// NewInsertionHandler creates a new insertion handler.
func NewInsertionHandler(orch *orchestrator.Orchestrator, fileOps *tools.FileOps) *InsertionHandler {
	insertionMgr := orchestrator.NewInsertionManager(orch, fileOps, nil)
	return &InsertionHandler{
		insertionMgr: insertionMgr,
		fileOps:      fileOps,
	}
}

// InsertFunction handles POST /api/insert
func (h *InsertionHandler) InsertFunction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FunctionCode   string `json:"function_code"`
		FilePath       string `json:"file_path"`
		Position       string `json:"position"`
		ReferenceFunc  string `json:"reference_function"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// In production, run this in a goroutine
	// go func() {
	// 	insertionPoint := orchestrator.InsertionPoint{
	// 		FilePath:  req.FilePath,
	// 		Position:  req.Position,
	// 		Reference: req.ReferenceFunc,
	// 	}
	// 	h.insertionMgr.InsertFunction(ctx, req.FunctionCode, insertionPoint)
	// }()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "accepted",
		"message":      "Function insertion accepted",
		"file_path":    req.FilePath,
		"position":     req.Position,
	})
}

// GetFileFunctions handles GET /api/file/functions
func (h *InsertionHandler) GetFileFunctions(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("file")
	
	// In production, parse the file and extract function names
	functions := []string{
		"Main",
		"Initialize",
		"ProcessData",
		"CalculateTotal",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file":      filePath,
		"functions": functions,
	})
}

// GetInsertionPoints handles GET /api/insertion-points
func (h *InsertionHandler) GetInsertionPoints(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("file")
	
	// In production, analyze the file and suggest insertion points
	points := []map[string]interface{}{
		{
			"position": "before",
			"reference": "Main",
			"description": "Insert before Main function",
		},
		{
			"position": "after",
			"reference": "Initialize",
			"description": "Insert after Initialize function",
		},
		{
			"position": "append",
			"reference": "",
			"description": "Append to end of file",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file":           filePath,
		"insertion_points": points,
	})
}
