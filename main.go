package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"path/filepath"
	"strconv"
	"strings"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"runtime/trace"

	"os/exec"
)

func printHelp() {
	fmt.Println("Usage: TrackHelper (STORE|READ) SOURCE [DEST] [\"(float64,float64\"...]")
	fmt.Println("This is a concurrent storage and reading program for trajectory data in XLSX format!")
	fmt.Println("Arguments: ")
	fmt.Println("  STORE:    mode of stroing Data, you need to provide an path \"SOURCE\" points to EXSITING and WELL-FORMED XLEX file and an AVALIABLE directory path at position \"DEST\".")
	fmt.Println("  READ:     mode of reading and query Data, you need to provide an AVALIABLE directory path \"SOURCE\" which owns an IndexTable.gop file and some xx.gob points files; and some points in form (float64, float64), the program will generate a picture called \"trajectory.png\" in your work directory and you can check it")
	fmt.Println("After running, the program will generate a \"trace.out\" file and you can view the situation of each Goroutine by using \"go tool trace trace.out\" ")


}

func execSTORE(excelPath, directory string) {
		// 读取数据
	points, err := readData(excelPath)
	if err != nil {
		log.Fatalf("读取数据失败: %v", err)
	}

	// 划分数据块
	tasks := splitT(points, 0.001, 0.001, 2)

	// 创建目录
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err := os.Mkdir(directory, os.ModePerm)
		if err != nil {
			log.Fatalf("创建目录失败: %v", err)
		}
	}

	// 创建并初始化IndexTable
	indexTable := NewIndexTable()

	// 如果IndexTable文件不存在，序列化并保存
	indexTablePath := filepath.Join(directory, "IndexTable.gob")
	if _, err := os.Stat(indexTablePath); os.IsNotExist(err) {
		err := indexTable.SerializeIndexTable(directory)
		if err != nil {
			log.Fatalf("序列化IndexTable失败: %v", err)
		}
	}


	taskChannel := make(chan Data, len(tasks))
	worker1Channel := make(chan struct{ TaskIdx int; Points []Point }, len(tasks))
	worker2Channel := make(chan struct{ TaskIdx int; Points []Point }, len(tasks))

	// 使用指定数量的goroutines来处理任务
	var wg1 sync.WaitGroup // 用于 worker_1
	var wg2 sync.WaitGroup // 用于 worker_2
	numWorker1 := 4
	numWorker2 := 4

	// 启动worker_1线程
	for i := 0; i < numWorker1; i++ {
		wg1.Add(1)
		go func(id int) {
			defer wg1.Done()
			worker_1(id, taskChannel, worker1Channel)
		}(i)
	}

	// 启动worker_2线程
	for i := 0; i < numWorker2; i++ {
		wg2.Add(1)
		go worker_2(i, worker1Channel, worker2Channel, indexTable, directory, &wg2)
	}

	// 将任务数据放入taskChannel
	for _, task := range tasks {
		taskChannel <- task
	}
	close(taskChannel)

	// 等待worker_1完成
	wg1.Wait()
	close(worker1Channel)

	// 等待worker_2完成
	wg2.Wait()
	close(worker2Channel)

	log.Println("所有任务处理完成")
	return


}

func execREAD(points []Point, directory string) {
    indexTable, err := readIndexTable(directory)
    if err != nil {
        log.Fatalf("读取索引表失败: %v", err)
    }

    p := plot.New()
    p.Title.Text = "轨迹数据"
    p.X.Label.Text = "经度"
    p.Y.Label.Text = "纬度"

    // 新增：保护 plot 对象
    var plotMu sync.Mutex

    pointCh := make(chan Point, len(points))

    var wg sync.WaitGroup
    numThreads := 4

    for i := 0; i < numThreads; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for pt := range pointCh {
                // 在调用 searchAndPlotPoints 前后传入 plotMu
                searchAndPlotPoints(pt.Longitude, pt.Latitude, directory, indexTable, p, &plotMu)
            }
        }()
    }

    for _, pt := range points {
        pointCh <- pt
    }
    close(pointCh)
    wg.Wait()

    if err := p.Save(4*vg.Inch, 4*vg.Inch, "trajectory.png"); err != nil {
        log.Fatalf("保存图表失败: %v", err)
    }
    log.Println("轨迹图保存为 trajectory.png")
}


func traceGO() {
	f, err := os.Create("trace.out")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()

     if err := trace.Start(f); err != nil {
        log.Fatal(err)
    }
    defer trace.Stop()
}

func main() {

	traceGO()

	if len(os.Args) < 3 {
		printHelp()
		return
	}

	mode := os.Args[1]
	directory := os.Args[2]
	if mode == "STORE" {
		if len(os.Args) != 4 {
			fmt.Println("ERROR: Missing Arguments!")
			return
		}

		filePath := os.Args[3]
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			fmt.Println("ERROR: File is not existing!")
			return
		}

		if _, err := os.Stat(directory); os.IsNotExist(err) {
			fmt.Println("ERROR: Directory is not existing!")
			return
		}

	// 固定格式的excel文件路径
	excelPath := filePath
	directory := directory + "/output"

	execSTORE(excelPath, directory)


}else if mode == "READ" {
	if len(os.Args) < 4 {
		fmt.Println("ERROR: You need to provide an avaliable directory path & at least 1 Point data like (Longitude, Latitude) ! ")
			return
		}
	if _, err := os.Stat(directory); os.IsNotExist(err) {
			fmt.Println("ERROR: The Directory is not Existing!")
			return
		}

	var points []Point
	
	for i, arg := range os.Args[3:] {
			// 去掉多余的空格
			arg = strings.TrimSpace(arg)
			// 检查格式是否为 "(A,B)"
			if !strings.HasPrefix(arg, "(") || !strings.HasSuffix(arg, ")") {
				fmt.Printf("错误: 参数 #%d 格式错误，应该是 (A,B) 的形式: %s\n", i+3, arg)
				return
			}
			// 去掉括号并分割
			arg = arg[1 : len(arg)-1] // 去掉两边的括号
			parts := strings.Split(arg, ",")
			if len(parts) != 2 {
				fmt.Printf("错误: 参数 #%d 格式错误，应该有两个数值: %s\n", i+3, arg)
				return
			}
			// 解析 A 和 B 为 float64
			A, err := strconv.ParseFloat(parts[0], 64)
			if err != nil {
				fmt.Printf("错误: 参数 #%d A 值解析失败，应该是浮动值: %s\n", i+3, parts[0])
				return
			}
			B, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				fmt.Printf("错误: 参数 #%d B 值解析失败，应该是浮动值: %s\n", i+3, parts[1])
				return
			}


			points = append(points, Point{
				Longitude: A, Latitude: B,
			})
		}


		execREAD(points, directory)

		path1 := "../trajectory.png" // 请将这个路径替换成实际的图片路径

	// 使用 Linux 命令 cp 拷贝文件，并重命名为 trajectory.png
	cmd := exec.Command("cp", path1, "./trajectory.png")

	// 执行命令
	 cmd.Run()

	}else {
		fmt.Println("ERROR: Wrong Mode Setting Argument!! ")
		return
	}



}
