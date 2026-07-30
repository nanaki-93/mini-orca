package api

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ErrorHandler provides HTTP error handling with template-based error pages.
type ErrorHandler struct {
	templatesPath  string
	templates      map[string]*template.Template
	errorIDCounter int
}

// ErrorPageData holds data for rendering error pages.
type ErrorPageData struct {
	ErrorID   string
	Timestamp string
	Path      string
}

// NewErrorHandler creates a new ErrorHandler instance.
func NewErrorHandler(templatesPath string) (*ErrorHandler, error) {
	eh := &ErrorHandler{
		templatesPath: templatesPath,
		templates:     make(map[string]*template.Template),
	}

	if err := eh.loadTemplates(); err != nil {
		return nil, fmt.Errorf("error handler: failed to load templates: %w", err)
	}

	return eh, nil
}

// loadTemplates loads error page templates.
func (eh *ErrorHandler) loadTemplates() error {
	templatesDir := filepath.Join(eh.templatesPath, "errors")

	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		// Templates directory doesn't exist, that's okay
		return nil
	}

	var templatePaths []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".html" {
			templatePaths = append(templatePaths, filepath.Join(templatesDir, entry.Name()))
		}
	}

	if len(templatePaths) == 0 {
		return nil
	}

	funcs := template.FuncMap{
		"default": func(v interface{}, fallback string) interface{} {
			if v == nil {
				return fallback
			}
			return v
		},
	}

	tmpls, err := template.New("").Funcs(funcs).ParseFiles(templatePaths...)
	if err != nil {
		return fmt.Errorf("parse error templates: %w", err)
	}

	for _, t := range tmpls.Templates() {
		if t != nil {
			eh.templates[t.Name()] = t
		}
	}

	return nil
}

// generateErrorID creates a unique error identifier.
func (eh *ErrorHandler) generateErrorID() string {
	eh.errorIDCounter++
	return fmt.Sprintf("err-%d-%d", time.Now().Unix(), eh.errorIDCounter)
}

// Next returns an http.Handler that wraps the provided handler with error handling.
func (eh *ErrorHandler) Next(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a custom response writer to capture status codes and buffer body
		rw := &errorResponseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		// Serve the request
		next.ServeHTTP(rw, r)

		// Handle error status codes only if the handler didn't write body content.
		// If the handler wrote a body, flush it to the client.
		switch rw.status {
		case http.StatusNotFound:
			if !rw.bodyWritten {
				eh.handleNotFound(w, r)
			} else {
				w.WriteHeader(rw.status)
				w.Write(rw.body)
			}
		case http.StatusInternalServerError:
			if !rw.bodyWritten {
				eh.handleInternalServerError(w, r)
			} else {
				w.WriteHeader(rw.status)
				w.Write(rw.body)
			}
		case http.StatusMethodNotAllowed:
			if !rw.bodyWritten {
				eh.handleMethodNotAllowed(w, r)
			} else {
				w.WriteHeader(rw.status)
				w.Write(rw.body)
			}
		default:
			// Success or other unhandled response — flush the buffered body
			w.WriteHeader(rw.status)
			w.Write(rw.body)
		}
	})
}

// handleNotFound renders the 404 error page.
func (eh *ErrorHandler) handleNotFound(w http.ResponseWriter, r *http.Request) {
	// Check if this is an HTMX request
	if isHTMXRequest(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
			<div class="error-fallback p-6 text-center">
				<svg class="w-12 h-12 text-error mx-auto mb-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/>
				</svg>
				<h3 class="text-lg font-medium text-text-primary mb-2">Page Not Found</h3>
				<p class="text-sm text-text-secondary mb-4">The requested resource was not found.</p>
				<button class="px-4 py-2 text-sm bg-dark-700 hover:bg-dark-600 text-text-primary rounded-md transition-colors"
						onclick="location.reload()">
					Refresh Page
				</button>
			</div>
		`)
		return
	}

	// Render full error page
	errorID := eh.generateErrorID()
	data := ErrorPageData{
		ErrorID:   errorID,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Path:      r.URL.Path,
	}

	if tmpl, ok := eh.templates["404.html"]; ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "404.html", data); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, "404 - Page Not Found")
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "404 - Page Not Found")
	}
}

// handleInternalServerError renders the 500 error page.
func (eh *ErrorHandler) handleInternalServerError(w http.ResponseWriter, r *http.Request) {
	// Check if this is an HTMX request
	if isHTMXRequest(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
			<div class="error-fallback p-6 text-center">
				<svg class="w-12 h-12 text-error mx-auto mb-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"/>
				</svg>
				<h3 class="text-lg font-medium text-text-primary mb-2">Server Error</h3>
				<p class="text-sm text-text-secondary mb-4">An internal server error occurred. Please try again later.</p>
				<button class="px-4 py-2 text-sm bg-dark-700 hover:bg-dark-600 text-text-primary rounded-md transition-colors"
						onclick="location.reload()">
					Refresh Page
				</button>
			</div>
		`)
		return
	}

	// Render full error page
	errorID := eh.generateErrorID()
	data := ErrorPageData{
		ErrorID:   errorID,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Path:      r.URL.Path,
	}

	if tmpl, ok := eh.templates["500.html"]; ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "500.html", data); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, "500 - Internal Server Error")
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "500 - Internal Server Error")
	}
}

// handleMethodNotAllowed renders a 405 error page.
func (eh *ErrorHandler) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	if isHTMXRequest(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
			<div class="error-fallback p-6 text-center">
				<svg class="w-12 h-12 text-warning mx-auto mb-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
				</svg>
				<h3 class="text-lg font-medium text-text-primary mb-2">Method Not Allowed</h3>
				<p class="text-sm text-text-secondary mb-4">The HTTP method is not supported for this endpoint.</p>
				<button class="px-4 py-2 text-sm bg-dark-700 hover:bg-dark-600 text-text-primary rounded-md transition-colors"
						onclick="history.back()">
					Go Back
				</button>
			</div>
		`)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<h1>405 Method Not Allowed</h1><p>The method %s is not allowed for this endpoint.</p>`, r.Method)
}

// errorResponseWriter wraps http.ResponseWriter to capture status codes
// and buffer body content, allowing the ErrorHandler to intercept error responses.
type errorResponseWriter struct {
	http.ResponseWriter
	status        int
	headerWritten bool
	bodyWritten   bool
	body          []byte
}

// WriteHeader captures the status code without writing it to the underlying writer.
func (ew *errorResponseWriter) WriteHeader(status int) {
	if !ew.headerWritten {
		ew.headerWritten = true
		ew.status = status
	}
}

// Write captures body content in a buffer instead of writing it immediately.
func (ew *errorResponseWriter) Write(b []byte) (int, error) {
	if !ew.headerWritten {
		ew.headerWritten = true
		ew.status = http.StatusOK
	}
	ew.bodyWritten = true
	ew.body = append(ew.body, b...)
	return len(b), nil
}

// isHTMXRequest checks if the request is an HTMX request.
func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
