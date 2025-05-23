package processor

import (
	"double-take-go-reborn/internal/util/timezone"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	stdimage "image"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/integrations/compreface"
	"double-take-go-reborn/internal/integrations/facerecognition"
	"double-take-go-reborn/internal/integrations/frigate"
	"double-take-go-reborn/internal/integrations/homeassistant"
	"double-take-go-reborn/internal/integrations/opencv"
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
	Metadata       map[string]interface{} // Zusätzliche Metadaten
	ExistingImageID uint                  // Falls gesetzt, wird dieses existierende Bild verwendet anstatt ein neues zu erstellen
}

// ImageProcessor verarbeitet Bilder, extrahiert Gesichter und identifiziert Personen
type ImageProcessor struct {
	db            *gorm.DB
	cfg           *config.Config
	compreface    *compreface.APIClient    // Für Abwärtskompatibilität
	providerManager *facerecognition.ProviderManager
	opencvService *opencv.Service     // OpenCV-Service für Gesichtserkennung als Vorfilter
	sseHub        *sse.Hub
	frigateClient *frigate.FrigateClient
	haPublisher   *homeassistant.Publisher
	workerPool    *WorkerPool // Referenz zum Worker-Pool für parallele Verarbeitung
}

// NewImageProcessor erstellt einen neuen Bildverarbeitungsprozessor
func NewImageProcessor(db *gorm.DB, cfg *config.Config, compreface *compreface.APIClient, providerManager *facerecognition.ProviderManager, sseHub *sse.Hub, frigateClient *frigate.FrigateClient, haPublisher *homeassistant.Publisher) *ImageProcessor {
	// OpenCV-Service initialisieren, falls in der Config aktiviert
	var opencvService *opencv.Service
	if cfg.OpenCV.Enabled {
		var err error
		opencvService, err = opencv.NewService(&cfg.OpenCV)
		if err != nil {
			log.Warnf("Fehler beim Initialisieren des OpenCV-Service: %v. OpenCV wird deaktiviert.", err)
		} else {
			// Logging der OpenCV-Konfiguration
			useGPU := cfg.OpenCV.UseGPU
			detectionMethod := cfg.OpenCV.PersonDetection.Method
			log.Infof("OpenCV-Service erfolgreich initialisiert (GPU: %v, Methode: %s)", useGPU, detectionMethod)
			
			if useGPU {
				log.Infof("OpenCV GPU-Konfiguration: Backend=%s, Target=%s", 
					cfg.OpenCV.PersonDetection.Backend, cfg.OpenCV.PersonDetection.Target)
			}
		}
	} else {
		log.Info("OpenCV-Service ist in der Konfiguration deaktiviert")
	}

	return &ImageProcessor{
		db:            db,
		cfg:           cfg,
		compreface:    compreface,
		providerManager: providerManager,
		opencvService: opencvService,
		sseHub:        sseHub,
		frigateClient: frigateClient,
		haPublisher:   haPublisher,
	}
}

// SetWorkerPool setzt den Worker-Pool für die Bildverarbeitung
func (p *ImageProcessor) SetWorkerPool(workerPool *WorkerPool) {
	p.workerPool = workerPool
}

// GetOpenCVService gibt einen Verweis auf den OpenCV-Service zurück
func (p *ImageProcessor) GetOpenCVService() *opencv.Service {
	return p.opencvService
}

// ProcessImage verarbeitet ein Bild, sucht nach Gesichtern und speichert die Ergebnisse
func (p *ImageProcessor) ProcessImage(ctx context.Context, imagePath, source string, options ProcessingOptions) (*models.Image, error) {
	// Falls der Worker-Pool verfügbar ist, diesen verwenden
	if p.workerPool != nil {
		return p.workerPool.ProcessImage(ctx, imagePath, source, options)
	}
	
	// Fallback auf die direkte Verarbeitung, wenn kein Worker-Pool verfügbar ist
	return p.processImageInternal(ctx, imagePath, source, options)
}

// processImageInternal enthält die interne Verarbeitungslogik für die Bildverarbeitung
// Diese Methode wird vom Worker-Pool aufgerufen
func (p *ImageProcessor) processImageInternal(ctx context.Context, imagePath, source string, options ProcessingOptions) (*models.Image, error) {
	log.Infof("Processing image %s from source %s", imagePath, source)

	// 1. Überprüfen, ob die Datei existiert
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("image file does not exist: %w", err)
	}

	// Keine Deduplizierung oder Limitierung mehr anwenden
	// Jedes Bild wird verarbeitet, unabhängig von Event-ID oder Zeitstempel
	log.Debugf("Verarbeite Bild ohne Einschränkungen: %s", imagePath)

	// 4. Bild in die Datenbank einfügen
	filename := filepath.Base(imagePath)
	
	// Relativen Pfad zum Snapshot-Verzeichnis ermitteln
	var relPath string
	if strings.HasPrefix(imagePath, p.cfg.Server.SnapshotDir) {
		// KORREKTUR: Bei Frigate-Bildern MÜSSEN wir das frigate/-Präfix hinzufügen, um Konsistenz mit dem Dateisystem zu gewährleisten
		if source == "frigate" {
			// Wichtig: Für Frigate-Bilder verwenden wir 'frigate/dateiname.jpg' als relativen Pfad
			// dadurch stimmt die URL '/image/frigate/dateiname.jpg' mit der Dateisystemstruktur überein
			relPath = "frigate/" + filepath.Base(imagePath)
		} else {
			// Für alle anderen Quellen den vollständigen relativen Pfad extrahieren
			relPath = strings.TrimPrefix(imagePath, p.cfg.Server.SnapshotDir)
			relPath = strings.TrimPrefix(relPath, "/") // Führenden Slash entfernen, falls vorhanden
		}
	} else {
		// Fallback: Nur Dateiname, wenn kein relativer Pfad erkannt wurde
		relPath = filename
	}
	
	log.Debugf("Using relative path for image: %s", relPath)
	
	// Bei Frigate-Bildern nutzen wir einen Standard-Hash, da wir die Hash-Berechnung einsparen
	var contentHash string
	if source == "frigate" {
		// Für Frigate-Events generieren wir einen einfachen Hash aus dem Dateinamen
		contentHash = fmt.Sprintf("frigate:%s", filepath.Base(imagePath))
	} else {
		// Für andere Quellen berechnen wir weiterhin den SHA-256 Hash
		var err error
		contentHash, err = calculateFileHash(imagePath)
		if err != nil {
			log.Warnf("Konnte Hash für %s nicht berechnen: %v, verwende Fallback", imagePath, err)
			contentHash = fmt.Sprintf("fallback:%s:%d", filepath.Base(imagePath), timezone.Now().UnixNano())
		}
	}

	image := models.Image{
		FilePath:    relPath,
		Timestamp:   timezone.Now(),
		ContentHash: contentHash,
		Source:      source,
	}

	// Optional sind Metadaten vorhanden (z.B. von Frigate-Events)
	if options.Metadata != nil {
		// Wir speichern relevante Metadaten in den entsprechenden Feldern
		if eventID, ok := options.Metadata["event_id"].(string); ok {
			image.EventID = eventID
		}
		if label, ok := options.Metadata["label"].(string); ok {
			image.Label = label
		}
		
		// Zonen-Informationen extrahieren und zusammenführen, wenn vorhanden
		var zones []string
		if currentZones, ok := options.Metadata["current_zones"].([]string); ok && len(currentZones) > 0 {
			zones = append(zones, currentZones...)
		}
		if enteredZones, ok := options.Metadata["entered_zones"].([]string); ok && len(enteredZones) > 0 {
			for _, zone := range enteredZones {
				if !contains(zones, zone) {
					zones = append(zones, zone)
				}
			}
		}
		if len(zones) > 0 {
			image.Zone = strings.Join(zones, ",")
		}

		// Alle übrigen Metadaten als JSON serialisieren und speichern
		if sourceData, err := json.Marshal(options.Metadata); err == nil {
			image.SourceData = sourceData
		}
	}

	// 5. OpenCV-Vorfilterung durchführen, falls aktiviert
	var matches []models.Match
	var opencvPersons []opencv.DetectedPerson
	var hasPersons = true // Standardmäßig nehmen wir an, dass Personen vorhanden sind

	// OpenCV-Personenerkennung als strikter Vorfilter verwenden, wenn aktiviert
	if p.opencvService != nil && p.cfg.OpenCV.Enabled {
		log.Debugf("Verwende OpenCV-Personenerkennung als Vorfilter für: %s", imagePath)
		
		// Prüfen, ob im Bild Personen vorhanden sind
		var opencvErr error
		hasPersons, opencvPersons, opencvErr = p.opencvService.DetectPersons(ctx, imagePath)
		
		if opencvErr != nil {
			log.Warnf("OpenCV-Personenerkennung fehlgeschlagen: %v. Fahre mit CompreFace fort.", opencvErr)
			// Bei Fehlern in der Vorfilterung trotzdem mit CompreFace fortfahren
		} else if !hasPersons {
			// Keine Personen erkannt - Verarbeitung komplett überspringen
			log.Infof("OpenCV: Keine Personen im Bild erkannt, Verarbeitung wird komplett übersprungen: %s", imagePath)
			return nil, nil // Bild nicht speichern und keine weitere Verarbeitung durchführen
		} else {
			// Personen erkannt - Metadaten speichern und mit Gesichtserkennung fortfahren
			log.Infof("OpenCV: %d Personen im Bild erkannt, fahre mit %s fort: %s", len(opencvPersons), p.providerManager.GetActiveProviderName(), imagePath)
			
			// Erkannte Personen in den Metadaten speichern
			personRects := make([]map[string]interface{}, 0, len(opencvPersons))
			for _, person := range opencvPersons {
				personRects = append(personRects, map[string]interface{}{
					"x_min": person.Rectangle.Min.X,
					"y_min": person.Rectangle.Min.Y,
					"x_max": person.Rectangle.Max.X,
					"y_max": person.Rectangle.Max.Y,
					"confidence": person.Confidence,
				})
			}
			
			// Metadaten zum Bild hinzufügen
			metadataJSON := map[string]interface{}{
				"opencv_processed": true,
				"persons_detected": true,
				"person_count": len(opencvPersons),
				"person_rects": personRects,
			}
			
			if metadataBytes, err := json.Marshal(metadataJSON); err == nil {
				image.SourceData = datatypes.JSON(metadataBytes)
			}
		}
	}

	// 6. Bild nur in der Datenbank speichern, wenn es verarbeitet werden soll
	// (also wenn OpenCV nicht aktiviert ist ODER Personen erkannt wurden)
	if options.ExistingImageID > 0 {
		// Vorhandenes Bild verwenden - wir müssen es aus der Datenbank laden
		log.Infof("Verwende existierendes Bild mit ID: %d", options.ExistingImageID)
		if err := p.db.First(&image, options.ExistingImageID).Error; err != nil {
			return nil, fmt.Errorf("failed to find existing image record: %w", err)
		}
		log.Infof("Bestehendes Bild mit ID %d geladen", image.ID)
	} else {
		// Neues Bild erstellen
		if err := p.db.Create(&image).Error; err != nil {
			return nil, fmt.Errorf("failed to create image record: %w", err)
		}
		log.Infof("Created new image record ID: %d", image.ID)
	}
	
	// 7. Gesichtserkennung durchführen
	// Variable matches wurde bereits in Zeile 202 deklariert
	var faceRecognitionErr error
	if !p.cfg.OpenCV.Enabled || hasPersons {
		// Gesichtserkennung mit dem aktiven Provider durchführen
		recognitionMatches, err := p.processWithFaceRecognition(ctx, imagePath, &image)
		faceRecognitionErr = err
		if err != nil {
			log.Warnf("Face recognition failed: %v", err)
		} else if len(recognitionMatches) > 0 {
			matches = recognitionMatches
		}
	}

	// 8. SSE-Broadcast durchführen
	if p.sseHub != nil {
		// Anzahl der Gesichter zählen für das Logging
		var faceCount int64 = 0
		if faceRecognitionErr == nil {
			// Gesichter in der Datenbank zählen
			p.db.Model(&models.Face{}).Where("image_id = ?", image.ID).Count(&faceCount)
			log.Infof("Gesichter in Bild ID %d erkannt: %d", image.ID, faceCount)
		}

		// Broadcast nur, wenn Gesichtserkennung erfolgreich oder explizit keine Gesichter gefunden wurden
		log.Infof("Sende SSE-Broadcast für Bild ID %d mit %d Matches (Gesichter: %d)", image.ID, len(matches), faceCount)
		p.sseHub.BroadcastNewImage(image, p.cfg.Server.SnapshotURL+"/"+image.FilePath, matches)
	}
	
	return &image, nil
}

// processWithFaceRecognition verarbeitet ein Bild mit dem aktiven Gesichtserkennungsanbieter
func (p *ImageProcessor) processWithFaceRecognition(ctx context.Context, imagePath string, image *models.Image) ([]models.Match, error) {
	// Den aktiven Provider ermitteln
	activeProvider, ok := p.providerManager.GetActiveProvider()
	if !ok || activeProvider == nil {
		return nil, fmt.Errorf("no active face recognition provider available")
	}
	
	// 1. Bilddaten lesen
	imageData, err := ioutil.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}
	
	// Bild für die API-Verarbeitung dekodieren
	img, _, err := stdimage.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	
	// 2. Gesichtserkennung durchführen
	detectionRequest := facerecognition.DetectionRequest{
		ReturnFaceData: true,
		ExtractEmbedding: true,
	}
	
	detectionResult, err := activeProvider.DetectFaces(ctx, img, detectionRequest)
	if err != nil {
		return nil, fmt.Errorf("face detection failed: %w", err)
	}
	
	log.Infof("%s detected %d faces in image %s", p.providerManager.GetActiveProviderName(), len(detectionResult.Faces), imagePath)

	// 3. Für jedes erkannte Gesicht Gesichts-DB-Eintrag erstellen
	var matches []models.Match
	
	for i, face := range detectionResult.Faces {
		log.Infof("Processing face #%d with confidence %.2f", i+1, face.Confidence)
		
		// BoundingBox Array auspacken [x_min, y_min, x_max, y_max]
		if len(face.BoundingBox) < 4 {
			log.Errorf("Invalid bounding box format for face #%d", i+1)
			continue
		}
		
		// Bounding Box als JSON für die Datenbank vorbereiten
		boundingBoxJSON, err := json.Marshal(map[string]interface{}{
			"x_min": face.BoundingBox[0],
			"y_min": face.BoundingBox[1],
			"x_max": face.BoundingBox[2],
			"y_max": face.BoundingBox[3],
		})
		if err != nil {
			log.Errorf("Failed to marshal bounding box data: %v", err)
			continue
		}
		
		// Gesichter-DB-Eintrag erstellen
		dbFace := models.Face{
			ImageID:     image.ID,
			BoundingBox: datatypes.JSON(boundingBoxJSON),
			Confidence:  face.Confidence,
			Detector:    string(p.providerManager.GetActiveProviderName()),
		}
		
		if err := p.db.Create(&dbFace).Error; err != nil {
			log.Errorf("Failed to create face record: %v", err)
			continue
		}

		log.Infof("Created face record ID: %d for image ID: %d", dbFace.ID, image.ID)

		// 4. Gesichtserkennung durchführen
		// Erkennungsparameter festlegen
		threshold := 0.7 // Standard-Schwellwert
		if p.providerManager.GetActiveProviderName() == facerecognition.ProviderCompreFace && p.cfg.CompreFace.SimilarityThreshold > 0 {
			threshold = p.cfg.CompreFace.SimilarityThreshold / 100.0 // Umrechnung von Prozent (0-100) auf Dezimalwert (0-1)
		} else if p.providerManager.GetActiveProviderName() == facerecognition.ProviderInsightFace && p.cfg.InsightFace.RecognitionThreshold > 0 {
			threshold = p.cfg.InsightFace.RecognitionThreshold
		}
		
		// Gesichtserkennung mit der RecognizeFaces-Methode (aus der Provider-Schnittstelle)
		recognitionRequest := facerecognition.RecognitionRequest{
			Threshold: threshold,
		}
		
		// Gesichtserkennung durchführen
		recognitionResult, err := activeProvider.RecognizeFaces(ctx, img, recognitionRequest)
		if err != nil {
			log.Errorf("Face recognition failed for face #%d: %v", i+1, err)
			continue
		}
		
		// Wenn Ergebnisse vorhanden sind und Gesichter gefunden wurden
		if recognitionResult != nil && len(recognitionResult.Faces) > 0 && len(recognitionResult.Matches) > 0 {
			// Für jedes gefundene Gesicht und dessen Matches DB-Einträge erstellen
			for faceIndex, _ := range recognitionResult.Faces {
				// Wir verwenden den faceIndex, um die entsprechenden Matches zu finden
				if faceIndex >= len(recognitionResult.Matches) || len(recognitionResult.Matches[faceIndex]) == 0 {
					// Keine Matches für dieses Gesicht
					continue
				}
				
				// Für jeden Match ein DB-Eintrag erstellen
				for _, match := range recognitionResult.Matches[faceIndex] {
					// Identity finden oder erstellen
					var identity models.Identity
					
					// Erst versuchen, vorhandene Identity zu finden
					if err := p.db.Where("external_id = ?", match.SubjectID).First(&identity).Error; err != nil {
						if err == gorm.ErrRecordNotFound {
							// Neue Identity erstellen
							identity = models.Identity{
								Name:       match.SubjectID, // SubjectID als Standardname verwenden
								ExternalID: match.SubjectID,
							}
						
							if err := p.db.Create(&identity).Error; err != nil {
								log.Errorf("Failed to create identity record: %v", err)
								continue
							}
							log.Infof("Created new identity record: %s (ID: %d)", identity.Name, identity.ID)
						} else {
							log.Errorf("Failed to query identity: %v", err)
							continue
						}
					}
					
					// Match-Eintrag erstellen
					matchRecord := models.Match{
						FaceID:     dbFace.ID,
						IdentityID: identity.ID,
						Confidence: match.Similarity,
					}
					
					if err := p.db.Create(&matchRecord).Error; err != nil {
						log.Errorf("Failed to create match record: %v", err)
						continue
					}

					// Match mit Identity nachladen für SSE
					p.db.Model(&matchRecord).Association("Identity").Find(&matchRecord.Identity)
					
					// Match zur Liste hinzufügen
					matches = append(matches, matchRecord)
					
					log.Infof("Created match record ID: %d (face: %d, identity: %d, confidence: %.2f)", 
						matchRecord.ID, matchRecord.FaceID, matchRecord.IdentityID, matchRecord.Confidence)
				}
			}
		} else {
			log.Infof("No matching identity found for face #%d", i+1)
		}
	}

	return matches, nil
}

// contains prüft, ob ein String in einem Slice enthalten ist
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// ProcessFrigateEvent verarbeitet ein Ereignis aus Frigate via MQTT
// und erfasst mehrere Snapshots während des Ereignisses, nicht nur den letzten
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

	// Ereignistyp prüfen und dementsprechend verarbeiten
	switch event.Type {
	case "new": 
		// Neues Ereignis - mehrere Snapshots verarbeiten, wenn verfügbar
		return p.processNewFrigateEvent(ctx, event)
	case "update": 
		// Update-Ereignis - Wir verarbeiten auch Updates, um mehr Bilder zu erfassen
		return p.processUpdateFrigateEvent(ctx, event)
	default:
		log.Debugf("Skipping Frigate event type: %s", event.Type)
		return nil
	}
}

// processNewFrigateEvent verarbeitet ein neues Frigate-Ereignis und versucht,
// möglichst frühe Snapshots zu erfassen, wenn die Person zur Kamera hinläuft
func (p *ImageProcessor) processNewFrigateEvent(ctx context.Context, event *frigate.FrigateEvent) error {
	// Prüfen, ob es ein Personen-Event ist (falls process_person_only aktiviert ist)
	if p.cfg.Frigate.ProcessPersonOnly && !p.frigateClient.IsPersonEvent(event) {
		log.Debug("Ignoriere Nicht-Personen-Event von Frigate")
		return nil
	}

	// Extrahieren der Event-Daten (After hat Priorität)
	eventData := p.frigateClient.GetEventData(event)
	if eventData == nil {
		return fmt.Errorf("keine Event-Daten im Frigate-Event gefunden")
	}

	// Prüfen, ob das Event einen Snapshot hat
	if !eventData.HasSnapshot {
		log.Debug("Ignoriere Frigate-Event ohne Snapshot")
		return nil
	}

	// Snapshot-URL extrahieren
	snapshotURL := eventData.GetSnapshotURL()
	if snapshotURL == "" {
		log.Debug("Ignoriere Frigate-Event: Snapshot-URL konnte nicht extrahiert werden")
		return nil
	}

	// Ein neues Ereignis verarbeiten - hier nutzen wir sowohl den Thumbnail als auch den Snapshot
	// Der Thumbnail enthält oft ein früheres Bild der Person, wenn sie auf die Kamera zuläuft
	snapshotPaths := []string{snapshotURL}
	
	// Wenn ein Thumbnail verfügbar ist, diesen auch verarbeiten
	thumbnailURL := eventData.GetThumbnailURL()
	if thumbnailURL != "" && thumbnailURL != snapshotURL {
		snapshotPaths = append(snapshotPaths, thumbnailURL)
		log.Debugf("Thumbnail zur Verarbeitungswarteschlange für Event %s hinzugefügt: %s", eventData.ID, thumbnailURL)
	}

	// Metadaten für die Bildverarbeitung vorbereiten
	metadata := map[string]interface{}{
		"event_id":        eventData.ID,
		"camera":          eventData.Camera,
		"label":           eventData.Label,
		"score":           eventData.Score,
		"current_zones":   eventData.CurrentZones,
		"entered_zones":   eventData.EnteredZones,
		"start_time":      eventData.GetStartTime().Format(time.RFC3339),
		"source":          "frigate",
	}
	
	// Liste der verarbeiteten Bilder für das spätere SSE-Event
	var processedImages []models.Image

	// Für jeden Snapshot-Pfad ein Bild verarbeiten
	for i, snapshotPath := range snapshotPaths {
		// Dateiname für den Snapshot generieren mit Sequenznummer
		baseFilename := p.frigateClient.GenerateFilename(eventData)
		filenameParts := strings.Split(baseFilename, ".")
		baseName := filenameParts[0]
		extension := ".jpg"
		if len(filenameParts) > 1 {
			extension = "." + filenameParts[1]
		}
		filename := fmt.Sprintf("%s_seq%d%s", baseName, i, extension)
		
		// Explizit prüfen, ob die Dateiendung vorhanden ist
		if !strings.HasSuffix(filename, ".jpg") {
			filename = filename + ".jpg"
		}
		
		// KORREKTUR: Der localPath MUSS ein "frigate/"-Präfix haben, damit der Browser die Bilder im richtigen Verzeichnis finden kann
		// Die Dateien werden im Unterverzeichnis "frigate" gespeichert, also muss auch der Pfad in der Datenbank so sein
		localPath := filepath.Join("frigate", filename)
		fullPath := filepath.Join(p.cfg.Server.SnapshotDir, "frigate", filename)

		// Pfad für den Snapshot vorbereiten
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			log.Warnf("Konnte Verzeichnis für Snapshot nicht erstellen: %v", err)
			continue
		}

		// Snapshot über API herunterladen
		if err := p.frigateClient.DownloadSnapshot(snapshotPath, fullPath); err != nil {
			log.Warnf("Konnte Snapshot %d für Event %s nicht über API laden: %v", i, eventData.ID, err)
			continue // Wir versuchen den nächsten Snapshot, falls einer fehlschlägt
		}

		// Hinweis: Die direkte Extraktion von Bilddaten aus dem Event ist momentan nicht implementiert
		// Zukünftige Implementierung: ExtractSnapshotFromEvent in FrigateClient

		// Zum aktuellen Bild spezifische Metadaten hinzufügen
		imageMetadata := make(map[string]interface{})
		for k, v := range metadata {
			imageMetadata[k] = v
		}
		imageMetadata["sequence"] = i
		if i == 1 { // Thumbnail
			imageMetadata["image_type"] = "thumbnail"
		} else { // Hauptbild
			imageMetadata["image_type"] = "snapshot"
		}

		// Bild zur Verarbeitung an den Worker-Pool übergeben
		// Dies stellt sicher, dass Karten erst nach der CompreFace-Verarbeitung erstellt werden
		image, processErr := p.ProcessImage(ctx, fullPath, "frigate", ProcessingOptions{
			DetectFaces:    true,
			RecognizeFaces: true,
			Metadata:       imageMetadata,
		})
		if processErr != nil {
			log.Warnf("Fehler bei der Verarbeitung des Bildes %s: %v", fullPath, processErr)
		} else if image == nil {
			// Wenn OpenCV keine Person gefunden hat, ist image nil, aber kein Fehler
			log.Infof("Frigate-Event-Bild %d von %d übersprungen (keine Person erkannt): %s", i+1, len(snapshotPaths), localPath)
		} else {
			// Das verarbeitete Bild zur Liste hinzufügen für das spätere SSE-Event
			processedImages = append(processedImages, *image)
			log.Infof("Frigate-Event-Bild %d von %d verarbeitet: %s", i+1, len(snapshotPaths), localPath)
		}
	}

	// Abschließend ein Event für die neue Bildgruppe senden, wenn der SSE-Hub verfügbar ist
	if p.sseHub != nil && len(processedImages) > 0 {
		eventData := p.frigateClient.GetEventData(event)
		
		// Event-Datendetails extrahieren
		camera := eventData.Camera
		label := eventData.Label
		zoneName := ""
		if len(eventData.CurrentZones) > 0 {
			zoneName = eventData.CurrentZones[0]
		} else if len(eventData.EnteredZones) > 0 {
			zoneName = eventData.EnteredZones[0]
		}
		
		// Thumbnail-URL bestimmen (für die erste Bildvorschau)
		thumbnailURL := ""
		if len(processedImages) > 0 {
			thumbnailURL = p.cfg.Server.SnapshotURL + "/" + processedImages[0].FilePath
		}
		
		log.Infof("Sende BroadcastNewGroup-Event für Event-ID %s mit %d Bildern", eventData.ID, len(processedImages))
		p.sseHub.BroadcastNewGroup(eventData.ID, "frigate", camera, label, zoneName, processedImages, thumbnailURL)
	}

	return nil
}

// processUpdateFrigateEvent verarbeitet ein Update eines Frigate-Ereignisses
// Updates können wichtig sein, weil sie oft bessere Bilder der Person enthalten
func (p *ImageProcessor) processUpdateFrigateEvent(ctx context.Context, event *frigate.FrigateEvent) error {
	// Prüfen, ob es ein Personen-Event ist (falls process_person_only aktiviert ist)
	if p.cfg.Frigate.ProcessPersonOnly && !p.frigateClient.IsPersonEvent(event) {
		log.Debug("Ignoriere Nicht-Personen-Update-Event von Frigate")
		return nil
	}

	// Bei Updates interessieren uns insbesondere die After-Daten
	if event.After == nil {
		log.Debug("Ignoriere Frigate-Update-Event ohne After-Daten")
		return nil
	}

	eventData := event.After
	
	// Prüfen, ob das Event einen Snapshot hat
	if !eventData.HasSnapshot {
		log.Debug("Ignoriere Frigate-Update-Event ohne Snapshot")
		return nil
	}

	snapshotURL := eventData.GetSnapshotURL()
	if snapshotURL == "" {
		log.Debug("Ignoriere Frigate-Update-Event: Snapshot-URL konnte nicht extrahiert werden")
		return nil
	}

	// Überprüfen, ob wir für dieses Ereignis bereits Updates verarbeitet haben
	var existingImages []models.Image
	if err := p.db.Where("source = ? AND event_id = ?", "frigate", eventData.ID).Find(&existingImages).Error; err != nil {
		log.Warnf("Fehler bei der Abfrage existierender Bilder für Event %s: %v", eventData.ID, err)
	}

	// Prüfen, ob das exakt gleiche Bild (mit gleichem Zeitstempel) bereits verarbeitet wurde
	frameTime := eventData.GetCurrentTime().Unix()
	for _, img := range existingImages {
		imgTime := img.Timestamp.Unix()
		// Wenn der Zeitstempel des Bildes und des Events übereinstimmen (mit 1 Sekunde Toleranz)
		if imgTime == frameTime || imgTime == frameTime-1 || imgTime == frameTime+1 {
			log.Infof("Bild mit dem gleichen Zeitstempel (%d) wurde bereits verarbeitet, überspringe", frameTime)
			return nil
		}
	}
	
	// Wenn mehr als 5 Bilder für dieses Event existieren, nur die ältesten löschen, um Platz zu schaffen
	if len(existingImages) >= 5 {
		log.Infof("Für dieses Event existieren bereits %d Bilder. Begrenze auf maximal 5.", len(existingImages))
		
		// Bestimme, wie viele Bilder gelöscht werden müssen
		excessCount := len(existingImages) - 4 // 5 - 1 für das neue Bild
		if excessCount > 0 {
			// Nimm die ältesten Bilder zum Löschen (die ersten 'excessCount' im Array)
			var imagesToDelete []models.Image
			var imageIDsToDelete []uint
			
			// Da wir keine Sortierung implementiert haben, nehmen wir einfach die ersten Einträge
			// In einer vollständigen Implementierung würden wir nach Zeitstempel sortieren
			for i := 0; i < excessCount; i++ {
				imagesToDelete = append(imagesToDelete, existingImages[i])
				imageIDsToDelete = append(imageIDsToDelete, existingImages[i].ID)
			}
			
			// Finde alle Gesichter für diese zu löschenden Bilder
			var faces []models.Face
			if err := p.db.Where("image_id IN ?", imageIDsToDelete).Find(&faces).Error; err != nil {
				log.Warnf("Fehler beim Laden zu löschender Gesichter: %v", err)
			} else {
				// Sammle alle Gesichts-IDs
				var faceIDs []uint
				for _, face := range faces {
					faceIDs = append(faceIDs, face.ID)
				}
				
				// Lösche alle Matches dieser Gesichter
				if len(faceIDs) > 0 {
					if err := p.db.Where("face_id IN ?", faceIDs).Delete(&models.Match{}).Error; err != nil {
						log.Warnf("Fehler beim Löschen alter Matches: %v", err)
					} else {
						log.Infof("Alte Matches für %d Gesichter gelöscht", len(faceIDs))
					}
				}
				
				// Lösche die Gesichter der zu löschenden Bilder
				if err := p.db.Where("image_id IN ?", imageIDsToDelete).Delete(&models.Face{}).Error; err != nil {
					log.Warnf("Fehler beim Löschen alter Gesichter: %v", err)
				} else {
					log.Infof("Alte Gesichter für Event %s gelöscht", eventData.ID)
				}
			}
			
			// Lösche die überzähligen Bildeinträge
			if err := p.db.Where("id IN ?", imageIDsToDelete).Delete(&models.Image{}).Error; err != nil {
				log.Warnf("Fehler beim Löschen alter Bilder: %v", err)
			} else {
				log.Infof("Alte Bilder für Event %s gelöscht (%d Stück)", eventData.ID, len(imageIDsToDelete))
			}
		}
	}

	// Früher: Limitierung auf max. 3 Bilder pro Event - jetzt entfernt
	// Wir verarbeiten alle Updates, um mehr Bilder in der UI zu haben

	// Metadaten für die Bildverarbeitung vorbereiten
	metadata := map[string]interface{}{
		"event_id":        eventData.ID,
		"camera":          eventData.Camera,
		"label":           eventData.Label,
		"score":           eventData.Score,
		"current_zones":   eventData.CurrentZones,
		"entered_zones":   eventData.EnteredZones,
		"update_number":   len(existingImages),
		"start_time":      eventData.GetStartTime().Format(time.RFC3339),
		"frame_time":      eventData.GetCurrentTime().Format(time.RFC3339),
		"source":          "frigate",
		"event_type":      "update",
	}

	// Dateiname für den Snapshot generieren
	baseFilename := p.frigateClient.GenerateFilename(eventData)
	filenameParts := strings.Split(baseFilename, ".")
	baseName := filenameParts[0]
	extension := ".jpg" // Immer .jpg als Standard-Erweiterung verwenden
	if len(filenameParts) > 1 {
		extension = "." + filenameParts[1]
	}
	// Sequenznummer basierend auf vorhandenen Bildern und sicherstellen, dass die Erweiterung vorhanden ist
	filename := fmt.Sprintf("%s_update%d%s", baseName, len(existingImages), extension)
	
	// Explizit prüfen, ob die Dateiendung vorhanden ist
	if !strings.HasSuffix(filename, ".jpg") {
		filename = filename + ".jpg"
	}
	
	// KORREKTUR: Der localPath MUSS ein "frigate/"-Präfix haben, damit der Browser die Bilder im richtigen Verzeichnis finden kann
	// Die Dateien werden im Unterverzeichnis "frigate" gespeichert, also muss auch der Pfad in der Datenbank so sein
	localPath := filepath.Join("frigate", filename)
	fullPath := filepath.Join(p.cfg.Server.SnapshotDir, "frigate", filename)

	// Versuche Snapshot über API herunterzuladen
	if err := p.frigateClient.DownloadSnapshot(snapshotURL, fullPath); err != nil {
		return fmt.Errorf("Fehler beim Herunterladen des Snapshots: %w", err)
	}
	
	// Nachfolgende Alternative kommentiert, bis ExtractSnapshotFromEvent implementiert ist
	/*
	// Bilddaten aus MQTT-Payload direkt in Datei schreiben
		log.Infof("Speichere Bilddaten aus MQTT-Payload in %s", fullPath)
		
		// Verzeichnis erstellen, falls es nicht existiert
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("Konnte Verzeichnis für Snapshot nicht erstellen: %v", err)
		}
		
		// Bild speichern
		if err := os.WriteFile(fullPath, imageData, 0644); err != nil {
			return fmt.Errorf("Konnte Bilddaten nicht in Datei speichern: %v", err)
		}
	*/

	// Bild zur Verarbeitung an den Worker-Pool übergeben
	// Dies stellt sicher, dass Karten erst nach der CompreFace-Verarbeitung erstellt werden
	processedImage, processErr := p.ProcessImage(ctx, fullPath, "frigate", ProcessingOptions{
		DetectFaces:    true,
		RecognizeFaces: true,
		Metadata:       metadata,
	})
	
	if processErr != nil {
		return fmt.Errorf("Fehler bei der Bildverarbeitung: %w", processErr)
	} else if processedImage == nil {
		// Wenn OpenCV aktiviert ist und keine Person erkannt wurde, wird nil zurückgegeben
		// Das ist kein Fehler, sondern ein normaler Filterungsfall
		log.Infof("Frigate Update-Event-Bild übersprungen (keine Person erkannt): %s", localPath)
		return nil
	}

	log.Infof("Frigate Update-Event-Bild verarbeitet: %s", localPath)
	return nil
}

// SetHomeAssistantPublisher setzt den Home Assistant Publisher für den ImageProcessor
func (p *ImageProcessor) SetHomeAssistantPublisher(publisher *homeassistant.Publisher) {
	p.haPublisher = publisher
}

// GetHomeAssistantPublisher gibt den Home Assistant Publisher zurück
func (p *ImageProcessor) GetHomeAssistantPublisher() *homeassistant.Publisher {
	return p.haPublisher
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
