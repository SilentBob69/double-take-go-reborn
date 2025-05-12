package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"double-take-go-reborn/config"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

// Client ist der MQTT-Client für die Kommunikation mit Frigate und anderen Quellen
type Client struct {
	config      config.MQTTConfig
	client      mqtt.Client
	isConnected bool
	handlers    []MessageHandler
}

// MessageHandler ist ein Interface für Handler, die MQTT-Nachrichten verarbeiten
type MessageHandler interface {
	HandleMessage(topic string, payload []byte)
}

// FrigateEvent repräsentiert ein Ereignis von Frigate
type FrigateEvent struct {
	Before  *FrigateEventData `json:"before,omitempty"`
	After   *FrigateEventData `json:"after,omitempty"`
	Type    string            `json:"type"`
	Camera  string            `json:"camera"`
	ID      string            `json:"id"`
}

// FrigateEventData enthält die Details eines Frigate-Ereignisses
type FrigateEventData struct {
	ID                string         `json:"id"`
	Camera            string         `json:"camera"`
	Label             string         `json:"label"`
	SubLabel          string         `json:"sub_label,omitempty"`
	Score             float64        `json:"score"`
	TopScore          float64        `json:"top_score"`
	FalsePositive     bool           `json:"false_positive"`
	StartTime         float64        `json:"start_time"`           // Zeitstempel als Float
	EndTime           float64        `json:"end_time,omitempty"`   // Zeitstempel als Float
	Box               []int          `json:"box"`
	Area              int            `json:"area"`
	Ratio             float64        `json:"ratio"`
	Region            []int          `json:"region"`
	Active            bool           `json:"active"`
	Stationary        bool           `json:"stationary"`
	MotionlessCount   int            `json:"motionless_count"`
	CurrentZones      []string       `json:"current_zones"`
	EnteredZones      []string       `json:"entered_zones"`
	HasClip           bool           `json:"has_clip"`
	HasSnapshot       bool           `json:"has_snapshot"`
	CurrentAttributes []string       `json:"current_attributes"`
	FrameTime         float64        `json:"frame_time"`           // Zeitstempel als Float
	Snapshot          interface{}    `json:"snapshot"`             // Objekt mit Snapshot-Daten
	Thumbnail         interface{}    `json:"thumbnail"`            // Objekt mit Thumbnail-Daten
	PathData          []interface{}  `json:"path_data,omitempty"`
}

// NewClient erstellt einen neuen MQTT-Client
func NewClient(cfg config.MQTTConfig) *Client {
	return &Client{
		config:   cfg,
		handlers: make([]MessageHandler, 0),
	}
}

// RegisterHandler registriert einen neuen MessageHandler
func (c *Client) RegisterHandler(handler MessageHandler) {
	c.handlers = append(c.handlers, handler)
	log.Debug("Registered new MQTT message handler")
}

// Start startet den MQTT-Client und verbindet ihn mit dem Broker
func (c *Client) Start() error {
	if !c.config.Enabled {
		log.Info("MQTT client is disabled in configuration")
		return nil
	}

	// MQTT-Client-Optionen konfigurieren
	opts := mqtt.NewClientOptions()
	
	// Broker-URL erstellen - TCP statt ws oder wss verwenden für bessere Stabilität
	brokerURL := fmt.Sprintf("tcp://%s:%d", c.config.Broker, c.config.Port)
	opts.AddBroker(brokerURL)
	
	// Client-ID mit Zeitstempel für Einzigartigkeit
	clientID := c.config.ClientID
	if !strings.HasPrefix(clientID, "dt_") {
		clientID = "dt_" + clientID
	}
	// Timestamp anhängen, um bei Neustarts eine eindeutige Client-ID zu haben
	clientID = fmt.Sprintf("%s_%d", clientID, time.Now().Unix())
	opts.SetClientID(clientID)
	
	// Optionale Authentifizierung
	if c.config.Username != "" {
		opts.SetUsername(c.config.Username)
		opts.SetPassword(c.config.Password)
	}
	
	// Connection-Callbacks konfigurieren
	opts.SetOnConnectHandler(c.onConnectHandler)
	opts.SetConnectionLostHandler(c.connectionLostHandler)
	
	// Verbindungsstabilität und automatische Wiederverbindung
	opts.SetAutoReconnect(true)
	// Häufigere, schnellere Wiederverbindungsversuche
	opts.SetMaxReconnectInterval(3 * time.Second)  
	opts.SetConnectTimeout(30 * time.Second)      
	// Kürzeres Keep-Alive-Intervall für häufigere Lebenszeichen 
	opts.SetKeepAlive(15 * time.Second)           
	opts.SetPingTimeout(5 * time.Second)        
	opts.SetCleanSession(true)                 
	opts.SetOrderMatters(false)                
	
	// Mehr Server-Puffer-Einstellungen
	opts.SetWriteTimeout(5 * time.Second)
	
	// Will-Nachricht (Last Will and Testament) einrichten
	willTopic := fmt.Sprintf("double-take/status/%s", clientID)
	willPayload := []byte("{\"status\": \"offline\", \"timestamp\": \"" + time.Now().Format(time.RFC3339) + "\"}")
	opts.SetWill(willTopic, string(willPayload), 0, false) // QoS 0 und nicht retained für weniger Overhead
	
	// Client erstellen
	c.client = mqtt.NewClient(opts)
	
	// Verbindung herstellen und Wiederverbindungslogik im Fehlerfall
	log.Infof("Connecting to MQTT broker at %s", brokerURL)
	retry := 0
	maxRetries := 3
	connectBackoff := time.Second
	
	for retry < maxRetries {
		if token := c.client.Connect(); token.Wait() && token.Error() != nil {
			retry++
			log.Warnf("MQTT connect attempt %d/%d failed: %v", retry, maxRetries, token.Error())
			if retry < maxRetries {
				log.Infof("Retrying in %v...", connectBackoff)
				time.Sleep(connectBackoff)
				connectBackoff *= 2 // Exponentielles Backoff
			} else {
				log.Errorf("Failed to connect to MQTT broker after %d attempts", maxRetries)
				return token.Error()
			}
		} else {
			// Verbindung erfolgreich
			break
		}
	}
	
	log.Info("MQTT client connected successfully")
	
	// Starte einen Hintergrund-Goroutine, um die Verbindung zu überwachen
	go c.monitorConnection()
	
	return nil
}

// monitorConnection überwacht die MQTT-Verbindung und führt regelmäßige Health-Checks durch
func (c *Client) monitorConnection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if !c.client.IsConnected() {
				log.Warn("MQTT connection check: Client is disconnected, attempting reconnect")
				if token := c.client.Connect(); token.Wait() && token.Error() != nil {
					log.Errorf("Failed to reconnect to MQTT broker: %v", token.Error())
				} else {
					log.Info("MQTT client reconnected successfully")
				}
			} else {
				// Veröffentliche eine Heartbeat-Nachricht
				topic := "double-take/heartbeat"
				payload := fmt.Sprintf("{\"timestamp\":\"%s\",\"client_id\":\"%s\"}", 
					time.Now().Format(time.RFC3339), c.config.ClientID)
				
				if token := c.client.Publish(topic, 0, false, payload); token.Wait() && token.Error() != nil {
					log.Warnf("Failed to publish heartbeat: %v", token.Error())
				} else {
					log.Debug("MQTT heartbeat sent successfully")
				}
			}
		}
	}
}

// Stop beendet den MQTT-Client
func (c *Client) Stop() {
	if c.client != nil && c.client.IsConnected() {
		log.Info("Disconnecting MQTT client...")
		c.client.Disconnect(250) // 250ms Wartezeit
		c.isConnected = false
		log.Info("MQTT client disconnected")
	}
}

// IsConnected prüft, ob der Client verbunden ist
func (c *Client) IsConnected() bool {
	return c.client != nil && c.client.IsConnected()
}

// onConnectHandler wird aufgerufen, wenn die Verbindung hergestellt wurde
func (c *Client) onConnectHandler(client mqtt.Client) {
	log.Infof("Connected to MQTT broker at %s:%d", c.config.Broker, c.config.Port)
	c.isConnected = true
	
	// Thema abonnieren
	log.Infof("Subscribing to MQTT topic: %s", c.config.Topic)
	if token := client.Subscribe(c.config.Topic, 1, c.messageHandler); token.Wait() && token.Error() != nil {
		log.Errorf("Failed to subscribe to topic %s: %v", c.config.Topic, token.Error())
	} else {
		log.Infof("Successfully subscribed to topic: %s", c.config.Topic)
	}
}

// connectionLostHandler wird aufgerufen, wenn die Verbindung verloren geht
func (c *Client) connectionLostHandler(client mqtt.Client, err error) {
	c.isConnected = false
	
	// Erweiterte Fehlerdiagnose
	if err != nil {
		log.Errorf("MQTT connection lost: %v", err)
		
		// Spezifische Fehlertypen identifizieren und protokollieren
		if strings.Contains(err.Error(), "EOF") {
			log.Warn("MQTT-Verbindung unterbrochen (EOF) - Der Broker hat möglicherweise die Verbindung geschlossen")
			log.Info("Automatische Wiederverbindung wird versucht...")
		} else if strings.Contains(err.Error(), "connection refused") {
			log.Warn("MQTT-Verbindung verweigert - Der Broker ist möglicherweise nicht erreichbar oder blockiert die Verbindung")
		} else if strings.Contains(err.Error(), "timeout") {
			log.Warn("MQTT-Verbindungs-Timeout - Mögliche Netzwerkprobleme oder Broker überlastet")
		}
	} else {
		log.Error("MQTT-Verbindung unterbrochen ohne spezifischen Fehler")
	}
}

// messageHandler verarbeitet eingehende MQTT-Nachrichten
func (c *Client) messageHandler(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()
	
	log.Debugf("Received MQTT message on topic: %s", topic)
	
	// Wenn es sich um ein Frigate-Ereignis handelt, das Ereignis parsen
	if topic == c.config.Topic {
		var event FrigateEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Errorf("Failed to parse Frigate event: %v", err)
			return
		}
		
		// Log-Eintrag für das Ereignis erstellen
		logFields := log.Fields{
			"event_id": event.ID,
			"camera":   event.Camera,
			"type":     event.Type,
		}
		
		// Prüfe, ob After-Ereignisdaten verfügbar sind
		if event.After != nil {
			logFields["label"] = event.After.Label
		}
		
		log.WithFields(logFields).Info("Received Frigate event")
	}
	
	// Nachricht an alle Handler weiterleiten
	for _, handler := range c.handlers {
		go handler.HandleMessage(topic, payload)
	}
}

// PublishMessage veröffentlicht eine Nachricht an ein MQTT-Topic
func (c *Client) PublishMessage(topic string, payload interface{}, retain bool) error {
	if !c.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	var payloadBytes []byte
	var err error

	// Konvertiere die Payload in JSON, wenn es sich um ein Objekt handelt
	switch p := payload.(type) {
	case string:
		payloadBytes = []byte(p)
	case []byte:
		payloadBytes = p
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		payloadBytes = []byte(fmt.Sprintf("%v", p))
	default:
		// Versuche, das Objekt in JSON zu konvertieren
		payloadBytes, err = json.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal payload to JSON: %w", err)
		}
	}

	token := c.client.Publish(topic, 1, retain, payloadBytes)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, token.Error())
	}

	log.Debugf("Published message to topic: %s", topic)
	return nil
}

// PublishRetain veröffentlicht eine Nachricht mit dem Retain-Flag
func (c *Client) PublishRetain(topic string, payload interface{}) error {
	return c.PublishMessage(topic, payload, true)
}

// Publish veröffentlicht eine Nachricht ohne Retain-Flag
func (c *Client) Publish(topic string, payload interface{}) error {
	return c.PublishMessage(topic, payload, false)
}
