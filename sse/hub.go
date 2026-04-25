package sse

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"sentinelaegis/types"
)

// Client represents a connected SSE browser client.
type Client struct {
	id      string
	channel chan string
	done    chan struct{}
}

// Hub manages all SSE client connections and broadcasts events.
type Hub struct {
	clients   map[string]*Client
	mu        sync.RWMutex
	broadcast chan string
	count     atomic.Int64
}

// NewHub creates a new SSE hub.
func NewHub() *Hub {
	return &Hub{
		clients:   make(map[string]*Client),
		broadcast: make(chan string, 256),
	}
}

// Run starts the broadcast loop. Call as: go hub.Run()
func (h *Hub) Run() {
	for msg := range h.broadcast {
		h.mu.RLock()
		for id, c := range h.clients {
			select {
			case c.channel <- msg:
			default:
				log.Printf("[sse] Client %s buffer full, dropping message", id)
			}
		}
		h.mu.RUnlock()
	}
}

// ServeSSE handles a new SSE client connection.
// GET /api/stream
func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientID := fmt.Sprintf("client-%d-%d", time.Now().UnixNano(), h.count.Add(1))
	client := &Client{
		id:      clientID,
		channel: make(chan string, 64),
		done:    make(chan struct{}),
	}

	h.mu.Lock()
	h.clients[clientID] = client
	h.mu.Unlock()

	log.Printf("[sse] Client connected: %s (total: %d)", clientID, h.ClientCount())

	// Send initial connected event
	fmt.Fprintf(w, "data: {\"event_type\":\"connected\",\"client_id\":\"%s\"}\n\n", clientID)
	flusher.Flush()

	// Send keepalive comment every 15s to prevent proxy timeouts
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			h.removeClient(clientID)
			return
		case msg := <-client.channel:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// Broadcast sends an AnalysisEvent to all connected clients.
func (h *Hub) Broadcast(event types.AnalysisEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[sse] Failed to marshal event: %v", err)
		return
	}
	h.broadcast <- string(data)
}

// BroadcastRaw sends a raw JSON string to all clients.
func (h *Hub) BroadcastRaw(data string) {
	h.broadcast <- data
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) removeClient(id string) {
	h.mu.Lock()
	if c, ok := h.clients[id]; ok {
		close(c.channel)
		delete(h.clients, id)
	}
	h.mu.Unlock()
	log.Printf("[sse] Client disconnected: %s (remaining: %d)", id, h.ClientCount())
}
