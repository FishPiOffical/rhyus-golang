package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"rhyus-golang/model"
	"rhyus-golang/service"
	"rhyus-golang/util"
)

// websocket 升级并跨域
var (
	upgrade = &websocket.Upgrader{
		// 允许跨域
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

// ChatroomWebSocket 将连接加入在线列表
func ChatroomWebSocket(c *gin.Context) {
	var (
		err  error
		conn *websocket.Conn
	)

	conn, err = upgrade.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		common.Log.Error("upgrade websocket failed: %s", err)
		return
	}

	if util.GetApiKey(c) == conf.Conf.AdminKey {
		// 主节点连接
		common.Log.Info("master node connected: %s", conn.RemoteAddr().String())
		service.Hub.SetMasterNode(conn)
		service.Hub.HandleMasterNodeMessage()
	} else {
		// 客户端连接
		userInfo, _ := c.Get("userInfo")
		service.Hub.AddClient(conn, userInfo.(*model.UserInfo))
		service.Hub.Register <- conn

		err := conn.WriteMessage(websocket.TextMessage, []byte(service.Hub.AllOnlineUsers))
		if err != nil {
			// Connection abnormal closed
			service.Hub.Unregister <- conn
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				common.Log.Error("conn %s closed by %s", conn.RemoteAddr().String(), err)
			} else {
				common.Log.Error("send message to %s's client failed: %s", userInfo.(*model.UserInfo).UserName, err)
			}
		}

	}
}
