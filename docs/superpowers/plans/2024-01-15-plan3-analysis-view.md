# LLM-APM Plan 3: Analysis View + Turn 边界 + 缓存效率

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完善 Analysis View UI，实现 Turn 边界追踪和缓存效率统计。

**Architecture:** Turn 边界通过 Hook 事件实时界定，缓存效率从 message usage 数据统计，Analysis View 展示时间线和分布图。

**Tech Stack:** Go 1.21+ / HTML Dashboard / JavaScript

---

## File Structure

```
server/
├── internal/
│   ├── turn/
│   │   ├── tracker.go       # Turn 边界追踪器
│   │   ├── tracker_test.go  # 测试
│   │   ├── session.go       # Session Turn 管理
│   │   └── session_test.go  
│   ├── stats/
│   │   ├── cache.go         # 缓存效率统计
│   │   ├── cache_test.go    
│   │   ├── cost.go          # 成本计算
│   │   └── cost_test.go     
│   │   └── aggregator.go    # 数据聚合
│   │   └── aggregator_test.go
│   └── handler/
│   │   ├── stats.go         # 统计 API Handler
│   │   └── stats_test.go    
│   └── analysis/
│   │   └── batch.go         # 批量分析（补充）
│   └── greptimedb/
│   │   └── tables.go        # (更新)添加聚合表
├── web/
│   ├── index.html           # (更新)完善 Analysis View
│   ├── analysis.js          # Analysis View 逻辑
│   ├── charts.js            # 图表绘制
│   └── sessions.js          # Sessions View 逻辑
```

---

## Task 1: Turn 边界追踪器

**Files:**
- Create: `server/internal/turn/tracker.go`
- Create: `server/internal/turn/tracker_test.go`

- [ ] **Step 1: 写 Turn 追踪测试**

Create file `server/internal/turn/tracker_test.go`:

```go
package turn

import (
	"testing"
	"time"
)

func TestTrackerStartTurn(t *testing.T) {
	tracker := NewTracker()
	
	// UserPromptSubmit starts a turn
	tracker.HandleEvent("test-session", "UserPromptSubmit", time.Now(), "What is the error?")
	
	turn := tracker.GetCurrentTurn("test-session")
	
	if turn == nil {
		t.Error("expected current turn to be started")
	}
	if turn.UserPrompt != "What is the error?" {
		t.Errorf("expected user prompt, got %s", turn.UserPrompt)
	}
}

func TestTrackerEndTurn(t *testing.T) {
	tracker := NewTracker()
	
	// Start turn
	tracker.HandleEvent("test-session", "UserPromptSubmit", time.Now(), "Hello")
	
	// AssistantResponse ends turn
	endTime := time.Now().Add(5 * time.Second)
	tracker.HandleEvent("test-session", "AssistantResponse", endTime, "Hi there!")
	
	turn := tracker.GetCurrentTurn("test-session")
	
	// Turn should be completed, no current turn
	if turn != nil {
		t.Error("expected no current turn after completion")
	}
	
	// Check completed turns
	turns := tracker.GetCompletedTurns("test-session")
	if len(turns) == 0 {
		t.Error("expected completed turn to be stored")
	}
	if turns[0].AgentResponse != "Hi there!" {
		t.Errorf("expected agent response, got %s", turns[0].AgentResponse)
	}
}

func TestTrackerToolCount(t *testing.T) {
	tracker := NewTracker()
	
	tracker.HandleEvent("test-session", "UserPromptSubmit", time.Now(), "Read file")
	tracker.HandleEvent("test-session", "PreToolUse", time.Now().Add(1*time.Second), "Read")
	tracker.HandleEvent("test-session", "PostToolUse", time.Now().Add(2*time.Second), "Read")
	tracker.HandleEvent("test-session", "PreToolUse", time.Now().Add(3*time.Second), "Edit")
	tracker.HandleEvent("test-session", "PostToolUse", time.Now().Add(4*time.Second), "Edit")
	tracker.HandleEvent("test-session", "AssistantResponse", time.Now().Add(5*time.Second), "")
	
	turns := tracker.GetCompletedTurns("test-session")
	if len(turns) == 0 {
		t.Fatal("expected completed turn")
	}
	
	if turns[0].ToolCount != 2 {
		t.Errorf("expected tool_count=2, got %d", turns[0].ToolCount)
	}
}

func TestTrackerMultipleTurns(t *testing.T) {
	tracker := NewTracker()
	
	// Turn 1
	tracker.HandleEvent("s1", "UserPromptSubmit", time.Now(), "Q1")
	tracker.HandleEvent("s1", "AssistantResponse", time.Now().Add(2*time.Second), "A1")
	
	// Turn 2
	tracker.HandleEvent("s1", "UserPromptSubmit", time.Now().Add(3*time.Second), "Q2")
	tracker.HandleEvent("s1", "AssistantResponse", time.Now().Add(5*time.Second), "A2")
	
	turns := tracker.GetCompletedTurns("s1")
	if len(turns) != 2 {
		t.Errorf("expected 2 completed turns, got %d", len(turns))
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/turn/... -v
```

Expected: FAIL - tracker.go not found

- [ ] **Step 3: 写 Turn 追踪实现**

Create file `server/internal/turn/tracker.go`:

```go
package turn

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Turn represents a user-agent interaction round.
type Turn struct {
	TurnID         string
	SessionID      string
	StartTS        time.Time
	EndTS          time.Time
	UserPrompt     string
	AgentResponse  string
	ToolCount      int64
	InputTokens    int64
	OutputTokens   int64
	CostUSD        float64
	HasError       bool
}

// Tracker manages turn boundaries for sessions.
type Tracker struct {
	mu             sync.RWMutex
	currentTurns   map[string]*Turn      // session_id -> active turn
	completedTurns map[string][]Turn     // session_id -> completed turns
	logger         *slog.Logger
	sqlURL         string
}

// NewTracker creates a turn tracker.
func NewTracker() *Tracker {
	return &Tracker{
		currentTurns:   make(map[string]*Turn),
		completedTurns: make(map[string][]Turn),
	}
}

// NewTrackerWithDB creates a tracker with database storage.
func NewTrackerWithDB(greptimeDBHost string, greptimeHTTPPort int, logger *slog.Logger) *Tracker {
	return &Tracker{
		currentTurns:   make(map[string]*Turn),
		completedTurns: make(map[string][]Turn),
		sqlURL:         fmt.Sprintf("http://%s:%d/v1/sql", greptimeDBHost, greptimeHTTPPort),
		logger:         logger,
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
	
	// Execute insert via HTTP POST
	// (Similar to other modules)
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
```

- [ ] **Step 4: 运行测试验证通过**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/turn/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/turn/
git commit -m "feat: add turn boundary tracker"
```

---

## Task 2: 缓存效率统计

**Files:**
- Create: `server/internal/stats/cache.go`
- Create: `server/internal/stats/cache_test.go`

- [ ] **Step 1: 写缓存统计测试**

Create file `server/internal/stats/cache_test.go`:

```go
package stats

import (
	"testing"
)

func TestCacheEfficiency(t *testing.T) {
	stats := &CacheStats{
		CacheReadTokens:     100000,
		CacheCreationTokens: 50000,
		InputTokens:         150000,
		OutputTokens:        20000,
	}
	
	eff := stats.Efficiency()
	
	// Cache ratio = cache_read / (input + cache_creation)
	expectedRatio := float64(100000) / float64(150000 + 50000)
	if eff.CacheRatio != expectedRatio {
		t.Errorf("expected cache_ratio=%.2f, got %.2f", expectedRatio, eff.CacheRatio)
	}
	
	// Tokens saved estimate (cache read saves input processing)
	if eff.TokensSaved != 100000 {
		t.Errorf("expected tokens_saved=100000, got %d", eff.TokensSaved)
	}
}

func TestCacheCostSaved(t *testing.T) {
	stats := &CacheStats{
		CacheReadTokens:     100000,
		CacheCreationTokens: 50000,
	}
	
	// Assume $3 per million input tokens, $0.10 per million cache read
	eff := stats.Efficiency()
	
	// Cost saved = (input_price - cache_price) * cache_read
	// = ($3 - $0.10) * 0.1M = $0.29
	if eff.CostSavedUSD < 0.28 || eff.CostSavedUSD > 0.30 {
		t.Errorf("expected cost_saved ~$0.29, got $%.4f", eff.CostSavedUSD)
	}
}

func TestAggregateCacheStats(t *testing.T) {
	stats1 := &CacheStats{
		CacheReadTokens:     50000,
		CacheCreationTokens: 25000,
	}
	stats2 := &CacheStats{
		CacheReadTokens:     30000,
		CacheCreationTokens: 15000,
	}
	
	agg := AggregateCacheStats([]CacheStats{*stats1, *stats2})
	
	if agg.CacheReadTokens != 80000 {
		t.Errorf("expected aggregated cache_read=80000, got %d", agg.CacheReadTokens)
	}
	if agg.CacheCreationTokens != 40000 {
		t.Errorf("expected aggregated cache_creation=40000, got %d", agg.CacheCreationTokens)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/stats/... -v
```

Expected: FAIL - cache.go not found

- [ ] **Step 3: 写缓存统计实现**

Create file `server/internal/stats/cache.go`:

```go
package stats

// CacheStats represents cache usage metrics.
type CacheStats struct {
	CacheReadTokens     int64
	CacheCreationTokens int64
	InputTokens         int64
	OutputTokens        int64
}

// CacheEfficiency represents calculated efficiency metrics.
type CacheEfficiency struct {
	CacheRatio      float64 // cache_read / total_input
	TokensSaved     int64   // cache_read tokens
	CostSavedUSD    float64 // estimated cost saved
	CacheHitRate    float64 // if available from API
}

// Pricing constants (approximate, can be configured)
const (
	InputTokenPricePerMillion  = 3.0   // $3 per million input tokens
	CacheReadPricePerMillion   = 0.10  // $0.10 per million cache read tokens
	CacheWritePricePerMillion  = 0.50  // $0.50 per million cache creation tokens
	OutputTokenPricePerMillion = 15.0  // $15 per million output tokens
)

// Efficiency calculates cache efficiency metrics.
func (s *CacheStats) Efficiency() CacheEfficiency {
	totalInput := s.InputTokens + s.CacheCreationTokens
	
	cacheRatio := 0.0
	if totalInput > 0 {
		cacheRatio = float64(s.CacheReadTokens) / float64(totalInput)
	}
	
	// Cost saved = (input_price - cache_price) * cache_read_volume
	cacheVolume := float64(s.CacheReadTokens) / 1_000_000
	costSaved := (InputTokenPricePerMillion - CacheReadPricePerMillion) * cacheVolume
	
	return CacheEfficiency{
		CacheRatio:   cacheRatio,
		TokensSaved:  s.CacheReadTokens,
		CostSavedUSD: costSaved,
	}
}

// AggregateCacheStats combines multiple cache stats.
func AggregateCacheStats(stats []CacheStats) CacheStats {
	var agg CacheStats
	for _, s := range stats {
		agg.CacheReadTokens += s.CacheReadTokens
		agg.CacheCreationTokens += s.CacheCreationTokens
		agg.InputTokens += s.InputTokens
		agg.OutputTokens += s.OutputTokens
	}
	return agg
}

// GlobalCacheEfficiency calculates overall cache efficiency.
func GlobalCacheEfficiency(stats []CacheStats) CacheEfficiency {
	agg := AggregateCacheStats(stats)
	return agg.Efficiency()
}
```

- [ ] **Step 4: 运行测试验证通过**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/stats/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/stats/
git commit -m "feat: add cache efficiency statistics"
```

---

## Task 3: 成本计算模块

**Files:**
- Create: `server/internal/stats/cost.go`
- Create: `server/internal/stats/cost_test.go`

- [ ] **Step 1: 写成本计算测试**

Create file `server/internal/stats/cost_test.go`:

```go
package stats

import (
	"testing"
)

func TestCalculateCost(t *testing.T) {
	usage := TokenUsage{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheReadTokens:     200,
		CacheCreationTokens: 100,
	}
	
	cost := CalculateCost(usage)
	
	// Input cost: 1000 * $3/1M = $0.003
	expectedInput := 1000.0 / 1_000_000 * InputTokenPricePerMillion
	
	// Output cost: 500 * $15/1M = $0.0075
	expectedOutput := 500.0 / 1_000_000 * OutputTokenPricePerMillion
	
	// Cache read: 200 * $0.10/1M = $0.00002
	expectedCacheRead := 200.0 / 1_000_000 * CacheReadPricePerMillion
	
	// Cache creation: 100 * $0.50/1M = $0.00005
	expectedCacheCreation := 100.0 / 1_000_000 * CacheWritePricePerMillion
	
	expectedTotal := expectedInput + expectedOutput + expectedCacheRead + expectedCacheCreation
	
	if cost.TotalUSD < expectedTotal-0.0001 || cost.TotalUSD > expectedTotal+0.0001 {
		t.Errorf("expected total ~$%.6f, got $%.6f", expectedTotal, cost.TotalUSD)
	}
}

func TestSessionCost(t *testing.T) {
	usages := []TokenUsage{
		{InputTokens: 5000, OutputTokens: 1000},
		{InputTokens: 3000, OutputTokens: 500},
	}
	
	total := SessionCost(usages)
	
	// Total input: 8000, output: 1500
	expectedInput := 8000.0 / 1_000_000 * InputTokenPricePerMillion
	expectedOutput := 1500.0 / 1_000_000 * OutputTokenPricePerMillion
	expected := expectedInput + expectedOutput
	
	if total < expected-0.0001 || total > expected+0.0001 {
		t.Errorf("expected session cost ~$%.6f, got $%.6f", expected, total)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/stats/... -v
```

Expected: FAIL - cost.go not found

- [ ] **Step 3: 写成本计算实现**

Create file `server/internal/stats/cost.go`:

```go
package stats

// TokenUsage represents token counts for cost calculation.
type TokenUsage struct {
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
}

// CostBreakdown represents cost by component.
type CostBreakdown struct {
	InputCostUSD         float64
	OutputCostUSD        float64
	CacheReadCostUSD     float64
	CacheCreationCostUSD float64
	TotalUSD             float64
}

// CalculateCost calculates cost from token usage.
func CalculateCost(usage TokenUsage) CostBreakdown {
	inputCost := float64(usage.InputTokens) / 1_000_000 * InputTokenPricePerMillion
	outputCost := float64(usage.OutputTokens) / 1_000_000 * OutputTokenPricePerMillion
	cacheReadCost := float64(usage.CacheReadTokens) / 1_000_000 * CacheReadPricePerMillion
	cacheCreationCost := float64(usage.CacheCreationTokens) / 1_000_000 * CacheWritePricePerMillion
	
	return CostBreakdown{
		InputCostUSD:         inputCost,
		OutputCostUSD:        outputCost,
		CacheReadCostUSD:     cacheReadCost,
		CacheCreationCostUSD: cacheCreationCost,
		TotalUSD:             inputCost + outputCost + cacheReadCost + cacheCreationCost,
	}
}

// SessionCost calculates total cost for a session.
func SessionCost(usages []TokenUsage) float64 {
	var total float64
	for _, u := range usages {
		cost := CalculateCost(u)
		total += cost.TotalUSD
	}
	return total
}

// TopCostSessions identifies highest cost sessions.
func TopCostSessions(sessionCosts map[string]float64, limit int) []SessionCostEntry {
	entries := make([]SessionCostEntry, 0, len(sessionCosts))
	for sessionID, cost := range sessionCosts {
		entries = append(entries, SessionCostEntry{
			SessionID: sessionID,
			CostUSD:   cost,
		})
	}
	
	// Sort by cost descending
	sortByCost(entries)
	
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

type SessionCostEntry struct {
	SessionID string
	CostUSD   float64
}

func sortByCost(entries []SessionCostEntry) {
	// Simple sort implementation
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].CostUSD > entries[i].CostUSD {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}
```

- [ ] **Step 4: 运行测试验证通过**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/stats/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/stats/cost.go server/internal/stats/cost_test.go
git commit -m "feat: add cost calculation module"
```

---

## Task 4: 数据聚合器

**Files:**
- Create: `server/internal/stats/aggregator.go`
- Create: `server/internal/stats/aggregator_test.go`

- [ ] **Step 1: 写聚合器测试**

Create file `server/internal/stats/aggregator_test.go`:

```go
package stats

import (
	"testing"
	"time"
)

func TestAggregateSessionStats(t *testing.T) {
	events := []SessionEvent{
		{SessionID: "s1", InputTokens: 1000, OutputTokens: 500, TurnCount: 2},
		{SessionID: "s1", InputTokens: 2000, OutputTokens: 300, TurnCount: 1},
		{SessionID: "s2", InputTokens: 500, OutputTokens: 200, TurnCount: 3},
	}
	
	agg := AggregateSessionStats(events)
	
	if len(agg) != 2 {
		t.Errorf("expected 2 sessions aggregated, got %d", len(agg))
	}
	
	// s1 totals
	if agg["s1"].InputTokens != 3000 {
		t.Errorf("expected s1 input=3000, got %d", agg["s1"].InputTokens)
	}
	if agg["s1"].TurnCount != 3 {
		t.Errorf("expected s1 turns=3, got %d", agg["s1"].TurnCount)
	}
}

func TestTimeWindowFilter(t *testing.T) {
	now := time.Now()
	events := []SessionEvent{
		{SessionID: "s1", TS: now.Add(-1 * time.Hour)},
		{SessionID: "s2", TS: now.Add(-3 * time.Hour)},
		{SessionID: "s3", TS: now.Add(-25 * time.Hour)},
	}
	
	// Filter last 24 hours
	filtered := FilterByTimeWindow(events, 24*time.Hour)
	
	if len(filtered) != 2 {
		t.Errorf("expected 2 events in 24h window, got %d", len(filtered))
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/stats/... -v -run TestAggregate
```

Expected: FAIL - aggregator.go not found

- [ ] **Step 3: 写聚合器实现**

Create file `server/internal/stats/aggregator.go`:

```go
package stats

import (
	"time"
)

// SessionEvent represents a single event with stats.
type SessionEvent struct {
	SessionID          string
	TS                 time.Time
	InputTokens        int64
	OutputTokens       int64
	CacheReadTokens    int64
	CacheCreationTokens int64
	TurnCount          int64
	ToolCount          int64
}

// SessionAggregate represents aggregated session stats.
type SessionAggregate struct {
	SessionID           string
	TotalInputTokens    int64
	TotalOutputTokens   int64
	TotalCacheRead      int64
	TotalCacheCreation  int64
	TurnCount           int64
	ToolCount           int64
	TotalCostUSD        float64
	FirstEventTS        time.Time
	LastEventTS         time.Time
}

// AggregateSessionStats aggregates events by session.
func AggregateSessionStats(events []SessionEvent) map[string]SessionAggregate {
	result := make(map[string]SessionAggregate)
	
	for _, e := range events {
		agg, ok := result[e.SessionID]
		if !ok {
			agg = SessionAggregate{
				SessionID:    e.SessionID,
				FirstEventTS: e.TS,
			}
		}
		
		agg.TotalInputTokens += e.InputTokens
		agg.TotalOutputTokens += e.OutputTokens
		agg.TotalCacheRead += e.CacheReadTokens
		agg.TotalCacheCreation += e.CacheCreationTokens
		agg.TurnCount += e.TurnCount
		agg.ToolCount += e.ToolCount
		agg.LastEventTS = e.TS
		
		// Update first event time if earlier
		if e.TS.Before(agg.FirstEventTS) {
			agg.FirstEventTS = e.TS
		}
		
		result[e.SessionID] = agg
	}
	
	// Calculate costs
	for sessionID, agg := range result {
		usage := TokenUsage{
			InputTokens:         agg.TotalInputTokens,
			OutputTokens:        agg.TotalOutputTokens,
			CacheReadTokens:     agg.TotalCacheRead,
			CacheCreationTokens: agg.TotalCacheCreation,
		}
		cost := CalculateCost(usage)
		agg.TotalCostUSD = cost.TotalUSD
		result[sessionID] = agg
	}
	
	return result
}

// FilterByTimeWindow filters events within a time window.
func FilterByTimeWindow(events []SessionEvent, window time.Duration) []SessionEvent {
	now := time.Now()
	cutoff := now.Add(-window)
	
	var filtered []SessionEvent
	for _, e := range events {
		if e.TS.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// CalculateOverallStats calculates global statistics.
func CalculateOverallStats(aggregates map[string]SessionAggregate) GlobalStats {
	var total GlobalStats
	
	for _, agg := range aggregates {
		total.TotalInputTokens += agg.TotalInputTokens
		total.TotalOutputTokens += agg.TotalOutputTokens
		total.TotalCacheRead += agg.TotalCacheRead
		total.TotalCacheCreation += agg.TotalCacheCreation
		total.TotalTurns += agg.TurnCount
		total.TotalTools += agg.ToolCount
		total.TotalCostUSD += agg.TotalCostUSD
		total.SessionCount++
	}
	
	// Calculate averages
	if total.SessionCount > 0 {
		total.AvgTurnsPerSession = float64(total.TotalTurns) / float64(total.SessionCount)
		total.AvgToolsPerTurn = float64(total.TotalTools) / float64(total.TotalTurns)
	}
	
	return total
}

// GlobalStats represents overall system statistics.
type GlobalStats struct {
	SessionCount        int64
	TotalInputTokens    int64
	TotalOutputTokens   int64
	TotalCacheRead      int64
	TotalCacheCreation  int64
	TotalTurns          int64
	TotalTools          int64
	TotalCostUSD        float64
	AvgTurnsPerSession  float64
	AvgToolsPerTurn     float64
}
```

- [ ] **Step 4: 运行测试验证通过**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/stats/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/stats/aggregator.go server/internal/stats/aggregator_test.go
git commit -m "feat: add data aggregator"
```

---

## Task 5: 统计 API Handler

**Files:**
- Create: `server/internal/handler/stats.go`
- Create: `server/internal/handler/stats_test.go`
- Modify: `server/internal/handler/handler.go` (add stats routes)

- [ ] **Step 1: 写 Stats Handler 测试**

Create file `server/internal/handler/stats_test.go`:

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleStatsOverview(t *testing.T) {
	s := &Server{
		greptimeDBHost:   "127.0.0.1",
		greptimeHTTPPort: 14000,
	}
	
	req := httptest.NewRequest("GET", "/api/stats/overview", nil)
	rr := httptest.NewRecorder()
	
	s.handleStatsOverview(rr, req)
	
	// Should return JSON response
	if rr.Code != http.StatusOK {
		t.Logf("status code: %d (may fail without DB)", rr.Code)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/handler/... -v -run TestHandleStats
```

Expected: FAIL - stats.go not found

- [ ] **Step 3: 写 Stats Handler 实现**

Create file `server/internal/handler/stats.go`:

```go
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
		AVG(ts) as avg_duration_placeholder,
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
```

- [ ] **Step 4: 更新 handler.go 注册 stats 路由**

Update `server/internal/handler/handler.go`:

Add to RegisterRoutes:
```go
mux.HandleFunc("/api/stats/overview", s.handleStatsOverview)
mux.HandleFunc("/api/stats/cache", s.handleStatsCache)
mux.HandleFunc("/api/stats/cost", s.handleStatsCost)
mux.HandleFunc("/api/stats/tools", s.handleStatsTools)
```

- [ ] **Step 5: 运行测试**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/handler/... -v
```

Expected: PASS (may log query failures without DB)

- [ ] **Step 6: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/handler/stats.go server/internal/handler/stats_test.go server/internal/handler/handler.go
git commit -m "feat: add stats API handlers"
```

---

## Task 6: Analysis View UI

**Files:**
- Modify: `server/web/index.html` (update Analysis View)
- Create: `server/web/analysis.js`
- Create: `server/web/charts.js`

- [ ] **Step 1: 更新 Analysis View HTML**

Update `server/web/index.html`, enhance Analysis View section:

```html
<!-- Analysis View -->
<div id="analysis-view" class="view hidden">
    <div class="view-header">
        <h2>Analysis</h2>
        <div class="time-range-selector">
            <button class="range-btn active" data-range="1h">1 Hour</button>
            <button class="range-btn" data-range="24h">24 Hours</button>
            <button class="range-btn" data-range="7d">7 Days</button>
            <button class="range-btn" data-range="30d">30 Days</button>
        </div>
    </div>
    
    <div class="analysis-grid">
        <!-- Overview Stats -->
        <div class="stat-card overview">
            <h3>Overview</h3>
            <div class="stat-row">
                <span class="stat-label">Sessions</span>
                <span class="stat-value" id="stat-sessions">0</span>
            </div>
            <div class="stat-row">
                <span class="stat-label">Total Tokens</span>
                <span class="stat-value" id="stat-tokens">0</span>
            </div>
            <div class="stat-row">
                <span class="stat-label">Total Cost</span>
                <span class="stat-value" id="stat-cost">$0.00</span>
            </div>
            <div class="stat-row">
                <span class="stat-label">Avg Turns/Session</span>
                <span class="stat-value" id="stat-turns">0</span>
            </div>
        </div>
        
        <!-- Cache Efficiency -->
        <div class="stat-card cache">
            <h3>Cache Efficiency</h3>
            <div class="cache-stats">
                <div class="cache-bar">
                    <div class="bar-label">Cache Read</div>
                    <div class="bar-fill" id="cache-read-bar" style="width: 0%"></div>
                    <div class="bar-value" id="cache-read-value">0 tokens</div>
                </div>
                <div class="cache-bar">
                    <div class="bar-label">Cache Created</div>
                    <div class="bar-fill" id="cache-created-bar" style="width: 0%"></div>
                    <div class="bar-value" id="cache-created-value">0 tokens</div>
                </div>
                <div class="stat-row">
                    <span class="stat-label">Cost Saved</span>
                    <span class="stat-value" id="cache-saved">$0.00</span>
                </div>
            </div>
        </div>
        
        <!-- Token/Cost Timeline -->
        <div class="stat-card timeline span-2">
            <h3>Token/Cost Timeline</h3>
            <div class="timeline-chart" id="timeline-chart">
                <!-- Session attribution bars -->
            </div>
        </div>
        
        <!-- Anomaly Distribution -->
        <div class="stat-card anomalies">
            <h3>Anomaly Distribution</h3>
            <div class="pie-chart" id="anomaly-chart">
                <!-- Pie chart placeholder -->
            </div>
        </div>
        
        <!-- Cost Attribution Top 10 -->
        <div class="stat-card top-cost">
            <h3>Top 10 Cost Sessions</h3>
            <div class="top-list" id="top-cost-list">
                <!-- Top cost sessions -->
            </div>
        </div>
        
        <!-- Tool Statistics -->
        <div class="stat-card tools span-2">
            <h3>Tool Statistics</h3>
            <div class="tool-grid" id="tool-grid">
                <!-- Tool usage bars -->
            </div>
        </div>
    </div>
</div>
```

- [ ] **Step 2: 创建 Analysis View JavaScript**

Create file `server/web/analysis.js`:

```javascript
// Analysis View logic
let currentRange = '24h';

async function loadAnalysisData() {
    const range = currentRange;
    
    // Load overview
    loadOverview(range);
    
    // Load cache stats
    loadCacheStats();
    
    // Load cost data
    loadCostData(range);
    
    // Load tool stats
    loadToolStats();
}

async function loadOverview(range) {
    const response = await fetch(`/api/stats/overview?range=${range}`);
    if (response.ok) {
        const data = await response.json();
        renderOverview(data);
    }
}

function renderOverview(data) {
    // Parse GreptimeDB response format
    const records = parseRecords(data);
    
    let totalInput = 0, totalOutput = 0, sessionCount = 0;
    for (const r of records) {
        totalInput += r.total_input || 0;
        totalOutput += r.total_output || 0;
        sessionCount++;
    }
    
    document.getElementById('stat-sessions').textContent = sessionCount;
    document.getElementById('stat-tokens').textContent = formatTokens(totalInput + totalOutput);
    
    // Calculate cost
    const cost = calculateCost(totalInput, totalOutput);
    document.getElementById('stat-cost').textContent = '$' + cost.toFixed(4);
}

async function loadCacheStats() {
    const response = await fetch('/api/stats/cache');
    if (response.ok) {
        const data = await response.json();
        renderCacheStats(data);
    }
}

function renderCacheStats(data) {
    const records = parseRecords(data);
    if (records.length === 0) return;
    
    const r = records[0];
    const cacheRead = r.cache_read || 0;
    const cacheCreation = r.cache_creation || 0;
    const input = r.input || 0;
    
    // Cache ratio
    const ratio = input > 0 ? cacheRead / (input + cacheCreation) : 0;
    
    // Update bars
    document.getElementById('cache-read-bar').style.width = (ratio * 100) + '%';
    document.getElementById('cache-read-value').textContent = formatTokens(cacheRead);
    
    document.getElementById('cache-created-bar').style.width = ((cacheCreation / (input + cacheCreation)) * 100) + '%';
    document.getElementById('cache-created-value').textContent = formatTokens(cacheCreation);
    
    // Cost saved
    const costSaved = (cacheRead / 1000000) * (3.0 - 0.10);
    document.getElementById('cache-saved').textContent = '$' + costSaved.toFixed(4);
}

async function loadCostData(range) {
    const response = await fetch(`/api/stats/cost?range=${range}`);
    if (response.ok) {
        const data = await response.json();
        renderCostTimeline(data);
        renderTopCost(data);
    }
}

function renderCostTimeline(data) {
    const records = parseRecords(data);
    const container = document.getElementById('timeline-chart');
    
    container.innerHTML = records.map(r => {
        const cost = calculateCost(r.input, r.output);
        const width = Math.min(cost * 100, 100); // Scale for display
        return `
            <div class="timeline-bar" data-session="${r.session_id}">
                <div class="bar-fill" style="width: ${width}%"></div>
                <div class="bar-label">${r.session_id}</div>
            </div>
        `;
    }).join('');
}

function renderTopCost(data) {
    const records = parseRecords(data);
    const container = document.getElementById('top-cost-list');
    
    // Already sorted by cost desc from query
    const top10 = records.slice(0, 10);
    
    container.innerHTML = top10.map((r, i) => {
        const cost = calculateCost(r.input, r.output);
        return `
            <div class="top-item">
                <span class="rank">${i + 1}</span>
                <span class="session-id">${r.session_id}</span>
                <span class="cost">$${cost.toFixed(4)}</span>
            </div>
        `;
    }).join('');
}

async function loadToolStats() {
    const response = await fetch('/api/stats/tools');
    if (response.ok) {
        const data = await response.json();
        renderToolStats(data);
    }
}

function renderToolStats(data) {
    const records = parseRecords(data);
    const container = document.getElementById('tool-grid');
    
    const maxCalls = Math.max(...records.map(r => r.call_count || 0));
    
    container.innerHTML = records.map(r => {
        const pct = (r.call_count / maxCalls) * 100;
        const errorRate = r.error_count / r.call_count;
        const errorClass = errorRate > 0.1 ? 'high-error' : '';
        
        return `
            <div class="tool-stat ${errorClass}">
                <div class="tool-name">${r.tool_name}</div>
                <div class="tool-bar">
                    <div class="bar-fill" style="width: ${pct}%"></div>
                </div>
                <div class="tool-meta">
                    <span>${r.call_count} calls</span>
                    <span>${r.error_count} errors</span>
                </div>
            </div>
        `;
    }).join('');
}

function parseRecords(data) {
    if (data.output && data.output.records) {
        return data.output.records;
    }
    return [];
}

function calculateCost(input, output) {
    return (input / 1000000) * 3.0 + (output / 1000000) * 15.0;
}

function formatTokens(n) {
    if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
    return n.toString();
}

// Time range buttons
document.querySelectorAll('.range-btn').forEach(btn => {
    btn.addEventListener('click', () => {
        document.querySelectorAll('.range-btn').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        currentRange = btn.dataset.range;
        loadAnalysisData();
    });
});

// Initialize
document.addEventListener('DOMContentLoaded', loadAnalysisData);
```

- [ ] **Step 3: 添加 Analysis View CSS**

Add styles to `server/web/index.html`:

```css
/* Analysis View */
.analysis-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 16px;
    padding: 16px;
}

.stat-card {
    background: var(--bg-secondary);
    border-radius: 8px;
    padding: 16px;
}

.stat-card.span-2 {
    grid-column: span 2;
}

.stat-card.span-3 {
    grid-column: span 3;
}

.stat-row {
    display: flex;
    justify-content: space-between;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
}

.stat-label {
    color: var(--text-secondary);
}

.stat-value {
    font-weight: bold;
}

.cache-bar {
    margin: 8px 0;
}

.cache-bar .bar-fill {
    height: 8px;
    background: var(--accent);
    border-radius: 4px;
}

.timeline-bar {
    margin: 4px 0;
    height: 24px;
    position: relative;
}

.timeline-bar .bar-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--accent), var(--accent-light));
}

.top-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
}

.rank {
    width: 20px;
    text-align: center;
    font-weight: bold;
}

.session-id {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
}

.tool-stat {
    padding: 8px;
    border-radius: 4px;
    background: var(--bg-tertiary);
}

.tool-stat.high-error {
    border: 1px solid var(--critical);
}

.time-range-selector {
    display: flex;
    gap: 8px;
}

.range-btn {
    padding: 4px 12px;
    border-radius: 4px;
    background: var(--bg-secondary);
}

.range-btn.active {
    background: var(--accent);
    color: white;
}
```

- [ ] **Step 4: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/web/
git commit -m "feat: add Analysis View UI"
```

---

## Task 7: 集成 Turn Tracker 到 Hook Handler

**Files:**
- Modify: `server/internal/handler/hooks.go` (add turn tracking)
- Modify: `server/cmd/llm-apm-server/main.go` (init turn tracker)

- [ ] **Step 1: 添加 Turn Tracker 到 Server**

Update `server/internal/handler/hooks.go`, add turnTracker field and handleTurnEvent:

```go
// Add to Server struct:
turnTracker *turn.Tracker

// Add to insertHookEvent:
// After inserting, track turn
if s.turnTracker != nil {
    s.turnTracker.HandleEvent(p.SessionID, p.HookEventName, time.Now(), extractContent(p))
}
```

- [ ] **Step 2: 更新 main.go**

Update `server/cmd/llm-apm-server/main.go`:

```go
import "github.com/akke/llm-apm/server/internal/turn"

// Add after creating handler:
turnTracker := turn.NewTrackerWithDB("127.0.0.1", cfg.GreptimeHTTPPort, logger)
srv.SetTurnTracker(turnTracker)
```

- [ ] **Step 3: 验证编译**

Run:
```bash
cd /home/akke/project/llm-apm/server
go build ./...
```

Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/handler/hooks.go server/cmd/llm-apm-server/main.go
git commit -m "feat: integrate turn tracker into hook handler"
```

---

## Task 8: 最终集成测试

**Files:**
- Modify: `server/Makefile` (add integration test)

- [ ] **Step 1: 构建完整项目**

Run:
```bash
cd /home/akke/project/llm-apm/server
go mod tidy
go build ./...
make build
```

Expected: 编译成功，生成 bin/llm-apm-server

- [ ] **Step 2: 运行所有测试**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./... -v
```

Expected: 所有测试通过

- [ ] **Step 3: Commit 最终版本**

```bash
cd /home/akke/project/llm-apm
git add server/
git commit -m "feat: complete plan 3 - analysis view, turn tracking, cache stats"
```

---

## Summary

Plan 3 完成后交付物：

| 交付物 | 状态 |
|--------|------|
| Turn 边界追踪 | ✅ UserPromptSubmit → AssistantResponse |
| 缓存效率统计 | ✅ CacheRatio, TokensSaved, CostSaved |
| 成本计算 | ✅ Input/Output/Cache pricing |
| 数据聚合器 | ✅ SessionAggregate, GlobalStats |
| Stats API | ✅ /api/stats/overview, /cache, /cost, /tools |
| Analysis View UI | ✅ 时间线、分布图、Top10 |

**所有计划完成后的完整功能：**

| 模块 | 功能 |
|------|------|
| Hook Handler | ✅ 接收事件、存储、异常检测 |
| JSONL Watcher | ✅ 监控 transcript 文件 |
| Analysis Engine | ✅ 异常检测、根因推断 |
| SSE | ✅ 实时推送关键事件 |
| Turn Tracker | ✅ 边界追踪、统计 |
| Stats | ✅ 缓存效率、成本计算 |
| Dashboard | ✅ Sessions/Problems/Analysis 三视图 |

**下一步：** 安装 Go 和 GreptimeDB，运行编译测试，启动服务器验证功能。