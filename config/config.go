package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	log "github.com/sirupsen/logrus"
)

// Config repräsentiert die Hauptkonfiguration der Anwendung
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Log        LogConfig        `mapstructure:"log"`
	DB         DBConfig         `mapstructure:"db"`
	CompreFace CompreFaceConfig `mapstructure:"compreface"`
	OpenCV     OpenCVConfig     `mapstructure:"opencv"`
	MQTT       MQTTConfig       `mapstructure:"mqtt"`
	Frigate    FrigateConfig    `mapstructure:"frigate"`
	Cleanup    CleanupConfig    `mapstructure:"cleanup"`
	Notifications NotificationsConfig `mapstructure:"notifications"`
}

// ServerConfig enthält Server-bezogene Einstellungen
type ServerConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	DataDir     string `mapstructure:"data_dir"`
	SnapshotDir string `mapstructure:"snapshot_dir"`
	SnapshotURL string `mapstructure:"snapshot_url"`
	TemplateDir string `mapstructure:"template_dir"`
	Timezone    string `mapstructure:"timezone"`
}

// LogConfig enthält Log-Einstellungen
type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

// DBConfig enthält Datenbankeinstellungen
type DBConfig struct {
	File     string `mapstructure:"file"`     // für SQLite
	Username string `mapstructure:"username"` // für PostgreSQL
	Password string `mapstructure:"password"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
}

// CompreFaceConfig enthält CompreFace-Einstellungen
type CompreFaceConfig struct {
	Enabled            bool    `mapstructure:"enabled"`
	URL                string  `mapstructure:"url"`
	RecognitionAPIKey  string  `mapstructure:"recognition_api_key"`
	DetectionAPIKey    string  `mapstructure:"detection_api_key"`
	DetProbThreshold   float64 `mapstructure:"det_prob_threshold"`
	SimilarityThreshold float64 `mapstructure:"similarity_threshold"`
	EnableDetection    bool    `mapstructure:"enable_detection"`
	EnableRecognition  bool    `mapstructure:"enable_recognition"`
	SyncIntervalMinutes int     `mapstructure:"sync_interval_minutes"`
	ServiceID          string  `mapstructure:"service_id"`
}

// MQTTConfig enthält die Konfiguration für den MQTT-Client
type MQTTConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Broker    string `mapstructure:"broker"`
	Port      int    `mapstructure:"port"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	ClientID  string `mapstructure:"client_id"`
	Topic     string `mapstructure:"topic"`
	HomeAssistant HomeAssistantConfig `mapstructure:"homeassistant"`
}

// HomeAssistantConfig enthält die Konfiguration für die Home Assistant Integration
type HomeAssistantConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	DiscoveryPrefix string `mapstructure:"discovery_prefix"`
	PublishResults bool   `mapstructure:"publish_results"`
}

// FrigateConfig enthält Frigate-Einstellungen
type FrigateConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	Host             string `mapstructure:"host"`
	EventTopic       string `mapstructure:"event_topic"`
	ProcessPersonOnly bool   `mapstructure:"process_person_only"`
	APIURL           string `mapstructure:"api_url"` // Legacy-Feld
	URL              string `mapstructure:"url"`     // Legacy-Feld
}

// CleanupConfig enthält Bereinigungseinstellungen
type CleanupConfig struct {
	RetentionDays int `mapstructure:"retention_days"`
}

// NotificationsConfig enthält Benachrichtigungseinstellungen
type NotificationsConfig struct {
	NewFaces  bool `mapstructure:"new_faces"`
	KnownFaces bool `mapstructure:"known_faces"`
}

// PersonDetectionConfig enthält Konfigurationsoptionen für die OpenCV-Personenerkennung
type PersonDetectionConfig struct {
	Method              string  `mapstructure:"method"`               // Detektionsmethode: "hog" oder "dnn"
	Model               string  `mapstructure:"model"`                // Für DNN: "ssd_mobilenet" oder "yolov4"
	ConfidenceThreshold float64 `mapstructure:"confidence_threshold"` // Schwellenwert für die Erkennungskonfidenz
	ScaleFactor         float64 `mapstructure:"scale_factor"`         // Skalierungsfaktor für Multi-Scale-Detektion
	MinNeighbors        int     `mapstructure:"min_neighbors"`        // Minimum benachbarter Erkennungen für Bestätigung
	MinSizeWidth        int     `mapstructure:"min_size_width"`       // Minimale Breite einer Person in Pixeln
	MinSizeHeight       int     `mapstructure:"min_size_height"`      // Minimale Höhe einer Person in Pixeln
	Backend             string  `mapstructure:"backend"`              // DNN-Backend: "default", "cuda", "opencl"
	Target              string  `mapstructure:"target"`               // DNN-Target: "cpu", "cuda", "opencl"
	ModelPath           string  `mapstructure:"model_path"`           // Pfad zur DNN-Modelldatei
	ConfigPath          string  `mapstructure:"config_path"`          // Pfad zur DNN-Konfigurationsdatei
}

// OpenCVConfig enthält Einstellungen für die OpenCV-Integration
type OpenCVConfig struct {
	Enabled         bool                 `mapstructure:"enabled"`
	UseGPU          bool                 `mapstructure:"use_gpu"`
	// Bisherige Parameter für Abwärtskompatibilität
	DetProbThreshold float64             `mapstructure:"det_prob_threshold"` // Schwellenwert für die Gesichtserkennung
	ScaleFactor     float64             `mapstructure:"scale_factor"`       // Skalierungsfaktor für Bildverkleinerung
	MinNeighbors    int                 `mapstructure:"min_neighbors"`     // Minimum benachbarter Erkennungen für Bestätigung
	MinSizeWidth    int                 `mapstructure:"min_size_width"`    // Minimale Breite eines Gesichts in Pixeln
	MinSizeHeight   int                 `mapstructure:"min_size_height"`   // Minimale Höhe eines Gesichts in Pixeln
	// Neue verschachtelte Konfiguration für erweiterte Personenerkennung
	PersonDetection PersonDetectionConfig `mapstructure:"person_detection"`
}

// Load lädt die Konfiguration aus Datei, Umgebungsvariablen und Standardwerten
func Load(configPath string) (*Config, error) {
	v := viper.New()
	
	// Standardwerte festlegen
	setDefaults(v)
	
	// Konfigurationsdatei laden, wenn vorhanden
	if configPath != "" {
		// Überprüfen, ob die Datei existiert
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			log.Warnf("Config file %s does not exist, using defaults", configPath)
		} else {
			v.SetConfigFile(configPath)
			if err := v.ReadInConfig(); err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
			log.Infof("Config loaded from %s", configPath)
		}
	}
	
	// Umgebungsvariablen überlagern die Konfiguration
	v.AutomaticEnv()
	v.SetEnvPrefix("DOUBLE_TAKE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	
	// Konfiguration in Struct umwandeln
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Sicherstellen, dass erforderliche Verzeichnisse existieren
	if err := ensureDirectories(&cfg); err != nil {
		return nil, fmt.Errorf("failed to create required directories: %w", err)
	}
	
	return &cfg, nil
}

// setDefaults legt Standardwerte für die Konfiguration fest
func setDefaults(v *viper.Viper) {
	// Server-Standardwerte
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 3000)
	v.SetDefault("server.data_dir", "/data")
	v.SetDefault("server.snapshot_dir", "/data/snapshots")
	v.SetDefault("server.snapshot_url", "/snapshots")
	v.SetDefault("server.template_dir", "/app/web/templates")
	v.SetDefault("server.timezone", "UTC")
	
	// Log-Standardwerte
	v.SetDefault("log.level", "info")
	v.SetDefault("log.file", "/data/logs/double-take.log")
	
	// DB-Standardwerte
	v.SetDefault("db.file", "/data/double-take.db")
	
	// CompreFace-Standardwerte
	v.SetDefault("compreface.enabled", false)
	v.SetDefault("compreface.det_prob_threshold", 0.8)
	v.SetDefault("compreface.similarity_threshold", 80.0)
	v.SetDefault("compreface.enable_detection", true)
	v.SetDefault("compreface.enable_recognition", true)
	v.SetDefault("compreface.sync_interval_minutes", 15)
	v.SetDefault("compreface.service_id", "")
	
	// OpenCV-Standardwerte
	v.SetDefault("opencv.enabled", true)
	v.SetDefault("opencv.use_gpu", false)
	// Legacy-Parameter für Abwärtskompatibilität
	v.SetDefault("opencv.det_prob_threshold", 0.7)
	v.SetDefault("opencv.scale_factor", 1.1)
	v.SetDefault("opencv.min_neighbors", 3)
	v.SetDefault("opencv.min_size_width", 60)
	v.SetDefault("opencv.min_size_height", 60)
	
	// Person Detection Standardwerte
	v.SetDefault("opencv.person_detection.method", "hog")          // HOG ist der Standard-Algorithmus
	v.SetDefault("opencv.person_detection.model", "ssd_mobilenet") // Standard für DNN
	v.SetDefault("opencv.person_detection.confidence_threshold", 0.5)
	v.SetDefault("opencv.person_detection.scale_factor", 1.05)
	v.SetDefault("opencv.person_detection.min_neighbors", 2)
	v.SetDefault("opencv.person_detection.min_size_width", 64)   // Typische Minimalbreite für Personen
	v.SetDefault("opencv.person_detection.min_size_height", 128) // Typisches Seitenverhältnis für Personen
	v.SetDefault("opencv.person_detection.backend", "default")
	v.SetDefault("opencv.person_detection.target", "cpu")
	
	// MQTT-Standardwerte
	v.SetDefault("mqtt.enabled", false)
	v.SetDefault("mqtt.port", 1883)
	v.SetDefault("mqtt.client_id", "double-take-go")
	v.SetDefault("mqtt.topic", "frigate/events")
	v.SetDefault("mqtt.homeassistant.enabled", false)
	v.SetDefault("mqtt.homeassistant.discovery_prefix", "homeassistant")
	v.SetDefault("mqtt.homeassistant.publish_results", true)
	
	// Frigate-Standardwerte
	v.SetDefault("frigate.enabled", false)
	v.SetDefault("frigate.event_topic", "frigate/events")
	v.SetDefault("frigate.process_person_only", true)
	
	// Cleanup-Standardwerte
	v.SetDefault("cleanup.retention_days", 30)
	
	// Benachrichtigungs-Standardwerte
	v.SetDefault("notifications.new_faces", true)
	v.SetDefault("notifications.known_faces", true)
}

// ensureDirectories stellt sicher, dass alle erforderlichen Verzeichnisse existieren
func ensureDirectories(cfg *Config) error {
	// Daten-Basisverzeichnis
	if cfg.Server.DataDir != "" {
		if err := os.MkdirAll(cfg.Server.DataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}
	}
	
	// Snapshot-Verzeichnis
	if err := os.MkdirAll(cfg.Server.SnapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}
	
	// Log-Verzeichnis
	logDir := filepath.Dir(cfg.Log.File)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	
	// Datenbank-Verzeichnis (für SQLite)
	if cfg.DB.File != "" {
		dbDir := filepath.Dir(cfg.DB.File)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory: %w", err)
		}
	}
	
	return nil
}
