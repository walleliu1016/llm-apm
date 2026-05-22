package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHooks(t *testing.T) {
	body := `{
		"session_id": "test-session-123",
		"hook_event_name": "PreToolUse",
		"tool_name": "Read",
		"tool_input": {"file_path": "/src/main.go"},
		"agent_id": "",
		"agent_type": "main"
	}`

	req := httptest.NewRequest("POST", "/api/hooks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	// Create mock server
	s := &Server{
		greptimeDBHost:   "127.0.0.1",
		greptimeHTTPPort: 14000,
		logger:           nil,
	}

	s.handleHooks(rr, req)

	// Should return 200 immediately
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleHooksInvalidJSON(t *testing.T) {
	body := `{invalid json}`

	req := httptest.NewRequest("POST", "/api/hooks", bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()

	s := &Server{}
	s.handleHooks(rr, req)

	// Should still return 200 (don't block agent)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for invalid JSON, got %d", rr.Code)
	}
}