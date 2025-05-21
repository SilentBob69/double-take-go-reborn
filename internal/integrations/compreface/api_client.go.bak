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
	"strconv"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// APIClient für CompreFace-API
type APIClient struct {
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

// NewAPIClient erstellt einen neuen CompreFace-APIClient
func NewAPIClient(cfg config.CompreFaceConfig) *APIClient {
	return &APIClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Ping prüft, ob der CompreFace-Dienst erreichbar ist
func (c *APIClient) Ping(ctx context.Context) (bool, error) {
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
func (c *APIClient) Recognize(ctx context.Context, imageData []byte, filename string) (*RecognitionResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	log.Debugf("Sending image to CompreFace recognition: %s", filename)

	// Multipart-Form-Daten erstellen
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Formularfeld für das Bild erstellen
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	// Bilddaten in das Formularfeld schreiben
	if _, err := io.Copy(part, bytes.NewReader(imageData)); err != nil {
		return nil, fmt.Errorf("failed to copy image data: %w", err)
	}

	// Boundary abschließen
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// URL für die Erkennung erstellen
	// Die URL sollte so aussehen: {api_url}/api/v1/recognition/recognize
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/recognize")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Limit-Parameter hinzufügen (maximale Anzahl von Treffern pro Gesicht)
	apiURLWithParams, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API URL: %w", err)
	}

	q := apiURLWithParams.Query()
	q.Set("limit", "10") // Standardwert: 10 Treffer pro Gesicht
	q.Set("det_prob_threshold", fmt.Sprintf("%g", c.config.DetProbThreshold))
	q.Set("prediction_count", "3")
	// Ähnlichkeitsschwelle in Prozent (z.B. 80.0 für 80%)
	q.Set("face_plugins", "")
	apiURLWithParams.RawQuery = q.Encode()

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "POST", apiURLWithParams.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Content-Type-Header setzen (wichtig für Multipart-Form-Daten)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// API-Key-Header setzen
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
	var result RecognitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Debugf("CompreFace recognize successful, found %d faces", len(result.Result))
	return &result, nil
}

// SubjectsResponse repräsentiert die Antwort der CompreFace API für die Liste der Subjekte
type SubjectsResponse struct {
	Subjects []string `json:"subjects"`
}

// GetAllSubjects ruft alle bekannten Subjekte/Personen von CompreFace ab
func (c *APIClient) GetAllSubjects(ctx context.Context) ([]string, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für die Abfrage aller Subjekte
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
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

	// Das korrekte Format der Antwort ist: {"subjects": ["name1", "name2", ...]}
	// Daher müssen wir zuerst die Antwort in eine Struktur deserialisieren
	var response SubjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Retrieved %d subjects from CompreFace", len(response.Subjects))
	return response.Subjects, nil
}

// SyncIdentities synchronisiert die Identitäten zwischen CompreFace und der lokalen Datenbank
// Dabei werden Identitäten in beiden Richtungen synchronisiert:
// 1. Lokale Identitäten werden in CompreFace erstellt, wenn sie dort nicht existieren
// 2. CompreFace-Identitäten werden in die lokale Datenbank importiert, wenn sie dort nicht existieren
func (c *APIClient) SyncIdentities(ctx context.Context, db *gorm.DB) error {
	if !c.config.Enabled {
		return fmt.Errorf("CompreFace is not enabled in config")
	}

	log.Info("Starting identity synchronization with CompreFace")

	// CompreFace-Subjekte abrufen
	subjects, err := c.GetAllSubjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve subjects from CompreFace: %w", err)
	}

	// Subjekte in Map umwandeln für einfachen Zugriff
	subjectMap := make(map[string]bool)
	for _, subject := range subjects {
		subjectMap[subject] = true
	}

	// Bekannte Identitäten aus der Datenbank abrufen
	var identities []models.Identity
	if err := db.Find(&identities).Error; err != nil {
		return fmt.Errorf("failed to retrieve identities from database: %w", err)
	}

	// Identitäten in CompreFace erstellen, die nicht existieren
	for _, identity := range identities {
		// Verwende den Namen für CompreFace, statt der ID
		subjectName := identity.Name
		
		// Falls die Identität bereits eine externe ID hat, überprüfe diese
		var subjectToCheck string
		if identity.ExternalID != "" {
			subjectToCheck = identity.ExternalID
		} else {
			subjectToCheck = subjectName
		}
		
		if _, exists := subjectMap[subjectToCheck]; !exists {
			log.Infof("Creating subject in CompreFace: %s", subjectName)
			_, err := c.CreateSubject(ctx, subjectName)
			if err != nil {
				log.Warnf("Failed to create subject %s in CompreFace: %v", subjectName, err)
			} else {
				// Aktualisiere die ExternalID, wenn sie noch nicht gesetzt ist
				if identity.ExternalID == "" {
					identity.ExternalID = subjectName
					db.Save(&identity)
				}
			}
		}
	}

	// Identitätsmap für einfachen Zugriff erstellen
	identityMap := make(map[string]bool)
	externalIdMap := make(map[string]bool)
	
	for _, identity := range identities {
		// Namen und externe IDs merken
		identityMap[identity.Name] = true
		
		if identity.ExternalID != "" {
			externalIdMap[identity.ExternalID] = true
		}
	}

	// CompreFace-Subjekte in die lokale Datenbank importieren, wenn sie nicht existieren
	importedCount := 0
	for subject := range subjectMap {
		// Prüfen, ob wir das Subjekt bereits als Namen haben
		if _, exists := identityMap[subject]; exists {
			continue
		}
		
		// Prüfen, ob wir das Subjekt bereits als externe ID haben
		if _, exists := externalIdMap[subject]; exists {
			continue
		}
		
		// Wir verwenden den Subjektnamen direkt als Identitätsnamen
		name := subject
		
		// Folgende Prüfung ist hilfreich für bereits vorhandene numerische IDs,
		// damit diese einen lesbaren Namen bekommen
		_, err := strconv.ParseUint(subject, 10, 64)
		if err == nil {
			// Es ist eine Zahl, wir fügen ein Prefix hinzu
			name = fmt.Sprintf("Person %s", subject)
		}
		
		// Neue Identität in der lokalen Datenbank erstellen
		newIdentity := models.Identity{
			Name:       name,
			ExternalID: subject,
		}
		
		if err := db.Create(&newIdentity).Error; err != nil {
			log.Warnf("Failed to create identity for CompreFace subject %s: %v", subject, err)
		} else {
			log.Infof("Imported CompreFace subject %s as identity %s (ID: %d)", subject, name, newIdentity.ID)
			importedCount++
		}
	}

	log.Infof("Imported %d CompreFace subjects to local database", importedCount)
	log.Info("Identity synchronization with CompreFace completed")
	return nil
}

// AddExample fügt ein Beispielbild zu einem Subjekt hinzu
func (c *APIClient) AddExample(ctx context.Context, imageData []byte, filename, subjectID string) (*AddResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	log.Debugf("Adding face example for subject: %s", subjectID)

	// Multipart-Form-Daten erstellen
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Formularfeld für das Bild erstellen
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	// Bilddaten in das Formularfeld schreiben
	if _, err := io.Copy(part, bytes.NewReader(imageData)); err != nil {
		return nil, fmt.Errorf("failed to copy image data: %w", err)
	}

	// Subjekt-Feld hinzufügen
	if err := writer.WriteField("subject", subjectID); err != nil {
		return nil, fmt.Errorf("failed to add subject field: %w", err)
	}

	if err := writer.WriteField("det_prob_threshold", fmt.Sprintf("%g", c.config.DetProbThreshold)); err != nil {
		return nil, fmt.Errorf("failed to add det_prob_threshold field: %w", err)
	}

	// Boundary abschließen
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// URL für das Hinzufügen eines Beispiels erstellen
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/faces")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Content-Type-Header setzen (wichtig für Multipart-Form-Daten)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// API-Key-Header setzen
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
	var result AddResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Successfully added example for subject %s with image ID: %s", subjectID, result.ImageID)
	return &result, nil
}

// AddSubjectExample ist ein Alias für AddExample für Abwärtskompatibilität
func (c *APIClient) AddSubjectExample(ctx context.Context, subjectID string, imageData []byte, filename string) (*AddResponse, error) {
	// Rufe die eigentliche Implementierung mit korrekter Reihenfolge der Parameter auf
	return c.AddExample(ctx, imageData, filename, subjectID)
}

// CreateSubject erstellt ein neues Subjekt in CompreFace
func (c *APIClient) CreateSubject(ctx context.Context, subjectID string) (*SubjectResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für die Erstellung eines Subjekts
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects")
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request-Body erstellen
	requestBody := map[string]string{"subject": subjectID}
	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(requestBodyBytes))
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

	log.Infof("Created subject: %s", result.Subject)
	return &result, nil
}

// DeleteSubject löscht ein Subjekt in CompreFace
func (c *APIClient) DeleteSubject(ctx context.Context, subjectID string) (*SubjectResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für das Löschen eines Subjekts
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects/", subjectID)
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

// RenameSubject benennt ein Subjekt in CompreFace um
func (c *APIClient) RenameSubject(ctx context.Context, oldSubjectID, newSubjectID string) (*SubjectRenameResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für das Umbenennen eines Subjekts
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects/", oldSubjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create API URL: %w", err)
	}

	// Request-Body erstellen
	requestBody := map[string]string{"subject": newSubjectID}
	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Request erstellen
	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, bytes.NewBuffer(requestBodyBytes))
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

	log.Infof("Renamed subject from %s to %s, success: %v", oldSubjectID, newSubjectID, result.Updated)
	return &result, nil
}

// DeleteAllSubjects löscht alle Subjekte in CompreFace
func (c *APIClient) DeleteAllSubjects(ctx context.Context) (*DeleteAllResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CompreFace is not enabled in config")
	}

	// URL für das Löschen aller Subjekte
	apiURL, err := url.JoinPath(c.config.URL, "/api/v1/recognition/subjects/")
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
func (c *APIClient) GetSubjectExamples(ctx context.Context, subjectName string) ([]SubjectExampleResponse, error) {
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
func (c *APIClient) DeleteSubjectExample(ctx context.Context, imageID string) error {
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
