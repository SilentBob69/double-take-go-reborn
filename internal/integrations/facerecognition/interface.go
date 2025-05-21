package facerecognition

import (
	"context"
	"image"
	"time"
)

// ProviderType definiert den Typ des Gesichtserkennungsdiensts
type ProviderType string

const (
	// ProviderCompreFace steht für den CompreFace-Dienst
	ProviderCompreFace ProviderType = "compreface"
	
	// ProviderInsightFace steht für den InsightFace-Dienst
	ProviderInsightFace ProviderType = "insightface"
)

// Face repräsentiert ein erkanntes Gesicht
type Face struct {
	// BoundingBox enthält die Koordinaten des Gesichts im Bild (x1, y1, x2, y2)
	BoundingBox []int `json:"bounding_box"`
	
	// Confidence ist die Konfidenz der Gesichtserkennung (0-1)
	Confidence float64 `json:"confidence"`
	
	// Embedding ist der Gesichtsvektor für die Gesichtserkennung
	Embedding []float32 `json:"embedding,omitempty"`
	
	// FaceImage enthält optional das zugeschnittene Gesichtsbild als Base64-String
	FaceImage string `json:"face_image,omitempty"`
}

// Match repräsentiert eine Übereinstimmung mit einem bekannten Gesicht
type Match struct {
	// SubjectID ist die ID des erkannten Subjekts
	SubjectID string `json:"subject_id"`
	
	// Similarity ist die Ähnlichkeit (0-1), wobei 1 eine perfekte Übereinstimmung ist
	Similarity float64 `json:"similarity"`
}

// SubjectInfo enthält Informationen zu einem registrierten Subjekt/Person
type SubjectInfo struct {
	// ID ist die eindeutige Kennung des Subjekts
	ID string `json:"id"`
	
	// Name ist der Name des Subjekts (falls vorhanden)
	Name string `json:"name,omitempty"`
	
	// FaceCount ist die Anzahl der gespeicherten Gesichter für dieses Subjekt
	FaceCount int `json:"face_count"`
	
	// CreatedAt ist der Zeitpunkt der Erstellung
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// DetectionRequest enthält Parameter für die Gesichtserkennung
type DetectionRequest struct {
	// MinFaceSize ist die minimale Größe eines zu erkennenden Gesichts
	MinFaceSize int `json:"min_face_size,omitempty"`
	
	// ReturnFaceData gibt an, ob Gesichtsdaten zurückgegeben werden sollen
	ReturnFaceData bool `json:"return_face_data,omitempty"`
	
	// ExtractEmbedding gibt an, ob Gesichtseinbettungen extrahiert werden sollen
	ExtractEmbedding bool `json:"extract_embedding,omitempty"`
}

// RecognitionRequest enthält Parameter für die Gesichtserkennung
type RecognitionRequest struct {
	// DetectionRequest enthält Parameter für die Gesichtserkennung
	DetectionRequest
	
	// Limit begrenzt die Anzahl der zurückgegebenen Übereinstimmungen
	Limit int `json:"limit,omitempty"`
	
	// Threshold ist der minimale Ähnlichkeitswert für eine Übereinstimmung
	Threshold float64 `json:"threshold,omitempty"`
}

// DetectionResponse enthält die Ergebnisse der Gesichtserkennung
type DetectionResponse struct {
	// Faces ist eine Liste der erkannten Gesichter
	Faces []Face `json:"faces"`
	
	// ExecutionTime ist die Verarbeitungszeit in Sekunden
	ExecutionTime float64 `json:"execution_time,omitempty"`
}

// RecognitionResponse enthält die Ergebnisse der Gesichtserkennung
type RecognitionResponse struct {
	// Faces ist eine Liste der erkannten Gesichter
	Faces []Face `json:"faces"`
	
	// Matches ist eine Liste von Übereinstimmungen für jedes erkannte Gesicht
	// Die äußere Liste entspricht den Gesichtern, die innere den Übereinstimmungen pro Gesicht
	Matches [][]Match `json:"matches"`
	
	// ExecutionTime ist die Verarbeitungszeit in Sekunden
	ExecutionTime float64 `json:"execution_time,omitempty"`
}

// AddFaceRequest enthält Parameter für das Hinzufügen eines Gesichts
type AddFaceRequest struct {
	// SubjectID ist die ID des Subjekts, zu dem das Gesicht hinzugefügt werden soll
	SubjectID string `json:"subject_id"`
	
	// DetectionRequest enthält Parameter für die Gesichtserkennung
	DetectionRequest
}

// AddFaceResponse enthält das Ergebnis des Hinzufügens eines Gesichts
type AddFaceResponse struct {
	// FaceID ist die ID des hinzugefügten Gesichts
	FaceID string `json:"face_id,omitempty"`
	
	// Success gibt an, ob das Hinzufügen erfolgreich war
	Success bool `json:"success"`
	
	// ErrorMessage enthält eine Fehlermeldung, falls das Hinzufügen fehlgeschlagen ist
	ErrorMessage string `json:"error_message,omitempty"`
}

// Provider definiert die Schnittstelle für Gesichtserkennungsdienste
type Provider interface {
	// GetProviderName gibt den Namen des Providers zurück
	GetProviderName() ProviderType
	
	// IsAvailable prüft, ob der Dienst verfügbar ist
	IsAvailable(ctx context.Context) bool
	
	// DetectFaces erkennt Gesichter in einem Bild
	DetectFaces(ctx context.Context, img image.Image, opts DetectionRequest) (*DetectionResponse, error)
	
	// RecognizeFaces erkennt Gesichter und vergleicht sie mit bekannten Gesichtern
	RecognizeFaces(ctx context.Context, img image.Image, opts RecognitionRequest) (*RecognitionResponse, error)
	
	// AddFace fügt ein Gesicht zu einer Sammlung hinzu
	AddFace(ctx context.Context, img image.Image, opts AddFaceRequest) (*AddFaceResponse, error)
	
	// GetSubjects gibt eine Liste aller Subjekte zurück
	GetSubjects(ctx context.Context) ([]SubjectInfo, error)
	
	// DeleteSubject löscht ein Subjekt
	DeleteSubject(ctx context.Context, subjectID string) error
}

// ProviderManager verwaltet verschiedene Gesichtserkennungsdienste
type ProviderManager struct {
	providers map[ProviderType]Provider
	active    ProviderType
}

// NewProviderManager erstellt einen neuen ProviderManager für Gesichtserkennungsdienste
func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		providers: make(map[ProviderType]Provider),
	}
}

// RegisterProvider registriert einen neuen Gesichtserkennungsdienst
func (m *ProviderManager) RegisterProvider(provider Provider) {
	m.providers[provider.GetProviderName()] = provider
}

// SetActiveProvider setzt den aktiven Gesichtserkennungsdienst
func (m *ProviderManager) SetActiveProvider(providerType ProviderType) bool {
	if _, exists := m.providers[providerType]; exists {
		m.active = providerType
		return true
	}
	return false
}

// GetActiveProviderName gibt den Namen des aktuell aktiven Gesichtserkennungsdiensts zurück
func (m *ProviderManager) GetActiveProviderName() ProviderType {
	return m.active
}

// GetProvider gibt den Gesichtserkennungsdienst mit dem angegebenen Namen zurück
func (m *ProviderManager) GetProvider(providerType ProviderType) (Provider, bool) {
	provider, exists := m.providers[providerType]
	return provider, exists
}

// GetActiveProvider gibt den aktuell aktiven Gesichtserkennungsdienst zurück
func (m *ProviderManager) GetActiveProvider() (Provider, bool) {
	if m.active == "" {
		return nil, false
	}
	return m.GetProvider(m.active)
}

// GetAvailableProviders gibt eine Liste aller verfügbaren Gesichtserkennungsdienste zurück
func (m *ProviderManager) GetAvailableProviders(ctx context.Context) []ProviderType {
	var available []ProviderType
	for name, provider := range m.providers {
		if provider.IsAvailable(ctx) {
			available = append(available, name)
		}
	}
	return available
}
