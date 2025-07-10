package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	mqttBroker   = "tcp://emqx:1883"
	mqttTopic    = "device/+/telemetry"
	rabbitMQURL  = "amqp://admin:admin@rabbitmq:5672/"
	exchangeName = "device" // RabbitMQ内置topic交换器
)

func main() {
	// 初始化MQTT客户端
	mqttClient := connectMQTT()
	defer mqttClient.Disconnect(250)

	// 初始化RabbitMQ连接
	rabbitConn, rabbitCh := setupRabbitMQ()
	defer rabbitConn.Close()
	defer rabbitCh.Close()

	// 订阅MQTT主题
	token := mqttClient.Subscribe(mqttTopic, 1, createMessageHandler(rabbitCh))
	if token.Wait() && token.Error() != nil {
		log.Fatalf("MQTT订阅失败: %v", token.Error())
	}

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("程序终止")
}

// 连接MQTT Broker
func connectMQTT() MQTT.Client {
	opts := MQTT.NewClientOptions().AddBroker(mqttBroker)
	opts.SetClientID("mqtt-to-rabbitmq-bridge")
	opts.SetAutoReconnect(true)

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("MQTT连接失败: %v", token.Error())
	}
	return client
}

// 设置RabbitMQ连接和通道
func setupRabbitMQ() (*amqp.Connection, *amqp.Channel) {
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("RabbitMQ连接失败: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("无法打开通道: %v", err)
	}

	// 声明交换器（使用已存在的amq.topic）
	err = ch.ExchangeDeclare(
		exchangeName,
		"topic", // topic类型
		true,    // 持久化
		false,   // 不自动删除
		false,   // 不内部使用
		false,   // 不等待
		nil,
	)
	if err != nil {
		log.Fatalf("交换器声明失败: %v", err)
	}

	return conn, ch
}

// 创建MQTT消息处理器
func createMessageHandler(ch *amqp.Channel) MQTT.MessageHandler {
	return func(client MQTT.Client, msg MQTT.Message) {
		// 从MQTT主题提取设备ID
		topicParts := strings.Split(msg.Topic(), "/")
		if len(topicParts) != 3 {
			log.Printf("无效主题格式: %s", msg.Topic())
			return
		}
		deviceID := topicParts[1]

		// 构造RabbitMQ路由键 (device.telemetry.{deviceID})
		routingKey := "device.telemetry." + deviceID

		// 发布到RabbitMQ
		err := ch.Publish(
			exchangeName,
			routingKey,
			false, // 非强制
			false, // 非立即
			amqp.Publishing{
				ContentType: "application/json",
				Body:       msg.Payload(),
			},
		)
		if err != nil {
			log.Printf("消息投递失败: %v (设备: %s)", err, deviceID)
		} else {
			log.Printf("已转发消息: %s => %s", msg.Topic(), routingKey)
		}
	}
}
