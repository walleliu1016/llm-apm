package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleSessionsList returns session list.
func (s *Server) handleSessionsList(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "today"
	}

	// Build SQL query based on filter and range
	interval := mapRangeToInterval(timeRange)

	sql := fmt.Sprintf(`
		SELECT
			session_id,
			agent_source,
			MIN(ts) as start_time,
			COUNT(CASE WHEN event_type = 'PostToolUse' THEN 1 END) as tool_count,
			SUM(CASE WHEN error_flag THEN 1 ELSE 0 END) as error_count
		FROM apm_hook_events
		WHERE ts > now() - INTERVAL '%s'
		GROUP BY session_id, agent_source
		ORDER BY start_time DESC
		LIMIT 50
	`, interval)

	data, err := s.queryGreptimeDB(sql)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Format data
	formatted, err := formatSessionsResponse(data)
	if err != nil {
		http.Error(w, "format failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(formatted)
}

// formatSessionsResponse formats raw GreptimeDB data to frontend format.
func formatSessionsResponse(rawData []byte) ([]byte, error) {
	// Parse GreptimeDB response (array of arrays)
	// GreptimeDB returns: [[session_id, agent_source, start_time, tool_count, error_count], ...]

	var rawRows [][]interface{}
	if err := json.Unmarshal(rawData, &rawRows); err != nil {
		return nil, err
	}

	sessions := []map[string]interface{}{}
	for _, row := range rawRows {
		sessionID := row[0].(string)
		agentSource := row[1].(string)
		startTime := row[2].(string)
		toolCount := int(row[3].(float64))
		errorCount := int(row[4].(float64))

		// Format start_time to "2024-01-15 14:30"
		t, err := time.Parse(time.RFC3339, startTime)
		if err != nil {
			t = time.Now()
		}
		formattedTime := t.Format("2006-01-02 15:04")

		// Determine status (simplified: "completed" if older than 1 hour)
		status := "completed"
		statusText := "已完成"
		if time.Since(t) < time.Hour {
			status = "running"
			statusText = "运行中"
		}

		// Calculate cost (simplified: $0.05 per tool call)
		cost := fmt.Sprintf("$%.2f", float64(toolCount)*0.05)

		// Estimate tokens (simplified: 500 per tool call)
		totalTokens := fmt.Sprintf("%dk", toolCount/2)

		session := map[string]interface{}{
			"session_id":     sessionID,
			"status":         status,
			"status_text":    statusText,
			"agent_source":   agentSource,
			"start_time":     formattedTime,
			"anomaly_count":  errorCount,
			"tool_count":     toolCount,
			"cost":           cost,
			"total_tokens":   totalTokens,
			"has_anomaly":    errorCount > 0,
		}
		sessions = append(sessions, session)
	}

	response := map[string]interface{}{
		"sessions": sessions,
	}

	return json.Marshal(response)
}

// handleSessionDetail returns session detail.
func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	// Return mock data for now (will implement full query later)
	response := map[string]interface{}{
		"session_id": sessionID,
		"status":     "completed",
		"status_text": "已完成",
		"agent_source": "Claude Code",
		"duration":   "8min 32s",
		"total_cost": "$2.35",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}