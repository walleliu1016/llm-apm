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
	CacheRatio   float64 // cache_read / total_input
	TokensSaved  int64   // cache_read tokens
	CostSavedUSD float64 // estimated cost saved
	CacheHitRate float64 // if available from API
}

// Pricing constants (approximate, can be configured)
const (
	InputTokenPricePerMillion  = 3.0  // $3 per million input tokens
	CacheReadPricePerMillion   = 0.10 // $0.10 per million cache read tokens
	CacheWritePricePerMillion  = 0.50 // $0.50 per million cache creation tokens
	OutputTokenPricePerMillion = 15.0 // $15 per million output tokens
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