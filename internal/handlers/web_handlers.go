package handlers

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	log "github.com/sirupsen/logrus" // Use logrus consistently
	"net/http"
	"path/filepath"

	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/models"
	"double-take-go-reborn/internal/mqtt"
	"double-take-go-reborn/internal/services"
	"double-take-go-reborn/internal/sse"
	"gorm.io/gorm"
	"github.com/go-chi/chi/v5"
)

// Force compiler to acknowledge potentially indirectly used imports
var _ embed.FS
var _ fs.FS

// WebHandler holds dependencies for web handlers
type WebHandler struct {
	DB             *gorm.DB
	Cfg            *config.Config
	IndexTmpl      *template.Template
	DiagnosticsTmpl *template.Template
	ComprefaceSvc *services.CompreFaceService 
	MqttCli        *mqtt.Client             
	StaticPath string 
	TemplatePath string 
	SSEHub         *sse.Hub                  // Add SSE Hub
}

// Custom template functions
var funcMap = template.FuncMap{
	"mult": func(a, b float64) float64 {
		return a * b
	},
	"basename": filepath.Base, 
}

// BaseData holds common data for all templates
type BaseData struct {
	ActivePage string
	// Add other common fields like SiteTitle etc. if needed
}

// IndexData holds data specific to the index template
type IndexData struct {
	BaseData
	ImagesWithFaces    []models.Image // Changed: Images with detected faces
	ImagesWithoutFaces []models.Image // Changed: Images without detected faces
}

// DiagnosticsData holds data specific to the diagnostics template
type DiagnosticsData struct {
	BaseData
	Config       ConfigInfo 
	MQTTStatus   string     
	CompreStatus string     
	DBStats      DBStatistics
}

// ConfigInfo holds selected, non-sensitive configuration values
// We explicitly list fields to avoid leaking secrets.
type ConfigInfo struct {
	MQTTEnabled   bool
	MQTTBroker    string
	MQTTTopic     string
	FrigateURL    string
	CompreEnabled bool
	CompreFaceURL string
	// DBType        string 
	// Add other relevant non-sensitive config fields here
}

// DBStatistics holds counts from the database
type DBStatistics struct {
	ImageCount    int64
	FaceCount     int64
	MatchCount    int64
	IdentityCount int64
}

// NewWebHandler creates a new WebHandler
func NewWebHandler(db *gorm.DB, cfg *config.Config, compreSvc *services.CompreFaceService, mqttCli *mqtt.Client, templatePath string, staticPath string, sseHub *sse.Hub) (*WebHandler, error) {
	// Define template file paths
	navbarPath := filepath.Join(templatePath, "_navbar.html")
	indexPath := filepath.Join(templatePath, "index.html")
	diagnosticsPath := filepath.Join(templatePath, "diagnostics.html")

	// Parse index template including the navbar partial
	indexTmpl, err := template.ParseFiles(indexPath, navbarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse index.html and navbar: %w", err)
	}

	// Parse diagnostics template including the navbar partial
	diagTmpl, err := template.ParseFiles(diagnosticsPath, navbarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diagnostics.html and navbar: %w", err)
	}

	wh := &WebHandler{
		DB:             db,
		Cfg:            cfg,
		IndexTmpl:      indexTmpl,
		DiagnosticsTmpl: diagTmpl,
		ComprefaceSvc: compreSvc, 
		MqttCli:        mqttCli,   
		StaticPath: staticPath,
		TemplatePath: templatePath,
		SSEHub: sseHub,
	}

	log.Debugf("WebHandler initialized", "templatePath", templatePath, "staticPath", staticPath)
	return wh, nil
}

// RegisterRoutes sets up the routes for the web interface using chi.Router
func (h *WebHandler) RegisterRoutes(r chi.Router) {
	// Static file server (optional, if you have CSS/JS)
	// fs := http.FileServer(http.Dir(h.StaticPath))
	// r.Handle("/static/*", http.StripPrefix("/static/", fs))

	r.Get("/", h.handleIndex)
	r.Get("/diagnostics", h.handleDiagnostics)
	r.Get("/sse/updates", h.handleSSEUpdates)

	log.Debugf("Registering route: GET /ping") 
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Pong")
	})

	log.Debug("Finished registering web routes.")
}

// handleIndex serves the index page, showing recent images
func (h *WebHandler) handleIndex(w http.ResponseWriter, r *http.Request) {
	var recentImages []models.Image
	// Eager load Faces and their Matches, and the Identity associated with the match
	// Order by Timestamp descending to get the most recent first
	// Limit to a reasonable number, e.g., 50
	if err := h.DB.Order("timestamp desc").Limit(50).Preload("Faces.Matches.Identity").Preload("Faces.Matches").Preload("Faces").Find(&recentImages).Error; err != nil {
		log.Error("Error fetching images", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Debug("Fetched images for index page", "count", len(recentImages))
	/* // Original logging (optional to keep)
	if len(recentImages) > 0 {
		// Log details of the first few images to avoid excessive logging
		maxLog := 3
		if len(recentImages) < maxLog {
			maxLog = len(recentImages)
		}
		for i := 0; i < maxLog; i++ {
			log.Debug("Image details", "ID", recentImages[i].ID, "FilePath", recentImages[i].FilePath, "Timestamp", recentImages[i].Timestamp)
		}
	}
	*/

	// Split images into two lists
	var imagesWithFaces []models.Image
	var imagesWithoutFaces []models.Image
	for _, img := range recentImages {
		if len(img.Faces) > 0 {
			imagesWithFaces = append(imagesWithFaces, img)
		} else {
			imagesWithoutFaces = append(imagesWithoutFaces, img)
		}
	}
	log.Debugf("Split images: %d with faces, %d without faces", len(imagesWithFaces), len(imagesWithoutFaces))

	// Prepare data for the template using the new structure
	data := IndexData{
		BaseData: BaseData{ActivePage: "index"},
		ImagesWithFaces:    imagesWithFaces,    // Use the new slice
		ImagesWithoutFaces: imagesWithoutFaces, // Use the new slice
		// Images:   recentImages, // Removed old combined slice
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.IndexTmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Error("Error executing index template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// --- New Diagnostics Handler ---

// handleDiagnostics serves the diagnostics page
func (h *WebHandler) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	log.Debug("Serving diagnostics page")

	// 1. Populate Config Info (non-sensitive)
	cfgInfo := ConfigInfo{
		MQTTEnabled:   h.Cfg.MQTT.Enabled,
		MQTTBroker:    fmt.Sprintf("%s:%d", h.Cfg.MQTT.Broker, h.Cfg.MQTT.Port), 
		MQTTTopic:     h.Cfg.MQTT.Topic,
		FrigateURL:    h.Cfg.Frigate.Url, 
		CompreEnabled: h.Cfg.CompreFace.Enabled,
		CompreFaceURL: h.Cfg.CompreFace.Url, 
		// DBType:        h.Cfg.DB.Type, 
		// Add more non-sensitive fields as needed
	}

	// 2. Check MQTT Status
	mqttStatus := "Disabled"
	if h.Cfg.MQTT.Enabled {
		if h.MqttCli != nil && h.MqttCli.IsActuallyConnected() { 
			mqttStatus = "Connected"
		} else if h.MqttCli != nil {
			mqttStatus = "Disconnected"
		} else {
			mqttStatus = "Error during initialization"
		}
	}

	// 3. Check CompreFace Status
	compreStatus := "Disabled"
	if h.Cfg.CompreFace.Enabled {
		if h.ComprefaceSvc != nil {
			ok, err := h.ComprefaceSvc.Ping()
			if err != nil {
				log.Warn("MQTT Ping failed", "error", err)
				compreStatus = "Unreachable"
			} else if ok {
				compreStatus = "Reachable"
			} else {
				// Should ideally not happen if err is nil and ok is false, but handle defensively
				compreStatus = "Unknown Status"
			}
		} else {
			compreStatus = "Error during initialization"
		}
	}

	// 4. Get DB Statistics
	dbStats := DBStatistics{}
	h.DB.Model(&models.Image{}).Count(&dbStats.ImageCount)
	h.DB.Model(&models.Face{}).Count(&dbStats.FaceCount)
	h.DB.Model(&models.Match{}).Count(&dbStats.MatchCount)
	h.DB.Model(&models.Identity{}).Count(&dbStats.IdentityCount)

	// 5. Prepare data for template
	data := DiagnosticsData{
		BaseData: BaseData{ActivePage: "Diagnostics"},
		Config:       cfgInfo,
		MQTTStatus:   mqttStatus,
		CompreStatus: compreStatus,
		DBStats:      dbStats,
	}

	// 6. Execute Template
	w.Header().Set("Content-Type", "text/html")
	err := h.DiagnosticsTmpl.Execute(w, data)
	if err != nil {
		log.Error("Error executing diagnostics template", "error", err)
		http.Error(w, "Failed to render diagnostics page", http.StatusInternalServerError)
	}
}

// handleSSEUpdates handles the Server-Sent Events endpoint.
func (h *WebHandler) handleSSEUpdates(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // Adjust if needed for specific origins

	// Create a new channel for this client.
	// This channel will receive messages from the hub.
	clientChan := make(sse.Client)

	// Register the new client with the hub.
	h.SSEHub.Register(clientChan)

	// Make sure to unregister the client when the handler exits.
	// This usually happens when the client closes the connection.
	defer func() {
		h.SSEHub.Unregister(clientChan)
		log.Debug("SSE client disconnected.")
	}()

	// Get the context from the request.
	ctx := r.Context()

	// Start a loop to listen for messages on the client's channel
	// or for the client disconnecting.
	for {
		select {
		case message, ok := <-clientChan:
			if !ok {
				// Channel was closed by the hub (e.g., during unregister).
				log.Debug("SSE client channel closed by hub.")
				return // Exit the handler
			}
			// Format the message according to SSE spec (data: <json-payload>\n\n)
			fmt.Fprintf(w, "data: %s\n\n", message)

			// Flush the data immediately to the client.
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			} else {
				log.Error("Streaming unsupported!")
			}
			log.Debug("Sent SSE message to client", "message_length", len(message))
		case <-ctx.Done():
			// Client disconnected (context cancelled).
			log.Info("SSE client connection closed via context.")
			// The defer function will handle unregistering.
			return
		}
	}
}
