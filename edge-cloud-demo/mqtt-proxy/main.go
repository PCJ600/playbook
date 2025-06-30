package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
    "time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/segmentio/kafka-go"
)

const (
	mqttClientID   = "mqtt-proxy"
	kafkaTopicName = "device.telemetry" // Kafka topic常量化
)

// KafkaWriter 全局writer，由main函数初始化
var kafkaWriter *kafka.Writer

// initKafkaWriter 初始化Kafka Writer
func initKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:     kafka.TCP("kafka:9092"),
		Topic:    kafkaTopicName,
		Balancer: &kafka.Hash{},
	}
}

// deliverToKafka 消息投递函数（核心抽离）
func deliverToKafka(ctx context.Context, deviceID string, payload []byte) error {
	msg := kafka.Message{
		Key:   []byte(deviceID),
		Value: payload,
	}

	// 带超时控制写入
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := kafkaWriter.WriteMessages(writeCtx, msg); err != nil {
		log.Printf("Kafka写入失败 [device:%s]: %v", deviceID, err)
		return err
	}
	log.Printf("已投递到Kafka [device:%s]", deviceID)
	return nil
}

// processMQTTMessage MQTT消息处理函数
func processMQTTMessage(msg mqtt.Message) {
	// 解析原始消息
	var data map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &data); err != nil {
		log.Printf("JSON解析失败 [topic:%s]: %v", msg.Topic(), err)
		return
	}

	// 添加处理标记
	data["processed_by"] = mqttClientID

	// 提取设备ID
	topicParts := strings.Split(msg.Topic(), "/")
	deviceID := ""
	if len(topicParts) >= 2 {
		deviceID = topicParts[1]
	}

	// 序列化处理后的消息
	modifiedMsg, err := json.Marshal(data)
	if err != nil {
		log.Printf("JSON序列化失败 [device:%s]: %v", deviceID, err)
		return
	}

	// 投递到Kafka
	if err := deliverToKafka(context.Background(), deviceID, modifiedMsg); err != nil {
		// 错误日志已在deliverToKafka中记录
		return
	}
}

func main() {
	// 初始化Kafka
	kafkaWriter = initKafkaWriter()
	defer kafkaWriter.Close()

	// 配置MQTT客户端
	opts := mqtt.NewClientOptions().
		AddBroker("tcp://emqx:1883").
		SetClientID(mqttClientID).
		SetCleanSession(false)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("MQTT连接失败: %v", token.Error())
	}

	// 订阅通配符主题
	token := client.Subscribe("device/+/telemetry", 1, func(_ mqtt.Client, msg mqtt.Message) {
		processMQTTMessage(msg) // 处理消息
	})
	if token.Wait() && token.Error() != nil {
		log.Fatalf("订阅失败: %v", token.Error())
	}

	// 优雅退出
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	client.Disconnect(250)
	log.Println("服务正常退出")
}
