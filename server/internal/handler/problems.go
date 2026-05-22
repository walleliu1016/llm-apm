package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
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
			description
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
	var rawRows [][]interface{}
	if err := json.Unmarshal(rawData, &rawRows); err != nil {
		return nil, err
	}

	problems := []map[string]interface{}{}
	severityCounts := map[string]int{}

	for _, row := range rawRows {
		ts := row[0].(string)
		sessionID := row[1].(string)
		anomalyType := row[2].(string)
		severity := row[3].(string)
		description := row[4].(string)

		// Format time to "2024-01-15 14:32:18"
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			t = time.Now()
		}
		formattedTime := t.Format("2006-01-02 15:04:05")

		// Short session_id (first 8 chars)
		shortSessionID := sessionID
		if len(sessionID) > 8 {
			shortSessionID = sessionID[:8]
		}

		problem := map[string]interface{}{
			"problem_id":         fmt.Sprintf("prob-%d", len(problems)+1),
			"problem_type":       anomalyType,
			"severity":           severity,
			"session_id_short":   shortSessionID,
			"agent_source":       "Claude Code", // Simplified
			"time":               formattedTime,
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
	problemID := r.PathValue("problem_id")
	if problemID == "" {
		http.Error(w, "problem_id required", http.StatusBadRequest)
		return
	}

	// Mock detail response (will implement full version later)
	response := map[string]interface{}{
		"problem_id":   problemID,
		"problem_title": "slow_tool: Bash (45秒)",
		"severity":     "critical",
		"time":         "2024-01-15 14:32:18",
		"stat_cards": []map[string]interface{}{
			{"label": "执行时间", "value": "45s", "has_error": true},
			{"label": "工具名称", "value": "Bash", "has_error": false},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}