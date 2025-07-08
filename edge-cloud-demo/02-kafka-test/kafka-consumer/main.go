package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	topic          = "device.telemetry"
	consumerGroup  = "telemetry-processor"
	kafkaBrokers   = "kafka:9092"
	batchSize      = 100              // 每积累100条消息提交一次
	maxWaitTime    = 1 * time.Second  // 最大等待时间
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reader := createKafkaReader()
	defer reader.Close()

	log.Printf("启动消费者 [主题:%s 组:%s 批量:%d]", topic, consumerGroup, batchSize)
	consumeMessages(ctx, reader)
}

func createKafkaReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{kafkaBrokers},
		GroupID:       consumerGroup,
		Topic:         topic,
		MinBytes:      5 << 10,  // 5KB（减少小请求）
		MaxBytes:      10 << 20, // 10MB（提升批量）
		MaxWait:       maxWaitTime,
		StartOffset:   kafka.LastOffset,
	})
}

func consumeMessages(ctx context.Context, reader *kafka.Reader) {
	var (
		batch      = make([]kafka.Message, 0, batchSize)
		lastCommit = time.Now()
	)

	for {
		select {
		case <-ctx.Done():
			commitBatch(ctx, reader, batch) // 退出前提交剩余消息
			log.Println("收到终止信号，停止消费")
			return

		default:
			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				log.Printf("拉取消息失败: %v (等待重试)", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// 处理消息（示例：仅打印）
			log.Printf("收到消息 [分区:%d 偏移量:%d]", msg.Partition, msg.Offset)

			batch = append(batch, msg)

			// 触发提交条件：达到批量大小或超时
			if len(batch) >= batchSize || time.Since(lastCommit) > maxWaitTime {
				if err := commitBatch(ctx, reader, batch); err != nil {
					log.Printf("批量提交失败: %v", err)
				} else {
					batch = batch[:0] // 清空batch但保留底层数组
					lastCommit = time.Now()
				}
			}
		}
	}
}

func commitBatch(ctx context.Context, reader *kafka.Reader, batch []kafka.Message) error {

	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		if err := reader.CommitMessages(ctx, batch...); err != nil {
			if i == maxRetries-1 {
				return err
			}
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}
		break
	}

	// log.Printf("已提交 %d 条消息", len(batch))
	return nil
}
