package service

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"rhyus-golang/model"
	"rhyus-golang/util"
	"strings"
	"sync"
	"time"
)

var Hub *webSocketHub

// webSocketHub WebSocket 连接池
type webSocketHub struct {
	basePool *common.SteadyWorkerPool // 基础协程池

	masterConnPool              *common.WorkerPool   // 服务端协程池
	masterInMessageHandlerPool  *common.WorkerPool   // 服务端入站消息处理协程池 <---
	masterOutMessageHandlerPool *common.WorkerPool   // 服务端出站消息处理协程池 --->
	masters                     sync.Map             // 存储所有连接的服务端
	masterInMessageChan         chan *Message        // 服务端消息通道 <---
	masterOutMessageChan        chan *Message        // 服务端消息通道 --->
	MasterNode                  chan *websocket.Conn // 主节点连接的通道
	masterUnregister            chan *websocket.Conn // 断开连接的通道

	clientConnPool              *common.WorkerPool   // 客户端协程池
	clientOutMessageHandlerPool *common.WorkerPool   // 客户端出站消息处理协程池 --->
	clients                     sync.Map             // 存储所有连接的客户端
	clientOutMessageChan        chan *Message        // 客户端消息通道 --->
	ClientNode                  chan *websocket.Conn // 新客户端连接的通道
	clientUnregister            chan *websocket.Conn // 断开连接的通道
	AllOnlineUsers              string               // 所有在线用户

	localOnlineUsernames map[string]int // 本地在线用户名
	mu                   sync.Mutex     // 保护 localOnlineUsernames 的并发安全
}

type activeMaster struct {
	Conn *websocket.Conn
}

type activeClient struct {
	Conn       *websocket.Conn
	UserInfo   *model.UserInfo
	LastActive time.Time
}

func init() {
	Hub = &webSocketHub{
		basePool: common.Pool.NewSteadyWorkerPool(16), // 主节点协程池

		masterConnPool:              common.Pool.NewWorkerPool(conf.Conf.MasterPoolSize),       // 客户端协程池
		masterInMessageHandlerPool:  common.Pool.NewWorkerPool(conf.Conf.MasterPoolSize * 2),   // 客户端消息处理协程池
		masterOutMessageHandlerPool: common.Pool.NewWorkerPool(conf.Conf.MasterPoolSize * 2),   // 客户端消息处理协程池
		masters:                     sync.Map{},                                                // key:*websocket.Conn value:*activeMaster
		masterInMessageChan:         make(chan *Message, conf.Conf.MasterMessageCacheSize),     // 服务端消息入站通道
		masterOutMessageChan:        make(chan *Message, conf.Conf.MasterMessageCacheSize),     // 服务端消息出站通道
		MasterNode:                  make(chan *websocket.Conn, conf.Conf.MasterNodeCacheSize), // 新连接
		masterUnregister:            make(chan *websocket.Conn, conf.Conf.MasterNodeCacheSize), // 断开连接

		clientConnPool:              common.Pool.NewWorkerPool(conf.Conf.ClientPoolSize),               // 客户端协程池
		clientOutMessageHandlerPool: common.Pool.NewWorkerPool(conf.Conf.ClientMessageHandlerPoolSize), // 客户端消息处理协程池
		clients:                     sync.Map{},                                                        // key:*websocket.Conn value:*activeClient
		clientOutMessageChan:        make(chan *Message, conf.Conf.ClientMessageCacheSize),             // 客户端消息通道
		ClientNode:                  make(chan *websocket.Conn, conf.Conf.ClientNodeCacheSize),         // 新连接
		clientUnregister:            make(chan *websocket.Conn, conf.Conf.ClientNodeCacheSize),         // 断开连接

		AllOnlineUsers:       "{}",                                                // 所有在线用户
		localOnlineUsernames: make(map[string]int, conf.Conf.ClientNodeCacheSize), // key:username value:true
	}

	Hub.masterConnPool.Start()
	Hub.masterInMessageHandlerPool.Start()
	Hub.masterOutMessageHandlerPool.Start()
	Hub.clientConnPool.Start()
	Hub.clientOutMessageHandlerPool.Start()

	Hub.basePool.AddTask(Hub.heartbeat)
	Hub.basePool.AddTasks(Hub.sendMessageToClient, conf.Conf.ClientPoolSize>>6+2)
	Hub.basePool.AddTasks(Hub.MasterUnregisterHandler, conf.Conf.MasterPoolSize>>6+2)
	Hub.basePool.AddTasks(Hub.masterHandler, conf.Conf.MasterPoolSize)
	Hub.basePool.AddTasks(Hub.ClientUnregisterHandler, conf.Conf.ClientPoolSize>>6+2)
	Hub.basePool.AddTasks(Hub.clientHandler, conf.Conf.ClientPoolSize)

	Hub.masterOutMessageHandlerPool.AddTasks(Hub.sendMessageToMaster, conf.Conf.MasterPoolSize>>6+2)
	Hub.masterInMessageHandlerPool.AddTasks(Hub.handleMasterMessage, conf.Conf.MasterPoolSize>>6+2)

}

func (h *webSocketHub) heartbeat() {
	ticker := time.NewTicker(time.Duration(conf.Conf.Heartbeat) * time.Second)
	defer ticker.Stop()
	masterNum := 0
	clientNum := 0
	for _ = range ticker.C {
		masterNum = 0
		h.masters.Range(func(key, value any) bool {
			conn := key.(*websocket.Conn)
			h.clientOutMessageChan <- &Message{Conn: conn, Data: []byte("{\"ping\":\"pong\"}")}
			masterNum++
			return true
		})
		clientNum = 0
		h.clients.Range(func(key, value any) bool {
			conn := key.(*websocket.Conn)
			h.clientOutMessageChan <- &Message{Conn: conn, Data: []byte("{\"ping\":\"pong\"}")}
			clientNum++
			return true
		})
		common.Log.Info("active master: %d active client: %d", masterNum, clientNum)
	}
}

func (h *webSocketHub) MasterUnregisterHandler() {
	for conn := range h.masterUnregister {
		_, ok := h.masters.LoadAndDelete(conn)
		if ok {
			common.Log.Info("master has leaved: %s", conn.RemoteAddr().String())
		}
		err := conn.Close()
		if err != nil {
			common.Log.Error("close conn failed: %s", err)
		}
	}
}

func (h *webSocketHub) ClientUnregisterHandler() {
	for conn := range h.clientUnregister {
		client, ok := h.clients.LoadAndDelete(conn)
		if ok {
			userInfo := client.(*activeClient).UserInfo
			common.Log.Info("client %s has leaved: %s", userInfo.UserName, conn.RemoteAddr().String())
			h.mu.Lock()
			count := h.localOnlineUsernames[userInfo.UserName]
			if count < 1 {
				h.masterOutMessageHandlerPool.AddTask(func() {
					util.PostMessageToMaster(conf.Conf.AdminKey, "leave", userInfo.UserName)
				})
			} else {
				h.localOnlineUsernames[userInfo.UserName] = count - 1
			}
			h.mu.Unlock()
		}
		err := conn.Close()
		if err != nil {
			common.Log.Error("close conn failed: %s", err)
		}
	}
}

func (h *webSocketHub) masterHandler() {
	for conn := range h.MasterNode {
		master, ok := h.masters.Load(conn)
		if ok {
			_ = master.(*activeMaster)
			common.Log.Info("master has joined: %s", conn.RemoteAddr().String())
			h.masterConnPool.AddTask(func() {
				h.listenMaterMessage(conn)
			})
		} else {
			common.Log.Error("master join failed: %s", conn.RemoteAddr().String())
		}
	}
}

func (h *webSocketHub) clientHandler() {
	for conn := range h.ClientNode {
		// 延迟发送 AllOnlineUsers 列表
		h.clientOutMessageChan <- &Message{Conn: conn, Data: []byte(h.AllOnlineUsers), Delay: 2 * time.Second}
		client, ok := h.clients.Load(conn)
		if ok {
			userInfo := client.(*activeClient).UserInfo
			common.Log.Info("client %s has joined: %s", userInfo.UserName, conn.RemoteAddr().String())
			count := h.localOnlineUsernames[userInfo.UserName]
			h.mu.Lock()
			if count < 1 {
				h.masterOutMessageHandlerPool.AddTask(func() {
					util.PostMessageToMaster(conf.Conf.AdminKey, "join", userInfo.UserName)
				})
			}
			h.localOnlineUsernames[userInfo.UserName] = count + 1
			h.mu.Unlock()
		}
	}
}

func (h *webSocketHub) sendMessageToClient() {
	for message := range h.clientOutMessageChan {
		h.clientOutMessageHandlerPool.AddTask(func() {
			if message.Delay > 0 {
				time.Sleep(message.Delay)
			}
			//common.Log.Info(" ---> client %s: %s", message.Conn.RemoteAddr().String(), string(message.Data))
			err := message.Conn.WriteMessage(websocket.TextMessage, message.Data)
			if err != nil {
				h.clientUnregister <- message.Conn
				return
			}
		})
	}
}

func (h *webSocketHub) listenMaterMessage(conn *websocket.Conn) {
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			h.masterUnregister <- conn
			return
		}

		switch messageType {
		case websocket.TextMessage:
			common.Log.Info(" <--- master %s: %s", conn.RemoteAddr().String(), string(p))
			h.masterInMessageChan <- &Message{Conn: conn, Data: p}
		default:
			common.Log.Info("read message unknown type: %d", messageType)
		}
	}
}

func (h *webSocketHub) handleMasterMessage() {
	for message := range h.masterInMessageChan {
		h.masterInMessageHandlerPool.AddTask(func() {
			msg := string(message.Data)
			if strings.Contains(msg, ":::") {
				split := strings.Split(msg, ":::")
				if len(split) == 2 {
					if split[0] == conf.Conf.AdminKey {
						command := split[1]
						if command == "hello" {
							common.Log.Info("[hello] from master %s", message.Conn.RemoteAddr().String())
							h.masterOutMessageChan <- &Message{Conn: message.Conn, Data: []byte("hello from rhyus-golang")}
						} else if strings.HasPrefix(command, "tell") {
							// 发送文本给指定用户
							to := strings.Split(command, " ")[1]
							content := strings.ReplaceAll(command, "tell "+to+" ", "")
							common.Log.Info("[tell] to %s: %s", to, content)
							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								if client.UserInfo.UserName == to {
									h.clientOutMessageChan <- &Message{Conn: client.Conn, Data: []byte(content)}
								}
								return true
							})
						} else if strings.HasPrefix(command, "msg") {
							// 广播文本：指定发送者先收到消息
							sender := strings.Split(command, " ")[1]
							content := strings.ReplaceAll(command, "msg "+sender+" ", "")
							num := 0
							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								if client.UserInfo.UserName == sender {
									h.clientOutMessageChan <- &Message{Conn: client.Conn, Data: []byte(content)}
									num++
								}
								return true
							})

							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								if client.UserInfo.UserName != sender {
									h.clientOutMessageChan <- &Message{Conn: client.Conn, Data: []byte(content), Delay: 10 * time.Millisecond}
									num++
								}
								return true
							})
							common.Log.Info("[msg] --> %d client %s first receive: %s", num, sender, content)
						} else if strings.HasPrefix(command, "all") {
							// 广播文本：直接广播，所有人按顺序收到消息
							content := strings.ReplaceAll(command, "all ", "")
							num := 0
							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								h.clientOutMessageChan <- &Message{Conn: client.Conn, Data: []byte(content), Delay: 10 * time.Millisecond}
								num++
								return true
							})
							common.Log.Info("[all]: --> %d clients %s", num, content)
						} else if strings.HasPrefix(command, "slow") {
							// 广播文本：慢广播，慢速发送，但所有人都能收到
							content := strings.ReplaceAll(command, "slow ", "")
							num := 0
							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								h.clientOutMessageChan <- &Message{Conn: client.Conn, Data: []byte(content), Delay: 100 * time.Millisecond}
								num++
								return true
							})
							common.Log.Info("[slow] --> %d clients : %s", num, content)
						} else if command == "online" {
							onlineUsersSet := make(map[string]*model.UserInfo)
							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								onlineUsersSet[client.UserInfo.OId] = client.UserInfo
								return true
							})
							onlineUsers := make([]*model.UserInfo, 0)
							for _, userInfo := range onlineUsersSet {
								onlineUsers = append(onlineUsers, userInfo)
							}

							result, err := json.Marshal(onlineUsers)
							if err != nil {
								common.Log.Error("marshal online users failed: %s", err)
							}
							common.Log.Info("[online]: number %d list %s", len(onlineUsers), string(result))
							if len(onlineUsers) == 0 {
								result = []byte("[]")
							}
							h.masterOutMessageChan <- &Message{Conn: message.Conn, Data: result}
						} else if strings.HasPrefix(command, "push") {
							content := strings.ReplaceAll(command, "push ", "")
							common.Log.Info("[push]: %s", content)
							h.AllOnlineUsers = content
							h.masterOutMessageChan <- &Message{Conn: message.Conn, Data: []byte("OK")}
						} else if strings.HasPrefix(command, "kick") {
							userName := strings.ReplaceAll(command, "kick ", "")
							common.Log.Info("[kick]: %s", userName)
							h.clients.Range(func(key, value any) bool {
								conn := key.(*websocket.Conn)
								client := value.(*activeClient)
								if client.UserInfo.UserName == userName {
									h.clientUnregister <- conn
								}
								return true
							})
						} else if command == "clear" {
							data := make(map[string]int)
							h.clients.Range(func(key, value any) bool {
								conn := key.(*websocket.Conn)
								client := value.(*activeClient)
								if client.LastActive.Add(time.Hour * 6).Before(time.Now()) {
									common.Log.Info("clear: %s", client.UserInfo.UserName)
									data[client.UserInfo.UserName] = time.Now().Second() - client.LastActive.Second()
									h.clientUnregister <- conn
								}
								return true
							})

							result, err := json.Marshal(data)
							if err != nil {
								common.Log.Error("marshal clear result failed: %s", err)
							}
							common.Log.Info("[clear]: number %d list %s", len(data), string(result))
							h.masterOutMessageChan <- &Message{Conn: message.Conn, Data: result}
						}
					}
				}
			}
		})
	}
}

func (h *webSocketHub) sendMessageToMaster() {
	for message := range h.masterOutMessageChan {
		common.Log.Info(" ---> master %s: %s", message.Conn.RemoteAddr().String(), string(message.Data))
		err := message.Conn.WriteMessage(websocket.TextMessage, message.Data)
		if err != nil {
			h.masterUnregister <- message.Conn
			return
		}
	}
}

func (h *webSocketHub) AddMaster(conn *websocket.Conn) {
	master := &activeMaster{
		Conn: conn,
	}
	h.masters.LoadOrStore(conn, master)
}

func (h *webSocketHub) AddClient(conn *websocket.Conn, userInfo *model.UserInfo) *activeClient {
	client := &activeClient{
		Conn:       conn,
		UserInfo:   userInfo,
		LastActive: time.Now(),
	}
	h.clients.LoadOrStore(conn, client)
	return client
}

type Message struct {
	Conn  *websocket.Conn
	Data  []byte
	Delay time.Duration
}
