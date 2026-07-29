// Package api provides HTTP handlers for the Mini-Orca REST API.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/nanaki-93/mini-orca/internal/config"
)

// Version represents an API version.
type Version string

const (
	// VersionV1 is the first version of the API.
	VersionV1 Version = "v1"
	// VersionV2 is the second version of the API.
	VersionV2 Version = "v2"
)

// VersionMiddleware provides API version detection and deprecation warning logging.
type VersionMiddleware struct {
	// config holds the API configuration.
	config config.APIConfig
	// logFunc is the logging function for deprecation warnings.
	logFunc func(format string, args ...any)
	// mu protects concurrent access to the warning log state.
	mu sync.Mutex
	// warned tracks which versions have already been logged as deprecated.
	warned map[Version]bool
}

// NewVersionMiddleware creates a new VersionMiddleware instance.
func NewVersionMiddleware(cfg config.APIConfig, logFunc func(format string, args ...any)) *VersionMiddleware {
	return &VersionMiddleware{
		config:  cfg,
		logFunc: logFunc,
		warned:  make(map[Version]bool),
	}
}

// VersionFromRequest extracts the API version from the request.
// It checks the following in order:
//  1. X-API-Version header
//  2. Accept header with vendor prefix (e.g., application/vnd.mini-orca.v1+json)
//  3. URL path prefix (e.g., /api/v1/...)
//  4. Default version from configuration
func VersionFromRequest(r *http.Request) Version {
	// Check X-API-Version header
	if version := r.Header.Get("X-API-Version"); version != "" {
		return Version(version)
	}

	// Check Accept header for vendor-specific versioning
	accept := r.Header.Get("Accept")
	if version := extractVersionFromAccept(accept); version != "" {
		return Version(version)
	}

	// Check URL path for version prefix
	if version := extractVersionFromPath(r.URL.Path); version != "" {
		return Version(version)
	}

	// Return default version from configuration
	return Version(config.Default().API.DefaultVersion)
}

// extractVersionFromAccept extracts the API version from the Accept header.
func extractVersionFromAccept(accept string) string {
	if accept == "" {
		return ""
	}

	parts := strings.Split(accept, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "application/vnd.mini-orca") {
			if strings.Contains(part, ".v1") {
				return "v1"
			}
			if strings.Contains(part, ".v2") {
				return "v2"
			}
		}
	}

	return ""
}

// extractVersionFromPath extracts the API version from the URL path.
func extractVersionFromPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "api" {
		switch parts[1] {
		case "v1", "v2":
			return parts[1]
		}
	}
	return ""
}

// IsDeprecated checks if the given version is deprecated.
func (m *VersionMiddleware) IsDeprecated(version Version) bool {
	for _, v := range m.config.DeprecatedVersions {
		if Version(v) == version {
			return true
		}
	}
	return false
}

// IsEnabled checks if the given version is enabled.
func (m *VersionMiddleware) IsEnabled(version Version) bool {
	for _, v := range m.config.EnabledVersions {
		if Version(v) == version {
			return true
		}
	}
	return false
}

// Next returns an HTTP handler that wraps the next handler with version detection and deprecation logging.
func (m *VersionMiddleware) Next(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := VersionFromRequest(r)

		// Check if version is enabled
		if !m.IsEnabled(version) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("API version %q is not enabled", version))
			return
		}

		// Log deprecation warning for deprecated versions
		if m.IsDeprecated(version) {
			m.logDeprecation(version)
		}

		// Add version to request context
		ctx := r.Context()
		ctx = contextWithVersion(ctx, version)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// logDeprecation logs a deprecation warning for the given version.
func (m *VersionMiddleware) logDeprecation(version Version) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.warned[version] {
		return
	}

	m.warned[version] = true

	if m.logFunc != nil {
		m.logFunc("[DEPRECATION] API version %s is deprecated and will be removed in a future release. Please migrate to a newer version.", version)
	}
}

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const versionContextKey contextKey = "api_version"

// contextWithVersion adds the API version to the request context.
func contextWithVersion(ctx context.Context, version Version) context.Context {
	return context.WithValue(ctx, versionContextKey, version)
}

// VersionFromContext extracts the API version from the request context.
func VersionFromContext(ctx context.Context) Version {
	if version, ok := ctx.Value(versionContextKey).(Version); ok {
		return version
	}
	return ""
}
