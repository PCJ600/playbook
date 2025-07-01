package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// === 配置区 ===
const (
	EMQX_SERVER      = "tcp://emqx:1883"
	BASE_TOPIC       = "device/%s/telemetry"
	PUBLISH_INTERVAL = 5 * time.Second
	SIMULATED_DEVICES = 4
	MAX_RETRIES      = 3
	KEEPALIVE        = 120
)

// DeviceSimulator 模拟单个设备
type DeviceSimulator struct {
	deviceID string
	client   mqtt.Client
	topic    string
}

// NewDeviceSimulator 创建新设备模拟器
func NewDeviceSimulator(deviceID string) *DeviceSimulator {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(EMQX_SERVER)
	opts.SetClientID(fmt.Sprintf("device_%s", deviceID))
	opts.SetCleanSession(false)
	opts.SetKeepAlive(KEEPALIVE * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("Received message: %s\n", msg.Payload())
	})

	return &DeviceSimulator{
		deviceID: deviceID,
		client:   mqtt.NewClient(opts),
		topic:    fmt.Sprintf(BASE_TOPIC, deviceID),
	}
}

// Connect 连接MQTT Broker
func (d *DeviceSimulator) Connect() error {
	token := d.client.Connect()
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	log.Printf("设备 %s 连接成功", d.deviceID)
	return nil
}

// GenerateTelemetry 生成设备遥测数据
func (d *DeviceSimulator) GenerateTelemetry() map[string]interface{} {
	return map[string]interface{}{
		"device_id":  d.deviceID,
		"timestamp":  time.Now().Unix(),
		"voltage":    220 + rand.Float64()*20 - 10,
		"current":    5 + rand.Float64()*4 - 2,
		"temperature": 30 + rand.Float64()*10 - 5,
		"location": map[string]float64{
			"lat": 39.9 + rand.Float64()*0.2 - 0.1,
			"lng": 116.4 + rand.Float64()*0.2 - 0.1,
		},
	}
}

// Run 启动设备模拟
func (d *DeviceSimulator) Run(wg *sync.WaitGroup) {
	defer wg.Done()

	retryCount := 0
	for {
		if err := d.Connect(); err != nil {
			if retryCount >= MAX_RETRIES {
				log.Printf("设备 %s 达到最大重试次数，放弃连接", d.deviceID)
				return
			}
			retryCount++
			delay := time.Duration(retryCount*retryCount) * time.Second
			log.Printf("设备 %s 连接失败，%v后重试... 错误: %v", d.deviceID, delay, err)
			time.Sleep(delay)
			continue
		}
		break
	}


	for {
		data := d.GenerateTelemetry()
		payload, _ := json.Marshal(data)
		token := d.client.Publish(d.topic, 1, false, payload)
		if token.Wait() && token.Error() != nil {
			log.Printf("设备 %s 发布数据失败: %v", d.deviceID, token.Error())
			continue
		}
		// log.Printf("设备 %s 发布数据: %s", d.deviceID, string(payload))
        time.Sleep(PUBLISH_INTERVAL)
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup
	wg.Add(SIMULATED_DEVICES)

	for i := 1; i <= SIMULATED_DEVICES; i++ {
		simulator := NewDeviceSimulator(fmt.Sprintf("%d", i))
		go simulator.Run(&wg)
        time.Sleep(time.Duration(10) * time.Microsecond)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	log.Printf("收到中断信号，关闭模拟器...")
	log.Printf("所有设备已停止")
}
