package main

import (
	"net/http"
	"sync"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// Cache only the most recently used checkout. Registration and scope are still
// checked by WorktreeService on every request, and old services cannot accumulate.
type worktreeRouter struct {
	mu      sync.Mutex
	service *app.Service
	routes  http.Handler
}

func (router *worktreeRouter) handler(service *app.Service, manager *project.Manager) http.Handler {
	router.mu.Lock()
	defer router.mu.Unlock()
	if router.routes == nil || router.service != service {
		router.routes = newProjectHTTPMux(service, manager)
		router.service = service
	}
	return router.routes
}
