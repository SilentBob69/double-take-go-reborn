package sse

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"double-take-go-reborn/internal/models"
)

// Client represents a single connected SSE client.
// It's essentially a channel where we send messages destined for this client.
type Client chan []byte

// Hub manages the set of active clients and broadcasts messages to them.
type Hub struct {
	// Registered clients.
	// Use a map where keys are the channels and values are bools (true indicates presence).
	clients map[Client]bool

	// Inbound messages from the application (e.g., new image processed).
	broadcast chan []byte

	// Register requests from the clients.
	register chan Client

	// Unregister requests from clients.
	unregister chan Client

	// Mutex to protect concurrent access to the clients map.
	mu sync.Mutex
}

// NewImageData defines the structure of the data sent via SSE when a new image is ready.
// Keep this lean, only include what's needed to render the card initially.
type NewImageData struct {
	ID        uint      `json:"id"`
	SnapshotURL string    `json:"snapshot_url"`
	Timestamp time.Time `json:"timestamp"`
	Matches   []string  `json:"matches"` // Simple list of matched names for display
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan Client),
		unregister: make(chan Client),
		clients:    make(map[Client]bool),
	}
}

// Run starts the hub's processing loop.
// It should be run in a separate goroutine.
func (h *Hub) Run() {
	log.Println("SSE Hub started.")
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("SSE Client registered. Total clients: %d\n", len(h.clients))
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client) // Close the channel to signal the client handler to stop.
				log.Printf("SSE Client unregistered. Total clients: %d\n", len(h.clients))
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			//log.Printf("Broadcasting message to %d clients: %s\n", len(h.clients), string(message))
			for client := range h.clients {
				// Use a select with a default case to prevent blocking if a client's channel is full.
				select {
				case client <- message:
					// Message sent successfully
				default:
					// Client channel is full or closed, maybe unregister?
					// For now, we just log it and potentially lose the message for this slow client.
					log.Println("Warning: SSE client channel full or closed. Skipping message.")
					// Consider removing the client here: delete(h.clients, client); close(client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Register adds a new client to the hub.
func (h *Hub) Register(client Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client Client) {
	h.unregister <- client
}

// Broadcast sends a message to all registered clients.
func (h *Hub) Broadcast(message []byte) {
	// Avoid blocking the caller if the broadcast channel is full
	select {
	case h.broadcast <- message:
	default:
		log.Println("Warning: SSE broadcast channel full. Message dropped.")
	}
}

// BroadcastNewImage formats and broadcasts information about a newly processed image.
func (h *Hub) BroadcastNewImage(image models.Image, matches []models.Match) {
	log.Printf("Preparing SSE broadcast for new Image ID: %d\n", image.ID)
	
	matchNames := []string{}
	for _, match := range matches {
		if match.Identity.Name != "unknown" { // Only include known matches
			matchNames = append(matchNames, match.Identity.Name)
		}
	}

	data := NewImageData{
		ID:        image.ID,
		SnapshotURL: "/snapshots/" + image.FilePath, // Construct the URL as served by the backend
		Timestamp: image.Timestamp,
		Matches:   matchNames,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshalling NewImageData for SSE: %v\n", err)
		return
	}

	h.Broadcast(jsonData)
}
