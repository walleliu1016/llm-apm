package broadcaster

import (
	"testing"
	"time"
)

func TestBroadcasterSubscribe(t *testing.T) {
	b := NewBroadcaster()

	client := b.Subscribe()
	defer b.Unsubscribe(client)

	if client == nil {
		t.Error("expected client channel to be created")
	}

	if b.Count() != 1 {
		t.Errorf("expected 1 subscriber, got %d", b.Count())
	}
}

func TestBroadcasterBroadcast(t *testing.T) {
	b := NewBroadcaster()

	client := b.Subscribe()
	defer b.Unsubscribe(client)

	// Broadcast a message
	msg := SSEMessage{
		Event: "anomaly",
		Data:  "test data",
	}

	b.Broadcast(msg)

	// Receive from client channel
	select {
	case received := <-client:
		if received.Event != "anomaly" {
			t.Errorf("expected event=anomaly, got %s", received.Event)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive broadcast message")
	}
}

func TestBroadcasterMultipleClients(t *testing.T) {
	b := NewBroadcaster()

	client1 := b.Subscribe()
	client2 := b.Subscribe()

	if b.Count() != 2 {
		t.Errorf("expected 2 subscribers, got %d", b.Count())
	}

	msg := SSEMessage{Event: "test", Data: "broadcast"}
	b.Broadcast(msg)

	// Both clients should receive
	select {
	case <-client1:
	case <-time.After(100 * time.Millisecond):
		t.Error("client1 did not receive")
	}

	select {
	case <-client2:
	case <-time.After(100 * time.Millisecond):
		t.Error("client2 did not receive")
	}

	b.Unsubscribe(client1)
	b.Unsubscribe(client2)

	if b.Count() != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", b.Count())
	}
}

func TestFormat(t *testing.T) {
	msg := SSEMessage{
		Event: "test",
		Data:  "hello",
		ID:    "123",
	}

	result := Format(msg)

	if result != "id: 123\nevent: test\ndata: hello\n\n" {
		t.Errorf("unexpected format result: %s", result)
	}
}