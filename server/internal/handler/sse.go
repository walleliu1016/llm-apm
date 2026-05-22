package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/akke/llm-apm/server/internal/broadcaster"
)

// SSEHeartbeatInterval for keep-alive messages.
const SSEHeartbeatInterval = 30 * time.Second

// handleSSEStream handles SSE connections for real-time updates.
func (s *Server) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush headers
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Subscribe to broadcaster
	client := s.broadcaster.Subscribe()
	defer s.broadcaster.Unsubscribe(client)

	if s.logger != nil {
		s.logger.Info("SSE client connected")
	}

	// Send initial connection message
	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Heartbeat ticker
	heartbeat := time.NewTicker(SSEHeartbeatInterval)
	defer heartbeat.Stop()

	// Event loop
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			if s.logger != nil {
				s.logger.Info("SSE client disconnected")
			}
			return

		case msg := <-client:
			// Write SSE message
			fmt.Fprint(w, broadcaster.Format(msg))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

		case <-heartbeat.C:
			// Send heartbeat comment
			fmt.Fprintf(w, ": heartbeat\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}