package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	// "time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func main() {
	// 命令行参数解析
	var queueName string
	flag.StringVar(&queueName, "queue", "device_telemetry_queue", "自定义队列名（默认: device_telemetry_queue）")
	flag.Parse()

	// 连接 RabbitMQ
	conn, err := amqp.Dial("amqp://admin:admin@rabbitmq:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	// 创建 Channel
	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	// 声明持久化 Topic 交换机
	err = ch.ExchangeDeclare(
		"device",
		"topic",
		true,  // 持久化
		false, // 不自动删除
		false, // 非内部
		false, // 不等待
		nil,
	)
	failOnError(err, "Failed to declare an exchange")

	// 声明持久化队列（队列名通过参数指定）
	_, err = ch.QueueDeclare(
		queueName,
		true,  // 持久化
		false, // 不自动删除
		false, // 非排他
		false, // 不等待
		nil,
	)
	failOnError(err, "Failed to declare a queue")

	// 绑定队列到交换机（路由键: device.telemetry.*）
	err = ch.QueueBind(
		queueName,
		"device.telemetry.*",
		"device",
		false,
		nil,
	)
	failOnError(err, "Failed to bind a queue")

	// 设置 QoS（公平分发）
	err = ch.Qos(
		1,     // 每个消费者最多同时处理1条消息
		0,     // 无大小限制
		false, // 仅对当前Channel有效
	)
	failOnError(err, "Failed to set QoS")

	// 注册消费者
	msgs, err := ch.Consume(
		queueName,
		"",
		false, // 手动确认
		false,
		false,
		false,
		nil,
	)
	failOnError(err, "Failed to register a consumer")

	// 优雅退出处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf(" [*] 等待消息 (队列: %s, 路由键: device.telemetry.*). 退出按 CTRL+C", queueName)

	// 消费消息
	for {
		select {
		case d := <-msgs:
			log.Printf(" [x] 收到设备遥测: %s (路由键: %s)", d.Body, d.RoutingKey)
			d.Ack(false)               // 手动确认
		case <-sigChan:
			log.Println("收到终止信号，退出...")
			return
		}
	}
}
