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
	
	// Component-Typen
	ComponentSensor = "sensor"
	ComponentBinarySensor = "binary_sensor" // Für Anwesenheitssensoren
	
	// Geräteklassen für Binary Sensors
	DeviceClassPresence = "presence"
	
	// Node-ID für Double-Take
	NodeID = "double_take"
	
	// Timeout für Anwesenheit in Sekunden (5 Minuten)
	PresenceTimeout = 300
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

// BinarySensorConfig repräsentiert die MQTT-Discovery-Konfiguration für einen binären Sensor in Home Assistant
type BinarySensorConfig struct {
	Name              string `json:"name"`
	UniqueID          string `json:"unique_id"`
	StateTopic        string `json:"state_topic"`
	DeviceClass       string `json:"device_class,omitempty"`
	Icon              string `json:"icon,omitempty"`
	JSONAttributesTopic string `json:"json_attributes_topic,omitempty"`
	PayloadOn         string `json:"payload_on,omitempty"`
	PayloadOff        string `json:"payload_off,omitempty"`
	ExpireAfter       int    `json:"expire_after,omitempty"`  // Timeout in Sekunden
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
	
	// 1. Anwesenheits-Sensor (binary_sensor) registrieren
	binarySensorConfig := BinarySensorConfig{
		Name:              fmt.Sprintf("%s Anwesenheit", identity.Name),
		UniqueID:          fmt.Sprintf("double_take_%s_presence", normalizedName),
		StateTopic:        fmt.Sprintf("double-take/presence/%s", normalizedName),
		JSONAttributesTopic: fmt.Sprintf("double-take/presence/%s/attributes", normalizedName),
		DeviceClass:       DeviceClassPresence,
		Icon:              "mdi:account-check",
		PayloadOn:         "on",
		PayloadOff:        "off",
		ExpireAfter:       PresenceTimeout, // Automatisch ausschalten nach X Sekunden
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic für Anwesenheits-Sensor
	binarySensorTopic := fmt.Sprintf("%s/%s/%s/%s_presence/config", 
		DiscoveryPrefix, 
		ComponentBinarySensor, 
		NodeID,
		normalizedName)

	// Konfiguration für Anwesenheits-Sensor senden
	log.Infof("Registering Home Assistant presence sensor for identity: %s", identity.Name)
	if err := dm.mqttClient.PublishRetain(binarySensorTopic, binarySensorConfig); err != nil {
		return fmt.Errorf("failed to publish presence sensor configuration: %w", err)
	}
	
	// 2. Informations-Sensor (letzter Standort etc.) registrieren
	sensorConfig := SensorConfig{
		Name:              fmt.Sprintf("%s Info", identity.Name),
		UniqueID:          fmt.Sprintf("double_take_%s_info", normalizedName),
		StateTopic:        fmt.Sprintf("double-take/presence/%s/location", normalizedName),
		Icon:              "mdi:information-outline",
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic für Info-Sensor
	infoSensorTopic := fmt.Sprintf("%s/%s/%s/%s_info/config", 
		DiscoveryPrefix, 
		ComponentSensor, 
		NodeID,
		normalizedName)

	// Konfiguration für Info-Sensor senden
	log.Infof("Registering Home Assistant info sensor for identity: %s", identity.Name)
	if err := dm.mqttClient.PublishRetain(infoSensorTopic, sensorConfig); err != nil {
		return fmt.Errorf("failed to publish info sensor configuration: %w", err)
	}

	return nil
}

// registerUnknownSensor erstellt eine Discovery-Konfiguration für unbekannte Gesichter
func (dm *DiscoveryManager) registerUnknownSensor(device *Device) error {
	// 1. Anwesenheits-Sensor (binary_sensor) für unbekannte Gesichter registrieren
	binarySensorConfig := BinarySensorConfig{
		Name:              "Unbekannte Person Anwesenheit",
		UniqueID:          "double_take_unknown_presence",
		StateTopic:        "double-take/presence/unknown",
		JSONAttributesTopic: "double-take/presence/unknown/attributes",
		DeviceClass:       DeviceClassPresence,
		Icon:              "mdi:account-question",
		PayloadOn:         "on",
		PayloadOff:        "off",
		ExpireAfter:       PresenceTimeout, // Automatisch ausschalten nach X Sekunden
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic für Anwesenheits-Sensor
	binarySensorTopic := fmt.Sprintf("%s/%s/%s/%s_presence/config", 
		DiscoveryPrefix, 
		ComponentBinarySensor, 
		NodeID,
		"unknown")

	// Konfiguration für Anwesenheits-Sensor senden
	log.Info("Registering Home Assistant presence sensor for unknown faces")
	if err := dm.mqttClient.PublishRetain(binarySensorTopic, binarySensorConfig); err != nil {
		return fmt.Errorf("failed to publish presence sensor configuration: %w", err)
	}
	
	// 2. Informations-Sensor (letzter Standort etc.) für unbekannte Gesichter registrieren
	sensorConfig := SensorConfig{
		Name:              "Unbekannte Person Info",
		UniqueID:          "double_take_unknown_info",
		StateTopic:        "double-take/presence/unknown/location",
		Icon:              "mdi:information-outline",
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic für Info-Sensor
	infoSensorTopic := fmt.Sprintf("%s/%s/%s/%s_info/config", 
		DiscoveryPrefix, 
		ComponentSensor, 
		NodeID,
		"unknown")

	// Konfiguration für Info-Sensor senden
	log.Info("Registering Home Assistant info sensor for unknown faces")
	if err := dm.mqttClient.PublishRetain(infoSensorTopic, sensorConfig); err != nil {
		return fmt.Errorf("failed to publish info sensor configuration: %w", err)
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
