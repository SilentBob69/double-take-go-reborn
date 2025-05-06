package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Image repräsentiert ein verarbeitetes Bild mit Metadaten
type Image struct {
	gorm.Model
	FilePath    string    `gorm:"uniqueIndex;not null"` // Relativer Pfad zum Bild
	Timestamp   time.Time `gorm:"index"`                // Zeitstempel der Aufnahme
	ContentHash string    `gorm:"index"`                // Hash des Bildinhalts zur Deduplizierung
	Source      string    `gorm:"index"`                // Quelle/Kamera
	EventID     string    `gorm:"index"`                // Frigate-Event-ID
	Label       string    `gorm:"index"`                // z.B. 'person'
	Zone        string    `gorm:"index"`                // Zone in der Kamera
	Faces       []Face    `gorm:"foreignKey:ImageID;constraint:OnDelete:CASCADE;"`
	// Für Frigate-Integration
	FileName    string         `gorm:"-"`                // Temporär, nicht in DB speichern
	DetectedAt  time.Time      `gorm:"-"`               // Zeitpunkt der Erkennung (für Frigate)
	SourceData  datatypes.JSON `gorm:"type:json;null"`  // Rohdaten vom Quellsystem
}

// Face repräsentiert ein erkanntes Gesicht in einem Bild
type Face struct {
	gorm.Model
	ImageID     uint           `gorm:"index;not null"` // Fremdschlüssel zur Image-Tabelle
	BoundingBox datatypes.JSON `gorm:"type:json"`      // JSON-Objekt mit x_min, y_min, x_max, y_max
	Confidence  float64        // Erkennungssicherheit
	Detector    string         `gorm:"index"` // Name des Detektors (z.B. 'compreface')
	Matches     []Match        `gorm:"foreignKey:FaceID;constraint:OnDelete:CASCADE;"`
	Image       Image          `gorm:"foreignKey:ImageID"`
}

// Identity repräsentiert eine bekannte Person
type Identity struct {
	gorm.Model
	Name       string  `gorm:"uniqueIndex;not null"` // Eindeutiger Name der Person
	ExternalID string  `gorm:"index"`                // ID im externen System (z.B. CompreFace)
	Matches    []Match `gorm:"foreignKey:IdentityID"`
}

// Match repräsentiert eine potenzielle Übereinstimmung eines Gesichts mit einer bekannten Identität
type Match struct {
	gorm.Model
	FaceID     uint    `gorm:"index;not null"` // Fremdschlüssel zur Face-Tabelle
	IdentityID uint    `gorm:"index;not null"` // Fremdschlüssel zur Identity-Tabelle
	Confidence float64 // Übereinstimmungssicherheit
	Face       Face    `gorm:"foreignKey:FaceID"`
	Identity   Identity `gorm:"foreignKey:IdentityID"`
}

// Statistics repräsentiert Statistiken über die verarbeiteten Bilder und erkannten Gesichter
type Statistics struct {
	TotalImages      int64       // Gesamtzahl der verarbeiteten Bilder
	TotalFaces       int64       // Gesamtzahl der erkannten Gesichter
	IdentifiedFaces  int64       // Anzahl der identifizierten Gesichter
	UnknownFaces     int64       // Anzahl der unbekannten Gesichter
	LatestImage      time.Time   // Zeitstempel des neuesten Bildes
	IdentityCount    int64       // Anzahl der bekannten Identitäten
	RecentDetections []Image     // Kürzlich erkannte Bilder (optional)
}
