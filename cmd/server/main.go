package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // Import for pprof
	"os"
	"os/signal"
	"syscall"
	"time"

	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/database"
	"double-take-go-reborn/internal/handlers"
	"double-take-go-reborn/internal/mqtt"
	"double-take-go-reborn/internal/services"
	"double-take-go-reborn/internal/sse" // Add sse import

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const configPath = "/config/config.yaml" // Define config path, assuming mount

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.InfoLevel) // Default level

	// Load configuration
	cfg, err := config.Load(configPath) // Use config.Load and defined path
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// --- Begin Debug ---
	log.Debugf("Config loaded: %+v", cfg)
	// --- End Debug ---

	// Initialize logger level from config
	if level, err := log.ParseLevel(cfg.Log.Level); err == nil {
		log.SetLevel(level)
		log.Infof("Logger level set to: %s", cfg.Log.Level)
	} else {
		log.Warnf("Invalid log level in config '%s', using default 'info': %v", cfg.Log.Level, err)
	}

	// Initialize Database
	// Use global database.DB, Init sets it
	if err := database.Init(cfg.DB); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Info("Database initialized successfully.")

	// Initialize MQTT Client
	var mqttClient *mqtt.Client
	if cfg.MQTT.Enabled {
		// Pass db, cfg, and nil for sseHub to NewMQTTClient
		mqttClient, err = mqtt.NewMQTTClient(cfg.MQTT, database.DB, cfg, nil)
		if err != nil {
			log.Warnf("Failed to initialize MQTT client: %v. MQTT features will be disabled.", err)
			mqttClient = nil // Ensure client is nil if init fails
		} else if mqttClient != nil { // Check if client was actually created (could be nil if !cfg.Enabled even without error)
			// Start MQTT client in a goroutine if initialization was successful
			go func() {
				if err := mqttClient.Start(); err != nil {
					log.Errorf("MQTT client error: %v", err)
				}
			}()
			defer mqttClient.Stop() // Ensure Stop is called on shutdown
			log.Info("MQTT client initialized and started.")
		} else {
			// This case might happen if NewMQTTClient returns nil without error when disabled internally
			log.Info("MQTT client initialization returned nil, likely disabled internally.")
		}
	} else {
		log.Info("MQTT is disabled in the configuration.")
	}

	// Initialize CompreFace Service
	compreService := services.NewCompreFaceService(cfg.CompreFace)
	if cfg.CompreFace.Enabled { // Check cfg.CompreFace.Enabled directly
		log.Info("CompreFace service initialized.")
		// Start CompreFace identity sync
		go func() {
			// Initial sync immediately
			log.Info("Running initial CompreFace identity synchronization...")
			if err := compreService.SyncIdentities(database.DB); err != nil {
				log.Errorf("Error during initial CompreFace identity sync: %v", err)
			} else {
				log.Info("Initial CompreFace identity synchronization finished successfully.")
			}
			// Periodic sync based on interval
			if cfg.CompreFace.SyncIntervalMinutes > 0 {
				ticker := time.NewTicker(time.Duration(cfg.CompreFace.SyncIntervalMinutes) * time.Minute)
				defer ticker.Stop()
				log.Infof("Starting periodic CompreFace sync every %d minutes...", cfg.CompreFace.SyncIntervalMinutes)
				for {
					<-ticker.C
					log.Info("Running periodic CompreFace identity synchronization...")
					if err := compreService.SyncIdentities(database.DB); err != nil {
						log.Errorf("Error during periodic CompreFace identity sync: %v", err)
					} else {
						log.Info("Periodic CompreFace identity synchronization finished successfully.")
					}
				}
			} else {
				log.Info("Periodic CompreFace sync disabled (interval is 0).")
			}
		}()
	} else {
		log.Info("CompreFace service is disabled.")
	}

	// Initialize Notifier Service
	notifier := services.NewNotifierService()

	// Initialize SSE Hub
	sseHub := sse.NewHub() // Use sse.NewHub()
	go sseHub.Run() // Run hub in a separate goroutine

	// Initialize Web Handler
	templatePath := "/app/web/templates" // Absolute path inside container
	staticPath := "/app/web/static"     // Absolute path inside container
	webHandler, err := handlers.NewWebHandler(database.DB, cfg, compreService, mqttClient, templatePath, staticPath, sseHub) // Pass sseHub
	if err != nil {
		log.Fatalf("Failed to initialize WebHandler: %v", err)
	}

	// Initialize API Handler
	// Removed mqttClient and sseHub as they are not needed by APIHandler
	apiHandler := handlers.NewAPIHandler(database.DB, cfg, compreService, notifier)

	// Initialize Processing Handler
	// Corrected argument order: cfg, database.DB, ...
	processingHandler := handlers.NewProcessingHandler(cfg, database.DB, compreService, notifier, mqttClient) // Pass MQTT client here

	// Setup Gin router
	router := gin.Default()

	// CORS Middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Allow all origins (adjust for production)
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Serve static files for the UI using absolute path inside container
	router.Static("/ui", "/app/ui/public") // Use absolute path

	// --- Setup API Handlers & Router ---
	// Instantiate the API handler with necessary dependencies
	// (Already done above)

	// Register API routes under /api prefix
	apiGroup := router.Group("/api")
	{
		// Register API routes
		apiHandler.RegisterRoutes(apiGroup)

		// Register Processing routes
		processingGroup := apiGroup.Group("/process")
		{
			processingGroup.POST("/compreface", processingHandler.ProcessCompreFace)
			// Add other processing routes here if needed
		}
	}

	// Register Web routes
	// Use an empty group to register at the root level
	webGroup := router.Group("")
	webHandler.RegisterRoutes(webGroup)

	// Serve snapshot images from the /data/snapshots directory
	snapshotDir := cfg.Server.SnapshotDir // Use path from config (/data/snapshots)
	router.Static("/snapshots", snapshotDir)
	log.Infof("Serving snapshots from %s under /snapshots/ route", snapshotDir)

	// Start the server
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Infof("Starting server on %s", serverAddr)
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: router, // Use the main gin router
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutdown signal received")

	// The context is used to inform the server it has 10 seconds to finish the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Info("Server exiting")
}
