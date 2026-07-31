package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
)

// SystemInfoResponse represents the system information response.
type SystemInfoResponse struct {
	CWD     string `json:"cwd"`
	HomeDir string `json:"home_dir"`
	Version string `json:"version"`
}

// SystemHandler provides system information endpoints.
type SystemHandler struct{}

// NewSystemHandler creates a new SystemHandler instance.
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

// GetSystemInfo handles GET /api/system/info
// Returns system information like current working directory and home directory.
func (h *SystemHandler) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	// Try to get a nice display name for the project
	displayName := filepath.Base(cwd)

	api.WriteJSON(w, http.StatusOK, SystemInfoResponse{
		CWD:     cwd,
		HomeDir: homeDir,
		Version: displayName,
	})
}
