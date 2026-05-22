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
	if agg["s1"].TotalInputTokens != 3000 {
		t.Errorf("expected s1 input=3000, got %d", agg["s1"].TotalInputTokens)
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

func TestCalculateOverallStats(t *testing.T) {
	aggregates := map[string]SessionAggregate{
		"s1": {TotalInputTokens: 1000, TotalOutputTokens: 500, TurnCount: 2, ToolCount: 5},
		"s2": {TotalInputTokens: 2000, TotalOutputTokens: 300, TurnCount: 1, ToolCount: 3},
	}

	stats := CalculateOverallStats(aggregates)

	if stats.SessionCount != 2 {
		t.Errorf("expected session_count=2, got %d", stats.SessionCount)
	}
	if stats.TotalInputTokens != 3000 {
		t.Errorf("expected total_input=3000, got %d", stats.TotalInputTokens)
	}
	if stats.TotalTurns != 3 {
		t.Errorf("expected total_turns=3, got %d", stats.TotalTurns)
	}
}