package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type AgentResearchHandler struct{ service *app.Service }

func NewAgentResearchHandler(service *app.Service) *AgentResearchHandler {
	return &AgentResearchHandler{service: service}
}

func (h *AgentResearchHandler) Overview(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ResearchOverview(r.Context(), query)
	respondAPI(w, value, err)
}

func (h *AgentResearchHandler) Run(w http.ResponseWriter, r *http.Request) {
	var request app.ResearchRequest
	if !decodeStrictJSON(w, r, &request, "invalid research request", "Provide current project identity, explicit source hashes and search/source-sharing confirmation.") {
		return
	}
	value, err := h.service.RunResearch(r.Context(), request)
	respondAPI(w, value, err)
}

func (h *AgentResearchHandler) Report(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	page, err := h.service.ResearchReportHTML(r.Context(), query, r.PathValue("reportID"))
	if err != nil {
		respondAPI(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", app.ResearchReportCSP)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(page)
}
