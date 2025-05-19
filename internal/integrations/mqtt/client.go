package mqtt

import (
	"double-take-go-reborn/internal/util/timezone"
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
	
	// Broker-URL erstellen - Hier wird das tcp:// protokoll hinzugefügt
	// So kann in der config.yaml einfach die IP-Adresse ohne Protokoll verwendet werden
	brokerURL := fmt.Sprintf("tcp://%s:%d", c.config.Broker, c.config.Port)
	
	log.Debugf("Verbinde mit MQTT-Broker: %s", brokerURL)
	opts.AddBroker(brokerURL)
	
	// Client-ID mit Zeitstempel für Einzigartigkeit
	clientID := c.config.ClientID
	if clientID == "" {
		clientID = "double_take" 
	} else if !strings.HasPrefix(clientID, "dt_") {
		clientID = "dt_" + clientID
	}
	// Timestamp anhängen, um bei Neustarts eine eindeutige Client-ID zu haben
	clientID = fmt.Sprintf("%s_%d", clientID, timezone.Now().Unix())
	opts.SetClientID(clientID)
	
	// Optionale Authentifizierung
	if c.config.Username != "" {
		opts.SetUsername(c.config.Username)
		opts.SetPassword(c.config.Password)
	}
	
	// Last Will Testament für Home Assistant einrichten
	// Das ermöglicht, dass Home Assistant automatisch erkennt, wenn die Verbindung abbricht
	topicPrefix := c.config.TopicPrefix
	if topicPrefix == "" {
		topicPrefix = "double-take"
	}
	statusTopic := fmt.Sprintf("%s/status", topicPrefix)
	opts.SetWill(statusTopic, "offline", 1, true) // QoS 1, retained
	
	// Connection-Callbacks konfigurieren
	opts.SetOnConnectHandler(c.onConnectHandler)
	opts.SetConnectionLostHandler(c.connectionLostHandler)
	
	// Verbindungsstabilität und automatische Wiederverbindung
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(30 * time.Second) // Maximal 30 Sekunden zwischen Verbindungsversuchen
	opts.SetKeepAlive(60 * time.Second)            // 60 Sekunden Keep-Alive
	opts.SetPingTimeout(10 * time.Second)          // 10 Sekunden für Ping-Timeout
	opts.SetConnectTimeout(30 * time.Second)       // 30 Sekunden für Verbindungsaufbau
	opts.SetCleanSession(false)                    // Session beibehalten für zuverlässige Subscriptions
	
	// Mehr Server-Puffer-Einstellungen
	opts.SetWriteTimeout(5 * time.Second)
	
	// Will-Nachricht (Last Will and Testament) einrichten
	willTopic := fmt.Sprintf("double-take/status/%s", clientID)
	willPayload := []byte("{\"status\": \"offline\", \"timestamp\": \"" + timezone.Now().Format(time.RFC3339) + "\"}")
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
	ticker := time.NewTicker(30 * time.Second) // Alle 30 Sekunden überprüfen
	defer ticker.Stop()
	
	// Topic für Status-Updates
	topicPrefix := c.config.TopicPrefix
	if topicPrefix == "" {
		topicPrefix = "double-take"
	}
	statusTopic := fmt.Sprintf("%s/status", topicPrefix)
	
	for range ticker.C {
		if c.client != nil && c.config.Enabled {
			if c.client.IsConnected() {
				// Online-Status veröffentlichen
				token := c.client.Publish(statusTopic, 1, true, "online")
				if token.Wait() && token.Error() != nil {
					log.Warnf("Failed to publish online status: %v", token.Error())
				}
				c.isConnected = true
			} else {
				log.Warn("MQTT connection lost, trying to reconnect...")
				c.isConnected = false
				payload := fmt.Sprintf("{\"timestamp\":\"%s\",\"client_id\":\"%s\"}", 
					timezone.Now().Format(time.RFC3339), c.config.ClientID)
				
				heartbeatTopic := fmt.Sprintf("%s/heartbeat", c.config.TopicPrefix)
				if token := c.client.Publish(heartbeatTopic, 0, false, payload); token.Wait() && token.Error() != nil {
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
	
	// Online-Status sofort veröffentlichen für Home Assistant
	topicPrefix := c.config.TopicPrefix
	if topicPrefix == "" {
		topicPrefix = "double-take"
	}
	statusTopic := fmt.Sprintf("%s/status", topicPrefix)
	
	if token := client.Publish(statusTopic, 1, true, "online"); token.Wait() && token.Error() != nil {
		log.Errorf("Failed to publish online status: %v", token.Error())
	} else {
		log.Info("Published online status to Home Assistant")
	}
	
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

// GetRetainedPayload ruft eine retained Nachricht von einem MQTT-Topic ab
func (c *Client) GetRetainedPayload(topic string) (string, error) {
	if !c.IsConnected() {
		return "", fmt.Errorf("MQTT client is not connected")
	}
	
	// Channel für die Antwort erstellen
	respChan := make(chan string, 1)
	
	// Temporären Handler für das angegebene Topic registrieren
	token := c.client.Subscribe(topic, 1, func(_ mqtt.Client, msg mqtt.Message) {
		// Nachricht in den Channel schreiben und Subscription beenden
		respChan <- string(msg.Payload())
		c.client.Unsubscribe(topic)
	})
	
	if token.Wait() && token.Error() != nil {
		return "", fmt.Errorf("failed to subscribe to topic %s: %w", topic, token.Error())
	}
	
	// Timeout für die Antwort setzen (2 Sekunden)
	select {
	case payload := <-respChan:
		return payload, nil
	case <-time.After(2 * time.Second):
		// Subscription beenden, falls sie noch aktiv ist
		c.client.Unsubscribe(topic)
		return "", nil // Keine retained Nachricht gefunden ist kein Fehler
	}
}
