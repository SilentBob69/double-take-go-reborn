package config

import (
	"log"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config holds the application configuration.
// Tags correspond to the keys in the YAML file.
type Config struct {
	Server     ServerConfig     `koanf:"server"`
	Log        LogConfig        `koanf:"log"`
	DB         DBConfig         `koanf:"db"`
	CompreFace CompreFaceConfig `koanf:"compreface"`
	MQTT       MQTTConfig       `koanf:"mqtt"`
	Frigate    FrigateConfig    `koanf:"frigate"`
	Cleanup    CleanupConfig    `koanf:"cleanup"` // Added Cleanup config
	// Add other sections like 'ui', 'detectors', 'storage', etc. later
}

// ServerConfig holds server-related settings.
type ServerConfig struct {
	Host        string `koanf:"host"`
	Port        int    `koanf:"port"`
	SnapshotDir string `koanf:"snapshot_dir"` // Directory where snapshots are stored
	SnapshotURL string `koanf:"snapshot_url"` // URL path to serve snapshots from
	TemplateDir string `koanf:"template_dir"` // Directory for HTML templates
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string `koanf:"level"`
	File  string `koanf:"file"` // Path to the log file inside the container
}

// DBConfig holds database settings.
type DBConfig struct {
	Username string `koanf:"username"`
	Password string `koanf:"password"`
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Name     string `koanf:"name"`
	File     string `koanf:"file"` // Path to the SQLite database file inside the container
}

// CompreFaceConfig holds settings for the CompreFace integration.
type CompreFaceConfig struct {
	Enabled           bool          `koanf:"enabled" default:"false"`
	Url               string        `koanf:"url"`
	RecognitionApiKey string        `koanf:"recognition_api_key"`
	DetectionApiKey   string        `koanf:"detection_api_key"`   // Optional: API key for the detection service (if different/used)
	DetProbThreshold  float64       `koanf:"det_prob_threshold" default:"0.8"`
	SyncIntervalMinutes int           `koanf:"sync_interval_minutes" default:"15"` // New: Interval for syncing subjects
}

// MQTTConfig holds settings for the MQTT client connection.
type MQTTConfig struct {
	Enabled   bool   `koanf:"enabled"`   // Whether MQTT connection is active
	Broker    string `koanf:"broker"`  // MQTT broker address (e.g., 192.168.1.100) - Tag corrected!
	Port      int    `koanf:"port"`      // MQTT broker port (e.g., 1883)
	Username  string `koanf:"username"`  // Optional: MQTT username
	Password  string `koanf:"password"`  // Optional: MQTT password
	ClientID  string `koanf:"client_id"` // Client ID to use for connection
	Topic     string `koanf:"topic"`     // Topic to subscribe to (e.g., frigate/events)
}

// FrigateConfig holds settings related to the Frigate instance.
type FrigateConfig struct {
	ApiUrl string `koanf:"api_url"` // Base URL for the Frigate API (e.g., http://frigate:5000)
	Url    string `koanf:"url"`     // Base URL for the Frigate API (e.g., http://<ip>:<port>)
	// Add other Frigate specific settings if needed
}

// CleanupConfig holds settings for automatic data cleanup.
type CleanupConfig struct {
	RetentionDays int `koanf:"retention_days"` // Number of days to keep images and data
}

// Global Koanf instance
var k = koanf.New(".")

// Load reads configuration from file and environment variables.
// It applies defaults selectively after attempting to load from file.
func Load(configPath string) (*Config, error) {
	// Default values - used only if not provided in config file
	defaultMqttBroker := "localhost"
	defaultMqttPort := 1883
	defaultMqttClientID := "double-take-go"
	defaultMqttTopic := "frigate/events"
	defaultLogLevel := "info"
	defaultLogFile := "/data/server.log"
	defaultDbFile := "/data/double-take.db"
	defaultServerHost := "0.0.0.0"
	defaultServerPort := 3000
	defaultSnapshotDir := "/data/snapshots"
	defaultSnapshotURL := "/snapshots"
	defaultTemplateDir := "./web/templates"
	defaultDetProbThreshold := 0.85
	defaultCleanupRetentionDays := 5
	defaultSyncIntervalMinutes := 15

	// Load YAML config file
	log.Printf("Loading configuration from %s...\n", configPath)
	if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
		log.Printf("Warning: Failed to load configuration file '%s': %v\n", configPath, err)
		// Continue even if file loading fails, env vars or defaults might apply
	}

	// TODO: Add environment variable provider
	// TODO: Add command line flag provider

	// Unmarshal the configuration into the Config struct
	// Values loaded from the file (or other providers) will be placed here.
	var cfg Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		log.Printf("Warning: Failed to unmarshal config structure: %v\n", err)
		// Continue, defaults will be applied below for zero-value fields
	}

	// --- Apply defaults selectively ONLY if fields are still zero-valued --- 
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaultServerHost
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = defaultServerPort
	}
	if cfg.Server.SnapshotDir == "" {
		cfg.Server.SnapshotDir = defaultSnapshotDir
	}
	if cfg.Server.SnapshotURL == "" {
		cfg.Server.SnapshotURL = defaultSnapshotURL
	}
	if cfg.Server.TemplateDir == "" {
		cfg.Server.TemplateDir = defaultTemplateDir
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = defaultLogLevel
	}
	if cfg.Log.File == "" {
		cfg.Log.File = defaultLogFile
	}
	if cfg.DB.File == "" {
		cfg.DB.File = defaultDbFile
	}
	// Apply MQTT defaults only if missing
	if cfg.MQTT.Broker == "" { // Check uses the Broker field, which now gets populated correctly
		cfg.MQTT.Broker = defaultMqttBroker
	}
	if cfg.MQTT.Port == 0 {
		cfg.MQTT.Port = defaultMqttPort
	}
	if cfg.MQTT.ClientID == "" {
		cfg.MQTT.ClientID = defaultMqttClientID
	}
	if cfg.MQTT.Topic == "" {
		cfg.MQTT.Topic = defaultMqttTopic
	}
	// Note: Username and Password defaults are empty strings, so no explicit check needed

	// CompreFace Defaults
	if cfg.CompreFace.DetProbThreshold == 0.0 { // float64 defaults to 0.0
		cfg.CompreFace.DetProbThreshold = defaultDetProbThreshold // Set a sensible default
	}
	if cfg.CompreFace.SyncIntervalMinutes == 0 {
		cfg.CompreFace.SyncIntervalMinutes = defaultSyncIntervalMinutes
	}

	// Apply Cleanup defaults only if missing
	if cfg.Cleanup.RetentionDays == 0 {
		cfg.Cleanup.RetentionDays = defaultCleanupRetentionDays
	}

	// Ensure log level is lowercase
	cfg.Log.Level = strings.ToLower(cfg.Log.Level)

	log.Println("Configuration loaded successfully.")
	// Debug print loaded MQTT config
	log.Printf("DEBUG: Loaded config value: MQTT.Enabled = %v", cfg.MQTT.Enabled)
	log.Printf("DEBUG: Loaded config value: MQTT.Broker = %s", cfg.MQTT.Broker)
	log.Printf("DEBUG: Loaded config value: MQTT.Port = %d", cfg.MQTT.Port)
	log.Printf("DEBUG: Loaded config value: MQTT.Username = %s", cfg.MQTT.Username)
	// Avoid logging password directly: log.Printf("DEBUG: Loaded config value: MQTT.Password = %s", cfg.MQTT.Password)
	log.Printf("DEBUG: Loaded config value: MQTT.Password set: %v", cfg.MQTT.Password != "")
	log.Printf("DEBUG: Loaded config value: MQTT.ClientID = %s", cfg.MQTT.ClientID)
	log.Printf("DEBUG: Loaded config value: MQTT.Topic = %s", cfg.MQTT.Topic)
	log.Printf("DEBUG: Loaded config value: Cleanup.RetentionDays = %d", cfg.Cleanup.RetentionDays)
	log.Printf("DEBUG: Loaded config value: Server.SnapshotDir = %s", cfg.Server.SnapshotDir)
	log.Printf("DEBUG: Loaded config value: Server.SnapshotURL = %s", cfg.Server.SnapshotURL)
	log.Printf("DEBUG: Loaded config value: Server.TemplateDir = %s", cfg.Server.TemplateDir)
	log.Printf("DEBUG: Loaded config value: CompreFace.SyncIntervalMinutes = %d", cfg.CompreFace.SyncIntervalMinutes)

	return &cfg, nil
}
