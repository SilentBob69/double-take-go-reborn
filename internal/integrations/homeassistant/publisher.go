package homeassistant

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/integrations/mqtt"
	"double-take-go-reborn/internal/util/timezone"

	"github.com/glebarez/sqlite" // Verwende den bereits im Projekt vorhandenen SQLite-Treiber
	"gorm.io/gorm"

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
	lastDetections   map[string]time.Time // Speichert die letzten Erkennungszeitpunkte pro Identität
}

// cleanCameraName bereinigt einen Kameranamen von Präfixen und Suffixen
func cleanCameraName(camera string) string {
	cleanCamera := camera
	
	// Entferne "frigate_" oder "frigate/" Präfix
	if strings.HasPrefix(cleanCamera, "frigate_") {
		cleanCamera = strings.TrimPrefix(cleanCamera, "frigate_")
	} else if strings.HasPrefix(cleanCamera, "frigate/") {
		cleanCamera = strings.TrimPrefix(cleanCamera, "frigate/")
	}
	
	// Entferne "_camera" Suffix
	if strings.HasSuffix(cleanCamera, "_camera") {
		cleanCamera = strings.TrimSuffix(cleanCamera, "_camera")
	}
	
	return cleanCamera
}

// MatchEvent enthält die Daten eines Matches, die über MQTT veröffentlicht werden
type MatchEvent struct {
	ID        string    `json:"id"`
	Duration  float64   `json:"duration"`
	Timestamp string    `json:"timestamp"` // ISO 8601 String mit expliziter Zeitzone
	Attempts  int       `json:"attempts"`
	Camera    string    `json:"camera"`
	Zones     []string  `json:"zones"`
	Match     *Match    `json:"match,omitempty"`
}

// CameraEvent enthält die Daten eines Kamera-Events mit allen Matches
type CameraEvent struct {
	ID        string    `json:"id"`
	Duration  float64   `json:"duration"`
	Timestamp string    `json:"timestamp"` // ISO 8601 String mit expliziter Zeitzone
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
	ImageData        string    `json:"image_data"`   // Base64-kodiertes Bild für Home Assistant
	DetectionHistory []string  `json:"history"`      // Liste der letzten Erkennungsorte
	Zones            []string  `json:"zones"`        // Erkannte Zonen (falls vorhanden)
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
	// Timezone wird bereits in main.go initialisiert
	
	return &Publisher{
		mqttClient:       mqttClient,
		cfg:              cfg,
		personCounters:   make(map[string]int),
		personLastUpdate: make(map[string]time.Time),
		lastDetections:   make(map[string]time.Time),
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
	now := timezone.Now()
	
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
	// Zeit mit expliziter Zeitzone formatieren (ISO 8601)
	now := timezone.Now()
	isoTime := now.Format(time.RFC3339) // Format: 2025-05-18T11:26:42+02:00
	
	// Bereinigten Kameranamen verwenden
	cleanCamera := cleanCameraName(image.Source)
	
	event := MatchEvent{
		ID:        fmt.Sprintf("%d", image.ID),
		Duration:  duration,
		Timestamp: isoTime,
		Attempts:  attempts,
		Camera:    cleanCamera,
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
	// Bereinigten Kameranamen verwenden
	cleanCamera := cleanCameraName(image.Source)
	
	// Kamera-Topic mit bereinigtem Namen
	topic := fmt.Sprintf("double-take/cameras/%s", cleanCamera)
	
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
	// Zeit mit expliziter Zeitzone formatieren (ISO 8601)
	now := timezone.Now()
	isoTime := now.Format(time.RFC3339) // Format: 2025-05-18T11:26:42+02:00
	
	event := CameraEvent{
		ID:        fmt.Sprintf("%d", image.ID),
		Duration:  duration,
		Timestamp: isoTime,
		Attempts:  attempts,
		Camera:    cleanCamera,
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
		
		// WICHTIG: Debug-Information zur Fehlersuche
		log.Infof("!!! ERKANNT: Person '%s' mit Konfidenz %.2f in Kamera '%s' !!!", 
			match.Name, match.Confidence, image.Source)
		
		// Anwesenheitssensor aktualisieren - DIREKT die UpdateRecognizedPerson-Methode aufrufen
		if err := p.UpdateRecognizedPerson(match.Name, image.Source, match.Confidence, 
			image.ID, image.FilePath); err != nil {
			log.Warnf("Failed to update person sensor for %s: %v", match.Name, err)
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
	return p.mqttClient.Publish("double-take/error", err.Error())
}

// UpdateRecognizedPerson aktualisiert die Entität mit der erkannten Person
func (p *Publisher) UpdateRecognizedPerson(identityName string, camera string, confidence float64, imageID uint, imagePath string) error {
	// 1. Aktualisiere den Wert des Haupt-Sensors mit Namen, Kamera und Zeitstempel
	personTopic := "double-take/person"
	
	// Versuche, die originale Bildzeit zu finden anstatt time.Now() zu verwenden
	timestamp := timezone.Now()
	formattedTime := timestamp.Format("15:04")
	
	// Versuche, ein Bild mit der angegebenen ID zu laden, um den tatsächlichen Zeitstempel zu erhalten
	var image models.Image
	var err error
	
	// Pfad zur Datenbank über Umgebungsvariable holen
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/double-take.db"
	}
	
	// Temporäre DB-Verbindung nur für diesen Zweck
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err == nil {
		if err := db.First(&image, imageID).Error; err == nil && !image.Timestamp.IsZero() {
			// Verwende den tatsächlichen Zeitstempel des Bildes
			timestamp = image.Timestamp
			formattedTime = timestamp.Format("15:04")
			log.Infof("Verwende tatsächlichen Zeitstempel aus Bild: %s", formattedTime)
		}
	}
	
	// Kameranamen korrekt extrahieren
	// Extrahiere den tatsächlichen Kameranamen ohne ihn zu sehr zu verändern
	cleanCamera := camera
	
	// Wenn der Kameraname "frigate" enthält, versuche Metadaten aus dem Bild zu verwenden
	if cleanCamera == "frigate" || strings.HasPrefix(cleanCamera, "frigate_") || strings.HasPrefix(cleanCamera, "frigate/") {
		// Versuche, die tatsächliche Kamera aus den Metadaten zu extrahieren
		if err == nil && image.ID > 0 {
			// Versuche die SourceData zu extrahieren, falls vorhanden
			var metadataJSON map[string]interface{}
			string_data := string(image.SourceData)
			if string_data != "" && json.Unmarshal([]byte(string_data), &metadataJSON) == nil {
				if cameraName, ok := metadataJSON["camera"].(string); ok && cameraName != "" {
					cleanCamera = cameraName
					log.Infof("Verwende Kameranamen aus Bild-Metadaten: %s", cleanCamera)
				}
			}
		}
	}
	
	// Kürzeres Format für den Sensorwert, nur mit Stunde:Minute
	// Format: "Name (Kamera 15:04)"
	personValue := fmt.Sprintf("%s (%s %s)", identityName, cleanCamera, formattedTime)
	
	// Debug-Logging vor dem Senden
	log.Infof("Aktualisiere Person-Sensor mit Wert '%s' auf Topic '%s'", personValue, personTopic)
	
	if err := p.mqttClient.PublishRetain(personTopic, personValue); err != nil {
		return fmt.Errorf("failed to publish person info: %w", err)
	}
	
	// Direkter Zugriff auf den MQTT-Client für zusätzliche Debug-Informationen
	log.Infof("Erfolgreich gesendet: Person='%s', Kamera='%s', ID=%d", identityName, camera, imageID)
	
	// 2. Detaillierte Attribute für die Erkennung
	type PersonInfo struct {
		Name            string   `json:"name"`            // Name der erkannten Person
		Camera          string   `json:"camera"`           // Kamera, die die Erkennung gemacht hat
		Confidence      float64  `json:"confidence"`       // Erkennungswahrscheinlichkeit
		Timestamp       string   `json:"timestamp"`        // Zeitstempel im lesbaren Format
		RecentDetections []string `json:"recent_detections"` // Liste der letzten 5 Erkennungen
		ImageID         uint     `json:"image_id"`         // ID des Bildes
	}
	
	// Attribute aktualisieren
	attributesTopic := fmt.Sprintf("%s/attributes", personTopic)
	
	// Versuche, existierende Attribute zu laden
	var personInfo PersonInfo
	existingAttributes, err := p.mqttClient.GetRetainedPayload(attributesTopic)
	if err == nil && existingAttributes != "" {
		if err := json.Unmarshal([]byte(existingAttributes), &personInfo); err == nil {
			log.Debugf("Loaded existing person info")
		}
	}
	
	// Aktualisiere die letzten Erkennungen
	var recentDetections []string
	if len(personInfo.RecentDetections) > 0 {
		recentDetections = personInfo.RecentDetections
	}
	
	// Füge neue Erkennung zur Liste hinzu
	newDetection := fmt.Sprintf("%s (%s, %.0f%%, %s)", 
		identityName, 
		camera, 
		confidence * 100, 
		timestamp.Format("15:04:05"))
	
	// Begrenze Liste auf 5 Einträge
	recentDetections = append(recentDetections, newDetection)
	if len(recentDetections) > 5 {
		recentDetections = recentDetections[len(recentDetections)-5:]
	}
	
	// Aktualisierte Informationen zusammenstellen
	personInfo = PersonInfo{
		Name:            identityName,
		Camera:          camera,
		Confidence:      confidence,
		Timestamp:       timestamp.Format("2006-01-02 15:04:05"),
		RecentDetections: recentDetections,
		ImageID:         imageID,
	}
	
	// Veröffentliche aktualisierte Attribute - JSON serialisieren
	attributesJSON, err := json.Marshal(personInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal person attributes: %w", err)
	}
	if err := p.mqttClient.PublishRetain(attributesTopic, string(attributesJSON)); err != nil {
		return fmt.Errorf("failed to publish person attributes: %w", err)
	}

	// 3. Aktualisiere das Kamera-Bild - mit Zeitstempel, um Caching zu verhindern
	timestampStr := fmt.Sprintf("%d", timezone.Now().UnixNano()/int64(time.Millisecond))
	imageTopic := fmt.Sprintf("%s/image", personTopic)
	
	// Zusätzlich ein Topic mit Zeitstempel, das Home Assistant zwingt, das Vorschaubild zu aktualisieren
	imageTopicWithTimestamp := fmt.Sprintf("%s/image/%s", personTopic, timestampStr)
	fullImagePath := filepath.Join(p.cfg.Server.SnapshotDir, imagePath)
	
	// Detailed debugging to track image issues
	log.Infof("Attempting to update camera image. Image path: %s, Full path: %s", imagePath, fullImagePath)
	
	// Check if file exists before reading
	if _, fileErr := os.Stat(fullImagePath); os.IsNotExist(fileErr) {
		log.Warnf("Image file does not exist: %s", fullImagePath)
		// Try with alternative path patterns
		alternativePath := filepath.Join(p.cfg.Server.SnapshotDir, "frigate", filepath.Base(imagePath))
		log.Infof("Trying alternative path: %s", alternativePath)
		
		if _, altErr := os.Stat(alternativePath); os.IsNotExist(altErr) {
			log.Warnf("Alternative image path also does not exist: %s", alternativePath)
		} else {
			fullImagePath = alternativePath
			log.Infof("Using alternative path for image: %s", fullImagePath)
		}
	}
	
	if imageBytes, err := os.ReadFile(fullImagePath); err == nil {
		size := len(imageBytes)
		log.Infof("Successfully read image file. Size: %d bytes", size)
		
		// Maximale Größe für MQTT-Payload in Bytes (50KB)
		maxSize := 50 * 1024
		processedImageBytes := imageBytes
		
		// Wenn das Bild zu groß ist (> 50KB), müssen wir es komprimieren
		if size > maxSize {
			log.Infof("Image too large (%d bytes), attempting to compress", size)
			
			// Versuche das Bild zu dekodieren
			img, err := jpeg.Decode(bytes.NewReader(imageBytes))
			if err != nil {
				log.Warnf("Failed to decode image for compression: %v", err)
			} else {
				// Stark komprimiertes JPEG erzeugen
				buf := new(bytes.Buffer)
				
				// Qualität schrittweise reduzieren bis das Bild klein genug ist
				qualityLevels := []int{70, 50, 30, 15, 10}
				successful := false
				
				for _, quality := range qualityLevels {
					buf.Reset() // Buffer zurücksetzen für jeden Versuch
					encErr := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality})
					
					if encErr != nil {
						log.Warnf("Failed to encode image at quality %d: %v", quality, encErr)
						continue
					}
					
					// Prüfen, ob das Bild jetzt klein genug ist
					compressedSize := buf.Len()
					if compressedSize <= maxSize {
						log.Infof("Successfully compressed image to %d bytes using quality level %d", compressedSize, quality)
						processedImageBytes = buf.Bytes()
						successful = true
						break
					}
				}
				
				if !successful {
					log.Warnf("Could not compress image to required size even with lowest quality")
				}
			}
		}
		
		// Endgültige Größe prüfen
		finalSize := len(processedImageBytes)
		if finalSize > maxSize {
			log.Warnf("Image still too large after processing: %d bytes. Cannot send to MQTT.", finalSize)
		} else {
			// Das Bild ohne Base64-Präfix veröffentlichen (Home Assistant erwartet reines Base64)
			rawImageData := base64.StdEncoding.EncodeToString(processedImageBytes)
			log.Infof("Publishing image data (length: %d) to topics: %s and %s", len(rawImageData), imageTopic, imageTopicWithTimestamp)
			
			// Senden an das Standard-Topic (für bestehende Konfigurationen)
			if err := p.mqttClient.PublishRetain(imageTopic, rawImageData); err != nil {
				log.Warnf("Failed to publish image data to standard topic: %v", err)
			}
			
			// Senden an das Zeitstempel-Topic (um Caching zu verhindern)
			if err := p.mqttClient.Publish(imageTopicWithTimestamp, rawImageData); err != nil {
				log.Warnf("Failed to publish image data to timestamped topic: %v", err)
				// Wir brechen nicht ab, falls das Bildupload fehlschlägt
			} else {
				log.Infof("Successfully published image data to MQTT for camera entity")
			}
		}
	} else {
		// Bei Fehler eine Warnung ausgeben
		log.Warnf("Failed to read image file: %v", err)
	}
	
	// Aktualisiere den Zeitpunkt der letzten Erkennung
	p.lastDetections[identityName] = timestamp
	
	return nil
}

// UpdatePresenceSensor aktualisiert einen Anwesenheitssensor für eine identifizierte Person
// Diese Methode verwendet jetzt das vereinfachte Entity-Modell mit nur zwei Entitäten
func (p *Publisher) UpdatePresenceSensor(identityName string, camera string, confidence float64, imageID uint, imagePath string) error {
	// Wir verwenden jetzt nur noch die vereinfachte UpdateRecognizedPerson-Methode
	return p.UpdateRecognizedPerson(identityName, camera, confidence, imageID, imagePath)
}

// UpdateUnknownPresenceSensor aktualisiert den Anwesenheitssensor für unbekannte Personen
func (p *Publisher) UpdateUnknownPresenceSensor(camera string, imageID uint, imagePath string, count int) error {
	// Erstelle einen Identitätsnamen für Unbekannte basierend auf der Anzahl
	identityName := "Unbekannt"
	if count > 1 {
		identityName = fmt.Sprintf("%d Unbekannte", count)
	}
	
	// Verwende die vereinfachte UpdateRecognizedPerson-Methode mit einer Vertrauenswürdigkeit von 0
	return p.UpdateRecognizedPerson(identityName, camera, 0, imageID, imagePath)
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
	p.personLastUpdate[camera] = timezone.Now()
	
	// Zähler veröffentlichen
	// Bereinigten Kameranamen verwenden
	cleanCamera := cleanCameraName(camera)
	topic := fmt.Sprintf("double-take/cameras/%s/person", cleanCamera)
	if err := p.mqttClient.Publish(topic, fmt.Sprintf("%d", counter)); err != nil {
		log.Errorf("Failed to publish person counter for camera %s: %v", camera, err)
	}
}
