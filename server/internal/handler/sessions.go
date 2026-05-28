package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
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
			SUM(CASE WHEN error_flag THEN 1 ELSE 0 END) as error_count,
			MAX(cwd) as cwd
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
	// Parse GreptimeDB response structure
	// GreptimeDB returns: {"output":[{"records":{"schema":..., "rows":[[...], ...]}}]}

	var greptimeResp struct {
		Output []struct {
			Records struct {
				Rows [][]interface{} `json:"rows"`
			} `json:"records"`
		} `json:"output"`
	}

	if err := json.Unmarshal(rawData, &greptimeResp); err != nil {
		return nil, err
	}

	// Extract rows from first output
	if len(greptimeResp.Output) == 0 || len(greptimeResp.Output[0].Records.Rows) == 0 {
		response := map[string]interface{}{
			"sessions": []map[string]interface{}{},
		}
		return json.Marshal(response)
	}

	rawRows := greptimeResp.Output[0].Records.Rows

	sessions := []map[string]interface{}{}
	for _, row := range rawRows {
		// Robust type extraction (GreptimeDB may return different types)
		sessionID, _ := row[0].(string)
		agentSource, _ := row[1].(string)

		// Handle startTime (can be string or float64 timestamp)
		var t time.Time
		switch v := row[2].(type) {
		case string:
			if parsed, err := time.Parse(time.RFC3339, v); err == nil {
				t = parsed
			} else {
				t = time.Now()
			}
		case float64:
			// GreptimeDB may return timestamp as milliseconds
			t = time.Unix(int64(v/1000), 0)
		default:
			t = time.Now()
		}

		// Handle numeric fields
		var toolCount, errorCount int
		switch v := row[3].(type) {
		case float64:
			toolCount = int(v)
		case int:
			toolCount = v
		}
		switch v := row[4].(type) {
		case float64:
			errorCount = int(v)
		case int:
			errorCount = v
		}

		// Extract project name from cwd
		cwd, _ := row[5].(string)
		projectName := extractProjectName(cwd)

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
			"session_id_short": sessionID[:8] + "...",
			"project":        projectName,
			"cwd":            cwd,
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

// extractProjectName extracts project name from cwd path.
func extractProjectName(cwd string) string {
	if cwd == "" {
		return "-"
	}
	// Get the last directory name
	base := filepath.Base(cwd)
	// Handle special cases like worktrees
	if strings.Contains(cwd, ".claude/worktrees") {
		// Extract parent project name
		parts := strings.Split(cwd, "/.claude/worktrees")
		if len(parts) > 0 {
			parentBase := filepath.Base(parts[0])
			return parentBase + " (worktree)"
		}
	}
	return base
}

// handleSessionDetail returns session detail with events timeline.
func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	// Extract session_id from path
	path := r.URL.Path
	sessionID := ""
	if strings.HasPrefix(path, "/api/sessions/") {
		sessionID = strings.TrimPrefix(path, "/api/sessions/")
	}
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	// Query session details from apm_hook_events
	sql := fmt.Sprintf(`
		SELECT
			session_id,
			agent_source,
			MIN(ts) as start_ts,
			MAX(ts) as end_ts,
			COUNT(CASE WHEN event_type = 'PostToolUse' THEN 1 END) as tool_count,
			SUM(CASE WHEN error_flag THEN 1 ELSE 0 END) as error_count,
			MAX(cwd) as cwd
		FROM apm_hook_events
		WHERE session_id = '%s'
		GROUP BY session_id, agent_source
	`, sessionID)

	data, err := s.queryGreptimeDB(sql)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Query events timeline for this session
	eventsSQL := fmt.Sprintf(`
		SELECT
			ts,
			event_type,
			tool_name,
			tool_input,
			tool_result,
			error_flag,
			agent_id,
			agent_depth
		FROM apm_hook_events
		WHERE session_id = '%s'
		ORDER BY ts ASC
	`, sessionID)

	eventsData, err := s.queryGreptimeDB(eventsSQL)
	if err != nil {
		http.Error(w, "events query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	formatted, err := formatSessionDetailResponse(data, eventsData, sessionID)
	if err != nil {
		http.Error(w, "format failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(formatted)
}

// formatSessionDetailResponse formats session detail response with events.
func formatSessionDetailResponse(rawData []byte, eventsData []byte, sessionID string) ([]byte, error) {
	var greptimeResp struct {
		Output []struct {
			Records struct {
				Rows [][]interface{} `json:"rows"`
			} `json:"records"`
		} `json:"output"`
	}

	if err := json.Unmarshal(rawData, &greptimeResp); err != nil {
		return nil, err
	}

	if len(greptimeResp.Output) == 0 || len(greptimeResp.Output[0].Records.Rows) == 0 {
		response := map[string]interface{}{
			"session_id":   sessionID,
			"status":       "completed",
			"status_text":  "已完成",
			"agent_source": "Unknown",
			"duration":     "-",
			"total_cost":   "$0.00",
			"project":      "-",
			"events":       []map[string]interface{}{},
		}
		return json.Marshal(response)
	}

	row := greptimeResp.Output[0].Records.Rows[0]
	agentSource, _ := row[1].(string)

	// Handle timestamps
	var startTs, endTs time.Time
	switch v := row[2].(type) {
	case string:
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			startTs = parsed
		}
	case float64:
		startTs = time.Unix(int64(v/1000), 0)
	}
	switch v := row[3].(type) {
	case string:
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			endTs = parsed
		}
	case float64:
		endTs = time.Unix(int64(v/1000), 0)
	}

	// Handle tool_count and error_count
	var toolCount, errorCount int
	switch v := row[4].(type) {
	case float64:
		toolCount = int(v)
	case int:
		toolCount = v
	}
	switch v := row[5].(type) {
	case float64:
		errorCount = int(v)
	case int:
		errorCount = v
	}

	// Extract project name from cwd
	cwd, _ := row[6].(string)
	projectName := extractProjectName(cwd)

	// Calculate duration
	duration := "-"
	if !startTs.IsZero() && !endTs.IsZero() {
		d := endTs.Sub(startTs)
		if d < time.Minute {
			duration = fmt.Sprintf("%ds", int(d.Seconds()))
		} else {
			duration = fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
		}
	}

	// Calculate cost (simplified: $0.05 per tool call)
	cost := fmt.Sprintf("$%.2f", float64(toolCount)*0.05)

	// Parse events data
	events := parseEventsData(eventsData)

	response := map[string]interface{}{
		"session_id":   sessionID,
		"status":       "completed",
		"status_text":  "已完成",
		"agent_source": agentSource,
		"duration":     duration,
		"total_cost":   cost,
		"tool_count":   toolCount,
		"error_count":  errorCount,
		"project":      projectName,
		"cwd":          cwd,
		"events":       events,
	}

	return json.Marshal(response)
}

// parseEventsData parses events timeline data from GreptimeDB.
func parseEventsData(eventsData []byte) []map[string]interface{} {
	var greptimeResp struct {
		Output []struct {
			Records struct {
				Rows [][]interface{} `json:"rows"`
			} `json:"records"`
		} `json:"output"`
	}

	if err := json.Unmarshal(eventsData, &greptimeResp); err != nil {
		return []map[string]interface{}{}
	}

	if len(greptimeResp.Output) == 0 || len(greptimeResp.Output[0].Records.Rows) == 0 {
		return []map[string]interface{}{}
	}

	events := []map[string]interface{}{}
	for idx, row := range greptimeResp.Output[0].Records.Rows {
		// Handle timestamp
		var ts time.Time
		switch v := row[0].(type) {
		case string:
			if parsed, err := time.Parse(time.RFC3339, v); err == nil {
				ts = parsed
			}
		case float64:
			ts = time.Unix(int64(v/1000), 0)
		}

		eventType, _ := row[1].(string)
		toolName, _ := row[2].(string)
		toolInput, _ := row[3].(string)
		toolResult, _ := row[4].(string)

		var errorFlag bool
		switch v := row[5].(type) {
		case bool:
			errorFlag = v
		case float64:
			errorFlag = v == 1
		case int:
			errorFlag = v == 1
		}

		agentID, _ := row[6].(string)
		var agentDepth int
		switch v := row[7].(type) {
		case float64:
			agentDepth = int(v)
		case int:
			agentDepth = v
		}

		event := map[string]interface{}{
			"idx":         idx,
			"ts":          ts.Format(time.RFC3339),
			"event_type":  eventType,
			"tool_name":   toolName,
			"tool_input":  toolInput,
			"tool_result": toolResult,
			"error_flag":  errorFlag,
			"agent_id":    agentID,
			"agent_depth": agentDepth,
		}
		events = append(events, event)
	}

	return events
}