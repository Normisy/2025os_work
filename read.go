package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"encoding/gob"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	// "gonum.org/v1/plot/plotutil"
	"math"
	"sync"
)

func readIndexTable(directory string) (*IndexTable, error) {
	filePath := filepath.Join(directory, "IndexTable.gob")
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("索引表损毁: %v", err)
	}
	defer file.Close()

	var indexTable IndexTable
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&indexTable)
	if err != nil {
		return nil, fmt.Errorf("解码索引表失败: %v", err)
	}

	return &indexTable, nil
}

func readPointsFromFile(taskIdx int, directory string) ([]Point, error) {
	filePath := filepath.Join(directory, fmt.Sprintf("%d.gob", taskIdx))
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件 %s: %v", filePath, err)
	}
	defer file.Close()

	var points []Point
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&points)
	if err != nil {
		return nil, fmt.Errorf("解码文件失败: %v", err)
	}

	return points, nil
}

// 墨卡托投影转换
func mercatorProjection(lon, lat float64) (x, y float64) {
	// 经度转换：-180 到 180 转换到 0 到 1 的范围
	x = (lon + 180) / 360

	// 纬度转换：-90 到 90 转换到 [-pi/2, pi/2]，然后做投影
	y = math.Log(math.Tan((math.Pi/4)+(lat*math.Pi/360)))

	// 将 y 映射到 [0, 1] 范围
	y = (1 - y/math.Pi) / 2

	return
}

func searchAndPlotPoints(lon, lat float64, directory string, indexTable *IndexTable, plt *plot.Plot, plotMu *sync.Mutex) {
    idx, found := indexTable.isContain(lon, lat)
    if !found {
        log.Printf("任务 %d 未找到\n", idx)
        return
    }
    pts, err := readPointsFromFile(idx, directory)
    if err != nil {
        log.Printf("读取文件失败: %v\n", err)
        return
    }

    xys := make(plotter.XYs, len(pts))
    for i, pt := range pts {
        xys[i].X, xys[i].Y = mercatorProjection(pt.Longitude, pt.Latitude)
    }

    scatter, err := plotter.NewScatter(xys)
    if err != nil {
        log.Printf("创建散点图失败: %v\n", err)
        return
    }

    // 保护对 plt 的并发修改
    plotMu.Lock()
    plt.Add(scatter)
    plotMu.Unlock()

    // 如果要动态更新 label，也要加锁；一般 label 在主 goroutine 里设置一次即可
}

