package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// handleStatsOverview returns overall statistics.
func (s *Server) handleStatsOverview(w http.ResponseWriter, r *http.Request) {
	// Query last 24 hours
	sql := `SELECT
		session_id,
		SUM(input_tokens) as total_input,
		SUM(output_tokens) as total_output,
		SUM(cache_read_tokens) as cache_read,
		SUM(cache_creation_tokens) as cache_creation
	FROM apm_messages
	WHERE ts > now() - INTERVAL '24 hours'
	GROUP BY session_id`

	data, err := s.queryGreptimeDB(sql)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleStatsCache returns cache efficiency stats.
func (s *Server) handleStatsCache(w http.ResponseWriter, r *http.Request) {
	sql := `SELECT
		SUM(cache_read_tokens) as cache_read,
		SUM(cache_creation_tokens) as cache_creation,
		SUM(input_tokens) as input,
		SUM(output_tokens) as output
	FROM apm_messages
	WHERE ts > now() - INTERVAL '7 days'`

	data, err := s.queryGreptimeDB(sql)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleStatsCost returns cost breakdown by session.
func (s *Server) handleStatsCost(w http.ResponseWriter, r *http.Request) {
	// Get time range from query params
	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "24h"
	}

	interval := mapRangeToInterval(rangeParam)

	sql := fmt.Sprintf(`SELECT
		session_id,
		SUM(input_tokens) as input,
		SUM(output_tokens) as output,
		SUM(cache_read_tokens) as cache_read,
		SUM(cache_creation_tokens) as cache_creation
	FROM apm_messages
	WHERE ts > now() - INTERVAL '%s'
	GROUP BY session_id
	ORDER BY input + output DESC
	LIMIT 10`, interval)

	data, err := s.queryGreptimeDB(sql)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleStatsTools returns tool usage statistics.
func (s *Server) handleStatsTools(w http.ResponseWriter, r *http.Request) {
	sql := `SELECT
		tool_name,
		COUNT(*) as call_count,
		SUM(CASE WHEN error_flag THEN 1 ELSE 0 END) as error_count
	FROM apm_hook_events
	WHERE event_type = 'PostToolUse' AND ts > now() - INTERVAL '7 days'
	GROUP BY tool_name
	ORDER BY call_count DESC`

	data, err := s.queryGreptimeDB(sql)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *Server) queryGreptimeDB(sql string) ([]byte, error) {
	sqlURL := fmt.Sprintf("http://%s:%d/v1/sql", s.greptimeDBHost, s.greptimeHTTPPort)

	form := url.Values{}
	form.Set("sql", sql)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(sqlURL, "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// parseGreptimeRows extracts rows from GreptimeDB response structure.
// GreptimeDB returns: {"output":[{"records":{"schema":..., "rows":[[...], ...]}}]}
func parseGreptimeRows(rawData []byte) ([][]interface{}, error) {
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

	if len(greptimeResp.Output) == 0 {
		return [][]interface{}{}, nil
	}

	return greptimeResp.Output[0].Records.Rows, nil
}

func mapRangeToInterval(rangeParam string) string {
	switch rangeParam {
	case "1h":
		return "1 hour"
	case "24h":
		return "24 hours"
	case "7d":
		return "7 days"
	case "30d":
		return "30 days"
	default:
		return "24 hours"
	}
}