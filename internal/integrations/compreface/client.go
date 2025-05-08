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

// SubjectResponse repräsentiert die Antwort beim Erstellen/Löschen eines Subjekts
type SubjectResponse struct {
	Subject string `json:"subject"`
}

// SubjectRenameResponse repräsentiert die Antwort beim Umbenennen eines Subjekts
type SubjectRenameResponse struct {
	Updated bool `json:"updated"`
}

// DeleteAllResponse repräsentiert die Antwort beim Löschen aller Subjekte
type DeleteAllResponse struct {
	Deleted int `json:"deleted"`
}

// SubjectExampleResponse repräsentiert ein Beispielbild für ein Subjekt
type SubjectExampleResponse struct {
	ImageID string `json:"image_id"`
	Subject string `json:"subject"`
}

// SubjectExamplesWrapper repräsentiert die Antwort der CompreFace API für Beispielbilder
type SubjectExamplesWrapper struct {
	Faces []SubjectExampleResponse `json:"faces"`
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

	// URL genau nach der Dokumentation erstellen
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects/")
	if err != nil {
		return false, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Headers genau nach der Dokumentation setzen
	req.Header.Set("x-api-key", c.config.RecognitionAPIKey)
	req.Header.Set("Content-Type", "application/json")

	log.Debugf("Testing CompreFace connection at: %s", apiURL)

	// Request senden
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Wenn wir einen 200 OK Status zurückbekommen, ist die Verbindung erfolgreich
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	// Bei anderen Status-Codes loggen wir den Fehler mit mehr Details
	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Warnf("CompreFace connection test failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	
	return false, nil
}

// Recognize sendet ein Bild zur Gesichtserkennung an CompreFace mit optimierten Parametern
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

	// Bilddaten in das Formular kopieren
	if _, err := part.Write(imageData); err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}

	// Zusätzliche Parameter für verbesserte Erkennung hinzufügen
	
	// Parameter 1: Limit - Maximum der zurückgegebenen Treffer pro Gesicht (default: 5)
	limit := "10"  // Erhöhen auf 10, um mehr potenzielle Treffer zu erhalten
	if err := writer.WriteField("limit", limit); err != nil {
		log.Warnf("Failed to add limit parameter: %v", err)
	}

	// Parameter 2: der Schwellenwert für die Ähnlichkeit (default: 0.8 bzw. 80%)
	// Wir verwenden den konfigurierten Wert, aber konvertieren von 0-100 zu 0-1
	similarityThreshold := fmt.Sprintf("%.2f", c.config.SimilarityThreshold/100.0)
	if err := writer.WriteField("threshold", similarityThreshold); err != nil {
		log.Warnf("Failed to add threshold parameter: %v", err)
	}

	// Parameter 3: Face Plugins für zusätzliche Gesichtsmerkmale
	// age,gender,detector,calculator,mask,landmarks,pose,calculating
	// Wir aktivieren zusätzlich calculator (für 128-dim Embeddings) und landmarks (für Gesichtsmerkmale)
	if err := writer.WriteField("face_plugins", "calculator,landmarks"); err != nil {
		log.Warnf("Failed to add face_plugins parameter: %v", err)
	}

	// Parameter 4: Detection-Probability-Threshold
	// Dieser Wert bestimmt, ab welcher Wahrscheinlichkeit ein Gesicht erkannt wird
	detProbThreshold := fmt.Sprintf("%.2f", c.config.DetProbThreshold)
	if err := writer.WriteField("det_prob_threshold", detProbThreshold); err != nil {
		log.Warnf("Failed to add det_prob_threshold parameter: %v", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// URL für die Gesichtserkennung erstellen
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

	// Start der Zeitmessung
	start := time.Now()

	// Request senden
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	
	// Zeitmessung beenden
	duration := time.Since(start)
	log.Debugf("CompreFace recognition request took %s", duration)

	// Prüfen ob Request erfolgreich war
	if resp.StatusCode != http.StatusOK {
		// Bei Fehler den Response-Body lesen
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("CompreFace API returned error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	defer resp.Body.Close()

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

	// URL genau nach der Dokumentation erstellen
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects/")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Headers genau nach der Dokumentation setzen
	req.Header.Set("x-api-key", c.config.RecognitionAPIKey)
	req.Header.Set("Content-Type", "application/json")

	// Ausführliche Logging für Debugging-Zwecke
	log.Infof("Sending request to CompreFace: %s", apiURL)

	// Request senden
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Antwort auslesen (für Logging-Zwecke)
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Status-Code prüfen
	if resp.StatusCode != http.StatusOK {
		log.Errorf("CompreFace API error (status %d): %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("CompreFace API returned error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Antwort auswerten
	var result struct {
		Subjects []string `json:"subjects"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Successfully retrieved %d subjects from CompreFace", len(result.Subjects))
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

// CreateSubject erstellt ein neues Subjekt in CompreFace
func (c *Client) CreateSubject(ctx context.Context, subjectName string) (*SubjectResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für die Subjekt-Erstellung
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request-Body erstellen
	reqBody := map[string]string{"subject": subjectName}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Header setzen
	req.Header.Set("Content-Type", "application/json")
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
	var result SubjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Created new subject: %s", result.Subject)
	return &result, nil
}

// RenameSubject benennt ein bestehendes Subjekt um
// Wenn das Ziel-Subjekt bereits existiert, werden die Subjekte zusammengeführt
func (c *Client) RenameSubject(ctx context.Context, oldSubjectName, newSubjectName string) (*SubjectRenameResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für die Subjekt-Umbenennung
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects/", oldSubjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request-Body erstellen
	reqBody := map[string]string{"subject": newSubjectName}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Header setzen
	req.Header.Set("Content-Type", "application/json")
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
	var result SubjectRenameResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Renamed subject from '%s' to '%s': update status: %v", oldSubjectName, newSubjectName, result.Updated)
	return &result, nil
}

// DeleteSubject löscht ein Subjekt und alle zugehörigen Beispielbilder
func (c *Client) DeleteSubject(ctx context.Context, subjectName string) (*SubjectResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für das Löschen eines Subjekts
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects/", subjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Header setzen
	req.Header.Set("Content-Type", "application/json")
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
	var result SubjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Deleted subject: %s", result.Subject)
	return &result, nil
}

// DeleteAllSubjects löscht alle Subjekte und alle zugehörigen Beispielbilder
func (c *Client) DeleteAllSubjects(ctx context.Context) (*DeleteAllResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für das Löschen aller Subjekte
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Header setzen
	req.Header.Set("Content-Type", "application/json")
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
	var result DeleteAllResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Deleted all subjects, count: %d", result.Deleted)
	return &result, nil
}

// GetSubjectExamples gibt alle Beispielbilder eines Subjekts zurück
func (c *Client) GetSubjectExamples(ctx context.Context, subjectName string) ([]SubjectExampleResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für die Abfrage der Beispielbilder eines Subjekts
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/faces")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// URL-Parameter für das Subjekt hinzufügen
	apiURLWithParams, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API URL: %w", err)
	}

	q := apiURLWithParams.Query()
	q.Set("subject", subjectName)
	apiURLWithParams.RawQuery = q.Encode()

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "GET", apiURLWithParams.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Header setzen
	req.Header.Set("Content-Type", "application/json")
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
	var wrapper SubjectExamplesWrapper
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		// Versuchen wir, direkt die Rohresponse zu debuggen
		resp.Body.Close()
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Retrieved %d examples for subject: %s", len(wrapper.Faces), subjectName)
	return wrapper.Faces, nil
}

// DeleteSubjectExample löscht ein einzelnes Beispielbild eines Subjekts
func (c *Client) DeleteSubjectExample(ctx context.Context, imageID string) error {
	if !c.config.Enabled {
		return fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für das Löschen eines Beispielbilds
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/faces/", imageID)
	if err != nil {
		return fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Header setzen
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.RecognitionAPIKey)

	// Request senden
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Status-Code prüfen
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("CompreFace API returned error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	log.Infof("Deleted example image with ID: %s", imageID)
	return nil
}
