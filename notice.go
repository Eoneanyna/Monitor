package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func checkDiskThreshold(disk DiskInfo, warn, critical float64) string {
	switch {
	case disk.Usage >= critical:
		return fmt.Sprintf("CRITICAL: %s使用率%.2f%%", disk.Description, disk.Usage)
	case disk.Usage >= warn:
		return fmt.Sprintf("WARNING: %s使用率%.2f%%", disk.Description, disk.Usage)
	default:
		return ""
	}
}

func sendAlert(message string) {
	webhook := "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN"

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": "磁盘告警: " + message,
		},
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("钉钉告警发送失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("钉钉告警发送失败: %s", resp.Status)
	}
}
