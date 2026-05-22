package turn

import (
	"testing"
	"time"
)

func TestTrackerStartTurn(t *testing.T) {
	tracker := NewTracker()

	// UserPromptSubmit starts a turn
	tracker.HandleEvent("test-session", "UserPromptSubmit", time.Now(), "What is the error?")

	turn := tracker.GetCurrentTurn("test-session")

	if turn == nil {
		t.Error("expected current turn to be started")
	}
	if turn.UserPrompt != "What is the error?" {
		t.Errorf("expected user prompt, got %s", turn.UserPrompt)
	}
}

func TestTrackerEndTurn(t *testing.T) {
	tracker := NewTracker()

	// Start turn
	tracker.HandleEvent("test-session", "UserPromptSubmit", time.Now(), "Hello")

	// AssistantResponse ends turn
	endTime := time.Now().Add(5 * time.Second)
	tracker.HandleEvent("test-session", "AssistantResponse", endTime, "Hi there!")

	turn := tracker.GetCurrentTurn("test-session")

	// Turn should be completed, no current turn
	if turn != nil {
		t.Error("expected no current turn after completion")
	}

	// Check completed turns
	turns := tracker.GetCompletedTurns("test-session")
	if len(turns) == 0 {
		t.Error("expected completed turn to be stored")
	}
	if turns[0].AgentResponse != "Hi there!" {
		t.Errorf("expected agent response, got %s", turns[0].AgentResponse)
	}
}

func TestTrackerToolCount(t *testing.T) {
	tracker := NewTracker()

	tracker.HandleEvent("test-session", "UserPromptSubmit", time.Now(), "Read file")
	tracker.HandleEvent("test-session", "PreToolUse", time.Now().Add(1*time.Second), "Read")
	tracker.HandleEvent("test-session", "PostToolUse", time.Now().Add(2*time.Second), "Read")
	tracker.HandleEvent("test-session", "PreToolUse", time.Now().Add(3*time.Second), "Edit")
	tracker.HandleEvent("test-session", "PostToolUse", time.Now().Add(4*time.Second), "Edit")
	tracker.HandleEvent("test-session", "AssistantResponse", time.Now().Add(5*time.Second), "")

	turns := tracker.GetCompletedTurns("test-session")
	if len(turns) == 0 {
		t.Fatal("expected completed turn")
	}

	if turns[0].ToolCount != 2 {
		t.Errorf("expected tool_count=2, got %d", turns[0].ToolCount)
	}
}

func TestTrackerMultipleTurns(t *testing.T) {
	tracker := NewTracker()

	// Turn 1
	tracker.HandleEvent("s1", "UserPromptSubmit", time.Now(), "Q1")
	tracker.HandleEvent("s1", "AssistantResponse", time.Now().Add(2*time.Second), "A1")

	// Turn 2
	tracker.HandleEvent("s1", "UserPromptSubmit", time.Now().Add(3*time.Second), "Q2")
	tracker.HandleEvent("s1", "AssistantResponse", time.Now().Add(5*time.Second), "A2")

	turns := tracker.GetCompletedTurns("s1")
	if len(turns) != 2 {
		t.Errorf("expected 2 completed turns, got %d", len(turns))
	}
}