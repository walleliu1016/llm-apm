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