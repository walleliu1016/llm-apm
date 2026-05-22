package transcript

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Watcher manages per-session JSONL transcript watchers.
type Watcher struct {
	mu       sync.Mutex
	sessions map[string]*sessionWatch
	sqlURL   string
	logger   *slog.Logger
}

type sessionWatch struct {
	cancel context.CancelFunc
	seen   map[string]struct{}
}

// NewWatcher creates a transcript watcher.
func NewWatcher(greptimeDBHost string, greptimeHTTPPort int, logger *slog.Logger) *Watcher {
	return &Watcher{
		sessions: make(map[string]*sessionWatch),
		sqlURL:   fmt.Sprintf("http://%s:%d/v1/sql", greptimeDBHost, greptimeHTTPPort),
		logger:   logger,
	}
}

// Watch starts tailing a JSONL transcript file.
func (w *Watcher) Watch(sessionID, transcriptPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.sessions[sessionID]; ok {
		return // already watching
	}

	ctx, cancel := context.WithCancel(context.Background())
	sw := &sessionWatch{
		cancel: cancel,
		seen:   make(map[string]struct{}),
	}
	w.sessions[sessionID] = sw

	go w.tailFile(ctx, sessionID, transcriptPath, sw.seen)
	w.logger.Info("started watching transcript", "session", sessionID, "path", transcriptPath)
}

// Stop stops watching a transcript.
func (w *Watcher) Stop(sessionID string) {
	w.mu.Lock()
	sw, ok := w.sessions[sessionID]
	if ok {
		delete(w.sessions, sessionID)
	}
	w.mu.Unlock()

	if ok {
		sw.cancel()
	}
}

// StopAll stops all active watchers.
func (w *Watcher) StopAll() {
	w.mu.Lock()
	sessions := w.sessions
	w.sessions = make(map[string]*sessionWatch)
	w.mu.Unlock()

	for _, sw := range sessions {
		sw.cancel()
	}
}

func (w *Watcher) tailFile(ctx context.Context, sessionID, filePath string, seen map[string]struct{}) {
	// Wait for file to appear
	var f *os.File
	for i := 0; i < 10; i++ {
		var err error
		f, err = os.Open(filePath)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	if f == nil {
		w.logger.Warn("transcript file not found", "path", filePath)
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var buf strings.Builder

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			buf.WriteString(line)
			if strings.HasSuffix(line, "\n") {
				trimmed := strings.TrimSpace(buf.String())
				buf.Reset()
				if trimmed != "" {
					w.processLine(sessionID, trimmed, seen)
				}
			}
			continue
		}

		if err == io.EOF {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}

		if err != nil {
			w.logger.Warn("transcript read error", "error", err)
			return
		}
	}
}

// TranscriptEntry matches Claude Code JSONL format.
type TranscriptEntry struct {
	SessionID string         `json:"sessionId"`
	Type      string         `json:"type"`
	UUID      string         `json:"uuid"`
	Message   *TranscriptMsg `json:"message"`
	CWD       string         `json:"cwd"`
}

type TranscriptMsg struct {
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   *MsgUsage       `json:"usage"`
}

type MsgUsage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheReadTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
}

func (w *Watcher) processLine(sessionID, line string, seen map[string]struct{}) {
	var entry TranscriptEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return
	}

	if entry.Type != "user" && entry.Type != "assistant" {
		return
	}
	if entry.Message == nil {
		return
	}

	role := entry.Message.Role
	if role == "human" {
		role = "user"
	}

	// Parse content
	var content string
	if err := json.Unmarshal(entry.Message.Content, &content); err == nil {
		if content != "" {
			w.insertMessage(sessionID, role, role, truncate(content, 32768),
				entry.Message.Model, "", "", entry.Message.Usage)
		}
		return
	}

	// Parse as content blocks
	var blocks []ContentBlock
	if err := json.Unmarshal(entry.Message.Content, &blocks); err != nil {
		return
	}

	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				w.insertMessage(sessionID, "text", role, truncate(b.Text, 32768),
					entry.Message.Model, "", "", nil)
			}
		case "tool_use":
			inputStr := truncate(string(b.Input), 2048)
			w.insertMessage(sessionID, "tool_use", role, inputStr,
				entry.Message.Model, b.Name, b.ID, nil)
		case "tool_result":
			w.insertMessage(sessionID, "tool_result", role,
				truncate(extractToolResultContent(b.Content), 4096),
				entry.Message.Model, "", b.ToolUseID, nil)
		}
	}
}

type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Name      string          `json:"name"`
	ID        string          `json:"id"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

func extractToolResultContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct{ Text string }
	if json.Unmarshal(raw, &parts) == nil {
		var sb strings.Builder
		for _, p := range parts {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(p.Text)
		}
		return sb.String()
	}
	return string(raw)
}

func (w *Watcher) insertMessage(sessionID, messageType, role, content, model,
	toolName, toolUseID string, usage *MsgUsage) {

	now := time.Now().UnixMilli()

	var sql string
	if usage != nil {
		sql = fmt.Sprintf(
			"INSERT INTO apm_messages "+
				"(ts, session_id, message_type, role, content, model, tool_name, tool_use_id, "+
				"input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens) "+
				"VALUES (%d, '%s', '%s', '%s', '%s', '%s', '%s', '%s', %d, %d, %d, %d)",
			now, escapeSQL(sessionID), escapeSQL(messageType), escapeSQL(role),
			escapeSQL(content), escapeSQL(model), escapeSQL(toolName), escapeSQL(toolUseID),
			usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens, usage.CacheCreationTokens)
	} else {
		sql = fmt.Sprintf(
			"INSERT INTO apm_messages "+
				"(ts, session_id, message_type, role, content, model, tool_name, tool_use_id) "+
				"VALUES (%d, '%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			now, escapeSQL(sessionID), escapeSQL(messageType), escapeSQL(role),
			escapeSQL(content), escapeSQL(model), escapeSQL(toolName), escapeSQL(toolUseID))
	}

	go w.execSQL(sql)
}

func (w *Watcher) execSQL(sql string) {
	form := url.Values{}
	form.Set("sql", sql)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", w.sqlURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
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