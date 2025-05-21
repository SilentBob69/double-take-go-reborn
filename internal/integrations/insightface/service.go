package insightface

import (
	"context"
	"fmt"
	"image"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/integrations/facerecognition"
)

// Service implementiert das facerecognition.Provider-Interface für InsightFace
type Service struct {
	client     *APIClient
	config     config.InsightFaceConfig
	subjectCache []facerecognition.SubjectInfo
	cacheTime    time.Time
}

// NewService erstellt einen neuen InsightFace-Service
func NewService(cfg config.InsightFaceConfig) *Service {
	return &Service{
		client: NewAPIClient(cfg),
		config: cfg,
	}
}

// GetProviderName gibt den Namen des Providers zurück
func (s *Service) GetProviderName() facerecognition.ProviderType {
	return facerecognition.ProviderInsightFace
}

// IsAvailable prüft, ob der InsightFace-Dienst verfügbar ist
func (s *Service) IsAvailable(ctx context.Context) bool {
	if !s.config.Enabled {
		return false
	}
	
	available, _ := s.client.Ping(ctx)
	return available
}

// DetectFaces erkennt Gesichter in einem Bild
func (s *Service) DetectFaces(ctx context.Context, img image.Image, opts facerecognition.DetectionRequest) (*facerecognition.DetectionResponse, error) {
	startTime := time.Now()
	
	// Anfrage an den API-Client senden
	apiResp, err := s.client.DetectFaces(
		ctx, 
		img, 
		s.config.DetectionThreshold, 
		opts.ReturnFaceData, 
		opts.ExtractEmbedding,
	)
	if err != nil {
		return nil, fmt.Errorf("fehler bei der Gesichtserkennung: %w", err)
	}
	
	// Antwort in unser generisches Format konvertieren
	result := &facerecognition.DetectionResponse{
		Faces:         make([]facerecognition.Face, len(apiResp.Faces)),
		ExecutionTime: time.Since(startTime).Seconds(),
	}
	
	for i, face := range apiResp.Faces {
		result.Faces[i] = facerecognition.Face{
			BoundingBox: face.BoundingBox,
			Confidence:  face.Confidence,
			Embedding:   face.Embedding,
			FaceImage:   face.FaceData,
		}
	}
	
	return result, nil
}

// RecognizeFaces erkennt Gesichter und vergleicht sie mit bekannten Gesichtern
// Hinweis: InsightFace hat im Standard keinen eigenen Gesichtserkennung/Abgleich,
// daher ist dies eine Implementierung, die nur die Gesichtserkennung durchführt.
// In einer vollständigen Implementierung würde man hier die Erkennung mit einer Datenbank abgleichen.
func (s *Service) RecognizeFaces(ctx context.Context, img image.Image, opts facerecognition.RecognitionRequest) (*facerecognition.RecognitionResponse, error) {
	// Gesichter erkennen
	detectResp, err := s.DetectFaces(ctx, img, opts.DetectionRequest)
	if err != nil {
		return nil, err
	}
	
	// Da InsightFace-API keine direkte Gesichtserkennung bietet,
	// geben wir nur die erkannten Gesichter zurück, ohne Übereinstimmungen
	result := &facerecognition.RecognitionResponse{
		Faces:         detectResp.Faces,
		Matches:       make([][]facerecognition.Match, len(detectResp.Faces)),
		ExecutionTime: detectResp.ExecutionTime,
	}
	
	return result, nil
}

// AddFace ist eine Stub-Implementierung für das Hinzufügen eines Gesichts
// Da InsightFace in der Basis-Implementierung keine Gesichtsdatenbank hat,
// würde dies eine eigene Implementierung erfordern
func (s *Service) AddFace(ctx context.Context, img image.Image, opts facerecognition.AddFaceRequest) (*facerecognition.AddFaceResponse, error) {
	return &facerecognition.AddFaceResponse{
		Success:      false,
		ErrorMessage: "Funktion nicht implementiert: InsightFace unterstützt in der Basisversion keine Gesichtsregistrierung",
	}, nil
}

// GetSubjects ist eine Stub-Implementierung für das Abrufen von Subjekten
// Da InsightFace in der Basis-Implementierung keine Gesichtsdatenbank hat,
// würde dies eine eigene Implementierung erfordern
func (s *Service) GetSubjects(ctx context.Context) ([]facerecognition.SubjectInfo, error) {
	return []facerecognition.SubjectInfo{}, nil
}

// DeleteSubject ist eine Stub-Implementierung für das Löschen eines Subjekts
// Da InsightFace in der Basis-Implementierung keine Gesichtsdatenbank hat,
// würde dies eine eigene Implementierung erfordern
func (s *Service) DeleteSubject(ctx context.Context, subjectID string) error {
	return fmt.Errorf("funktion nicht implementiert: InsightFace unterstützt in der Basisversion keine Gesichtsregistrierung")
}
