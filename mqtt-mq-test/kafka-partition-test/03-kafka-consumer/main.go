package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
    "encoding/json"

	"github.com/segmentio/kafka-go"
)

func main() {
	// Kafka 配置
	brokers := []string{"kafka:9092"}
	topic := "device.test"
	groupID := "device-test-group"

	// 创建 Kafka Reader (消费者)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
	})

	defer reader.Close() // 程序退出时关闭消费者

	// 监听中断信号（优雅退出）
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// 消费消息
	fmt.Println("开始消费 Kafka 消息...")
	for {
		select {
		case <-sigchan:
			fmt.Println("收到终止信号，退出...")
			return
		default:
			// 读取消息（超时设置防止阻塞）
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			msg, err := reader.FetchMessage(ctx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					continue // 超时继续轮询
				}
				fmt.Printf("读取消息失败: %v\n", err)
				continue
			}

            var payload map[string]interface{}
            if err := json.Unmarshal(msg.Value, &payload); err != nil {
                fmt.Printf("消息解析失败: %v", err)
                return
            }
            deviceID, _ := payload["device_id"]
            msgID, _ := payload["message_id"]
		    fmt.Printf("Recv Msg: Topic=%s Partition=%d Offset=%d deviceID=%v, Key=%s, msgID=%v\n",
			    msg.Topic, msg.Partition, msg.Offset, deviceID, string(msg.Key), msgID)

			// 手动提交偏移量
			if err := reader.CommitMessages(context.Background(), msg); err != nil {
				fmt.Printf("提交偏移量失败: %v\n", err)
			} else {
				// fmt.Printf("已提交偏移量: Partition=%d Offset=%d\n", msg.Partition, msg.Offset)
			}
		}
	}
}


