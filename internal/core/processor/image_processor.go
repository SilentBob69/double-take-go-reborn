package processor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/integrations/compreface"
	"double-take-go-reborn/internal/integrations/frigate"
	"double-take-go-reborn/internal/server/sse"

	log "github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ProcessingOptions enthält Optionen für die Bildverarbeitung
type ProcessingOptions struct {
	// Hier können bei Bedarf weitere Optionen hinzugefügt werden
	DetectFaces    bool
	RecognizeFaces bool
}

// ImageProcessor verarbeitet Bilder, extrahiert Gesichter und identifiziert Personen
type ImageProcessor struct {
	db           *gorm.DB
	cfg          *config.Config
	compreface   *compreface.Client
	sseHub       *sse.Hub
	frigateClient *frigate.FrigateClient
}

// NewImageProcessor erstellt einen neuen Bildverarbeitungsprozessor
func NewImageProcessor(db *gorm.DB, cfg *config.Config, compreface *compreface.Client, sseHub *sse.Hub, frigateClient *frigate.FrigateClient) *ImageProcessor {
	return &ImageProcessor{
		db:           db,
		cfg:          cfg,
		compreface:   compreface,
		sseHub:       sseHub,
		frigateClient: frigateClient,
	}
}

// ProcessImage verarbeitet ein Bild, sucht nach Gesichtern und speichert die Ergebnisse
func (p *ImageProcessor) ProcessImage(ctx context.Context, imagePath, source string, options ProcessingOptions) (*models.Image, error) {
	log.Infof("Processing image %s from source %s", imagePath, source)

	// 1. Überprüfen, ob die Datei existiert
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("image file does not exist: %w", err)
	}

	// 2. Datei-Hash berechnen für Deduplizierung
	hash, err := calculateFileHash(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// 3. Prüfen, ob Bild bereits verarbeitet wurde (Deduplizierung)
	var existingImage models.Image
	if result := p.db.Where("content_hash = ?", hash).First(&existingImage); result.Error == nil {
		log.Infof("Image with hash %s already processed (ID: %d), skipping", hash, existingImage.ID)
		return &existingImage, nil
	}

	// 4. Bild in die Datenbank einfügen
	filename := filepath.Base(imagePath)
	
	// Relativen Pfad zum Snapshot-Verzeichnis ermitteln
	var relPath string
	if strings.HasPrefix(imagePath, p.cfg.Server.SnapshotDir) {
		// Den vollständigen relativen Pfad innerhalb des Snapshot-Verzeichnisses extrahieren
		relPath = strings.TrimPrefix(imagePath, p.cfg.Server.SnapshotDir)
		relPath = strings.TrimPrefix(relPath, "/") // Führenden Slash entfernen, falls vorhanden
	} else {
		// Fallback: Nur Dateiname, wenn kein relativer Pfad erkannt wurde
		relPath = filename
	}
	
	log.Debugf("Using relative path for image: %s", relPath)
	
	image := models.Image{
		FilePath:    relPath,
		Timestamp:   time.Now(),
		ContentHash: hash,
		Source:      source,
	}

	if err := p.db.Create(&image).Error; err != nil {
		return nil, fmt.Errorf("failed to create image record: %w", err)
	}

	log.Infof("Created new image record ID: %d", image.ID)

	// 5. CompreFace-Verarbeitung, falls aktiviert
	var allMatches []models.Match
	
	if p.cfg.CompreFace.Enabled && p.compreface != nil {
		allMatches, err = p.processWithCompreFace(ctx, imagePath, &image)
		if err != nil {
			log.Errorf("CompreFace processing failed: %v", err)
			// Wir fahren fort, auch wenn CompreFace fehlschlägt
		}
	} else {
		log.Info("CompreFace processing is disabled")
	}

	// 6. SSE-Broadcast für neue Bild-Erkennung, falls aktiviert
	if p.sseHub != nil {
		// Broadcast für alle Bilder, auch ohne erkannte Gesichter
		p.sseHub.BroadcastNewImage(image, p.cfg.Server.SnapshotURL+"/"+image.FilePath, allMatches)
	}

	return &image, nil
}

// processWithCompreFace verarbeitet ein Bild mit CompreFace
func (p *ImageProcessor) processWithCompreFace(ctx context.Context, imagePath string, image *models.Image) ([]models.Match, error) {
	log.Infof("Processing image %s with CompreFace", imagePath)
	
	// 1. Bilddaten lesen
	imageFile, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image file: %w", err)
	}
	defer imageFile.Close()
	
	imageData, err := io.ReadAll(imageFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// 2. CompreFace aufrufen
	result, err := p.compreface.Recognize(ctx, imageData, filepath.Base(imagePath))
	if err != nil {
		return nil, fmt.Errorf("CompreFace recognition failed: %w", err)
	}

	if result == nil || len(result.Result) == 0 {
		log.Info("No faces detected by CompreFace")
		return nil, nil
	}

	// 3. Ergebnisse verarbeiten
	var allMatches []models.Match
	
	for _, faceResult := range result.Result {
		// Box-Informationen als JSON vorbereiten
		boxJSON := map[string]interface{}{
			"x_min":       faceResult.Box.XMin,
			"y_min":       faceResult.Box.YMin,
			"x_max":       faceResult.Box.XMax,
			"y_max":       faceResult.Box.YMax,
			"probability": faceResult.Box.Probability,
		}
		
		boxBytes, err := json.Marshal(boxJSON)
		if err != nil {
			log.Errorf("Failed to marshal bounding box data: %v", err)
			continue
		}

		// Face-Eintrag erstellen
		face := models.Face{
			ImageID:     image.ID,
			BoundingBox: datatypes.JSON(boxBytes),
			Confidence:  faceResult.Box.Probability,
			Detector:    "compreface",
		}

		if err := p.db.Create(&face).Error; err != nil {
			log.Errorf("Failed to create face record: %v", err)
			continue
		}

		log.Infof("Created face record ID: %d for image ID: %d", face.ID, image.ID)

		// Alle Subjekte/Personen für dieses Gesicht durchgehen
		for _, subject := range faceResult.Subjects {
			// Identity finden oder erstellen
			var identity models.Identity
			
			// Erst versuchen, vorhandene Identity zu finden
			if err := p.db.Where("name = ?", subject.Subject).First(&identity).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					// Neue Identity erstellen
					identity = models.Identity{
						Name:      subject.Subject,
						ExternalID: subject.Subject, // ExternalID = Name in CompreFace
					}
					
					if err := p.db.Create(&identity).Error; err != nil {
						log.Errorf("Failed to create identity record: %v", err)
						continue
					}
					
					log.Infof("Created new identity record ID: %d, name: %s", identity.ID, identity.Name)
				} else {
					log.Errorf("Database error when finding identity: %v", err)
					continue
				}
			}

			// Match-Eintrag erstellen
			match := models.Match{
				FaceID:     face.ID,
				IdentityID: identity.ID,
				Confidence: subject.Similarity,
			}

			if err := p.db.Create(&match).Error; err != nil {
				log.Errorf("Failed to create match record: %v", err)
				continue
			}

			// Match mit Identity nachladen für SSE
			p.db.Model(&match).Association("Identity").Find(&match.Identity)
			
			// Match zur Liste hinzufügen
			allMatches = append(allMatches, match)
			
			log.Infof("Created match record ID: %d (face: %d, identity: %d, confidence: %.2f)", 
				match.ID, match.FaceID, match.IdentityID, match.Confidence)
		}
	}

	return allMatches, nil
}

// ProcessFrigateEvent verarbeitet ein Ereignis aus Frigate via MQTT
func (p *ImageProcessor) ProcessFrigateEvent(ctx context.Context, payload []byte) error {
	// Wenn Frigate-Integration deaktiviert ist, nichts tun
	if !p.cfg.Frigate.Enabled {
		log.Debug("Frigate integration is disabled, skipping event")
		return nil
	}

	var frigateEvent map[string]interface{}
	if err := json.Unmarshal(payload, &frigateEvent); err != nil {
		return fmt.Errorf("failed to unmarshal Frigate event: %w", err)
	}

	// Einfache Validierung: Prüfen, ob das Event eine After- oder Before-Sektion hat
	if _, hasAfter := frigateEvent["after"]; !hasAfter {
		if _, hasBefore := frigateEvent["before"]; !hasBefore {
			log.Debug("Skipping Frigate event: neither 'after' nor 'before' section found")
			return nil
		}
	}

	// Frigate-Client erstellen, falls noch nicht vorhanden
	if p.frigateClient == nil {
		p.frigateClient = frigate.NewFrigateClient(p.cfg.Frigate)
	}

	// Frigate-Event parsen
	event, err := p.frigateClient.ParseEventMessage(payload)
	if err != nil {
		return fmt.Errorf("failed to parse Frigate event: %w", err)
	}

	// Prüfen, ob es ein Personen-Event ist (falls process_person_only aktiviert ist)
	if p.cfg.Frigate.ProcessPersonOnly && !p.frigateClient.IsPersonEvent(event) {
		log.Debug("Skipping non-person Frigate event")
		return nil
	}

	eventData := p.frigateClient.GetEventData(event)
	if eventData == nil {
		return fmt.Errorf("no event data found in Frigate event")
	}

	// Snapshot-URL extrahieren
	if eventData.Snapshot == "" {
		log.Debug("Skipping Frigate event: no snapshot URL")
		return nil
	}

	// Dateiname für den Snapshot generieren
	filename := p.frigateClient.GenerateFilename(eventData)
	localPath := filepath.Join("frigate", filename)
	fullPath := filepath.Join(p.cfg.Server.SnapshotDir, localPath)

	// Snapshot herunterladen
	if err := p.frigateClient.DownloadSnapshot(eventData.Snapshot, fullPath); err != nil {
		return fmt.Errorf("failed to download snapshot: %w", err)
	}

	// Bild in der Datenbank speichern
	image := p.frigateClient.ToImage(eventData, localPath)
	if err := p.db.Create(image).Error; err != nil {
		return fmt.Errorf("failed to save image to database: %w", err)
	}

	log.Infof("Saved Frigate event image: %s", localPath)

	// Bild verarbeiten (Gesichtserkennung)
	_, err = p.ProcessImage(ctx, fullPath, "frigate", ProcessingOptions{})
	if err != nil {
		return fmt.Errorf("failed to process image: %w", err)
	}

	return nil
}

// calculateFileHash berechnet einen SHA-256-Hash für eine Datei
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
