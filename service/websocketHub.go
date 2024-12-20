package service

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"rhyus-golang/model"
	"rhyus-golang/util"
	"runtime"
	"strings"
	"sync"
	"time"
)

// webSocketHub WebSocket 连接池
type webSocketHub struct {
	clients        sync.Map             // 存储所有连接的客户端
	masterNode     *websocket.Conn      // 主节点
	Register       chan *websocket.Conn // 新客户端连接的通道
	Unregister     chan *websocket.Conn // 断开连接的通道
	messageInChan  chan []byte          // 收到消息队列
	messageOutChan chan []byte          // 发送消息队列
	AllOnlineUsers string               // 所有在线用户

	mu sync.Mutex // 保护 clients 的并发安全
}

type activeUser struct {
	*model.UserInfo
	LastActive time.Time
}

var Hub *webSocketHub

func init() {
	Hub = &webSocketHub{
		clients:        sync.Map{},                     // key:*websocket.Conn value:*activeUser
		messageInChan:  make(chan []byte, 64),          // 收到消息队列
		messageOutChan: make(chan []byte, 64),          // 发送消息队列
		Register:       make(chan *websocket.Conn, 64), // 新连接
		Unregister:     make(chan *websocket.Conn, 64), // 断开连接
		AllOnlineUsers: "",                             // 所有在线用户
	}

	go registerHandler()
	go unregisterHandler()
	go sendMessageToMaster()

}

func registerHandler() {
	conn := <-Hub.Register
	user, ok := Hub.clients.Load(conn)
	if ok {
		common.Log.Info("user %s has joined: %s", user.(*activeUser).UserName, conn.RemoteAddr().String())
		go util.PostMessageToMaster(conf.Conf.AdminKey, "join", user.(*activeUser).UserName)
	} else {
		common.Log.Error("user %s join failed: %s", user.(*activeUser).UserName, conn.RemoteAddr().String())
	}
}

func unregisterHandler() {
	conn := <-Hub.Register
	user, ok := Hub.clients.LoadAndDelete(conn)
	if ok {
		common.Log.Info("user %s has leaved: %s", user.(*activeUser).UserName, conn.RemoteAddr().String())
		go util.PostMessageToMaster(conf.Conf.AdminKey, "leave", user.(*activeUser).UserName)
	} else {
		common.Log.Error("user %s leave failed: %s", user.(*activeUser).UserName, conn.RemoteAddr().String())
	}
	err := conn.Close()
	if err != nil {
		common.Log.Error("close conn failed: %s", err)
		return
	}
}

func sendMessageToMaster() {
	for {
		message := <-Hub.messageOutChan
		err := Hub.masterNode.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			// Connection abnormal closed
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				common.Log.Error("conn abnormal closed by %s", err)
			} else {
				common.Log.Error("send message to master node failed: %s", err)
			}
			return
		}
	}
}

func (h *webSocketHub) SetMasterNode(conn *websocket.Conn) {
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

	go func() {
		for {
			messageType, p, err := h.masterNode.ReadMessage()
			if err != nil {
				// Connection abnormal closed
				common.Log.Info("master node closed: %s", conn.RemoteAddr().String())
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
				h.messageInChan <- p
			default:
				common.Log.Info("read message unknown type: %d", messageType)
			}
		}
	}()
}

func (h *webSocketHub) HandleMasterNodeMessage() {
	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU*10; i++ {
		go func() {
			for {
				select {
				case message := <-h.messageInChan:
					msg := string(message)
					if strings.Contains(msg, ":::") {
						split := strings.Split(msg, ":::")
						if len(split) == 2 {
							if split[0] == conf.Conf.AdminKey {
								command := split[1]

								if command == "hello" {
									h.messageOutChan <- []byte("hello from rhyus-golang")
								} else if strings.HasPrefix(command, "tell") {
									// 发送文本给指定用户
									to := strings.Split(command, " ")[1]
									content := strings.ReplaceAll(command, "tell "+to+" ", "")
									common.Log.Info("tell %s: %s", to, content)
									h.clients.Range(func(key, value any) bool {
										conn := key.(*websocket.Conn)
										user := value.(*activeUser)
										if user.UserName == to {
											err := conn.WriteMessage(websocket.TextMessage, []byte(content))
											if err != nil {
												// Connection abnormal closed
												h.Unregister <- conn
												if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
													common.Log.Error("conn abnormal closed by %s", err)
												} else {
													common.Log.Error("send message to %s's client failed: %s", user.UserName, err)
												}
											}
										}
										return true
									})
								} else if strings.HasPrefix(command, "msg") {
									// 广播文本：指定发送者先收到消息
									sender := strings.Split(command, " ")[1]
									content := strings.ReplaceAll(command, "msg "+sender+" ", "")
									common.Log.Info("msg %s: %s", sender, content)
									h.clients.Range(func(key, value any) bool {
										conn := key.(*websocket.Conn)
										user := value.(*activeUser)
										if user.UserName == sender {
											err := conn.WriteMessage(websocket.TextMessage, []byte(content))
											if err != nil {
												// Connection abnormal closed
												h.Unregister <- conn
												if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
													common.Log.Error("conn abnormal closed by %s", err)
												} else {
													common.Log.Error("send message to %s's client failed: %s", user.UserName, err)
												}
											}
										}
										return true
									})

									h.clients.Range(func(key, value any) bool {
										conn := key.(*websocket.Conn)
										user := value.(*activeUser)
										if user.UserName != sender {
											err := conn.WriteMessage(websocket.TextMessage, []byte(content))
											if err != nil {
												// Connection abnormal closed
												h.Unregister <- conn
												if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
													common.Log.Error("conn abnormal closed by %s", err)
												} else {
													common.Log.Error("send message to %s's client failed: %s", user.UserName, err)
												}
											}
											time.Sleep(10 * time.Millisecond)
										}
										return true
									})
								} else if strings.HasPrefix(command, "all") {
									// 广播文本：直接广播，所有人按顺序收到消息
									content := strings.ReplaceAll(command, "all ", "")
									common.Log.Info("all: %s", content)
									h.clients.Range(func(key, value any) bool {
										conn := key.(*websocket.Conn)
										user := value.(*activeUser)
										err := conn.WriteMessage(websocket.TextMessage, []byte(content))
										if err != nil {
											// Connection abnormal closed
											h.Unregister <- conn
											if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
												common.Log.Error("conn abnormal closed by %s", err)
											} else {
												common.Log.Error("send message to %s's client failed: %s", user.UserName, err)
											}
										}
										time.Sleep(10 * time.Millisecond)
										return true
									})
								} else if strings.HasPrefix(command, "slow") {
									// 广播文本：慢广播，慢速发送，但所有人都能收到
									content := strings.ReplaceAll(command, "slow ", "")
									common.Log.Info("slow: %s", content)
									h.clients.Range(func(key, value any) bool {
										conn := key.(*websocket.Conn)
										user := value.(*activeUser)
										err := conn.WriteMessage(websocket.TextMessage, []byte(content))
										if err != nil {
											// Connection abnormal closed
											h.Unregister <- conn
											if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
												common.Log.Error("conn abnormal closed by %s", err)
											} else {
												common.Log.Error("send message to %s's client failed: %s", user.UserName, err)
											}
										}
										time.Sleep(100 * time.Millisecond)
										return true
									})
								} else if command == "online" {
									onlineUsersSet := make(map[*activeUser]struct{})
									h.clients.Range(func(key, value any) bool {
										user := value.(*activeUser)
										onlineUsersSet[user] = struct{}{}
										return true
									})
									onlineUsers := make([]*activeUser, len(onlineUsersSet))
									for user := range onlineUsersSet {
										onlineUsers = append(onlineUsers, user)
									}

									common.Log.Info("online: %d", len(onlineUsersSet))
									bytes, err := json.Marshal(onlineUsers)
									if err != nil {
										common.Log.Error("marshal online users failed: %s", err)
									}
									h.messageOutChan <- bytes
								} else if strings.HasPrefix(command, "push") {
									content := strings.ReplaceAll(command, "push ", "")
									common.Log.Info("push: %s", content)
									h.AllOnlineUsers = content
									h.messageOutChan <- []byte("OK")
								} else if strings.HasPrefix(command, "kick") {
									userName := strings.ReplaceAll(command, "kick ", "")
									common.Log.Info("kick: %s", userName)
									h.clients.Range(func(key, value any) bool {
										conn := key.(*websocket.Conn)
										user := value.(*activeUser)
										if user.UserName == userName {
											h.Unregister <- conn
										}
										return true
									})
								} else if command == "clear" {
									common.Log.Info("clear")
									data := make(map[string]int)
									h.clients.Range(func(key, value any) bool {
										conn := key.(*websocket.Conn)
										user := value.(*activeUser)
										if user.LastActive.Add(time.Hour * 6).Before(time.Now()) {
											common.Log.Info("clear: %s", user.UserName)
											data[user.UserName] = time.Now().Second() - user.LastActive.Second()
											h.Unregister <- conn
										}
										return true
									})

									result, err := json.Marshal(data)
									if err != nil {
										common.Log.Error("marshal clear result failed: %s", err)
									}
									h.messageOutChan <- []byte(result)
								}
							}
						}
					}
				}
			}
		}()
	}
}

func (h *webSocketHub) AddClient(conn *websocket.Conn, userInfo *model.UserInfo) {
	user := &activeUser{
		UserInfo:   userInfo,
		LastActive: time.Time{},
	}
	h.clients.LoadOrStore(conn, user)
}
