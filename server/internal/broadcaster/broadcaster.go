package broadcaster

import (
	"encoding/json"
	"sync"
)

// SSEMessage represents a Server-Sent Event message.
type SSEMessage struct {
	Event string
	Data  string
	ID    string
}

// Broadcaster manages SSE subscriptions and broadcasts messages.
type Broadcaster struct {
	clients map[chan SSEMessage]struct{}
	mu      sync.RWMutex
}

// NewBroadcaster creates a new broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[chan SSEMessage]struct{}),
	}
}

// Subscribe creates a new client channel for receiving messages.
func (b *Broadcaster) Subscribe() chan SSEMessage {
	b.mu.Lock()
	defer b.mu.Unlock()

	client := make(chan SSEMessage, 10)
	b.clients[client] = struct{}{}
	return client
}

// Unsubscribe removes a client channel.
func (b *Broadcaster) Unsubscribe(client chan SSEMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.clients, client)
	close(client)
}

// Broadcast sends a message to all subscribed clients.
func (b *Broadcaster) Broadcast(msg SSEMessage) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for client := range b.clients {
		// Non-blocking send to prevent slow clients
		select {
		case client <- msg:
		default:
			// Client buffer full, skip
		}
	}
}

// Count returns number of active subscribers.
func (b *Broadcaster) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// BroadcastJSON sends JSON-encoded data.
func (b *Broadcaster) BroadcastJSON(event string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	b.Broadcast(SSEMessage{
		Event: event,
		Data:  string(jsonData),
	})
}

// Format formats an SSE message for HTTP response.
func Format(msg SSEMessage) string {
	result := ""
	if msg.ID != "" {
		result += "id: " + msg.ID + "\n"
	}
	if msg.Event != "" {
		result += "event: " + msg.Event + "\n"
	}
	result += "data: " + msg.Data + "\n\n"
	return result
}