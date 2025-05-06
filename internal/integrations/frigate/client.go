package frigate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"

	log "github.com/sirupsen/logrus"
)

// FrigateClient verwaltet die Interaktion mit einer Frigate NVR-Instanz
type FrigateClient struct {
	config     config.FrigateConfig
	httpClient *http.Client
}

// FrigateEvent repräsentiert ein Ereignis von Frigate, das über MQTT empfangen wurde
type FrigateEvent struct {
	Before  *FrigateEventData `json:"before,omitempty"`
	After   *FrigateEventData `json:"after,omitempty"`
	Type    string            `json:"type"`
	Camera  string            `json:"camera"`
	ID      string            `json:"id"`
}

// FrigateEventData enthält die Details eines Frigate-Ereignisses
type FrigateEventData struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	Score       float64   `json:"score"`
	TopScore    float64   `json:"top_score"`
	Type        string    `json:"type"`
	Camera      string    `json:"camera"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time,omitempty"`
	CurrentTime time.Time `json:"current_time"`
	Thumbnail   string    `json:"thumbnail"`
	Snapshot    string    `json:"snapshot"`
	Zone        string    `json:"zone"`
}

// NewFrigateClient erstellt einen neuen Frigate-Client
func NewFrigateClient(config config.FrigateConfig) *FrigateClient {
	return &FrigateClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ParseEventMessage analysiert ein MQTT-Event-Nachrichten-Payload von Frigate
func (c *FrigateClient) ParseEventMessage(payload []byte) (*FrigateEvent, error) {
	var event FrigateEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Frigate event: %w", err)
	}
	return &event, nil
}

// IsPersonEvent prüft, ob das Ereignis eine Person betrifft
func (c *FrigateClient) IsPersonEvent(event *FrigateEvent) bool {
	if event.After != nil && event.After.Label == "person" {
		return true
	}
	return false
}

// GetEventData extrahiert die relevanten Daten aus einem Frigate-Ereignis
func (c *FrigateClient) GetEventData(event *FrigateEvent) *FrigateEventData {
	if event.After != nil {
		return event.After
	}
	return event.Before
}

// DownloadSnapshot lädt einen Snapshot von einer Frigate-Instanz herunter
func (c *FrigateClient) DownloadSnapshot(snapshotPath string, savePath string) error {
	if !c.config.Enabled {
		return fmt.Errorf("frigate integration is disabled")
	}

	// Verwende Host aus der neuen Konfiguration, mit Fallback auf Legacy-Felder
	hostURL := c.config.Host
	if hostURL == "" {
		// Fallback auf alte Konfigurationsfelder
		if c.config.APIURL != "" {
			hostURL = c.config.APIURL
		} else if c.config.URL != "" {
			hostURL = c.config.URL
		} else {
			return fmt.Errorf("no frigate host URL configured")
		}
	}

	url := fmt.Sprintf("%s%s", hostURL, snapshotPath)
	log.Debugf("Downloading snapshot from: %s", url)

	// HTTP GET-Anfrage
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download snapshot, status code: %d", resp.StatusCode)
	}

	// Datei zum Speichern öffnen
	outFile, err := createFile(savePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	// Snapshot in die Datei kopieren
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	log.Debugf("Snapshot saved to: %s", savePath)
	return nil
}

// GenerateFilename generiert einen Dateinamen für ein Frigate-Ereignis
func (c *FrigateClient) GenerateFilename(event *FrigateEventData) string {
	// Format: frigate_camera_eventID_timestamp.jpg
	timestamp := event.CurrentTime.Format("20060102_150405")
	return fmt.Sprintf("frigate_%s_%s_%s.jpg", event.Camera, event.ID, timestamp)
}

// ToImage konvertiert ein Frigate-Ereignis in ein Bild-Modell
func (c *FrigateClient) ToImage(event *FrigateEventData, filePath string) *models.Image {
	// JSON-Daten für SourceData vorbereiten
	sourceDataJSON, err := json.Marshal(map[string]interface{}{
		"camera":    event.Camera,
		"event_id":  event.ID,
		"label":     event.Label,
		"score":     event.Score,
		"top_score": event.TopScore,
	})
	
	if err != nil {
		log.Errorf("Fehler beim Serialisieren der Frigatedaten: %v", err)
		sourceDataJSON = []byte("{}")
	}
	
	return &models.Image{
		Source:      "frigate",
		EventID:     event.ID,
		Label:       event.Label,
		Zone:        event.Zone,
		FilePath:    filePath,
		Timestamp:   event.CurrentTime,
		ContentHash: "",  // Wird später berechnet
		SourceData:  sourceDataJSON,
		// Temporäre Felder für die Verarbeitung
		DetectedAt:  event.CurrentTime,
		FileName:    filepath.Base(filePath),
	}
}

// Hilfsfunktion zum Erstellen der Datei und des übergeordneten Verzeichnisses
func createFile(filePath string) (*os.File, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return os.Create(filePath)
}
