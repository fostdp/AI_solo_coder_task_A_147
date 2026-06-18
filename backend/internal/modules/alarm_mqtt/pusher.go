package alarm_mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/messages"
)

const (
	DefaultQoS = 1
)

type AlertPayload struct {
	BearingID   int        `json:"bearing_id"`
	BearingCode string     `json:"bearing_code,omitempty"`
	AlertType   string     `json:"alert_type"`
	AlertLevel  string     `json:"alert_level"`
	Message     string     `json:"message"`
	Threshold   *float64   `json:"threshold,omitempty"`
	ActualValue *float64   `json:"actual_value,omitempty"`
	Timestamp   time.Time  `json:"timestamp"`
}

type AlertClient struct {
	client   mqtt.Client
	config   *config.MQTTConfig
	connected bool
	mu       sync.RWMutex
}

type AlertManager struct {
	alertChan   <-chan messages.AlertMessage
	alertClient *AlertClient
	alertMap    map[string]time.Time
	mu          sync.Mutex
	cooldown    time.Duration
	running     bool
}

func NewAlertClient(cfg *config.MQTTConfig) *AlertClient {
	return &AlertClient{
		config:    cfg,
		connected: false,
	}
}

func (ac *AlertClient) Connect(ctx context.Context) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	opts := mqtt.NewClientOptions()
	opts.AddBroker(ac.config.Broker)
	opts.SetClientID(ac.config.ClientID)
	if ac.config.Username != "" {
		opts.SetUsername(ac.config.Username)
		opts.SetPassword(ac.config.Password)
	}
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetMaxReconnectInterval(1 * time.Minute)

	opts.OnConnect = func(client mqtt.Client) {
		ac.mu.Lock()
		ac.connected = true
		ac.mu.Unlock()
		log.Printf("MQTT告警客户端已连接到 %s", ac.config.Broker)
	}

	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		ac.mu.Lock()
		ac.connected = false
		ac.mu.Unlock()
		log.Printf("MQTT告警客户端连接断开: %v", err)
	}

	ac.client = mqtt.NewClient(opts)

	token := ac.client.Connect()
	go func() {
		<-ctx.Done()
		if ac.client != nil && ac.client.IsConnected() {
			ac.client.Disconnect(250)
			log.Println("MQTT告警客户端已断开连接")
		}
	}()

	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT连接失败: %w", token.Error())
	}

	return nil
}

func (ac *AlertClient) PublishAlert(bearing *models.Bearing, msg *messages.AlertMessage) error {
	ac.mu.RLock()
	if !ac.connected {
		ac.mu.RUnlock()
		return fmt.Errorf("MQTT未连接")
	}
	ac.mu.RUnlock()

	payload := AlertPayload{
		BearingID:   msg.Bearing.ID,
		BearingCode: msg.Bearing.BearingCode,
		AlertType:   msg.AlertType,
		AlertLevel:  msg.AlertLevel,
		Message:     msg.AlertMessage,
		Threshold:   msg.Threshold,
		ActualValue: msg.ActualValue,
		Timestamp:   msg.Timestamp,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化告警消息失败: %w", err)
	}

	topic := fmt.Sprintf("%s/%d/%s", ac.config.TopicPrefix, msg.Bearing.ID, msg.AlertType)

	token := ac.client.Publish(topic, DefaultQoS, false, payloadBytes)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("发布告警消息失败: %w", token.Error())
	}

	log.Printf("告警已发布到MQTT主题 %s: %s", topic, msg.AlertMessage)
	return nil
}

func (ac *AlertClient) IsConnected() bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.connected
}

func (ac *AlertClient) Disconnect() {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.client != nil && ac.client.IsConnected() {
		ac.client.Disconnect(250)
		ac.connected = false
	}
}

func NewAlertManager(
	alertChan <-chan messages.AlertMessage,
	alertClient *AlertClient,
	cooldownMinutes int,
) *AlertManager {
	return &AlertManager{
		alertChan:   alertChan,
		alertClient: alertClient,
		alertMap:    make(map[string]time.Time),
		cooldown:    time.Duration(cooldownMinutes) * time.Minute,
		running:     false,
	}
}

func (am *AlertManager) Start(ctx context.Context) {
	am.mu.Lock()
	if am.running {
		am.mu.Unlock()
		return
	}
	am.running = true
	am.mu.Unlock()

	log.Println("告警管理器已启动")

	go func() {
		for {
			select {
			case <-ctx.Done():
				am.Stop()
				return
			case msg, ok := <-am.alertChan:
				if !ok {
					return
				}
				am.ProcessAlert(&msg)
			}
		}
	}()
}

func (am *AlertManager) Stop() {
	am.mu.Lock()
	defer am.mu.Unlock()
	if !am.running {
		return
	}
	am.running = false
	am.alertClient.Disconnect()
	log.Println("告警管理器已停止")
}

func (am *AlertManager) ShouldAlert(bearingID int, alertType string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := fmt.Sprintf("%d:%s", bearingID, alertType)
	lastAlert, exists := am.alertMap[key]

	if !exists {
		am.alertMap[key] = time.Now()
		return true
	}

	if time.Since(lastAlert) >= am.cooldown {
		am.alertMap[key] = time.Now()
		return true
	}

	return false
}

func (am *AlertManager) MarkAlerted(bearingID int, alertType string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := fmt.Sprintf("%d:%s", bearingID, alertType)
	am.alertMap[key] = time.Now()
}

func (am *AlertManager) ProcessAlert(msg *messages.AlertMessage) {
	if !am.ShouldAlert(msg.Bearing.ID, msg.AlertType) {
		log.Printf("告警冷却中，跳过推送 (轴承 %d, 类型 %s)", msg.Bearing.ID, msg.AlertType)
		return
	}

	if err := am.alertClient.PublishAlert(msg.Bearing, msg); err != nil {
		log.Printf("推送告警失败: %v", err)
		return
	}

	am.saveToDatabase(msg)
	am.MarkAlerted(msg.Bearing.ID, msg.AlertType)
}

func (am *AlertManager) saveToDatabase(msg *messages.AlertMessage) {
	if database.Instance == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	alert := &models.AlertEvent{
		BearingID:      msg.Bearing.ID,
		AlertTime:      msg.Timestamp,
		AlertType:      msg.AlertType,
		AlertLevel:     msg.AlertLevel,
		AlertMessage:   msg.AlertMessage,
		ThresholdValue: msg.Threshold,
		ActualValue:    msg.ActualValue,
		Acknowledged:   false,
	}

	if err := database.Instance.InsertAlertEvent(ctx, alert); err != nil {
		log.Printf("保存告警事件失败: %v", err)
	}
}
