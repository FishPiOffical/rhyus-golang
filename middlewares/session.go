package middlewares

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"rhyus-golang/model"
	"rhyus-golang/util"
	"time"
)

// Logging 打印请求信息
func Logging(c *gin.Context) {
	start := time.Now()
	path := c.Request.URL.Path
	raw := c.Request.URL.RawQuery
	defer func() {
		end := time.Now()
		latency := end.Sub(start)

		if raw != "" {
			path = path + "?" + raw
		}

		common.Log.Info(" %d | %d | %s | %s | %s %s ", c.Writer.Status(), c.Writer.Size(), latency, c.ClientIP(), c.Request.Method, path)
	}()
	c.Next()
}

// Recover 异常处理、日志记录
func Recover(c *gin.Context) {
	defer func() {
		if e := recover(); nil != e {
			common.Log.RecoverError(e)
			model.Fail(c, fmt.Sprintf("%v", e))
		}
	}()
	c.Next()
}

// CorsMiddleware 配置跨域请求
func CorsMiddleware(c *gin.Context) {
	origin := c.GetHeader("Origin")
	c.Header("Access-Control-Allow-Origin", origin)
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Allow-Headers", "origin, Content-Length, Content-Type, Authorization")
	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
	c.Header("Access-Control-Allow-Private-Network", "true")

	if c.Request.Method == http.MethodOptions {
		c.Header("Access-Control-Max-Age", "600")
		c.AbortWithStatus(http.StatusNoContent)
		return
	}

	c.Next()
}

// Authorize 鉴权
func Authorize(c *gin.Context) {
	apiKey := util.GetApiKey(c)
	common.Log.Debug("apiKey: %s", apiKey)
	if apiKey == "" {
		model.Forbidden(c)
	} else if apiKey != conf.Conf.ApiKey {
		userInfo := util.GetUserInfo(apiKey)
		if userInfo == nil {
			model.Forbidden(c)
			return
		}
		c.Set("userInfo", userInfo)
	}

	c.Next()
}
