package handlers

import (
	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/core/processor"
	"double-take-go-reborn/internal/integrations/compreface"

	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// APIHandler behandelt API-Anfragen für das System
type APIHandler struct {
	db            *gorm.DB
	cfg           *config.Config
	compreface    *compreface.Client
	imageProcessor *processor.ImageProcessor
}

// NewAPIHandler erstellt einen neuen API-Handler
func NewAPIHandler(db *gorm.DB, cfg *config.Config, compreface *compreface.Client, imageProcessor *processor.ImageProcessor) *APIHandler {
	return &APIHandler{
		db:            db,
		cfg:           cfg,
		compreface:    compreface,
		imageProcessor: imageProcessor,
	}
}

// RegisterRoutes registriert alle API-Routen
func (h *APIHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Verarbeitungs-Endpunkte
	router.POST("/process/image", h.ProcessImage)
	router.POST("/process/compreface", h.ProcessCompreFace)

	// Bilder-Endpunkte
	router.GET("/images", h.ListImages)
	router.GET("/images/:id", h.GetImage)
	router.DELETE("/images/:id", h.DeleteImage)
	router.POST("/images/:id/recognize", h.RecognizeImage)

	// Identitäts-Endpunkte
	router.GET("/identities", h.ListIdentities)
	router.POST("/identities", h.CreateIdentity)
	router.GET("/identities/:id", h.GetIdentity)
	router.PUT("/identities/:id", h.UpdateIdentity)
	router.DELETE("/identities/:id", h.DeleteIdentity)
	router.POST("/identities/:id/examples", h.AddIdentityExample)

	// System-Endpunkte
	router.GET("/status", h.GetStatus)
	router.POST("/sync/compreface", h.SyncCompreFace)
}

// ProcessImage verarbeitet ein hochgeladenes Bild
func (h *APIHandler) ProcessImage(c *gin.Context) {
	// Datei aus Formular erhalten
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded or invalid form data"})
		return
	}
	defer file.Close()

	// Quelle aus Formular erhalten (optional)
	source := c.PostForm("source")
	if source == "" {
		source = "api_upload"
	}

	// Temporären Dateinamen generieren
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s", timestamp, header.Filename)
	filePath := filepath.Join(h.cfg.Server.SnapshotDir, filename)

	// Zielverzeichnis sicherstellen
	os.MkdirAll(filepath.Dir(filePath), 0755)

	// Datei auf Festplatte speichern
	outFile, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create file: %v", err)})
		return
	}
	defer outFile.Close()

	// Inhalt kopieren
	if _, err = io.Copy(outFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save file: %v", err)})
		return
	}

	// Bild verarbeiten
	ctx := c.Request.Context()
	image, err := h.imageProcessor.ProcessImage(ctx, filePath, source, processor.ProcessingOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Image processing failed: %v", err)})
		return
	}

	// Preload relevante Beziehungen für Antwort
	if err := h.db.Preload("Faces.Matches.Identity").First(&image, image.ID).Error; err != nil {
		log.Errorf("Failed to preload image relationships: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Image processed successfully",
		"image": image,
	})
}

// ProcessCompreFace verarbeitet ein Bild direkt mit CompreFace
func (h *APIHandler) ProcessCompreFace(c *gin.Context) {
	if !h.cfg.CompreFace.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CompreFace integration is not enabled"})
		return
	}

	// Datei aus Formular erhalten
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded or invalid form data"})
		return
	}
	defer file.Close()

	// Bilddaten lesen
	imageData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read image data: %v", err)})
		return
	}

	// CompreFace-Erkennung
	ctx := c.Request.Context()
	result, err := h.compreface.Recognize(ctx, imageData, header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("CompreFace recognition failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "CompreFace processing completed",
		"result": result,
	})
}

// ListImages gibt eine Liste von Bildern zurück
func (h *APIHandler) ListImages(c *gin.Context) {
	var images []models.Image
	
	// Paginierung
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	offset := (page - 1) * pageSize

	// Filtern nach Quelle (optional)
	source := c.Query("source")
	
	// Query vorbereiten
	query := h.db.Model(&models.Image{}).Order("created_at DESC")
	
	if source != "" {
		query = query.Where("source = ?", source)
	}
	
	// Gesamtanzahl für Paginierung abrufen
	var total int64
	query.Count(&total)
	
	// Bilder abrufen
	if err := query.Offset(offset).Limit(pageSize).Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch images: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"images": images,
		"pagination": gin.H{
			"page": page,
			"pageSize": pageSize,
			"total": total,
		},
	})
}

// GetImage gibt ein einzelnes Bild mit Details zurück
func (h *APIHandler) GetImage(c *gin.Context) {
	id := c.Param("id")
	
	var image models.Image
	if err := h.db.Preload("Faces.Matches.Identity").First(&image, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	c.JSON(http.StatusOK, image)
}

// DeleteImage löscht ein Bild
func (h *APIHandler) DeleteImage(c *gin.Context) {
	id := c.Param("id")
	
	var image models.Image
	if err := h.db.First(&image, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Physische Datei löschen
	if image.FilePath != "" {
		filePath := filepath.Join(h.cfg.Server.SnapshotDir, image.FilePath)
		if err := os.Remove(filePath); err != nil {
			log.Warnf("Failed to delete image file %s: %v", filePath, err)
			// Weiter mit Löschen des DB-Eintrags
		}
	}

	// Datenbankeintrag löschen (cascaded zu Faces und Matches)
	if err := h.db.Delete(&image).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete image: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image deleted successfully"})
}

// ListIdentities gibt eine Liste aller Identitäten zurück
func (h *APIHandler) ListIdentities(c *gin.Context) {
	var identities []models.Identity
	
	if err := h.db.Find(&identities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch identities: %v", err)})
		return
	}

	c.JSON(http.StatusOK, identities)
}

// CreateIdentity erstellt eine neue Identität
func (h *APIHandler) CreateIdentity(c *gin.Context) {
	var identity models.Identity
	
	if err := c.ShouldBindJSON(&identity); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid identity data: %v", err)})
		return
	}

	if identity.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Identity name is required"})
		return
	}

	// Prüfen, ob der Name bereits existiert
	var existingIdentity models.Identity
	if err := h.db.Where("name = ?", identity.Name).First(&existingIdentity).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Identity with this name already exists"})
		return
	}

	// Identität erstellen
	if err := h.db.Create(&identity).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create identity: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, identity)
}

// GetIdentity gibt eine einzelne Identität zurück
func (h *APIHandler) GetIdentity(c *gin.Context) {
	id := c.Param("id")
	
	var identity models.Identity
	if err := h.db.First(&identity, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Identity not found"})
		return
	}

	c.JSON(http.StatusOK, identity)
}

// UpdateIdentity aktualisiert eine Identität
func (h *APIHandler) UpdateIdentity(c *gin.Context) {
	id := c.Param("id")
	
	var identity models.Identity
	if err := h.db.First(&identity, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Identity not found"})
		return
	}

	var updateData models.Identity
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid identity data: %v", err)})
		return
	}

	// Name aktualisieren, wenn angegeben
	if updateData.Name != "" && updateData.Name != identity.Name {
		// Prüfen, ob der neue Name bereits existiert
		var existingIdentity models.Identity
		if err := h.db.Where("name = ? AND id != ?", updateData.Name, id).First(&existingIdentity).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Identity with this name already exists"})
			return
		}
		
		identity.Name = updateData.Name
	}

	// ExternalID aktualisieren, wenn angegeben
	if updateData.ExternalID != "" {
		identity.ExternalID = updateData.ExternalID
	}

	// Änderungen speichern
	if err := h.db.Save(&identity).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update identity: %v", err)})
		return
	}

	c.JSON(http.StatusOK, identity)
}

// DeleteIdentity löscht eine Identität
func (h *APIHandler) DeleteIdentity(c *gin.Context) {
	id := c.Param("id")
	
	var identity models.Identity
	if err := h.db.First(&identity, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Identity not found"})
		return
	}

	// Identität löschen
	if err := h.db.Delete(&identity).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete identity: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Identity deleted successfully"})
}

// AddIdentityExample fügt ein Beispielbild zu einer Identität hinzu
func (h *APIHandler) AddIdentityExample(c *gin.Context) {
	if !h.cfg.CompreFace.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CompreFace integration is not enabled"})
		return
	}

	id := c.Param("id")
	
	var identity models.Identity
	if err := h.db.First(&identity, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Identity not found"})
		return
	}

	// Datei aus Formular erhalten
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded or invalid form data"})
		return
	}
	defer file.Close()

	// Bilddaten lesen
	imageData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read image data: %v", err)})
		return
	}

	// Beispiel zu CompreFace hinzufügen
	ctx := c.Request.Context()
	result, err := h.compreface.AddSubjectExample(ctx, identity.Name, imageData, header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to add example to CompreFace: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Example added successfully",
		"result": result,
	})
}

// GetStatus gibt den Systemstatus zurück
func (h *APIHandler) GetStatus(c *gin.Context) {
	status := gin.H{
		"status": "ok",
		"timestamp": time.Now(),
		"compreface": gin.H{
			"enabled": h.cfg.CompreFace.Enabled,
		},
	}

	// CompreFace-Konnektivität prüfen, wenn aktiviert
	if h.cfg.CompreFace.Enabled {
		ctx := c.Request.Context()
		reachable, err := h.compreface.Ping(ctx)
		status["compreface"].(gin.H)["reachable"] = reachable
		if err != nil {
			status["compreface"].(gin.H)["error"] = err.Error()
		}
	}

	c.JSON(http.StatusOK, status)
}

// SyncCompreFace synchronisiert Identitäten mit CompreFace
func (h *APIHandler) SyncCompreFace(c *gin.Context) {
	if !h.cfg.CompreFace.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CompreFace integration is not enabled"})
		return
	}

	ctx := c.Request.Context()
	if err := h.compreface.SyncIdentities(ctx, h.db); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("CompreFace synchronization failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "CompreFace synchronization completed successfully"})
}

// RecognizeImage verarbeitet ein Bild neu
func (h *APIHandler) RecognizeImage(c *gin.Context) {
	id := c.Param("id")
	
	var image models.Image
	if err := h.db.First(&image, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Vollständigen Pfad zum Bild erstellen
	imagePath := filepath.Join(h.cfg.Server.SnapshotDir, image.FilePath)
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Image file not found at path: %s", image.FilePath)})
		return
	}

	// Bild neu verarbeiten
	ctx := c.Request.Context()
	processingOptions := processor.ProcessingOptions{
		DetectFaces:    true,
		RecognizeFaces: true,
	}
	_, err := h.imageProcessor.ProcessImage(ctx, imagePath, image.Source, processingOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Image processing failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image reprocessed successfully"})
}
