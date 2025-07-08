package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
)

const (
	mqttClientID   = "mqtt-proxy"
	kafkaTopicName = "device.telemetry"
	workerCount    = 1
	reconnectDelay = 5 * time.Second // 重连延迟时间
)

var (
	kafkaWriter *kafka.Writer
	msgChan     = make(chan mqtt.Message, 1000)
	mqttClient  mqtt.Client
	mqttMutex   sync.Mutex
)

func init() {
	// 初始化异步日志
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

func initKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:       kafka.TCP("kafka:9092"),
		Topic:      kafkaTopicName,
		Balancer:   &kafka.Hash{},
		BatchSize:  100,
		BatchBytes: 1e6,
		Async:      true,
	}
}

func processMessage(msg mqtt.Message) {
	var data map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &data); err != nil {
		log.Error().Str("topic", msg.Topic()).Err(err).Msg("JSON解析失败")
		return
	}

	topicParts := strings.Split(msg.Topic(), "/")
	if len(topicParts) < 2 {
		return
	}

	deviceID := topicParts[1]
	data["processed_by"] = mqttClientID

	modifiedMsg, err := json.Marshal(data)
	if err != nil {
		log.Error().Str("device", deviceID).Err(err).Msg("JSON序列化失败")
		return
	}

	if err := kafkaWriter.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte(deviceID),
			Value: modifiedMsg,
		},
	); err != nil {
		log.Error().Str("device", deviceID).Err(err).Msg("Kafka写入失败")
		return
	}
	log.Info().Str("device", deviceID).Msg("已投递到Kafka")
}

func startWorkers(ctx context.Context) {
	for i := 0; i < workerCount; i++ {
		go func() {
			for {
				select {
				case msg := <-msgChan:
					processMessage(msg)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

// 创建MQTT客户端并设置回调
func createMqttClient() mqtt.Client {
	opts := mqtt.NewClientOptions().
		AddBroker("tcp://emqx:1883").
		SetClientID(mqttClientID).
		SetCleanSession(false).
		SetAutoReconnect(true). // 启用自动重连
		SetOnConnectHandler(func(c mqtt.Client) {
			log.Info().Msg("MQTT连接成功")
			// 连接成功后重新订阅
			if token := c.Subscribe("device/+/telemetry", 1, func(_ mqtt.Client, msg mqtt.Message) {
				msgChan <- msg
			}); token.Wait() && token.Error() != nil {
				log.Error().Err(token.Error()).Msg("订阅失败")
			} else {
				log.Info().Msg("订阅主题成功: device/+/telemetry")
			}
		}).
		SetConnectionLostHandler(func(c mqtt.Client, err error) {
			log.Error().Err(err).Msg("MQTT连接丢失")
		}).
		SetReconnectingHandler(func(c mqtt.Client, opts *mqtt.ClientOptions) {
			log.Info().Msg("尝试重新连接MQTT服务器...")
		})

	client := mqtt.NewClient(opts)
	return client
}

// 确保MQTT连接
func ensureMqttConnection() {
	mqttMutex.Lock()
	defer mqttMutex.Unlock()

	if mqttClient == nil || !mqttClient.IsConnected() {
		if mqttClient != nil {
			mqttClient.Disconnect(250)
		}
		mqttClient = createMqttClient()
		
		for {
			if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
				log.Error().Err(token.Error()).Msg("MQTT连接失败，重试中...")
				time.Sleep(reconnectDelay)
				continue
			}
			break
		}
	}
}

func main() {
	// 初始化
	kafkaWriter = initKafkaWriter()
	defer kafkaWriter.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startWorkers(ctx)

	// 初始化MQTT客户端并确保连接
	ensureMqttConnection()

	// 启动一个goroutine定期检查连接状态
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if !mqttClient.IsConnected() {
					log.Warn().Msg("检测到MQTT连接断开，尝试重新连接...")
					ensureMqttConnection()
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 优雅退出
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	
	log.Info().Msg("开始关闭服务...")
	
	// 关闭MQTT连接
	if mqttClient != nil && mqttClient.IsConnected() {
		mqttClient.Disconnect(250)
	}
	
	log.Info().Msg("服务正常退出")
}
