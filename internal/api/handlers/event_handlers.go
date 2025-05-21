package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"double-take-go-reborn/internal/core/models"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// EventHandler behandelt Anfragen für Event-Gruppen
type EventHandler struct {
	db *gorm.DB
	// Referenz zum WebHandler für Template-Rendering
	webHandler *WebHandler
}

// NewEventHandler erstellt einen neuen Event-Handler
func NewEventHandler(db *gorm.DB, webHandler *WebHandler) *EventHandler {
	return &EventHandler{
		db: db,
		webHandler: webHandler,
	}
}

// RegisterRoutes registriert alle API-Routen für Event-Gruppen
func (h *EventHandler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		// API Endpunkte für Event-Gruppen
		api.DELETE("/events/:event_id", h.handleDeleteEvent)
		api.GET("/events/:event_id", h.handleGetEventDetails)
	}

	// Web-Routen
	router.GET("/events/:event_id", h.handleEventDetailPage)
}

// handleDeleteEvent löscht eine Event-Gruppe und alle zugehörigen Bilder
func (h *EventHandler) handleDeleteEvent(c *gin.Context) {
	eventID := c.Param("event_id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Keine Event-ID angegeben"})
		return
	}

	log.Infof("Lösche Event mit ID: %s", eventID)

	// Zuerst alle zugehörigen Bilder finden
	var images []models.Image
	result := h.db.Where("event_id = ?", eventID).Find(&images)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Suchen von Bildern für das Event"})
		return
	}

	// Keine Bilder für dieses Event gefunden
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Keine Bilder für dieses Event gefunden"})
		return
	}

	// Alle Bilder löschen (GORM wird die Kaskadenlöschung für Faces und Matches vornehmen)
	deleteResult := h.db.Delete(&images)
	if deleteResult.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Löschen der Bilder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Event und alle zugehörigen Bilder erfolgreich gelöscht",
		"count":   deleteResult.RowsAffected,
	})
}

// handleGetEventDetails gibt Details zu einer Event-Gruppe zurück
func (h *EventHandler) handleGetEventDetails(c *gin.Context) {
	eventID := c.Param("event_id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Keine Event-ID angegeben"})
		return
	}

	// Alle Bilder für dieses Event finden
	var images []models.Image
	result := h.db.Where("event_id = ?", eventID).
		Order("timestamp DESC").
		Preload("Faces.Matches.Identity").
		Find(&images)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Laden der Event-Details"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event nicht gefunden"})
		return
	}

	// Antwort formatieren
	c.JSON(http.StatusOK, gin.H{
		"event_id": eventID,
		"count":    len(images),
		"images":   images,
	})
}

// Zusätzliche Strukturen für die Datenaufbereitung

// ImageData repräsentiert die aufbereiteten Bilddaten für die Template-Anzeige
type ImageData struct {
	ID         uint
	URL        string
	Timestamp  time.Time
	Source     string
	HasFaces   bool
	HasMatches bool
	FaceCount  int
	MatchCount int
	Faces      []FaceData
}

// FaceData repräsentiert die aufbereiteten Gesichtsdaten für die Template-Anzeige
type FaceData struct {
	ID         uint
	HasMatch   bool
	MatchName  string
	Confidence float64
	Provider   string    // Erkennungsprovider (CompreFace, InsightFace, etc.)
}

// createEventGroupFromImages erstellt eine EventGroup aus einer Liste von Bildern
func createEventGroupFromImages(eventID string, images []models.Image) EventGroup {
	if len(images) == 0 {
		return EventGroup{
			EventID: eventID,
			Count:   0,
		}
	}
	
	// Erste Bild als Referenz nehmen
	firstImage := images[0]
	
	// Prüfen, ob Bilder mit Gesichtern oder Matches dabei sind
	hasFaces := false
	hasMatches := false
	for _, img := range images {
		if len(img.Faces) > 0 {
			hasFaces = true
			
			for _, face := range img.Faces {
				if len(face.Matches) > 0 {
					hasMatches = true
					break
				}
			}
		}
		
		if hasFaces && hasMatches {
			break
		}
	}
	
	// Pfad für das Thumbnail erstellen
	thumbnailURL := "/snapshots/" + firstImage.FilePath
	
	// Kameraname aus SourceData extrahieren
	cameraName := firstImage.Source // Standard-Fallback
	
	// Wenn SourceData vorhanden ist, extrahiere den Kameranamen
	if len(firstImage.SourceData) > 0 {
		var data map[string]interface{}
		if err := json.Unmarshal(firstImage.SourceData, &data); err == nil {
			if camera, ok := data["camera"].(string); ok && camera != "" {
				cameraName = camera
			}
		}
	}
	
	// Event-Gruppe erstellen
	return EventGroup{
		EventID:      eventID,
		Images:       images,
		HasFaces:     hasFaces,
		HasMatches:   hasMatches,
		ThumbnailURL: thumbnailURL,
		Source:       firstImage.Source,
		Camera:       cameraName, // Setze den extrahierten Kameranamen
		Label:        firstImage.Label,
		Zone:         firstImage.Zone,
		Timestamp:    firstImage.Timestamp,
		Count:        len(images),
	}
}

// prepareImagesForTemplate bereitet Bilder für die Anzeige im Template vor
func prepareImagesForTemplate(images []models.Image) []ImageData {
	imageData := make([]ImageData, 0, len(images))
	
	for _, img := range images {
		faceCount := len(img.Faces)
		matchCount := 0
		faces := make([]FaceData, 0, faceCount)
		
		for _, face := range img.Faces {
			hasMatch := len(face.Matches) > 0
			matchName := ""
			confidence := 0.0
			
			if hasMatch {
				matchCount++
				match := face.Matches[0] // Beste Übereinstimmung nehmen
				// Identity ist bereits von GORM geladen, kein Null-Check nötig
				matchName = match.Identity.Name
				confidence = match.Confidence * 100 // Als Prozent anzeigen
			}
			
			faces = append(faces, FaceData{
				ID:         face.ID,
				HasMatch:   hasMatch,
				MatchName:  matchName,
				Confidence: confidence,
				Provider:   face.Detector,
			})
		}
		
		// Pfad für das Bild erstellen
		imageURL := "/snapshots/" + img.FilePath
		
		imageData = append(imageData, ImageData{
			ID:         img.ID,
			URL:        imageURL,
			Timestamp:  img.Timestamp,
			Source:     img.Source,
			HasFaces:   faceCount > 0,
			HasMatches: matchCount > 0,
			FaceCount:  faceCount,
			MatchCount: matchCount,
			Faces:      faces,
		})
	}
	
	return imageData
}

// handleEventDetailPage zeigt eine Detailseite für ein Event an
func (h *EventHandler) handleEventDetailPage(c *gin.Context) {
	// Prüfen, ob der WebHandler gesetzt ist
	if h.webHandler == nil {
		log.Error("WebHandler ist nicht gesetzt")
		c.String(http.StatusInternalServerError, "Interner Serverfehler: WebHandler nicht verfügbar")
		return
	}

	eventID := c.Param("event_id")
	if eventID == "" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Alle Bilder für dieses Event finden
	var images []models.Image
	result := h.db.Where("event_id = ?", eventID).
		Order("timestamp DESC").
		Preload("Faces.Matches.Identity").
		Find(&images)

	if result.Error != nil {
		log.Errorf("Fehler beim Laden der Bilder für Event %s: %v", eventID, result.Error)
		c.Redirect(http.StatusFound, "/")
		return
	}

	if result.RowsAffected == 0 {
		log.Warnf("Keine Bilder für Event %s gefunden", eventID)
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Event-Gruppe erstellen
	eventGroup := createEventGroupFromImages(eventID, images)

	// Bilder mit zusätzlichen Informationen für das Template aufbereiten
	imageData := prepareImagesForTemplate(images)

	// WebHandler nutzen, um das Template zu rendern
	h.webHandler.renderTemplate(c, "event_details", gin.H{
		"Title":    "Event " + eventID,
		"Event":    eventGroup,
		"Images":   imageData,
		"Count":    len(images),
	})
}
