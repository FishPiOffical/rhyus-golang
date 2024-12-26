package service

import (
	"fmt"
	"log"
	"net/http"
	netpporf "net/http/pprof"
	"os"
	"path/filepath"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"runtime/pprof"
	"sort"
	"strconv"
	"time"
)

func StartPProfServe() {
	if conf.Conf.Pprof.Enable {
		// 启动 HTTP 服务，暴露 pprof 信息
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", netpporf.Index)
		mux.HandleFunc("/debug/pprof/cmdline", netpporf.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", netpporf.Profile)
		mux.HandleFunc("/debug/pprof/symbol", netpporf.Symbol)
		mux.HandleFunc("/debug/pprof/trace", netpporf.Trace)
		go func() {
			err := http.ListenAndServe(":"+strconv.Itoa(conf.Conf.Pprof.PporfPort), mux)
			common.Log.Info("pprof server started on :%d", conf.Conf.Pprof.PporfPort)
			if err != nil {
				common.Log.Fatal(common.ExitCodeUnavailablePort, "serve start failed: %s", err)
			}
		}()

		// 定期保存各类 pprof 信息
		go func() {
			for {
				time.Sleep(time.Duration(conf.Conf.Pprof.SampleTime) * time.Minute)
				savePProf("heap")
				savePProf("cpu")
				savePProf("goroutine")
				savePProf("threadcreate")
				savePProf("allocs")
				savePProf("block")
				savePProf("mutex")
				keepRecentFiles(conf.Conf.Pprof.MaxFile * 7)
			}
		}()
	}
}

// 保存指定类型的 pprof 信息
func savePProf(profileType string) {

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("%s_profile_%s.pprof", profileType, timestamp)
	path, err := os.Getwd()
	if err != nil {
		common.Log.Fatal(common.ExitCodeFileSysErr, "get current directory failed: %s", err)
		return
	}
	path = filepath.Join(path, "pprof")
	if !common.File.IsExist(path) {
		_ = os.Mkdir(path, os.ModePerm)
	}
	filePath := filepath.Join(path, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		common.Log.Fatal(common.ExitCodeFileSysErr, "create pprof file failed: %s", err)
		return
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	switch profileType {
	case "heap":
		// 保存 heap 信息
		if err := pprof.WriteHeapProfile(file); err != nil {
			common.Log.Error("Error writing heap profile:", err)
			return
		}
	case "cpu":
		go func() { // 保存 CPU profile
			if err := pprof.StartCPUProfile(file); err != nil {
				common.Log.Error("Error starting CPU profile:", err)
				return
			}
			time.Sleep(30 * time.Second) // 记录 CPU 信息 30 秒
			pprof.StopCPUProfile()
		}()
	case "goroutine":
		// 保存 goroutine 信息
		if err := pprof.Lookup("goroutine").WriteTo(file, 0); err != nil {
			common.Log.Error("Error writing goroutine profile:", err)
			return
		}
	case "threadcreate":
		// 保存 threadcreate 信息
		if err := pprof.Lookup("threadcreate").WriteTo(file, 0); err != nil {
			common.Log.Error("Error writing threadcreate profile:", err)
			return
		}
	case "allocs":
		// 保存 allocs 信息
		if err := pprof.Lookup("allocs").WriteTo(file, 0); err != nil {
			common.Log.Error("Error writing allocs profile:", err)
			return
		}
	case "block":
		// 保存 block 信息
		if err := pprof.Lookup("block").WriteTo(file, 0); err != nil {
			common.Log.Error("Error writing block profile:", err)
			return
		}
	case "mutex":
		// 保存 mutex 信息
		if err := pprof.Lookup("mutex").WriteTo(file, 0); err != nil {
			common.Log.Error("Error writing mutex profile:", err)
			return
		}
	default:
		log.Println("Unknown profile type:", profileType)
		return
	}

	common.Log.Info("%s profile saved to %s", profileType, fileName)
}

func keepRecentFiles(maxFiles int) {
	path, err := os.Getwd()
	if err != nil {
		common.Log.Fatal(common.ExitCodeFileSysErr, "get current directory failed: %s", err)
		return
	}
	path = filepath.Join(path, "pprof")
	if !common.File.IsExist(path) {
		_ = os.Mkdir(path, os.ModePerm)
	}
	files, err := os.ReadDir(path)
	if err != nil {
		common.Log.Error("Error reading directory:", err)
		return
	}

	// 按照文件的时间戳进行排序（降序）
	sort.Slice(files, func(i, j int) bool {
		infoI, _ := files[i].Info()
		infoJ, _ := files[j].Info()
		return infoI.ModTime().After(infoJ.ModTime())
	})

	// 删除多余的文件，保留最新的
	for i := maxFiles; i < len(files); i++ {
		name := filepath.Join(path, files[i].Name())
		if err := os.Remove(name); err != nil {
			common.Log.Error("Error removing old file:", err)
		} else {
			common.Log.Info("Removed old profile file: %s", name)
		}
	}
}
