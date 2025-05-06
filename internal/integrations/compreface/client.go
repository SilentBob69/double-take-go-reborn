package compreface

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Client für CompreFace-API
type Client struct {
	config     config.CompreFaceConfig
	httpClient *http.Client
}

// Box repräsentiert die Begrenzungsbox eines Gesichts
type Box struct {
	Probability float64 `json:"probability"`
	XMin        int     `json:"x_min"`
	YMin        int     `json:"y_min"`
	XMax        int     `json:"x_max"`
	YMax        int     `json:"y_max"`
}

// Subject repräsentiert eine erkannte Person
type Subject struct {
	Subject    string  `json:"subject"`
	Similarity float64 `json:"similarity"`
}

// RecognitionResult repräsentiert ein erkanntes Gesicht
type RecognitionResult struct {
	Box      Box       `json:"box"`
	Subjects []Subject `json:"subjects"`
}

// RecognitionResponse repräsentiert die Antwort der CompreFace-API
type RecognitionResponse struct {
	Result []RecognitionResult `json:"result"`
}

// AddResponse repräsentiert die Antwort beim Hinzufügen eines Beispiels
type AddResponse struct {
	ImageID string `json:"image_id"`
	Subject string `json:"subject"`
}

// NewClient erstellt einen neuen CompreFace-Client
func NewClient(cfg config.CompreFaceConfig) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Ping prüft, ob der CompreFace-Dienst erreichbar ist
func (c *Client) Ping(ctx context.Context) (bool, error) {
	if !c.config.Enabled {
		return false, fmt.Errorf("CompreFace is not enabled in config")
	}

	// Zuerst API-URL erstellen
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects")
	if err != nil {
		return false, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// API-Key hinzufügen
	req.Header.Set("x-api-key", c.config.RecognitionAPIKey)

	// Request senden
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// Recognize sendet ein Bild zur Gesichtserkennung an CompreFace
func (c *Client) Recognize(ctx context.Context, imageData []byte, filename string) (*RecognitionResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	log.Debugf("Sending image to CompreFace recognition: %s", filename)

	// Multipart-Form-Daten erstellen
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Bildteil hinzufügen
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(imageData)); err != nil {
		return nil, fmt.Errorf("failed to copy image data: %w", err)
	}

	// Erkennungsparameter hinzufügen
	if err := writer.WriteField("det_prob_threshold", fmt.Sprintf("%f", c.config.DetProbThreshold)); err != nil {
		return nil, fmt.Errorf("failed to add threshold field: %w", err)
	}

	// Multipart-Form abschließen
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Ziel-URL erstellen
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/recognize")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Header setzen
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-key", c.config.RecognitionAPIKey)

	// Request senden
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(startTime)
	
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	log.Debugf("CompreFace recognition request took %s", duration)

	// Antwort prüfen
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CompreFace API returned error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Antwort auswerten
	var result RecognitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Debugf("CompreFace detected %d faces", len(result.Result))
	return &result, nil
}

// GetAllSubjects ruft alle bekannten Subjekte/Personen von CompreFace ab
func (c *Client) GetAllSubjects(ctx context.Context) ([]string, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL erstellen
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// API-Key hinzufügen
	req.Header.Set("x-api-key", c.config.RecognitionAPIKey)

	// Request senden
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Status-Code prüfen
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CompreFace API returned error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Antwort auswerten
	var result struct {
		Subjects []string `json:"subjects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Subjects, nil
}

// SyncIdentities synchronisiert die Identitäten zwischen CompreFace und der lokalen Datenbank
func (c *Client) SyncIdentities(ctx context.Context, db *gorm.DB) error {
	if !c.config.Enabled {
		return fmt.Errorf("CompreFace is not enabled in config")
	}

	// Alle Subjekte von CompreFace abrufen
	compreSubjects, err := c.GetAllSubjects(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to get subjects from CompreFace")
		return fmt.Errorf("failed to get subjects from CompreFace: %w", err)
	}

	// Map für schnellen Zugriff erstellen
	compreSubjectMap := make(map[string]bool)
	for _, name := range compreSubjects {
		compreSubjectMap[strings.ToLower(name)] = true // Kleinbuchstaben für Vergleich
	}

	// Alle lokalen Identitäten abrufen
	var localIdentities []models.Identity
	if err := db.Find(&localIdentities).Error; err != nil {
		log.WithError(err).Error("Failed to get identities from local database")
		return fmt.Errorf("failed to get local identities: %w", err)
	}

	// Map für schnellen Zugriff erstellen
	localIdentityMap := make(map[string]bool)
	for _, identity := range localIdentities {
		localIdentityMap[strings.ToLower(identity.Name)] = true // Kleinbuchstaben für Vergleich
	}

	// Überprüfen, welche Subjekte in CompreFace sind, aber nicht lokal
	newIdentitiesCount := 0
	for _, compreName := range compreSubjects {
		if !localIdentityMap[strings.ToLower(compreName)] {
			// Subjekt in CompreFace, aber nicht lokal -> erstellen
			newIdentity := models.Identity{
				Name:       compreName,
				ExternalID: compreName, // ExternalID = Name in CompreFace
			}
			if err := db.Create(&newIdentity).Error; err != nil {
				log.WithError(err).Errorf("Failed to create local identity for CompreFace subject: %s", compreName)
				// Trotz Fehler fortsetzen
			} else {
				newIdentitiesCount++
				log.Infof("Created new local identity for CompreFace subject: %s", compreName)
			}
		}
	}

	log.Infof("CompreFace sync completed: %d subjects in CompreFace, created %d new local identities",
		len(compreSubjects), newIdentitiesCount)
	return nil
}

// AddSubjectExample fügt ein Beispielbild für ein Subjekt/eine Person hinzu
func (c *Client) AddSubjectExample(ctx context.Context, subjectName string, imageData []byte, filename string) (*AddResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// Multipart-Form-Daten erstellen
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Bildteil hinzufügen
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(imageData)); err != nil {
		return nil, fmt.Errorf("failed to copy image data: %w", err)
	}

	// Multipart-Form abschließen
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Ziel-URL mit Subjektnamen erstellen
	baseURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/faces")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Query-Parameter hinzufügen
	q := u.Query()
	q.Set("subject", subjectName)
	u.RawQuery = q.Encode()

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Header setzen
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-key", c.config.RecognitionAPIKey)

	// Request senden
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Status-Code prüfen
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CompreFace API returned error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Antwort auswerten
	var result AddResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Added example for subject %s with image ID %s", result.Subject, result.ImageID)
	return &result, nil
}
