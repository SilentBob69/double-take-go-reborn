package handlers

import (
	"double-take-go-reborn/internal/util/timezone"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/core/processor"
	"double-take-go-reborn/internal/integrations/compreface"
	syncservice "double-take-go-reborn/internal/services/sync"
	"double-take-go-reborn/internal/server/sse"
	"double-take-go-reborn/internal/utils"

	"gorm.io/datatypes"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// WebHandler behandelt Anfragen für die Weboberfläche
type WebHandler struct {
	db          *gorm.DB
	cfg         *config.Config
	templates   *template.Template // Alle Template mit Standardfunktionen
	sseHub      *sse.Hub
	workerPool  *processor.WorkerPool // Zugriff auf den Worker-Pool
	compreface  *compreface.APIClient    // Zugriff auf den CompreFace-Client
	syncService *syncservice.Service     // Synchronisierungsservice für ausstehende Operationen
	translations map[string]map[string]string // Cache für Übersetzungen
	transMutex  sync.RWMutex               // Mutex für thread-sicheren Zugriff
	activeLanguage string                 // Aktuelle Sprache für Standardanzeige
}

// loadTranslations lädt Übersetzungen aus JSON-Dateien
func loadTranslations(language string) (map[string]string, error) {
	// Prüfen, ob eine gültige Sprache angegeben wurde
	if language != "de" && language != "en" {
		language = "de" // Fallback auf Deutsch
	}
	
	// Pfad zur Übersetzungsdatei (auf absoluten Pfad im Docker-Container setzen)
	filePath := filepath.Join("/app/web/locales", language+".json")
	
	// Zusätzliche Debug-Ausgabe
	log.Debugf("Versuche Übersetzungsdatei zu laden: %s", filePath)
	
	// Prüfen, ob die Datei existiert
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Versuche es mit relativen Pfad für lokale Entwicklung
		alternativePath := filepath.Join("./web/locales", language+".json")
		log.Debugf("Datei nicht gefunden, versuche alternativen Pfad: %s", alternativePath)
		
		if _, err := os.Stat(alternativePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("translation file not found at %s or %s", filePath, alternativePath)
		}
		
		filePath = alternativePath
	}
	
	// Datei lesen
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading translation file %s: %w", filePath, err)
	}
	
	// JSON-Daten parsen
	var data map[string]interface{}
	if err := json.Unmarshal(fileData, &data); err != nil {
		return nil, fmt.Errorf("error parsing JSON in %s: %w", filePath, err)
	}
	
	// Flache Map erstellen für einfacheren Zugriff
	translations := make(map[string]string)
	flattenTranslations(data, "", translations)
	
	log.Debugf("Erfolgreich %d Übersetzungen für Sprache %s geladen", len(translations), language)
	return translations, nil
}

// flattenTranslations konvertiert verschachtelte JSON-Struktur in flache Map
func flattenTranslations(data map[string]interface{}, prefix string, result map[string]string) {
	for key, value := range data {
		newKey := key
		if prefix != "" {
			newKey = prefix + "." + key
		}
		
		switch v := value.(type) {
		case string:
			result[newKey] = v
			log.Debugf("Flache Übersetzung hinzugefügt: %s = %s", newKey, v)
		case map[string]interface{}:
			log.Debugf("Verschachtelte Map gefunden für Schlüssel: %s", newKey)
			flattenTranslations(v, newKey, result)
		default:
			log.Warnf("Unbekannter Typ für Schlüssel %s: %T", newKey, v)
		}
	}
}

// PageData ist eine Basisstruktur für Templatedaten
type PageData struct {
	Title       string
	CurrentPage string
	Data        interface{}
	Config      *config.Config
}

// getImagePath erzeugt den korrekten URL-Pfad für ein Bild basierend auf dem Dateinamen und der Quelle
func (h *WebHandler) getImagePath(filename string) string {
	// Prüfen, ob der Pfad bereits mit "frigate/" beginnt
	if strings.HasPrefix(filename, "frigate/") {
		// Pfad enthält bereits das frigate/-Präfix, direkt verwenden
		log.Debugf("Frigate-Pfad erkannt (bereits präfixiert): %s, Pfad: /snapshots/%s", filename, filename)
		return "/snapshots/" + filename
	}
	
	// Prüfen, ob es ein Frigate-Bild ohne Pfad-Präfix ist
	if strings.HasPrefix(filename, "frigate_") ||
		(len(filename) > 8 && strings.Contains(filename, "_camera_")) {
		// Frigate-Bild ohne Pfad-Präfix
		log.Debugf("Frigate-Bild erkannt (ohne Pfad-Präfix): %s, Pfad: /snapshots/frigate/%s", filename, filename)
		return "/snapshots/frigate/" + filename
	}
	
	// Generischer Fall: Datei direkt aus dem Snapshot-Verzeichnis laden
	log.Debugf("Generisches Bild erkannt: %s, Pfad: /snapshots/%s", filename, filename)
	return "/snapshots/" + filename
}

// NewWebHandler erstellt einen neuen Web-Handler
func NewWebHandler(db *gorm.DB, cfg *config.Config, sseHub *sse.Hub, workerPool *processor.WorkerPool, compreFaceClient *compreface.APIClient, syncService *syncservice.Service) (*WebHandler, error) {
	h := &WebHandler{
		db:          db,
		cfg:         cfg,
		sseHub:      sseHub,
		workerPool:  workerPool,
		compreface:  compreFaceClient,
		syncService: syncService,
		translations: make(map[string]map[string]string),
	}

	// Übersetzungen vorab laden
	deTranslations, err := loadTranslations("de")
	if err != nil {
		return nil, fmt.Errorf("failed to load default translations: %w", err)
	}
	h.translations["de"] = deTranslations

	enTranslations, err := loadTranslations("en")
	if err != nil {
		log.Warnf("Failed to load English translations: %v", err)
	} else {
		h.translations["en"] = enTranslations
	}

	// Templates laden
	if err := h.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return h, nil
}

// loadTemplates lädt alle HTML-Templates mit der Übersetzungsfunktion
func (h *WebHandler) loadTemplates() error {
	log.Infof("Loading templates from %s", h.cfg.Server.TemplateDir)

	// Setze die Standard-Sprache
	h.activeLanguage = "de"

	// Übersetzungsfunktion definieren
	tFunc := func(key string) string {
		// Get active language from handler instance
		lang := h.activeLanguage
		
		// Try to get translation for active language
		h.transMutex.RLock()
		defer h.transMutex.RUnlock()
		
		translations, exists := h.translations[lang]
		if !exists {
			// Fall back to German if language not found
			translations = h.translations["de"]
		}
		
		// Return translation or key if not found
		if translations != nil {
			if val, ok := translations[key]; ok {
				return val
			}
		}
		
		// For English, try falling back to German as second attempt
		if lang == "en" {
			if deTranslations, ok := h.translations["de"]; ok {
				if val, ok := deTranslations[key]; ok {
					return val
				}
			}
		}
		
		return key
	}

	// Template-Funktionen definieren
	funcMap := template.FuncMap{
		// Übersetzungsfunktion
		"t": tFunc,
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
		"sub": func(a, b int) int {
			return a - b
		},
		// Alias für die Subtraktion (wird in den Templates verwendet)
		"subtract": func(a, b int) int {
			return a - b
		},
		"formatBytes": utils.FormatBytes,
		"paginationRange": func(currentPage, totalPages, maxPages int) []int {
			var pages []int
			
			// Berechne Start- und Endseite basierend auf maxPages
			halfMax := maxPages / 2
			startPage := math.Max(1, float64(currentPage-halfMax))
			endPage := math.Min(float64(totalPages), startPage+float64(maxPages)-1)
			
			// Adjustiere die Startseite, wenn wir am Ende des Bereichs sind
			if endPage-startPage+1 < float64(maxPages) && startPage > 1 {
				startPage = math.Max(1, endPage-float64(maxPages)+1)
			}
			
			for i := int(startPage); i <= int(endPage); i++ {
				pages = append(pages, i)
			}
			return pages
		},
		"formatJSON": func(data datatypes.JSON) string {
			var jsonObj interface{}
			err := json.Unmarshal(data, &jsonObj)
			if err != nil {
				return fmt.Sprintf("Error parsing JSON: %v", err)
			}
			
			jsonBytes, err := json.MarshalIndent(jsonObj, "", "  ")
			if err != nil {
				return fmt.Sprintf("Error formatting JSON: %v", err)
			}
			return string(jsonBytes)
		},
		"getCameraName": func(source string, sourceData datatypes.JSON) string {
			// Wenn keine SourceData vorhanden sind, gib die Quelle zurück
			if len(sourceData) == 0 {
				return source
			}
			
			// Versuche, den Kameranamen aus den SourceData zu extrahieren
			var data map[string]interface{}
			err := json.Unmarshal(sourceData, &data)
			if err != nil {
				log.Errorf("Fehler beim Parsen der SourceData: %v", err)
				return source
			}
			
			// Überprüfe, ob die SourceData einen Kameranamen enthalten
			if camera, ok := data["camera"].(string); ok && camera != "" {
				return camera
			}
			
			return source
		},
		"imagePath": h.getImagePath, // Funktion zur korrekten Pfadbestimmung für Bilder
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

// renderTemplate rendert ein Template mit den gegebenen Daten und berücksichtigt die Spracheinstellung
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
		// Wenn data kein gin.H ist, erstellen wir eine neue Map
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
	
	// Sprachauswahl über lang-Parameter oder Cookie
	language := "de" // Standardsprache Deutsch
	
	// 1. Zuerst URL-Parameter prüfen
	langParam := c.Query("lang")
	if langParam == "de" || langParam == "en" {
		language = langParam
		// Bei language-Parameter im URL immer den Cookie aktualisieren
		log.Infof("Setze Sprache aus URL-Parameter auf: %s", language)
		c.SetCookie("language", language, 3600*24*30, "/", "", false, false)
	} else {
		// 2. Wenn kein Parameter vorhanden, Cookie prüfen
		lang, err := c.Cookie("language")
		if err == nil && (lang == "de" || lang == "en") {
			// Cookie vorhanden und gültige Sprache
			language = lang
			log.Infof("Verwende Sprache aus Cookie: %s", language)
		} else {
			// 3. Default-Sprache verwenden und neuen Cookie setzen
			log.Infof("Verwende Standard-Sprache: %s", language)
			c.SetCookie("language", language, 3600*24*30, "/", "", false, false)
		}
	}

	// Sprache an Template übergeben für die Anzeige des aktiven Buttons
	templateData["language"] = language

	// Wichtig: Die aktuelle Sprache im WebHandler setzen, damit die t-Funktion (Closure) die richtige Sprache verwendet
	h.activeLanguage = language
	
	// Übersetzungsfunktion zum Template hinzufügen
	templateData["t"] = func(key string) string {
		log.Debugf("Suche Übersetzung für Schlüssel: '%s' in Sprache: %s", key, language)
		
		if translations, ok := h.translations[language]; ok {
			// Direkter Lookup in der flachen Map (flattenTranslations erstellt bereits eine flache Map)
			if val, ok := translations[key]; ok {
				log.Debugf("Übersetzung gefunden für '%s': '%s'", key, val)
				return val
			}
			
			// Debug: Zeige alle verfügbaren Schlüssel an, wenn der gesuchte nicht gefunden wurde
			log.Warnf("Keine Übersetzung gefunden für Schlüssel: '%s'", key)
			
			// Versuche, ähnliche Schlüssel zu finden (zur Fehlersuche)
			for k := range translations {
				if strings.Contains(k, key) || strings.Contains(key, k) {
					log.Debugf("Ähnlicher Schlüssel gefunden: '%s'", k)
				}
			}
		}
		
		// Fallback auf den Schlüssel selbst, wenn keine Übersetzung gefunden wurde
		return key
	}

	// Template mit Daten rendern
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(c.Writer, name, templateData); err != nil {
		log.Errorf("Template execution error: %v", err)
		c.String(http.StatusInternalServerError, "Template error: "+err.Error())
		return
	}

	log.Infof("Template '%s' erfolgreich gerendert mit Sprache: %s", name, language)
}

// RegisterRoutes registriert alle Web-Routen
func (h *WebHandler) RegisterRoutes(router *gin.Engine) {
	// Statische Dateien und Router für Frontend-Komponenten
	router.Static("/static", "./web/static")
	
	// Einfache statische Route für Bilder - direkter Zugriff auf die Dateien
	router.Static("/image", "./web/image")
	
	// Bereitstellung der Snapshot-Bilder aus dem Verzeichnis
	router.Static("/snapshots", "/data/snapshots")
	
	// Hauptseiten
	router.GET("/", h.handleIndex)
	router.GET("/gallery", h.handleGallery)
	router.GET("/identities", h.handleIdentities)
	router.GET("/identities/:id", h.handleIdentityDetails)
	router.POST("/identities/:id/training", h.handleAddTrainingImage)
	router.POST("/identities/:id/delete", h.handleDeleteIdentity)
	router.GET("/settings", h.handleSettings)
	router.GET("/diagnostics", h.handleDiagnostics)
	
	// CompreFace-Training
	router.GET("/train-compreface/:id", h.handleTrainCompreFace)
	
	// Treffer/Matches
	router.POST("/matches/:id/update", h.handleUpdateMatch)
	
	// SSE-Endpunkt für Echtzeit-Updates
	router.GET("/events", h.handleSSE)
	
	// API für die Weboberfläche
	router.GET("/system/stats", h.handleSystemStats)
}

// handleTrainCompreFace zeigt eine Seite an, um ein erkanntes Gesicht mit CompreFace zu trainieren
func (h *WebHandler) handleTrainCompreFace(c *gin.Context) {
	// Image-ID aus der URL extrahieren
	imageIDStr := c.Param("id")
	imageID, err := strconv.ParseUint(imageIDStr, 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"Title": "Fehler",
			"Error": "Ungültige Bild-ID",
			"StatusCode": http.StatusBadRequest,
		})
		return
	}

	// Bild mit der angegebenen ID abrufen
	var image models.Image
	if err := h.db.Preload("Faces.Matches.Identity").First(&image, imageID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"Title": "Fehler",
			"Error": "Bild nicht gefunden",
			"StatusCode": http.StatusNotFound,
		})
		return
	}

	// Alle verfügbaren Identitäten abrufen
	var identities []models.Identity
	if err := h.db.Order("name").Find(&identities).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Title": "Fehler",
			"Error": "Fehler beim Abrufen der Identitäten",
			"StatusCode": http.StatusInternalServerError,
		})
		return
	}

	// Template-Daten vorbereiten
	templateData := gin.H{
		"Title":       "CompreFace anlernen", // Wird im Template übersetzt
		"Image":       image,
		"Identities":  identities,
		"CurrentYear": timezone.Now().Year(),
	}

	// Template rendern
	h.renderTemplate(c, "train_compreface.html", templateData)
}

// EventGroup repräsentiert eine Gruppe von Bildern, die zum selben Event gehören
type EventGroup struct {
	EventID      string
	Images       []models.Image
	HasFaces     bool
	HasMatches   bool
	ThumbnailURL string
	Source       string
	Camera       string
	Label        string
	Zone         string
	Timestamp    time.Time
	Count        int
}

// handleIndex zeigt die Hauptseite an mit integrierten Bildern und Filterfunktionen
func (h *WebHandler) handleIndex(c *gin.Context) {
	// Paginierung und Filter extrahieren
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := 18 // Gruppen pro Seite
	offset := (page - 1) * pageSize
	
	// Prüfen, ob Filter aktiv sind
	filtersActive := false
	
	// Filteroptionen extrahieren
	source := c.DefaultQuery("source", "")
	label := c.DefaultQuery("label", "")
	zone := c.DefaultQuery("zone", "")
	hasfaces := c.DefaultQuery("hasfaces", "")
	hasmatches := c.DefaultQuery("hasmatches", "")
	daterange := c.DefaultQuery("daterange", "")
	
	// Prüfen, welche Filter aktiv sind
	if source != "" || label != "" || zone != "" || hasfaces != "" || hasmatches != "" || daterange != "" {
		filtersActive = true
		log.Infof("Filter sind aktiv: source=%s, label=%s, zone=%s, hasfaces=%s, hasmatches=%s, daterange=%s", 
			source, label, zone, hasfaces, hasmatches, daterange)
	}
	
	// Datenbankabfrage vorbereiten
	db := h.db.Model(&models.Image{})
	
	// Filter anwenden
	if source != "" {
		db = db.Where("source = ?", source)
	}
	
	// Filter für Label (Bezeichnung)
	if label != "" {
		db = db.Where("label = ?", label)
	}
	
	// Filter für Zone
	if zone != "" {
		db = db.Where("zone LIKE ?", "%"+zone+"%")
	}
	
	// Filter für Gesichter
	if hasfaces == "yes" {
		db = db.Joins("LEFT JOIN faces ON faces.image_id = images.id").Where("faces.id IS NOT NULL").Group("images.id")
	} else if hasfaces == "no" {
		db = db.Where("NOT EXISTS (SELECT 1 FROM faces WHERE faces.image_id = images.id)")
	}
	
	// Filter für Matches/Identitäten
	if hasmatches == "yes" {
		db = db.Joins("LEFT JOIN faces ON faces.image_id = images.id").Joins("LEFT JOIN matches ON matches.face_id = faces.id").Where("matches.id IS NOT NULL").Group("images.id")
	} else if hasmatches == "no" {
		db = db.Where("NOT EXISTS (SELECT 1 FROM faces WHERE faces.image_id = images.id AND EXISTS (SELECT 1 FROM matches WHERE matches.face_id = faces.id))")
	}
	
	// Datumsbereich-Filter
	var startDate time.Time
	switch daterange {
		case "today":
			today := timezone.Now().Truncate(24 * time.Hour) // Heute 00:00
			startDate = today
		case "yesterday":
			yesterday := timezone.Now().Truncate(24 * time.Hour).Add(-24 * time.Hour) // Gestern 00:00
			startDate = yesterday
		case "week":
			weekStart := timezone.Now().Truncate(24 * time.Hour).Add(-7 * 24 * time.Hour) // Vor 7 Tagen 00:00
			startDate = weekStart
		case "month":
			monthStart := timezone.Now().Truncate(24 * time.Hour).Add(-30 * 24 * time.Hour) // Vor 30 Tagen 00:00
			startDate = monthStart
	}
	
	if !startDate.IsZero() {
		db = db.Where("timestamp >= ?", startDate)
	}
	
	// Zunächst alle Bilderdaten ohne Limit abfragen, um die korrekte Filterung und Gruppierung zu ermöglichen
	var images []models.Image
	dataQuery := db.Order("timestamp DESC").Preload("Faces.Matches.Identity")
	
	// Separate Zählabfrage für die Gesamtanzahl der passenden Datensätze
	var totalFiltered int64
	db.Count(&totalFiltered)
	
	// Nur zur Diagnose: Zähle auch, wie viele Bilder es insgesamt gibt
	var totalUnfiltered int64
	h.db.Model(&models.Image{}).Count(&totalUnfiltered)
	
	log.Infof("Filter-Diagnose: %d von %d Bildern entsprechen den Filterkriterien (%.1f%%)", 
		totalFiltered, totalUnfiltered, float64(totalFiltered)/float64(totalUnfiltered)*100)
	
	// Alle gefilterten Daten abrufen
	result := dataQuery.Find(&images)
	
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Laden der Bilder"})
		return
	}
	
	log.Infof("Anzahl geladener Bilder: %d", len(images))
	
	// Bilder nach EventID gruppieren
	eventGroups := make(map[string]*EventGroup)
	singleImages := make([]models.Image, 0)
	
	for i := range images {
		images[i].FileName = filepath.Base(images[i].FilePath)
		
		// Zusätzlich für Frigate-Events: Versuche, Metadaten zu extrahieren
		if images[i].Source == "frigate" && len(images[i].SourceData) > 0 {
			// Versuche, als Frigate-Event zu parsen
			var eventData map[string]interface{}
			if err := json.Unmarshal(images[i].SourceData, &eventData); err == nil {
				// Bestimmte Felder extrahieren, falls vorhanden
				if zones, ok := eventData["current_zones"].([]interface{}); ok && len(zones) > 0 {
					// Zonen als Komma-getrennte Liste
					zoneStrs := make([]string, 0, len(zones))
					for _, z := range zones {
						if zStr, ok := z.(string); ok {
							zoneStrs = append(zoneStrs, zStr)
						}
					}
					images[i].Zone = strings.Join(zoneStrs, ", ")
				}
			}
		}
		
		// Bilder nach EventID gruppieren
		if images[i].EventID != "" {
			// Prüfen, ob bereits eine Gruppe für diese EventID existiert
			group, exists := eventGroups[images[i].EventID]
			
			if !exists {
				// Neue Gruppe erstellen
				group = &EventGroup{
					EventID:      images[i].EventID,
					Images:       make([]models.Image, 0),
					Source:       images[i].Source,
					Label:        images[i].Label,
					Zone:         images[i].Zone,
					Timestamp:    images[i].Timestamp,
					ThumbnailURL: h.getImagePath(images[i].FilePath),
				}
				eventGroups[images[i].EventID] = group
			}
			
			// Bild zur Gruppe hinzufügen
			group.Images = append(group.Images, images[i])
			group.Count = len(group.Images)
			
			// Prüfen, ob dieses Bild Gesichter oder Matches hat
			if len(images[i].Faces) > 0 {
				group.HasFaces = true
				
				// Prüfen, ob Gesichter Matches haben
				for _, face := range images[i].Faces {
					if len(face.Matches) > 0 {
						group.HasMatches = true
						break
					}
				}
				
				// Falls dieses Bild Gesichter hat, als Thumbnail verwenden
				group.ThumbnailURL = h.getImagePath(images[i].FilePath)
			}
		} else {
			// Bilder ohne EventID als einzelne Bilder behandeln
			singleImages = append(singleImages, images[i])
		}
	}
	
	// EventGroups in eine sortierte Liste umwandeln
	groupsList := make([]*EventGroup, 0, len(eventGroups))
	for _, group := range eventGroups {
		groupsList = append(groupsList, group)
	}
	
	// Gruppen nach Timestamp absteigend sortieren
	sort.Slice(groupsList, func(i, j int) bool {
		return groupsList[i].Timestamp.After(groupsList[j].Timestamp)
	})
	
	// Anzahl der Gruppenkarten für Paginierung ermitteln
	total := int64(len(groupsList))
	log.Infof("Anzahl der Gruppenkarten nach Gruppierung: %d", total)
	
	// Alle Items in eine gemeinsame Liste legen
	allItems := make([]interface{}, 0, len(groupsList))
	
	// EventGroups hinzufügen
	for _, group := range groupsList {
		allItems = append(allItems, group)
	}
	
	// Berechnen der korrekten Seitenzahl basierend auf der Gesamtzahl der Items
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1 // Mindestens eine Seite anzeigen
	}
	
	// Korrigieren der aktuellen Seite, falls sie außerhalb des gültigen Bereichs liegt
	if page > totalPages {
		page = totalPages
	}
	
	var itemsToShow []interface{}
	
	// Bei aktiven Filtern alle Ergebnisse anzeigen, sonst paginieren
	if filtersActive {
		// Alle gefilterten Ergebnisse anzeigen
		itemsToShow = allItems
		log.Infof("Filter aktiv: Zeige alle %d Ergebnisse ohne Paginierung", len(itemsToShow))
	} else {
		// Paginierte Teilmenge auswählen
		start := offset
		end := offset + pageSize
		
		if start > len(allItems) {
			start = len(allItems)
		}
		
		if end > len(allItems) {
			end = len(allItems)
		}
		
		itemsToShow = allItems[start:end]
		log.Infof("Keine Filter aktiv: Zeige Elemente %d bis %d (Seite %d von %d)", start+1, end, page, totalPages)
	}
	
	// Verfügbare Quellen für Filter-Dropdown abfragen
	var sources []string
	h.db.Model(&models.Image{}).Distinct().Pluck("source", &sources)
	
	// Verfügbare Labels für Filter-Dropdown abfragen
	var labels []string
	h.db.Model(&models.Image{}).Where("label != ''").Distinct().Pluck("label", &labels)
	
	// Verfügbare Zonen für Filter-Dropdown abfragen
	var zones []string
	h.db.Model(&models.Image{}).Where("zone != ''").Distinct().Pluck("zone", &zones)
	
	// Pagination-Informationen bereits vorbereitet (siehe oben)
	
	// Daten an das Template übergeben
	data := gin.H{
		"Items": itemsToShow, // Gemischte Liste aus Gruppen und einzelnen Bildern
		"Sources": sources,
		"Labels": labels,
		"Zones": zones,
		"FiltersActive": filtersActive, // Flag für das Template, ob Filter aktiv sind
		"Filter": gin.H{
			"Source": source,
			"Label": label,
			"Zone": zone,
			"HasFaces": hasfaces,
			"HasMatches": hasmatches,
			"DateRange": daterange,
		},
		"Pagination": gin.H{
			"Current": page,
			"TotalPages": totalPages,
			"HasPrev": page > 1,
			"HasNext": page < totalPages,
			"PrevPage": page - 1,
			"NextPage": page + 1,
			"Total": total, // Gesamtzahl der Items
			"PageSize": pageSize,
		},
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

	h.renderTemplate(c, "images.html", data)
}

// handleIdentities zeigt die Identitäten-Seite an
func (h *WebHandler) handleIdentities(c *gin.Context) {
	// Alle Identitäten abrufen
	var identities []models.Identity
	if err := h.db.Find(&identities).Error; err != nil {
		log.Errorf("Failed to fetch identities: %v", err)
	}

	// Daten für das Template vorbereiten
	type IdentityData struct {
		ID         uint
		Name       string
		ExternalID string
		MatchCount int64
		BestMatchURL string
	}

	// Identitätsdaten mit zusätzlichen Informationen erstellen
	var identityDataList []IdentityData
	for _, identity := range identities {
		var count int64
		h.db.Model(&models.Match{}).Where("identity_id = ?", identity.ID).Count(&count)

		// Bestes Match-Bild finden
		bestMatchURL := "/static/img/placeholder.png" // Standard-Platzhalter
		
		// Versuchen, ein Matchbild zu finden, wenn Matches vorhanden sind
		if count > 0 {
			var match models.Match
			var face models.Face
			var image models.Image
			
			// Den letzten Match mit einem Bild finden
			if err := h.db.Model(&models.Match{}).Where("identity_id = ?", identity.ID).Order("created_at DESC").First(&match).Error; err == nil {
				if err := h.db.Model(&models.Face{}).Where("id = ?", match.FaceID).First(&face).Error; err == nil {
					if err := h.db.Model(&models.Image{}).Where("id = ?", face.ImageID).First(&image).Error; err == nil {
						bestMatchURL = "/snapshots/" + image.FilePath
					}
				}
			}
		}

		identityDataList = append(identityDataList, IdentityData{
			ID:         identity.ID,
			Name:       identity.Name,
			ExternalID: identity.ExternalID,
			MatchCount: count,
			BestMatchURL: bestMatchURL,
		})
	}

	// Daten für das Template
	data := gin.H{
		"Title": "Identitäten", 
		"CurrentPage": "identities", 
		"Identities": identityDataList,
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

	// Flush-Header, um sicherzustellen, dass Header sofort gesendet werden
	c.Writer.Flush()

	// Client-Kanal erstellen mit größerem Puffer
	client := make(sse.Client, 20) // Puffer für 20 Nachrichten

	// Timeout für Verbindungsabbrüche
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Client beim Hub registrieren
	h.sseHub.Register(client)
	
	// WICHTIG: Unregister wird bei Verbindungsabbruch ausgeführt
	defer func() {
		h.sseHub.Unregister(client)
		log.Debug("SSE client connection closed and unregistered")
	}()

	// Ping-Nachrichten senden, um Verbindung aktiv zu halten
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Kommentar-Nachricht senden, die als Ping dient
				// und vom Browser ignoriert wird
				if _, err := fmt.Fprintf(c.Writer, ": ping %v\n\n", timezone.Now().Unix()); err != nil {
					cancel()
					return
				}
				c.Writer.Flush()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Client-Verbindung überwachen
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-client:
			if !ok {
				return false // Kanal geschlossen, Stream beenden
			}
			// Nachricht im SSE-Format senden
			c.SSEvent("message", string(msg))
			return true
		case <-ctx.Done():
			// Kontext wurde abgebrochen
			return false
		}
	})
}

// handleIdentityDetails zeigt die Detailseite einer Identität an
func (h *WebHandler) handleIdentityDetails(c *gin.Context) {
	id := c.Param("id")

	// Identität aus Datenbank abrufen
	var identity models.Identity
	if err := h.db.First(&identity, id).Error; err != nil {
		log.WithError(err).Warnf("Identität mit ID %s nicht gefunden", id)
		c.Redirect(http.StatusFound, "/identities")
		return
	}

	// Letzten Matches für diese Identität finden
	type matchData struct {
		ID          uint
		ImageID     uint
		ImagePath   string
		Source      string
		Confidence  float64
		Timestamp   time.Time
	}

	var matches []matchData

	// SQL-Query für die Verbindung mehrerer Tabellen
	query := h.db.Table("matches").Select(
		"matches.id, faces.image_id, images.file_path as image_path, " +
		"images.source, matches.confidence, images.created_at as timestamp",
	).Joins(
		"LEFT JOIN faces ON matches.face_id = faces.id",
	).Joins(
		"LEFT JOIN images ON faces.image_id = images.id",
	).Where(
		"matches.identity_id = ?", identity.ID,
	).Order("images.created_at DESC").Limit(12)

	if err := query.Find(&matches).Error; err != nil {
		log.WithError(err).Error("Fehler beim Abrufen der Matches")
	}

	// Statistiken berechnen
	type statsData struct {
		MatchCount     int64
		AvgConfidence  float64
		FirstMatch     time.Time
		LastMatch      time.Time
	}

	var stats statsData

	// Anzahl der Matches
	h.db.Model(&models.Match{}).Where("identity_id = ?", identity.ID).Count(&stats.MatchCount)

	// Durchschnittliches Vertrauen
	h.db.Model(&models.Match{}).Where("identity_id = ?", identity.ID).Select("AVG(confidence)").Row().Scan(&stats.AvgConfidence)

	// Erster und letzter Match
	var firstMatch models.Match
	var lastMatch models.Match
	h.db.Model(&models.Match{}).Where("identity_id = ?", identity.ID).Order("created_at ASC").First(&firstMatch)
	h.db.Model(&models.Match{}).Where("identity_id = ?", identity.ID).Order("created_at DESC").First(&lastMatch)
	stats.FirstMatch = firstMatch.CreatedAt
	stats.LastMatch = lastMatch.CreatedAt

	// Bestes Bild für Avatar finden
	bestMatchURL := ""
	if len(matches) > 0 {
		bestMatchURL = "/snapshots/" + matches[0].ImagePath
	}

	data := gin.H{
		"Title":        identity.Name,
		"CurrentPage":  "identities",
		"Identity":     identity,
		"Matches":      matches,
		"Stats":        stats,
		"BestMatchURL": bestMatchURL,
		"CompreFaceURL": h.cfg.CompreFace.URL,
	}

	h.renderTemplate(c, "identity_detail.html", data)
}

// handleAddTrainingImage verarbeitet die Formularübermittlung zum Hinzufügen eines Trainingsbilds
func (h *WebHandler) handleAddTrainingImage(c *gin.Context) {
	id := c.Param("id")

	// Identität aus Datenbank abrufen
	var identity models.Identity
	if err := h.db.First(&identity, id).Error; err != nil {
		log.WithError(err).Warnf("Identität mit ID %s nicht gefunden", id)
		c.Redirect(http.StatusFound, "/identities")
		return
	}

	// Datei aus Formular erhalten
	file, header, err := c.Request.FormFile("imageFile")
	if err != nil {
		log.WithError(err).Error("Fehler beim Abrufen der Datei aus dem Formular")
		c.Redirect(http.StatusFound, fmt.Sprintf("/identities/%d", identity.ID))
		return
	}
	defer file.Close()

	// Bilddaten lesen
	imageData, err := io.ReadAll(file)
	if err != nil {
		log.WithError(err).Error("Fehler beim Lesen der Bilddaten")
		c.Redirect(http.StatusFound, fmt.Sprintf("/identities/%d", identity.ID))
		return
	}

	// An CompreFace senden
	ctx := c.Request.Context()
	_, err = h.compreface.AddSubjectExample(ctx, identity.Name, imageData, header.Filename)
	if err != nil {
		log.WithError(err).Error("Fehler beim Hinzufügen des Beispiels zu CompreFace")
		c.Redirect(http.StatusFound, fmt.Sprintf("/identities/%d", identity.ID))
		return
	}

	// Zurück zur Identitätsseite
	c.Redirect(http.StatusFound, fmt.Sprintf("/identities/%d", identity.ID))
}

// handleDeleteIdentity verarbeitet die Formularübermittlung zum Löschen einer Identität
func (h *WebHandler) handleDeleteIdentity(c *gin.Context) {
	id := c.Param("id")

	// Identität aus Datenbank abrufen
	var identity models.Identity
	if err := h.db.First(&identity, id).Error; err != nil {
		log.WithError(err).Warnf("Identität mit ID %s nicht gefunden", id)
		c.Redirect(http.StatusFound, "/identities")
		return
	}

	// WICHTIG: Wir priorisieren das Löschen aus der lokalen Datenbank
	// und stellen alle externen Operationen (wie CompreFace-Löschung) in eine Warteschlange
	// So ist die Benutzererfahrung nicht durch Netzwerkprobleme beeinträchtigt

	compreFaceDeleteError := false
	
	// Versuche sofortiges Löschen in CompreFace, falls aktiviert und erreichbar
	if h.cfg.CompreFace.Enabled && h.compreface != nil {
		// Prüfen, ob CompreFace erreichbar ist mit einem kurzen Timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		
		// Versuche direkte Löschung mit Timeout
		_, err := h.compreface.DeleteSubject(ctx, identity.Name)
		if err != nil {
			compreFaceDeleteError = true
			log.WithError(err).Warn("Direktes Löschen des Subjekts in CompreFace fehlgeschlagen - wird in Warteschlange gestellt")
		}
	}

	// Identität in der Datenbank löschen
	if err := h.db.Delete(&identity).Error; err != nil {
		log.WithError(err).Error("Fehler beim Löschen der Identität aus der Datenbank")
		c.Redirect(http.StatusFound, fmt.Sprintf("/identities/%d", identity.ID))
		return
	}

	// Wenn CompreFace-Löschung fehlgeschlagen ist und SyncService verfügbar ist,
	// zur Warteschlange hinzufügen
	if compreFaceDeleteError && h.syncService != nil {
		log.Infof("Identity %s (%d) wird zur Lösch-Warteschlange hinzugefügt", identity.Name, identity.ID)
		err := h.syncService.AddPendingOperation(
			models.POTypeDeleteIdentity,
			models.POResourceIdentity, 
			identity.Name,
			identity.ID,
			nil, // Keine zusätzlichen Daten nötig
		)
		
		if err != nil {
			log.WithError(err).Error("Fehler beim Hinzufügen der Löschoperation zur Warteschlange")
		}
	}

	// Zurück zur Identitätsübersicht
	c.Redirect(http.StatusFound, "/identities")
}

// handleSystemStats gibt aktuelle Systemstatistiken als JSON zurück
func (h *WebHandler) handleSystemStats(c *gin.Context) {
	systemStats := utils.GetSystemStats(h.workerPool)
	c.JSON(http.StatusOK, systemStats)
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
	
	// InsightFace-Status prüfen
	insightFaceStatus := "Unbekannt"
	if h.cfg.InsightFace.Enabled {
		// Prüfen ob InsightFace REST Server erreichbar ist
		client := &http.Client{
			Timeout: 5 * time.Second,
		}
		resp, err := client.Get(h.cfg.InsightFace.URL + "/info")
		if err != nil {
			log.Warnf("Konnte keine Verbindung zu InsightFace herstellen: %v", err)
			insightFaceStatus = "Nicht erreichbar"
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				insightFaceStatus = "Verbunden"
			} else {
				insightFaceStatus = fmt.Sprintf("Fehler: HTTP %d", resp.StatusCode)
			}
		}
	} else {
		insightFaceStatus = "Deaktiviert"
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
	
	// OpenCV-Status und Informationen
	opencvStatus := "Unbekannt"
	opencvDetailedConfig := make(map[string]interface{})
	
	if h.cfg.OpenCV.Enabled {
		opencvStatus = "Aktiviert"
		
		// Detaillierte Konfigurationsinformationen sammeln
		opencvDetailedConfig["Methode"] = h.cfg.OpenCV.PersonDetection.Method
		opencvDetailedConfig["GPU"] = h.cfg.OpenCV.UseGPU
		opencvDetailedConfig["Konfidenz-Schwellenwert"] = h.cfg.OpenCV.PersonDetection.ConfidenceThreshold
		opencvDetailedConfig["Skalierungsfaktor"] = h.cfg.OpenCV.PersonDetection.ScaleFactor
		opencvDetailedConfig["Min. Nachbarn"] = h.cfg.OpenCV.PersonDetection.MinNeighbors
		opencvDetailedConfig["Min. Größe (B x H)"] = fmt.Sprintf("%d x %d", 
			h.cfg.OpenCV.PersonDetection.MinSizeWidth, 
			h.cfg.OpenCV.PersonDetection.MinSizeHeight)
	} else {
		opencvStatus = "Deaktiviert"
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
		"InsightFaceURL": h.cfg.InsightFace.URL,
		"InsightEnabled": h.cfg.InsightFace.Enabled,
		"DataDir":        "/data", // Hardcoded Standardwert, falls nicht in der Config
		"Version":        "1.0.0", // Hier könnte eine Versionsnummer eingetragen werden
		"ActiveProvider": h.cfg.FaceRecognitionProvider,
	}
	
	// Template-Daten
	// Debug-Ausgabe zur Fehlersuche
	log.Infof("OpenCV Status: %s", opencvStatus)
	log.Infof("OpenCV Config Map Größe: %d", len(opencvDetailedConfig))
	// Direkte Debug-Ausgabe des Konfidenz-Schwellenwerts
	log.Infof("KONFIDENZ DEBUG: Wert direkt aus Konfiguration = %v", h.cfg.OpenCV.PersonDetection.ConfidenceThreshold)
	log.Infof("KONFIDENZ DEBUG: Gesamte PersonDetection-Struktur = %+v", h.cfg.OpenCV.PersonDetection)
	// Ausgabe aller Konfigurationsdetails
	for k, v := range opencvDetailedConfig {
		log.Infof("OpenCV Config: %s = %v", k, v)
	}
	
	data := gin.H{
		"DBStats": dbStats,
		"Services": gin.H{
			"CompreFace": compreFaceStatus,
			"InsightFace": insightFaceStatus,
			"MQTT": mqttStatus,
			"OpenCV": opencvStatus,
		},
		"Config": configData,
		"CompreFaceSubjects": compreFaceSubjects,
		"OpenCVConfig": opencvDetailedConfig,
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


// handleUpdateMatch verarbeitet die Aktualisierung eines Treffers mit einer neuen Identität
func (h *WebHandler) handleUpdateMatch(c *gin.Context) {
	// Überprüfe, ob CompreFace aktiviert ist
	if !h.cfg.CompreFace.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CompreFace-Integration ist nicht aktiviert"})
		return
	}

	// ID des zu aktualisierenden Treffers
	id := c.Param("id")
	
	// Formularparameter abrufen
	identityID := c.PostForm("identity_id")
	
	// Validiere die Parameter
	if id == "" || identityID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Parameter"})
		return
	}
	
	// Match in der Datenbank finden
	var match models.Match
	if err := h.db.Preload("Face").Preload("Face.Image").Preload("Identity").First(&match, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Treffer nicht gefunden"})
		return
	}

	// Alte Identität für Logging und Weiterleitung
	oldIdentityName := match.Identity.Name
	
	// Konvertiere identity_id zu uint
	newIdentityID, err := strconv.ParseUint(identityID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Identitäts-ID"})
		return
	}
	
	// Neue Identität in der Datenbank finden
	var newIdentity models.Identity
	if err := h.db.First(&newIdentity, newIdentityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Neue Identität nicht gefunden"})
		return
	}

	// Aktualisieren des Matches in der lokalen Datenbank
	match.IdentityID = uint(newIdentityID)
	match.Identity = newIdentity
	
	if err := h.db.Save(&match).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Fehler beim Aktualisieren des Treffers: %v", err)})
		return
	}

	log.Infof("Treffer %d von Identität %s zu %s aktualisiert", match.ID, oldIdentityName, newIdentity.Name)
	
	// Erfolgreiche Meldung an den Benutzer und Weiterleitung zur Bilddetailseite
	messageKey := fmt.Sprintf("Treffer erfolgreich von '%s' zu '%s' aktualisiert", oldIdentityName, newIdentity.Name)
	c.SetCookie("success_message", messageKey, 300, "/", "", false, true)
	
	// Zurück zur Bilddetailseite
	c.Redirect(http.StatusFound, fmt.Sprintf("/images/%d", match.Face.ImageID))
}
