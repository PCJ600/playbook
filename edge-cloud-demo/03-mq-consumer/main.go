package main

import (
	"context"
	"errors"
    "fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	topic         = "device.telemetry"
	consumerGroup = "telemetry-processor"
	kafkaBrokers  = "kafka:9092"
)


func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reader := createKafkaReader()
	defer reader.Close()

	log.Printf("Start Kafka consumer [topic:%s, group:%s]", topic, consumerGroup)
	consumeMessages(ctx, reader)
}

// 创建Kafka消费者实例
func createKafkaReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{kafkaBrokers},
		GroupID:     consumerGroup,
		Topic:       topic,
        MinBytes:    1e3,
        MaxBytes:    10e6,
		MaxWait:     100 * time.Millisecond,
		StartOffset: kafka.LastOffset,
	})
}

// consumeMessages 核心消费逻辑
func consumeMessages(ctx context.Context, reader *kafka.Reader) {
	for {
		select {
		case <-ctx.Done():
			log.Println("收到终止信号，停止消费")
			return
		default:
			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				handleFetchError(ctx, err)
				continue
			}

			if err := processMessage(ctx, reader, msg); err != nil {
				log.Printf("消息处理失败: %v", err)
			}
		}
	}
}

func processMessage(ctx context.Context, reader *kafka.Reader, msg kafka.Message) error {
    log.Printf("Receive Kafka Msg: Topic=%s Partition=%d, Offset=%d", msg.Topic, msg.Partition, msg.Offset)
    log.Printf("Msg Key: %s", string(msg.Key))
    log.Printf("Msg Value: %s", string(msg.Value))

    // 手动ACK确认
	if err := commitMessage(ctx, reader, msg); err != nil {
		return fmt.Errorf("提交偏移量失败: %w", err)
	}
	return nil
}


func handleFetchError(ctx context.Context, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	log.Printf("获取消息失败: %v (等待重试)", err)
	time.Sleep(1 * time.Second)
}


func commitMessage(ctx context.Context, reader *kafka.Reader, msg kafka.Message) error {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		err := reader.CommitMessages(ctx, msg)
		if err == nil {
			return nil
		}
		if i == maxRetries-1 {
			return err
		}
		time.Sleep(time.Second * time.Duration(i+1))
	}
	return nil
}
