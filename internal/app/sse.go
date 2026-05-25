package app

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// sseEvent is a server-sent event.
type sseEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data,omitempty"`
}

// sseHub manages SSE client connections and broadcasts events.
type sseHub struct {
	mu      sync.Mutex
	clients map[chan sseEvent]struct{}
}

func newSSEHub() *sseHub {
	return &sseHub{
		clients: make(map[chan sseEvent]struct{}),
	}
}

func (h *sseHub) subscribe() chan sseEvent {
	ch := make(chan sseEvent, 32)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *sseHub) unsubscribe(ch chan sseEvent) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

// Broadcast sends an event to all connected SSE clients.
func (h *sseHub) Broadcast(event string, data any) {
	e := sseEvent{Event: event, Data: data}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- e:
		default:
			// drop if the client is slow
		}
	}
}

// ServeHTTP handles the /api/events SSE endpoint.
func (h *sseHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.subscribe()
	defer h.unsubscribe(ch)

	// Send a "connected" event immediately so the client knows it's live.
	if _, err := fmt.Fprintf(w, "event: connected\ndata: {}\n\n"); err != nil {
		return
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(e.Data)
			if err != nil {
				log.Printf("sse marshal: %v", err)
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Event, payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
