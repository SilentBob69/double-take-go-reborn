package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"double-take-go-reborn/internal/config"

	"github.com/glebarez/sqlite" // Pure Go
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
	log "github.com/sirupsen/logrus"
)

// --- Struct Definitions ---

// Image represents a processed snapshot image event.
type Image struct {
	gorm.Model        // Adds ID, CreatedAt, UpdatedAt, DeletedAt
	EventID     string    `gorm:"index"` // Frigate event ID
	Camera      string    `gorm:"index"`
	Label       string    `gorm:"index"` // e.g., 'person'
	Zone        string    `gorm:"index"`
	Filename    string    `gorm:"uniqueIndex"` // Unique filename on disk
	Timestamp   time.Time `gorm:"index"`
	ContentHash string    `gorm:"uniqueIndex"` // Hash of image content to prevent duplicates
	ProcessedAt time.Time
	Faces       []Face `gorm:"foreignKey:ImageID;constraint:OnDelete:CASCADE;"` // One-to-many relationship
}

// Face represents a detected face within an Image.
type Face struct {
	gorm.Model        // Adds ID, CreatedAt, UpdatedAt, DeletedAt
	ImageID     uint      `gorm:"index"` // Foreign key to Image
	BoundingBox string    // e.g., "x,y,w,h"
	Confidence  float64   // Detection confidence
	Matches     []Match `gorm:"foreignKey:FaceID;constraint:OnDelete:CASCADE;"` // One-to-many relationship
}

// Identity represents a recognized person/identity.
type Identity struct {
	gorm.Model        // Adds ID, CreatedAt, UpdatedAt, DeletedAt
	Name    string `gorm:"uniqueIndex"` // The unique name of the person
	Matches []Match `gorm:"foreignKey:IdentityID"` // One-to-many relationship (optional, depends on query needs)
}

// Match represents a potential match between a detected Face and a known Identity.
type Match struct {
	gorm.Model        // Adds ID, CreatedAt, UpdatedAt, DeletedAt
	FaceID     uint      `gorm:"index"` // Foreign key to Face
	IdentityID uint      `gorm:"index"` // Foreign key to Identity
	Confidence float64   // Recognition confidence
	Identity   Identity  `gorm:"foreignKey:IdentityID"` // Eager load Identity info
}

// Recognition represents a result from a face recognition service for a specific image.
// Used by the re-recognition API endpoint.
type Recognition struct {
	gorm.Model
	ImageID    int64   `gorm:"index"` // Foreign key to Image
	IdentityID int64   `gorm:"index"` // Foreign key to Identity (or 0 if unknown)
	Subject    string  // Name returned by recognition service (e.g., "Hans", "unknown")
	Confidence float64 // Confidence score from recognition service
	BoxX       int
	BoxY       int
	BoxWidth   int
	BoxHeight  int
}

// --- Global DB Connection ---

// DB holds the global GORM database connection pool.
var DB *gorm.DB

// Init initializes the database connection using the provided configuration.
func Init(cfg config.DBConfig) error {
	// Ensure the directory for the database file exists
	dbDir := filepath.Dir(cfg.File)
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		log.Errorf("Failed to create database directory '%s': %v", dbDir, err)
		return err
	}

	// Configure GORM logger to use our logrus instance
	gormConfiguredLogger := gormlog.New(
		log.StandardLogger(), // Use the configured logrus standard logger
		gormlog.Config{ // Use gormlog.Config
			SlowThreshold:             time.Second * 2, // Slow SQL threshold
			LogLevel:                  gormlog.Warn,    // Log level (Silent, Error, Warn, Info) - Use gormlog.Warn
			IgnoreRecordNotFoundError: true,            // Don't log ErrRecordNotFound
			Colorful:                  false,           // Disable color
		},
	)

	log.Infof("Connecting to database: %s", cfg.File)
	db, err := gorm.Open(sqlite.Open(cfg.File), &gorm.Config{
		Logger: gormConfiguredLogger, // Use the correctly configured GORM logger
	})
	if err != nil {
		log.Errorf("Failed to connect to database '%s': %v", cfg.File, err)
		return err
	}

	// Set the global DB instance
	DB = db

	log.Info("Database connection established.")

	// Run migrations
	log.Info("Running database migrations...")
	err = DB.AutoMigrate(
		&Image{},
		&Face{},
		&Identity{},
		&Match{},
		&Recognition{},
	)
	if err != nil {
		log.Errorf("Database migration failed: %v", err)
		return err // Or handle more gracefully, depending on requirements
	}
	log.Info("Database migrations completed.")

	return nil
}

// GetDB returns the initialized GORM DB instance.
// It's a helper function to avoid direct access to the global variable if preferred.
func GetDB() (*gorm.DB, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	return DB, nil
}

// GetImageByID fetches a single Image record by its primary key ID using GORM.
func GetImageByID(id int64) (*Image, error) {
	var img Image
	result := DB.First(&img, id) // Use GORM's First method
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil // Not found is not an error here
		}
		return nil, fmt.Errorf("failed to get image by ID %d: %w", id, result.Error)
	}
	return &img, nil
}

// DeleteRecognitionsByImageIDTx deletes all recognition records associated with a given image ID within a GORM transaction.
func DeleteRecognitionsByImageIDTx(tx *gorm.DB, imageID int64) error {
	// Use GORM's Delete method within the transaction
	result := tx.Where("image_id = ?", imageID).Delete(&Recognition{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete recognitions for image ID %d: %w", imageID, result.Error)
	}
	rowsAffected := result.RowsAffected
	log.Debugf("Deleted %d recognition records for image ID %d in transaction", rowsAffected, imageID)
	return nil
}

// InsertRecognition inserts a new recognition record.
func InsertRecognition(rec *Recognition) (uint, error) {
	result := DB.Create(rec) // Use GORM's Create method
	if result.Error != nil {
		return 0, fmt.Errorf("failed to insert recognition: %w", result.Error)
	}
	// GORM automatically populates the ID field of the passed struct
	return rec.ID, nil
}

// InsertRecognitionTx inserts a new recognition record within a GORM transaction.
func InsertRecognitionTx(tx *gorm.DB, rec *Recognition) (uint, error) {
	// Use GORM's Create method within the transaction
	result := tx.Create(rec)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to insert recognition in transaction: %w", result.Error)
	}
	// GORM automatically populates the ID field of the passed struct
	return rec.ID, nil
}
