package service

import (
	"github.com/gorilla/websocket"
	"rhyus-golang/common"
	"rhyus-golang/model"
	"runtime"
	"sync"
	"time"
)

// WebSocketHub WebSocket 连接池
type WebSocketHub struct {
	clients    sync.Map             // 存储所有连接的客户端
	masterNode *websocket.Conn      // 主节点
	Register   chan *websocket.Conn // 新客户端连接的通道
	Unregister chan *websocket.Conn // 断开连接的通道
	broadcast  chan []byte          // 广播消息的通道
	mu         sync.Mutex           // 保护 clients 的并发安全
}

type activeUser struct {
	model.UserInfo
	LastActive time.Time
}

var Hub *WebSocketHub

func init() {
	Hub = &WebSocketHub{
		clients:    sync.Map{},                     // key:*websocket.Conn value:*activeUser
		broadcast:  make(chan []byte, 64),          // 广播消息
		Register:   make(chan *websocket.Conn, 64), // 新连接
		Unregister: make(chan *websocket.Conn, 64), // 断开连接
	}

	go registerHandler()
	go unregisterHandler()
}

func registerHandler() {
	conn := <-Hub.Register
	user, ok := Hub.clients.Load(conn)
	if ok {
		common.Log.Info("user %s register success: %s", user.(activeUser).UserName, conn.RemoteAddr().String())
		//go util.PostMessageToMaster()
	} else {
		common.Log.Error("user %s register failed: %s", user.(activeUser).UserName, conn.RemoteAddr().String())
	}
}

func unregisterHandler() {

}

func (h *WebSocketHub) SetMasterNode(conn *websocket.Conn) {
	h.masterNode = conn

	err := conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	if err != nil {
		common.Log.Error("set read deadline failed: %s", err)
		return
	}
	conn.SetPongHandler(func(appData string) error {
		err := conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		if err != nil {
			common.Log.Error("set read deadline failed: %s", err)
			return err
		}
		return nil
	})

	for {
		messageType, p, err := h.masterNode.ReadMessage()
		if err != nil {
			// Connection abnormal closed
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				common.Log.Error("conn abnormal closed by %s", err)
			} else {
				common.Log.Error("read message failed: %s", err)
			}
			return
		}

		switch messageType {
		case websocket.TextMessage:
			common.Log.Info("read message: %s", string(p))
			h.broadcast <- p
		default:
			common.Log.Info("read message unknown type: %d", messageType)
		}
	}
}

func (h *WebSocketHub) HandleMasterNodeMessage() {
	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU*10; i++ {
		go func() {
			for {
				select {
				case message := <-h.broadcast:
					h.clients.Range(func(key, value any) bool {
						if arr, ok := value.([]*websocket.Conn); ok {
							for _, conn := range arr {
								// Todo : 消息类型处理

								err := conn.WriteMessage(websocket.TextMessage, message)
								if err != nil {
									common.Log.Error("send message to %s's client failed: %s", key.(string), err)
								}
							}
						}
						return true
					})
				}
			}
		}()
	}
}

func (h *WebSocketHub) AddClient(conn *websocket.Conn, userIndo model.UserInfo) {
	user := &activeUser{
		UserInfo:   userIndo,
		LastActive: time.Time{},
	}
	h.clients.LoadOrStore(conn, user)
}
