package analysis

import (
	"testing"
	"time"
)

func TestSlowToolRule(t *testing.T) {
	rule := SlowToolRule()

	event := HookEvent{
		EventType: "PostToolUse",
		ToolName:  "Bash",
		Duration:  35 * time.Second,
		SessionID: "test-session",
	}

	result := rule.Check(event)

	if !result.Detected {
		t.Error("expected slow_tool to be detected for 35s duration")
	}
	if result.AnomalyType != AnomalySlowTool {
		t.Errorf("expected anomaly_type=slow_tool, got %s", result.AnomalyType)
	}
	if result.Severity != SeverityMedium {
		t.Errorf("expected severity=medium, got %s", result.Severity)
	}
}

func TestSlowToolCriticalRule(t *testing.T) {
	rule := SlowToolCriticalRule()

	event := HookEvent{
		EventType: "PostToolUse",
		ToolName:  "Bash",
		Duration:  65 * time.Second,
		SessionID: "test-session",
	}

	result := rule.Check(event)

	if !result.Detected {
		t.Error("expected slow_tool_critical to be detected for 65s duration")
	}
	if result.Severity != SeverityCritical {
		t.Errorf("expected severity=critical, got %s", result.Severity)
	}
}

func TestErrorSpikeRule(t *testing.T) {
	rule := ErrorSpikeRule()

	// Simulate error spike: 4 failures in 1 minute
	events := []HookEvent{
		{EventType: "PostToolUseFailure", TS: time.Now().Add(-30 * time.Second)},
		{EventType: "PostToolUseFailure", TS: time.Now().Add(-20 * time.Second)},
		{EventType: "PostToolUseFailure", TS: time.Now().Add(-10 * time.Second)},
		{EventType: "PostToolUseFailure", TS: time.Now()},
	}

	result := rule.CheckBatch(events)

	if !result.Detected {
		t.Error("expected error_spike to be detected")
	}
	if result.Severity != SeverityCritical {
		t.Errorf("expected severity=critical, got %s", result.Severity)
	}
}

func TestHighCostTurnRule(t *testing.T) {
	rule := HighCostTurnRule()

	event := TurnEvent{
		SessionID:    "test-session",
		InputTokens:  60000,
		OutputTokens: 10000,
		CostUSD:      0.5,
	}

	result := rule.CheckTurn(event)

	if !result.Detected {
		t.Error("expected high_cost_turn for 70k tokens")
	}
	if result.Severity != SeverityHigh {
		t.Errorf("expected severity=high, got %s", result.Severity)
	}
}