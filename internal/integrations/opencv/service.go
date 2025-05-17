package opencv

import (
	"context"
	"fmt"
	"sync"

	"double-take-go-reborn/config"

	log "github.com/sirupsen/logrus"
)

// Service ist der Hauptdienst für die OpenCV-Integration
type Service struct {
	cfg       *config.OpenCVConfig
	detector  *PersonDetector
	DebugSvc  *DebugService // Debug-Service für die Visualisierung
	mutex     sync.Mutex
	initialized bool
}

// NewService erstellt einen neuen OpenCV-Service
func NewService(cfg *config.OpenCVConfig) (*Service, error) {
	// Debug-Service erstellen
	debugSvc := NewDebugService(30) // speichere bis zu 30 Debug-Bilder
	
	service := &Service{
		cfg:         cfg,
		DebugSvc:    debugSvc,
		initialized: false,
	}

	if cfg.Enabled {
		if err := service.initialize(); err != nil {
			return nil, fmt.Errorf("fehler beim Initialisieren des OpenCV-Service: %w", err)
		}
	} else {
		log.Info("OpenCV-Service ist deaktiviert in der Konfiguration")
	}

	return service, nil
}

// initialize initialisiert den OpenCV-Service
func (s *Service) initialize() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.initialized {
		return nil
	}

	var err error
	s.detector, err = NewPersonDetector(s.cfg)
	if err != nil {
		return fmt.Errorf("konnte OpenCV Personendetektor nicht initialisieren: %w", err)
	}
	
	// Debug-Service an den Detektor weitergeben
	s.detector.debugService = s.DebugSvc

	// Initialisiere den Detektor mit Standardkontext
	ctx := context.Background()
	if err := s.detector.Initialize(ctx); err != nil {
		return fmt.Errorf("konnte OpenCV Personendetektor nicht initialisieren: %w", err)
	}

	s.initialized = true
	return nil
}

// DetectPersons prüft, ob ein Bild Personen enthält
// Gibt true zurück, wenn Personen gefunden wurden, sowie eine Liste der erkannten Personen
func (s *Service) DetectPersons(ctx context.Context, imagePath string) (bool, []DetectedPerson, error) {
	if !s.cfg.Enabled || !s.initialized {
		// Bei deaktiviertem OpenCV immer true zurückgeben, damit die weitere Verarbeitung stattfindet
		return true, nil, nil
	}

	// Versuche, den OpenCV-Service zu initialisieren, falls noch nicht geschehen
	if !s.initialized {
		if err := s.initialize(); err != nil {
			return false, nil, fmt.Errorf("konnte OpenCV nicht initialisieren: %w", err)
		}
	}

	persons, err := s.detector.DetectPersons(ctx, imagePath)
	if err != nil {
		log.Warnf("Fehler bei der OpenCV-Personenerkennung: %v", err)
		// Bei Fehler lieber true zurückgeben, damit die Verarbeitung nicht blockiert wird
		return true, nil, err
	}

	hasPersons := len(persons) > 0

	log.Debugf("OpenCV-Vorfilter: %s hat %d Personen", imagePath, len(persons))
	return hasPersons, persons, nil
}

// Close gibt die Ressourcen des OpenCV-Service frei
func (s *Service) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.initialized && s.detector != nil {
		if err := s.detector.Close(); err != nil {
			return err
		}
		s.initialized = false
	}
	return nil
}
