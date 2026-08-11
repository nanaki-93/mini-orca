package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
)

// RenderDashboard handles GET /api/render/dashboard
// Renders both phase tracker and activity log in a single request.
// Uses caching to avoid unnecessary re-renders.
func (h *HTMXRenderHandler) RenderDashboard(w http.ResponseWriter, r *http.Request) {
	// Check If-None-Match header for conditional requests
	ifNoneMatch := r.Header.Get("If-None-Match")
	if h.cache != nil && ifNoneMatch != "" {
		if entry, ok := h.cache.Get("/api/render/dashboard"); ok {
			if entry.Etag == ifNoneMatch[1:len(ifNoneMatch)-1] {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	rendered, err := h.templateEngine.RenderComponentPartial("dashboard", nil)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the dashboard component.", err))
		return
	}

	// Cache the response
	if h.cache != nil {
		h.cache.Set("/api/render/dashboard", []byte(rendered), 10*time.Second)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=10")
	w.Header().Set("ETag", "\"dashboard-"+fmt.Sprintf("%d", time.Now().Unix()/10)+"\"")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}
