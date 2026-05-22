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

// QueryRequest represents a SQL query request.
type QueryRequest struct {
	SQL string `json:"sql"`
}

// handleQuery proxies SQL queries to GreptimeDB.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var req QueryRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.SQL == "" {
		http.Error(w, "sql required", http.StatusBadRequest)
		return
	}

	// Proxy to GreptimeDB
	sqlURL := fmt.Sprintf("http://%s:%d/v1/sql", s.greptimeDBHost, s.greptimeHTTPPort)

	form := url.Values{}
	form.Set("sql", req.SQL)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(sqlURL, "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()))
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}