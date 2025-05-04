package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Using a common base model can help with standard fields like ID, CreatedAt, UpdatedAt
// type BaseModel struct {
// 	ID        uint           `gorm:"primarykey"`
// 	CreatedAt time.Time
// 	UpdatedAt time.Time
// 	DeletedAt gorm.DeletedAt `gorm:"index"`
// }

// Image represents a processed image file.
// We might need more fields later (e.g., camera source, width, height).
type Image struct {
	gorm.Model        // Includes ID, CreatedAt, UpdatedAt, DeletedAt
	FilePath  string `gorm:"uniqueIndex;not null"` // Path to the image file
	Timestamp time.Time `gorm:"index"`           // Timestamp of the image (e.g., from EXIF or file mod time)
	Faces     []Face    `gorm:"foreignKey:ImageID"` // Explicit one-to-many relationship
	ContentHash string   `gorm:"index"`           // SHA-256 hash of the image content, indexed for deduplication
	FrigateEventID string `gorm:"index"`           // ID of the Frigate event that triggered this image (indexed for checking duplicates)
}

// Face represents a detected face within an image.
type Face struct {
	gorm.Model        // Includes ID, CreatedAt, UpdatedAt, DeletedAt
	ImageID   uint   `gorm:"index;not null"` // Foreign key to Image
	Detector  string `gorm:"index"`           // Name of the detector used (e.g., 'compreface', 'deepstack')
	// Bounding box coordinates (consider JSON or separate fields)
	Box datatypes.JSON `gorm:"type:json"` // Example: {"x_min": 10, "y_min": 20, "width": 50, "height": 60}
	// Embedding might be large, consider how to store efficiently if needed.
	// Embedding datatypes.JSON `gorm:"type:json"` // Example: [0.1, 0.2, ..., 1.5]
	Matches   []Match   `gorm:"foreignKey:FaceID"` // Explicit one-to-many relationship
	Image     Image     `gorm:"foreignKey:ImageID"` // Explicit belongs-to relationship
}

// Identity represents a known person.
type Identity struct {
	gorm.Model        // Includes ID, CreatedAt, UpdatedAt, DeletedAt
	Name    string `gorm:"uniqueIndex;not null"` // Unique name/identifier for the person
	Matches []Match `gorm:"foreignKey:IdentityID"` // Explicit one-to-many relationship
}

// Match represents a successful match between a detected Face and a known Identity.
type Match struct {
	gorm.Model        // Includes ID, CreatedAt, UpdatedAt, DeletedAt
	FaceID     uint    `gorm:"index;not null"` // Foreign key to Face
	IdentityID uint    `gorm:"index;not null"` // Foreign key to Identity
	Confidence float64 // Confidence score of the match (0.0 to 1.0)
	Timestamp  time.Time `gorm:"index"`        // Timestamp when the match was recorded
	Detector   string    `gorm:"index"`        // Detector that produced the face leading to the match
	Face       Face      `gorm:"foreignKey:FaceID"`     // Explicit belongs-to relationship
	Identity   Identity  `gorm:"foreignKey:IdentityID"` // Explicit belongs-to relationship
}

// Note: Relationships (like `Faces []Face` in Image) are commented out for now.
// GORM can often infer them, but explicit definitions can be added later
// once the interaction logic is clearer. Using simple foreign keys (ImageID, FaceID, etc.)
// is often sufficient initially. -> // Updated: Added explicit relationships for Preload
