package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"time"

	"github.com/gosnmp/gosnmp"
)

// 磁盘信息结构体
type DiskInfo struct {
	Index       int
	Description string
	TotalBytes  uint64
	UsedBytes   uint64
	FreeBytes   uint64
	Usage       float64
}

func main() {
	// 配置SNMP客户端
	snmp := &gosnmp.GoSNMP{
		Target:    "192.168.1.100", // 服务器IP
		Port:      161,
		Community: "public",
		Version:   gosnmp.Version2c,
		Timeout:   5 * time.Second,
	}

	// 连接SNMP
	if err := snmp.Connect(); err != nil {
		log.Fatalf("SNMP连接失败: %v", err)
	}
	defer snmp.Conn.Close()

	// 获取所有磁盘信息
	disks, err := getDiskUsage(snmp)
	if err != nil {
		log.Fatalf("获取磁盘信息失败: %v", err)
	}

	// 打印结果
	for _, disk := range disks {
		fmt.Printf("磁盘: %s\n", disk.Description)
		fmt.Printf("  总空间: %.2f GB\n", float64(disk.TotalBytes)/1024/1024/1024)
		fmt.Printf("  已用空间: %.2f GB\n", float64(disk.UsedBytes)/1024/1024/1024)
		fmt.Printf("  空闲空间: %.2f GB\n", float64(disk.FreeBytes)/1024/1024/1024)
		fmt.Printf("  使用率: %.2f%%\n", disk.Usage)
		fmt.Println("----------------------")
	}

	// 启动Prometheus HTTP服务
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":9100", nil)

	// 在main函数中调用
	for _, disk := range disks {
		if alert := checkDiskThreshold(disk, 80, 90); alert != "" {
			sendAlert(alert)
		}
		diskUsage.WithLabelValues(disk.Description).Set(disk.Usage)
	}
}

func getDiskUsage(snmp *gosnmp.GoSNMP) ([]DiskInfo, error) {
	// 获取存储索引列表
	indexOid := "1.3.6.1.2.1.25.2.3.1.1"
	indexResult, err := snmp.WalkAll(indexOid)
	if err != nil {
		return nil, fmt.Errorf("获取存储索引失败: %w", err)
	}

	// 创建索引到磁盘的映射
	diskMap := make(map[int]*DiskInfo)
	for _, pdu := range indexResult {
		index := pdu.Value.(int)
		diskMap[index] = &DiskInfo{Index: index}
	}

	// 批量查询磁盘属性
	oids := []string{
		"1.3.6.1.2.1.25.2.3.1.2", // 存储类型
		"1.3.6.1.2.1.25.2.3.1.3", // 描述
		"1.3.6.1.2.1.25.2.3.1.4", // 分配单元大小
		"1.3.6.1.2.1.25.2.3.1.5", // 总大小
		"1.3.6.1.2.1.25.2.3.1.6", // 已用大小
	}

	for _, oid := range oids {
		result, err := snmp.BulkWalkAll(oid)
		if err != nil {
			return nil, fmt.Errorf("查询OID %s失败: %w", oid, err)
		}

		for _, pdu := range result {
			// 从OID提取索引
			index := extractIndexFromOID(pdu.Name)
			disk, exists := diskMap[index]
			if !exists {
				continue
			}

			switch oid {
			case "1.3.6.1.2.1.25.2.3.1.2": // 类型
				// 只处理物理磁盘
				if pdu.Value.(string) != "hrStorageFixedDisk" {
					delete(diskMap, index)
				}
			case "1.3.6.1.2.1.25.2.3.1.3": // 描述
				disk.Description = string(pdu.Value.([]byte))
			case "1.3.6.1.2.1.25.2.3.1.4": // 单元大小
				disk.TotalBytes = uint64(pdu.Value.(uint)) * uint64(pdu.Value.(uint))
			case "1.3.6.1.2.1.25.2.3.1.5": // 总单元数
				disk.TotalBytes *= uint64(pdu.Value.(uint))
			case "1.3.6.1.2.1.25.2.3.1.6": // 已用单元数
				disk.UsedBytes = uint64(pdu.Value.(uint)) * disk.TotalBytes / uint64(disk.TotalBytes) // 简化计算
			}
		}
	}

	// 计算最终结果
	var disks []DiskInfo
	for _, disk := range diskMap {
		disk.FreeBytes = disk.TotalBytes - disk.UsedBytes
		disk.Usage = float64(disk.UsedBytes) / float64(disk.TotalBytes) * 100
		disks = append(disks, *disk)
	}

	return disks, nil
}

// 从OID提取索引号
func extractIndexFromOID(oid string) int {
	// OID格式: .1.3.6.1.2.1.25.2.3.1.X.INDEX
	lastDot := 0
	for i := len(oid) - 1; i >= 0; i-- {
		if oid[i] == '.' {
			lastDot = i
			break
		}
	}

	var index int
	fmt.Sscanf(oid[lastDot+1:], "%d", &index)
	return index
}
