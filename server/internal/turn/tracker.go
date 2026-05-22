package turn

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Turn represents a user-agent interaction round.
type Turn struct {
	TurnID        string
	SessionID     string
	StartTS       time.Time
	EndTS         time.Time
	UserPrompt    string
	AgentResponse string
	ToolCount     int64
	InputTokens   int64
	OutputTokens  int64
	CostUSD       float64
	HasError      bool
}

// Tracker manages turn boundaries for sessions.
type Tracker struct {
	mu             sync.RWMutex
	currentTurns   map[string]*Turn  // session_id -> active turn
	completedTurns map[string][]Turn // session_id -> completed turns
	logger         *slog.Logger
	sqlURL         string
	httpClient     *http.Client
}

// NewTracker creates a turn tracker.
func NewTracker() *Tracker {
	return &Tracker{
		currentTurns:   make(map[string]*Turn),
		completedTurns: make(map[string][]Turn),
		httpClient:     &http.Client{},
	}
}

// NewTrackerWithDB creates a tracker with database storage.
func NewTrackerWithDB(greptimeDBHost string, greptimeHTTPPort int, logger *slog.Logger) *Tracker {
	return &Tracker{
		currentTurns:   make(map[string]*Turn),
		completedTurns: make(map[string][]Turn),
		sqlURL:         fmt.Sprintf("http://%s:%d/v1/sql", greptimeDBHost, greptimeHTTPPort),
		logger:         logger,
		httpClient:     &http.Client{},
	}
}

// HandleEvent processes a hook event to track turn boundaries.
func (t *Tracker) HandleEvent(sessionID, eventType string, ts time.Time, content string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch eventType {
	case "UserPromptSubmit":
		// Start new turn
		turnID := fmt.Sprintf("turn-%d", ts.UnixMilli())
		turn := &Turn{
			TurnID:     turnID,
			SessionID:  sessionID,
			StartTS:    ts,
			UserPrompt: content,
		}
		t.currentTurns[sessionID] = turn

	case "AssistantResponse":
		// End current turn
		if turn, ok := t.currentTurns[sessionID]; ok {
			turn.EndTS = ts
			turn.AgentResponse = content

			// Store completed turn
			t.completedTurns[sessionID] = append(t.completedTurns[sessionID], *turn)

			// Remove from current
			delete(t.currentTurns, sessionID)

			// Persist to database
			if t.sqlURL != "" {
				go t.insertTurn(*turn)
			}
		}

	case "PreToolUse":
		// Tool starting - nothing to track

	case "PostToolUse":
		// Tool completed - increment count
		if turn, ok := t.currentTurns[sessionID]; ok {
			turn.ToolCount++
		}

	case "PostToolUseFailure":
		// Tool failed
		if turn, ok := t.currentTurns[sessionID]; ok {
			turn.ToolCount++
			turn.HasError = true
		}
	}
}

// GetCurrentTurn returns the active turn for a session.
func (t *Tracker) GetCurrentTurn(sessionID string) *Turn {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentTurns[sessionID]
}

// GetCompletedTurns returns all completed turns for a session.
func (t *Tracker) GetCompletedTurns(sessionID string) []Turn {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.completedTurns[sessionID]
}

// GetAllCompletedTurns returns all completed turns.
func (t *Tracker) GetAllCompletedTurns() map[string][]Turn {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string][]Turn)
	for k, v := range t.completedTurns {
		result[k] = v
	}
	return result
}

// UpdateTokens updates token counts for the current turn.
func (t *Tracker) UpdateTokens(sessionID string, inputTokens, outputTokens int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if turn, ok := t.currentTurns[sessionID]; ok {
		turn.InputTokens += inputTokens
		turn.OutputTokens += outputTokens
	}
}

func (t *Tracker) insertTurn(turn Turn) {
	sql := fmt.Sprintf(
		"INSERT INTO apm_turns "+
			"(ts, turn_id, session_id, start_ts, end_ts, user_prompt, agent_response, "+
			"input_tokens, output_tokens, cost_usd, tool_count, has_error) "+
			"VALUES (%d, '%s', '%s', %d, %d, '%s', '%s', %d, %d, %f, %d, %v)",
		turn.EndTS.UnixMilli(),
		escapeSQL(turn.TurnID),
		escapeSQL(turn.SessionID),
		turn.StartTS.UnixMilli(),
		turn.EndTS.UnixMilli(),
		escapeSQL(truncate(turn.UserPrompt, 512)),
		escapeSQL(truncate(turn.AgentResponse, 256)),
		turn.InputTokens,
		turn.OutputTokens,
		turn.CostUSD,
		turn.ToolCount,
		turn.HasError,
	)

	form := url.Values{}
	form.Set("sql", sql)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", t.sqlURL, strings.NewReader(form.Encode()))
	if err != nil {
		if t.logger != nil {
			t.logger.Debug("turn insert: create request failed", "error", err)
		}
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		if t.logger != nil {
			t.logger.Debug("turn insert failed", "error", err)
		}
		return
	}
	defer resp.Body.Close()
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

func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}