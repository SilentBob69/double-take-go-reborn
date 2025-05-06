package homeassistant

import (
	"fmt"
	"strings"

	"double-take-go-reborn/config"
	"double-take-go-reborn/internal/core/models"
	"double-take-go-reborn/internal/integrations/mqtt"

	log "github.com/sirupsen/logrus"
)

// Constants for Home Assistant MQTT Discovery
const (
	// Discovery-Präfix für Home Assistant (Standard ist "homeassistant")
	DiscoveryPrefix = "homeassistant"
	
	// Component-Typ für Sensoren
	ComponentSensor = "sensor"
	
	// Node-ID für Double-Take
	NodeID = "double_take"
)

// SensorConfig repräsentiert die MQTT-Discovery-Konfiguration für einen Sensor in Home Assistant
type SensorConfig struct {
	Name              string `json:"name"`
	UniqueID          string `json:"unique_id"`
	StateTopic        string `json:"state_topic"`
	Icon              string `json:"icon,omitempty"`
	JSONAttributesTopic string `json:"json_attributes_topic,omitempty"`
	ValueTemplate     string `json:"value_template,omitempty"`
	AvailabilityTopic string `json:"availability_topic,omitempty"`
	PayloadAvailable  string `json:"payload_available,omitempty"`
	PayloadNotAvailable string `json:"payload_not_available,omitempty"`
	Device            *Device `json:"device,omitempty"`
}

// Device repräsentiert die Geräteinformationen für Home Assistant
type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	SWVersion    string   `json:"sw_version,omitempty"`
}

// DiscoveryManager verwaltet die Home Assistant MQTT Discovery
type DiscoveryManager struct {
	mqttClient  *mqtt.Client
	cfg         *config.Config
}

// NewDiscoveryManager erstellt einen neuen Manager für Home Assistant Discovery
func NewDiscoveryManager(mqttClient *mqtt.Client, cfg *config.Config) *DiscoveryManager {
	return &DiscoveryManager{
		mqttClient: mqttClient,
		cfg:        cfg,
	}
}

// RegisterIdentities veröffentlicht Discovery-Konfigurationen für alle Identitäten
func (dm *DiscoveryManager) RegisterIdentities(identities []models.Identity) error {
	// Erstelle den Device-Eintrag für Double-Take
	device := &Device{
		Identifiers:  []string{"double_take_go"},
		Name:         "Double Take Go",
		Manufacturer: "Double Take Go Project",
		Model:        "Go Edition",
		SWVersion:    "1.0.0", // TODO: Versionsverwaltung implementieren
	}

	// Für jede Identität einen Sensor registrieren
	for _, identity := range identities {
		if err := dm.registerIdentitySensor(identity, device); err != nil {
			log.Errorf("Failed to register sensor for identity %s: %v", identity.Name, err)
		}
	}

	// Auch einen Sensor für unbekannte Gesichter registrieren
	if err := dm.registerUnknownSensor(device); err != nil {
		log.Errorf("Failed to register sensor for unknown faces: %v", err)
	}

	return nil
}

// registerIdentitySensor erstellt eine Discovery-Konfiguration für eine einzelne Identität
func (dm *DiscoveryManager) registerIdentitySensor(identity models.Identity, device *Device) error {
	// Normalisiere den Namen für Verwendung in Topics (Kleinbuchstaben, Leerzeichen durch Unterstriche)
	normalizedName := strings.ToLower(strings.ReplaceAll(identity.Name, " ", "_"))
	
	// Sensor-Konfiguration erstellen
	sensorConfig := SensorConfig{
		Name:              fmt.Sprintf("Double Take %s", identity.Name),
		UniqueID:          fmt.Sprintf("double_take_%s", normalizedName),
		StateTopic:        fmt.Sprintf("double-take/cameras/+"), // Der State ist die Kamera
		JSONAttributesTopic: fmt.Sprintf("double-take/matches/%s", normalizedName),
		ValueTemplate:     fmt.Sprintf("{{ value_json.camera }}"), // Die Kamera als State verwenden
		Icon:              "mdi:face-recognition",
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic
	topic := fmt.Sprintf("%s/%s/%s/%s/config", 
		DiscoveryPrefix, 
		ComponentSensor, 
		NodeID,
		normalizedName)

	// Konfiguration an MQTT senden
	log.Infof("Registering Home Assistant sensor for identity: %s", identity.Name)
	if err := dm.mqttClient.PublishRetain(topic, sensorConfig); err != nil {
		return fmt.Errorf("failed to publish discovery configuration: %w", err)
	}

	return nil
}

// registerUnknownSensor erstellt eine Discovery-Konfiguration für unbekannte Gesichter
func (dm *DiscoveryManager) registerUnknownSensor(device *Device) error {
	// Sensor-Konfiguration erstellen
	sensorConfig := SensorConfig{
		Name:              "Double Take Unknown",
		UniqueID:          "double_take_unknown",
		StateTopic:        "double-take/cameras/+", // Der State ist die Kamera
		JSONAttributesTopic: "double-take/unknown",
		ValueTemplate:     "{{ value_json.camera }}", // Die Kamera als State verwenden
		Icon:              "mdi:face-recognition",
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic
	topic := fmt.Sprintf("%s/%s/%s/%s/config", 
		DiscoveryPrefix, 
		ComponentSensor, 
		NodeID,
		"unknown")

	// Konfiguration an MQTT senden
	log.Info("Registering Home Assistant sensor for unknown faces")
	if err := dm.mqttClient.PublishRetain(topic, sensorConfig); err != nil {
		return fmt.Errorf("failed to publish discovery configuration: %w", err)
	}

	return nil
}

// PublishAvailability veröffentlicht den Online-Status von Double-Take
func (dm *DiscoveryManager) PublishAvailability(online bool) error {
	status := "offline"
	if online {
		status = "online"
	}
	
	// Status an MQTT senden
	return dm.mqttClient.PublishRetain("double-take/status", status)
}
