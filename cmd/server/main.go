package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/api/handlers"
	"double-take-go-reborn/internal/core/processor"
	"double-take-go-reborn/internal/db"
	"double-take-go-reborn/internal/integrations/compreface"
	"double-take-go-reborn/internal/integrations/frigate"
	"double-take-go-reborn/internal/integrations/mqtt"
	"double-take-go-reborn/internal/server/sse"
	"double-take-go-reborn/internal/services/cleanup"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
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
	setLogLevel(cfg.Log.Level)

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
	imageProcessor := processor.NewImageProcessor(db.DB, cfg, compreFaceClient, sseHub, frigateClient)

	// 8. MQTT-Client erstellen, falls aktiviert
	var mqttClient *mqtt.Client
	if cfg.MQTT.Enabled {
		log.Info("MQTT integration enabled, initializing client...")
		mqttClient = mqtt.NewClient(cfg.MQTT)
		
		// Einen Handler registrieren, der MQTT-Nachrichten an den Processor weiterleitet
		mqttClient.RegisterHandler(NewMQTTHandler(imageProcessor))
		
		// MQTT-Client starten
		if err := mqttClient.Start(); err != nil {
			log.Warnf("Failed to start MQTT client: %v", err)
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

	// 10. Web-Server initialisieren
	router := setupRouter(cfg)

	// 11. Web- und API-Handler erstellen und Routen registrieren
	log.Info("Setting up web and API handlers...")
	
	// Web-Handler
	webHandler, err := handlers.NewWebHandler(db.DB, cfg, sseHub)
	if err != nil {
		log.Fatalf("Failed to create web handler: %v", err)
	}
	webHandler.RegisterRoutes(router)
	
	// API-Handler
	apiHandler := handlers.NewAPIHandler(db.DB, cfg, compreFaceClient, imageProcessor)
	apiGroup := router.Group("/api")
	apiHandler.RegisterRoutes(apiGroup)

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
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutdown signal received")

	// 14. Graceful Shutdown
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
}

// NewMQTTHandler erstellt einen neuen MQTT-Handler
func NewMQTTHandler(processor *processor.ImageProcessor) *MQTTHandler {
	return &MQTTHandler{processor: processor}
}

// HandleMessage verarbeitet eine MQTT-Nachricht
func (h *MQTTHandler) HandleMessage(topic string, payload []byte) {
	ctx := context.Background()
	if err := h.processor.ProcessFrigateEvent(ctx, payload); err != nil {
		log.Errorf("Failed to process Frigate event: %v", err)
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
