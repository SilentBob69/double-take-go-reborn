package processing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/models"
	"double-take-go-reborn/internal/services"
	"double-take-go-reborn/internal/sse"

	log "github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// FrigateEvent represents the structure of a Frigate MQTT event message.
// Simplified structure based on common usage - may need adjustment based on your Frigate version/config.
type FrigateEvent struct {
	Type   string         `json:"type"` // e.g., "new", "update", "end"
	Before FrigateEventData `json:"before"`
	After  FrigateEventData `json:"after"`
}

type FrigateEventData struct {
	ID          string    `json:"id"`
	Camera      string    `json:"camera"`
	Label       string    `json:"label"` // e.g., "person"
	HasSnapshot bool      `json:"has_snapshot"`
	StartTime   float64   `json:"start_time"`
	EndTime     *float64  `json:"end_time,omitempty"`
	TopScore    float64   `json:"top_score"`
	Stationary  bool      `json:"stationary"`
	// Add other fields as needed
}

// Processor handles the core logic of processing events and images.
type Processor struct {
	Cfg           *config.Config
	DB            *gorm.DB
	HttpClient    *http.Client
	compreService *services.CompreFaceService
	sseHub        *sse.Hub // Add SSE Hub
}

// NewProcessor creates a new processor instance.
func NewProcessor(cfg *config.Config, db *gorm.DB, sseHub *sse.Hub) *Processor {
	httpClient := &http.Client{
		Timeout: time.Second * 30, // Timeout for fetching images etc.
	}

	var cs *services.CompreFaceService
	if cfg.CompreFace.Enabled {
		cs = services.NewCompreFaceService(cfg.CompreFace)
	}

	return &Processor{
		Cfg:           cfg,
		DB:            db,
		HttpClient:    httpClient,
		compreService: cs,
		sseHub:        sseHub, // Store the SSE Hub
	}
}

// ProcessFrigateEvent parses a Frigate event payload and triggers processing.
func (p *Processor) ProcessFrigateEvent(payload []byte) {
	var event FrigateEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Warnf("Failed to decode Frigate MQTT event JSON: %v. Payload: %s", err, string(payload))
		return
	}

	// --- Decide whether to process this event --- 
	// Example: Process 'new', 'update', or 'end' events for 'person' labels that have snapshots.
	isRelevantType := event.Type == "new" || event.Type == "update" || event.Type == "end"
	isPerson := event.After.Label == "person"
	hasSnapshot := event.After.HasSnapshot

	if !(isRelevantType && isPerson && hasSnapshot) {
		log.Debugf("Skipping Frigate event type '%s' for label '%s' (Snapshot: %t)", event.Type, event.After.Label, hasSnapshot)
		return
	}

	log.Infof("Processing Frigate event '%s' (Type: %s) for a '%s' on camera '%s'", event.After.ID, event.Type, event.After.Label, event.After.Camera)

	// --- Fetch Snapshot --- 
	log.Debugf("Attempting to fetch snapshot for event '%s', type '%s'", event.After.ID, event.Type)
	imageBytes, filename, err := p.fetchFrigateSnapshot(event.After.ID, event.Type) // Pass event.Type
	if err != nil {
		log.Errorf("Failed to fetch snapshot for Frigate event %s: %v", event.After.ID, err)
		return
	}
	log.Debugf("Successfully fetched snapshot for event '%s', filename: %s", event.After.ID, filename)

	// --- Calculate Content Hash ---
	hasher := sha256.New()
	hasher.Write(imageBytes)
	contentHash := hex.EncodeToString(hasher.Sum(nil))
	log.Debugf("Calculated content hash for event %s: %s", event.After.ID, contentHash)

	// --- Check for Duplicates --- 
	var existingImage models.Image
	// Query for an existing image with the same FrigateEventID AND ContentHash
	result := p.DB.Where("frigate_event_id = ? AND content_hash = ?", event.After.ID, contentHash).First(&existingImage)
	if result.Error == nil {
		// Duplicate found!
		log.Infof("Duplicate image detected for event %s (Hash: %s). Skipping further processing.", event.After.ID, contentHash)
		// Optional: Consider deleting the newly downloaded file if it won't be used?
		// snapshotPath := filepath.Join("/data/snapshots", filename)
		// os.Remove(snapshotPath) // Best effort removal
		return
	} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// An actual error occurred during the DB query
		log.Errorf("Database error checking for duplicate image for event %s: %v", event.After.ID, result.Error)
		return // Stop processing on DB error
	}
	// If we reach here, it means err == gorm.ErrRecordNotFound, so no duplicate found.
	log.Debugf("No duplicate found for event %s with hash %s. Proceeding.", event.After.ID, contentHash)

	// --- Process Image (using CompreFace for now) --- 
	if p.compreService == nil {
		log.Warnf("Cannot process image for event %s: CompreFace service is not enabled/initialized.", event.After.ID)
		// Store the image anyway without recognition?
		log.Warnf("Proceeding to store image record for '%s' without CompreFace processing.", filename)
		// Pass event ID and hash even when CompreFace is skipped
		p.storeRecognitionResults(event.After.ID, filename, contentHash, time.Now(), nil)
		return
	}

	log.Debugf("Attempting CompreFace recognition for image %s (event %s)", filename, event.After.ID)
	recognitionResult, err := p.compreService.Recognize(imageBytes, filename)

	// --- Handle CompreFace Result (or lack thereof) and Store --- 
	if err != nil {
		log.Errorf("CompreFace recognition failed for Frigate event '%s' (image %s): %v", event.After.ID, filename, err)
		// Don't return! Set result to nil so storeRecognitionResults can still save the image.
		recognitionResult = nil
		log.Warnf("Proceeding to store image record for '%s' despite CompreFace error.", filename)
	} else if recognitionResult == nil || len(recognitionResult.Result) == 0 {
		log.Infof("CompreFace found no faces or matches in snapshot '%s' for Frigate event '%s'", filename, event.After.ID)
		// Don't return! storeRecognitionResults will handle the empty result.
		log.Infof("Proceeding to store image record for '%s' despite no faces found.", filename)
	} else {
		log.Infof("CompreFace processed snapshot '%s' for event '%s', found %d potential face(s).", filename, event.After.ID, len(recognitionResult.Result))
	}

	// --- Store Results (even if empty/error from CompreFace) --- 
	log.Debugf("Calling storeRecognitionResults for image '%s' (event %s)", filename, event.After.ID)
	// Pass event ID and hash here
	p.storeRecognitionResults(event.After.ID, filename, contentHash, time.Now(), recognitionResult) // Pass recognitionResult (can be nil)
}

// fetchFrigateSnapshot gets the snapshot image for a given event ID and type from the Frigate API.
// It saves the image to the configured snapshot directory and returns the image bytes and filename.
func (p *Processor) fetchFrigateSnapshot(eventID string, eventType string) ([]byte, string, error) { // Added eventType parameter
	// Construct snapshot URL (ensure Frigate URL in config ends without /)
	// Request snapshot in original resolution (removed h=1080)
	snapshotURL := fmt.Sprintf("%s/api/events/%s/snapshot.jpg", p.Cfg.Frigate.Url, eventID)
	log.Debugf("Constructed Frigate snapshot URL: %s", snapshotURL)

	// TODO: Add retries based on config.Frigate.Attempts.Snapshot
	// TODO: Add timeout based on config.Frigate.Attempts.Timeout
	resp, err := p.HttpClient.Get(snapshotURL)
	if err != nil {
		log.WithError(err).Errorf("Failed to fetch snapshot for Frigate event %s from %s", eventID, snapshotURL)
		return nil, "", fmt.Errorf("failed to fetch snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) // Read body for error context
		log.Errorf("Frigate API returned non-OK status %d for snapshot %s. Body: %s", resp.StatusCode, snapshotURL, string(bodyBytes))
		return nil, "", fmt.Errorf("frigate snapshot request failed with status %s", resp.Status)
	}

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Failed to read snapshot response body from %s: %v", snapshotURL, err)
		return nil, "", fmt.Errorf("failed to read snapshot response body: %w", err)
	}
	log.Debugf("Successfully read %d bytes for snapshot from %s", len(imageBytes), snapshotURL)

	// --- Save the snapshot locally ---
	// Ensure snapshot directory exists (consider doing this once at startup)
	snapshotDir := "/data/snapshots" // Make this configurable?
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		// Log error but maybe try to continue if dir already exists? This shouldn't normally fail.
		log.Errorf("Non-fatal error creating snapshot directory %s (maybe already exists?): %v", snapshotDir, err)
	}

	// Construct filename incorporating the event type and a nanosecond timestamp for uniqueness
	filename := fmt.Sprintf("%s_%s_%d_snapshot.jpg", eventID, eventType, time.Now().UnixNano()) // Changed filename format
	filePath := filepath.Join(snapshotDir, filename)
	log.Debugf("Attempting to save snapshot to file path: %s", filePath)

	// Write file
	if err := os.WriteFile(filePath, imageBytes, 0644); err != nil {
		log.Errorf("CRITICAL: Failed to write snapshot file %s: %v", filePath, err)
		return nil, "", fmt.Errorf("failed to write snapshot file: %w", err)
	}

	log.Infof("Successfully saved snapshot to %s", filePath)

	return imageBytes, filename, nil // Return filename only, not full path
}

// storeRecognitionResults saves the image metadata, detected faces, and recognized matches to the database.
// It now accepts the Frigate Event ID and Content Hash for storage.
func (p *Processor) storeRecognitionResults(frigateEventID string, filename string, contentHash string, timestamp time.Time, recognitionResult *services.CompreFaceRecognitionResponse) {
	// Use a transaction to ensure atomicity
	tx := p.DB.Begin()
	if tx.Error != nil {
		log.Errorf("Failed to start database transaction: %v", tx.Error)
		return // Cannot proceed
	}

	// Create Image record
	newImage := models.Image{
		FilePath:    filename, // Store ONLY the filename
		Timestamp:   timestamp,
		FrigateEventID: frigateEventID, // Store the Frigate Event ID
		ContentHash: contentHash, // Store the Content Hash
	}
	if err := tx.Create(&newImage).Error; err != nil {
		log.Errorf("Failed to save image record for %s (Event: %s): %v", filename, frigateEventID, err)
		tx.Rollback()
		return
	}
	log.Debugf("Successfully created Image DB record ID %d for %s (Event: %s)", newImage.ID, filename, frigateEventID)

	// Initialize slice to hold matches for SSE broadcast, declare *before* the conditional block
	var savedMatches []models.Match

	// If CompreFace processing happened and yielded results, process them
	if recognitionResult != nil && len(recognitionResult.Result) > 0 {
		log.Debugf("Processing %d CompreFace results for image ID %d", len(recognitionResult.Result), newImage.ID)
		for _, faceResult := range recognitionResult.Result {
			// Convert bounding box to JSON
			boxJSON, err := json.Marshal(faceResult.Box)
			if err != nil {
				log.Errorf("Failed to marshal bounding box to JSON for image ID %d: %v", newImage.ID, err)
				// Entscheiden, ob hier abgebrochen oder mit leerem Box-Feld weitergemacht werden soll?
				// Vorerst mit leerem Feld fortfahren.
				boxJSON = []byte("{}") // Leeres JSON-Objekt als Fallback
			}

			// Create Face record
			newFace := models.Face{
				ImageID:  newImage.ID,         // Link to the new image
				Detector: "compreface",        // Set detector name
				Box:      datatypes.JSON(boxJSON), // Store bounding box as JSON
				// Felder wie Age, Gender, etc. sind nicht im Modell definiert
			}

			if err := tx.Create(&newFace).Error; err != nil {
				log.Errorf("Failed to save face record for image ID %d: %v", newImage.ID, err)
				tx.Rollback()
				return // Stop processing this image's results
			}
			log.Debugf("Successfully created Face DB record ID %d for Image ID %d", newFace.ID, newImage.ID)

			// Process Matches for this Face
			if len(faceResult.Subjects) > 0 {
				log.Debugf("Processing %d subject matches for Face ID %d", len(faceResult.Subjects), newFace.ID)
				for _, subject := range faceResult.Subjects {
					// Check if Identity already exists or create a new one
					var identity models.Identity
					err := tx.Where("name = ?", subject.Subject).FirstOrCreate(&identity, models.Identity{Name: subject.Subject}).Error
					if err != nil {
						log.Errorf("Failed to find or create identity '%s': %v", subject.Subject, err)
						tx.Rollback()
						return // Stop processing
					}
					log.Debugf("Found/Created Identity ID %d for name '%s'", identity.ID, identity.Name)

					// Create Match record linking Face and Identity
					newMatch := models.Match{
						FaceID:     newFace.ID, // Korrigiert: FaceID verwenden
						IdentityID: identity.ID,
						Confidence: subject.Similarity,
						Timestamp:  time.Now(), // Setze Zeitstempel für den Match
						Detector:   "compreface", // Setze den Detector, der zum Match führte
					}
					if err := tx.Create(&newMatch).Error; err != nil {
						log.Errorf("Failed to save match record for Face ID %d, Identity ID %d: %v", newFace.ID, identity.ID, err)
						tx.Rollback()
						return // Stop processing
					}
					log.Debugf("Successfully created Match DB record ID %d linking Face %d and Identity %d", newMatch.ID, newFace.ID, identity.ID)
					// Eager load identity for SSE broadcast
					newMatch.Identity = identity
					savedMatches = append(savedMatches, newMatch)
				}
			} else {
				log.Debugf("No subject matches found for Face ID %d", newFace.ID)
			}
		}
	} else {
		log.Infof("No CompreFace results to store for image ID %d (Event: %s)", newImage.ID, frigateEventID)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		log.Errorf("Failed to commit database transaction for image %s (Event: %s): %v", filename, frigateEventID, err)
		// Rollback already attempted if errors occurred during creation
		return
	}

	log.Infof("Successfully stored results for image %s (Event: %s, DB Image ID: %d, Hash: %s)", filename, frigateEventID, newImage.ID, contentHash)

	// --- Broadcast the new image information via SSE --- 
	if p.sseHub != nil {
		p.sseHub.BroadcastNewImage(newImage, savedMatches)
	}
}
