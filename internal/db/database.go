package db

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"

	"github.com/glebarez/sqlite" // Pure Go SQLite Treiber
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB ist die globale Datenbankverbindung
var DB *gorm.DB

// Initialize initialisiert die Datenbankverbindung
func Initialize(cfg *config.Config) error {
	// Sicherstellen, dass das Verzeichnis für die Datenbankdatei existiert
	if cfg.DB.File != "" {
		dbDir := filepath.Dir(cfg.DB.File)
		if err := os.MkdirAll(dbDir, 0750); err != nil {
			log.Errorf("Failed to create database directory '%s': %v", dbDir, err)
			return fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Konfiguration des GORM-Loggers
	gormLogger := logger.New(
		log.StandardLogger(), // Verwende den konfigurierten logrus-Logger
		logger.Config{
			SlowThreshold:             time.Second * 2, // SQL-Abfragen langsamer als 2 Sekunden werden geloggt
			LogLevel:                  logger.Warn,     // Log-Level (Silent, Error, Warn, Info)
			IgnoreRecordNotFoundError: true,            // ErrRecordNotFound wird nicht geloggt
			Colorful:                  false,           // Keine farbige Ausgabe
		},
	)

	var err error
	log.Infof("Connecting to database: %s", cfg.DB.File)

	// SQLite-Verbindung öffnen
	DB, err = gorm.Open(sqlite.Open(cfg.DB.File), &gorm.Config{
		Logger: gormLogger,
	})

	if err != nil {
		log.Errorf("Failed to connect to database: %v", err)
		return fmt.Errorf("database connection failed: %w", err)
	}

	// Optional: Konfiguration des Connection-Pools
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	
	// Verbindungs-Pool-Einstellungen
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Info("Database connection established successfully")

	// Auto-Migrationen durchführen
	log.Info("Running database migrations...")
	if err := DB.AutoMigrate(
		&models.Image{},
		&models.Face{},
		&models.Identity{},
		&models.Match{},
		&models.PendingOperation{},
	); err != nil {
		log.Errorf("Database migration failed: %v", err)
		return fmt.Errorf("database migration failed: %w", err)
	}
	
	log.Info("Database migrations completed successfully")
	return nil
}

// GetDB gibt die initialisierte GORM-DB-Instanz zurück
func GetDB() (*gorm.DB, error) {
	if DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	return DB, nil
}
