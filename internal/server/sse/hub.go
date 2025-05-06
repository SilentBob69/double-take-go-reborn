package sse

import (
	"encoding/json"
	"sync"
	"time"

	"double-take-go-reborn/internal/core/models"

	log "github.com/sirupsen/logrus"
)

// Client repräsentiert einen einzelnen verbundenen SSE-Client
type Client chan []byte

// Hub verwaltet die Menge der aktiven Clients und sendet Broadcasts an sie
type Hub struct {
	// Registrierte Clients
	clients map[Client]bool

	// Eingehende Nachrichten von der Anwendung
	broadcast chan []byte

	// Registrierungsanfragen von Clients
	register chan Client

	// Abmeldeanfragen von Clients
	unregister chan Client

	// Mutex zum Schutz des simultanen Zugriffs auf die Clients-Map
	mu sync.Mutex
}

// NewImageData definiert die Struktur der Daten, die über SSE gesendet werden
type NewImageData struct {
	ID         uint      `json:"id"`
	FilePath   string    `json:"file_path"`
	SnapshotURL string    `json:"snapshot_url"`
	Timestamp  time.Time `json:"timestamp"`
	FacesCount int       `json:"faces_count"`
	Source     string    `json:"source"`
	Matches    []MatchData `json:"matches"`
}

// MatchData enthält vereinfachte Informationen über Matches für die SSE-Nachricht
type MatchData struct {
	Identity   string  `json:"identity"`
	Confidence float64 `json:"confidence"`
}

// NewHub erstellt eine neue Hub-Instanz
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte, 100),  // Puffer für 100 Nachrichten
		register:   make(chan Client),
		unregister: make(chan Client),
		clients:    make(map[Client]bool),
	}
}

// Run startet die Verarbeitungsschleife des Hubs
// Dies sollte in einer separaten Goroutine ausgeführt werden
func (h *Hub) Run() {
	log.Info("SSE Hub started and running")
	
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			clientCount := len(h.clients)
			h.mu.Unlock()
			log.Infof("SSE client registered. Total clients: %d", clientCount)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client)
				clientCount := len(h.clients)
				log.Infof("SSE client unregistered. Total clients: %d", clientCount)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			log.Debugf("Broadcasting message to %d SSE clients", len(h.clients))
			
			for client := range h.clients {
				select {
				case client <- message:
					// Nachricht erfolgreich gesendet
				default:
					// Client-Kanal ist voll oder geschlossen
					log.Warn("SSE client channel full or closed, removing client")
					delete(h.clients, client)
					close(client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Register registriert einen neuen Client am Hub
func (h *Hub) Register(client Client) {
	h.register <- client
}

// Unregister meldet einen Client vom Hub ab
func (h *Hub) Unregister(client Client) {
	h.unregister <- client
}

// Broadcast sendet eine Nachricht an alle registrierten Clients
func (h *Hub) Broadcast(message []byte) {
	// Blockieren vermeiden, wenn der Broadcast-Kanal voll ist
	select {
	case h.broadcast <- message:
		// Nachricht erfolgreich zum Senden in die Queue gestellt
	default:
		log.Warn("SSE broadcast channel full, message dropped")
	}
}

// BroadcastNewImage formatiert Informationen über ein neues Bild und sendet sie als Broadcast
func (h *Hub) BroadcastNewImage(image models.Image, snapshotURL string, matches []models.Match) {
	log.Infof("Broadcasting new image information (ID: %d) to SSE clients", image.ID)
	
	// Matches für die Anzeige aufbereiten
	matchDataList := make([]MatchData, 0, len(matches))
	for _, match := range matches {
		matchDataList = append(matchDataList, MatchData{
			Identity:   match.Identity.Name,
			Confidence: match.Confidence,
		})
	}
	
	// Daten für die SSE-Nachricht aufbereiten
	data := NewImageData{
		ID:         image.ID,
		FilePath:   image.FilePath,
		SnapshotURL: snapshotURL,
		Timestamp:  image.Timestamp,
		FacesCount: len(image.Faces),
		Source:     image.Source,
		Matches:    matchDataList,
	}
	
	// Daten als JSON serialisieren
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Errorf("Failed to marshal new image data for SSE: %v", err)
		return
	}
	
	// Daten broadcasten
	h.Broadcast(jsonData)
}
