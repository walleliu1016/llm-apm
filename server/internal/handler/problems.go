package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// handleProblemsList returns anomaly problems list.
func (s *Server) handleProblemsList(w http.ResponseWriter, r *http.Request) {
	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "today"
	}

	interval := mapRangeToInterval(timeRange)

	// Query anomalies from apm_anomalies table
	sql := fmt.Sprintf(`
		SELECT
			ts,
			session_id,
			anomaly_type,
			severity,
			description,
			extra
		FROM apm_anomalies
		WHERE ts > now() - INTERVAL '%s'
		ORDER BY ts DESC
		LIMIT 50
	`, interval)

	data, err := s.queryGreptimeDB(sql)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	formatted, err := formatProblemsResponse(data)
	if err != nil {
		http.Error(w, "format failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(formatted)
}

func formatProblemsResponse(rawData []byte) ([]byte, error) {
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
			"problems":        []map[string]interface{}{},
			"severity_counts": map[string]int{},
		}
		return json.Marshal(response)
	}

	rawRows := greptimeResp.Output[0].Records.Rows

	problems := []map[string]interface{}{}
	severityCounts := map[string]int{}

	for _, row := range rawRows {
		// Robust type extraction
		sessionID, _ := row[1].(string)
		anomalyType, _ := row[2].(string)
		severity, _ := row[3].(string)
		description, _ := row[4].(string)

		// Parse extra field (JSON string or object)
		var extra map[string]interface{}
		switch v := row[5].(type) {
		case string:
			if v != "" {
				json.Unmarshal([]byte(v), &extra)
			}
		case map[string]interface{}:
			extra = v
		}

		// Handle timestamp (can be string or float64)
		var t time.Time
		switch v := row[0].(type) {
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
		formattedTime := t.Format("2006-01-02 15:04:05")

		// Short session_id (first 8 chars)
		shortSessionID := sessionID
		if len(sessionID) > 8 {
			shortSessionID = sessionID[:8]
		}

		// Build problem title
		problemTitle := anomalyType
		if extra != nil {
			if toolName, ok := extra["tool_name"].(string); ok && toolName != "" {
				if durationMs, ok := extra["duration_ms"].(float64); ok && durationMs > 0 {
					problemTitle = fmt.Sprintf("%s: %s (%ds)", anomalyType, toolName, int(durationMs/1000))
				} else {
					problemTitle = fmt.Sprintf("%s: %s", anomalyType, toolName)
				}
			}
		}

		// Serialize extra to JSON string for frontend
		extraJSON, _ := json.Marshal(extra)

		problem := map[string]interface{}{
			"problem_id":       fmt.Sprintf("prob-%d", len(problems)+1),
			"problem_type":     anomalyType,
			"problem_title":    problemTitle,
			"severity":         severity,
			"session_id":       sessionID,
			"session_id_short": shortSessionID,
			"agent_source":     "Claude Code", // Simplified
			"time":             formattedTime,
			"description":      description,
			"extra":            string(extraJSON),
		}

		problems = append(problems, problem)
		severityCounts[severity]++
	}

	response := map[string]interface{}{
		"problems":        problems,
		"severity_counts": severityCounts,
	}

	return json.Marshal(response)
}

// handleProblemDetail returns problem detail.
func (s *Server) handleProblemDetail(w http.ResponseWriter, r *http.Request) {
	// Extract problem_id from path
	path := r.URL.Path
	problemID := ""
	if strings.HasPrefix(path, "/api/problems/") {
		problemID = strings.TrimPrefix(path, "/api/problems/")
	}
	if problemID == "" {
		http.Error(w, "problem_id required", http.StatusBadRequest)
		return
	}

	// Parse problem_id (format: "prob-{n}")
	// Query the nth anomaly from apm_anomalies table
	index := 1
	if strings.HasPrefix(problemID, "prob-") {
		if n, err := fmt.Sscanf(problemID, "prob-%d", &index); err != nil || n != 1 {
			index = 1
		}
	}

	// Query anomaly by index (using LIMIT and OFFSET)
	sql := fmt.Sprintf(`
		SELECT
			ts,
			session_id,
			anomaly_type,
			severity,
			description,
			suggested_cause,
			extra
		FROM apm_anomalies
		ORDER BY ts DESC
		LIMIT 1 OFFSET %d
	`, index-1)

	data, err := s.queryGreptimeDB(sql)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	formatted, err := formatProblemDetailResponse(data, problemID)
	if err != nil {
		http.Error(w, "format failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(formatted)
}

func formatProblemDetailResponse(rawData []byte, problemID string) ([]byte, error) {
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
			"problem_id":    problemID,
			"problem_title": "Unknown",
			"severity":      "medium",
			"time":          "-",
			"stat_cards":    []map[string]interface{}{},
		}
		return json.Marshal(response)
	}

	row := greptimeResp.Output[0].Records.Rows[0]

	// Handle timestamp
	var t time.Time
	switch v := row[0].(type) {
	case string:
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			t = parsed
		}
	case float64:
		t = time.Unix(int64(v/1000), 0)
	}
	formattedTime := t.Format("2006-01-02 15:04:05")

	sessionID, _ := row[1].(string)
	anomalyType, _ := row[2].(string)
	severity, _ := row[3].(string)
	description, _ := row[4].(string)
	suggestedCause, _ := row[5].(string)

	// Parse extra field (JSON string or object)
	var extra map[string]interface{}
	switch v := row[6].(type) {
	case string:
		json.Unmarshal([]byte(v), &extra)
	case map[string]interface{}:
		extra = v
	}

	// Build problem title
	problemTitle := anomalyType
	if toolName, ok := extra["tool_name"].(string); ok && toolName != "" {
		if durationMs, ok := extra["duration_ms"].(float64); ok && durationMs > 0 {
			problemTitle = fmt.Sprintf("%s: %s (%ds)", anomalyType, toolName, int(durationMs/1000))
		} else {
			problemTitle = fmt.Sprintf("%s: %s", anomalyType, toolName)
		}
	}

	// Build stat cards
	statCards := []map[string]interface{}{}
	if anomalyType == "slow_tool" || anomalyType == "slow_tool_critical" {
		if durationMs, ok := extra["duration_ms"].(float64); ok {
			d := int(durationMs / 1000)
			statCards = append(statCards, map[string]interface{}{
				"label":     "执行时间",
				"value":     fmt.Sprintf("%ds", d),
				"has_error": d > 30,
			})
		}
		if toolName, ok := extra["tool_name"].(string); ok {
			statCards = append(statCards, map[string]interface{}{
				"label":     "工具名称",
				"value":     toolName,
				"has_error": false,
			})
		}
	} else if anomalyType == "tool_failure" {
		if toolName, ok := extra["tool_name"].(string); ok {
			statCards = append(statCards, map[string]interface{}{
				"label":     "工具名称",
				"value":     toolName,
				"has_error": true,
			})
		}
		statCards = append(statCards, map[string]interface{}{
			"label":     "失败状态",
			"value":     "执行失败",
			"has_error": true,
		})
	}

	response := map[string]interface{}{
		"problem_id":      problemID,
		"problem_title":   problemTitle,
		"severity":        severity,
		"time":            formattedTime,
		"session_id":      sessionID,
		"description":     description,
		"suggested_cause": suggestedCause,
		"stat_cards":      statCards,
		"extra":           extra,
	}

	return json.Marshal(response)
}