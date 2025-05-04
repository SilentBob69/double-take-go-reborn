package mqtt

import (
	"fmt"
	"time"

	"github.com/eclipse/paho.mqtt.golang" // Official Eclipse Paho Go client
	"double-take-go-reborn/internal/config"
	"double-take-go-reborn/internal/processing"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"double-take-go-reborn/internal/sse" // Assuming sse package is located here
)

var ( // Use vars for functions to allow mocking in tests later if needed
	NewClientFunc = mqtt.NewClient
)

// Client wraps the MQTT client and its configuration.
type Client struct {
	Cfg        config.MQTTConfig
	Client     mqtt.Client
	Processor  *processing.Processor // Reference to the core processing logic
	IsConnected bool
}

// IsActuallyConnected checks the status of the underlying Paho client.
func (c *Client) IsActuallyConnected() bool {
	return c.Client != nil && c.Client.IsConnected()
}

// NewMQTTClient creates and configures a new MQTT client wrapper.
func NewMQTTClient(cfg config.MQTTConfig, db *gorm.DB, appCfg *config.Config, sseHub *sse.Hub) (*Client, error) {
	if !cfg.Enabled {
		log.Info("MQTT client is disabled in the configuration.")
		return nil, nil // Not an error, just not enabled
	}

	processor := processing.NewProcessor(appCfg, db, sseHub) // Create the processor

	mqttClient := &Client{
		Cfg:       cfg,
		Processor: processor,
	}

	// Construct the full broker URL
	brokerURL := fmt.Sprintf("tcp://%s:%d", cfg.Broker, cfg.Port)

	opts := mqtt.NewClientOptions()
	// Use the full URL
	opts.AddBroker(brokerURL)
	opts.SetClientID(cfg.ClientID)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}

	// Configure connection callbacks
	opts.SetConnectionLostHandler(mqttClient.connectionLostHandler)
	opts.SetOnConnectHandler(mqttClient.onConnectHandler)
	// Automatically reconnect
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(1 * time.Minute)

	// Create the client instance
	client := NewClientFunc(opts)
	mqttClient.Client = client

	return mqttClient, nil
}

// Start connects to the MQTT broker and starts listening.
func (c *Client) Start() error {
	if c.Client == nil {
		return fmt.Errorf("MQTT client not initialized (likely disabled)")
	}
	// Construct the full broker URL for logging
	brokerURL := fmt.Sprintf("tcp://%s:%d", c.Cfg.Broker, c.Cfg.Port)
	log.Infof("Attempting to connect to MQTT broker: %s", brokerURL)
	if token := c.Client.Connect(); token.Wait() && token.Error() != nil {
		log.Errorf("Failed to connect to MQTT broker %s: %v", brokerURL, token.Error())
		// Don't return error here, rely on auto-reconnect
		return token.Error() // Return error to potentially stop app start if initial connect fails?
	}
	// Connection might happen asynchronously via onConnectHandler
	return nil
}

// Stop disconnects the MQTT client.
func (c *Client) Stop() {
	if c.Client != nil && c.Client.IsConnected() {
		log.Info("Disconnecting MQTT client...")
		c.Client.Disconnect(250) // Wait 250ms for disconnection
		log.Info("MQTT client disconnected.")
	}
	c.IsConnected = false
}

// connectionLostHandler logs when the connection is lost.
func (c *Client) connectionLostHandler(client mqtt.Client, err error) {
	log.Errorf("MQTT connection lost: %v. Attempting to reconnect...", err)
	c.IsConnected = false
}

// onConnectHandler subscribes to the configured topic when connected.
func (c *Client) onConnectHandler(client mqtt.Client) {
	// Construct the full broker URL for logging
	brokerURL := fmt.Sprintf("tcp://%s:%d", c.Cfg.Broker, c.Cfg.Port)
	log.Infof("Successfully connected to MQTT broker: %s", brokerURL)
	c.IsConnected = true

	// Subscribe to the topic
	log.Infof("Subscribing to MQTT topic: %s", c.Cfg.Topic)
	if token := client.Subscribe(c.Cfg.Topic, 1, c.messageHandler); token.Wait() && token.Error() != nil {
		log.Errorf("Failed to subscribe to topic %s: %v", c.Cfg.Topic, token.Error())
	} else {
		log.Infof("Successfully subscribed to MQTT topic: %s", c.Cfg.Topic)
	}
}

// messageHandler is called when a new message arrives on the subscribed topic.
func (c *Client) messageHandler(client mqtt.Client, msg mqtt.Message) {
	log.Debugf("Received raw message on topic: %s", msg.Topic()) // Log message reception
	log.Debugf("Received MQTT message on topic '%s'", msg.Topic())
	// Pass the message payload to the processor
	go c.Processor.ProcessFrigateEvent(msg.Payload()) // Process in a goroutine to not block the MQTT client
}
