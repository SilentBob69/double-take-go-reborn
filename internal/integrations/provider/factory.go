package provider

import (
	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/integrations/compreface"
	"double-take-go-reborn/internal/integrations/facerecognition"
	"double-take-go-reborn/internal/integrations/insightface"

	log "github.com/sirupsen/logrus"
)

// Log-Felder für die Gesichtserkennungsanbieter

// CreateManager erstellt einen neuen ProviderManager für Gesichtserkennungsdienste
// basierend auf der Konfiguration
func CreateManager(cfg *config.Config) (*facerecognition.ProviderManager, error) {
	manager := facerecognition.NewProviderManager()
	
	// CompreFace-Provider registrieren, falls konfiguriert
	if cfg.CompreFace.Enabled {
		log.Info("Registriere CompreFace als Gesichtserkennungsanbieter")
		compreFaceService := compreface.NewService(cfg.CompreFace)
		manager.RegisterProvider(compreFaceService)
	}
	
	// InsightFace-Provider registrieren, falls konfiguriert
	if cfg.InsightFace.Enabled {
		log.Info("Registriere InsightFace als Gesichtserkennungsanbieter")
		insightFaceService := insightface.NewService(cfg.InsightFace)
		manager.RegisterProvider(insightFaceService)
	}
	
	// Den aktiven Provider basierend auf der Konfiguration festlegen
	activeProvider := cfg.FaceRecognitionProvider
	if activeProvider == "" {
		// Standardmäßig CompreFace verwenden, wenn verfügbar
		if cfg.CompreFace.Enabled {
			activeProvider = string(facerecognition.ProviderCompreFace)
		} else if cfg.InsightFace.Enabled {
			activeProvider = string(facerecognition.ProviderInsightFace)
		}
	}
	
	if activeProvider != "" {
		success := manager.SetActiveProvider(facerecognition.ProviderType(activeProvider))
		if !success {
			log.Warnf("Konnte den konfigurierten Provider '%s' nicht aktivieren, möglicherweise ist er nicht registriert", activeProvider)
		} else {
			log.Infof("Aktiver Gesichtserkennungsanbieter: %s", activeProvider)
		}
	} else {
		log.Warn("Kein Gesichtserkennungsanbieter aktiv")
	}
	
	return manager, nil
}
