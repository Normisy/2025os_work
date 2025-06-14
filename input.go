package main

import (
	"math"
	"strconv"
	"testing"
	"fmt"
	"github.com/tealeg/xlsx"
	"reflect"
)

func readData(path string) ([]Point, error) {
	file, err := xlsx.OpenFile(path)
	if err != nil {
		return nil, err
	}

	sheet := file.Sheets[0]
	if sheet == nil {
		return nil, fmt.Errorf("该文件%s中没有工作表", path)
	}
	points := make([]Point, 0, int(len(sheet.Rows)))

	for i := 0; i < int(len(sheet.Rows)); i++ {
		row := sheet.Row(i)
		if row == nil {
			fmt.Printf("跳过空行 %d\n", i) // 添加调试输出
			continue
		}
		sa := row.Cells[0].String()
		sb := row.Cells[1].String()
		pa, err1 := strconv.ParseFloat(sa, 64)
		pb, err2 := strconv.ParseFloat(sb, 64)

		if err1 != nil || err2 != nil {
			fmt.Printf("解析错误: %v, %v 在第 %d 行\n", err1, err2, i) // 添加调试输出
			continue
		}
		points = append(points, Point{Longitude : pa, Latitude : pb})
	}
		return points, nil
}

func splitT(points []Point, maxLon float64, maxLat float64, extra int) []Data {
	if len(points) == 0 {
		return nil
	}
	
	var tasks []Data  // 分派给每个线程的任务数据
	start := 0
	taskCode := 0
	
	for i := 0; i < len(points); i++ {
		if i == 0  {
		continue
	}
	cumLon := math.Abs(points[i].Longitude - points[start].Longitude)
	cumLat := math.Abs(points[i].Latitude - points[start].Latitude)

		if cumLon > maxLon || cumLat > maxLat { // 若超出范围，生成任务数据
			begin := max(0, start - extra)
			last := min(len(points)-1, i+extra)

			var startidx int
			if start < extra {
				startidx = start
			} else {
				startidx = extra
			}

			endidx := startidx + (i - start)
			
			task := Data{
				Points: points[begin : last+1],
				Start: startidx,
				End: endidx,
				TaskCode: taskCode,
			}
			tasks = append(tasks, task)

			start = i+1
			taskCode++
			// 复原累计值，将起始索引指向当前的位置

		}
	}
	if start < len(points) {  // 处理剩下的最后一节
		begin := max(0, start - extra)
		last := len(points) - 1
		task := Data{
			Points: points[begin : last+1],
			Start: start,
			End: last,

		}
		tasks = append(tasks, task)

	}
	return tasks
}

func TestRead(t *testing.T) {
	points, err := readData("testA.xlsx")
	if err != nil {
        t.Fatalf("读取文件失败: %v", err)
    }
    expectedPoints := []Point{
        {Longitude: 85.012497, Latitude: 27.729147},
        {Longitude: 85.013000, Latitude: 27.730000},
        {Longitude: 85.014000, Latitude: 27.731000},
        {Longitude: 85.015000, Latitude: 27.732000},
        {Longitude: 85.016000, Latitude: 27.733000},
    }
   if len(points) != len(expectedPoints) {
        t.Errorf("读取的点数不正确，期望 %d，实际 %d", len(expectedPoints), len(points))
    }

    for i, p := range points {
        if p.Longitude != expectedPoints[i].Longitude || p.Latitude != expectedPoints[i].Latitude {
            t.Errorf("第 %d 个点不匹配，期望 (%f, %f)，实际 (%f, %f)",
                i, expectedPoints[i].Longitude, expectedPoints[i].Latitude, p.Longitude, p.Latitude)
        }
    }
}

func TestSplit(t *testing.T) {
	points, err := readData("testC.xlsx")
	if err != nil {
        t.Fatalf("读取文件失败: %v", err)
    }

    deltaLon := 0.001 // 经度变化阈值
    deltaLat := 0.001 // 纬度变化阈值
    overlap := 1      // 重叠点数

    // 预期任务划分
    expectedTasks := []Data{
        {
            Points:   points[0:3], // 前 3 个点（包含重叠）
            Start: 0,
            End:   2,
        },
        {
            Points:   points[1:5], // 后 4 个点（包含重叠）
            Start: 3,
            End:   4,
        },
    }
    tasks := splitT(points, deltaLon, deltaLat, overlap)

    isAll := make([]bool, len(points))
    for i := 0; i < len(tasks); i++{
	    for j := tasks[i].Start; j <= tasks[i].End; j++ {
		    isAll[j] = true
	    }
    }
    for i:= 0; i < len(isAll); i++ {
	    if !isAll[i] {
		    t.Errorf("第 %d 个点未被纳入任务中！", i)
		    continue
	    }
    }

    t.Logf("任务数:期望 %d，实际 %d", len(expectedTasks), len(tasks))
    for i := 0; i < len(tasks); i++ {
	    t.Logf("任务%d：起始索引%d, 终止索引%d", tasks[i].TaskCode, tasks[i].Start, tasks[i].End)
	    for j := 0; j < len(tasks[i].Points); j++ {
		    t. Logf("任务%d的第%d个包含点为(%v,%v)", i, j, tasks[i].Points[j].Longitude, tasks[i].Points[j].Latitude)
	    }
    }

    if len(tasks) != len(expectedTasks) {
        t.Errorf("任务数量不正确，期望 %d，实际 %d", len(expectedTasks), len(tasks))
    }

    for i, task := range tasks {
        if task.Start != expectedTasks[i].Start || task.End != expectedTasks[i].End {
            t.Errorf("任务 %d 的索引范围不匹配，期望 (%d, %d)，实际 (%d, %d)",
                i, expectedTasks[i].Start, expectedTasks[i].End, task.Start, task.End)
        }
        if !reflect.DeepEqual(task.Points, expectedTasks[i].Points) {
            t.Errorf("任务 %d 的点列表不匹配，期望 %v，实际 %v", i, expectedTasks[i].Points, task.Points)
        }
    }

}
