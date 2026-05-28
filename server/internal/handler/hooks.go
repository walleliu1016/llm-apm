package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/akke/llm-apm/server/internal/analysis"
	"github.com/akke/llm-apm/server/internal/broadcaster"
	"github.com/akke/llm-apm/server/internal/transcript"
	"github.com/akke/llm-apm/server/internal/turn"
)

// Server holds handler dependencies.
type Server struct {
	greptimeDBHost     string
	greptimeHTTPPort   int
	httpClient         *http.Client
	logger             *slog.Logger
	transcriptWatcher  *transcript.Watcher
	broadcaster        *broadcaster.Broadcaster
	analysisEngine     *analysis.Engine
	turnTracker        *turn.Tracker
	pendingTools       sync.Map // map[string]time.Time - tracks tool execution start times (key: session_id_tool_use_id)
	watchedSessions    sync.Map // map[string]struct{} - tracks sessions already being watched
}

// SetTurnTracker sets the turn tracker for the server.
func (s *Server) SetTurnTracker(tracker *turn.Tracker) {
	s.turnTracker = tracker
}

// SetTranscriptWatcher sets the transcript watcher for the server.
func (s *Server) SetTranscriptWatcher(watcher *transcript.Watcher) {
	s.transcriptWatcher = watcher
}

// HookPayload represents a hook event from Claude Code.
type HookPayload struct {
	SessionID      string `json:"session_id"`
	HookEventName  string `json:"hook_event_name"`
	ToolName       string `json:"tool_name"`
	ToolInput      any    `json:"tool_input"`
	ToolUseID      string `json:"tool_use_id"`
	ToolResponse   any    `json:"tool_response"`
	AgentID        string `json:"agent_id"`
	AgentType      string `json:"agent_type"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	TenantID       string `json:"tenant_id"` // 预留
	Prompt         string `json:"prompt"`    // UserPromptSubmit 的用户输入
}

const (
	maxHookBody   = 1 << 20 // 1 MB
	maxToolInput  = 2048
	maxToolResult = 4096
)

// handleHooks receives hook events and stores them in GreptimeDB.
func (s *Server) handleHooks(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxHookBody))
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload HookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	if payload.SessionID == "" || payload.HookEventName == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Return 200 immediately - don't block the agent
	w.WriteHeader(http.StatusOK)

	// Detect agent source from query param
	agentSource := r.URL.Query().Get("source")
	if agentSource == "" {
		agentSource = "claude_code"
	}

	// Normalize tool_response
	toolResult := normalizeToolResponse(payload.ToolResponse)
	toolInput := serializeToolInput(payload.ToolInput)

	// For UserPromptSubmit, store prompt in tool_input
	if payload.HookEventName == "UserPromptSubmit" && payload.Prompt != "" {
		toolInput = payload.Prompt
	}

	// Determine error flag
	errorFlag := payload.HookEventName == "PostToolUseFailure" ||
		strings.Contains(payload.HookEventName, "Error")

	// Async insert
	go s.insertHookEvent(payload, agentSource, toolInput, toolResult, errorFlag)
}

func (s *Server) insertHookEvent(p HookPayload, agentSource, toolInput, toolResult string, errorFlag bool) {
	now := time.Now().UnixMilli()

	// Start transcript watcher on first hook for this session
	if s.transcriptWatcher != nil && p.TranscriptPath != "" {
		if _, loaded := s.watchedSessions.LoadOrStore(p.SessionID, struct{}{}); !loaded {
			s.transcriptWatcher.Watch(p.SessionID, p.TranscriptPath)
		}
	}

	// Track tool execution time: record PreToolUse start time
	if p.HookEventName == "PreToolUse" && p.ToolUseID != "" {
		key := p.SessionID + "_" + p.ToolUseID
		s.pendingTools.Store(key, time.Now())
	}

	// Calculate Duration for PostToolUse/PostToolUseFailure
	var duration time.Duration
	if (p.HookEventName == "PostToolUse" || p.HookEventName == "PostToolUseFailure") && p.ToolUseID != "" {
		key := p.SessionID + "_" + p.ToolUseID
		if start, ok := s.pendingTools.Load(key); ok {
			duration = time.Since(start.(time.Time))
			s.pendingTools.Delete(key)
		}
	}

	// Determine agent depth
	agentDepth := 0
	if p.AgentType == "subagent" || (p.AgentID != "" && p.AgentID != "main") {
		agentDepth = 1
	}

	sql := fmt.Sprintf(
		"INSERT INTO apm_hook_events "+
			"(ts, session_id, event_type, agent_source, agent_id, agent_depth, "+
			"tool_name, tool_input, tool_result, tool_use_id, cwd, error_flag, tenant_id) "+
			"VALUES (%d, '%s', '%s', '%s', '%s', %d, '%s', '%s', '%s', '%s', '%s', %v, '%s')",
		now,
		escapeSQL(p.SessionID),
		escapeSQL(p.HookEventName),
		escapeSQL(agentSource),
		escapeSQL(p.AgentID),
		agentDepth,
		escapeSQL(truncate(p.ToolName, 256)),
		escapeSQL(truncate(toolInput, maxToolInput)),
		escapeSQL(truncate(toolResult, maxToolResult)),
		escapeSQL(p.ToolUseID),
		escapeSQL(truncate(p.CWD, 512)),
		errorFlag,
		escapeSQL(p.TenantID),
	)

	sqlURL := fmt.Sprintf("http://%s:%d/v1/sql", s.greptimeDBHost, s.greptimeHTTPPort)
	form := url.Values{}
	form.Set("sql", sql)

	resp, err := s.httpClient.Post(sqlURL, "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()))
	if err != nil {
		if s.logger != nil {
			s.logger.Debug("hook insert failed", "error", err)
		}
		return
	}
	defer resp.Body.Close()

	// Run anomaly detection
	if s.analysisEngine != nil {
		event := analysis.HookEvent{
			TS:          time.Now(),
			SessionID:   p.SessionID,
			EventType:   p.HookEventName,
			ToolName:    p.ToolName,
			ToolInput:   toolInput,
			ToolResult:  toolResult,
			Duration:    duration, // Actual tool execution duration
			ErrorFlag:   errorFlag,
			AgentSource: agentSource,
			AgentID:     p.AgentID,
		}

		anomalies := s.analysisEngine.AnalyzeHookEvent(event)

		// Broadcast anomalies via SSE
		for _, anomaly := range anomalies {
			if s.analysisEngine.ShouldBroadcast(anomaly) {
				if s.broadcaster != nil {
					s.broadcaster.BroadcastJSON("anomaly", anomaly)
				}
			}
		}
	}

	// Track turn boundaries
	if s.turnTracker != nil {
		content := extractContent(p)
		s.turnTracker.HandleEvent(p.SessionID, p.HookEventName, time.Now(), content)
	}
}

// extractContent extracts relevant content from hook payload for turn tracking.
func extractContent(p HookPayload) string {
	switch p.HookEventName {
	case "UserPromptSubmit":
		if input, ok := p.ToolInput.(map[string]any); ok {
			if prompt, ok := input["prompt"].(string); ok {
				return prompt
			}
		}
	case "AssistantResponse":
		return truncate(normalizeToolResponse(p.ToolResponse), 256)
	}
	return ""
}

func normalizeToolResponse(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case map[string]any:
		if c, ok := val["content"]; ok {
			if s, ok := c.(string); ok {
				return s
			}
		}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func serializeToolInput(v any) string {
	if v == nil {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}