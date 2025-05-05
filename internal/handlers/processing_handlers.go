package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/models"
	"double-take-go-reborn/internal/services"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"double-take-go-reborn/internal/mqtt"
)

// ProcessingHandler handles endpoints related to image processing status and potentially triggering processing.
type ProcessingHandler struct {
	Cfg        *config.Config
	DB         *gorm.DB
	CompreFace *services.CompreFaceService
	Notifier   *services.NotifierService
	MQTT       *mqtt.Client
}

// NewProcessingHandler creates a new handler for processing endpoints.
func NewProcessingHandler(cfg *config.Config, db *gorm.DB, comprefaceService *services.CompreFaceService, notifier *services.NotifierService, mqttClient *mqtt.Client) *ProcessingHandler {
	return &ProcessingHandler{
		Cfg:        cfg,
		DB:         db,
		CompreFace: comprefaceService,
		Notifier:   notifier,
		MQTT:       mqttClient,
	}
}

// ProcessCompreFace handles image uploads, sends them to CompreFace, and stores results.
// @Summary Process image with CompreFace
// @Description Upload an image file to be processed by the configured CompreFace service.
// @Tags Processing
// @Accept multipart/form-data
// @Param file formData file true "Image file to process"
// @Produce json
// @Success 200 {object} map[string]interface{} "Processing results (e.g., matches found)"
// @Failure 400 {object} map[string]string "Bad Request (e.g., no file, bad file)"
// @Failure 500 {object} map[string]string "Internal Server Error (e.g., CompreFace error, DB error)"
// @Router /api/process/compreface [post]
func (h *ProcessingHandler) ProcessCompreFace(c *gin.Context) {
	if !h.Cfg.CompreFace.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CompreFace processing is not enabled in the configuration."})
		return
	}

	// --- 1. Get Image Upload --- 
	fileHeader, err := c.FormFile("file") // "file" must match the name attribute in the form
	if err != nil {
		log.Errorf("Failed to get form file: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get image file from form data. Make sure the field name is 'file'."})
		return
	}

	log.Infof("Received file upload: %s (Size: %d bytes)", fileHeader.Filename, fileHeader.Size)

	// Open the uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		log.Errorf("Failed to open uploaded file '%s': %v", fileHeader.Filename, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file."})
		return
	}
	defer file.Close()

	// Read the file content into memory
	imageBytes, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Failed to read uploaded file '%s': %v", fileHeader.Filename, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file content."})
		return
	}

	// --- 2. Call CompreFace Service --- 
	recognitionResult, err := h.CompreFace.Recognize(imageBytes, fileHeader.Filename)
	if err != nil {
		log.Errorf("CompreFace recognition failed for '%s': %v", fileHeader.Filename, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("CompreFace processing failed: %v", err)})
		return
	}

	if recognitionResult == nil || len(recognitionResult.Result) == 0 {
		log.Infof("CompreFace found no faces or matches in '%s'", fileHeader.Filename)
		c.JSON(http.StatusOK, gin.H{"message": "No faces or matches found by CompreFace.", "compreface_result": recognitionResult})
		return
	}

	log.Infof("CompreFace processed '%s', found %d potential face(s).", fileHeader.Filename, len(recognitionResult.Result))

	// --- 3. Store Results in Database (Simplified Example) --- 
	// In a real scenario, we'd likely store the image file somewhere persistent first.
	// For now, we'll just record the processing event and matches.

	// Create Image record (using filename as a temporary path identifier)
	imgRecord := models.Image{
		FilePath:  fileHeader.Filename, // Placeholder, use actual path if stored
		Timestamp: time.Now(),          // Or use EXIF data if available
	}
	if err := h.DB.Create(&imgRecord).Error; err != nil {
		log.Errorf("Failed to create image record for '%s': %v", fileHeader.Filename, err)
		// Continue processing faces/matches even if image record fails?
	} else {
		log.Debugf("Created image record ID %d for %s", imgRecord.ID, imgRecord.FilePath)
	}

	matchesFound := 0
	// Process each detected face result from CompreFace
	for _, faceResult := range recognitionResult.Result {
		// Create Face record
		faceRecord := models.Face{
			ImageID:  imgRecord.ID, // Link to the image record (might be 0 if image save failed)
			Detector: "compreface",
			Box:      datatypes.JSON(fmt.Sprintf(`{"x_min": %d, "y_min": %d, "x_max": %d, "y_max": %d, "probability": %.4f}`, 
											faceResult.Box.XMin, faceResult.Box.YMin, faceResult.Box.XMax, faceResult.Box.YMax, faceResult.Box.Probability)),
		}
		if err := h.DB.Create(&faceRecord).Error; err != nil {
			log.Errorf("Failed to create face record for image %d: %v", imgRecord.ID, err)
			continue // Skip matches for this face if save failed
		}

		// Process subjects (matches) for this face
		for _, subject := range faceResult.Subjects {
			// Find or Create Identity record
			var identityRecord models.Identity
			// Use FirstOrCreate to avoid duplicates based on name
			if err := h.DB.Where(models.Identity{Name: subject.Subject}).FirstOrCreate(&identityRecord).Error; err != nil {
				log.Errorf("Failed to find/create identity '%s': %v", subject.Subject, err)
				continue // Skip this match if identity handling fails
			}

			// Create Match record
			matchRecord := models.Match{
				FaceID:     faceRecord.ID,
				IdentityID: identityRecord.ID,
				Confidence: subject.Similarity,
				Timestamp:  time.Now(), // Or align with image timestamp
				Detector:   "compreface",
			}
			if err := h.DB.Create(&matchRecord).Error; err != nil {
				log.Errorf("Failed to create match record for face %d and identity %d ('%s'): %v", faceRecord.ID, identityRecord.ID, identityRecord.Name, err)
			} else {
				matchesFound++
				log.Debugf("Stored match: Face %d -> Identity %d ('%s') with confidence %.4f", faceRecord.ID, identityRecord.ID, identityRecord.Name, subject.Similarity)
			}
		}
	}

	log.Infof("Stored %d matches from CompreFace result for '%s'", matchesFound, fileHeader.Filename)

	c.JSON(http.StatusOK, gin.H{
		"message":         fmt.Sprintf("Successfully processed '%s' with CompreFace.", fileHeader.Filename),
		"faces_detected":  len(recognitionResult.Result),
		"matches_stored":  matchesFound,
		"compreface_result": recognitionResult, // Include raw result for debugging
	})
}
