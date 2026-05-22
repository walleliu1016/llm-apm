package analysis

import (
	"time"
)

// AnomalyType constants
const (
	AnomalySlowTool         = "slow_tool"
	AnomalySlowToolCritical = "slow_tool_critical"
	AnomalyHighCostTurn     = "high_cost_turn"
	AnomalyTokenBurnFast    = "token_burn_fast"
	AnomalyErrorSpike       = "error_spike"
	AnomalySubagentTimeout  = "subagent_timeout"
	AnomalyRepeatedTool     = "repeated_tool"
	AnomalyContextOverflow  = "context_overflow"
	AnomalyToolRejectSpike  = "tool_reject_spike"
)

// Severity constants
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// HookEvent represents a hook event for analysis.
type HookEvent struct {
	TS          time.Time
	SessionID   string
	EventType   string
	ToolName    string
	ToolInput   string
	ToolResult  string
	Duration    time.Duration
	ErrorFlag   bool
	AgentSource string
	AgentID     string
}

// TurnEvent represents a turn summary for analysis.
type TurnEvent struct {
	SessionID    string
	TurnID       string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	ToolCount    int64
	HasError     bool
}

// AnomalyResult represents detected anomaly.
type AnomalyResult struct {
	Detected       bool
	AnomalyType    string
	Severity       string
	Description    string
	SuggestedCause string
	SessionID      string
	RelatedEvent   HookEvent
}

// Rule defines an anomaly detection rule.
type Rule interface {
	Check(event HookEvent) AnomalyResult
	CheckBatch(events []HookEvent) AnomalyResult
	CheckTurn(event TurnEvent) AnomalyResult
	Name() string
}

// BaseRule provides common rule functionality.
type BaseRule struct {
	name string
}

func (r *BaseRule) Name() string {
	return r.name
}

func (r *BaseRule) CheckBatch(events []HookEvent) AnomalyResult {
	return AnomalyResult{Detected: false}
}

func (r *BaseRule) CheckTurn(event TurnEvent) AnomalyResult {
	return AnomalyResult{Detected: false}
}

// SlowToolRule detects tool execution > 30s.
func SlowToolRule() *SlowToolRuleType {
	return &SlowToolRuleType{
		BaseRule:   BaseRule{name: AnomalySlowTool},
		Threshold:  30 * time.Second,
	}
}

type SlowToolRuleType struct {
	BaseRule
	Threshold time.Duration
}

func (r *SlowToolRuleType) Check(event HookEvent) AnomalyResult {
	if event.EventType != "PostToolUse" {
		return AnomalyResult{Detected: false}
	}

	if event.Duration >= r.Threshold && event.Duration < 60*time.Second {
		return AnomalyResult{
			Detected:     true,
			AnomalyType:  AnomalySlowTool,
			Severity:     SeverityMedium,
			Description:  "Tool execution slow: " + event.ToolName + " took " + event.Duration.String(),
			SessionID:    event.SessionID,
			RelatedEvent: event,
		}
	}
	return AnomalyResult{Detected: false}
}

// SlowToolCriticalRule detects tool execution > 60s.
func SlowToolCriticalRule() *SlowToolCriticalRuleType {
	return &SlowToolCriticalRuleType{
		BaseRule:   BaseRule{name: AnomalySlowToolCritical},
		Threshold:  60 * time.Second,
	}
}

type SlowToolCriticalRuleType struct {
	BaseRule
	Threshold time.Duration
}

func (r *SlowToolCriticalRuleType) Check(event HookEvent) AnomalyResult {
	if event.EventType != "PostToolUse" {
		return AnomalyResult{Detected: false}
	}

	if event.Duration >= r.Threshold {
		return AnomalyResult{
			Detected:       true,
			AnomalyType:    AnomalySlowToolCritical,
			Severity:       SeverityCritical,
			Description:    "Tool execution critically slow: " + event.ToolName + " took " + event.Duration.String(),
			SuggestedCause: "Consider timeout configuration or long-running process",
			SessionID:      event.SessionID,
			RelatedEvent:   event,
		}
	}
	return AnomalyResult{Detected: false}
}

// ErrorSpikeRule detects concentrated errors.
func ErrorSpikeRule() *ErrorSpikeRuleType {
	return &ErrorSpikeRuleType{
		BaseRule:   BaseRule{name: AnomalyErrorSpike},
		Threshold:  3,
		TimeWindow: 1 * time.Minute,
	}
}

type ErrorSpikeRuleType struct {
	BaseRule
	Threshold  int
	TimeWindow time.Duration
}

func (r *ErrorSpikeRuleType) Check(event HookEvent) AnomalyResult {
	return AnomalyResult{Detected: false} // Batch rule
}

func (r *ErrorSpikeRuleType) CheckBatch(events []HookEvent) AnomalyResult {
	now := time.Now()
	count := 0

	for _, e := range events {
		if e.EventType == "PostToolUseFailure" &&
			now.Sub(e.TS) <= r.TimeWindow {
			count++
		}
	}

	if count > r.Threshold {
		return AnomalyResult{
			Detected:       true,
			AnomalyType:    AnomalyErrorSpike,
			Severity:       SeverityCritical,
			Description:    "Error spike: multiple failures in 1 minute",
			SuggestedCause: "Check tool permissions, API connectivity, or input validity",
		}
	}
	return AnomalyResult{Detected: false}
}

// HighCostTurnRule detects expensive turns.
func HighCostTurnRule() *HighCostTurnRuleType {
	return &HighCostTurnRuleType{
		BaseRule:       BaseRule{name: AnomalyHighCostTurn},
		TokenThreshold: 50000,
		CostThreshold:  1.0,
	}
}

type HighCostTurnRuleType struct {
	BaseRule
	TokenThreshold int64
	CostThreshold  float64
}

func (r *HighCostTurnRuleType) Check(event HookEvent) AnomalyResult {
	return AnomalyResult{Detected: false}
}

func (r *HighCostTurnRuleType) CheckTurn(event TurnEvent) AnomalyResult {
	totalTokens := event.InputTokens + event.OutputTokens

	if totalTokens > r.TokenThreshold || event.CostUSD > r.CostThreshold {
		desc := "High cost turn"
		if totalTokens > r.TokenThreshold {
			desc += ": high token usage"
		}
		if event.CostUSD > r.CostThreshold {
			desc += ": cost exceeds threshold"
		}
		return AnomalyResult{
			Detected:    true,
			AnomalyType: AnomalyHighCostTurn,
			Severity:    SeverityHigh,
			Description: desc,
			SessionID:   event.SessionID,
		}
	}
	return AnomalyResult{Detected: false}
}

// ToolFailureRule detects tool execution failures.
func ToolFailureRule() *ToolFailureRuleType {
	return &ToolFailureRuleType{
		BaseRule: BaseRule{name: "tool_failure"},
	}
}

type ToolFailureRuleType struct {
	BaseRule
}

func (r *ToolFailureRuleType) Check(event HookEvent) AnomalyResult {
	if event.EventType == "PostToolUseFailure" {
		description := "Tool failed: " + event.ToolName
		if event.ToolInput != "" && len(event.ToolInput) > 0 {
			description += " - check input validity"
		}
		return AnomalyResult{
			Detected:       true,
			AnomalyType:    "tool_failure",
			Severity:       SeverityHigh,
			Description:    description,
			SuggestedCause: "Check tool permissions, input parameters, or external dependencies",
			SessionID:      event.SessionID,
			RelatedEvent:   event,
		}
	}
	return AnomalyResult{Detected: false}
}

// AllRules returns all detection rules.
func AllRules() []Rule {
	return []Rule{
		ToolFailureRule(),
		SlowToolRule(),
		SlowToolCriticalRule(),
		ErrorSpikeRule(),
		HighCostTurnRule(),
	}
}