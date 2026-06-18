package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	mqttclient "github.com/eclipse/paho.mqtt.golang"
	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

type AlertClient struct {
	client   mqttclient.Client
	topicPrefix string
	connected bool
}

type AlertPayload struct {
	BearingID    int       `json:"bearing_id"`
	BearingCode  string    `json:"bearing_code,omitempty"`
	AlertType    string    `json:"alert_type"`
	AlertLevel   string    `json:"alert_level"`
	AlertMessage string    `json:"alert_message"`
	Threshold    *float64  `json:"threshold,omitempty"`
	ActualValue  *float64  `json:"actual_value,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

func NewAlertClient() *AlertClient {
	return &AlertClient{
		topicPrefix: config.AppConfig.MQTT.TopicPrefix,
		connected:   false,
	}
}

func (a *AlertClient) Connect() error {
	opts := mqttclient.NewClientOptions()
	opts.AddBroker(config.AppConfig.MQTT.Broker)
	opts.SetClientID(config.AppConfig.MQTT.ClientID)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	if config.AppConfig.MQTT.Username != "" {
		opts.SetUsername(config.AppConfig.MQTT.Username)
		opts.SetPassword(config.AppConfig.MQTT.Password)
	}

	opts.OnConnect = func(c mqttclient.Client) {
		log.Println("MQTT 告警客户端已连接")
		a.connected = true
	}

	opts.OnConnectionLost = func(c mqttclient.Client, err error) {
		log.Printf("MQTT 连接丢失: %v", err)
		a.connected = false
	}

	a.client = mqttclient.NewClient(opts)

	token := a.client.Connect()
	if token.WaitTimeout(10 * time.Second) {
		if token.Error() != nil {
			return fmt.Errorf("MQTT连接失败: %w", token.Error())
		}
	}

	a.connected = true
	return nil
}

func (a *AlertClient) Disconnect() {
	if a.client != nil {
		a.client.Disconnect(250)
	}
	a.connected = false
}

func (a *AlertClient) PublishAlert(bearing *models.Bearing, alert *models.AlertEvent) (string, error) {
	if !a.connected {
		log.Println("MQTT未连接，告警未推送")
		return "", nil
	}

	topic := fmt.Sprintf("%s/%d/%s", a.topicPrefix, bearing.ID, alert.AlertType)

	payload := AlertPayload{
		BearingID:    bearing.ID,
		BearingCode:  bearing.BearingCode,
		AlertType:    alert.AlertType,
		AlertLevel:   alert.AlertLevel,
		AlertMessage: alert.AlertMessage,
		Threshold:    alert.ThresholdValue,
		ActualValue:  alert.ActualValue,
		Timestamp:    alert.AlertTime,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化告警失败: %w", err)
	}

	token := a.client.Publish(topic, 1, false, data)
	if token.WaitTimeout(5 * time.Second) {
		if token.Error() != nil {
			return topic, fmt.Errorf("MQTT发布失败: %w", token.Error())
		}
	}

	log.Printf("告警已推送至MQTT: topic=%s, level=%s, msg=%s", topic, alert.AlertLevel, alert.AlertMessage)
	return topic, nil
}

type AlertManager struct {
	mqttClient *AlertClient
	alertMap   map[string]time.Time
}

func NewAlertManager() *AlertManager {
	return &AlertManager{
		mqttClient: NewAlertClient(),
		alertMap:   make(map[string]time.Time),
	}
}

func (am *AlertManager) Start() error {
	return am.mqttClient.Connect()
}

func (am *AlertManager) Stop() {
	am.mqttClient.Disconnect()
}

func (am *AlertManager) ShouldAlert(bearingID int, alertType string) bool {
	key := fmt.Sprintf("%d:%s", bearingID, alertType)
	lastTime, exists := am.alertMap[key]
	if !exists {
		return true
	}

	cooldown := time.Duration(config.AppConfig.Alert.CooldownMinutes) * time.Minute
	return time.Since(lastTime) >= cooldown
}

func (am *AlertManager) MarkAlerted(bearingID int, alertType string) {
	key := fmt.Sprintf("%d:%s", bearingID, alertType)
	am.alertMap[key] = time.Now()
}

func (am *AlertManager) PublishAlert(bearing *models.Bearing, alert *models.AlertEvent) (string, error) {
	return am.mqttClient.PublishAlert(bearing, alert)
}
