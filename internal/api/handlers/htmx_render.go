// Package handlers provides HTTP handlers for the Mini-Orca REST API.
package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/nanaki-93/mini-orca/internal/api"
	apperrors "github.com/nanaki-93/mini-orca/internal/errors"
	"github.com/nanaki-93/mini-orca/internal/logging"
	"github.com/nanaki-93/mini-orca/internal/state"
)

// ─── Template Data Types ─────────────────────────────────────────────────────

// PhaseRenderData holds data for rendering phase partials.
type PhaseRenderData struct {
	GoalDescription         string                   `json:"goal_description"`
	StatusMessage           string                   `json:"status_message"`
	EstimatedTime           string                   `json:"estimated_time"`
	PlanningSteps           []PhaseStep              `json:"planning_steps"`
	CurrentStep             int                      `json:"current_step"`
	Subtasks                []PhaseSubtask           `json:"subtasks"`
	StartTime               string                   `json:"start_time"`
	CanCancel               bool                     `json:"can_cancel"`
	TotalPhases             int                      `json:"total_phases"`
	CodingPhase             bool                     `json:"coding_phase"`
	CodingPhaseData         *CodingPhaseData         `json:"coding_phase_data,omitempty"`
	TestingPhase            bool                     `json:"testing_phase"`
	TestingPhaseData        *TestingPhaseData        `json:"testing_phase_data,omitempty"`
	ReviewPhase             bool                     `json:"review_phase"`
	ReviewPhaseData         *ReviewPhaseData         `json:"review_phase_data,omitempty"`
	HumanReviewPhase        bool                     `json:"human_review_phase"`
	HumanReviewPhaseData    *HumanReviewPhaseData    `json:"human_review_phase_data,omitempty"`
	PlanningReviewPhase     bool                     `json:"planning_review_phase"`
	PlanningReviewPhaseData *PlanningReviewPhaseData `json:"planning_review_phase_data,omitempty"`
}

// PhaseStep represents a step in the planning process.
type PhaseStep struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PhaseSubtask represents a subtask generated during planning.
type PhaseSubtask struct {
	Name     string `json:"name"`
	Priority string `json:"priority,omitempty"`
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

// PlanningReviewPhaseData holds data for the planning review phase partial.
type PlanningReviewPhaseData struct {
	Plan        *state.Plan `json:"plan"`
	CurrentStep int         `json:"current_step"`
	CanApprove  bool        `json:"can_approve"`
	CanReject   bool        `json:"can_reject"`
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

// ─── Template Engine ──────────────────────────────────────────────────────────

// TemplateEngine manages and renders HTMX partial templates.
type TemplateEngine struct {
	mu        sync.RWMutex
	templates map[string]*template.Template
	basePath  string
}

// NewTemplateEngine creates a new TemplateEngine and loads all templates.
func NewTemplateEngine(basePath string) (*TemplateEngine, error) {
	te := &TemplateEngine{
		templates: make(map[string]*template.Template),
		basePath:  basePath,
	}

	if err := te.loadTemplates(); err != nil {
		return nil, fmt.Errorf("template engine: failed to load templates: %w", err)
	}

	return te, nil
}

// loadTemplates loads all embedded templates from disk.
func (te *TemplateEngine) loadTemplates() error {
	var allTemplatePaths []string

	// Helper to collect templates from a directory
	collectTemplates := func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
				allTemplatePaths = append(allTemplatePaths, filepath.Join(dir, entry.Name()))
			}
		}
		return nil
	}

	// Collect root-level templates
	if err := collectTemplates(te.basePath); err != nil {
		return fmt.Errorf("collect root: %w", err)
	}

	// Collect component templates
	if err := collectTemplates(filepath.Join(te.basePath, "components")); err != nil {
		logging.Warn("Failed to collect components", "error", err)
	}

	// Collect phase templates
	if err := collectTemplates(filepath.Join(te.basePath, "phases")); err != nil {
		logging.Warn("Failed to collect phases", "error", err)
	}

	if len(allTemplatePaths) == 0 {
		return fmt.Errorf("no templates found in %s", te.basePath)
	}

	// Parse all templates together
	funcs := te.funcMap()
	tmpl, err := template.New("").Funcs(funcs).ParseFiles(allTemplatePaths...)
	if err != nil {
		return fmt.Errorf("parse all templates: %w", err)
	}

	te.mu.Lock()
	defer te.mu.Unlock()
	for _, t := range tmpl.Templates() {
		if t != nil {
			name := t.Name()
			// Only store the base name as the template name for easy lookup
			// if it's a full path, but ParseFiles already uses base names.
			te.templates[name] = t
		}
	}

	return nil
}

// funcMap returns the custom template functions.
func (te *TemplateEngine) funcMap() template.FuncMap {
	return template.FuncMap{
		"lower": lower,
		"title": title,
		"add":   func(a, b int) int { return a + b },
		"sub":   func(a, b int) int { return a - b },
		"div":   func(a, b int) int { return a / b },
		"now": func() string {
			return time.Now().Format("15:04:05")
		},
		"getStats": func(entries []ActivityEntry) map[string]int {
			stats := map[string]int{"success": 0, "error": 0, "running": 0}
			for _, e := range entries {
				switch e.Status {
				case "success":
					stats["success"]++
				case "error":
					stats["error"]++
				case "running":
					stats["running"]++
				}
			}
			return stats
		},
		"logTypeClass": func(t string) string {
			switch t {
			case "llm":
				return "type-llm"
			case "file":
				return "type-file"
			case "test":
				return "type-test"
			case "review":
				return "type-review"
			case "system":
				return "type-system"
			default:
				return ""
			}
		},
		"logStatusClass": func(s string) string {
			switch s {
			case "error":
				return "status-error"
			case "running":
				return "status-running"
			default:
				return ""
			}
		},
		"typeBadgeClass": func(t string) string {
			switch t {
			case "llm":
				return "badge-llm"
			case "file":
				return "badge-file"
			case "test":
				return "badge-test"
			case "review":
				return "badge-review"
			case "system":
				return "badge-system"
			default:
				return ""
			}
		},
		"phaseState": func(s string) string {
			switch s {
			case "completed":
				return "completed"
			case "failed":
				return "failed"
			case "in_progress":
				return "in-progress"
			case "pending":
				return "waiting"
			default:
				return "pending"
			}
		},
		"phaseClass": func(s string) string {
			switch s {
			case "completed":
				return "border-green-500 text-green-500"
			case "failed":
				return "border-red-500 text-red-500"
			case "in-progress":
				return "border-blue-500 text-blue-500 ring-2 ring-blue-500/30"
			case "waiting":
				return "border-yellow-500 text-yellow-500"
			default:
				return "border-dark-600 text-dark-600"
			}
		},
		"phaseLabelClass": func(s string) string {
			switch s {
			case "completed":
				return "text-green-500"
			case "failed":
				return "text-red-500"
			case "in-progress":
				return "text-blue-500 font-bold"
			case "waiting":
				return "text-yellow-500"
			default:
				return "text-dark-600"
			}
		},
		"phaseConnectorClass": func(s string) string {
			switch s {
			case "completed":
				return "border-green-500"
			case "failed":
				return "border-red-500"
			case "in-progress":
				return "border-blue-500"
			case "waiting":
				return "border-yellow-500"
			default:
				return "border-dark-600"
			}
		},
		"phaseProgress": func(s string) int {
			switch s {
			case "completed":
				return 100
			case "in-progress":
				return 75
			case "waiting":
				return 25
			default:
				return 0
			}
		},
		"currentPhaseDotClass": func(s string) string {
			switch s {
			case "completed":
				return "bg-green-500"
			case "failed":
				return "bg-red-500"
			case "in-progress":
				return "bg-blue-500 animate-pulse"
			case "waiting":
				return "bg-yellow-500"
			default:
				return "bg-dark-600"
			}
		},
		"priorityClass": func(p string) string {
			switch p {
			case "high":
				return "priority-high"
			case "medium":
				return "priority-medium"
			case "low":
				return "priority-low"
			default:
				return ""
			}
		},
		"fileExtension": func(name string) string {
			for i := len(name) - 1; i >= 0; i-- {
				if name[i] == '.' {
					return name[i+1:]
				}
			}
			return ""
		},
		"fileIcon": func(ext string) string {
			switch ext {
			case "ts", "tsx":
				return "typescript"
			case "js", "jsx":
				return "javascript"
			case "go":
				return "go"
			case "py":
				return "python"
			case "rs":
				return "rust"
			case "java":
				return "java"
			case "kt":
				return "kotlin"
			case "html":
				return "html"
			case "css":
				return "css"
			case "md":
				return "markdown"
			case "yaml", "yml":
				return "yaml"
			case "json":
				return "json"
			case "sh", "bash":
				return "shell"
			case "gitignore", "gitattributes":
				return "git"
			default:
				return "file"
			}
		},
		"codeLines": func(code string) []string {
			if code == "" {
				return []string{}
			}
			return strings.Split(code, "\n")
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict: invalid number of values")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict: keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"agentIconClass": func(name string) string {
			switch name {
			case "planner":
				return "bg-blue-500/20 text-blue-400"
			case "coder":
				return "bg-green-500/20 text-green-400"
			case "tester":
				return "bg-yellow-500/20 text-yellow-400"
			case "reviewer":
				return "bg-purple-500/20 text-purple-400"
			default:
				return "bg-dark-600 text-text-secondary"
			}
		},
		"agentIcon": func(name string) string {
			switch name {
			case "planner":
				return `<svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M2 9.5A3.5 3.5 0 005.5 13H9v2.586l-1.293-1.293a1 1 0 00-1.414 1.414l3 3a1 1 0 001.414 0l3-3a1 1 0 00-1.414-1.414L11 15.586V13h2.5a4.5 4.5 0 10-.616-8.958 4.002 4.002 0 10-7.753 1.977A3.5 3.5 0 002 9.5zm9 3.5H9V8a1 1 0 012 0v5z" clip-rule="evenodd"/></svg>`
			case "coder":
				return `<svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M13.5 2a2 2 0 012 2v12a2 2 0 01-2 2h-7a2 2 0 01-2-2V4a2 2 0 012-2h7zm0 2h-7v12h7V4zM7 7a.5.5 0 01.5-.5h5a.5.5 0 010 1h-5A.5.5 0 017 7zm0 2.5a.5.5 0 01.5-.5h5a.5.5 0 010 1h-5a.5.5 0 01-.5-.5zm0 2.5a.5.5 0 01.5-.5h3a.5.5 0 010 1h-3a.5.5 0 01-.5-.5z" clip-rule="evenodd"/></svg>`
			case "tester":
				return `<svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M15.312 11.424a5.5 5.5 0 01-9.201 2.466l-.312-.311h2.433a.5.5 0 000-1H3.999A.5.5 0 003.5 15v3a.5.5 0 00.5.5h3a.5.5 0 00.5-.5v-2.433l.311.312a7.5 7.5 0 0012.548-3.362.5.5 0 00-.049-.586l-.002-.001zM10 4a6 6 0 100 12A6 6 0 0010 4z" clip-rule="evenodd"/></svg>`
			case "reviewer":
				return `<svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M10 12.5a2.5 2.5 0 100-5 2.5 2.5 0 000 5z"/><path fill-rule="evenodd" d="M.664 10.59a1.651 1.651 0 010-1.186A10.004 10.004 0 0110 3c4.257 0 7.893 2.66 9.336 6.41.147.381.146.804 0 1.186A10.004 10.004 0 0110 17c-4.257 0-7.893-2.66-9.336-6.41zM14 10a4 4 0 11-8 0 4 4 0 018 0z" clip-rule="evenodd"/></svg>`
			default:
				return `<svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" clip-rule="evenodd"/></svg>`
			}
		},
		"humanName": func(name string) string {
			switch name {
			case "planner":
				return "Planner"
			case "coder":
				return "Coder"
			case "tester":
				return "Tester"
			case "reviewer":
				return "Reviewer"
			default:
				return title(name)
			}
		},
		"categoryBadgeClass": func(cat string) string {
			switch cat {
			case "knowledge":
				return "badge-knowledge"
			case "tool":
				return "badge-tool"
			default:
				return "badge-default"
			}
		},
		"curlyOpen":  func() string { return "{{" },
		"curlyClose": func() string { return "}}" },
		"default": func(val, fallback interface{}) interface{} {
			if val == nil {
				return fallback
			}
			return val
		},
		"scoreColor": func(p int) string {
			if p >= 80 {
				return "#22c55e"
			}
			if p >= 60 {
				return "#eab308"
			}
			if p >= 40 {
				return "#f97316"
			}
			return "#ef4444"
		},
		"totalIssues": func(issues []interface{}) int { return len(issues) },
		"countBySeverity": func(issues []interface{}, severity string) int {
			count := 0
			for _, issue := range issues {
				if m, ok := issue.(map[string]interface{}); ok {
					if s, ok := m["severity"].(string); ok && s == severity {
						count++
					}
				}
			}
			return count
		},
		"filterBySeverity": func(issues []interface{}, severity string) []interface{} {
			result := []interface{}{}
			for _, issue := range issues {
				if m, ok := issue.(map[string]interface{}); ok {
					if s, ok := m["severity"].(string); ok && s == severity {
						result = append(result, issue)
					}
				}
			}
			return result
		},
		"isSkillAssigned": func(skillName string, skills []string) bool {
			for _, s := range skills {
				if s == skillName {
					return true
				}
			}
			return false
		},
		"phaseBorderClass": func(s string) string {
			switch s {
			case "completed":
				return "border-green-500"
			case "failed":
				return "border-red-500"
			case "in-progress":
				return "border-blue-500"
			case "waiting":
				return "border-yellow-500"
			default:
				return "border-dark-600"
			}
		},
		"phaseDotClass": func(s string) string {
			switch s {
			case "completed":
				return "bg-green-500"
			case "failed":
				return "bg-red-500"
			case "in-progress":
				return "bg-blue-500 animate-pulse"
			case "waiting":
				return "bg-yellow-500"
			default:
				return "bg-dark-600"
			}
		},
		"phaseProgressClass": func(s string) string {
			switch s {
			case "completed":
				return "bg-green-500"
			case "failed":
				return "bg-red-500"
			case "in-progress":
				return "bg-blue-500"
			case "waiting":
				return "bg-yellow-500"
			default:
				return "bg-dark-600"
			}
		},
		"phaseTextClass": func(s string) string {
			switch s {
			case "completed":
				return "text-green-500"
			case "failed":
				return "text-red-500"
			case "in-progress":
				return "text-blue-500 font-bold"
			case "waiting":
				return "text-yellow-500"
			default:
				return "text-text-secondary"
			}
		},
		"coverageColor": func(p int) string {
			if p >= 80 {
				return "#22c55e"
			}
			if p >= 60 {
				return "#eab308"
			}
			if p >= 40 {
				return "#f97316"
			}
			return "#ef4444"
		},
	}
}

// RenderPhasePartial renders a phase partial for the given phase name.
func (te *TemplateEngine) RenderPhasePartial(phase string, data PhaseRenderData) (string, error) {
	phaseKey := fmt.Sprintf("%s-phase", lower(phase))

	te.mu.RLock()
	tmpl, ok := te.templates[phaseKey]
	te.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("phase: unknown phase %q", phase)
	}

	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, phaseKey, data); err != nil {
		return "", fmt.Errorf("phase: failed to render %q: %w", phaseKey, err)
	}

	return buf.String(), nil
}

// RenderComponentPartial renders a component partial by name.
func (te *TemplateEngine) RenderComponentPartial(name string, data interface{}) (string, error) {
	te.mu.RLock()
	tmpl, ok := te.templates[name]
	te.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("component: template %q not found", name)
	}

	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("component: failed to render %q: %w", name, err)
	}

	return buf.String(), nil
}

// RenderMain renders the main page with the given content template and data.
func (te *TemplateEngine) RenderMain(w http.ResponseWriter, contentTemplate string, data map[string]interface{}) {
	te.mu.RLock()
	baseTmpl, ok := te.templates["base"]
	te.mu.RUnlock()

	if !ok {
		http.Error(w, "base template not found", http.StatusInternalServerError)
		return
	}

	if data == nil {
		data = make(map[string]interface{})
	}
	data["Content"] = contentTemplate

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := baseTmpl.ExecuteTemplate(w, "base", data); err != nil {
		logging.Error("Error rendering page", "error", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

// ─── Handler ──────────────────────────────────────────────────────────────────

// HTMXRenderHandler manages HTTP handlers for HTMX partial rendering.
type HTMXRenderHandler struct {
	// templateEngine provides template rendering capabilities.
	templateEngine *TemplateEngine
	// sessionStore provides access to session data.
	sessionStore interface {
		ListSessions() []*state.Session
	}
	// projectStore provides access to project data.
	projectStore *ProjectStore
}

// NewHTMXRenderHandler creates a new HTMXRenderHandler instance.
func NewHTMXRenderHandler(
	templateEngine *TemplateEngine,
	sessionStore interface {
		ListSessions() []*state.Session
	},
	projectStore *ProjectStore,
) *HTMXRenderHandler {
	return &HTMXRenderHandler{
		templateEngine: templateEngine,
		sessionStore:   sessionStore,
		projectStore:   projectStore,
	}
}

// RenderPhase handles GET /api/render/phase/:phase
// Renders the HTML partial for the specified phase.
func (h *HTMXRenderHandler) RenderPhase(w http.ResponseWriter, r *http.Request) {
	phase := extractPhaseFromPath(r.URL.Path)
	if phase == "" {
		api.WriteAppError(w, apperrors.BadRequest("phase is required", "A phase name must be provided.", nil))
		return
	}

	// Get session data if available
	var data PhaseRenderData
	sessions := h.sessionStore.ListSessions()
	if len(sessions) > 0 {
		session := sessions[0]
		data.GoalDescription = session.Goal
		data.TotalPhases = 6 // planning, planning_review, coding, testing, review, human_review

		// Get current step based on current phase
		data.CurrentStep = getPhaseStep(phase)

		// Build planning steps
		data.PlanningSteps = []PhaseStep{
			{Name: "Analyze requirements", Description: "Understand project scope and goals"},
			{Name: "Break down tasks", Description: "Create atomic units of work"},
			{Name: "Generate execution plan", Description: "Order tasks and define dependencies"},
		}

		// Check if planning is complete
		if session.CurrentPhase == state.PhasePlanning && session.Status == state.SessionStatusRunning {
			data.StatusMessage = "Analyzing project context and generating execution plan"
			data.EstimatedTime = "2-5 minutes"
		} else if session.Status == state.SessionStatusCompleted {
			data.StatusMessage = "Planning phase completed"
			data.CanCancel = false
		} else {
			data.StatusMessage = "Planning in progress"
			data.CanCancel = true
		}

		// Add subtasks if plan exists
		if session.Plan != nil {
			for _, unit := range session.Plan.Units {
				data.Subtasks = append(data.Subtasks, PhaseSubtask{
					Name:     unit.Name,
					Priority: "medium",
				})
			}
		}
	}

	rendered, err := h.templateEngine.RenderPhasePartial(phase, data)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the phase component.", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// RenderFileTree handles GET /api/render/file-tree
// Renders the HTML partial for the file tree explorer.
func (h *HTMXRenderHandler) RenderFileTree(w http.ResponseWriter, r *http.Request) {
	// Get project data
	projects := h.projectStore.ListProjects()
	if len(projects) == 0 {
		api.WriteAppError(w, apperrors.BadRequest("no project opened", "No project is currently open.", nil))
		return
	}

	project := projects[0]

	// List files from the project store
	entries, err := h.projectStore.ListProjectFiles(project.ID, "")
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("file listing failed", "Failed to list project files.", err))
		return
	}

	// Convert entries to file system items
	items := make([]FileSystemItem, 0, len(entries))
	for _, entry := range entries {
		item := FileSystemItem{
			Name:  entry.Name,
			Path:  entry.Path,
			IsDir: entry.IsDir,
			Size:  entry.Size,
		}
		items = append(items, item)
	}

	data := FileTreeRenderData{
		RootItems:   items,
		CurrentPath: "",
		ProjectPath: project.Path,
	}

	rendered, err := h.templateEngine.RenderComponentPartial("file-tree", data)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the file tree component.", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// RenderActivityLog handles GET /api/render/activity-log
// Renders the HTML partial for the activity log.
func (h *HTMXRenderHandler) RenderActivityLog(w http.ResponseWriter, r *http.Request) {
	// Get session data
	sessions := h.sessionStore.ListSessions()
	if len(sessions) == 0 {
		api.WriteAppError(w, apperrors.BadRequest("no active session", "No active session found. Please start a new session.", nil))
		return
	}

	session := sessions[0]

	// Build activity entries from session history
	entries := make([]ActivityEntry, 0, len(session.History))
	for _, hist := range session.History {
		entries = append(entries, ActivityEntry{
			Phase:     hist.Phase,
			Type:      "system",
			Status:    string(hist.Status),
			Timestamp: hist.StartedAt.Format("15:04:05"),
			Action:    fmt.Sprintf("Phase %s %s", hist.Phase, hist.Status),
		})
	}

	// Get unique phases
	phases := []string{"planning", "coding", "testing", "review"}
	for _, hist := range session.History {
		found := false
		for _, p := range phases {
			if hist.Phase == p {
				found = true
				break
			}
		}
		if !found {
			phases = append(phases, hist.Phase)
		}
	}

	data := ActivityLogRenderData{
		Entries:    entries,
		Phases:     phases,
		AutoScroll: true,
	}

	rendered, err := h.templateEngine.RenderComponentPartial("activity-log", data)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the activity log component.", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// RenderPhaseTracker handles GET /api/render/phase-tracker
// Renders the HTML partial for the phase tracker.
func (h *HTMXRenderHandler) RenderPhaseTracker(w http.ResponseWriter, r *http.Request) {
	// Get session data
	sessions := h.sessionStore.ListSessions()
	if len(sessions) == 0 {
		api.WriteAppError(w, apperrors.BadRequest("no active session", "No active session found. Please start a new session.", nil))
		return
	}

	session := sessions[0]

	// Build phase tracker items
	allPhases := []PhaseTrackerItem{
		{Name: "Planning", Status: "pending", MaxRetries: 3},
		{Name: "Planning Review", Status: "pending", MaxRetries: 3},
		{Name: "Coding", Status: "pending", MaxRetries: 3},
		{Name: "Testing", Status: "pending", MaxRetries: 3},
		{Name: "Review", Status: "pending", MaxRetries: 3},
		{Name: "Human Review", Status: "pending", MaxRetries: 3},
	}

	// Update statuses based on session state
	for i := range allPhases {
		switch session.CurrentPhase {
		case state.PhasePlanning:
			if i == 0 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhasePlanningReview:
			if i == 0 {
				allPhases[i].Status = "completed"
			}
			if i == 1 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseCoding:
			if i < 2 {
				allPhases[i].Status = "completed"
			}
			if i == 2 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseTesting:
			if i < 3 {
				allPhases[i].Status = "completed"
			}
			if i == 3 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseReview:
			if i < 4 {
				allPhases[i].Status = "completed"
			}
			if i == 4 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseHumanReview:
			if i < 5 {
				allPhases[i].Status = "completed"
			}
			if i == 5 {
				allPhases[i].Status = "in_progress"
			}
		}

		// Mark future phases as pending
		if session.Status == state.SessionStatusCompleted {
			allPhases[i].Status = "completed"
		}
	}

	// Get current phase info
	currentPhaseName, currentPhaseStatus := getCurrentPhaseInfo(session)

	data := PhaseTrackerRenderData{
		Phases:              allPhases,
		CurrentPhaseName:    currentPhaseName,
		CurrentPhaseStatus:  currentPhaseStatus,
		CurrentPhaseMessage: fmt.Sprintf("Session: %s", session.Goal),
	}

	rendered, err := h.templateEngine.RenderComponentPartial("phase-tracker", data)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the phase tracker component.", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// ─── Helper Functions ─────────────────────────────────────────────────────────

// extractPhaseFromPath extracts the phase name from the URL path.
// Expected format: /api/render/phase/{phase}
func extractPhaseFromPath(path string) string {
	parts := api.SplitPath(path)
	if len(parts) < 4 {
		return ""
	}
	// parts: ["", "api", "render", "phase", "{phase}"]
	return parts[4]
}

// getPhaseStep returns the current step number for a phase.
func getPhaseStep(phase string) int {
	switch lower(phase) {
	case "planning":
		return 0
	case "planning_review":
		return 1
	case "coding":
		return 2
	case "testing":
		return 3
	case "review":
		return 4
	case "human_review":
		return 5
	default:
		return 0
	}
}

// getCurrentPhaseInfo returns the current phase name and status.
func getCurrentPhaseInfo(session *state.Session) (string, string) {
	switch session.CurrentPhase {
	case state.PhasePlanning:
		return "Planning", "in_progress"
	case state.PhasePlanningReview:
		return "Planning Review", "in_progress"
	case state.PhaseCoding:
		return "Coding", "in_progress"
	case state.PhaseTesting:
		return "Testing", "in_progress"
	case state.PhaseReview:
		return "Review", "in_progress"
	case state.PhaseHumanReview:
		return "Human Review", "in_progress"
	default:
		return "Unknown", "pending"
	}
}

// RenderMainPage renders the main IDE page.
func (h *HTMXRenderHandler) RenderMainPage(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})

	// Get session data
	sessions := h.sessionStore.ListSessions()
	if len(sessions) > 0 {
		session := sessions[0]
		data["SessionID"] = session.ID
		data["Goal"] = session.Goal
		data["SessionStatus"] = string(session.Status)

		currentPhaseName, currentPhaseStatus := getCurrentPhaseInfo(session)
		data["CurrentPhaseName"] = currentPhaseName
		data["CurrentPhaseStatus"] = currentPhaseStatus
		data["PhaseProgress"] = getPhaseProgress(session.Status)
	} else {
		data["SessionID"] = "no-active-session"
		data["CurrentPhaseName"] = "Idle"
		data["CurrentPhaseStatus"] = "pending"
		data["PhaseProgress"] = 0
	}

	// Get project data
	projects := h.projectStore.ListProjects()
	if len(projects) > 0 {
		project := projects[0]
		data["ProjectName"] = project.Name
		data["ProjectPath"] = project.Path

		// List initial files for the tree
		entries, err := h.projectStore.ListProjectFiles(project.ID, "")
		if err == nil {
			items := make([]FileSystemItem, 0, len(entries))
			for _, entry := range entries {
				items = append(items, FileSystemItem{
					Name:  entry.Name,
					Path:  entry.Path,
					IsDir: entry.IsDir,
					Size:  entry.Size,
				})
			}
			data["RootItems"] = items
		}
	} else {
		data["ProjectName"] = "No Project"
		data["ProjectPath"] = "Please open or create a project"
		data["RootItems"] = []FileSystemItem{}
	}
	data["CurrentPath"] = ""

	// Placeholder for model info
	data["ModelName"] = "gpt-4o"
	data["ProviderName"] = "OpenAI"

	h.templateEngine.RenderMain(w, "ide", data)
}

// getPhaseProgress returns a progress percentage based on session status.
func getPhaseProgress(status state.SessionStatus) int {
	switch status {
	case state.SessionStatusCompleted:
		return 100
	case state.SessionStatusRunning:
		return 45
	case state.SessionStatusPending:
		return 0
	default:
		return 0
	}
}

// lower converts a string to lowercase.
func lower(s string) string {
	return strings.ToLower(s)
}

// title capitalizes the first letter of a string.
func title(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// ─── JSON Response Types ─────────────────────────────────────────────────────

// RenderErrorResponse represents an error response for render endpoints.
type RenderErrorResponse struct {
	Error string `json:"error"`
}

// RenderStatusResponse represents the status response for render endpoints.
type RenderStatusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
