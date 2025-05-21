package compreface

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"bytes"
	"time"

	"double-take-go-reborn/internal/integrations/facerecognition"
	"double-take-go-reborn/config"
)

// Service implementiert das facerecognition.Provider-Interface mit dem CompreFace-APIClient
type Service struct {
	client *APIClient
	config config.CompreFaceConfig
}

// NewService erstellt einen neuen CompreFace-Service
func NewService(cfg config.CompreFaceConfig) *Service {
	return &Service{
		client: NewAPIClient(cfg),
		config: cfg,
	}
}

// GetProviderName gibt den Namen des Providers zurück
func (s *Service) GetProviderName() facerecognition.ProviderType {
	return facerecognition.ProviderCompreFace
}

// IsAvailable prüft, ob der CompreFace-Dienst verfügbar ist
func (s *Service) IsAvailable(ctx context.Context) bool {
	if !s.config.Enabled {
		return false
	}
	
	available, _ := s.client.Ping(ctx)
	return available
}

// imageToBytes konvertiert ein Image zu einem Byte-Array im JPEG-Format
func imageToBytes(img image.Image) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, nil)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DetectFaces erkennt Gesichter in einem Bild
func (s *Service) DetectFaces(ctx context.Context, img image.Image, opts facerecognition.DetectionRequest) (*facerecognition.DetectionResponse, error) {
	startTime := time.Now()
	
	// Bild in Bytes umwandeln
	imgData, err := imageToBytes(img)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Konvertieren des Bildes: %w", err)
	}
	
	// Erkennung durchführen (verwendet die Recognize-Funktion, ignoriert aber die Erkennung)
	resp, err := s.client.Recognize(ctx, imgData, "detection.jpg")
	if err != nil {
		return nil, fmt.Errorf("fehler bei der Gesichtserkennung: %w", err)
	}
	
	// Ergebnis in unser generisches Format konvertieren
	result := &facerecognition.DetectionResponse{
		Faces:         make([]facerecognition.Face, 0, len(resp.Result)),
		ExecutionTime: time.Since(startTime).Seconds(),
	}
	
	for _, r := range resp.Result {
		// Bounding-Box konvertieren
		bbox := []int{r.Box.XMin, r.Box.YMin, r.Box.XMax, r.Box.YMax}
		
		// CompreFace liefert kein Embedding, wir können es hier nicht extrahieren
		face := facerecognition.Face{
			BoundingBox: bbox,
			Confidence:  r.Box.Probability,
		}
		
		// Optional: Wenn angefordert, extrahieren wir das Gesichtsbild
		if opts.ReturnFaceData {
			// Gesicht aus dem Originalbild ausschneiden
			subImg := img.(interface {
				SubImage(r image.Rectangle) image.Image
			}).SubImage(image.Rect(r.Box.XMin, r.Box.YMin, r.Box.XMax, r.Box.YMax))
			
			// In Base64 konvertieren
			faceBytes, err := imageToBytes(subImg)
			if err == nil {
				face.FaceImage = base64.StdEncoding.EncodeToString(faceBytes)
			}
		}
		
		result.Faces = append(result.Faces, face)
	}
	
	return result, nil
}

// RecognizeFaces erkennt Gesichter und vergleicht sie mit bekannten Gesichtern
func (s *Service) RecognizeFaces(ctx context.Context, img image.Image, opts facerecognition.RecognitionRequest) (*facerecognition.RecognitionResponse, error) {
	startTime := time.Now()
	
	// Bild in Bytes umwandeln
	imgData, err := imageToBytes(img)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Konvertieren des Bildes: %w", err)
	}
	
	// Erkennung durchführen
	resp, err := s.client.Recognize(ctx, imgData, "recognition.jpg")
	if err != nil {
		return nil, fmt.Errorf("fehler bei der Gesichtserkennung: %w", err)
	}
	
	// Ergebnis in unser generisches Format konvertieren
	result := &facerecognition.RecognitionResponse{
		Faces:         make([]facerecognition.Face, 0, len(resp.Result)),
		Matches:       make([][]facerecognition.Match, 0, len(resp.Result)),
		ExecutionTime: time.Since(startTime).Seconds(),
	}
	
	for _, r := range resp.Result {
		// Bounding-Box konvertieren
		bbox := []int{r.Box.XMin, r.Box.YMin, r.Box.XMax, r.Box.YMax}
		
		// CompreFace liefert kein Embedding, wir können es hier nicht extrahieren
		face := facerecognition.Face{
			BoundingBox: bbox,
			Confidence:  r.Box.Probability,
		}
		
		// Optional: Wenn angefordert, extrahieren wir das Gesichtsbild
		if opts.ReturnFaceData {
			// Gesicht aus dem Originalbild ausschneiden
			subImg := img.(interface {
				SubImage(r image.Rectangle) image.Image
			}).SubImage(image.Rect(r.Box.XMin, r.Box.YMin, r.Box.XMax, r.Box.YMax))
			
			// In Base64 konvertieren
			faceBytes, err := imageToBytes(subImg)
			if err == nil {
				face.FaceImage = base64.StdEncoding.EncodeToString(faceBytes)
			}
		}
		
		// Matches verarbeiten
		matches := make([]facerecognition.Match, 0, len(r.Subjects))
		for _, subject := range r.Subjects {
			// Threshold-Filter anwenden, falls gesetzt
			if opts.Threshold > 0 && subject.Similarity < opts.Threshold {
				continue
			}
			
			match := facerecognition.Match{
				SubjectID:  subject.Subject,
				Similarity: subject.Similarity,
			}
			matches = append(matches, match)
		}
		
		// Limit anwenden, falls gesetzt
		if opts.Limit > 0 && len(matches) > opts.Limit {
			matches = matches[:opts.Limit]
		}
		
		result.Faces = append(result.Faces, face)
		result.Matches = append(result.Matches, matches)
	}
	
	return result, nil
}

// AddFace fügt ein Gesicht zu einer Sammlung hinzu
func (s *Service) AddFace(ctx context.Context, img image.Image, opts facerecognition.AddFaceRequest) (*facerecognition.AddFaceResponse, error) {
	// Bild in Bytes umwandeln
	imgData, err := imageToBytes(img)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Konvertieren des Bildes: %w", err)
	}
	
	// Gesicht hinzufügen
	resp, err := s.client.AddExample(ctx, imgData, "example.jpg", opts.SubjectID)
	if err != nil {
		return &facerecognition.AddFaceResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	
	return &facerecognition.AddFaceResponse{
		FaceID:  resp.ImageID,
		Success: true,
	}, nil
}

// GetSubjects gibt eine Liste aller Subjekte zurück
func (s *Service) GetSubjects(ctx context.Context) ([]facerecognition.SubjectInfo, error) {
	subjects, err := s.client.GetAllSubjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Abrufen der Subjekte: %w", err)
	}
	
	result := make([]facerecognition.SubjectInfo, 0, len(subjects))
	
	// Für jedes Subjekt die Beispielbilder abfragen, um die Anzahl zu ermitteln
	for _, subjectName := range subjects {
		examples, err := s.client.GetSubjectExamples(ctx, subjectName)
		faceCount := 0
		if err == nil {
			faceCount = len(examples)
		}
		
		subjectInfo := facerecognition.SubjectInfo{
			ID:        subjectName,
			Name:      subjectName, // CompreFace verwendet die ID auch als Namen
			FaceCount: faceCount,
		}
		
		result = append(result, subjectInfo)
	}
	
	return result, nil
}

// DeleteSubject löscht ein Subjekt
func (s *Service) DeleteSubject(ctx context.Context, subjectID string) error {
	_, err := s.client.DeleteSubject(ctx, subjectID)
	if err != nil {
		return fmt.Errorf("fehler beim Löschen des Subjekts: %w", err)
	}
	
	return nil
}
