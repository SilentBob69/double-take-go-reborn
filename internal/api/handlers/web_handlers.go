package handlers

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/core/processor"
	"double-take-go-reborn/internal/server/sse"
	"double-take-go-reborn/internal/utils"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"encoding/json"
)

// WebHandler behandelt Anfragen für die Weboberfläche
type WebHandler struct {
	db          *gorm.DB
	cfg         *config.Config
	templates   *template.Template
	sseHub      *sse.Hub
	workerPool  *processor.WorkerPool // Zugriff auf den Worker-Pool
}

// PageData ist eine Basisstruktur für Templatedaten
type PageData struct {
	Title       string
	CurrentPage string
	Data        interface{}
	Config      *config.Config
}

// NewWebHandler erstellt einen neuen Web-Handler
func NewWebHandler(db *gorm.DB, cfg *config.Config, sseHub *sse.Hub, workerPool *processor.WorkerPool) (*WebHandler, error) {
	h := &WebHandler{
		db:          db,
		cfg:         cfg,
		sseHub:      sseHub,
		workerPool:  workerPool,
	}

	// Templates laden
	if err := h.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return h, nil
}

// loadTemplates lädt alle HTML-Templates
func (h *WebHandler) loadTemplates() error {
	log.Infof("Loading templates from %s", h.cfg.Server.TemplateDir)

	// Template-Funktionen definieren
	funcMap := template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("02.01.2006 15:04:05")
		},
		"formatDateTime": func(t time.Time) string {
			return t.Format("02.01.2006 15:04:05")
		},
		"formatConfidence": func(c float64) string {
			return fmt.Sprintf("%.2f%%", c*100)
		},
		"inc": func(i int) int {
			return i + 1
		},
		"dec": func(i int) int {
			return i - 1
		},
		"add": func(a, b int) int {
			return a + b
		},
		"subtract": func(a, b int) int {
			return a - b
		},
		"paginationRange": func(current, total int) []int {
			start := current - 2
			if start < 1 {
				start = 1
			}
			
			end := start + 4
			if end > total {
				end = total
			}
			
			if end-start < 4 && start > 1 {
				start = end - 4
				if start < 1 {
					start = 1
				}
			}
			
			var pages []int
			for i := start; i <= end; i++ {
				pages = append(pages, i)
			}
			return pages
		},
		"formatJSON": func(data interface{}) string {
			jsonBytes, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				return fmt.Sprintf("Error formatting JSON: %v", err)
			}
			return string(jsonBytes)
		},
	}

	// Templates mit Funktionen parsen
	pattern := filepath.Join(h.cfg.Server.TemplateDir, "*.html")
	templates, err := template.New("").Funcs(funcMap).ParseGlob(pattern)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	h.templates = templates
	log.Infof("Loaded %d templates", len(templates.Templates()))
	return nil
}

// renderTemplate rendert ein Template mit den gegebenen Daten
func (h *WebHandler) renderTemplate(c *gin.Context, name string, data interface{}) {
	// Prüfen, ob das Template existiert
	if h.templates.Lookup(name) == nil {
		log.Errorf("Template %s not found", name)
		c.String(http.StatusInternalServerError, "Template not found")
		return
	}

	// Gin-Daten in eine Map umwandeln, damit wir sie erweitern können
	var templateData gin.H
	
	// Konvertieren des data-Interface in gin.H
	if dataMap, ok := data.(gin.H); ok {
		templateData = dataMap
	} else {
		// Fallback, wenn data kein gin.H ist
		templateData = gin.H{"Data": data}
	}
	
	// Basisdaten hinzufügen, aber nur wenn nicht bereits vorhanden
	if _, exists := templateData["Title"]; !exists {
		templateData["Title"] = "Double-Take"
	}
	
	if _, exists := templateData["CurrentPage"]; !exists {
		templateData["CurrentPage"] = name
	}
	
	// Config nur hinzufügen, wenn nicht bereits gesetzt
	if _, exists := templateData["Config"]; !exists {
		templateData["Config"] = h.cfg
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(c.Writer, name, templateData); err != nil {
		log.Errorf("Template execution error: %v", err)
		c.String(http.StatusInternalServerError, "Template error: "+err.Error())
		return
	}
}

// RegisterRoutes registriert alle Web-Routen
func (h *WebHandler) RegisterRoutes(router *gin.Engine) {
	// Statische Dateien
	router.Static("/static", filepath.Join(h.cfg.Server.TemplateDir, "../static"))
	
	// Snapshots
	router.Static(h.cfg.Server.SnapshotURL, h.cfg.Server.SnapshotDir)

	// Hauptseiten
	router.GET("/", h.handleIndex)
	router.GET("/gallery", h.handleGallery)
	router.GET("/identities", h.handleIdentities)
	router.GET("/settings", h.handleSettings)
	router.GET("/diagnostics", h.handleDiagnostics)

	// SSE-Endpunkt für Echtzeitaktualisierungen
	router.GET("/events", h.handleSSE)
}

// handleIndex zeigt die Hauptseite an
func (h *WebHandler) handleIndex(c *gin.Context) {
	// Letzte Erkennungen mit Gesichtern abrufen
	var recentImages []models.Image
	if err := h.db.Preload("Faces.Matches.Identity").Order("created_at DESC").Limit(24).Find(&recentImages).Error; err != nil {
		log.Errorf("Failed to fetch recent images: %v", err)
	}
	
	// Bilder in solche mit und ohne Gesichter aufteilen
	var imagesWithFaces []models.Image
	var imagesWithoutFaces []models.Image
	
	for _, img := range recentImages {
		if len(img.Faces) > 0 {
			imagesWithFaces = append(imagesWithFaces, img)
		} else {
			imagesWithoutFaces = append(imagesWithoutFaces, img)
		}
	}
	
	// Statistiken abrufen
	var imageCount int64
	var faceCount int64
	var identityCount int64

	h.db.Model(&models.Image{}).Count(&imageCount)
	h.db.Model(&models.Face{}).Count(&faceCount)
	h.db.Model(&models.Identity{}).Count(&identityCount)

	// Daten für das Template
	data := gin.H{
		"Images":            recentImages,
		"ImagesWithFaces":   imagesWithFaces,
		"ImagesWithoutFaces": imagesWithoutFaces,
		"ImageCount":        imageCount,
		"FaceCount":         faceCount,
		"IdentityCount":     identityCount,
		"UpdatedAt":         time.Now(),
	}

	h.renderTemplate(c, "index.html", data)
}

// handleGallery zeigt die Galerie-Seite an
func (h *WebHandler) handleGallery(c *gin.Context) {
	// Paginierung
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := 20
	offset := (page - 1) * pageSize

	// Filter
	sourceFilter := c.Query("source")
	identityFilter := c.Query("identity")

	// Query vorbereiten
	query := h.db.Model(&models.Image{}).Order("created_at DESC")

	// Filter anwenden
	if sourceFilter != "" {
		query = query.Where("source = ?", sourceFilter)
	}

	if identityFilter != "" {
		// Hier müssten wir einen komplexeren Join machen, um nach Identity zu filtern
		// In einer realen Implementierung würde man das mit einem JOIN auf Faces und Matches implementieren
	}

	// Zählen für Paginierung
	var total int64
	query.Count(&total)

	// Bilder abrufen
	var images []models.Image
	if err := query.Preload("Faces.Matches.Identity").Offset(offset).Limit(pageSize).Find(&images).Error; err != nil {
		log.Errorf("Failed to fetch images for gallery: %v", err)
	}

	// Quellen für Filter abrufen
	var sources []string
	h.db.Model(&models.Image{}).Distinct().Pluck("source", &sources)

	// Identitäten für Filter abrufen
	var identities []models.Identity
	h.db.Find(&identities)

	// Daten für das Template
	data := gin.H{
		"Images":      images,
		"Sources":     sources,
		"Identities":  identities,
		"Pagination": gin.H{
			"Current":     page,
			"PageSize":    pageSize,
			"Total":       total,
			"TotalPages":  (total + int64(pageSize) - 1) / int64(pageSize),
			"HasPrevious": page > 1,
			"HasNext":     int64(page*pageSize) < total,
		},
		"Filter": gin.H{
			"Source":   sourceFilter,
			"Identity": identityFilter,
		},
	}

	h.renderTemplate(c, "gallery.html", data)
}

// handleIdentities zeigt die Identitäten-Seite an
func (h *WebHandler) handleIdentities(c *gin.Context) {
	// Alle Identitäten abrufen
	var identities []models.Identity
	if err := h.db.Find(&identities).Error; err != nil {
		log.Errorf("Failed to fetch identities: %v", err)
	}

	// Für jede Identität die Anzahl der Matches abrufen
	for i := range identities {
		var count int64
		h.db.Model(&models.Match{}).Where("identity_id = ?", identities[i].ID).Count(&count)
		identities[i].Matches = []models.Match{{}} // Platzhalter um die Anzahl hinzuzufügen
	}

	// Daten für das Template
	data := gin.H{
		"Identities": identities,
		"CanAddExample": h.cfg.CompreFace.Enabled,
	}

	h.renderTemplate(c, "identities.html", data)
}

// handleSettings zeigt die Einstellungen-Seite an
func (h *WebHandler) handleSettings(c *gin.Context) {
	// Konfiguration für die Anzeige aufbereiten
	settings := gin.H{
		"Server": gin.H{
			"SnapshotDir": h.cfg.Server.SnapshotDir,
			"Port":        h.cfg.Server.Port,
		},
		"CompreFace": gin.H{
			"Enabled":           h.cfg.CompreFace.Enabled,
			"URL":               h.cfg.CompreFace.URL,
			"DetProbThreshold":  h.cfg.CompreFace.DetProbThreshold,
			"SyncIntervalMinutes": h.cfg.CompreFace.SyncIntervalMinutes,
		},
		"MQTT": gin.H{
			"Enabled": h.cfg.MQTT.Enabled,
			"Broker":  h.cfg.MQTT.Broker,
			"Port":    h.cfg.MQTT.Port,
			"Topic":   h.cfg.MQTT.Topic,
		},
		"Cleanup": gin.H{
			"RetentionDays": h.cfg.Cleanup.RetentionDays,
		},
	}

	h.renderTemplate(c, "settings.html", settings)
}

// handleSSE behandelt SSE-Verbindungen für Echtzeit-Updates
func (h *WebHandler) handleSSE(c *gin.Context) {
	// SSE-Header setzen
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Client-Kanal erstellen
	client := make(sse.Client, 10) // Puffer für 10 Nachrichten

	// Client beim Hub registrieren
	h.sseHub.Register(client)
	defer h.sseHub.Unregister(client)

	// Client-Verbindung überwachen
	c.Stream(func(w io.Writer) bool {
		// Auf die nächste Nachricht warten
		msg, ok := <-client
		if !ok {
			return false // Kanal geschlossen, Stream beenden
		}

		// Nachricht im SSE-Format senden
		c.SSEvent("message", string(msg))
		return true
	})
}

// handleDiagnostics zeigt Diagnose-Informationen an
func (h *WebHandler) handleDiagnostics(c *gin.Context) {
	// System-Informationen sammeln
	var dbStats struct {
		ImageCount     int64
		FaceCount      int64
		IdentityCount  int64
		EventCount     int64
		DBSize         string
		LastDetection  time.Time
		LastRecognition time.Time
	}
	
	h.db.Model(&models.Image{}).Count(&dbStats.ImageCount)
	h.db.Model(&models.Face{}).Count(&dbStats.FaceCount)
	h.db.Model(&models.Identity{}).Count(&dbStats.IdentityCount)
	
	// Größe der DB-Datei ermitteln
	dbFilePath := h.cfg.DB.File
	if fileInfo, err := os.Stat(dbFilePath); err == nil {
		sizeInBytes := fileInfo.Size()
		if sizeInBytes < 1024 {
			dbStats.DBSize = fmt.Sprintf("%d Bytes", sizeInBytes)
		} else if sizeInBytes < 1024*1024 {
			dbStats.DBSize = fmt.Sprintf("%.1f KB", float64(sizeInBytes)/1024)
		} else if sizeInBytes < 1024*1024*1024 {
			dbStats.DBSize = fmt.Sprintf("%.1f MB", float64(sizeInBytes)/(1024*1024))
		} else {
			dbStats.DBSize = fmt.Sprintf("%.2f GB", float64(sizeInBytes)/(1024*1024*1024))
		}
	} else {
		log.Warnf("Konnte DB-Dateigröße nicht ermitteln: %v", err)
		dbStats.DBSize = "Unbekannt"
	}
	
	// Letzte Erkennung und Identifizierung
	var lastImage models.Image
	if err := h.db.Order("created_at DESC").First(&lastImage).Error; err == nil {
		dbStats.LastDetection = lastImage.CreatedAt
	}
	
	var lastMatch models.Match
	if err := h.db.Order("created_at DESC").First(&lastMatch).Error; err == nil {
		dbStats.LastRecognition = lastMatch.CreatedAt
	}
	
	// CompreFace-Status und Subjects
	compreFaceStatus := "Unbekannt"
	if h.cfg.CompreFace.Enabled {
		compreFaceStatus = "Aktiviert"
	} else {
		compreFaceStatus = "Deaktiviert"
	}
	
	// CompreFace-Subjects aus der Datenbank abrufen
	var identities []models.Identity
	var compreFaceSubjects []string
	if h.cfg.CompreFace.Enabled {
		if err := h.db.Where("external_id IS NOT NULL").Find(&identities).Error; err == nil {
			for _, identity := range identities {
				compreFaceSubjects = append(compreFaceSubjects, identity.Name)
			}
			log.Infof("Gefundene Identitäten mit External ID: %d", len(identities))
		}
	}
	
	// MQTT-Status
	mqttStatus := "Unbekannt"
	if h.cfg.MQTT.Enabled {
		mqttStatus = "Verbunden"
		// Hier könnte man den MQTT-Client-Status prüfen
	} else {
		mqttStatus = "Deaktiviert"
	}
	
	// System-Statistiken abrufen
	systemStats := utils.GetSystemStats(h.workerPool)
	
	// Konfigurationsdaten aufbereiten
	configData := gin.H{
		"FrigateURL":     h.cfg.Frigate.URL,
		"MQTTEnabled":    h.cfg.MQTT.Enabled,
		"MQTTBroker":     h.cfg.MQTT.Broker,
		"MQTTPort":       h.cfg.MQTT.Port,
		"MQTTTopic":      h.cfg.MQTT.Topic,
		"CompreFaceURL":  h.cfg.CompreFace.URL,
		"CompreEnabled":  h.cfg.CompreFace.Enabled,
		"DataDir":        "/data", // Hardcoded Standardwert, falls nicht in der Config
		"Version":        "1.0.0", // Hier könnte eine Versionsnummer eingetragen werden
	}
	
	// Template-Daten
	data := gin.H{
		"DBStats": dbStats,
		"Services": gin.H{
			"CompreFace": compreFaceStatus,
			"MQTT": mqttStatus,
		},
		"Config": configData,
		"CompreFaceSubjects": compreFaceSubjects,
		"SystemStats": gin.H{
			"CPUs":             systemStats.NumCPU,
			"GoRoutines":       systemStats.GoRoutines,
			"CPUUsage":         systemStats.CPUUsage,
			"MemoryAlloc":      utils.FormatBytes(systemStats.MemoryAlloc),
			"MemorySys":        utils.FormatBytes(systemStats.MemorySys),
			"WorkerCount":      systemStats.WorkerCount,
			"ActiveJobs":       systemStats.ActiveJobs,
			"QueueCapacity":    systemStats.QueueCapacity,
			"Timestamp":        systemStats.Timestamp,
		},
	}
	
	h.renderTemplate(c, "diagnostics.html", data)
}
