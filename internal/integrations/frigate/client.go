package frigate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	ID          string       `json:"id"`
	Camera      string       `json:"camera"`
	Label       string       `json:"label"`
	SubLabel    string       `json:"sub_label,omitempty"`
	Score       float64      `json:"score"`
	TopScore    float64      `json:"top_score"`
	FalsePositive bool       `json:"false_positive"`
	StartTime   interface{}  `json:"start_time"` // Kann unterschiedliche Formate haben
	EndTime     interface{}  `json:"end_time,omitempty"`
	Box         []int        `json:"box"`
	Area        int          `json:"area"`
	Ratio       float64      `json:"ratio"`
	Region      []int        `json:"region"`
	Active      bool         `json:"active"`
	Stationary  bool         `json:"stationary"`
	MotionlessCount int      `json:"motionless_count"`
	CurrentZones   []string  `json:"current_zones"`
	EnteredZones   []string  `json:"entered_zones"`
	HasClip     bool         `json:"has_clip"`
	HasSnapshot bool         `json:"has_snapshot"`
	CurrentAttributes []string `json:"current_attributes"`
	FrameTime   interface{}  `json:"frame_time"`
	Snapshot    interface{}  `json:"snapshot"` // Kann ein String oder ein Objekt sein
	Thumbnail   interface{}  `json:"thumbnail"` // Kann ein String oder ein Objekt sein
	PathData    [][]interface{} `json:"path_data,omitempty"`
}

// GetStartTime konvertiert StartTime in ein time.Time-Objekt
func (d *FrigateEventData) GetStartTime() time.Time {
	return parseTimeValue(d.StartTime)
}

// GetEndTime konvertiert EndTime in ein time.Time-Objekt
func (d *FrigateEventData) GetEndTime() time.Time {
	return parseTimeValue(d.EndTime)
}

// GetCurrentTime liefert die aktuelle Zeit des Events, verwendet FrameTime
func (d *FrigateEventData) GetCurrentTime() time.Time {
	// Verwende FrameTime, falls vorhanden, ansonsten StartTime
	if d.FrameTime != nil {
		return parseTimeValue(d.FrameTime)
	}
	// Fallback auf StartTime
	return d.GetStartTime()
}

// GetSnapshotURL extrahiert die Snapshot-URL aus dem Snapshot-Feld
func (d *FrigateEventData) GetSnapshotURL() string {
	// Wenn die has_snapshot Flag false ist, gibt es keinen Snapshot
	if !d.HasSnapshot {
		return ""
	}
	
	// Wenn wir eine ID haben, versuchen wir zuerst das neue Frigate API-Format
	if d.ID != "" {
		// Neues Frigate API-Format: /events/{id}/snapshot.jpg
		return fmt.Sprintf("/events/%s/snapshot.jpg", d.ID)
	}
	
	// Fallback: Extrahiere URL mit Kamera-Kontext aus dem Snapshot-Feld
	return extractURLWithCamera(d.Snapshot, d.Camera)
}

// GetThumbnailURL extrahiert die Thumbnail-URL aus dem Thumbnail-Feld
func (d *FrigateEventData) GetThumbnailURL() string {
	// Extrahiere URL mit Kamera-Kontext
	return extractURLWithCamera(d.Thumbnail, d.Camera)
}

// extractURLWithCamera extrahiert eine URL aus verschiedenen möglichen Formaten mit Kamerakontext
func extractURLWithCamera(value interface{}, camera string) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		// Einfache String-URL
		return v
		
	case map[string]interface{}:
		// Frigate v16+ Format: Das Snapshot-Objekt enthält evtl. eine URL direkt
		if v["url"] != nil {
			if url, ok := v["url"].(string); ok {
				return url
			}
		}

		// Frigate v16+ Format mit Frame-ID und Kamera aus dem Event
		if v["frame_time"] != nil {
			hasCamera := camera != ""
			
			// Frame-Zeit extrahieren
			var frameTime float64
			switch ft := v["frame_time"].(type) {
			case float64:
				frameTime = ft
			case int64:
				frameTime = float64(ft)
			case string:
				if f, err := strconv.ParseFloat(ft, 64); err == nil {
					frameTime = f
				}
			default:
				log.Warnf("Unerwarteter Typ für frame_time: %T", ft)
			}

			if hasCamera && frameTime > 0 {
				// Aktuelles Frigate v16 Snapshot-URL-Format
				// Probiere verschiedene Formate aus, basierend auf der Dokumentation und üblichen Mustern
				
				// Wir benötigen die ID für den neuen Frigate API-Pfad
				var eventID string
				if v["id"] != nil {
					if id, ok := v["id"].(string); ok {
						eventID = id
					}
				}

				// Hier können wir nur die ID aus dem Snapshot-Objekt selbst verwenden

				// Format 1: /events/{id}/snapshot.jpg (neues Frigate API-Format ohne /api/ Präfix)
				if eventID != "" {
					return fmt.Sprintf("/events/%s/snapshot.jpg", eventID)
				}
				
				// Format 2: Fallback auf altes Format
				return fmt.Sprintf("/api/snapshots/%s/%f.jpg", camera, frameTime)
			}
		}
		
		// Alternatives Feld mit Pfad (für ältere Frigate-Versionen)
		if v["path"] != nil {
			if path, ok := v["path"].(string); ok {
				return path
			}
		}
		
		// Wenn kein bekanntes Feld gefunden wurde, als JSON protokollieren
		jsonData, _ := json.Marshal(v)
		log.Warnf("Unbekanntes URL-Objekt: %s", string(jsonData))
		return ""
		
	default:
		// Unbekannter Typ
		log.Warnf("Unbekannter URL-Typ: %T", v)
		return ""
	}
}

// extractURL extrahiert eine URL aus verschiedenen möglichen Formaten
// Diese Funktion wird für Rückwärtskompatibilität beibehalten
func extractURL(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		// Einfache String-URL
		return v
		
	case map[string]interface{}:
		// Frigate v16+ Format: Das Snapshot-Objekt enthält evtl. eine URL direkt
		if v["url"] != nil {
			if url, ok := v["url"].(string); ok {
				return url
			}
		}

		// Für Frigate v16+ mit Frame-ID
		// In dieser Version der Funktion schauen wir nur nach URL oder path
		// Die camera wird in extractURLWithCamera verwendet
		
		// Alternatives Feld mit Pfad (für ältere Frigate-Versionen)
		if v["path"] != nil {
			if path, ok := v["path"].(string); ok {
				return path
			}
		}
		
		// Wenn kein bekanntes Feld gefunden wurde, als JSON protokollieren
		jsonData, _ := json.Marshal(v)
		log.Warnf("Unbekanntes URL-Objekt: %s", string(jsonData))
		return ""
		
	default:
		// Unbekannter Typ
		log.Warnf("Unbekannter URL-Typ: %T", v)
		return ""
	}
}

// parseTimeValue konvertiert verschiedene Zeitformaty in time.Time
func parseTimeValue(value interface{}) time.Time {
	// Standardwert
	defaultTime := time.Now()
	
	// Verschiedene mögliche Formate verarbeiten
	switch v := value.(type) {
	case string:
		// Standardformat für Zeitstring (RFC3339)
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			return t
		}
		
		// Alternatives Format (ISO8601)
		t, err = time.Parse("2006-01-02T15:04:05.999999999Z07:00", v)
		if err == nil {
			return t
		}
		
		// Unix-Timestamp als String
		if seconds, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(seconds, 0)
		}
		
		// Fehler beim Parsen
		log.Warnf("Unbekanntes Zeitformat in String: %s", v)
		return defaultTime
		
	case float64:
		// Unix-Timestamp als Zahl
		return time.Unix(int64(v), 0)
		
	case int64:
		// Unix-Timestamp als Ganzzahl
		return time.Unix(v, 0)
		
	case map[string]interface{}:
		// Wenn es ein komplexes Objekt ist, versuchen wir es als Unix-Timestamp zu interpretieren
		// Dies ist ein Fallback für unerwartete Strukturen
		log.Warnf("Zeitwert ist ein Objekt, versuche Seconds-Feld zu finden: %v", v)
		if seconds, ok := v["seconds"].(float64); ok {
			return time.Unix(int64(seconds), 0)
		}
		return defaultTime
		
	default:
		// Unbekannter Typ
		log.Warnf("Unbekannter Zeitwert-Typ: %T %v", v, v)
		return defaultTime
	}
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

	// Prüfen, ob der Pfad leer ist
	if snapshotPath == "" {
		return fmt.Errorf("empty snapshot path")
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

	// Stelle sicher, dass hostURL nicht mit einem Schrägstrich endet
	if strings.HasSuffix(hostURL, "/") {
		hostURL = hostURL[:len(hostURL)-1]
	}

	// Stelle sicher, dass snapshotPath mit einem Schrägstrich beginnt, wenn es kein vollständiges URL ist
	if !strings.HasPrefix(snapshotPath, "http") && !strings.HasPrefix(snapshotPath, "/") {
		snapshotPath = "/" + snapshotPath
	}

	// Wenn der Pfad bereits eine vollständige URL ist, diese direkt verwenden
	url := snapshotPath
	if !strings.HasPrefix(snapshotPath, "http") {
		// Wenn es das neue API-Format ist (ohne /api/ Präfix), füge /api/ hinzu
		if strings.HasPrefix(snapshotPath, "/events/") {
			// Neues API-Format verwendet /api vor dem Pfad
			url = fmt.Sprintf("%s/api%s", hostURL, snapshotPath)
		} else {
			// Andernfalls den Host voranstellen
			url = fmt.Sprintf("%s%s", hostURL, snapshotPath)
		}
	}

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
	timestamp := event.GetCurrentTime().Format("20060102_150405")
	return fmt.Sprintf("frigate_%s_%s_%s.jpg", event.Camera, event.ID, timestamp)
}

// ToImage konvertiert ein Frigate-Ereignis in ein Bild-Modell
func (c *FrigateClient) ToImage(event *FrigateEventData, filePath string) *models.Image {
	// Zeiten mit Getter-Methoden abrufen
	currentTime := event.GetCurrentTime()
	
	// JSON-Daten für SourceData vorbereiten
	sourceDataJSON, err := json.Marshal(map[string]interface{}{
		"camera":          event.Camera,
		"event_id":        event.ID,
		"label":           event.Label,
		"score":           event.Score,
		"top_score":       event.TopScore,
		"start_time":      event.GetStartTime().Format(time.RFC3339),
		"current_zones":   event.CurrentZones,
		"entered_zones":   event.EnteredZones,
	})
	
	if err != nil {
		log.Errorf("Fehler beim Serialisieren der Frigatedaten: %v", err)
		sourceDataJSON = []byte("{}")
	}

	// Kombiniere die Zonen zu einem String
	var zoneString string
	if len(event.CurrentZones) > 0 {
		zoneString = strings.Join(event.CurrentZones, ",")
	} else if len(event.EnteredZones) > 0 {
		zoneString = strings.Join(event.EnteredZones, ",")
	}
	
	return &models.Image{
		Source:      "frigate",
		EventID:     event.ID,
		Label:       event.Label,
		Zone:        zoneString,
		FilePath:    filePath,
		Timestamp:   currentTime,
		ContentHash: "",  // Wird später berechnet
		SourceData:  sourceDataJSON,
		// Temporäre Felder für die Verarbeitung
		DetectedAt:  currentTime,
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
