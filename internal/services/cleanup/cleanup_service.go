package cleanup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CleanupService ist verantwortlich für die automatische Bereinigung alter Daten
type CleanupService struct {
	db            *gorm.DB
	config        config.CleanupConfig
	snapshotDir   string
	checkInterval time.Duration
}

// NewCleanupService erstellt einen neuen Cleanup-Service
func NewCleanupService(db *gorm.DB, cfg config.CleanupConfig, snapshotDir string) *CleanupService {
	return &CleanupService{
		db:            db,
		config:        cfg,
		snapshotDir:   snapshotDir,
		checkInterval: 24 * time.Hour, // Standardmäßig einmal täglich prüfen
	}
}

// Start startet den Bereinigungsdienst im Hintergrund
func (s *CleanupService) Start(ctx context.Context) {
	log.Info("Cleanup service started")
	
	// Sofort eine erste Bereinigung durchführen
	if err := s.RunCleanup(ctx); err != nil {
		log.Errorf("Initial cleanup failed: %v", err)
	}
	
	// Ticker für regelmäßige Bereinigung einrichten
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			log.Info("Running scheduled cleanup")
			if err := s.RunCleanup(ctx); err != nil {
				log.Errorf("Scheduled cleanup failed: %v", err)
			}
		case <-ctx.Done():
			log.Info("Cleanup service stopped")
			return
		}
	}
}

// RunCleanup führt die eigentliche Bereinigung durch
func (s *CleanupService) RunCleanup(ctx context.Context) error {
	if s.config.RetentionDays <= 0 {
		log.Info("Cleanup disabled (retention days <= 0)")
		return nil
	}
	
	// Berechnungsdatum für Vergleich
	cutoffDate := time.Now().AddDate(0, 0, -s.config.RetentionDays)
	log.Infof("Cleaning up data older than %s", cutoffDate.Format("2006-01-02"))
	
	// 1. Alte Bilder in der Datenbank finden
	var oldImages []models.Image
	if err := s.db.Where("created_at < ?", cutoffDate).Find(&oldImages).Error; err != nil {
		return fmt.Errorf("failed to find old images: %w", err)
	}
	
	log.Infof("Found %d images to clean up", len(oldImages))
	
	// 2. Bilder und zugehörige Dateien löschen
	var deleteCount int
	var errorCount int
	
	for _, image := range oldImages {
		// Physische Datei löschen
		if image.FilePath != "" {
			filePath := filepath.Join(s.snapshotDir, image.FilePath)
			if err := os.Remove(filePath); err != nil {
				if !os.IsNotExist(err) {
					log.Warnf("Failed to delete image file %s: %v", filePath, err)
					errorCount++
				}
			}
		}
		
		// Datenbankeintrag löschen (cascaded zu Faces und Matches durch DB-Constraints)
		if err := s.db.Delete(&image).Error; err != nil {
			log.Errorf("Failed to delete image record ID %d: %v", image.ID, err)
			errorCount++
			continue
		}
		
		deleteCount++
	}
	
	log.Infof("Cleanup completed: deleted %d images, encountered %d errors", deleteCount, errorCount)
	
	// 3. Leere Identitäten ohne Matches bereinigen
	result := s.db.Exec(`
		DELETE FROM identities 
		WHERE id NOT IN (
			SELECT DISTINCT identity_id FROM matches
		)
	`)
	
	if result.Error != nil {
		log.Errorf("Failed to clean up unused identities: %v", result.Error)
	} else {
		log.Infof("Removed %d unused identities", result.RowsAffected)
	}
	
	return nil
}
