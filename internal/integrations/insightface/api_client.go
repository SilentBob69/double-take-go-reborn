package insightface

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"double-take-go-reborn/config"

	log "github.com/sirupsen/logrus"
)

// Log-Felder für InsightFace-Komponente definieren
var logFields = log.Fields{
	"component": "insightface",
}

// APIClient implementiert die Kommunikation mit dem InsightFace-Dienst
type APIClient struct {
	config     config.InsightFaceConfig
	httpClient *http.Client
}

// apiInfoResponse enthält Informationen über den InsightFace-Dienst
type apiInfoResponse struct {
	Status    string   `json:"status"`
	Version   string   `json:"version"`
	Backend   string   `json:"backend"`
	Providers []string `json:"providers"`
}

// apiDetectResponse enthält die Antwort auf eine Gesichtserkennungsanfrage
type apiDetectResponse struct {
	Status     string `json:"status"`
	FacesCount int    `json:"faces_count"`
	Faces      []struct {
		BoundingBox []int     `json:"bbox"`
		Confidence  float64   `json:"confidence"`
		Embedding   []float32 `json:"embedding,omitempty"`
		FaceData    string    `json:"face_data,omitempty"`
	} `json:"faces"`
	ProcessTime float64 `json:"process_time"`
}

// NewAPIClient erstellt einen neuen InsightFace-APIClient
func NewAPIClient(config config.InsightFaceConfig) *APIClient {
	return &APIClient{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

// Ping prüft, ob der InsightFace-Dienst verfügbar ist
func (c *APIClient) Ping(ctx context.Context) (bool, error) {
	if !c.config.Enabled {
		return false, fmt.Errorf("InsightFace ist nicht aktiviert")
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/info", c.config.URL), nil)
	if err != nil {
		return false, fmt.Errorf("fehler beim Erstellen der Anfrage: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("fehler bei der Verbindung zu InsightFace: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("InsightFace-Dienst ist nicht verfügbar, Status: %d", resp.StatusCode)
	}
	
	var info apiInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return false, fmt.Errorf("fehler beim Dekodieren der Antwort: %w", err)
	}
	
	return info.Status == "ok", nil
}

// encodeImage kodiert ein Bild im JPEG-Format für die Übertragung
func encodeImage(img image.Image) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, nil)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DetectFaces sendet eine Anfrage zur Gesichtserkennung an den InsightFace-Dienst
func (c *APIClient) DetectFaces(ctx context.Context, img image.Image, threshold float64, returnFaceData bool, extractEmbedding bool) (*apiDetectResponse, error) {
	// Bild für die Übertragung vorbereiten
	imgData, err := encodeImage(img)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Kodieren des Bildes: %w", err)
	}
	
	// Multipart-Form vorbereiten
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	// Bild zum Formular hinzufügen
	part, err := writer.CreateFormFile("file", "image.jpg")
	if err != nil {
		return nil, fmt.Errorf("fehler beim Erstellen des Formularfeldes: %w", err)
	}
	
	if _, err := io.Copy(part, bytes.NewReader(imgData)); err != nil {
		return nil, fmt.Errorf("fehler beim Kopieren der Bilddaten: %w", err)
	}
	
	// Parameter zum Formular hinzufügen
	if err := writer.WriteField("threshold", fmt.Sprintf("%f", threshold)); err != nil {
		return nil, fmt.Errorf("fehler beim Schreiben von threshold: %w", err)
	}
	
	if err := writer.WriteField("return_face_data", fmt.Sprintf("%t", returnFaceData)); err != nil {
		return nil, fmt.Errorf("fehler beim Schreiben von return_face_data: %w", err)
	}
	
	if err := writer.WriteField("extract_embedding", fmt.Sprintf("%t", extractEmbedding)); err != nil {
		return nil, fmt.Errorf("fehler beim Schreiben von extract_embedding: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("fehler beim Schließen des Formularschreibers: %w", err)
	}
	
	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/detect", c.config.URL), body)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Erstellen der Anfrage: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Anfrage senden
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fehler bei der HTTP-Anfrage: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unerwarteter Status: %d, Antwort: %s", resp.StatusCode, string(bodyBytes))
	}
	
	// Antwort auswerten
	var apiResp apiDetectResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("fehler beim Dekodieren der Antwort: %w", err)
	}
	
	if apiResp.Status != "ok" {
		return nil, fmt.Errorf("API-Fehler: %s", apiResp.Status)
	}
	
	return &apiResp, nil
}
