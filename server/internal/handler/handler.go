package handler

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/akke/llm-apm/server/internal/analysis"
	"github.com/akke/llm-apm/server/internal/broadcaster"
	"github.com/akke/llm-apm/server/web"
)

// NewServer creates a handler server.
func NewServer(greptimeDBHost string, greptimeHTTPPort int, logger *slog.Logger) *Server {
	return &Server{
		greptimeDBHost:   greptimeDBHost,
		greptimeHTTPPort: greptimeHTTPPort,
		httpClient:       &http.Client{},
		logger:           logger,
		broadcaster:      broadcaster.NewBroadcaster(),
		analysisEngine:   analysis.NewEngineWithDB(greptimeDBHost, greptimeHTTPPort, logger),
	}
}

// RegisterRoutes sets up all HTTP endpoints.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/hooks", s.handleHooks)
	mux.HandleFunc("/api/hooks/stream", s.handleSSEStream)
	mux.HandleFunc("/api/query", s.handleQuery)
	mux.HandleFunc("/api/stats/overview", s.handleStatsOverview)
	mux.HandleFunc("/api/stats/cache", s.handleStatsCache)
	mux.HandleFunc("/api/stats/cost", s.handleStatsCost)
	mux.HandleFunc("/api/stats/tools", s.handleStatsTools)

	// Sessions API
	mux.HandleFunc("/api/sessions", s.handleSessionsList)
	mux.HandleFunc("/api/sessions/", s.handleSessionDetail)

	// Problems API
	mux.HandleFunc("/api/problems", s.handleProblemsList)
	mux.HandleFunc("/api/problems/", s.handleProblemDetail)

	// Analysis API (11 endpoints)
	mux.HandleFunc("/api/analysis/overview", s.handleAnalysisOverview)
	mux.HandleFunc("/api/analysis/timeline", s.handleAnalysisTimeline)
	mux.HandleFunc("/api/analysis/models", s.handleAnalysisModels)
	mux.HandleFunc("/api/analysis/cache", s.handleAnalysisCache)
	mux.HandleFunc("/api/analysis/anomalies", s.handleAnalysisAnomalies)
	mux.HandleFunc("/api/analysis/ttft", s.handleAnalysisTTFT)
	mux.HandleFunc("/api/analysis/cost-ranking", s.handleAnalysisCostRanking)
	mux.HandleFunc("/api/analysis/tools", s.handleAnalysisTools)
	mux.HandleFunc("/api/analysis/subagent", s.handleAnalysisSubagent)
	mux.HandleFunc("/api/analysis/turn-efficiency", s.handleAnalysisTurnEfficiency)
	mux.HandleFunc("/api/analysis/agents", s.handleAnalysisAgents)

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleDashboard)
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleDashboard serves the embedded HTML dashboard.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(web.FS, "index.html")
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}