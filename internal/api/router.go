package api

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/nanaki-93/mini-orca/internal/operation"
)

// Use go:embed to compile templates right inside your final static binary file
//
//go:embed templates/*
var templateFS embed.FS

type Server struct {
	SM           *operation.StateMachine
	ApprovalChan chan bool
	tmpl         *template.Template
}

func NewRouter(sm *operation.StateMachine, appChan chan bool) http.Handler {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	s := &Server{
		SM:           sm,
		ApprovalChan: appChan,
		tmpl:         tmpl,
	}

	mux := http.NewServeMux()

	// API Route Definitions
	mux.HandleFunc("/api/session", s.handleSession)
	mux.HandleFunc("/api/session/edit", s.handleEdit)
	mux.HandleFunc("/api/session/approve", s.handleApprove)

	// HTMX Server-Side UI Rendering Endpoints
	mux.HandleFunc("/dashboard", s.handleRenderDashboard)
	mux.HandleFunc("/web/status", s.handleRenderStatusBadge)
	mux.HandleFunc("/web/workstream", s.handleRenderWorkstream)

	// Wrap with basic CORS middleware so local React/Next.js apps can talk to it cleanly
	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
