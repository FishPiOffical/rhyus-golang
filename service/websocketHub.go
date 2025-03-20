package service

import (
	"context"
	"encoding/json"
	"github.com/lesismal/nbio/nbhttp/websocket"
	"rhyus-golang/common"
	sync2 "rhyus-golang/common/sync"
	"rhyus-golang/conf"
	"rhyus-golang/model"
	"rhyus-golang/util"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var Hub *webSocketHub

// webSocketHub WebSocket 连接池
type webSocketHub struct {
	masters              sync.Map
	masterNum            int64
	clients              sync.Map
	clientNum            int64
	AllOnlineUsers       string           // 所有在线用户
	localOnlineUsernames map[string]int64 // 本地在线用户名
	mu                   sync.Mutex       // 保护 localOnlineUsernames 的并发安全
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
		masters:              sync.Map{}, // key:*websocket.Conn value:*activeMaster
		masterNum:            0,
		clients:              sync.Map{}, // key:*websocket.Conn value:*activeClient
		clientNum:            0,
		AllOnlineUsers:       "{}",                   // 所有在线用户
		localOnlineUsernames: make(map[string]int64), // key:username value:true
	}

	pool := sync2.NewPool(context.Background())
	pool.SubmitTask("heartbeat", func(ctx context.Context) (err error) {
		Hub.heartbeat()
		return nil
	})
}

func (h *webSocketHub) heartbeat() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		//h.masters.Range(func(key, value any) bool {
		//	master := value.(*activeMaster)
		//	h.sendMessage(&Message{Conn: master.Conn, Data: []byte("heartbeat")})
		//	return true
		//})
		//h.clients.Range(func(key, value any) bool {
		//	client := value.(*activeClient)
		//	h.sendMessage(&Message{Conn: client.Conn, Data: []byte("heartbeat")})
		//	return true
		//})
		common.Log.Info("active master: %d active client: %d", atomic.LoadInt64(&h.masterNum), atomic.LoadInt64(&h.clientNum))
	}
}

type Message struct {
	Conn  *websocket.Conn
	Data  []byte
	Delay time.Duration
}

func (h *webSocketHub) MasterRegister(conn *websocket.Conn) {
	common.Log.Info("master has joined: %s", conn.RemoteAddr().String())
	master := &activeMaster{
		Conn: conn,
	}
	h.masters.LoadOrStore(conn, master)
	atomic.AddInt64(&h.masterNum, 1)
}

func (h *webSocketHub) GetUser(conn *websocket.Conn) *model.UserInfo {
	userInfo, ok := h.clients.Load(conn)
	if ok {
		return userInfo.(*activeClient).UserInfo
	}
	return nil
}

func (h *webSocketHub) ClientRegister(conn *websocket.Conn, userInfo *model.UserInfo) {
	client := &activeClient{
		Conn:       conn,
		UserInfo:   userInfo,
		LastActive: time.Now(),
	}
	h.clients.LoadOrStore(conn, client)
	common.Log.Info("client %s has joined: %s", userInfo.UserName, conn.RemoteAddr().String())
	h.mu.Lock()
	count := h.localOnlineUsernames[userInfo.UserName]
	h.localOnlineUsernames[userInfo.UserName] = count + 1
	h.mu.Unlock()
	atomic.AddInt64(&h.clientNum, 1)

	if count < 1 {
		util.PostMessageToMaster(conf.Conf.AdminKey, "join", userInfo.UserName)
	}
	// 用户连接数过多则关闭最早的连接
	if count > conf.Conf.SessionMaxConnection {
		common.Log.Info("client %s has too many connections: %d", userInfo.UserName, count)
		firstClient := client
		h.clients.Range(func(key, value any) bool {
			client := value.(*activeClient)
			if client.UserInfo.UserName == userInfo.UserName {
				if client.LastActive.Before(firstClient.LastActive) {
					firstClient = client
				}
			}
			return true
		})
		h.ClientUnregister(firstClient.Conn)
		return
	}
	// 延迟发送 AllOnlineUsers 列表
	h.sendMessage(&Message{Conn: conn, Data: []byte(h.AllOnlineUsers), Delay: 2 * time.Second})
}

func (h *webSocketHub) MasterUnregister(conn *websocket.Conn) {
	_, ok := h.masters.LoadAndDelete(conn)
	if ok {
		common.Log.Info("master has leaved: %s", conn.RemoteAddr().String())
		atomic.AddInt64(&h.masterNum, -1)
	}
	err := conn.Close()
	if err != nil {
		common.Log.Error("close conn failed: %s", err)
		return
	}
}

func (h *webSocketHub) ClientUnregister(conn *websocket.Conn) {
	client, ok := h.clients.LoadAndDelete(conn)
	if ok {
		c := client.(*activeClient)
		userInfo := c.UserInfo
		common.Log.Info("client %s has leaved: %s loginTime: %s", userInfo.UserName, conn.RemoteAddr().String(), c.LastActive.Format("2006-01-02 15:04:05"))
		h.mu.Lock()
		count := h.localOnlineUsernames[userInfo.UserName]
		h.localOnlineUsernames[userInfo.UserName] = count - 1
		h.mu.Unlock()
		atomic.AddInt64(&h.clientNum, -1)
		if count < 1 {
			util.PostMessageToMaster(conf.Conf.AdminKey, "leave", userInfo.UserName)
		}
	}
	err := conn.Close()
	if err != nil {
		common.Log.Error("close conn failed: %s", err)
		return
	}
}

func (h *webSocketHub) sendMessage(message *Message) {
	time.AfterFunc(message.Delay, func() {
		err := message.Conn.WriteMessage(websocket.TextMessage, message.Data)
		if err != nil {
			h.ClientUnregister(message.Conn)
			return
		}
	})
}

func (h *webSocketHub) HandleMasterMessage(message *Message) {
	msg := string(message.Data)
	if strings.Contains(msg, ":::") {
		split := strings.Split(msg, ":::")
		if len(split) == 2 && split[0] == conf.Conf.AdminKey {
			command := split[1]
			if command == "hello" {
				common.Log.Info("[hello] from master %s", message.Conn.RemoteAddr().String())
				h.sendMessage(&Message{Conn: message.Conn, Data: []byte("hello from rhyus-golang")})
			} else if strings.HasPrefix(command, "tell") {
				// 发送文本给指定用户
				to := strings.Split(command, " ")[1]
				content := strings.ReplaceAll(command, "tell "+to+" ", "")
				common.Log.Info("[tell] to %s: %s", to, content)
				h.clients.Range(func(key, value any) bool {
					client := value.(*activeClient)
					if client.UserInfo.UserName == to {
						h.sendMessage(&Message{Conn: client.Conn, Data: []byte(content)})
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
						h.sendMessage(&Message{Conn: client.Conn, Data: []byte(content)})
						num++
					}
					return true
				})

				h.clients.Range(func(key, value any) bool {
					client := value.(*activeClient)
					if client.UserInfo.UserName != sender {
						h.sendMessage(&Message{Conn: client.Conn, Data: []byte(content), Delay: 10 * time.Millisecond})
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
					h.sendMessage(&Message{Conn: client.Conn, Data: []byte(content), Delay: 10 * time.Millisecond})
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
					h.sendMessage(&Message{Conn: client.Conn, Data: []byte(content), Delay: 100 * time.Millisecond})
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
				common.Log.Info(" ---> master %s: %s", message.Conn.RemoteAddr().String(), string(message.Data))
				h.sendMessage(&Message{Conn: message.Conn, Data: result})
			} else if strings.HasPrefix(command, "push") {
				content := strings.ReplaceAll(command, "push ", "")
				common.Log.Info("[push]: %s", content)
				h.AllOnlineUsers = content
				common.Log.Info(" ---> master %s: %s", message.Conn.RemoteAddr().String(), string(message.Data))
				h.sendMessage(&Message{Conn: message.Conn, Data: []byte("OK")})
			} else if strings.HasPrefix(command, "kick") {
				userName := strings.ReplaceAll(command, "kick ", "")
				common.Log.Info("[kick]: %s", userName)
				h.clients.Range(func(key, value any) bool {
					conn := key.(*websocket.Conn)
					client := value.(*activeClient)
					if client.UserInfo.UserName == userName {
						h.ClientUnregister(conn)
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
						data[client.UserInfo.UserName] = time.Now().Hour() - client.LastActive.Hour()
						h.ClientUnregister(conn)
					}
					return true
				})

				result, err := json.Marshal(data)
				if err != nil {
					common.Log.Error("marshal clear result failed: %s", err)
				}
				common.Log.Info("[clear]: number %d list %s", len(data), string(result))
				common.Log.Info(" ---> master %s: %s", message.Conn.RemoteAddr().String(), result)
				h.sendMessage(&Message{Conn: message.Conn, Data: result})
			}
		}
	}
}
