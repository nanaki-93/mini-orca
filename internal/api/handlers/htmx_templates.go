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

	"github.com/nanaki-93/mini-orca/v2/internal/logging"
)

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
			te.templates[name] = t
		}
	}

	return nil
}

// funcMap returns the custom template functions.
func (te *TemplateEngine) funcMap() template.FuncMap {
	return template.FuncMap{
		"lower":         lower,
		"title":         title,
		"add":           func(a, b int) int { return a + b },
		"sub":           func(a, b int) int { return a - b },
		"mul":           func(a, b int) int { return a * b },
		"div":           func(a, b int) int { return a / b },
		"now":           func() string { return time.Now().Format("15:04:05") },
		"fileExtension": fileExtension,
		"fileIcon":      fileIcon,
		"codeLines": func(code string) []string {
			if code == "" {
				return []string{}
			}
			return strings.Split(code, "\n")
		},
		"dict":       dict,
		"curlyOpen":  func() string { return "{{" },
		"curlyClose": func() string { return "}}" },
		"default":    defFunc,
		"quote":      func(s string) string { return s },
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
