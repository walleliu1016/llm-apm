package stats

import (
	"time"
)

// SessionEvent represents a single event with stats.
type SessionEvent struct {
	SessionID           string
	TS                  time.Time
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	TurnCount           int64
	ToolCount           int64
}

// SessionAggregate represents aggregated session stats.
type SessionAggregate struct {
	SessionID          string
	TotalInputTokens   int64
	TotalOutputTokens  int64
	TotalCacheRead     int64
	TotalCacheCreation int64
	TurnCount          int64
	ToolCount          int64
	TotalCostUSD       float64
	FirstEventTS       time.Time
	LastEventTS        time.Time
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

// GlobalStats represents overall system statistics.
type GlobalStats struct {
	SessionCount       int64
	TotalInputTokens   int64
	TotalOutputTokens  int64
	TotalCacheRead     int64
	TotalCacheCreation int64
	TotalTurns         int64
	TotalTools         int64
	TotalCostUSD       float64
	AvgTurnsPerSession float64
	AvgToolsPerTurn    float64
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
	}
	if total.TotalTurns > 0 {
		total.AvgToolsPerTurn = float64(total.TotalTools) / float64(total.TotalTurns)
	}

	return total
}