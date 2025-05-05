package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CompreFace API response structures (adapt based on actual CompreFace API docs/version)

// --- Recognition Response --- //
type CompreFaceRecognitionResult struct {
	Box        CompreFaceBox   `json:"box"`
	Subjects   []CompreFaceSubject `json:"subjects"`
	Age        *CompreFaceAge  `json:"age,omitempty"`
	Gender     *CompreFaceGender `json:"gender,omitempty"`
	Mask       *CompreFaceMask `json:"mask,omitempty"`
	// Add other plugins if needed (e.g., landmarks, pose)
}

type CompreFaceRecognitionResponse struct {
	Result []CompreFaceRecognitionResult `json:"result"`
	// TODO: Add fields for plugins if enabled (e.g., "plugins_versions")
}

// --- Common Sub-structures --- //
type CompreFaceBox struct {
	Probability float64 `json:"probability"`
	XMin        int     `json:"x_min"`
	YMin        int     `json:"y_min"`
	XMax        int     `json:"x_max"`
	YMax        int     `json:"y_max"`
}

type CompreFaceSubject struct {
	Subject     string  `json:"subject"`
	Similarity  float64 `json:"similarity"`
}

type CompreFaceAge struct {
	Probability float64 `json:"probability"`
	Low         int     `json:"low"`
	High        int     `json:"high"`
}

type CompreFaceGender struct {
	Probability float64 `json:"probability"`
	Value       string  `json:"value"`
}

type CompreFaceMask struct {
	Probability float64 `json:"probability"`
	Value       string  `json:"value"` // e.g., "no_mask", "mask"
}

// CompreFaceAddResponse represents the response from adding a subject example
type CompreFaceAddResponse struct {
	ImageID string `json:"image_id"`
	Subject string `json:"subject"`
}

// CompreFaceService handles communication with the CompreFace API.
type CompreFaceService struct {
	Cfg    config.CompreFaceConfig
	Client *http.Client
}

// NewCompreFaceService creates a new service instance.
func NewCompreFaceService(cfg config.CompreFaceConfig) *CompreFaceService {
	return &CompreFaceService{
		Cfg: cfg,
		Client: &http.Client{
			Timeout: 10 * time.Second, // Add a timeout
		},
	}
}

// Recognize sends an image to the CompreFace recognition endpoint.
func (s *CompreFaceService) Recognize(imageBytes []byte, filename string) (*CompreFaceRecognitionResponse, error) {
	if !s.Cfg.Enabled {
		return nil, fmt.Errorf("CompreFace service is not enabled in config")
	}
	log.Debugf("Sending image to CompreFace recognition: %s", filename)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add image file part
	part, err := writer.CreateFormFile("file", filename) // "file" is the expected field name
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = io.Copy(part, bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to copy image bytes to form: %w", err)
	}

	// Add other form fields (check CompreFace API docs for available options)
	// Example: _ = writer.WriteField("limit", "1")
	// Example: _ = writer.WriteField("prediction_count", "1")
	_ = writer.WriteField("det_prob_threshold", fmt.Sprintf("%.2f", s.Cfg.DetProbThreshold))
	// Example: Enable plugins if needed
	// _ = writer.WriteField("face_plugins", "age,gender,mask")

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Construct the target URL
	apiURL, err := url.JoinPath(s.Cfg.Url, "/api/v1/recognition/recognize")
	if err != nil {
		return nil, fmt.Errorf("failed to join CompreFace URL path: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create CompreFace request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-key", s.Cfg.RecognitionApiKey)

	// Send request
	startTime := time.Now()
	resp, err := s.Client.Do(req)
	duration := time.Since(startTime)
	if err != nil {
		log.Errorf("CompreFace request failed after %v: %v", duration, err)
		return nil, fmt.Errorf("failed to send request to CompreFace: %w", err)
	}
	defer resp.Body.Close()

	log.Debugf("CompreFace request completed in %v with status: %s", duration, resp.Status)

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read CompreFace response body: %w", err)
	}

	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		log.Errorf("CompreFace returned non-OK status: %s. Body: %s", resp.Status, string(respBody))
		// Try to parse potential error message from CompreFace
		var apiError map[string]interface{}
		if json.Unmarshal(respBody, &apiError) == nil {
			if msg, ok := apiError["message"]; ok {
				return nil, fmt.Errorf("CompreFace API error (%s): %v", resp.Status, msg)
			}
		}
		return nil, fmt.Errorf("CompreFace request failed with status: %s", resp.Status)
	}

	// Parse the successful response
	var recognitionResponse CompreFaceRecognitionResponse
	if err := json.Unmarshal(respBody, &recognitionResponse); err != nil {
		log.Errorf("Failed to decode CompreFace JSON response: %v. Body: %s", err, string(respBody))
		return nil, fmt.Errorf("failed to decode CompreFace response: %w", err)
	}

	return &recognitionResponse, nil
}

// Ping checks if the CompreFace recognition service is reachable and responding
// by querying a known-working endpoint that requires the recognition key.
func (s *CompreFaceService) Ping() (bool, error) {
	// Use the subjects endpoint as it requires the key and failed if recognition is down.
	// The /status endpoint seems buggy in some installations (returns 500 with key).
	pingURL := fmt.Sprintf("%s/api/v1/recognition/subjects/", s.Cfg.Url)
	req, err := http.NewRequest("GET", pingURL, nil)
	if err != nil {
		log.Errorf("CompreFace Ping: Failed to create request for %s: %v", pingURL, err)
		return false, fmt.Errorf("failed to create ping request: %w", err)
	}
	// Recognition endpoint requires the key
	req.Header.Set("x-api-key", s.Cfg.RecognitionApiKey)

	resp, err := s.Client.Do(req)
	if err != nil {
		// Log the error but treat network errors as unreachable
		log.Warnf("CompreFace Ping: Failed to send request to %s: %v", pingURL, err)
		return false, nil // Return false, nil for network/connection errors (unreachable)
	}
	defer resp.Body.Close()

	// Check if the status code indicates success (e.g., 200 OK)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Debugf("CompreFace Ping to %s successful with status %d", pingURL, resp.StatusCode)
		return true, nil
	}

	// Read the body for more context in case of non-2xx status
	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Warnf("CompreFace Ping: Received non-OK status code %d from %s. Body: %s", resp.StatusCode, pingURL, string(bodyBytes))
	// Treat non-2xx status codes as reachable but potentially problematic, but for Ping's purpose, it means the service *responded*.
	// However, specifically for auth errors (401), we might consider it unreachable.
	if resp.StatusCode == http.StatusUnauthorized {
		return false, fmt.Errorf("compreface ping failed: unauthorized (401), check API key")
	}

	// For other non-2xx errors, we could argue it's 'reachable' but unhealthy.
	// Let's return false for simplicity, indicating a problem.
	return false, fmt.Errorf("compreface ping failed with status %d", resp.StatusCode)
}

// GetAllSubjects retrieves a list of all subject names known to CompreFace.
func (s *CompreFaceService) GetAllSubjects() ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/recognition/subjects", s.Cfg.Url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to get subjects: %w", err)
	}
	req.Header.Set("x-api-key", s.Cfg.RecognitionApiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request to get subjects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Errorf("CompreFace get subjects API returned non-OK status: %d - Body: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("compreface get subjects API error: status %d", resp.StatusCode)
	}

	type SubjectsResponse struct {
		Subjects []string `json:"subjects"`
	}
	var subjectsResp SubjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&subjectsResp); err != nil {
		return nil, fmt.Errorf("error decoding subjects response: %w", err)
	}

	log.Debugf("Fetched %d subjects from CompreFace", len(subjectsResp.Subjects))
	return subjectsResp.Subjects, nil
}

// SyncIdentities synchronizes the subjects from CompreFace with the local Identity table.
func (s *CompreFaceService) SyncIdentities(db *gorm.DB) error {
	log.Info("Starting CompreFace identity synchronization...")
	compreSubjects, err := s.GetAllSubjects()
	if err != nil {
		log.WithError(err).Error("Failed to get subjects from CompreFace for sync")
		return fmt.Errorf("failed to get subjects from compreface: %w", err)
	}

	compreSubjectMap := make(map[string]bool)
	for _, name := range compreSubjects {
		compreSubjectMap[strings.ToLower(name)] = true // Use lower case for comparison
	}

	var localIdentities []models.Identity
	if err := db.Find(&localIdentities).Error; err != nil {
		log.WithError(err).Error("Failed to get identities from local database for sync")
		return fmt.Errorf("failed to get local identities: %w", err)
	}

	localIdentityMap := make(map[string]bool)
	for _, identity := range localIdentities {
		localIdentityMap[strings.ToLower(identity.Name)] = true // Use lower case for comparison
	}

	newIdentitiesCount := 0
	// Check for subjects in CompreFace that are missing locally
	for _, compreName := range compreSubjects {
		if !localIdentityMap[strings.ToLower(compreName)] {
			// Subject exists in CompreFace but not locally, create it
			newIdentity := models.Identity{Name: compreName} // Keep original casing for DB
			if err := db.Where("lower(name) = ?", strings.ToLower(compreName)).FirstOrCreate(&newIdentity).Error; err != nil {
				// Log error but continue with other subjects
				log.WithError(err).Errorf("Failed to create missing local identity: %s", compreName)
			} else {
				log.Infof("Created missing local identity: %s", compreName)
				newIdentitiesCount++
			}
		}
	}

	log.Infof("CompreFace identity synchronization finished. Found %d subjects in CompreFace. Created %d new local identities.", len(compreSubjects), newIdentitiesCount)
	return nil
}

// AddSubjectExample adds an example image for a specific subject to CompreFace.
// If the subject does not exist, CompreFace creates it automatically.
func (s *CompreFaceService) AddSubjectExample(subjectName string, file io.Reader, fileHeader *multipart.FileHeader) (*CompreFaceAddResponse, error) {
	if !s.Cfg.Enabled {
		return nil, fmt.Errorf("CompreFace service is not enabled in config")
	}

	// 1. Prepare the multipart request body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the file part
	part, err := writer.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return nil, fmt.Errorf("error creating form file part: %w", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("error copying file content to form part: %w", err)
	}

	// Close the writer to finalize the body
	writer.Close() // Important!

	// 2. Construct the URL with query parameters
	reqURL := fmt.Sprintf("%s/api/v1/recognition/faces", s.Cfg.Url)
	u, err := url.Parse(reqURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing base URL: %w", err)
	}
	q := u.Query()
	q.Set("subject", subjectName)
	// Add det_prob_threshold if needed from config, e.g.:
	// q.Set("det_prob_threshold", fmt.Sprintf("%.2f", s.detProbThreshold))
	u.RawQuery = q.Encode()

	// 3. Create the HTTP request
	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("error creating add subject example request: %w", err)
	}

	// Set headers
	req.Header.Set("x-api-key", s.Cfg.RecognitionApiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType()) // Use the writer's content type

	// 4. Execute the request
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing add subject example request: %w", err)
	}
	defer resp.Body.Close()

	// 5. Handle the response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading add subject example response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated { // CompreFace might return 200 or 201
		return nil, fmt.Errorf("error adding subject example, status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var addResponse CompreFaceAddResponse
	if err := json.Unmarshal(bodyBytes, &addResponse); err != nil {
		// Log the raw response if JSON decoding fails
		log.Warnf("Failed to decode CompreFace JSON response: %s", string(bodyBytes))
		return nil, fmt.Errorf("error decoding add subject example response: %w", err)
	}

	return &addResponse, nil
}
