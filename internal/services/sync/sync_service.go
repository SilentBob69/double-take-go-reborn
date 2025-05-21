package sync

import (
	"context"
	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/integrations/compreface"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Service ist verantwortlich für die Verarbeitung ausstehender Operationen mit externen Diensten
type Service struct {
	db         *gorm.DB
	cfg        *config.Config
	compreface *compreface.APIClient
	stopCh     chan struct{}
	wg         sync.WaitGroup
	running    bool
	mutex      sync.Mutex
}

// NewService erstellt eine neue Instanz des SyncService
func NewService(db *gorm.DB, cfg *config.Config, compreface *compreface.APIClient) *Service {
	return &Service{
		db:         db,
		cfg:        cfg,
		compreface: compreface,
		stopCh:     make(chan struct{}),
	}
}

// Start startet den SyncService
func (s *Service) Start() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return
	}

	s.running = true
	s.stopCh = make(chan struct{})

	s.wg.Add(1)
	go s.processingLoop()

	log.Info("SyncService gestartet")
}

// Stop stoppt den SyncService
func (s *Service) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.wg.Wait()
	s.running = false

	log.Info("SyncService gestoppt")
}

// processingLoop ist die Hauptschleife, die regelmäßig ausstehende Operationen verarbeitet
func (s *Service) processingLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.cfg.Sync.ProcessingInterval) * time.Second)
	defer ticker.Stop()

	// Beim Start einmal sofort ausführen
	s.processPendingOperations()

	for {
		select {
		case <-ticker.C:
			s.processPendingOperations()
		case <-s.stopCh:
			return
		}
	}
}

// processPendingOperations verarbeitet alle ausstehenden Operationen
func (s *Service) processPendingOperations() {
	var pendingOps []models.PendingOperation

	// Nur Operationen mit Versuchen unter dem Maximum und im Status "pending" abrufen
	result := s.db.Where("retries < max_retries AND status = ?", models.POStatusPending).Find(&pendingOps)
	if result.Error != nil {
		log.WithError(result.Error).Error("Fehler beim Abrufen ausstehender Operationen")
		return
	}

	if len(pendingOps) == 0 {
		return // Keine ausstehenden Operationen
	}

	log.Infof("Verarbeite %d ausstehende Operationen", len(pendingOps))

	for i := range pendingOps {
		op := &pendingOps[i] // Pointer für in-place Updates

		// Prüfen, ob wir nach dem Backoff-Algorithmus bereits wieder versuchen sollten
		if !s.shouldRetryNow(op) {
			continue
		}

		success := false
		var err error

		// Operation je nach Typ verarbeiten
		switch op.OperationType {
		case models.POTypeDeleteIdentity:
			err = s.processDeleteIdentity(op)
			success = (err == nil)
		case models.POTypeRenameIdentity:
			err = s.processRenameIdentity(op)
			success = (err == nil)
		case models.POTypeAddExample:
			err = s.processAddExample(op)
			success = (err == nil)
		default:
			log.Warnf("Unbekannter Operationstyp: %s, markiere als fehlgeschlagen", op.OperationType)
			op.Status = models.POStatusFailed
			op.LastError = "Unbekannter Operationstyp"
			s.db.Save(op)
			continue
		}

		// Aktualisierung des Operationsstatus
		op.LastAttempt = time.Now()
		op.Retries++

		if success {
			op.Status = models.POStatusCompleted
			log.Infof("Ausstehende Operation ID %d erfolgreich abgeschlossen: %s für %s",
				op.ID, op.OperationType, op.ResourceName)
		} else {
			op.LastError = err.Error()

			// Bei maximaler Anzahl Versuche erreicht: als fehlgeschlagen markieren
			if op.Retries >= op.MaxRetries {
				op.Status = models.POStatusFailed
				log.Warnf("Ausstehende Operation ID %d nach %d Versuchen als fehlgeschlagen markiert: %s",
					op.ID, op.Retries, op.LastError)
			}
		}

		// Aktualisierte Operation speichern
		if err := s.db.Save(op).Error; err != nil {
			log.WithError(err).Errorf("Fehler beim Speichern der aktualisierten Operation ID %d", op.ID)
		}
	}
}

// shouldRetryNow prüft, ob eine ausstehende Operation jetzt erneut versucht werden sollte
func (s *Service) shouldRetryNow(op *models.PendingOperation) bool {
	if op.LastAttempt.IsZero() {
		return true // Erster Versuch
	}

	// Exponentielles Backoff berechnen
	delaySeconds := float64(s.cfg.Sync.RetryInitialDelay) * math.Pow(s.cfg.Sync.RetryBackoffFactor, float64(op.Retries-1))

	// Delay auf Maximum begrenzen
	if delaySeconds > float64(s.cfg.Sync.RetryMaxDelay) {
		delaySeconds = float64(s.cfg.Sync.RetryMaxDelay)
	}

	// Prüfen, ob genügend Zeit seit dem letzten Versuch vergangen ist
	delay := time.Duration(delaySeconds) * time.Second
	return time.Since(op.LastAttempt) >= delay
}

// AddPendingOperation fügt eine neue ausstehende Operation zur Datenbank hinzu
func (s *Service) AddPendingOperation(opType, resourceType, resourceName string, resourceID uint, data interface{}) error {
	var jsonData []byte
	var err error

	if data != nil {
		jsonData, err = json.Marshal(data)
		if err != nil {
			return fmt.Errorf("Fehler beim Serialisieren der Daten: %w", err)
		}
	}

	op := models.PendingOperation{
		OperationType: opType,
		ResourceType:  resourceType,
		ResourceName:  resourceName,
		ResourceID:    resourceID,
		Data:          jsonData,
		CreatedAt:     time.Now(),
		Status:        models.POStatusPending,
		MaxRetries:    s.cfg.Sync.MaxRetries,
	}

	err = s.db.Create(&op).Error
	if err != nil {
		return fmt.Errorf("Fehler beim Erstellen der ausstehenden Operation: %w", err)
	}

	log.Infof("Ausstehende Operation vom Typ '%s' für '%s' erstellt (ID: %d)", opType, resourceName, op.ID)
	return nil
}

// processDeleteIdentity verarbeitet eine ausstehende Löschoperation für eine Identität
func (s *Service) processDeleteIdentity(op *models.PendingOperation) error {
	if !s.cfg.CompreFace.Enabled || s.compreface == nil {
		return fmt.Errorf("CompreFace ist nicht aktiviert")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := s.compreface.DeleteSubject(ctx, op.ResourceName)
	if err != nil {
		return fmt.Errorf("Fehler beim Löschen des Subjekts in CompreFace: %w", err)
	}

	return nil
}

// processRenameIdentity verarbeitet eine ausstehende Umbenennungsoperation für eine Identität
func (s *Service) processRenameIdentity(op *models.PendingOperation) error {
	if !s.cfg.CompreFace.Enabled || s.compreface == nil {
		return fmt.Errorf("CompreFace ist nicht aktiviert")
	}

	// Für Umbenennungen benötigen wir zusätzliche Daten
	var renameData struct {
		OldName string `json:"old_name"`
		NewName string `json:"new_name"`
	}

	if err := json.Unmarshal(op.Data, &renameData); err != nil {
		return fmt.Errorf("Fehler beim Deserialisieren der Umbenennungsdaten: %w", err)
	}

	// Bei CompreFace muss eine Umbenennung als Lösch- und Neuerstellungsoperation durchgeführt werden
	// TODO: Implementieren, wenn benötigt
	// Beispiel für Implementierung:
	// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// defer cancel()

	return fmt.Errorf("Umbenennung in CompreFace noch nicht implementiert")
}

// processAddExample verarbeitet eine ausstehende Operation zum Hinzufügen eines Beispiels
func (s *Service) processAddExample(op *models.PendingOperation) error {
	if !s.cfg.CompreFace.Enabled || s.compreface == nil {
		return fmt.Errorf("CompreFace ist nicht aktiviert")
	}

	// Für Beispiel-Hinzufügungen benötigen wir zusätzliche Daten
	var addData struct {
		SubjectID  string `json:"subject_id"`
		ImagePath  string `json:"image_path"`
		ExampleID  string `json:"example_id"`
	}

	if err := json.Unmarshal(op.Data, &addData); err != nil {
		return fmt.Errorf("Fehler beim Deserialisieren der Beispieldaten: %w", err)
	}

	// TODO: Implementieren, wenn benötigt

	return fmt.Errorf("Hinzufügen von Beispielen aus ausstehenden Operationen noch nicht implementiert")
}
