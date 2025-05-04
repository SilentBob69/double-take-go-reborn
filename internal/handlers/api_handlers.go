package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/database"
	"double-take-go-reborn/internal/models"
	"double-take-go-reborn/internal/services"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// APIHandler holds dependencies for API endpoints.
type APIHandler struct {
	Cfg        *config.Config
	CompreFace *services.CompreFaceService
	Notifier   *services.NotifierService
	DB         *gorm.DB
}

// NewAPIHandler creates a new API handler with dependencies.
func NewAPIHandler(db *gorm.DB, cfg *config.Config, comprefaceService *services.CompreFaceService, notifier *services.NotifierService) *APIHandler {
	return &APIHandler{
		DB:         db,
		Cfg:        cfg,
		CompreFace: comprefaceService,
		Notifier:   notifier,
	}
}

// RegisterRoutes sets up the API endpoints using chi.Router
func (h *APIHandler) RegisterRoutes(r chi.Router) {
	r.Post("/images/{imageID}/recognize", h.handleRecognizeImage)
	r.Delete("/images/{id}", h.handleImageDelete)       // Added delete route
	r.Delete("/images/all", h.handleDeleteAllImages) // Added delete all route
	r.Post("/images/{id}/recognize", h.handleImageRecognize)
	r.Get("/identities", h.handleGetIdentities) // NEW: Get all identities

	// Routes for assigning/correcting identities
	r.Put("/matches/{matchID}/identity", h.handleUpdateMatchIdentity) // NEW: Update existing match identity
	r.Post("/matches", h.handleCreateMatch)                         // NEW: Create a new match for a face

	// Add other API routes here
}

// handleRecognizeImage triggers CompreFace recognition for an existing image.
func (h *APIHandler) handleRecognizeImage(w http.ResponseWriter, r *http.Request) {
	imageIDStr := chi.URLParam(r, "imageID")
	imageID, err := strconv.ParseInt(imageIDStr, 10, 64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid image ID format")
		return
	}

	log.Infof("Received request to re-recognize image ID: %d", imageID)

	// 1. Fetch Image from Database
	img, err := database.GetImageByID(imageID)
	if err != nil {
		log.Errorf("Error fetching image %d from database: %v", imageID, err)
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch image data")
		return
	}
	if img == nil {
		log.Warnf("Image with ID %d not found", imageID)
		respondWithError(w, http.StatusNotFound, "Image not found")
		return
	}

	log.Debugf("Found image: %s", img.Filename)

	// 2. Read Image File from Storage
	imagePath := filepath.Join(h.Cfg.Server.SnapshotDir, img.Filename)
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		log.Errorf("Error reading image file '%s': %v", imagePath, err)
		respondWithError(w, http.StatusInternalServerError, "Failed to read image file")
		return
	}

	log.Debugf("Read %d bytes from image file %s", len(imageBytes), img.Filename)

	// 3. Call CompreFace Service
	compreResp, err := h.CompreFace.Recognize(imageBytes, img.Filename)
	if err != nil {
		log.Errorf("Error calling CompreFace service for image %d: %v", imageID, err)
		respondWithError(w, http.StatusInternalServerError, "Failed to recognize faces")
		return
	}

	// 4. Start Database Transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		log.Errorf("Failed to begin database transaction: %v", tx.Error)
		respondWithError(w, http.StatusInternalServerError, "Database transaction error")
		return
	}
	// Defer rollback in case of errors, commit at the end
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic occurred during transaction: %v", r)
			tx.Rollback()
		} else if err != nil {
			log.Warnf("Rolling back transaction due to error: %v", err)
			tx.Rollback()
		}
	}()

	// 5. Delete Old Recognition Records within Transaction
	err = tx.Where("image_id = ?", imageID).Delete(&database.Recognition{}).Error
	if err != nil {
		log.Errorf("Failed to delete old recognition records for image %d: %v", imageID, err)
		// Don't http.Error here, let defer handle rollback
		return
	}

	// 6. Insert New Recognition Records within Transaction
	var newRecognitions []database.Recognition
	for _, result := range compreResp.Result {
		for _, face := range result.Subjects {
			// Find IdentityID (handle potential errors)
			var identity database.Identity
			var identityID int64 = 0 // Default to 0 (unknown/not found)
			if face.Subject != "" && face.Subject != "unknown" { // Only lookup known names
				idResult := h.DB.Where("name = ?", face.Subject).First(&identity)
				if idResult.Error == nil {
					identityID = int64(identity.ID)
				} else if !errors.Is(idResult.Error, gorm.ErrRecordNotFound) {
					// Log error if it's not just 'not found'
					log.Warnf("Error looking up identity '%s': %v", face.Subject, idResult.Error)
				}
			}

			rec := database.Recognition{
				ImageID:    imageID, // Keep as int64
				IdentityID: identityID, // Keep as int64
				Subject:    face.Subject,
				Confidence: face.Similarity, // Map Similarity to Confidence
				BoxX:       result.Box.XMin, // Map to BoxX
				BoxY:       result.Box.YMin, // Map to BoxY
				BoxWidth:   result.Box.XMax - result.Box.XMin, // Calculate BoxWidth
				BoxHeight:  result.Box.YMax - result.Box.YMin, // Calculate BoxHeight
			}
			newRecognitions = append(newRecognitions, rec)
			err = tx.Create(&rec).Error
			if err != nil {
				log.Errorf("Failed to insert new recognition record for image %d: %v", imageID, err)
				// Let defer handle rollback
				return
			}
		}
	}

	// 7. Commit Transaction
	err = tx.Commit().Error
	if err != nil {
		log.Errorf("Failed to commit database transaction for image %d: %v", imageID, err)
		respondWithError(w, http.StatusInternalServerError, "Database commit error")
		return
	}

	log.Infof("Successfully re-recognized image %d and stored %d results.", imageID, len(newRecognitions))

	// 8. Respond to Client
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"message":      fmt.Sprintf("Image %d re-recognized successfully.", imageID),
		"recognitions": newRecognitions,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Failed to encode response: %v", err)
		// Don't try to write headers again if encode fails
	}
}

// handleImageDelete handles DELETE requests to /api/images/{id}
func (h *APIHandler) handleImageDelete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id") // Use chi URL param
	imageID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Image ID")
		return
	}

	log.Infof("Received request to delete image ID: %d", imageID)

	// Use a transaction to ensure atomicity
	tx := h.DB.Begin() // Use global DB
	if tx.Error != nil {
		log.Errorf("Failed to start transaction for image deletion: %v", tx.Error)
		respondWithError(w, http.StatusInternalServerError, "Database transaction error")
		return
	}

	// Defer rollback in case of error
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic occurred during delete transaction: %v", r)
			tx.Rollback()
			respondWithError(w, http.StatusInternalServerError, "Internal Server Error during delete")
		} else if err != nil { // Check outer err variable
			log.Warnf("Rolling back delete transaction due to error: %v", err)
			tx.Rollback()
			// Error response already sent by the code causing the error
		}
	}()

	// 1. Find the image record to get the file path
	var img database.Image // Use database package
	err = tx.First(&img, imageID).Error // Assign to outer err
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warnf("Image not found for deletion: %d", imageID)
			respondWithError(w, http.StatusNotFound, "Image not found")
			return
		}
		log.Errorf("Failed to find image %d for deletion: %v", imageID, err)
		respondWithError(w, http.StatusInternalServerError, "Database error finding image")
		return
	}

	// 2. Delete associated Recognitions (GORM should handle this via constraints if set up,
	// but explicit deletion is safer if constraints aren't perfect or don't exist)
	err = tx.Where("image_id = ?", imageID).Delete(&database.Recognition{}).Error // Assign to outer err
	if err != nil {
		log.Errorf("Failed to delete recognitions for image %d: %v", imageID, err)
		respondWithError(w, http.StatusInternalServerError, "Database error deleting recognitions")
		return
	}

	// NOTE: Assuming Faces/Matches are not separate tables anymore based on previous DB schema changes.
	// If they ARE separate, uncomment and adapt the deletion logic from web_handlers.go
	/*
		// Find face IDs associated with the image
		var faceIDs []uint
		if err := tx.Model(&database.Face{}).Where("image_id = ?", imageID).Pluck("id", &faceIDs).Error; err != nil {
		    log.Error("Failed to find faces for image during deletion", "image_id", imageID, "error", err)
	        respondWithError(w, http.StatusInternalServerError, "Database error finding faces")
	        return // Let defer handle rollback
	    }
	    // Delete matches associated with these faces
	    if len(faceIDs) > 0 {
		    if err := tx.Where("face_id IN ?", faceIDs).Delete(&database.Match{}).Error; err != nil {
		        log.Error("Failed to delete matches for image", "image_id", imageID, "error", err)
		        respondWithError(w, http.StatusInternalServerError, "Database error deleting matches")
		        return // Let defer handle rollback
		    }
	    }
		// Delete associated Faces
		if err := tx.Where("image_id = ?", imageID).Delete(&database.Face{}).Error; err != nil {
			log.Error("Failed to delete faces for image", "image_id", imageID, "error", err)
			respondWithError(w, http.StatusInternalServerError, "Database error deleting faces")
			return // Let defer handle rollback
		}
	*/

	// 3. Delete the Image record itself
	err = tx.Delete(&img).Error // Assign to outer err
	if err != nil {
		log.Errorf("Failed to delete image record %d: %v", imageID, err)
		respondWithError(w, http.StatusInternalServerError, "Database error deleting image")
		return
	}

	// 4. Delete the actual snapshot file
	if img.Filename != "" { // Check if filename exists
		snapshotPath := filepath.Join(h.Cfg.Server.SnapshotDir, img.Filename) // Use config for path
		if err := os.Remove(snapshotPath); err != nil {
			if !errors.Is(err, os.ErrNotExist) { // Don't rollback for file not found
				// Log error but commit transaction anyway, as DB is cleaned
				log.Errorf("Failed to delete snapshot file '%s' for image %d: %v", snapshotPath, imageID, err)
				// Do not return error here, let transaction commit
				err = nil // Clear error so defer doesn't rollback
			} else {
				log.Warnf("Snapshot file '%s' not found for image %d, proceeding with DB commit", snapshotPath, imageID)
				err = nil // Clear error so defer doesn't rollback
			}
		} else {
			log.Infof("Successfully deleted snapshot file '%s' for image %d", snapshotPath, imageID)
		}
	} else {
		log.Warnf("Image record %d has empty filename, skipping file deletion.", imageID)
	}

	// Commit the transaction
	if err = tx.Commit().Error; err != nil { // Assign to outer err
		log.Errorf("Failed to commit transaction for image deletion %d: %v", imageID, err)
		respondWithError(w, http.StatusInternalServerError, "Database commit error")
		return // Error response already sent
	}

	log.Infof("Successfully deleted image %d and associated data", imageID)
	respondWithJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("Image %d deleted successfully", imageID)})
}

// handleDeleteAllImages deletes all images and recognition data
func (h *APIHandler) handleDeleteAllImages(w http.ResponseWriter, r *http.Request) {
	log.Warn("Received request to delete ALL images and recognitions.")

	var images []database.Image // Use database package
	var err error              // Declare error variable for deferred rollback check

	// 1. Find all Image records to get file paths
	if err = h.DB.Find(&images).Error; err != nil {
		log.Errorf("Failed to query all images from database: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error querying images")
		return
	}

	if len(images) == 0 {
		log.Info("No images found in the database to delete.")
		respondWithJSON(w, http.StatusOK, map[string]string{"message": "No images found to delete."}) // Use JSON response
		return
	}

	log.Infof("Found %d images to delete", len(images))

	// 2. Delete the actual snapshot files (best effort)
	deletedFiles := 0
	failedFiles := 0
	for _, img := range images {
		if img.Filename == "" {
			log.Warnf("Image ID %d has empty filename, skipping file deletion", img.ID)
			continue
		}
		snapshotPath := filepath.Join(h.Cfg.Server.SnapshotDir, img.Filename) // Use config path
		if errDel := os.Remove(snapshotPath); errDel != nil {
			if !errors.Is(errDel, os.ErrNotExist) {
				log.Warnf("Failed to delete snapshot file '%s' for image %d: %v", snapshotPath, img.ID, errDel)
				failedFiles++
			} else {
				log.Debugf("Snapshot file '%s' already deleted or never existed for image %d", snapshotPath, img.ID)
			}
		} else {
			log.Debugf("Successfully deleted snapshot file '%s' for image %d", snapshotPath, img.ID)
			deletedFiles++
		}
	}
	log.Infof("File deletion attempt summary: successful=%d, failed=%d, skipped/not_found=%d", deletedFiles, failedFiles, len(images)-deletedFiles-failedFiles)

	// 3. Delete all Image and Recognition records in a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		log.Errorf("Failed to start database transaction for deleting all images: %v", tx.Error)
		respondWithError(w, http.StatusInternalServerError, "Database transaction error")
		return
	}
	// Defer rollback
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic occurred during delete all transaction: %v", r)
			tx.Rollback()
		} else if err != nil { // Check outer err variable
			log.Warnf("Rolling back delete all transaction due to error: %v", err)
			tx.Rollback()
		}
	}()

	// Delete all Recognitions
	if err = tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&database.Recognition{}).Error; err != nil {
		log.Errorf("Failed to delete all recognition records: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error deleting recognitions")
		return // defer will rollback
	}

	// Delete all Images
	if err = tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&database.Image{}).Error; err != nil {
		log.Errorf("Failed to delete all image records: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error deleting images")
		return // defer will rollback
	}

	// Commit the transaction
	if err = tx.Commit().Error; err != nil {
		log.Errorf("Failed to commit transaction for deleting all images: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database commit error")
		return
	}

	log.Warnf("Successfully deleted %d images and all recognition records.", len(images))
	respondWithJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("Successfully deleted %d images and all recognition records.", len(images))})
}

// handleImageRecognize triggers re-recognition of a specific image
func (h *APIHandler) handleImageRecognize(w http.ResponseWriter, r *http.Request) {
	imageIDStr := chi.URLParam(r, "id")
	imageID, err := strconv.ParseUint(imageIDStr, 10, 32)
	if err != nil {
		log.Warn("Invalid image ID received for re-recognition", "id", imageIDStr, "error", err)
		http.Error(w, "Invalid image ID", http.StatusBadRequest)
		return
	}

	log.Infof("Received request to re-recognize image ID: %d", imageID)

	// 1. Fetch image record from DB to get file path
	var image models.Image // Use models.Image
	if err := h.DB.First(&image, uint(imageID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Warn("Image not found for re-recognition", "id", imageID)
			http.Error(w, "Image not found", http.StatusNotFound)
		} else {
			log.Error("Database error fetching image for re-recognition", "id", imageID, "error", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	if image.FilePath == "" {
		log.Error("Image record has empty FilePath, cannot re-recognize", "id", imageID)
		http.Error(w, "Image record is incomplete", http.StatusInternalServerError)
		return
	}

	// 2. Construct full file path and read image data
	fullPath := filepath.Join(h.Cfg.Server.SnapshotDir, image.FilePath)
	imageData, err := os.ReadFile(fullPath)
	if err != nil {
		log.Error("Failed to read image file for re-recognition", "id", imageID, "path", fullPath, "error", err)
		http.Error(w, "Failed to read image file", http.StatusInternalServerError)
		return
	}

	log.Debugf("Read %d bytes for image %s", len(imageData), image.FilePath)

	// 3. Call CompreFace service
	log.Debugf("Sending image %d (%s) to CompreFace for re-recognition", imageID, image.FilePath)
	compreFaceResult, err := h.CompreFace.Recognize(imageData, image.FilePath) // Pass filename for context
	if err != nil {
		// Log the error from CompreFace, but proceed anyway
		log.Error("CompreFace re-recognition failed", "id", imageID, "file", image.FilePath, "error", err)
		// We might still want to clear old results even if new recognition fails
	}

	if compreFaceResult != nil {
		log.Infof("CompreFace re-recognition successful for image %d, found %d results.", imageID, len(compreFaceResult.Result))
	} else {
		log.Info("CompreFace re-recognition did not return results (or failed) for image %d", imageID)
	}

	// 4. Delete old recognition results (Faces and Matches) for this image ID
	// Use unscoped delete to ensure soft-deleted records are not accidentally kept
	tx := h.DB.Begin()
	if tx.Error != nil {
		log.Error("Failed to start transaction for deleting old recognition data", "id", imageID, "error", tx.Error)
		http.Error(w, "Database transaction error", http.StatusInternalServerError)
		return
	}

	// Delete Matches first due to foreign key constraints
	if err := tx.Unscoped().Where("face_id IN (SELECT id FROM faces WHERE image_id = ?)", imageID).Delete(&models.Match{}).Error; err != nil { // Use models.Match
		tx.Rollback()
		log.Error("Failed to delete old matches for image", "id", imageID, "error", err)
		http.Error(w, "Failed to delete old recognition data", http.StatusInternalServerError)
		return
	}
	// Delete Faces
	if err := tx.Unscoped().Where("image_id = ?", imageID).Delete(&models.Face{}).Error; err != nil { // Use models.Face
		tx.Rollback()
		log.Error("Failed to delete old faces for image", "id", imageID, "error", err)
		http.Error(w, "Failed to delete old recognition data", http.StatusInternalServerError)
		return
	}

	log.Debugf("Successfully deleted old Faces and Matches for image ID: %d", imageID)

	// 5. Store new recognition results (if any)
	if compreFaceResult != nil && len(compreFaceResult.Result) > 0 {
		log.Debugf("Storing %d new recognition results for image ID: %d", len(compreFaceResult.Result), imageID)
		for _, result := range compreFaceResult.Result {
			// Marshal the bounding box from CompreFace result into JSON
			boxData, err := json.Marshal(result.Box) // Marshal the Box struct from the result
			if err != nil {
				tx.Rollback()
				log.Error("Failed to marshal bounding box to JSON", "image_id", imageID, "error", err)
				http.Error(w, "Failed to process recognition data", http.StatusInternalServerError)
				return
			}

			face := models.Face{ // Use models.Face
				ImageID: image.ID, // Link to the existing image
				Box:     datatypes.JSON(boxData), // Store marshaled JSON
				// Detector field could be set here if needed, e.g., Detector: "compreface"
			}

			if err := tx.Create(&face).Error; err != nil {
				tx.Rollback()
				log.Error("Failed to create Face record during re-recognition", "image_id", imageID, "error", err)
				http.Error(w, "Failed to store new recognition data", http.StatusInternalServerError)
				return
			}

			for _, subject := range result.Subjects {
				// Find or create identity
				identity := models.Identity{Name: subject.Subject}
				if err := tx.Where("name = ?", subject.Subject).FirstOrCreate(&identity).Error; err != nil {
					tx.Rollback()
					log.Error("Failed to find or create Identity record during re-recognition", "subject", subject.Subject, "error", err)
					http.Error(w, "Failed to process identity data", http.StatusInternalServerError)
					return
				}

				match := models.Match{ // Use models.Match
					FaceID:     face.ID,
					IdentityID: identity.ID, // Correct: Use the ID of the found/created identity
					Confidence: subject.Similarity * 100, // Convert to percentage
				}

				if err := tx.Create(&match).Error; err != nil {
					tx.Rollback()
					log.Error("Failed to create Match record during re-recognition", "face_id", face.ID, "identity_id", identity.ID, "error", err) // Correct: Log identity.ID
					http.Error(w, "Failed to store new match data", http.StatusInternalServerError)
					return
				}
			}
		}
		log.Infof("Successfully stored new recognition results for image ID: %d", imageID)
	} else {
		log.Info("No new recognition results to store for image ID: %d", imageID)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Error("Failed to commit transaction for re-recognition", "id", imageID, "error", err)
		http.Error(w, "Database commit error", http.StatusInternalServerError)
		return
	}

	// 6. Respond (Consider sending back updated matches? For now, just 204)
	log.Infof("Re-recognition process completed for image ID: %d", imageID)
	w.WriteHeader(http.StatusNoContent) // Indicate success, no content to return directly
	// Optionally: Trigger an SSE broadcast here to update the specific card in the UI
}

// handleGetIdentities fetches all known identities from the database.
func (h *APIHandler) handleGetIdentities(w http.ResponseWriter, r *http.Request) {
	var identities []models.Identity
	if err := h.DB.Order("name asc").Find(&identities).Error; err != nil {
		log.WithError(err).Error("Failed to fetch identities from database")
		respondWithError(w, http.StatusInternalServerError, "Error fetching identities")
		return
	}

	log.Debugf("Fetched %d identities", len(identities))
	respondWithJSON(w, http.StatusOK, identities)
}

// --- Handlers for Identity Assignment ---

type UpdateMatchIdentityRequest struct {
	IdentityID uint `json:"identity_id"` // Use 0 to unassign
}

// handleUpdateMatchIdentity updates the identity associated with an existing match.
func (h *APIHandler) handleUpdateMatchIdentity(w http.ResponseWriter, r *http.Request) {
	matchIDStr := chi.URLParam(r, "matchID")
	matchID, err := strconv.ParseUint(matchIDStr, 10, 64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid match ID format")
		return
	}

	var req UpdateMatchIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Infof("Attempting to update match %d with identity ID %d", matchID, req.IdentityID)

	// Find the match
	var match models.Match
	if err := h.DB.First(&match, uint(matchID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondWithError(w, http.StatusNotFound, "Match not found")
		} else {
			log.WithError(err).Errorf("Failed to find match %d", matchID)
			respondWithError(w, http.StatusInternalServerError, "Error finding match")
		}
		return
	}

	// Update the IdentityID or delete the match
	if req.IdentityID == 0 {
		// If IdentityID is 0, unassign by deleting the match
		log.Infof("Unassigning identity by deleting match %d", matchID)
		if err := h.DB.Delete(&match).Error; err != nil {
			log.WithError(err).Errorf("Failed to delete match %d for unassignment", matchID)
			respondWithError(w, http.StatusInternalServerError, "Error unassigning identity")
			return
		}
		log.Infof("Successfully deleted match %d for unassignment.", matchID)
		respondWithJSON(w, http.StatusOK, map[string]string{"message": "Match unassigned successfully by deletion"})

	} else {
		// If IdentityID is non-zero, update the field
		log.Infof("Assigning identity %d to match %d", req.IdentityID, matchID)
		if err := h.DB.Model(&match).Update("IdentityID", req.IdentityID).Error; err != nil {
			log.WithError(err).Errorf("Failed to update match %d identity", matchID)
			respondWithError(w, http.StatusInternalServerError, "Error updating match identity")
			return
		}

		// Fetch the updated match to return, preloading necessary data
		var updatedMatch models.Match
		if err := h.DB.Preload("Identity").First(&updatedMatch, match.ID).Error; err != nil {
		    log.WithError(err).Errorf("Failed to fetch updated match %d", match.ID)
		    // Non-critical error, proceed without returning the updated object
		    respondWithJSON(w, http.StatusOK, map[string]string{"message": "Match identity updated successfully"})
		    return
	    }

		log.Infof("Successfully updated match %d. New Identity: %v", matchID, updatedMatch.Identity)
		// TODO: Consider SSE broadcast here
		respondWithJSON(w, http.StatusOK, updatedMatch)
	}
}

type CreateMatchRequest struct {
	FaceID     uint `json:"face_id"`
	IdentityID uint `json:"identity_id"`
}

// handleCreateMatch creates a new match record linking a face to an identity.
func (h *APIHandler) handleCreateMatch(w http.ResponseWriter, r *http.Request) {
	var req CreateMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.FaceID == 0 || req.IdentityID == 0 {
		respondWithError(w, http.StatusBadRequest, "Face ID and Identity ID are required and cannot be 0")
		return
	}

	log.Infof("Attempting to create a new match for face %d with identity %d", req.FaceID, req.IdentityID)

	// Check if face and identity exist (optional but good practice)
	var face models.Face
    if err := h.DB.First(&face, req.FaceID).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            respondWithError(w, http.StatusNotFound, "Face not found")
        } else {
            log.WithError(err).Errorf("Error checking face %d", req.FaceID)
            respondWithError(w, http.StatusInternalServerError, "Error checking face")
        }
        return
    }
    var identity models.Identity
    if err := h.DB.First(&identity, req.IdentityID).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            respondWithError(w, http.StatusNotFound, "Identity not found")
        } else {
            log.WithError(err).Errorf("Error checking identity %d", req.IdentityID)
            respondWithError(w, http.StatusInternalServerError, "Error checking identity")
        }
        return
    }

	// Create the new match
	// Setting Confidence to 100.0 for manual assignments
	newMatch := models.Match{
		FaceID:     req.FaceID,
		IdentityID: req.IdentityID, // Assign directly, assuming IdentityID is uint
		Confidence: 100.0,        // Manual assignment confidence
	}

	if err := h.DB.Create(&newMatch).Error; err != nil {
		// Handle potential constraint violations (e.g., if a match already exists for this face?)
		log.WithError(err).Errorf("Failed to create match for face %d and identity %d", req.FaceID, req.IdentityID)
		respondWithError(w, http.StatusInternalServerError, "Error creating match")
		return
	}

    // Fetch the created match to return, preloading necessary data
    var createdMatch models.Match
    if err := h.DB.Preload("Identity").Preload("Face").First(&createdMatch, newMatch.ID).Error; err != nil {
        log.WithError(err).Errorf("Failed to fetch created match %d", newMatch.ID)
        // Non-critical error, proceed without returning the created object
        respondWithJSON(w, http.StatusCreated, map[string]string{"message": "Match created successfully"})
        return
    }

	log.Infof("Successfully created match %d for face %d with identity %d", createdMatch.ID, req.FaceID, req.IdentityID)
	// TODO: Consider SSE broadcast here
	respondWithJSON(w, http.StatusCreated, createdMatch)
}

// Helper function to send JSON errors
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// Helper function to send JSON responses
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Errorf("Failed to marshal JSON response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("{\"error\": \"Internal Server Error\"}")) // Fallback error
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write(response); err != nil {
		log.Errorf("Failed to write JSON response: %v", err)
	}
}

// Helper function (consider moving to a utility package)
func stringContains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
