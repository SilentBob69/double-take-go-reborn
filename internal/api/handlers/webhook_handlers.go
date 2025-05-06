package handlers

import (
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/core/processor"
	"double-take-go-reborn/internal/db/repository"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

// WebhookHandler verarbeitet eingehende Webhook-Anfragen
type WebhookHandler struct {
	imageProcessor *processor.ImageProcessor
	repo           repository.Repository
	snapshotDir    string
}

// NewWebhookHandler erstellt einen neuen Webhook-Handler
func NewWebhookHandler(imageProcessor *processor.ImageProcessor, repo repository.Repository, snapshotDir string) *WebhookHandler {
	return &WebhookHandler{
		imageProcessor: imageProcessor,
		repo:           repo,
		snapshotDir:    snapshotDir,
	}
}

// WebhookRequest repräsentiert die Anfrage an einen Webhook
type WebhookRequest struct {
	ImageURL     string                 `json:"image_url,omitempty"`
	ImageBase64  string                 `json:"image_base64,omitempty"`
	Source       string                 `json:"source,omitempty"`
	CameraName   string                 `json:"camera_name,omitempty"`
	Timestamp    string                 `json:"timestamp,omitempty"`
	DetectedAt   time.Time              `json:"detected_at,omitempty"`
	SourceData   map[string]interface{} `json:"source_data,omitempty"`
	DetectFaces  bool                   `json:"detect_faces,omitempty"`
	RecognizeFaces bool                 `json:"recognize_faces,omitempty"`
}

// ReceiveWebhook verarbeitet eingehende Webhook-Anfragen
func (h *WebhookHandler) ReceiveWebhook(c *gin.Context) {
	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf("Failed to parse webhook request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges Anforderungsformat"})
		return
	}

	// Validierung
	if req.ImageURL == "" && req.ImageBase64 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bild-URL oder Base64-kodiertes Bild erforderlich"})
		return
	}

	// Quelle setzen, falls nicht angegeben
	if req.Source == "" {
		req.Source = "webhook"
	}

	// Zeit verarbeiten
	now := time.Now()
	detectedAt := now
	
	if req.Timestamp != "" {
		if parsedTime, err := time.Parse(time.RFC3339, req.Timestamp); err == nil {
			detectedAt = parsedTime
		} else {
			log.Warnf("Konnte Zeitstempel nicht parsen: %v", err)
		}
	} else if !req.DetectedAt.IsZero() {
		detectedAt = req.DetectedAt
	}

	// Datei herunterladen oder aus Base64 dekodieren
	var imageBytes []byte
	var err error
	
	if req.ImageURL != "" {
		// Bild von URL herunterladen
		resp, err := http.Get(req.ImageURL)
		if err != nil {
			log.Errorf("Fehler beim Herunterladen des Bildes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Bild konnte nicht heruntergeladen werden"})
			return
		}
		defer resp.Body.Close()
		
		imageBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("Fehler beim Lesen des Bildinhalts: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Bild konnte nicht gelesen werden"})
			return
		}
	} else if req.ImageBase64 != "" {
		// Base64-kodiertes Bild dekodieren
		imageBytes, err = decodeBase64Image(req.ImageBase64)
		if err != nil {
			log.Errorf("Fehler beim Dekodieren des Base64-Bildes: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges Base64-kodiertes Bild"})
			return
		}
	}

	// Dateinamen generieren
	filename := generateWebhookFilename(req.Source, req.CameraName, detectedAt)
	filepath := filepath.Join(h.snapshotDir, filename)

	// Bild auf Festplatte speichern
	if err := saveImageFile(imageBytes, filepath); err != nil {
		log.Errorf("Fehler beim Speichern des Bildes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bild konnte nicht gespeichert werden"})
		return
	}

	// Bild in die Datenbank einfügen
	image := &models.Image{
		Source:      req.Source,
		FileName:    filename,
		FilePath:    filename,
		DetectedAt:  detectedAt,
		SourceData:  datatypes.JSON{},
	}

	if err := h.repo.SaveImage(image); err != nil {
		log.Errorf("Fehler beim Speichern des Bildes in der Datenbank: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bild konnte nicht in der Datenbank gespeichert werden"})
		return
	}

	// Bild für die Gesichtserkennung verarbeiten
	processingOptions := processor.ProcessingOptions{
		DetectFaces:    req.DetectFaces,
		RecognizeFaces: req.RecognizeFaces,
	}

	ctx := c.Request.Context()
	_, err = h.imageProcessor.ProcessImage(ctx, filepath, req.Source, processingOptions)
	if err != nil {
		log.Errorf("Fehler bei der Bildverarbeitung: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler bei der Bildverarbeitung"})
		return
	}

	// Erfolgsantwort senden
	c.JSON(http.StatusOK, gin.H{
		"message": "Bild erfolgreich verarbeitet",
		"image_id": image.ID,
	})
}

// Hilfsfunktionen

// decodeBase64Image dekodiert ein Base64-kodiertes Bild
func decodeBase64Image(base64Data string) ([]byte, error) {
	// Implementierung hier
	// Beispiel: return base64.StdEncoding.DecodeString(strings.TrimPrefix(base64Data, "data:image/jpeg;base64,"))
	return nil, nil // Platzhalter
}

// generateWebhookFilename generiert einen Dateinamen für ein über Webhook empfangenes Bild
func generateWebhookFilename(source, cameraName string, timestamp time.Time) string {
	timeStr := timestamp.Format("20060102_150405")
	
	if cameraName == "" {
		cameraName = "unknown"
	}
	
	return filepath.Join("webhook", source, cameraName, timeStr+".jpg")
}

// saveImageFile speichert ein Bild auf der Festplatte
func saveImageFile(imageData []byte, filePath string) error {
	// Implementierung hier
	// Beispiel: return ioutil.WriteFile(filePath, imageData, 0644)
	return nil // Platzhalter
}
