package provider

// Temporäre Datei mit einer vereinfachten Implementierung der fehlenden Schnittstellen
// Diese könnte später in die richtigen Dateien integriert werden

import (
	"context"
	"double-take-go-reborn/internal/integrations/facerecognition"
	"image"
)

// RecognizeProvider ist eine vereinfachte Schnittstelle für Gesichtserkennung
type RecognizeProvider interface {
	Recognize(ctx context.Context, img image.Image, request RecognizeRequest) (*RecognizeResponse, error)
}

// RecognizeRequest stellt die Anfrage für die Gesichtserkennung dar
type RecognizeRequest struct {
	DetectionThreshold   float64
	RecognitionThreshold float64
}

// RecognizeResponse stellt die Antwort für die Gesichtserkennung dar
type RecognizeResponse struct {
	Faces []facerecognition.Face
}

// RecognizeFacesAdapter wandelt einen RecognizeFaces-Aufruf in einen Recognize-Aufruf um
func RecognizeFacesAdapter(ctx context.Context, provider facerecognition.Provider, img image.Image, req facerecognition.RecognitionRequest) (*facerecognition.RecognitionResponse, error) {
	// Einfacher Adapter, der die facerecognition.RecognizeFaces-Methode verwendet
	return provider.RecognizeFaces(ctx, img, req)
}
