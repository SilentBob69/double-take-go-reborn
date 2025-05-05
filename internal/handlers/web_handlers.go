package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/models"
	"double-take-go-reborn/internal/mqtt"
	"double-take-go-reborn/internal/services"
	"double-take-go-reborn/internal/sse"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// WebHandler handles requests for the web interface
type WebHandler struct {
	db            *gorm.DB
	cfg           *config.Config
	compreService *services.CompreFaceService
	mqttClient    *mqtt.Client
	templates     *template.Template
	sseHub        *sse.Hub
	templatePath  string
	staticPath    string
	// Include other dependencies as needed
}

// NewWebHandler creates a new WebHandler
func NewWebHandler(db *gorm.DB, cfg *config.Config, compreService *services.CompreFaceService, mqttClient *mqtt.Client, templatePath string, staticPath string, sseHub *sse.Hub) (*WebHandler, error) {
	h := &WebHandler{
		db:            db,
		cfg:           cfg,
		compreService: compreService,
		mqttClient:    mqttClient,
		templatePath:  templatePath,
		staticPath:    staticPath,
		sseHub:        sseHub, // Store the SSE hub
	}

	// Load templates
	err := h.loadTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return h, nil
}

func (h *WebHandler) loadTemplates() error {
	// Parse templates directly from the filesystem path provided
	mainPattern := filepath.Join(h.templatePath, "*.html")
	layoutPattern := filepath.Join(h.templatePath, "layouts", "*.html")
	log.Infof("Loading templates from filesystem: main='%s', layout='%s'", mainPattern, layoutPattern)

	t, err := template.ParseGlob(mainPattern)
	if err != nil {
		return fmt.Errorf("error parsing main templates (%s): %w", mainPattern, err)
	}
	t, err = t.ParseGlob(layoutPattern)
	if err != nil {
		// Don't fail if layouts don't exist, maybe they aren't used
		log.Warnf("Could not parse layout templates (%s), proceeding without them: %v", layoutPattern, err)
	} else {
		log.Infof("Successfully parsed layout templates from %s", layoutPattern)
	}

	h.templates = t
	log.Infof("Successfully loaded templates from %s", h.templatePath)
	return nil
}

// executeTemplate renders a template with the given data
func (h *WebHandler) executeTemplate(c *gin.Context, name string, data interface{}) {
	// Add global data if needed (e.g., version)
	// Create a map or struct to hold both global and specific data
	pageData := gin.H{
		"Data": data, // Pass specific data under 'Data'
		// Add other global fields like "Version", "AppName" here
	}

	if h.templates == nil {
		log.Error("Templates not loaded, cannot execute")
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("templates not loaded"))
		return
	}

	err := h.templates.ExecuteTemplate(c.Writer, name, pageData)
	if err != nil {
		log.Errorf("Error executing template %s: %v", name, err)
		// Use Gin's error handling
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}

// handleIndex handles requests for the main page
func (h *WebHandler) handleIndex(c *gin.Context) {
	log.Debug("Handling request for /index")
	var images []models.Image
	// Retrieve latest images, recognitions, faces, matches, identities
	// Use Preload for eager loading associations
	if err := h.db.Order("created_at desc").Limit(50). // Limit results
		Preload("Recognitions").
		Preload("Recognitions.Faces").
		Preload("Recognitions.Faces.Matches").
		Preload("Recognitions.Faces.Matches.Identity"). // Eager load Identity
		Find(&images).Error; err != nil {
		log.Errorf("Error fetching images: %v", err)
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("database error"))
		return
	}

	// Split images into two lists
	var imagesWithFaces []models.Image
	var imagesWithoutFaces []models.Image
	for _, img := range images {
		if len(img.Faces) > 0 { // Check for Faces instead of Recognitions
			imagesWithFaces = append(imagesWithFaces, img)
		} else {
			imagesWithoutFaces = append(imagesWithoutFaces, img)
		}
	}
	log.Debugf("Split images: %d with faces, %d without faces", len(imagesWithFaces), len(imagesWithoutFaces))

	// Pass images (with preloaded data) to the template
	data := gin.H{
		"ImagesWithFaces":    imagesWithFaces,
		"ImagesWithoutFaces": imagesWithoutFaces,
	}
	h.executeTemplate(c, "index.html", data)
}

// handleDiagnostics handles requests for the diagnostics page
func (h *WebHandler) handleDiagnostics(c *gin.Context) {
	// Gather diagnostic data (placeholder)
	data := gin.H{
		"Timestamp": time.Now(),
		"Status":    "OK",
		// Add more diagnostic info here
	}
	h.executeTemplate(c, "diagnostics.html", data)
}

// handleSSE handles Server-Sent Events connections.
// Based on the sse.Hub structure.
func (h *WebHandler) handleSSE(c *gin.Context) {
	log.Info("SSE client connecting...")

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*") // Optional: Adjust CORS if needed

	// Create a channel for this specific client
	clientChannel := make(sse.Client)

	// Register the client with the hub
	h.sseHub.Register(clientChannel)
	log.Info("SSE client registered.")

	// Ensure client is unregistered when the connection closes
	defer func() {
		h.sseHub.Unregister(clientChannel)
		log.Info("SSE client unregistered.")
	}()

	// Use a flusher to send data immediately
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		log.Error("Streaming unsupported by the writer")
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("streaming unsupported"))
		return
	}

	// Listen for messages from the hub and send them to the client
	// Also listen for the connection closing
	for {
		select {
		case message, open := <-clientChannel:
			if !open {
				// Channel closed by the hub, connection should close
				log.Info("SSE client channel closed by hub.")
				return
			}
			// Format the message according to SSE spec (data: ...\n\n)
			_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", message)
			if err != nil {
				log.Errorf("Error writing to SSE client: %v", err)
				// Error writing, likely connection closed, stop loop
				return
			}
			flusher.Flush() // Flush the data to the client
			log.Debugf("Sent SSE message: %s", string(message))

		case <-c.Request.Context().Done():
			// Client disconnected
			log.Info("SSE client disconnected (context done).")
			return
		}
	}
}

// RegisterRoutes sets up the routes for the web interface using Gin router group
func (h *WebHandler) RegisterRoutes(web *gin.RouterGroup) {
	// Serve static files directly from the filesystem path provided
	log.Infof("Serving static files from filesystem path: %s", h.staticPath)
	fs := http.Dir(h.staticPath)
	staticServer := http.FileServer(fs)
	// staticServer := http.StripPrefix(web.BasePath()+"/static/", http.FileServer(fs)) // Alternative strip prefix

	// Serve static files under /static path relative to the group
	web.GET("/static/*filepath", func(c *gin.Context) {
		// Manually strip prefix because Gin groups interfere otherwise
		originalPath := c.Request.URL.Path
		prefix := web.BasePath() + "/static/"
		if strings.HasPrefix(originalPath, prefix) {
			c.Request.URL.Path = strings.TrimPrefix(originalPath, prefix)
			log.Debugf("Serving static file: original='%s', mapped='%s'", originalPath, c.Request.URL.Path)
			staticServer.ServeHTTP(c.Writer, c.Request)
		} else {
			// Should not happen if routing is correct, but handle defensively
			log.Warnf("Static path mismatch: expected prefix '%s', got '%s'", prefix, originalPath)
			c.AbortWithStatus(http.StatusNotFound)
		}
	})

	// --- Register handlers using Gin context ---
	web.GET("/", h.handleIndex)
	web.GET("/diagnostics", h.handleDiagnostics)
	web.GET("/events", h.handleSSE) // Add route for SSE

	log.Debugf("Registering route: GET /ping within group %s", web.BasePath())
	web.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "Pong")
	})

	log.Debug("Finished registering web routes.")
}

// --- Helper functions/types (if any) ---

// Example Data structure for index page (if needed)
type IndexData struct {
	ImagesWithFaces    []models.Image
	ImagesWithoutFaces []models.Image
	// Add other fields like Version, etc.
}
