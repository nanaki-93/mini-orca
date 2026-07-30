package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ErrorResponse represents an error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON writes a JSON response to the HTTP writer.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError writes an error JSON response to the HTTP writer.
func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// SplitPath splits a URL path into its components.
func SplitPath(path string) []string {
	if path == "/" {
		return []string{""}
	}
	path = CleanPath(path)
	if path[0] == '/' {
		path = path[1:]
	}
	if path == "" {
		return []string{""}
	}
	return strings.Split(path, "/")
}

// CleanPath removes redundant slashes from the path.
func CleanPath(path string) string {
	if path == "" {
		return "/"
	}
	if path[0] != '/' {
		path = "/" + path
	}
	n := len(path)
	for i := 1; i < n-1; {
		if path[i] == '/' && path[i+1] == '/' {
			path = path[:i+1] + path[i+2:]
			n--
		} else {
			i++
		}
	}
	return path
}

// ExtractSessionID extracts the session ID from the URL path.
// Expected format: /api/sessions/{id}...
func ExtractSessionID(path string) string {
	parts := SplitPath(path)
	if len(parts) < 4 || parts[1] != "api" || parts[2] != "sessions" {
		return ""
	}
	return parts[3]
}

// ExtractProjectID extracts the project ID from the URL path.
// Expected format: /api/projects/{id}...
func ExtractProjectID(path string) string {
	parts := SplitPath(path)
	if len(parts) < 4 || parts[1] != "api" || parts[2] != "projects" {
		return ""
	}
	return parts[3]
}

// ExtractSubPath extracts the sub-path after a known prefix.
// For example, given path="/api/projects/123/files/src/main.go" and
// prefix="/api/projects/123/files", it returns "src/main.go".
func ExtractSubPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	subPath := strings.TrimPrefix(path, prefix)
	subPath = strings.TrimPrefix(subPath, "/")
	return subPath
}
