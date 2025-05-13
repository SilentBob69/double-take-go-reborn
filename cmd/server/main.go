package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"strings"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/api/handlers"
	"double-take-go-reborn/internal/core/processor"
	"double-take-go-reborn/internal/db"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/integrations/compreface"
	"double-take-go-reborn/internal/integrations/frigate"
	"double-take-go-reborn/internal/integrations/homeassistant"
	"double-take-go-reborn/internal/integrations/mqtt"
	"double-take-go-reborn/internal/server/sse"
	"double-take-go-reborn/internal/services/cleanup"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
)

const defaultConfigPath = "/config/config.yaml"

func main() {
	// 1. Logger-Konfiguration
	setupLogger()

	// 2. Konfiguration laden
	configPath := getConfigPath()
	log.Infof("Loading configuration from %s", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 3. Logger-Level aus Konfiguration setzen
	if err := configureLogger(cfg); err != nil {
		log.Fatalf("Failed to configure logger: %v", err)
	}

	// Zeitzone setzen
	if cfg.Server.Timezone != "" {
		log.Infof("Setting timezone to %s", cfg.Server.Timezone)
		loc, err := time.LoadLocation(cfg.Server.Timezone)
		if err != nil {
			log.Warnf("Failed to set timezone to %s: %v, using UTC", cfg.Server.Timezone, err)
		} else {
			time.Local = loc
		}
	}

	// 4. Datenbankverbindung initialisieren
	log.Info("Initializing database...")
	if err := db.Initialize(cfg); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Info("Database initialized successfully")

	// 5. CompreFace-Client erstellen
	var compreFaceClient *compreface.Client
	if cfg.CompreFace.Enabled {
		log.Info("CompreFace integration enabled, initializing client...")
		compreFaceClient = compreface.NewClient(cfg.CompreFace)
		
		// CompreFace-Verbindung testen
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		reachable, err := compreFaceClient.Ping(ctx)
		if err != nil || !reachable {
			log.Warnf("CompreFace seems unreachable: %v", err)
		} else {
			log.Info("CompreFace connection successful")
		}
	} else {
		log.Info("CompreFace integration is disabled")
	}

	// 6. SSE-Hub für Echtzeit-Updates initialisieren
	log.Info("Initializing SSE hub...")
	sseHub := sse.NewHub()
	go sseHub.Run()

	// Frigate Client erstellen
	var frigateClient *frigate.FrigateClient
	if cfg.Frigate.Enabled {
		log.Info("Frigate integration enabled, initializing client...")
		frigateClient = frigate.NewFrigateClient(cfg.Frigate)
	} else {
		log.Info("Frigate integration is disabled")
	}

	// 7. Image-Processor erstellen
	log.Info("Initializing image processor...")
	imageProcessor := processor.NewImageProcessor(
		db.DB,
		cfg,
		compreFaceClient,
		sseHub,
		frigateClient,
		nil, // Zunächst ohne Home Assistant Publisher
	)

	// 7.1 Worker-Pool für Bildverarbeitung erstellen
	log.Info("Initializing image processing worker pool...")
	workerPool := processor.NewWorkerPool(imageProcessor)
	
	// Worker-Pool im ImageProcessor registrieren
	imageProcessor.SetWorkerPool(workerPool)
	
	log.Info("Image processor and worker pool initialized successfully")

	// 8. MQTT-Client erstellen, falls aktiviert
	var mqttClient *mqtt.Client
	if cfg.MQTT.Enabled {
		log.Info("MQTT integration enabled, initializing client...")
		mqttClient = mqtt.NewClient(cfg.MQTT)
		
		// Einen Handler registrieren, der MQTT-Nachrichten an den Processor weiterleitet
		mqttHandler := NewMQTTHandler(imageProcessor, cfg)
		mqttClient.RegisterHandler(mqttHandler)
		
		// MQTT-Client starten
		if err := mqttClient.Start(); err != nil {
			log.Fatalf("Failed to start MQTT client: %v", err)
		}
		
		// Home Assistant Integration (falls aktiviert)
		if cfg.MQTT.HomeAssistant.Enabled {
			// Discovery-Manager initialisieren
			discoveryManager := homeassistant.NewDiscoveryManager(mqttClient, cfg)
			
			// Publisher initialisieren
			haPublisher := homeassistant.NewPublisher(mqttClient, cfg)
			
			// Timer für das Zurücksetzen von Personenzählern starten
			haPublisher.StartResetTimers()
			
			// Status als "online" veröffentlichen
			if err := discoveryManager.PublishAvailability(true); err != nil {
				log.Errorf("Failed to publish Home Assistant availability: %v", err)
			}
			
			// Alle bekannten Identitäten aus der Datenbank laden
			var identities []models.Identity
			if err := db.DB.Find(&identities).Error; err != nil {
				log.Errorf("Failed to load identities for Home Assistant discovery: %v", err)
			} else {
				// Discovery-Konfigurationen für alle Identitäten veröffentlichen
				if err := discoveryManager.RegisterIdentities(identities); err != nil {
					log.Errorf("Failed to register Home Assistant sensors: %v", err)
				} else {
					log.Infof("Successfully registered %d identities with Home Assistant", len(identities))
				}
			}
			
			// ImageProcessor mit dem Publisher verbinden
			imageProcessor.SetHomeAssistantPublisher(haPublisher)
			
			// Shutdown-Handler für "offline"-Status
			defer func() {
				if err := discoveryManager.PublishAvailability(false); err != nil {
					log.Errorf("Failed to publish offline status to Home Assistant: %v", err)
				}
			}()
		}
		
		// Sicherstellen, dass der MQTT-Client beim Beenden gestoppt wird
		defer func() {
			if mqttClient != nil {
				mqttClient.Stop()
			}
		}()
	} else {
		log.Info("MQTT integration is disabled")
	}

	// 9. Cleanup-Service starten, falls konfiguriert
	if cfg.Cleanup.RetentionDays > 0 {
		log.Infof("Starting cleanup service with retention days: %d", cfg.Cleanup.RetentionDays)
		cleanupService := cleanup.NewCleanupService(db.DB, cfg.Cleanup, cfg.Server.SnapshotDir)
		go cleanupService.Start(context.Background())
	}

	// 9.1. CompreFace-Synchronisations-Timer starten, falls konfiguriert
	if cfg.CompreFace.Enabled && cfg.CompreFace.SyncIntervalMinutes > 0 {
		log.Infof("Starting CompreFace sync service with interval: %d minutes", cfg.CompreFace.SyncIntervalMinutes)
		ticker := time.NewTicker(time.Duration(cfg.CompreFace.SyncIntervalMinutes) * time.Minute)
		
		// Erste Synchronisation direkt beim Start durchführen
		if err := compreFaceClient.SyncIdentities(context.Background(), db.DB); err != nil {
			log.Errorf("Initial CompreFace sync failed: %v", err)
		} else {
			log.Info("Initial CompreFace synchronization completed successfully")
		}
		
		// Timer für regelmäßige Synchronisation starten
		go func() {
			for {
				select {
				case <-ticker.C:
					if err := compreFaceClient.SyncIdentities(context.Background(), db.DB); err != nil {
						log.Errorf("Periodic CompreFace sync failed: %v", err)
					} else {
						log.Info("Periodic CompreFace synchronization completed successfully")
					}
				}
			}
		}()
		
		// Ticker beim Beenden anhalten
		defer ticker.Stop()
	}

	// 10. Web-Server initialisieren
	router := setupRouter(cfg)

	// 11. Web- und API-Handler erstellen und Routen registrieren
	log.Info("Setting up web and API handlers...")
	
	// Web-Handler
	log.Info("Initializing web handler...")
	webHandler, err := handlers.NewWebHandler(db.DB, cfg, sseHub, workerPool, compreFaceClient)
	if err != nil {
		log.Fatalf("Failed to create web handler: %v", err)
	}
	webHandler.RegisterRoutes(router)
	
	// API-Handler
	apiHandler := handlers.NewAPIHandler(db.DB, cfg, compreFaceClient, imageProcessor)
	apiGroup := router.Group("/api")
	apiHandler.RegisterRoutes(apiGroup)
	
	// Event-Handler
	log.Info("Initializing event handler...")
	eventHandler := handlers.NewEventHandler(db.DB, webHandler)
	eventHandler.RegisterRoutes(router)

	// 12. Server starten
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	// Server in einem separaten Goroutine starten
	go func() {
		log.Infof("Starting server on %s", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 13. Auf Shutdown-Signal warten
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Hauptlogik blockieren, bis Signal empfangen wird
	sig := <-sigChan
	log.Infof("Received signal %v, shutting down gracefully...", sig)
	
	// Signal zum Aufräumen und Schließen der Ressourcen
	// Zuerst den Worker-Pool beenden
	log.Info("Shutting down worker pool...")
	workerPool.Shutdown()

	// Dann Server und andere Dienste stoppen
	log.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited gracefully")
}

// MQTTHandler implementiert das mqtt.MessageHandler-Interface
type MQTTHandler struct {
	processor *processor.ImageProcessor
	cfg       *config.Config
}

// NewMQTTHandler erstellt einen neuen MQTT-Handler
func NewMQTTHandler(processor *processor.ImageProcessor, cfg *config.Config) *MQTTHandler {
	return &MQTTHandler{
		processor: processor,
		cfg:       cfg,
	}
}

// HandleMessage verarbeitet eine MQTT-Nachricht
func (h *MQTTHandler) HandleMessage(topic string, payload []byte) {
	ctx := context.Background()
	log.Debugf("Received MQTT message on topic: %s", topic)
	
	// Überprüfen, ob das Topic zu den relevanten Frigate-Topics gehört
	configTopic := h.cfg.MQTT.Topic

	// Unterstütze sowohl exakte Übereinstimmung als auch Wildcard-Abonnements
	if topic == configTopic || (strings.HasSuffix(configTopic, "/#") && 
		strings.HasPrefix(topic, strings.TrimSuffix(configTopic, "/#")+"/")) {
		// Standard Event in JSON-Format
		log.Infof("Processing JSON event from topic: %s", topic)
		if err := h.processor.ProcessFrigateEvent(ctx, payload); err != nil {
			log.Errorf("Failed to process Frigate event: %v", err)
		}
	} else if strings.Contains(topic, "/person/snapshot") {
		// Person-Snapshot (Binärdaten)
		log.Infof("Processing person snapshot from topic: %s", topic)
		h.processPersonSnapshot(ctx, topic, payload)
	} else {
		log.Debugf("Ignoring MQTT message on topic: %s (expected: %s)", topic, configTopic)
	}
}

// processPersonSnapshot verarbeitet ein Person-Snapshot aus dem MQTT-Topic
func (h *MQTTHandler) processPersonSnapshot(ctx context.Context, topic string, payload []byte) {
	// Extract camera name from topic: frigate/CAMERA_NAME/person/snapshot
	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		log.Errorf("Invalid topic format for person snapshot: %s", topic)
		return
	}
	
	cameraName := parts[1]
	
	// Snapshot-Datei generieren
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s_%s_person.jpg", timestamp, cameraName)
	
	// Snapshot-Verzeichnis aus der Konfiguration auslesen
	configPath := getConfigPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Errorf("Failed to load config for snapshot directory: %v", err)
		return
	}
	
	localPath := filepath.Join("frigate", filename)
	fullPath := filepath.Join(cfg.Server.SnapshotDir, localPath)
	
	// Verzeichnis erstellen, falls es nicht existiert
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		log.Errorf("Failed to create snapshot directory: %v", err)
		return
	}
	
	// Snapshot als Datei speichern
	if err := os.WriteFile(fullPath, payload, 0644); err != nil {
		log.Errorf("Failed to save snapshot to file: %v", err)
		return
	}
	
	log.Infof("Saved person snapshot to: %s", fullPath)
	
	// Bild zur Verarbeitung weiterleiten
	_, err = h.processor.ProcessImage(ctx, fullPath, "frigate", processor.ProcessingOptions{})
	if err != nil {
		log.Errorf("Failed to process snapshot image: %v", err)
		return
	}
}

// setupLogger konfiguriert den Logger
func setupLogger() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel) // Standardwert, wird später möglicherweise überschrieben
}

// configureLogger konfiguriert den Logger mit einer Datei und Stdout
func configureLogger(cfg *config.Config) error {
	// Log-Level setzen
	if level, err := log.ParseLevel(cfg.Log.Level); err == nil {
		log.SetLevel(level)
		log.Infof("Log level set to: %s", cfg.Log.Level)
	} else {
		log.Warnf("Invalid log level '%s', using default 'info': %v", cfg.Log.Level, err)
	}

	// Wenn eine Log-Datei konfiguriert ist, Multi-Writer erstellen
	if cfg.Log.File != "" {
		// Verzeichnis erstellen, falls nicht vorhanden
		logDir := filepath.Dir(cfg.Log.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// Log-Datei öffnen
		logFile, err := os.OpenFile(cfg.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		// Multi-Writer für Stdout und Datei
		mw := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(mw)
		log.Infof("Logging to file: %s and stdout", cfg.Log.File)
	}

	return nil
}

// getConfigPath ermittelt den Pfad zur Konfigurationsdatei
func getConfigPath() string {
	// Umgebungsvariable prüfen
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		return envPath
	}

	// Als Argument übergebenen Pfad prüfen
	if len(os.Args) > 1 {
		return os.Args[1]
	}

	// Standard-Konfigurationspfad
	// Wenn wir im Entwicklungsmodus sind, versuchen wir eine lokale Konfigurationsdatei zu finden
	if _, err := os.Stat("./config/config.yaml"); err == nil {
		return "./config/config.yaml"
	}

	// Versuchen wir es mit einem relativen Pfad zum aktuellen Ausführungsverzeichnis
	execDir, err := os.Executable()
	if err == nil {
		localPath := filepath.Join(filepath.Dir(execDir), "config/config.yaml")
		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}

	// Standard-Container-Pfad
	return defaultConfigPath
}

// setLogLevel setzt das Log-Level basierend auf der Konfiguration
func setLogLevel(levelStr string) {
	if level, err := log.ParseLevel(levelStr); err == nil {
		log.SetLevel(level)
		log.Infof("Log level set to: %s", levelStr)
	} else {
		log.Warnf("Invalid log level '%s', using default 'info': %v", levelStr, err)
	}
}

// setupRouter erstellt und konfiguriert den Gin-Router
func setupRouter(cfg *config.Config) *gin.Engine {
	// Produktions- oder Debug-Modus
	if cfg.Log.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware())
	
	// CORS konfigurieren
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	
	// Hinweis: Die Sprachauswahl wird jetzt direkt in den Handlern implementiert
	
	// Template-Funktionen registrieren
	router.SetFuncMap(template.FuncMap{
		"t":            func(key string) string { return key }, // Standardimplementierung, wird von renderTemplate überschrieben
		"add":          func(a, b int) int { return a + b },
		"subtract":     func(a, b int) int { return a - b },
		"paginationRange": func(current int, total int64) []int {
			start := current - 2
			if start < 1 {
				start = 1
			}
			end := start + 4
			if int64(end) > total {
				end = int(total)
			}
			if end-start < 4 && start > 1 {
				start = end - 4
				if start < 1 {
					start = 1
				}
			}
			result := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
	})

	return router
}

// loggerMiddleware erstellt eine Gin-Middleware für das Logging
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		
		// Request bearbeiten
		c.Next()
		
		// Log nach der Bearbeitung
		end := time.Now()
		latency := end.Sub(start)
		
		if query != "" {
			path = path + "?" + query
		}
		
		log.WithFields(log.Fields{
			"status":     c.Writer.Status(),
			"method":     c.Request.Method,
			"path":       path,
			"ip":         c.ClientIP(),
			"latency":    latency,
			"user-agent": c.Request.UserAgent(),
		}).Info("HTTP request")
	}
}
