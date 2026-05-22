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

// SessionCostEntry represents cost per session.
type SessionCostEntry struct {
	SessionID string
	CostUSD   float64
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