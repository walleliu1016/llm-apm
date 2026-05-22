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

func TestTopCostSessions(t *testing.T) {
	sessionCosts := map[string]float64{
		"s1": 0.05,
		"s2": 0.10,
		"s3": 0.03,
	}

	top := TopCostSessions(sessionCosts, 2)

	if len(top) != 2 {
		t.Errorf("expected 2 top sessions, got %d", len(top))
	}

	// Should be sorted by cost desc
	if top[0].SessionID != "s2" {
		t.Errorf("expected top session s2, got %s", top[0].SessionID)
	}
	if top[0].CostUSD != 0.10 {
		t.Errorf("expected cost 0.10, got %.4f", top[0].CostUSD)
	}
}