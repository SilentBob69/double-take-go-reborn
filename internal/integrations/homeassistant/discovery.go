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
	ComponentCamera = "camera"         // Für Kamerabilder
	
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
	EntityCategory    string `json:"entity_category,omitempty"` // Kategorie der Entität (config, diagnostic)
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
	PictureEntityID   string `json:"entity_picture_template,omitempty"` // Template für das Bild
	AvailabilityTopic string `json:"availability_topic,omitempty"`
	PayloadAvailable  string `json:"payload_available,omitempty"`
	PayloadNotAvailable string `json:"payload_not_available,omitempty"`
	Device            *Device `json:"device,omitempty"`
	EntityCategory    string `json:"entity_category,omitempty"` // Kategorie der Entität (config, diagnostic)
}

// Device repräsentiert die Geräteinformationen für Home Assistant
type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	SWVersion    string   `json:"sw_version,omitempty"`
}

// CameraConfig repräsentiert die MQTT-Discovery-Konfiguration für eine Kamera in Home Assistant
type CameraConfig struct {
	Name              string `json:"name"`
	UniqueID          string `json:"unique_id"`
	Topic             string `json:"topic"`              // Topic für das Kamerabild
	ImageEncoding     string `json:"image_encoding,omitempty"` // z.B. "b64" für Base64-codierte Bilder
	AvailabilityTopic string `json:"availability_topic,omitempty"`
	PayloadAvailable  string `json:"payload_available,omitempty"`
	PayloadNotAvailable string `json:"payload_not_available,omitempty"`
	Device            *Device `json:"device,omitempty"`
}

// DiscoveryManager verwaltet die Home Assistant MQTT Discovery
type DiscoveryManager struct {
	mqttClient  *mqtt.Client
	cfg         *config.Config
	registeredEntities map[string]bool // Speichert bereits registrierte Entitäten um Duplikate zu vermeiden
}

// NewDiscoveryManager erstellt einen neuen Manager für Home Assistant Discovery
func NewDiscoveryManager(mqttClient *mqtt.Client, cfg *config.Config) *DiscoveryManager {
	return &DiscoveryManager{
		mqttClient: mqttClient,
		cfg:        cfg,
		registeredEntities: make(map[string]bool),
	}
}

// RegisterIdentities veröffentlicht Discovery-Konfigurationen für Home Assistant
func (dm *DiscoveryManager) RegisterIdentities(identities []models.Identity) error {
	if dm.mqttClient == nil || !dm.mqttClient.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	// Version aus Konfiguration oder Standard
	version := "1.0.0"
	if dm.cfg != nil && dm.cfg.Version != "" {
		version = dm.cfg.Version
	}

	// Gerät-Konfiguration erstellen nach Home Assistant Standard
	device := &Device{
		// HomeAssistant empfiehlt einen eindeutigen Identifier
		Identifiers:  []string{"double_take_go"},
		Name:         "Double Take Go",
		Manufacturer: "Double Take Community",
		Model:        "Go Edition",
		SWVersion:    version,
	}

	// Einfache Sensoren registrieren
	if err := dm.registerSimpleSensors(device); err != nil {
		log.Errorf("Failed to register recognition sensors: %v", err)
		return err
	}

	log.Infof("Successfully registered Home Assistant entities")
	return nil
}

// registerSimpleSensors erstellt genau zwei Entitäten für Home Assistant:
// 1. Einen Sensor für die erkannte Person
// 2. Eine Kamera-Entity für das Bild
func (dm *DiscoveryManager) registerSimpleSensors(device *Device) error {
	// 1. Haupt-Erkennungssensor als normaler Sensor (nicht binary)
	sensorConfig := SensorConfig{
		Name:              "Erkannte Person",
		UniqueID:          "double_take_recognized_person",
		StateTopic:        "double-take/person",
		Icon:              "mdi:face-recognition",
		JSONAttributesTopic: "double-take/person/attributes",
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic für den Sensor
	sensorTopic := fmt.Sprintf("%s/%s/%s/person/config", 
		DiscoveryPrefix, 
		ComponentSensor, 
		NodeID)

	// Konfiguration für Sensor senden
	log.Info("Registering Home Assistant person sensor entity")
	if err := dm.mqttClient.PublishRetain(sensorTopic, sensorConfig); err != nil {
		return fmt.Errorf("failed to publish person sensor configuration: %w", err)
	}
	
	// Initialisiere den Sensor mit Standardwert "unbekannt"
	if err := dm.mqttClient.PublishRetain("double-take/person", "unbekannt"); err != nil {
		log.Warnf("Failed to set initial value for person sensor: %v", err)
		// Kein Abbruch, wenn nur der Initialwert nicht gesetzt werden kann
	}

	// 2. Kamera-Element für die Erkennung
	cameraConfig := CameraConfig{
		Name:              "Erkennungsbild",
		UniqueID:          "double_take_detection_image",
		Topic:             "double-take/person/image",
		ImageEncoding:     "b64",  // Angabe, dass wir Base64-codierte Bilder senden
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic für Kamera
	cameraTopic := fmt.Sprintf("%s/%s/%s/detection_image/config", 
		DiscoveryPrefix, 
		ComponentCamera, 
		NodeID)

	// Konfiguration für Kamera senden
	log.Info("Registering Home Assistant detection camera entity")
	if err := dm.mqttClient.PublishRetain(cameraTopic, cameraConfig); err != nil {
		return fmt.Errorf("failed to publish camera configuration: %w", err)
	}

	return nil
}

// registerIdentitySensor erstellt eine Discovery-Konfiguration für eine einzelne Identität
func (dm *DiscoveryManager) registerIdentitySensor(identity models.Identity, device *Device) error {
	// Normalisiere den Namen für Topic-Verwendung (nur Kleinbuchstaben, keine Sonderzeichen, Leerzeichen durch Unterstrich ersetzen)
	normalizedName := strings.ToLower(identity.Name)
	normalizedName = strings.ReplaceAll(normalizedName, " ", "_")
	
	// Prüfen, ob die Entität bereits registriert wurde
	entityKey := fmt.Sprintf("identity_%s", normalizedName)
	if _, exists := dm.registeredEntities[entityKey]; exists {
		log.Debugf("Identity %s is already registered, skipping", identity.Name)
		return nil
	}
	
	// 1. Anwesenheits-Sensor (binary_sensor) registrieren
	binarySensorConfig := BinarySensorConfig{
		Name:              fmt.Sprintf("%s Anwesenheit", identity.Name),
		UniqueID:          fmt.Sprintf("double_take_%s_presence", normalizedName),
		StateTopic:        fmt.Sprintf("double-take/presence/%s", normalizedName),
		JSONAttributesTopic: fmt.Sprintf("double-take/presence/%s/attributes", normalizedName),
		Icon:              "mdi:account",
		PayloadOn:         "on",
		PayloadOff:        "off",
		ExpireAfter:       PresenceTimeout, // Automatisch ausschalten nach X Sekunden
		PictureEntityID:   "{{ value_json.image_data }}", // Bildvorlage für erkannte Gesichter
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
		EntityCategory:    "diagnostic", // Als diagnostische Entität markieren
	}

	// Discovery-Topic für Anwesenheits-Sensor
	binarySensorTopic := fmt.Sprintf("%s/%s/%s/%s_presence/config", 
		DiscoveryPrefix, 
		ComponentBinarySensor, 
		NodeID,
		normalizedName)

	// Konfiguration für Anwesenheits-Sensor senden
	log.Debugf("Registering Home Assistant presence sensor for identity: %s", identity.Name)
	if err := dm.mqttClient.PublishRetain(binarySensorTopic, binarySensorConfig); err != nil {
		return fmt.Errorf("failed to publish presence sensor configuration: %w", err)
	}
	
	// 2. Informations-Sensor (letzter Standort etc.) registrieren - als versteckte Entität
	sensorConfig := SensorConfig{
		Name:              fmt.Sprintf("%s Info", identity.Name),
		UniqueID:          fmt.Sprintf("double_take_%s_info", normalizedName),
		StateTopic:        fmt.Sprintf("double-take/presence/%s/location", normalizedName),
		Icon:              "mdi:information-outline",
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
		EntityCategory:    "diagnostic", // Als diagnostische Entität markieren
	}

	// Discovery-Topic für Info-Sensor
	infoSensorTopic := fmt.Sprintf("%s/%s/%s/%s_info/config", 
		DiscoveryPrefix, 
		ComponentSensor, 
		NodeID,
		normalizedName)

	// Konfiguration für Info-Sensor senden
	log.Debugf("Registering Home Assistant info sensor for identity: %s", identity.Name)
	if err := dm.mqttClient.PublishRetain(infoSensorTopic, sensorConfig); err != nil {
		return fmt.Errorf("failed to publish info sensor configuration: %w", err)
	}

	// 3. Kamera-Element für das Erkennungsbild registrieren - als versteckte Entität
	cameraConfig := CameraConfig{
		Name:              fmt.Sprintf("%s Bild", identity.Name),
		UniqueID:          fmt.Sprintf("double_take_%s_image", normalizedName),
		Topic:             fmt.Sprintf("double-take/presence/%s/image", normalizedName),
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic für Kamera
	cameraTopic := fmt.Sprintf("%s/%s/%s/%s_image/config", 
		DiscoveryPrefix, 
		ComponentCamera, 
		NodeID,
		normalizedName)

	// Konfiguration für Kamera senden
	log.Debugf("Registering Home Assistant camera for identity: %s", identity.Name)
	if err := dm.mqttClient.PublishRetain(cameraTopic, cameraConfig); err != nil {
		return fmt.Errorf("failed to publish camera configuration: %w", err)
	}

	// Markieren, dass diese Entität registriert wurde
	dm.registeredEntities[entityKey] = true
	return nil
}

// registerUnknownSensor erstellt eine Discovery-Konfiguration für unbekannte Gesichter
func (dm *DiscoveryManager) registerUnknownSensor(device *Device) error {
	// Prüfen, ob die Entität bereits registriert wurde
	entityKey := "identity_unknown"
	if _, exists := dm.registeredEntities[entityKey]; exists {
		log.Debug("Unknown identity sensor is already registered, skipping")
		return nil
	}

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
		PictureEntityID:   "{{ value_json.image_data }}", // Bildvorlage für unbekannte Gesichter
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
		EntityCategory:    "diagnostic", // Als diagnostische Entität markieren
	}

	// Discovery-Topic für Anwesenheits-Sensor
	binarySensorTopic := fmt.Sprintf("%s/%s/%s/%s_presence/config", 
		DiscoveryPrefix, 
		ComponentBinarySensor, 
		NodeID,
		"unknown")

	// Konfiguration für Anwesenheits-Sensor senden
	log.Debug("Registering Home Assistant presence sensor for unknown faces")
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
		EntityCategory:    "diagnostic", // Als diagnostische Entität markieren
	}

	// Discovery-Topic für Info-Sensor
	infoSensorTopic := fmt.Sprintf("%s/%s/%s/%s_info/config", 
		DiscoveryPrefix, 
		ComponentSensor, 
		NodeID,
		"unknown")

	// Konfiguration für Info-Sensor senden
	log.Debug("Registering Home Assistant info sensor for unknown faces")
	if err := dm.mqttClient.PublishRetain(infoSensorTopic, sensorConfig); err != nil {
		return fmt.Errorf("failed to publish info sensor configuration: %w", err)
	}

	// 3. Kamera-Element für das Erkennungsbild registrieren
	cameraConfig := CameraConfig{
		Name:              "Unbekannte Person Bild",
		UniqueID:          "double_take_unknown_image",
		Topic:             "double-take/presence/unknown/image",
		AvailabilityTopic: "double-take/status",
		PayloadAvailable:  "online",
		PayloadNotAvailable: "offline",
		Device:            device,
	}

	// Discovery-Topic für Kamera
	cameraTopic := fmt.Sprintf("%s/%s/%s/%s_image/config", 
		DiscoveryPrefix, 
		ComponentCamera, 
		NodeID,
		"unknown")

	// Konfiguration für Kamera senden
	log.Debug("Registering Home Assistant camera for unknown faces")
	if err := dm.mqttClient.PublishRetain(cameraTopic, cameraConfig); err != nil {
		return fmt.Errorf("failed to publish camera configuration: %w", err)
	}

	// Markieren, dass diese Entität registriert wurde
	dm.registeredEntities[entityKey] = true
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
