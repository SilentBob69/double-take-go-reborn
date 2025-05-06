package mqtt

import (
	"encoding/json"
	"fmt"
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
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	Score    float64    `json:"score"`
	TopScore float64    `json:"top_score"`
	Box      [4]int     `json:"box"`
	Area     int        `json:"area"`
	Ratio    float64    `json:"ratio"`
	Region   []int      `json:"region"`
	Current  bool       `json:"current"`
	Stationary bool     `json:"stationary"`
	Motionless int      `json:"motionless"`
	Thumbnail string    `json:"thumbnail"`
	Snapshot  string    `json:"snapshot"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Zones     []string  `json:"zones"`
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
	
	// Broker-URL erstellen
	brokerURL := fmt.Sprintf("tcp://%s:%d", c.config.Broker, c.config.Port)
	opts.AddBroker(brokerURL)
	
	// Client-ID
	opts.SetClientID(c.config.ClientID)
	
	// Optionale Authentifizierung
	if c.config.Username != "" {
		opts.SetUsername(c.config.Username)
		opts.SetPassword(c.config.Password)
	}
	
	// Connection-Callbacks konfigurieren
	opts.SetOnConnectHandler(c.onConnectHandler)
	opts.SetConnectionLostHandler(c.connectionLostHandler)
	
	// Automatische Wiederverbindung
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(1 * time.Minute)
	
	// Client erstellen
	c.client = mqtt.NewClient(opts)
	
	// Verbindung herstellen
	log.Infof("Connecting to MQTT broker at %s", brokerURL)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		log.Errorf("Failed to connect to MQTT broker: %v", token.Error())
		return token.Error()
	}
	
	log.Info("MQTT client connected successfully")
	return nil
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
	log.Errorf("MQTT connection lost: %v", err)
	c.isConnected = false
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
		log.WithFields(log.Fields{
			"event_id": event.ID,
			"camera":   event.Camera,
			"type":     event.Type,
			"label":    event.After.Label,
		}).Info("Received Frigate event")
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
