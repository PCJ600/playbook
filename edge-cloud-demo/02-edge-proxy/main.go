package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strings"
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
)

var (
	kafkaWriter *kafka.Writer
	msgChan     = make(chan mqtt.Message, 1000)
)

func init() {
	// 初始化异步日志
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

func initKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:        kafka.TCP("kafka:9092"),
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
    // log.Printf("已投递到Kafka [device:%s]", deviceID)
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

func main() {
	// 初始化
	kafkaWriter = initKafkaWriter()
	defer kafkaWriter.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startWorkers(ctx)

	// MQTT客户端配置
	opts := mqtt.NewClientOptions().
		AddBroker("tcp://emqx:1883").
		SetClientID(mqttClientID).
		SetCleanSession(false)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal().Err(token.Error()).Msg("MQTT连接失败")
	}

	// 订阅
	token := client.Subscribe("device/+/telemetry", 1, func(_ mqtt.Client, msg mqtt.Message) {
		msgChan <- msg
	})
	if token.Wait() && token.Error() != nil {
		log.Fatal().Err(token.Error()).Msg("订阅失败")
	}

	// 优雅退出
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	client.Disconnect(250)
	log.Info().Msg("服务正常退出")
}
