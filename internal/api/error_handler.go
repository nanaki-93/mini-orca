package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
)

// ErrorHandler provides HTTP error handling with template-based error pages.
type ErrorHandler struct {
	templatesPath  string
	templates      map[string]*template.Template
	errorIDCounter int
}

// ErrorPageData holds data for rendering error pages.
type ErrorPageData struct {
	ErrorID     string
	Timestamp   string
	Path        string
	UserMessage string
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

		// Handle error status codes only if the handler didn't write body content,
		// OR if the body contains a JSON error that we want to render as HTML.
		if rw.status >= 400 {
			// If it's a browser request (not API) or an HTMX request, we might want to render HTML
			if (isHTMXRequest(r) || !isAPIRequest(r)) && rw.bodyWritten {
				// Try to extract user message from JSON body
				userMsg := eh.extractUserMessage(rw.body)
				if userMsg != "" {
					eh.handleError(w, r, rw.status, userMsg)
					return
				}
			}

			if !rw.bodyWritten {
				eh.handleError(w, r, rw.status, "")
				return
			}
		}

		// Flush the buffered body
		w.WriteHeader(rw.status)
		w.Write(rw.body)
	})
}

// isAPIRequest checks if the request is for an API endpoint.
func isAPIRequest(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/api/render/")
}

// extractUserMessage attempts to extract a user-friendly message from a JSON error body.
func (eh *ErrorHandler) extractUserMessage(body []byte) string {
	var appErr apperrors.Error
	if err := json.Unmarshal(body, &appErr); err == nil && appErr.UserMessage != "" {
		return appErr.UserMessage
	}
	return ""
}

// handleError dispatches to the appropriate error handler based on status code.
func (eh *ErrorHandler) handleError(w http.ResponseWriter, r *http.Request, status int, userMsg string) {
	switch status {
	case http.StatusNotFound:
		eh.handleNotFound(w, r, userMsg)
	case http.StatusInternalServerError:
		eh.handleInternalServerError(w, r, userMsg)
	case http.StatusMethodNotAllowed:
		eh.handleMethodNotAllowed(w, r)
	case http.StatusBadRequest:
		eh.handleBadRequest(w, r, userMsg)
	case http.StatusUnauthorized:
		eh.handleUnauthorized(w, r, userMsg)
	default:
		eh.handleGenericError(w, r, status, userMsg)
	}
}

// handleNotFound renders the 404 error page.
func (eh *ErrorHandler) handleNotFound(w http.ResponseWriter, r *http.Request, userMsg string) {
	if userMsg == "" {
		userMsg = "The requested resource was not found."
	}
	// Check if this is an HTMX request
	if isHTMXRequest(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
			<div class="error-fallback p-6 text-center">
				<svg class="w-12 h-12 text-error mx-auto mb-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/>
				</svg>
				<h3 class="text-lg font-medium text-text-primary mb-2">Page Not Found</h3>
				<p class="text-sm text-text-secondary mb-4">%s</p>
				<button class="px-4 py-2 text-sm bg-dark-700 hover:bg-dark-600 text-text-primary rounded-md transition-colors"
						onclick="location.reload()">
					Refresh Page
				</button>
			</div>
		`, userMsg)
		return
	}

	// Render full error page
	errorID := eh.generateErrorID()
	data := ErrorPageData{
		ErrorID:     errorID,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
		Path:        r.URL.Path,
		UserMessage: userMsg,
	}

	if tmpl, ok := eh.templates["404.html"]; ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		if err := tmpl.ExecuteTemplate(w, "404.html", data); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, "404 - Page Not Found")
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "404 - Page Not Found")
	}
}

// handleInternalServerError renders the 500 error page.
func (eh *ErrorHandler) handleInternalServerError(w http.ResponseWriter, r *http.Request, userMsg string) {
	if userMsg == "" {
		userMsg = "An internal server error occurred. Please try again later."
	}
	// Check if this is an HTMX request
	if isHTMXRequest(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
			<div class="error-fallback p-6 text-center">
				<svg class="w-12 h-12 text-error mx-auto mb-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"/>
				</svg>
				<h3 class="text-lg font-medium text-text-primary mb-2">Server Error</h3>
				<p class="text-sm text-text-secondary mb-4">%s</p>
				<button class="px-4 py-2 text-sm bg-dark-700 hover:bg-dark-600 text-text-primary rounded-md transition-colors"
						onclick="location.reload()">
					Refresh Page
				</button>
			</div>
		`, userMsg)
		return
	}

	// Render full error page
	errorID := eh.generateErrorID()
	data := ErrorPageData{
		ErrorID:     errorID,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
		Path:        r.URL.Path,
		UserMessage: userMsg,
	}

	if tmpl, ok := eh.templates["500.html"]; ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		if err := tmpl.ExecuteTemplate(w, "500.html", data); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, "500 - Internal Server Error")
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "500 - Internal Server Error")
	}
}

// handleBadRequest renders a 400 error response.
func (eh *ErrorHandler) handleBadRequest(w http.ResponseWriter, r *http.Request, userMsg string) {
	if userMsg == "" {
		userMsg = "The request was invalid."
	}
	if isHTMXRequest(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
			<div class="error-fallback p-6 text-center">
				<svg class="w-12 h-12 text-warning mx-auto mb-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
				</svg>
				<h3 class="text-lg font-medium text-text-primary mb-2">Invalid Request</h3>
				<p class="text-sm text-text-secondary mb-4">%s</p>
			</div>
		`, userMsg)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, "<h1>400 Bad Request</h1><p>%s</p>", userMsg)
}

// handleUnauthorized renders a 401 error response.
func (eh *ErrorHandler) handleUnauthorized(w http.ResponseWriter, r *http.Request, userMsg string) {
	if userMsg == "" {
		userMsg = "Authentication is required."
	}
	if isHTMXRequest(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
			<div class="error-fallback p-6 text-center">
				<svg class="w-12 h-12 text-warning mx-auto mb-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clip-rule="evenodd"/>
				</svg>
				<h3 class="text-lg font-medium text-text-primary mb-2">Unauthorized</h3>
				<p class="text-sm text-text-secondary mb-4">%s</p>
				<button class="px-4 py-2 text-sm bg-primary-600 hover:bg-primary-700 text-white rounded-md transition-colors"
						onclick="location.href='/login'">
					Log In
				</button>
			</div>
		`, userMsg)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, "<h1>401 Unauthorized</h1><p>%s</p>", userMsg)
}

// handleGenericError renders a generic error response.
func (eh *ErrorHandler) handleGenericError(w http.ResponseWriter, r *http.Request, status int, userMsg string) {
	if userMsg == "" {
		userMsg = fmt.Sprintf("An error occurred (Status %d).", status)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, "<h1>Error %d</h1><p>%s</p>", status, userMsg)
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
