package api

import (
	"github.com/gin-gonic/gin"
	"github.com/lesismal/nbio/nbhttp/websocket"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"rhyus-golang/model"
	"rhyus-golang/service"
	"rhyus-golang/util"
)

// websocket 升级并跨域
var Upgrade = websocket.NewUpgrader()

// ChatroomWebSocket 将连接加入在线列表
func ChatroomWebSocket(c *gin.Context) {
	var (
		err  error
		conn *websocket.Conn
	)

	conn, err = Upgrade.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		common.Log.Error("upgrade websocket failed: %s", err)
		return
	}

	if util.GetApiKey(c) == conf.Conf.AdminKey {
		// 服务端连接
		service.Hub.AddMaster(conn)
	} else {
		// 客户端连接
		userInfo, _ := c.Get("userInfo")
		service.Hub.AddClient(conn, userInfo.(*model.UserInfo))
	}
}
