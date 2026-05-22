package analysis

import (
	"strings"
	"testing"
	"time"
)

func TestInferSlowToolCause(t *testing.T) {
	// Scenario: slow tool with permission prompt
	anomaly := AnomalyResult{
		AnomalyType: AnomalySlowTool,
		SessionID:   "test-session",
		RelatedEvent: HookEvent{
			ToolName: "Bash",
			Duration: 45 * time.Second,
			TS:       time.Now(),
		},
	}

	// Related events showing permission prompt
	relatedEvents := []HookEvent{
		{
			EventType: "PreToolUse",
			ToolName:  "Bash",
			TS:        time.Now().Add(-45 * time.Second),
		},
		{
			EventType: "Notification",
			TS:        time.Now().Add(-44 * time.Second),
		},
	}

	cause := InferCause(anomaly, relatedEvents)

	if cause == "" {
		t.Error("expected cause to be inferred")
	}

	// Should mention permission/user confirmation
	if !strings.Contains(cause, "permission") && !strings.Contains(cause, "user") {
		t.Errorf("expected cause to mention permission or user, got: %s", cause)
	}
}

func TestInferErrorSpikeCause(t *testing.T) {
	anomaly := AnomalyResult{
		AnomalyType: AnomalyErrorSpike,
		SessionID:   "test-session",
	}

	// Related events showing same tool failing repeatedly
	relatedEvents := []HookEvent{
		{EventType: "PostToolUseFailure", ToolName: "Read", ErrorFlag: true},
		{EventType: "PostToolUseFailure", ToolName: "Read", ErrorFlag: true},
		{EventType: "PostToolUseFailure", ToolName: "Read", ErrorFlag: true},
	}

	cause := InferCause(anomaly, relatedEvents)

	if cause == "" {
		t.Error("expected cause to be inferred")
	}

	// Should mention the failing tool
	if !strings.Contains(cause, "Read") {
		t.Errorf("expected cause to mention Read tool, got: %s", cause)
	}
}