package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Engine runs anomaly detection and stores results.
type Engine struct {
	rules     []Rule
	anomalies map[string][]AnomalyResult // session_id -> anomalies
	mu        sync.RWMutex
	logger    *slog.Logger
	sqlURL    string
	httpClient *http.Client
}

// NewEngine creates an analysis engine.
func NewEngine() *Engine {
	return &Engine{
		rules:      AllRules(),
		anomalies:  make(map[string][]AnomalyResult),
		httpClient: &http.Client{},
	}
}

// NewEngineWithDB creates an engine with database storage.
func NewEngineWithDB(greptimeDBHost string, greptimeHTTPPort int, logger *slog.Logger) *Engine {
	return &Engine{
		rules:      AllRules(),
		anomalies:  make(map[string][]AnomalyResult),
		sqlURL:     fmt.Sprintf("http://%s:%d/v1/sql", greptimeDBHost, greptimeHTTPPort),
		logger:     logger,
		httpClient: &http.Client{},
	}
}

// AnalyzeHookEvent checks all rules against a hook event.
func (e *Engine) AnalyzeHookEvent(event HookEvent) []AnomalyResult {
	var results []AnomalyResult

	for _, rule := range e.rules {
		result := rule.Check(event)
		if result.Detected {
			results = append(results, result)
		}
	}

	// Store detected anomalies
	for _, r := range results {
		e.StoreAnomaly(r)
	}

	return results
}

// AnalyzeTurn checks turn-level rules.
func (e *Engine) AnalyzeTurn(turn TurnEvent) []AnomalyResult {
	var results []AnomalyResult

	for _, rule := range e.rules {
		result := rule.CheckTurn(turn)
		if result.Detected {
			results = append(results, result)
		}
	}

	for _, r := range results {
		e.StoreAnomaly(r)
	}

	return results
}

// AnalyzeBatch checks batch rules against multiple events.
func (e *Engine) AnalyzeBatch(events []HookEvent) []AnomalyResult {
	var results []AnomalyResult

	for _, rule := range e.rules {
		result := rule.CheckBatch(events)
		if result.Detected {
			results = append(results, result)
		}
	}

	for _, r := range results {
		e.StoreAnomaly(r)
	}

	return results
}

// StoreAnomaly saves an anomaly to memory and database.
func (e *Engine) StoreAnomaly(anomaly AnomalyResult) {
	e.mu.Lock()
	e.anomalies[anomaly.SessionID] = append(e.anomalies[anomaly.SessionID], anomaly)
	e.mu.Unlock()

	// Persist to database if configured
	if e.sqlURL != "" {
		go e.insertAnomaly(anomaly)
	}
}

// GetAnomalies retrieves anomalies for a session.
func (e *Engine) GetAnomalies(sessionID string) []AnomalyResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.anomalies[sessionID]
}

// GetAllAnomalies retrieves all anomalies.
func (e *Engine) GetAllAnomalies() map[string][]AnomalyResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string][]AnomalyResult)
	for k, v := range e.anomalies {
		result[k] = v
	}
	return result
}

// ShouldBroadcast determines if an anomaly should be pushed via SSE.
func (e *Engine) ShouldBroadcast(anomaly AnomalyResult) bool {
	return anomaly.Severity == SeverityMedium ||
		anomaly.Severity == SeverityHigh ||
		anomaly.Severity == SeverityCritical
}

func (e *Engine) insertAnomaly(anomaly AnomalyResult) {
	now := time.Now().UnixMilli()

	// Build extra JSON with related event details for detailed view
	extraJSON := ""
	if anomaly.RelatedEvent.ToolName != "" || anomaly.RelatedEvent.AgentSource != "" {
		extra := map[string]interface{}{
			"tool_name":    anomaly.RelatedEvent.ToolName,
			"duration_ms":  anomaly.RelatedEvent.Duration.Milliseconds(),
			"agent_source": anomaly.RelatedEvent.AgentSource,
			"tool_input":   truncateString(anomaly.RelatedEvent.ToolInput, 500),
			"tool_result":  truncateString(anomaly.RelatedEvent.ToolResult, 500),
			"event_type":   anomaly.RelatedEvent.EventType,
		}
		extraBytes, _ := json.Marshal(extra)
		extraJSON = string(extraBytes)
	}

	sql := fmt.Sprintf(
		"INSERT INTO apm_anomalies "+
			"(ts, session_id, anomaly_type, severity, description, suggested_cause, extra) "+
			"VALUES (%d, '%s', '%s', '%s', '%s', '%s', '%s')",
		now,
		escapeSQLEngine(anomaly.SessionID),
		escapeSQLEngine(anomaly.AnomalyType),
		escapeSQLEngine(anomaly.Severity),
		escapeSQLEngine(anomaly.Description),
		escapeSQLEngine(anomaly.SuggestedCause),
		escapeSQLEngine(extraJSON),
	)

	form := url.Values{}
	form.Set("sql", sql)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", e.sqlURL, strings.NewReader(form.Encode()))
	if err != nil {
		if e.logger != nil {
			e.logger.Debug("anomaly insert: create request failed", "error", err)
		}
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		if e.logger != nil {
			e.logger.Debug("anomaly insert failed", "error", err)
		}
		return
	}
	defer resp.Body.Close()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func escapeSQLEngine(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}