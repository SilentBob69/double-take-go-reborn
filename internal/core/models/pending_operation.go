package models

import (
	"time"
)

// PendingOperation repräsentiert eine ausstehende Operation für externe Dienste
// wie CompreFace, die aufgrund von Verbindungsproblemen nicht abgeschlossen werden konnte
// und später erneut versucht werden soll.
type PendingOperation struct {
	ID            uint      `gorm:"primaryKey"`
	OperationType string    `gorm:"index;not null"` // "delete_identity", "rename_identity", etc.
	ResourceType  string    `gorm:"index;not null"` // "identity", etc.
	ResourceName  string    `gorm:"not null"`       // Name der Ressource (z.B. Identity-Name für CompreFace)
	ResourceID    uint      // Optional: ID der Ressource in der Datenbank
	Data          []byte    // JSON-Daten für die Operation (optional)
	CreatedAt     time.Time `gorm:"index"`
	LastAttempt   time.Time // Zeitpunkt des letzten Versuchs
	Retries       int       `gorm:"default:0"` // Anzahl der bisherigen Versuche
	MaxRetries    int       `gorm:"default:5"` // Maximale Anzahl Versuche
	LastError     string    // Letzte Fehlermeldung
	Status        string    `gorm:"index;default:'pending'"` // "pending", "failed", "completed"
}

// PendingOperationTypes definiert die möglichen Operationstypen
const (
	POTypeDeleteIdentity = "delete_identity"
	POTypeRenameIdentity = "rename_identity"
	POTypeAddExample     = "add_example"
)

// PendingOperationStatus definiert die möglichen Status
const (
	POStatusPending   = "pending"
	POStatusFailed    = "failed"
	POStatusCompleted = "completed"
)

// PendingOperationResourceTypes definiert die möglichen Ressourcentypen
const (
	POResourceIdentity = "identity"
	POResourceExample  = "example"
)
