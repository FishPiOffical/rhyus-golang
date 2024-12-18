package api

import (
	"github.com/gin-gonic/gin"
	"rhyus-golang/model"
)

func ServeAPI(ginServer *gin.Engine) {

	ginServer.GET("/api/v1/ping", func(c *gin.Context) {
		model.Success("pong")
	})

	//ginServer.GET("/", WebSocketBase)

}
