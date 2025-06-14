package main

import (
	"fmt"
	"log"
	"sync"
	"encoding/gob"
	"path/filepath"
	"os"
)

type Point struct {
	Longitude float64
	Latitude float64
}

type Data struct {
	Points []Point
	Start int
	End int
	TaskCode int
}

type IndexTable struct {
	Ranges map[string]int
	mu     sync.RWMutex
}

func NewIndexTable() *IndexTable {
	return &IndexTable{
		Ranges: make(map[string]int),
	}
}

func (it *IndexTable) AddRange(p1, p2 Point, taskIdx int) {
    it.mu.Lock()  // 写优先
    defer it.mu.Unlock()
    // 生成索引 key
    rangeKey := fmt.Sprintf("%f,%f,%f,%f", p1.Longitude, p1.Latitude, p2.Longitude, p2.Latitude)
    // 索引对应taskIdx
    it.Ranges[rangeKey] = taskIdx
}

// 查找表格是否具有包含点(x,y)的范围数据块，返回该数据块对应的TaskIdx
func (it *IndexTable) isContain(x, y float64) (int, bool) {
	 it.mu.RLock()
    defer it.mu.RUnlock()

    for key, taskIdx := range it.Ranges {
        // 解析索引键
        var p1, p2 Point
        _, err := fmt.Sscanf(key, "%f,%f,%f,%f", &p1.Longitude, &p1.Latitude, &p2.Longitude, &p2.Latitude)
        if err != nil {
            log.Println("解析索引键时出错:", err)
            continue
        }

        // 检查点是否在矩形范围内
        if x >= p1.Longitude && x <= p2.Longitude && y >= p1.Latitude && y <= p2.Latitude {
            return taskIdx, true
        }
    }
    return -2, false
}

// 序列化表格
func (it *IndexTable) SerializeIndexTable(directory string) error {
	it.mu.RLock()
	defer it.mu.RUnlock()

	// 创建一个文件以保存序列化数据
	filePath := filepath.Join(directory, "IndexTable.gob")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	// 序列化 IndexTable
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(it)
	if err != nil {
		return fmt.Errorf("序列化失败: %v", err)
	}

	return nil
}

