package templates

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
)

//go:embed all:templates
var templateFS embed.FS

//go:embed all:static
var StaticFS embed.FS

// TemplateSet holds all parsed templates.
type TemplateSet struct {
	Base       *template.Template
	IDE        *template.Template
	Components *template.Template
	Phases     *template.Template
	Editors    *template.Template
}

// LoadTemplates loads all templates from the embedded filesystem.
func LoadTemplates() (*TemplateSet, error) {
	ts := &TemplateSet{}

	// Load base template
	base, err := template.ParseFS(templateFS, "templates/base.html")
	if err != nil {
		return nil, err
	}
	ts.Base = base

	// Load IDE template
	ide, err := template.ParseFS(templateFS, "templates/ide.html")
	if err != nil {
		return nil, err
	}
	ts.IDE = ide

	// Load components
	components, err := template.ParseFS(templateFS, "templates/components/*.html")
	if err != nil {
		return nil, err
	}
	ts.Components = components

	// Load phase templates
	phases, err := template.ParseFS(templateFS, "templates/phases/*.html")
	if err != nil {
		return nil, err
	}
	ts.Phases = phases

	// Load editor templates
	editors, err := template.ParseFS(templateFS, "templates/editors/*.html")
	if err != nil {
		return nil, err
	}
	ts.Editors = editors

	return ts, nil
}

// ServeHTTP serves static files.
func (ts *TemplateSet) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Serve static files
	if staticFile, err := StaticFS.Open(r.URL.Path[1:]); err == nil {
		defer staticFile.Close()
		http.ServeFile(w, r, r.URL.Path[1:])
		return
	}

	// Serve templates
	switch r.URL.Path {
	case "/ide":
		ts.IDE.Execute(w, nil)
	case "/":
		http.Redirect(w, r, "/ide", http.StatusFound)
	default:
		http.NotFound(w, r)
	}
}

// GetTemplate returns a specific template by name.
func (ts *TemplateSet) GetTemplate(name string) *template.Template {
	if t := ts.Base.Lookup(name); t != nil {
		return ts.Base
	}
	if t := ts.IDE.Lookup(name); t != nil {
		return ts.IDE
	}
	if t := ts.Components.Lookup(name); t != nil {
		return ts.Components
	}
	if t := ts.Phases.Lookup(name); t != nil {
		return ts.Phases
	}
	if t := ts.Editors.Lookup(name); t != nil {
		return ts.Editors
	}
	return nil
}

// ParsePartial parses a partial template from the embedded filesystem.
func ParsePartial(templateDir, filename string) (*template.Template, error) {
	return template.ParseFS(templateFS, filepath.Join("templates", templateDir, filename))
}

// WalkTemplates walks all embedded templates and returns their paths.
func WalkTemplates() ([]string, error) {
	var paths []string
	err := fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

// GetStaticFile returns a static file from the embedded filesystem.
func GetStaticFile(path string) (fs.File, error) {
	return StaticFS.Open(path)
}

// ListStaticFiles lists all static files.
func ListStaticFiles() ([]string, error) {
	var files []string
	err := fs.WalkDir(StaticFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
