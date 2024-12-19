package api

import (
	"github.com/gin-gonic/gin"
)

func ServeAPI(ginServer *gin.Engine) {
	ginServer.GET("/", ChatroomWebSocket)
}
