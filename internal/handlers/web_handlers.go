package handlers

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/database"
	"double-take-go-reborn/internal/models"
	"double-take-go-reborn/internal/mqtt"
	"double-take-go-reborn/internal/services"
	"double-take-go-reborn/internal/sse"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

//go:embed ../../web
var webFS embed.FS

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
	// Parse templates from the embedded webFS, using paths relative to 'web'
	t, err := template.ParseFS(webFS, "web/templates/*.html", "web/templates/layouts/*.html")
	if err != nil {
		return fmt.Errorf("error parsing templates: %w", err)
	}
	h.templates = t
	log.Infof("Loaded templates from embedded FS: %s", h.templatePath)
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
		if len(img.Recognitions) > 0 {
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

// handleSSEUpdates handles Server-Sent Events connection
func (h *WebHandler) handleSSEUpdates(c *gin.Context) {
	if h.sseHub == nil {
		log.Warn("SSE endpoint called but SSE Hub is not initialized.")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSE service not available"})
		return
	}

	log.Info("SSE client connected")
	h.sseHub.ServeHTTP(c.Writer, c.Request)
	// Gin context will handle connection closing etc. when ServeHTTP returns
	log.Info("SSE client disconnected")
}

// RegisterRoutes sets up the routes for the web interface using Gin router group
func (h *WebHandler) RegisterRoutes(web *gin.RouterGroup) {
	// Serve static files from the 'web/static' directory within the embedded webFS
	staticRoot, err := fs.Sub(webFS, "web/static")
	if err != nil {
		log.Fatalf("Failed to create sub FS for static assets: %v", err)
	}
	staticServer := http.FileServer(http.FS(staticRoot))

	// Serve static files under /static path relative to the group
	web.GET("/static/*filepath", func(c *gin.Context) {
		// Strip the group prefix and the static prefix
		relativePath := strings.TrimPrefix(c.Request.URL.Path, web.BasePath()+"/static")
		// Update the request URL path for the file server (now relative to the staticRoot)
		c.Request.URL.Path = relativePath
		staticServer.ServeHTTP(c.Writer, c.Request)
	})

	// --- Register handlers using Gin context ---
	web.GET("/", h.handleIndex)
	web.GET("/diagnostics", h.handleDiagnostics)
	web.GET("/sse/updates", h.handleSSEUpdates)

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
