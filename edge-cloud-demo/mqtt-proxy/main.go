package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/segmentio/kafka-go"
)

const (
	mqttClientID = "mqtt-proxy" // 使用固定的ClientID
)

func main() {
	// 配置参数
	broker := "tcp://emqx:1883"
	topic := "device/+/telemetry"

	// 创建Kafka writer
	kafkaWriter := &kafka.Writer{
		Addr:     kafka.TCP("kafka:9092"), // Kafka broker地址
		Topic:    "device.telemetry",
		Balancer: &kafka.Hash{},
	}
	defer kafkaWriter.Close()

	// 创建MQTT客户端
	opts := mqtt.NewClientOptions().AddBroker(broker)
	opts.SetClientID(mqttClientID) // 使用固定ClientID
	opts.SetCleanSession(false)    // 启用持久会话

	client := mqtt.NewClient(opts)

	// 连接MQTT
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("MQTT连接失败: ", token.Error())
	}

	// 订阅通配符主题
	if token := client.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
		// 解析并修改消息
		var msg map[string]interface{}
		if err := json.Unmarshal(m.Payload(), &msg); err != nil {
			log.Println("JSON解析错误:", err)
			return
		}
		msg["modified"] = true

		// 输出处理后的消息
		if modifiedMsg, err := json.Marshal(msg); err == nil {
			log.Println(string(modifiedMsg))

			// 提取topic_id作为Kafka key
			topicParts := strings.Split(m.Topic(), "/")
			topicID := ""
			if len(topicParts) >= 2 {
				topicID = topicParts[1] // 获取设备ID部分
			}

			// 发布到Kafka
			err := kafkaWriter.WriteMessages(context.Background(),
				kafka.Message{
					Key:   []byte(topicID),
					Value: modifiedMsg,
				},
			)
			if err != nil {
				log.Println("写入Kafka失败:", err)
			}
		}
	}); token.Wait() && token.Error() != nil {
		log.Fatal("订阅失败: ", token.Error())
	}

	// 等待退出信号
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	client.Disconnect(250)
}
