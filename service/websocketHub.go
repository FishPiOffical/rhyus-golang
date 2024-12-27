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
	masterPool       *common.WorkerPool   // 主节点协程池
	masters          sync.Map             // 存储所有连接的服务端
	MasterNode       chan *websocket.Conn // 主节点连接的通道
	MasterUnregister chan *websocket.Conn // 断开连接的通道

	clientPool       *common.WorkerPool   // 客户端协程池
	clients          sync.Map             // 存储所有连接的客户端
	ClientNode       chan *websocket.Conn // 新客户端连接的通道
	ClientUnregister chan *websocket.Conn // 断开连接的通道
	AllOnlineUsers   string               // 所有在线用户

	localOnlineUsernames map[string]int // 本地在线用户名
	mu                   sync.Mutex     // 保护 localOnlineUsernames 的并发安全
}

type activeMaster struct {
	MessageInChan  chan []byte
	MessageOutChan chan []byte
}

type activeClient struct {
	UserInfo       *model.UserInfo
	MessageOutChan chan Message
	LastActive     time.Time
}

func init() {
	Hub = &webSocketHub{
		masterPool:           common.Pool.NewWorkerPool(32),                             // 主节点协程池
		masters:              sync.Map{},                                                // key:*websocket.Conn value:*activeMaster
		MasterNode:           make(chan *websocket.Conn, conf.Conf.MasterNodeCacheSize), // key:*websocket.Conn value:chan []byte
		MasterUnregister:     make(chan *websocket.Conn, conf.Conf.MasterNodeCacheSize), // 断开连接
		clientPool:           common.Pool.NewWorkerPool(conf.Conf.ClientPoolSize),       // 客户端协程池
		clients:              sync.Map{},                                                // key:*websocket.Conn value:*activeClient
		ClientNode:           make(chan *websocket.Conn, conf.Conf.ClientNodeCacheSize), // 新连接
		ClientUnregister:     make(chan *websocket.Conn, conf.Conf.ClientNodeCacheSize), // 断开连接
		AllOnlineUsers:       "{}",                                                      // 所有在线用户
		localOnlineUsernames: make(map[string]int, conf.Conf.ClientNodeCacheSize),       // key:username value:true
	}

	Hub.masterPool.Start()
	Hub.clientPool.Start()

	Hub.masterPool.AddTask(Hub.MasterUnregisterHandler)
	Hub.masterPool.AddTask(Hub.masterHandler)
	Hub.masterPool.AddTask(Hub.ClientUnregisterHandler)
	Hub.masterPool.AddTask(Hub.clientHandler)
}

func (h *webSocketHub) MasterUnregisterHandler() {
	for conn := range h.MasterUnregister {
		master, ok := h.masters.LoadAndDelete(conn)
		if ok {
			common.Log.Info("master has leaved: %s", conn.RemoteAddr().String())
			close(master.(*activeMaster).MessageInChan)
			close(master.(*activeMaster).MessageOutChan)
			err := conn.Close()
			if err != nil {
				common.Log.Error("close conn failed: %s", err)
			}
		}
	}
}

func (h *webSocketHub) ClientUnregisterHandler() {
	for conn := range h.ClientUnregister {
		client, ok := h.clients.LoadAndDelete(conn)
		userInfo := client.(*activeClient).UserInfo
		if ok {
			common.Log.Info("client %s has leaved: %s", userInfo.UserName, conn.RemoteAddr().String())
			h.mu.Lock()
			count := h.localOnlineUsernames[userInfo.UserName]
			if count < 1 {
				h.masterPool.AddTask(func() {
					util.PostMessageToMaster(conf.Conf.AdminKey, "leave", userInfo.UserName)
				})
			} else {
				h.localOnlineUsernames[userInfo.UserName] = count - 1
			}
			h.mu.Unlock()
		} else {
			common.Log.Info("client %s leave failed: %s", userInfo.UserName, conn.RemoteAddr().String())
		}
		close(client.(*activeClient).MessageOutChan)
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
			common.Log.Info("master has joined: %s", conn.RemoteAddr().String())
			master := master.(*activeMaster)
			h.masterPool.AddTask(func() {
				h.listenMaterMessage(conn, master)
			})
			h.masterPool.AddTask(func() {
				h.sendMessageToMaster(conn, master)
			})
			h.masterPool.AddTask(func() {
				h.handleMasterMessage(conn, master)
			})
		} else {
			common.Log.Error("master join failed: %s", conn.RemoteAddr().String())
		}
	}
}

func (h *webSocketHub) clientHandler() {
	for conn := range h.ClientNode {
		client, ok := h.clients.Load(conn)
		userInfo := client.(*activeClient).UserInfo
		if ok {
			common.Log.Info("client %s has joined: %s", userInfo.UserName, conn.RemoteAddr().String())
			h.clientPool.AddTask(func() {
				h.mu.Lock()
				count := h.localOnlineUsernames[userInfo.UserName]
				if count < 1 {
					util.PostMessageToMaster(conf.Conf.AdminKey, "join", userInfo.UserName)
				}
				h.localOnlineUsernames[userInfo.UserName] = count + 1
				h.mu.Unlock()
			})
			h.clientPool.AddTask(func() {
				for message := range client.(*activeClient).MessageOutChan {
					if message.Delay > 0 {
						time.Sleep(message.Delay)
					}
					err := conn.WriteMessage(websocket.TextMessage, message.Data)
					if err != nil {
						h.ClientUnregister <- conn
						return
					}
				}
			})
		} else {
			common.Log.Error("client %s join failed: %s", userInfo.UserName, conn.RemoteAddr().String())
			h.ClientUnregister <- conn
		}
	}
}

func (h *webSocketHub) listenMaterMessage(conn *websocket.Conn, master *activeMaster) {
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			h.MasterUnregister <- conn
			return
		}

		switch messageType {
		case websocket.TextMessage:
			common.Log.Info(" <--- master %s: %s", conn.RemoteAddr().String(), string(p))
			master.MessageInChan <- p
		default:
			common.Log.Info("read message unknown type: %d", messageType)
		}
	}
}

func (h *webSocketHub) handleMasterMessage(connMaster *websocket.Conn, master *activeMaster) {
	for message := range master.MessageInChan {
		h.masterPool.AddTask(func() {
			msg := string(message)
			if strings.Contains(msg, ":::") {
				split := strings.Split(msg, ":::")
				if len(split) == 2 {
					if split[0] == conf.Conf.AdminKey {
						command := split[1]
						if command == "hello" {
							common.Log.Info("[%s] from master %s", command, connMaster.RemoteAddr().String())
							master.MessageOutChan <- []byte("hello from rhyus-golang")
						} else if strings.HasPrefix(command, "tell") {
							// 发送文本给指定用户
							to := strings.Split(command, " ")[1]
							content := strings.ReplaceAll(command, "tell "+to+" ", "")
							common.Log.Info("[tell] to %s: %s", to, content)
							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								if client.UserInfo.UserName == to {
									client.MessageOutChan <- Message{Data: []byte(content)}
								}
								return true
							})
						} else if strings.HasPrefix(command, "msg") {
							// 广播文本：指定发送者先收到消息
							sender := strings.Split(command, " ")[1]
							content := strings.ReplaceAll(command, "msg "+sender+" ", "")
							common.Log.Info("[%s] %s: %s", command, sender, content)
							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								if client.UserInfo.UserName == sender {
									client.MessageOutChan <- Message{Data: []byte(content)}
								}
								return true
							})

							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								if client.UserInfo.UserName != sender {
									client.MessageOutChan <- Message{Data: []byte(content), Delay: 10 * time.Millisecond}
								}
								return true
							})
						} else if strings.HasPrefix(command, "all") {
							// 广播文本：直接广播，所有人按顺序收到消息
							content := strings.ReplaceAll(command, "all ", "")
							common.Log.Info("[%S]: %s", command, content)
							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								client.MessageOutChan <- Message{Data: []byte(content), Delay: 10 * time.Millisecond}
								return true
							})
						} else if strings.HasPrefix(command, "slow") {
							// 广播文本：慢广播，慢速发送，但所有人都能收到
							content := strings.ReplaceAll(command, "slow ", "")
							common.Log.Info("[%s]: %s", command, content)
							h.clients.Range(func(key, value any) bool {
								client := value.(*activeClient)
								client.MessageOutChan <- Message{Data: []byte(content), Delay: 100 * time.Millisecond}
								return true
							})
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
							common.Log.Info("[%s]: number %d list %s", command, len(onlineUsers), string(result))
							if len(onlineUsers) == 0 {
								result = []byte("[]")
							}
							master.MessageOutChan <- []byte(result)
						} else if strings.HasPrefix(command, "push") {
							content := strings.ReplaceAll(command, "push ", "")
							common.Log.Info("[%s]: %s", command, content)
							h.AllOnlineUsers = content
							master.MessageOutChan <- []byte("OK")
						} else if strings.HasPrefix(command, "kick") {
							userName := strings.ReplaceAll(command, "kick ", "")
							common.Log.Info("[%s]: %s", command, userName)
							h.clients.Range(func(key, value any) bool {
								conn := key.(*websocket.Conn)
								client := value.(*activeClient)
								if client.UserInfo.UserName == userName {
									h.ClientUnregister <- conn
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
									h.ClientUnregister <- conn
								}
								return true
							})

							result, err := json.Marshal(data)
							if err != nil {
								common.Log.Error("marshal clear result failed: %s", err)
							}
							common.Log.Info("[%s]: number %d list %s", command, len(data), string(result))
							master.MessageOutChan <- result
						}
					}
				}
			}
		})
	}
}

func (h *webSocketHub) sendMessageToMaster(conn *websocket.Conn, master *activeMaster) {
	for message := range master.MessageOutChan {
		common.Log.Info(" ---> master %s: %s", conn.RemoteAddr().String(), string(message))
		err := conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			common.Log.Info("send message to master %s failed: %s", conn.RemoteAddr().String(), err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				common.Log.Info("conn %s closed by %s", conn.RemoteAddr().String(), err)
			} else {
				common.Log.Info("read message failed: %s", err)
			}
			err := conn.Close()
			close(master.MessageOutChan)
			close(master.MessageInChan)
			if err != nil {
				common.Log.Error("close conn failed: %s", err)
				return
			}
			return
		}
	}
}

func (h *webSocketHub) AddMaster(conn *websocket.Conn) {
	master := &activeMaster{
		MessageInChan:  make(chan []byte, conf.Conf.MasterMessageCacheSize),
		MessageOutChan: make(chan []byte, conf.Conf.MasterMessageCacheSize),
	}
	h.masters.LoadOrStore(conn, master)
}

func (h *webSocketHub) AddClient(conn *websocket.Conn, userInfo *model.UserInfo) *activeClient {
	client := &activeClient{
		UserInfo:       userInfo,
		MessageOutChan: make(chan Message, conf.Conf.ClientMessageCacheSize),
		LastActive:     time.Now(),
	}
	h.clients.LoadOrStore(conn, client)
	return client
}

type Message struct {
	Data  []byte
	Delay time.Duration
}
