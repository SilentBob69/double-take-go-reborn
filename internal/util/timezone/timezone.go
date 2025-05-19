package timezone

import (
	"time"
	"os"

	log "github.com/sirupsen/logrus"
)

// Globale Variable für die aktuelle Zeitzone
var currentLocation *time.Location

// Initialize setzt die Zeitzone basierend auf der TZ-Umgebungsvariable
// Diese Funktion sollte beim Programmstart aufgerufen werden
func Initialize() {
	// Standard ist UTC
	tzName := "UTC"
	
	// TZ-Umgebungsvariable auslesen
	envTZ := os.Getenv("TZ")
	if envTZ != "" {
		tzName = envTZ
	}
	
	// Lokation laden
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		log.Warnf("Failed to load timezone %s from environment: %v. Falling back to UTC.", tzName, err)
		currentLocation = time.UTC
		return
	}
	
	log.Infof("Successfully initialized timezone to %s", tzName)
	currentLocation = loc
}

// Now gibt die aktuelle Zeit in der konfigurierten Zeitzone zurück
func Now() time.Time {
	if currentLocation == nil {
		// Wenn die Zeitzone noch nicht initialisiert wurde, initialisiere sie jetzt
		Initialize()
	}
	return time.Now().In(currentLocation)
}

// Format formatiert ein time.Time-Objekt mit der konfigurierten Zeitzone
func Format(t time.Time, layout string) string {
	if currentLocation == nil {
		Initialize()
	}
	return t.In(currentLocation).Format(layout)
}

// ISO8601 formatiert ein time.Time-Objekt im ISO 8601-Format mit der konfigurierten Zeitzone
func ISO8601(t time.Time) string {
	return Format(t, time.RFC3339)
}

// RFC3339 ist ein Alias für ISO8601, formatiert die Zeit im RFC3339-Format
func RFC3339(t time.Time) string {
	return ISO8601(t)
}

// GetTimeInConfiguredZone ist ein Alias für Now() für Kompatibilität mit existierendem Code
func GetTimeInConfiguredZone() time.Time {
	return Now()
}
