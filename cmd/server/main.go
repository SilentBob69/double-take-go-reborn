package main

import (
	"fmt"
	"net/http"
	"time"

	"double-take-go-reborn/internal/cleanup"
	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/database"
	"double-take-go-reborn/internal/handlers"
	"double-take-go-reborn/internal/logger"
	"double-take-go-reborn/internal/mqtt"
	"double-take-go-reborn/internal/services"
	"double-take-go-reborn/internal/sse"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
)

const configPath = "/config/config.yaml"

func main() {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		// Use logrus fatal even before full initialization if config fails
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// --- Debug: Log the loaded CompreFace enabled status BEFORE logger init ---
	// Use standard fmt.Printf here as logger might not be ready
	fmt.Printf("DEBUG: Loaded config value: CompreFace.Enabled = %t\n", cfg.CompreFace.Enabled)
	// --- End Debug ---

	// Initialize logger
	if err := logger.Init(cfg.Log); err != nil {
		// Log the error but continue, the logger might have defaulted
		log.Errorf("Failed to initialize logger completely: %v", err)
	}

	// Initialize database connection
	log.Info("Initializing database...")
	// Call Init and check only the error, Init sets global database.DB
	if err := database.Init(cfg.DB); err != nil {
		// Decide if the application can run without a database
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Info("Database initialization complete.")

	// Initialize Cleanup Service
	// Use global database.DB
	cleanupService := cleanup.NewService(database.DB, cfg.Cleanup.RetentionDays, cfg.Server.SnapshotDir, 24*time.Hour)
	if cleanupService != nil {
		cleanupService.StartBackgroundCleanup()
	}

	// Initialize CompreFace service
	compreService := services.NewCompreFaceService(cfg.CompreFace)

	// --- Initial Identity Sync ---
	if cfg.CompreFace.Enabled {
		log.Info("Performing initial CompreFace identity synchronization...")
		if err := compreService.SyncIdentities(database.DB); err != nil {
			log.WithError(err).Error("Initial CompreFace identity synchronization failed")
			// Continue running even if sync fails initially?
		} else {
			log.Info("Initial CompreFace identity synchronization completed.")
		}

		// --- Start Periodic Identity Sync Goroutine ---
		if cfg.CompreFace.SyncIntervalMinutes > 0 {
			go func() {
				ticker := time.NewTicker(time.Duration(cfg.CompreFace.SyncIntervalMinutes) * time.Minute)
				defer ticker.Stop()
				log.Infof("Starting periodic CompreFace identity sync every %d minutes", cfg.CompreFace.SyncIntervalMinutes)
				for range ticker.C {
					log.Info("Running periodic CompreFace identity synchronization...")
					if err := compreService.SyncIdentities(database.DB); err != nil {
						log.WithError(err).Error("Periodic CompreFace identity synchronization failed")
					} else {
						log.Info("Periodic CompreFace identity synchronization completed.")
					}
				}
			}()
		} else {
			log.Info("Periodic CompreFace identity sync disabled (interval set to 0).")
		}
	}

	// Initialize MQTT Client if enabled
	var mqttClient *mqtt.Client
	if cfg.MQTT.Enabled {
		var err error
		// Use global database.DB
		mqttClient, err = mqtt.NewMQTTClient(cfg.MQTT, database.DB, cfg, sse.NewHub())
		if err != nil {
			log.Warnf("Failed to initialize MQTT client: %v. Continuing without MQTT.", err)
			mqttClient = nil // Ensure client is nil if initialization failed but wasn't fatal
		} else {
			// Start MQTT client in a goroutine
			go func() {
				if err := mqttClient.Start(); err != nil {
					log.Errorf("MQTT client error: %v", err)
					// Handle client stopping unexpectedly, maybe attempt reconnect or log critical error
				}
			}()
			defer mqttClient.Stop()
		}
	} else {
		log.Info("MQTT is disabled in config.")
	}

	// Initialize Notifier Service
	notifier := services.NewNotifierService()

	// --- Setup Web Handlers --- 
	// Use global database.DB and provide all required arguments
	webHandler, err := handlers.NewWebHandler(database.DB, cfg, compreService, mqttClient, "web/templates", "web/static", sse.NewHub())
	if err != nil {
		log.Fatalf("Failed to initialize web handlers: %v", err)
	}

	// --- Setup API Handlers & Router --- 
	// Instantiate the API handler with necessary dependencies
	// Use global database.DB
	apiHandler := handlers.NewAPIHandler(database.DB, cfg, compreService, notifier) // Removed sseHub argument

	// Create a new router for API endpoints
	apiRouter := chi.NewRouter()
	apiRouter.Use(middleware.Recoverer) // Add basic middleware
	apiRouter.Use(middleware.RequestID)
	apiRouter.Use(middleware.Logger)    // Log API requests

	// Register API routes
	apiHandler.RegisterRoutes(apiRouter)

	// --- Setup Web Router --- 
	// Create a new router for web endpoints
	webRouter := chi.NewRouter()
	webRouter.Use(middleware.Recoverer)

	// Register Web handlers
	webHandler.RegisterRoutes(webRouter)

	// --- Setup Main HTTP Router --- 
	// Use chi as the main router to easily mount sub-routers
	mainRouter := chi.NewRouter()
	mainRouter.Use(middleware.Recoverer)

	// Mount the web router (serving UI, static files, SSE)
	mainRouter.Mount("/", webRouter)

	// Mount the API router under the /api prefix
	mainRouter.Mount("/api", apiRouter)

	// Serve snapshot images from the /data/snapshots directory
	snapshotDir := cfg.Server.SnapshotDir // Use path from config (/data/snapshots)
	fs := http.FileServer(http.Dir(snapshotDir))
	// Mount the file server under /snapshots/, stripping the prefix
	mainRouter.Mount("/snapshots", http.StripPrefix("/snapshots", fs))
	log.Infof("Serving snapshots from %s under /snapshots/ route", snapshotDir)

	// Start the server
	serverAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.Port)
	log.Infof("Starting server on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, mainRouter); err != nil { // Use mainRouter
		log.Fatalf("Server failed to start: %v", err)
	}

	// Stop Cleanup Service
	if cleanupService != nil {
		cleanupService.StopBackgroundCleanup()
		log.Info("Cleanup service stopped.")
	}

	log.Info("Server stopped.")
}
