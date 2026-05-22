package analysis

import (
	"time"
)

// InferenceRule defines cause inference logic.
type InferenceRule struct {
	AnomalyType string
	InferFunc   func(anomaly AnomalyResult, relatedEvents []HookEvent) string
}

// InferCause attempts to determine root cause of an anomaly.
func InferCause(anomaly AnomalyResult, relatedEvents []HookEvent) string {
	rules := GetInferenceRules()

	for _, rule := range rules {
		if rule.AnomalyType == anomaly.AnomalyType {
			return rule.InferFunc(anomaly, relatedEvents)
		}
	}

	return ""
}

// GetInferenceRules returns all inference rules.
func GetInferenceRules() []InferenceRule {
	return []InferenceRule{
		{
			AnomalyType: AnomalySlowTool,
			InferFunc:   inferSlowToolCause,
		},
		{
			AnomalyType: AnomalyErrorSpike,
			InferFunc:   inferErrorSpikeCause,
		},
		{
			AnomalyType: AnomalyRepeatedTool,
			InferFunc:   inferRepeatedToolCause,
		},
	}
}

func inferSlowToolCause(anomaly AnomalyResult, relatedEvents []HookEvent) string {
	// Look for permission prompts near the slow tool
	toolTime := anomaly.RelatedEvent.TS

	for _, e := range relatedEvents {
		// Check for notification/permission events
		if e.EventType == "Notification" ||
			e.EventType == "PreToolUse" {
			timeDiff := toolTime.Sub(e.TS)
			// If permission prompt happened just before tool execution
			if timeDiff > 0 && timeDiff < 5*time.Minute {
				return "User permission confirmation delay - consider auto-approve settings"
			}
		}
	}

	// Default cause for slow tool
	return "Long-running process or network delay - consider timeout configuration"
}

func inferErrorSpikeCause(anomaly AnomalyResult, relatedEvents []HookEvent) string {
	// Find most common failing tool
	toolCounts := make(map[string]int)

	for _, e := range relatedEvents {
		if e.ErrorFlag && e.ToolName != "" {
			toolCounts[e.ToolName]++
		}
	}

	maxTool := ""
	maxCount := 0
	for tool, count := range toolCounts {
		if count > maxCount {
			maxTool = tool
			maxCount = count
		}
	}

	if maxTool != "" {
		return "Tool '" + maxTool + "' failing repeatedly - check permissions, API status, or input validity"
	}

	return "Multiple tool failures - check API connectivity or session state"
}

func inferRepeatedToolCause(anomaly AnomalyResult, relatedEvents []HookEvent) string {
	// Find the tool being repeated
	toolName := anomaly.RelatedEvent.ToolName

	if toolName != "" {
		return "Tool '" + toolName + "' called repeatedly after failures - review logic or increase error tolerance"
	}

	return "Repeated tool calls after failures"
}