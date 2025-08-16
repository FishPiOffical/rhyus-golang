package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/lesismal/nbio/nbhttp"
	"os"
	"os/signal"
	"rhyus-golang/api"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"rhyus-golang/middlewares"
	"rhyus-golang/service"
	"rhyus-golang/util"
	"strconv"
	"time"
)

func main() {

	conf.InitConfig()
	// service.NewTimeWheel().Start()
	util.InitLimiter()
	service.InitTasks()

	service.StartPProfServe()

	gin.SetMode(gin.ReleaseMode)
	ginServer := gin.New()
	ginServer.UseH2C = true
	ginServer.MaxMultipartMemory = 1024 * 1024 * 32 // 表示处理上传的文件时，最多将32MB的数据保存在内存中，超出部分会保存到临时文件中。这样可以避免大文件上传时占用过多内存。
	ginServer.Use(
		middlewares.Recover,
		middlewares.Logging,
		middlewares.CorsMiddleware,
		middlewares.Limiter,
		middlewares.Authorize,
	)

	api.ServeAPI(ginServer)

	addr := conf.Conf.Server.Host + ":" + strconv.Itoa(conf.Conf.Server.Port)
	engine := nbhttp.NewEngine(nbhttp.Config{
		Network: "tcp",
		Addrs:   []string{addr},
		Handler: ginServer,
	})
	common.Log.Info("server start at: %s", addr)
	err := engine.Start()
	if err != nil {
		common.Log.Fatal(common.ExitCodeUnavailablePort, "serve start failed: %s", err)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	engine.Shutdown(ctx)

}
