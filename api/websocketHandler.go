package api

import (
	"github.com/gin-gonic/gin"
	"github.com/lesismal/nbio/nbhttp/websocket"
	"net/http"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"rhyus-golang/model"
	"rhyus-golang/service"
	"rhyus-golang/util"
	"time"
)

var masterUpgrade *websocket.Upgrader
var clientUpgrade *websocket.Upgrader

func init() {
	masterUpgrade = websocket.NewUpgrader()
	masterUpgrade.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	clientUpgrade = websocket.NewUpgrader()
	clientUpgrade.CheckOrigin = func(r *http.Request) bool {
		return true
	}
}

// ChatroomWebSocket 将连接加入在线列表
func ChatroomWebSocket(c *gin.Context) {
	var err error
	if util.GetApiKey(c) == conf.Conf.AdminKey {
		// 服务端连接
		masterUpgrade.KeepaliveTime = time.Minute * time.Duration(conf.Conf.KeepaliveTime)
		masterUpgrade.OnOpen(func(conn *websocket.Conn) {
			common.Log.Info("master has joined: %s", conn.RemoteAddr().String())
			service.Hub.MasterRegister(conn)
		})
		masterUpgrade.OnClose(func(conn *websocket.Conn, err error) {
			service.Hub.MasterUnregister(conn)
		})
		masterUpgrade.OnMessage(func(conn *websocket.Conn, messageType websocket.MessageType, data []byte) {
			common.Log.Info(" <--- master %s: %s", conn.RemoteAddr().String(), string(data))
			service.Hub.HandleMasterMessage(&service.Message{Conn: conn, Data: data})
		})
		_, err = masterUpgrade.Upgrade(c.Writer, c.Request, nil)
	} else {
		// 客户端连接
		clientUpgrade.KeepaliveTime = time.Minute * time.Duration(conf.Conf.KeepaliveTime)
		clientUpgrade.OnOpen(func(conn *websocket.Conn) {
			info, _ := c.Get("userInfo")
			userInfo := info.(*model.UserInfo)
			common.Log.Info("client %s has joined: %s", userInfo.UserName, conn.RemoteAddr().String())
			service.Hub.ClientRegister(conn, userInfo)
		})
		clientUpgrade.OnMessage(func(conn *websocket.Conn, messageType websocket.MessageType, data []byte) {
			common.Log.Info(" <--- client %s %s: %s", service.Hub.GetUser(conn).UserName, conn.RemoteAddr().String(), string(data))
		})
		clientUpgrade.OnClose(func(conn *websocket.Conn, err error) {
			service.Hub.ClientUnregister(conn)
		})
		_, err = clientUpgrade.Upgrade(c.Writer, c.Request, nil)
	}

	if err != nil {
		common.Log.Error("upgrade websocket failed: %s", err)
		return
	}
}
