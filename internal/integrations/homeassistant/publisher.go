package homeassistant

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/integrations/mqtt"

	log "github.com/sirupsen/logrus"
)

// BoundingBoxData enthält die Position eines Gesichts
type BoundingBoxData struct {
	XMin float64 `json:"x_min"`
	YMin float64 `json:"y_min"`
	XMax float64 `json:"x_max"`
	YMax float64 `json:"y_max"`
}

// Publisher verwaltet die Veröffentlichung von Erkennungsergebnissen via MQTT
type Publisher struct {
	mqttClient       *mqtt.Client
	cfg              *config.Config
	personCounters   map[string]int // Zähler für Personen pro Kamera
	personLastUpdate map[string]time.Time // Zeitpunkt der letzten Aktualisierung
}

// MatchEvent enthält die Daten eines Matches, die über MQTT veröffentlicht werden
type MatchEvent struct {
	ID        string    `json:"id"`
	Duration  float64   `json:"duration"`
	Timestamp time.Time `json:"timestamp"`
	Attempts  int       `json:"attempts"`
	Camera    string    `json:"camera"`
	Zones     []string  `json:"zones"`
	Match     *Match    `json:"match,omitempty"`
}

// CameraEvent enthält die Daten eines Kamera-Events mit allen Matches
type CameraEvent struct {
	ID        string    `json:"id"`
	Duration  float64   `json:"duration"`
	Timestamp time.Time `json:"timestamp"`
	Attempts  int       `json:"attempts"`
	Camera    string    `json:"camera"`
	Zones     []string  `json:"zones"`
	Matches   []*Match  `json:"matches"`
	Misses    []*Match  `json:"misses"`
	Unknowns  []*Match  `json:"unknowns"`
	Counts    Counts    `json:"counts"`
}

// Match enthält die Details eines erkannten Gesichts
type Match struct {
	Name       string     `json:"name"`
	Confidence float64    `json:"confidence"`
	Match      bool       `json:"match"`
	Box        Box        `json:"box"`
	Type       string     `json:"type"`
	Duration   float64    `json:"duration"`
	Detector   string     `json:"detector"`
	Filename   string     `json:"filename"`
}

// PresenceInfo enthält die Informationen über die Anwesenheit einer Person
type PresenceInfo struct {
	CameraName       string    `json:"camera"`       // Name der Kamera
	Confidence       float64   `json:"confidence"`   // Erkennungswahrscheinlichkeit
	LastSeen         time.Time `json:"last_seen"`    // Zeitpunkt der letzten Erkennung
	ImageID          uint      `json:"image_id"`     // ID des Bildes
	ImagePath        string    `json:"image_path"`   // Pfad zum Bild
	DetectionHistory []string  `json:"history"`      // Liste der letzten Erkennungsorte
}

// Box enthält die Koordinaten eines erkannten Gesichts
type Box struct {
	Top    int `json:"top"`
	Left   int `json:"left"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Counts enthält Zählungen verschiedener Ergebnistypen
type Counts struct {
	Person  int `json:"person"`
	Match   int `json:"match"`
	Miss    int `json:"miss"`
	Unknown int `json:"unknown"`
}

// NewPublisher erstellt einen neuen MQTT-Publisher für Home Assistant
func NewPublisher(mqttClient *mqtt.Client, cfg *config.Config) *Publisher {
	return &Publisher{
		mqttClient:       mqttClient,
		cfg:              cfg,
		personCounters:   make(map[string]int),
		personLastUpdate: make(map[string]time.Time),
	}
}

// StartResetTimers startet die Timer zum Zurücksetzen der Personenzähler
func (p *Publisher) StartResetTimers() {
	// Regelmäßig alle 30 Sekunden überprüfen, ob Zähler zurückgesetzt werden müssen
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			p.checkAndResetCounters()
		}
	}()
}

// checkAndResetCounters prüft, ob Zähler zurückgesetzt werden müssen
func (p *Publisher) checkAndResetCounters() {
	now := time.Now()
	
	for camera, lastUpdate := range p.personLastUpdate {
		// Wenn der letzte Update mehr als 30 Sekunden her ist, Zähler zurücksetzen
		if now.Sub(lastUpdate) > 30*time.Second {
			p.personCounters[camera] = 0
			
			// Zähler auf 0 setzen und veröffentlichen
			if err := p.mqttClient.Publish(fmt.Sprintf("double-take/cameras/%s/person", camera), "0"); err != nil {
				log.Errorf("Failed to publish person counter reset for camera %s: %v", camera, err)
			} else {
				log.Debugf("Reset person counter for camera %s", camera)
			}
			
			// Zeitstempel entfernen
			delete(p.personLastUpdate, camera)
		}
	}
}

// PublishMatchResult veröffentlicht ein einzelnes Match-Ergebnis
func (p *Publisher) PublishMatchResult(face models.Face, match models.Match, image *models.Image, duration float64, attempts int) error {
	// BoundingBox-Daten extrahieren
	var boundingBox BoundingBoxData
	if err := json.Unmarshal([]byte(face.BoundingBox), &boundingBox); err != nil {
		log.Errorf("Failed to unmarshal bounding box data: %v", err)
		return fmt.Errorf("failed to unmarshal bounding box data: %w", err)
	}
	
	// Box-Koordinaten erstellen
	box := Box{
		Top:    int(boundingBox.YMin),
		Left:   int(boundingBox.XMin),
		Width:  int(boundingBox.XMax - boundingBox.XMin),
		Height: int(boundingBox.YMax - boundingBox.YMin),
	}
	
	// Match-Informationen erstellen
	matchInfo := &Match{
		Name:       match.Identity.Name,
		Confidence: match.Confidence,
		Match:      true,
		Box:        box,
		Type:       "face",
		Duration:   duration,
		Detector:   "compreface",
		Filename:   image.FilePath,
	}
	
	// Event erstellen
	event := MatchEvent{
		ID:        fmt.Sprintf("%d", image.ID),
		Duration:  duration,
		Timestamp: time.Now(),
		Attempts:  attempts,
		Camera:    image.Source,
		Zones:     []string{},
		Match:     matchInfo,
	}
	
	// Match-Topic (pro Person)
	topic := fmt.Sprintf("double-take/matches/%s", match.Identity.Name)
	
	// Veröffentlichen
	if err := p.mqttClient.Publish(topic, event); err != nil {
		return fmt.Errorf("failed to publish match result: %w", err)
	}
	
	// Personen-Zähler für diese Kamera aktualisieren
	p.updatePersonCounter(image.Source)
	
	return nil
}

// PublishCameraResult veröffentlicht ein Kamera-Ereignis mit allen Matches
func (p *Publisher) PublishCameraResult(image *models.Image, matches []models.Match, duration float64, attempts int) error {
	// Kamera-Topic
	topic := fmt.Sprintf("double-take/cameras/%s", image.Source)
	
	// Listen für die verschiedenen Ergebnistypen
	matchItems := make([]*Match, 0)
	missItems := make([]*Match, 0)
	unknownItems := make([]*Match, 0)
	
	// Zähler
	counts := Counts{
		Person:  len(image.Faces),
		Match:   0,
		Miss:    0,
		Unknown: len(image.Faces), // Standardmäßig alle als unbekannt betrachten
	}
	
	// Map, um erkannte Gesichter zu markieren
	processedFaces := make(map[uint]bool)
	
	// Matches verarbeiten
	for _, match := range matches {
		// Das zugehörige Face für dieses Match finden
		var matchFace models.Face
		var foundFace bool
		for _, face := range image.Faces {
			if face.ID == match.FaceID {
				matchFace = face
				foundFace = true
				processedFaces[face.ID] = true
				break
			}
		}
		
		if !foundFace {
			log.Warnf("Could not find face for match (ID: %d)", match.FaceID)
			continue
		}
		
		// BoundingBox-Daten extrahieren
		var boundingBox BoundingBoxData
		if err := json.Unmarshal([]byte(matchFace.BoundingBox), &boundingBox); err != nil {
			log.Errorf("Failed to unmarshal bounding box data: %v", err)
			continue
		}
		
		// Box-Koordinaten
		box := Box{
			Top:    int(boundingBox.YMin),
			Left:   int(boundingBox.XMin),
			Width:  int(boundingBox.XMax - boundingBox.XMin),
			Height: int(boundingBox.YMax - boundingBox.YMin),
		}
		
		// Match-Informationen
		matchInfo := &Match{
			Name:       match.Identity.Name,
			Confidence: match.Confidence,
			Match:      true,
			Box:        box,
			Type:       "face",
			Duration:   duration,
			Detector:   "compreface",
			Filename:   image.FilePath,
		}
		
		matchItems = append(matchItems, matchInfo)
		counts.Match++
		counts.Unknown--
	}
	
	// Nicht erkannte Gesichter als "unknown" markieren
	for _, face := range image.Faces {
		if !processedFaces[face.ID] {
			// BoundingBox-Daten extrahieren
			var boundingBox BoundingBoxData
			if err := json.Unmarshal([]byte(face.BoundingBox), &boundingBox); err != nil {
				log.Errorf("Failed to unmarshal bounding box data: %v", err)
				continue
			}
			
			// Box-Koordinaten
			box := Box{
				Top:    int(boundingBox.YMin),
				Left:   int(boundingBox.XMin),
				Width:  int(boundingBox.XMax - boundingBox.XMin),
				Height: int(boundingBox.YMax - boundingBox.YMin),
			}
			
			// Unknown-Face
			unknownInfo := &Match{
				Name:       "unknown",
				Confidence: 0,
				Match:      false,
				Box:        box,
				Type:       "face",
				Duration:   duration,
				Detector:   "compreface",
				Filename:   image.FilePath,
			}
			
			unknownItems = append(unknownItems, unknownInfo)
		}
	}
	
	// Event erstellen
	event := CameraEvent{
		ID:        fmt.Sprintf("%d", image.ID),
		Duration:  duration,
		Timestamp: time.Now(),
		Attempts:  attempts,
		Camera:    image.Source,
		Zones:     []string{},
		Matches:   matchItems,
		Misses:    missItems,
		Unknowns:  unknownItems,
		Counts:    counts,
	}
	
	// Veröffentlichen des Kamera-Events
	if err := p.mqttClient.Publish(topic, event); err != nil {
		return fmt.Errorf("failed to publish camera result: %w", err)
	}
	
	// Personen-Zähler für diese Kamera aktualisieren
	p.updatePersonCounter(image.Source)
	
	// Anwesenheitssensoren für erkannte Identitäten aktualisieren
	publishedIdentities := make(map[string]bool)
	
	// Für jedes Match den entsprechenden Anwesenheitssensor aktualisieren
	for _, match := range matchItems {
		// Doppelte Erkennungen vermeiden
		if _, exists := publishedIdentities[match.Name]; exists {
			continue
		}
		
		// Anwesenheitssensor aktualisieren
		if err := p.UpdatePresenceSensor(match.Name, image.Source, match.Confidence, 
			image.ID, image.FilePath); err != nil {
			log.Warnf("Failed to update presence sensor for %s: %v", match.Name, err)
		}
		
		// Identität als verarbeitet markieren
		publishedIdentities[match.Name] = true
	}
	
	// Falls unbekannte Gesichter vorhanden sind, auch deren Sensor aktualisieren
	if len(unknownItems) > 0 {
		if err := p.UpdateUnknownPresenceSensor(image.Source, image.ID, image.FilePath, 
			len(unknownItems)); err != nil {
			log.Warnf("Failed to update unknown presence sensor: %v", err)
		}
	}
	
	return nil
}

// PublishError veröffentlicht eine Fehlermeldung
func (p *Publisher) PublishError(err error) error {
	return p.mqttClient.Publish("double-take/errors", err.Error())
}

// UpdatePresenceSensor aktualisiert einen Anwesenheitssensor für eine identifizierte Person
func (p *Publisher) UpdatePresenceSensor(identityName string, camera string, confidence float64, imageID uint, imagePath string) error {
	// Namen für MQTT normalisieren
	normalizedName := strings.ToLower(strings.ReplaceAll(identityName, " ", "_"))
	
	// 1. Den Anwesenheitssensor auf "on" setzen
	presenceTopic := fmt.Sprintf("double-take/presence/%s", normalizedName)
	if err := p.mqttClient.PublishRetain(presenceTopic, "on"); err != nil {
		return fmt.Errorf("failed to publish presence state: %w", err)
	}
	
	// 2. Detailierte Informationen für die Attribute
	timestamp := time.Now()
	presenceInfo := PresenceInfo{
		CameraName: camera,
		Confidence: confidence,
		LastSeen:   timestamp,
		ImageID:    imageID,
		ImagePath:  imagePath,
		// Historie könnte aus einer DB kommen, vereinfacht hier nur den aktuellsten Eintrag
		DetectionHistory: []string{fmt.Sprintf("%s (%s)", camera, timestamp.Format("15:04:05"))},
	}
	
	// Attribute veröffentlichen
	attributesTopic := fmt.Sprintf("%s/attributes", presenceTopic)
	if err := p.mqttClient.PublishRetain(attributesTopic, presenceInfo); err != nil {
		return fmt.Errorf("failed to publish presence attributes: %w", err)
	}
	
	// 3. Lokationsinformation aktualisieren
	locationTopic := fmt.Sprintf("%s/location", presenceTopic)
	locationText := fmt.Sprintf("Zuletzt gesehen: %s (%.1f%%)", camera, confidence*100)
	if err := p.mqttClient.PublishRetain(locationTopic, locationText); err != nil {
		return fmt.Errorf("failed to publish location info: %w", err)
	}
	
	return nil
}

// UpdateUnknownPresenceSensor aktualisiert den Anwesenheitssensor für unbekannte Personen
func (p *Publisher) UpdateUnknownPresenceSensor(camera string, imageID uint, imagePath string, count int) error {
	// 1. Den Anwesenheitssensor für unbekannte Personen auf "on" setzen
	presenceTopic := "double-take/presence/unknown"
	if err := p.mqttClient.PublishRetain(presenceTopic, "on"); err != nil {
		return fmt.Errorf("failed to publish unknown presence state: %w", err)
	}
	
	// 2. Detailierte Informationen für die Attribute
	timestamp := time.Now()
	presenceInfo := PresenceInfo{
		CameraName: camera,
		Confidence: 0,
		LastSeen:   timestamp,
		ImageID:    imageID,
		ImagePath:  imagePath,
		DetectionHistory: []string{fmt.Sprintf("%s (%s)", camera, timestamp.Format("15:04:05"))},
	}
	
	// Attribute veröffentlichen
	attributesTopic := fmt.Sprintf("%s/attributes", presenceTopic)
	if err := p.mqttClient.PublishRetain(attributesTopic, presenceInfo); err != nil {
		return fmt.Errorf("failed to publish unknown presence attributes: %w", err)
	}
	
	// 3. Lokationsinformation aktualisieren
	locationTopic := fmt.Sprintf("%s/location", presenceTopic)
	locationText := fmt.Sprintf("%d unbekannte Personen bei %s", count, camera)
	if err := p.mqttClient.PublishRetain(locationTopic, locationText); err != nil {
		return fmt.Errorf("failed to publish unknown location info: %w", err)
	}
	
	return nil
}

// updatePersonCounter aktualisiert den Personenzähler für eine Kamera
func (p *Publisher) updatePersonCounter(camera string) {
	// Zähler erhöhen
	counter, exists := p.personCounters[camera]
	if !exists {
		counter = 0
	}
	counter++
	p.personCounters[camera] = counter
	
	// Zeitstempel aktualisieren
	p.personLastUpdate[camera] = time.Now()
	
	// Zähler veröffentlichen
	topic := fmt.Sprintf("double-take/cameras/%s/person", camera)
	if err := p.mqttClient.Publish(topic, fmt.Sprintf("%d", counter)); err != nil {
		log.Errorf("Failed to publish person counter for camera %s: %v", camera, err)
	}
}
