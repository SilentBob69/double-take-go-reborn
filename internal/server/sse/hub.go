package sse

import (
	"encoding/json"
	"strings"
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

	// Ping-Intervall für aktive Clients
	pingInterval time.Duration
}

// SseEventType definiert die möglichen Ereignistypen für SSE-Nachrichten
type SseEventType string

const (
	EventNewImage    SseEventType = "new_image"     // Einzelnes neues Bild
	EventUpdateImage SseEventType = "update_image"  // Aktualisierung eines Bildes
	EventNewGroup    SseEventType = "new_group"     // Neue Bildgruppe
	EventDeleteImage SseEventType = "delete_image"  // Bild wurde gelöscht
)

// SseEvent ist die Basisstruktur für alle SSE-Ereignisse
type SseEvent struct {
	Type      SseEventType `json:"type"`
	Timestamp time.Time    `json:"timestamp"`
	Data      interface{}  `json:"data"`
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
	// Frigate-spezifische Felder
	EventID    string    `json:"event_id,omitempty"`
	Camera     string    `json:"camera,omitempty"`
	Label      string    `json:"label,omitempty"`
	Zone       string    `json:"zone,omitempty"`
	EventType  string    `json:"event_type,omitempty"` // "new", "update", etc.
}

// NewGroupData definiert die Struktur für eine neue Bildgruppe
type NewGroupData struct {
	EventID    string    `json:"event_id"`
	Source     string    `json:"source"`
	Camera     string    `json:"camera,omitempty"`
	Label      string    `json:"label,omitempty"`
	Zone       string    `json:"zone,omitempty"`
	Count      int       `json:"count"`
	Timestamp  time.Time `json:"timestamp"`
	Images     []uint    `json:"image_ids"`
	ThumbnailURL string  `json:"thumbnail_url"`
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
		pingInterval: 30 * time.Second,       // Standard-Ping alle 30 Sekunden
	}
}

// lastActivity speichert den Zeitpunkt der letzten Aktivität für jeden Client
type clientInfo struct {
	lastActivity time.Time
}

// Run startet die Verarbeitungsschleife des Hubs
// Dies sollte in einer separaten Goroutine ausgeführt werden
func (h *Hub) Run() {
	log.Info("SSE Hub started and running")
	
	// Verbessert: Speichert Aktivitätsdaten für jeden Client
	clientInfoMap := make(map[Client]*clientInfo)

	// Cleanup-Ticker für inaktive Clients (alle 5 Minuten prüfen)
	cleanupTicker := time.NewTicker(5 * time.Minute)

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			clientInfoMap[client] = &clientInfo{lastActivity: time.Now()}
			clientCount := len(h.clients)
			h.mu.Unlock()
			log.Infof("SSE client registered. Total clients: %d", clientCount)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(clientInfoMap, client)
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
					// Aktualisiere den Zeitstempel der letzten Aktivität
					if info, ok := clientInfoMap[client]; ok {
						info.lastActivity = time.Now()
					}
				default:
					// Client-Kanal ist voll oder geschlossen
					log.Warn("SSE client channel full or closed, removing client")
					delete(h.clients, client)
					delete(clientInfoMap, client)
					close(client)
				}
			}
			h.mu.Unlock()
		
		case <-cleanupTicker.C:
			// Prüfe und entferne inaktive Clients (älter als 30 Minuten)
			h.mu.Lock()
			inactiveThreshold := time.Now().Add(-30 * time.Minute)
			removed := 0
			
			for client, info := range clientInfoMap {
				if info.lastActivity.Before(inactiveThreshold) {
					// Client ist inaktiv, entfernen
					delete(h.clients, client)
					delete(clientInfoMap, client)
					close(client)
					removed++
				}
			}
			
			if removed > 0 {
				log.Infof("Cleaned up %d inactive SSE clients. Remaining: %d", removed, len(h.clients))
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
	imageData := NewImageData{
		ID:         image.ID,
		FilePath:   image.FilePath,
		SnapshotURL: snapshotURL,
		Timestamp:  image.Timestamp,
		FacesCount: len(image.Faces),
		Source:     image.Source,
		Matches:    matchDataList,
	}
	
	// Frigate-Metadaten extrahieren, falls vorhanden (z.B. EventID, camera, etc.)
	if image.Source == "frigate" {
		// EventID ist direkt im Image-Modell
		imageData.EventID = image.EventID
		// Zone ist direkt im Image-Modell
		imageData.Zone = image.Zone
		// Label ist direkt im Image-Modell
		imageData.Label = image.Label
		
		// Event-Typ aus dem Dateinamen extrahieren
		if strings.Contains(image.FilePath, "_seq") {
			imageData.EventType = "new"
		} else if strings.Contains(image.FilePath, "_update") {
			imageData.EventType = "update"
		}
		
		// Kamera aus den SourceData extrahieren, falls vorhanden
		if len(image.SourceData) > 0 {
			var sourceData map[string]interface{}
			if err := json.Unmarshal(image.SourceData, &sourceData); err == nil {
				if camera, ok := sourceData["camera"].(string); ok {
					imageData.Camera = camera
				}
			}
		}
	}
	
	// SSE-Event mit Typ erstellen
	eventType := EventNewImage
	if strings.Contains(image.FilePath, "_update") {
		eventType = EventUpdateImage
	}
	
	sseEvent := SseEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      imageData,
	}
	
	// Daten als JSON serialisieren
	jsonData, err := json.Marshal(sseEvent)
	if err != nil {
		log.Errorf("Failed to marshal new image data for SSE: %v", err)
		return
	}
	
	// Daten broadcasten
	h.Broadcast(jsonData)
}

// BroadcastNewGroup sendet Informationen über eine neue Bildgruppe an alle Clients
func (h *Hub) BroadcastNewGroup(eventID, source, camera, label, zone string, images []models.Image, thumbnailURL string) {
	if len(images) == 0 {
		log.Warn("Attempting to broadcast empty image group, ignoring")
		return
	}
	
	log.Infof("Broadcasting new image group (EventID: %s) with %d images to SSE clients", eventID, len(images))
	
	// Bilder-IDs sammeln
	imageIDs := make([]uint, 0, len(images))
	for _, img := range images {
		imageIDs = append(imageIDs, img.ID)
	}
	
	// Zeitstempel ist der des neuesten Bildes
	latestTimestamp := images[0].Timestamp
	for _, img := range images {
		if img.Timestamp.After(latestTimestamp) {
			latestTimestamp = img.Timestamp
		}
	}
	
	// Daten für die SSE-Nachricht aufbereiten
	groupData := NewGroupData{
		EventID:     eventID,
		Source:      source,
		Camera:      camera,
		Label:       label,
		Zone:        zone,
		Count:       len(images),
		Timestamp:   latestTimestamp,
		Images:      imageIDs,
		ThumbnailURL: thumbnailURL,
	}
	
	// SSE-Event erstellen
	sseEvent := SseEvent{
		Type:      EventNewGroup,
		Timestamp: time.Now(),
		Data:      groupData,
	}
	
	// Daten als JSON serialisieren
	jsonData, err := json.Marshal(sseEvent)
	if err != nil {
		log.Errorf("Failed to marshal new group data for SSE: %v", err)
		return
	}
	
	// Daten broadcasten
	h.Broadcast(jsonData)
}
