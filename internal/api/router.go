package api

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/internal/operation"
)

type Server struct {
	SM           *operation.StateMachine
	ApprovalChan chan bool
}

func NewRouter(sm *operation.StateMachine, appChan chan bool) http.Handler {
	s := &Server{
		SM:           sm,
		ApprovalChan: appChan,
	}

	mux := http.NewServeMux()

	// API Route Definitions
	mux.HandleFunc("/api/session", s.handleSession)
	mux.HandleFunc("/api/session/edit", s.handleEdit)
	mux.HandleFunc("/api/session/approve", s.handleApprove)

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
