package handlers

import (
	"fmt"
	"io"
	"net/http"

	"double-take-go-reborn/internal/core/models"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// TrainIdentityWithImage fügt ein hochgeladenes Bild als Trainingsbeispiel zu einer Identität in CompreFace hinzu
func (h *APIHandler) TrainIdentityWithImage(c *gin.Context) {
	id := c.Param("id")

	// Identität abrufen
	var identity models.Identity
	if err := h.db.First(&identity, id).Error; err != nil {
		log.WithError(err).Errorf("Identität mit ID %s nicht gefunden", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "Identität nicht gefunden"})
		return
	}

	// Prüfen, ob CompreFace aktiviert ist
	if !h.cfg.CompreFace.Enabled {
		log.Error("CompreFace ist nicht aktiviert, Trainingsbilder können nicht hinzugefügt werden")
		c.JSON(http.StatusPreconditionFailed, gin.H{"error": "CompreFace ist nicht aktiviert"})
		return
	}

	// Datei aus Anfrage erhalten
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		log.WithError(err).Error("Kein Bild hochgeladen oder ungültiges Formular")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kein Bild hochgeladen oder ungültiges Formular"})
		return
	}
	defer file.Close()

	// Bilddaten lesen
	imageData, err := io.ReadAll(file)
	if err != nil {
		log.WithError(err).Error("Fehler beim Lesen der Bilddaten")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Verarbeiten des Bildes"})
		return
	}

	// Debugging-Informationen ausgeben
	log.Infof("Füge Trainingsbild für Identität '%s' (%d) mit Datei '%s' (Größe: %d Bytes) hinzu", 
		identity.Name, identity.ID, header.Filename, len(imageData))

	// An CompreFace senden
	ctx := c.Request.Context()
	
	// Sicherstellen, dass das Subjekt existiert
	_, err = h.compreface.CreateSubject(ctx, identity.Name)
	if err != nil {
		log.WithError(err).Warnf("Fehler beim Erstellen des Subjekts in CompreFace (existiert möglicherweise bereits)")
		// Wir fahren trotzdem fort, da das Subjekt möglicherweise bereits existiert
	}

	// Bild als Beispiel zu CompreFace hinzufügen
	result, err := h.compreface.AddSubjectExample(ctx, identity.Name, imageData, header.Filename)
	if err != nil {
		log.WithError(err).Error("Fehler beim Hinzufügen des Trainingsbilds zu CompreFace")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Hinzufügen des Trainingsbilds zu CompreFace"})
		return
	}

	log.Infof("Trainingsbild erfolgreich zu Identität '%s' hinzugefügt (CompreFace-ID: %s)", identity.Name, result.ImageID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Trainingsbild erfolgreich zu Identität '%s' hinzugefügt", identity.Name),
		"identity": identity,
		"compreface_image_id": result.ImageID,
	})
}
