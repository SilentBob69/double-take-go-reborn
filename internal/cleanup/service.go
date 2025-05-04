package cleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"double-take-go-reborn/internal/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Service handles the automatic cleanup of old data.
type Service struct {
	db             *gorm.DB
	retentionDays  int
	snapshotDir    string
	checkInterval time.Duration
	stopChan      chan struct{} // Channel to signal stopping the background routine
}

// NewService creates a new CleanupService.
func NewService(db *gorm.DB, retentionDays int, snapshotDir string, checkInterval time.Duration) *Service {
	if retentionDays <= 0 {
		log.Info("Automatic cleanup disabled (retention_days <= 0).")
		return nil // Return nil if cleanup is disabled
	}
	if db == nil {
		log.Error("Cannot initialize CleanupService: database connection is nil")
		return nil
	}
	if snapshotDir == "" {
		log.Error("Cannot initialize CleanupService: snapshot directory is empty")
		return nil
	}
	log.Infof("Initializing CleanupService: RetentionDays=%d, SnapshotDir='%s', CheckInterval=%s", retentionDays, snapshotDir, checkInterval)
	return &Service{
		db:            db,
		retentionDays: retentionDays,
		snapshotDir:   snapshotDir,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
	}
}

// StartBackgroundCleanup starts a goroutine that periodically runs the cleanup cycle.
func (s *Service) StartBackgroundCleanup() {
	if s == nil {
		return // Service was not initialized (cleanup disabled)
	}
	log.Info("Starting background cleanup routine...")

	// Run cleanup once immediately on start
	go func() {
		log.Info("Running initial cleanup check on startup...")
		s.RunCleanupCycle()
	}()

	ticker := time.NewTicker(s.checkInterval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Info("Running scheduled cleanup cycle...")
				s.RunCleanupCycle()
			case <-s.stopChan:
				log.Info("Stopping background cleanup routine.")
				return
			}
		}
	}()
}

// StopBackgroundCleanup signals the background cleanup routine to stop.
func (s *Service) StopBackgroundCleanup() {
	if s == nil || s.stopChan == nil {
		return
	}
	// Check if channel is already closed to prevent panic
	select {
	case <-s.stopChan:
		// Already closed
	default:
		close(s.stopChan)
	}
}

// RunCleanupCycle performs one cleanup cycle, deleting data older than the retention period.
func (s *Service) RunCleanupCycle() {
	if s == nil || s.retentionDays <= 0 {
		log.Debug("Skipping cleanup cycle: service not initialized or cleanup disabled.")
		return
	}

	cutoffTime := time.Now().AddDate(0, 0, -s.retentionDays)
	log.Infof("Cleanup: Deleting records older than %s", cutoffTime.Format(time.RFC3339))

	var imagesToDelete []models.Image
	// Find images based on CreatedAt field from gorm.Model
	if err := s.db.Where("created_at < ?", cutoffTime).Find(&imagesToDelete).Error; err != nil {
		log.Errorf("Cleanup: Error finding old images: %v", err)
		return
	}

	if len(imagesToDelete) == 0 {
		log.Info("Cleanup: No old images found to delete.")
		return
	}

	log.Infof("Cleanup: Found %d image(s) older than retention period to delete.", len(imagesToDelete))
	deletedCount := 0
	failedCount := 0

	for _, img := range imagesToDelete {
		if err := s.deleteImageAndData(img); err != nil {
			log.Errorf("Cleanup: Failed to delete image ID %d (Path: %s): %v", img.ID, img.FilePath, err)
			failedCount++
		} else {
			// Log success inside the loop for clarity
			// log.Infof("Cleanup: Successfully deleted image ID %d (Path: %s) and associated data.", img.ID, img.FilePath)
			deletedCount++
		}
	}

	log.Infof("Cleanup cycle finished. Successfully deleted: %d, Failed: %d", deletedCount, failedCount)
}

// deleteImageAndData handles the deletion of a single image, its DB records, and the file.
func (s *Service) deleteImageAndData(img models.Image) error {
	tx := s.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	imageID := img.ID // For logging clarity
	filePath := img.FilePath // Store before potential deletion

	// 1. Delete associated Matches (related via Faces)
	var faceIDs []uint
	if err := tx.Model(&models.Face{}).Where("image_id = ?", imageID).Pluck("id", &faceIDs).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to find faces for image ID %d: %w", imageID, err)
	}
	if len(faceIDs) > 0 {
		if err := tx.Where("face_id IN ?", faceIDs).Delete(&models.Match{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete matches for image ID %d: %w", imageID, err)
		}
	}

	// 2. Delete associated Faces
	if err := tx.Where("image_id = ?", imageID).Delete(&models.Face{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete faces for image ID %d: %w", imageID, err)
	}

	// 3. Delete the Image record itself
	if err := tx.Delete(&img).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete image record ID %d: %w", imageID, err)
	}

	// 4. Delete the actual snapshot file
	snapshotPath := filepath.Join(s.snapshotDir, filePath)
	if _, err := os.Stat(snapshotPath); err == nil {
		// File exists, attempt to remove it
		if err := os.Remove(snapshotPath); err != nil {
			// Log error but commit transaction anyway, as DB is cleaned
			log.Warnf("Cleanup: Failed to delete snapshot file '%s' for image ID %d, but DB records deleted: %v", snapshotPath, imageID, err)
		} else {
			log.Debugf("Cleanup: Successfully deleted snapshot file '%s' for image ID %d", snapshotPath, imageID)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
        // Log error if it's something other than 'file not found'
        log.Warnf("Cleanup: Error checking snapshot file '%s' before delete: %v", snapshotPath, err)
    }

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		// Rollback might have occurred implicitly on error
		return fmt.Errorf("failed to commit transaction for image ID %d: %w", imageID, err)
	}

	log.Infof("Cleanup: Successfully deleted image ID %d (Path: %s) and associated data.", img.ID, filePath)
	return nil
}
