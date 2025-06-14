package main

import (
	"math"
	"testing"
	"log"
	"sync"
	"encoding/gob"
	"os"
	"path/filepath"
	"fmt"
	"bytes"

)

func distance (p1 Point, p2 Point) float64 {
	const R = 6371000
	lat1 := p1.Latitude * math.Pi / 180
    	lat2 := p2.Latitude * math.Pi / 180
    	deltaLat := (p2.Latitude - p1.Latitude) * math.Pi / 180
    	deltaLon := (p2.Longitude - p1.Longitude) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
        math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
    	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
    	return R * c
}

func interPoints(p1, p2 Point, v1, v2 [2]float64, numPoints int) []Point {
    answer := make([]Point, numPoints)
    for i := 0; i < numPoints; i++ {
        t := float64(i+1) / float64(numPoints+1)
        h1 := 2*t*t*t - 3*t*t + 1
        h2 := -2*t*t*t + 3*t*t
        h3 := t*t*t - 2*t*t + t
        h4 := t*t*t - t*t

        lon := h1*p1.Longitude + h2*p2.Longitude + h3*v1[0] + h4*v2[0]
        lat := h1*p1.Latitude + h2*p2.Latitude + h3*v1[1] + h4*v2[1]

        answer[i] = Point{Longitude: lon, Latitude: lat}
    }
    return answer
}

func speedOutliner(aTask Data) []Point {
	start := aTask.Start
	end := aTask.End
	points := aTask.Points
	lenth := end-start+1

	// 检查空切片
    if len(points) == 0 {
        log.Println("错误: Points 切片为空")
        return []Point{}
    }

    // 验证索引范围
    if start < 0 || start >= len(points) || end < 0 || end >= len(points) || start > end {
        // log.Println("完成任务派发")
        return []Point{}
    }

	result := make([]Point, lenth)
	copy(result, points[start : end+1])

	isAno := make([]bool, lenth)

	for i := 0; i < lenth; i++ {
		idx := start + i
		if idx == 0 || idx == len(points)-1 {
			continue
		}
		dist1 := distance(points[idx-1], points[idx])
		v1 := dist1 / 1.0

		dist2 := distance(points[idx], points[idx+1])
		v2 := dist2 / 1.0

		a := math.Abs(v2 - v1) / 1.0

		const sheld = 10.0
		if a > sheld {
			isAno[i] = true

		}
	}
	
	var allAno [][]int
	var aAno []int

	for i := 0; i < len(isAno); i++ {
		if isAno[i] {
			aAno = append(aAno, i)
		} else if len(aAno) > 0 {
			allAno = append(allAno, aAno)
			aAno = nil
		}

	}
	if len(aAno) > 0 {
		allAno = append(allAno, aAno)
	}

	for _, group := range allAno {
		if len(group) == 0 {
			log.Println("出现group长度为0！")
			continue
		}
		beginIdx := group[0]
		lastIdx := group[len(group)-1]
		numAnomalies := lastIdx - beginIdx + 1

		if numAnomalies <= 0 {
			log.Println("错误：长度<=0")
			continue
		}

		indexP := max(0, start+beginIdx - 1)
		indexN := min(len(points)-1, start + lastIdx + 1)
		pointP := points[indexP]
		pointN := points[indexN]

		var v1, v2[2]float64
		if indexP > 0 {
			v1 = [2]float64 {
				points[indexP].Longitude - points[indexP - 1].Longitude,
				points[indexP].Latitude - points[indexP - 1].Latitude,
		}
	}else {
		v1 = [2]float64{
			pointN.Longitude - pointP.Longitude,
			pointN.Latitude - pointN.Latitude,
		}
		if indexN < len(points)-1 {
			v2 = [2]float64{
				points[indexN+1].Longitude - points[indexN].Longitude,
				points[indexN+1].Latitude - points[indexN].Latitude,
			}
		}else {
			v2 = [2]float64{
				pointN.Longitude - pointP.Longitude,
				pointN.Latitude - pointP.Latitude,
			}
		}
	}

		correctPoints := interPoints(pointP, pointN, v1, v2, numAnomalies)
		if len(result) !=  lenth{
			log.Println("result != lenth！长度出错")
		}

		for j, idx := range group {
			result[idx] = correctPoints[j]
		}	

	}
	return result

}

func worker_1(id int, tasks <-chan Data, results chan<- struct {
	TaskIdx int
	Points  []Point
}) {
	for task := range tasks {
		log.Printf("Worker %d 处理任务%d: StartIdx=%d, EndIdx=%d，长度%d", id,task.TaskCode, task.Start, task.End, len(task.Points))
		processedPoints := speedOutliner(task)
		results <- struct {
			TaskIdx int
			Points  []Point
		}{
			TaskIdx: task.TaskCode,  // 传递任务的 TaskCode
			Points:  processedPoints, // 任务处理后的点
		}
	}
}

// 将(taskIdx, points)序列化并写入文件
func writePoints(taskIdx int, points []Point, directory string) error {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(points)
	if err != nil {
		return fmt.Errorf("Points 序列化失败: %v", err)
	}

	filePath := filepath.Join(directory, fmt.Sprintf("%d.gob", taskIdx))
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	// 将序列化的 Points 写入文件
	_, err = file.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil

}

func worker_2(id int, tasks <-chan struct{ TaskIdx int; Points []Point }, results chan<- struct{ TaskIdx int; Points []Point }, indexTable *IndexTable, directory string, wg *sync.WaitGroup) {
	for task := range tasks {
		if len(task.Points) <= 0 {
			continue
		}
		p1 := task.Points[0]
		p2 := task.Points[len(task.Points)-1]
		indexTable.AddRange(p1, p2, task.TaskIdx)
		// 可增加一层循环找经纬度最值，这里简化为轨迹端点

		// 写入点文件
		err := writePoints(task.TaskIdx, task.Points, directory)
		if err != nil {
			log.Printf("错误: 写入 TaskIdx %d 的 Points 到文件失败: %v", task.TaskIdx, err)
			continue
		}

		// 将 IndexTable 序列化并保存到目录
		err = indexTable.SerializeIndexTable(directory)
		if err != nil {
			log.Printf("错误: 序列化 IndexTable 失败: %v", err)
			continue
		}

		log.Printf("Worker %d 完成任务 %d", id, task.TaskIdx)
		// results <- task // 任务结果不需要发送回去
	}
	wg.Done() // 在处理完所有任务后调用
}



func TestWorkerA(t *testing.T) {
	testPoints := []Point{
        {Longitude: 0.0, Latitude: 0.0},   // P0
        {Longitude: 0.1, Latitude: 0.0},   // P1
        {Longitude: 0.2, Latitude: 0.0},   // P2
        {Longitude: 0.3, Latitude: 0.1},   // P3 (异常点)
        {Longitude: 0.4, Latitude: 0.0},   // P4
        {Longitude: 0.5, Latitude: 0.0},   // P5
        {Longitude: 0.6, Latitude: 0.0},   // P6
        {Longitude: 0.7, Latitude: 0.2},   // P7 (异常点)
        {Longitude: 0.8, Latitude: 0.0},   // P8
        {Longitude: 0.9, Latitude: 0.0},   // P9
    }

    tasks := []Data{
        {
            Points:   testPoints[0:5], // P0 ~ P4
            Start: 1,               // P1
            End:   3,               // P3
	    TaskCode: 0, 
        },
        {
            Points:   testPoints[3:8], // P3 ~ P7
            Start: 1,               // P4
            End:   3,               // P6
	    TaskCode: 1, 
        },
        {
            Points:   testPoints[6:10], // P6 ~ P9
            Start: 1,                // P7
            End:   2,                // P8
	    TaskCode: 2, 
        },
    }

    taskChan := make(chan Data, len(tasks))
    resultChan := make(chan struct {
	    TaskIdx int
	    Points []Point
    }, len(tasks))

  // Start worker goroutines
	var wg sync.WaitGroup
	numWorkers := 3
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker_1(id, taskChan, resultChan)
		}(i)
	}

	// Send tasks to taskChan
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	// Collect results from resultChan
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		log.Printf("Task %d completed, processed %d points\n", result.TaskIdx, len(result.Points))
		for _, p := range result.Points {
			log.Printf("(%f, %f)", p.Longitude, p.Latitude)

		}
	}


}
