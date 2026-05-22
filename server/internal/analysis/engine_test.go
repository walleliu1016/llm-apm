package analysis

import (
	"testing"
	"time"
)

func TestEngineAnalyzeHookEvent(t *testing.T) {
	engine := NewEngine()

	event := HookEvent{
		TS:        time.Now(),
		SessionID: "test-session-123",
		EventType: "PostToolUse",
		ToolName:  "Bash",
		Duration:  45 * time.Second,
		ErrorFlag: false,
	}

	anomalies := engine.AnalyzeHookEvent(event)

	if len(anomalies) == 0 {
		t.Error("expected anomaly to be detected for slow tool")
	}

	found := false
	for _, a := range anomalies {
		if a.AnomalyType == AnomalySlowTool {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected slow_tool anomaly type")
	}
}

func TestEngineAnalyzeTurn(t *testing.T) {
	engine := NewEngine()

	turn := TurnEvent{
		SessionID:    "test-session-456",
		TurnID:       "turn-001",
		InputTokens:  55000,
		OutputTokens: 10000,
		CostUSD:      0.8,
	}

	anomalies := engine.AnalyzeTurn(turn)

	if len(anomalies) == 0 {
		t.Error("expected anomaly for high cost turn")
	}
}

func TestEngineStoreAnomaly(t *testing.T) {
	engine := NewEngine()

	anomaly := AnomalyResult{
		Detected:    true,
		AnomalyType: AnomalySlowTool,
		Severity:    SeverityMedium,
		SessionID:   "test-session",
		Description: "Test anomaly",
	}

	engine.StoreAnomaly(anomaly)

	// Retrieve stored anomalies
	anomalies := engine.GetAnomalies("test-session")

	if len(anomalies) == 0 {
		t.Error("expected stored anomaly to be retrievable")
	}
}