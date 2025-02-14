package api

import (
	"github.com/gin-gonic/gin"
	"github.com/lesismal/nbio/nbhttp/websocket"
	"github.com/lesismal/nbio/timer"
	"log"
	"net/http"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"rhyus-golang/model"
	"rhyus-golang/service"
	"rhyus-golang/util"
)

// ChatroomWebSocket 将连接加入在线列表
func ChatroomWebSocket(c *gin.Context) {
	upgrade := websocket.NewUpgrader()
	upgrade.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	upgrade.KeepaliveTime = timer.TimeForever
	if util.GetApiKey(c) == conf.Conf.AdminKey {
		// 服务端连接
		upgrade.OnOpen(func(conn *websocket.Conn) {
			common.Log.Info("master has joined: %s", conn.RemoteAddr().String())
			service.Hub.MasterRegister(conn)
		})
		upgrade.OnClose(func(conn *websocket.Conn, err error) {
			service.Hub.MasterUnregister(conn)
		})
		upgrade.OnMessage(func(conn *websocket.Conn, messageType websocket.MessageType, data []byte) {
			if conn == nil {
				log.Println("WebSocket connection is closed or nil")
				return
			}
			common.Log.Info(" <--- master %s: %s", conn.RemoteAddr().String(), string(data))
			service.Hub.HandleMasterMessage(&service.Message{Conn: conn, Data: data})
		})
	} else {
		// 客户端连接
		upgrade.OnOpen(func(conn *websocket.Conn) {
			info, _ := c.Get("userInfo")
			userInfo := info.(*model.UserInfo)
			common.Log.Info("client %s has joined: %s", userInfo.UserName, conn.RemoteAddr().String())
			service.Hub.ClientRegister(conn, userInfo)
		})
		upgrade.OnClose(func(conn *websocket.Conn, err error) {
			service.Hub.ClientUnregister(conn)
		})
	}

	_, err := upgrade.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		common.Log.Error("upgrade websocket failed: %s", err)
		return
	}
}
