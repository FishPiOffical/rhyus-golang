package service

import (
	"context"
	"encoding/json"
	"github.com/lesismal/nbio/nbhttp/websocket"
	cmap "github.com/orcaman/concurrent-map/v2"
	"golang.org/x/time/rate"
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
	fastQueue            chan *Message
	normalQueue          chan *Message
	slowQueue            chan *Message
	AllOnlineUsers       string                            // 所有在线用户
	localOnlineUsernames cmap.ConcurrentMap[string, int64] // 本地在线用户名
}

type activeMaster struct {
	Conn *websocket.Conn
}

type activeClient struct {
	Conn       *websocket.Conn
	UserInfo   *model.UserInfo
	LastActive time.Time
}

func InitTasks() {
	Hub = &webSocketHub{
		masters:              sync.Map{}, // key:*websocket.Conn value:*activeMaster
		masterNum:            0,
		clients:              sync.Map{}, // key:*websocket.Conn value:*activeClient
		clientNum:            0,
		fastQueue:            make(chan *Message, 1024),
		normalQueue:          make(chan *Message, 1024),
		slowQueue:            make(chan *Message, 1024),
		AllOnlineUsers:       "{}",              // 所有在线用户
		localOnlineUsernames: cmap.New[int64](), // key:username value:count
	}

	pool := sync2.NewPool(context.Background())
	pool.SubmitTask("heartbeat", func(ctx context.Context) (err error) {
		Hub.heartbeat()
		return nil
	})
	pool.SubmitTasks("slowQueue", conf.Conf.Server.SlowQueueThreadNum, func(ctx context.Context) (err error) {
		Hub.slowQueueHandler(ctx)
		return nil
	})
	pool.SubmitTasks("normalQueue", conf.Conf.Server.NormalQueueThreadNum, func(ctx context.Context) (err error) {
		Hub.normalQueueHandler(ctx)
		return nil
	})
	pool.SubmitTasks("fastQueue", conf.Conf.Server.FastQueueThreadNum, func(ctx context.Context) (err error) {
		Hub.fastQueueHandler(ctx)
		return nil
	})
	pool.SubmitTask("monitorFastQueue", func(ctx context.Context) (err error) {
		// 监控紧急队列以调整一般队列的限速
		ticker := time.NewTicker(time.Millisecond * 100)
		defer ticker.Stop()
		l := 0
		for range ticker.C {
			l = len(Hub.fastQueue)
			if l != 0 {
				common.Log.Debug("fastQueue length: %d", l)
				util.NormalQueueLimiter.SetLimit(rate.Every(time.Second / time.Duration(util.FastNormalBandwidth)))
			} else {
				util.NormalQueueLimiter.SetLimit(rate.Every(time.Second / time.Duration(util.NormalBandwidth)))
			}
		}
		return nil
	})
}

func (h *webSocketHub) heartbeat() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// h.masters.Range(func(key, value any) bool {
		//	master := value.(*activeMaster)
		//	h.sendMessage(&Message{Conn: master.Conn, Data: []byte("heartbeat")})
		//	return true
		// })
		// h.clients.Range(func(key, value any) bool {
		//	client := value.(*activeClient)
		//	h.sendMessage(&Message{Conn: client.Conn, Data: []byte("heartbeat")})
		//	return true
		// })
		common.Log.Info("active master: %d active client: %d", atomic.LoadInt64(&h.masterNum), atomic.LoadInt64(&h.clientNum))
	}
}

type Message struct {
	Conn     *websocket.Conn
	Data     []byte
	Priority MessagePriority
	Delay    time.Duration
}

type MessagePriority int

const (
	NoPriorityMessage MessagePriority = iota
	NormalMessage
	FastMessage
	SlowMessage
)

func (p MessagePriority) String() string {
	return [...]string{"noPriority", "normal", "fast", "slow"}[p]
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
	count, _ := h.localOnlineUsernames.Get(userInfo.UserName)
	h.localOnlineUsernames.Set(userInfo.UserName, count+1)
	atomic.AddInt64(&h.clientNum, 1)

	if count < 1 {
		util.PostMessageToMaster(conf.Conf.Server.AdminKey, "join", userInfo.UserName)
	}
	// 用户连接数过多则关闭最早的连接
	if count > conf.Conf.Server.SessionMaxConnection {
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
		count, _ := h.localOnlineUsernames.Get(userInfo.UserName)
		h.localOnlineUsernames.Set(userInfo.UserName, count-1)
		atomic.AddInt64(&h.clientNum, -1)
		if count < 1 {
			util.PostMessageToMaster(conf.Conf.Server.AdminKey, "leave", userInfo.UserName)
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
			common.Log.Error("send message failed with %s : %s", err, string(message.Data))
			h.ClientUnregister(message.Conn)
			return
		}
	})
}

func (h *webSocketHub) submitMessage(message *Message) {
	switch message.Priority {
	case SlowMessage:
		h.slowQueue <- message
	case NormalMessage:
		h.normalQueue <- message
	case FastMessage:
		h.fastQueue <- message
	default:
		h.sendMessage(message)
	}
}

func (h *webSocketHub) slowQueueHandler(ctx context.Context) {
	for message := range h.slowQueue {
		l := len(message.Data)
		err := util.SlowQueueLimiter.WaitN(ctx, l)
		if err != nil {
			common.Log.Error("slowQueueLimiter waitN %d failed: %s", l, err)
		}
		h.sendMessage(message)
	}
}

func (h *webSocketHub) normalQueueHandler(ctx context.Context) {
	for message := range h.normalQueue {
		l := len(message.Data)
		err := util.NormalQueueLimiter.WaitN(ctx, l)
		if err != nil {
			common.Log.Error("normalQueueLimiter waitN %d failed: %s", l, err)
		}
		h.sendMessage(message)
	}
}

func (h *webSocketHub) fastQueueHandler(ctx context.Context) {
	for message := range h.fastQueue {
		l := len(message.Data)
		err := util.FastQueueLimiter.WaitN(ctx, l)
		if err != nil {
			common.Log.Error("fastQueueLimiter waitN %d failed: %s", l, err)
		}
		h.sendMessage(message)
	}
}

func (h *webSocketHub) HandleMasterMessage(message *Message) {
	msg := string(message.Data)
	if strings.Contains(msg, ":::") {
		split := strings.SplitN(msg, ":::", 2)
		if len(split) == 2 && split[0] == conf.Conf.Server.AdminKey {
			command := split[1]
			if command == "hello" {
				common.Log.Info("[hello] from master %s", message.Conn.RemoteAddr().String())
				h.submitMessage(&Message{Conn: message.Conn, Data: []byte("hello from rhyus-golang")})
			} else if strings.HasPrefix(command, "tell") {
				// 发送文本给指定用户
				to := strings.Split(command, " ")[1]
				content := strings.ReplaceAll(command, "tell "+to+" ", "")
				common.Log.Info("[tell] to %s: %s", to, content)
				h.clients.Range(func(key, value any) bool {
					client := value.(*activeClient)
					if client.UserInfo.UserName == to {
						h.submitMessage(&Message{Conn: client.Conn, Data: []byte(content)})
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
						h.submitMessage(&Message{Conn: client.Conn, Data: []byte(content)})
						num++
					}
					return true
				})

				h.clients.Range(func(key, value any) bool {
					client := value.(*activeClient)
					if client.UserInfo.UserName != sender {
						h.submitMessage(&Message{Conn: client.Conn, Data: []byte(content), Priority: NormalMessage, Delay: 10 * time.Millisecond})
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
					h.submitMessage(&Message{Conn: client.Conn, Data: []byte(content), Priority: FastMessage, Delay: 10 * time.Millisecond})
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
					h.submitMessage(&Message{Conn: client.Conn, Data: []byte(content), Priority: SlowMessage, Delay: 100 * time.Millisecond})
					num++
					return true
				})
				common.Log.Info("[slow] --> %d clients : %s", num, content)
			} else if command == "online" {
				onlineUsers := make([]*model.UserInfo, 0)
				h.clients.Range(func(key, value any) bool {
					client := value.(*activeClient)
					onlineUsers = append(onlineUsers, client.UserInfo)
					return true
				})
				result, err := json.Marshal(onlineUsers)
				if err != nil {
					common.Log.Error("marshal online users failed: %s", err)
				}
				common.Log.Info("[online]: number %d list %s", len(onlineUsers), string(result))
				if len(onlineUsers) == 0 {
					result = []byte("[]")
				}
				common.Log.Debug(" ---> master %s: %s", message.Conn.RemoteAddr().String(), string(message.Data))
				h.submitMessage(&Message{Conn: message.Conn, Data: result})
			} else if strings.HasPrefix(command, "push") {
				content := strings.ReplaceAll(command, "push ", "")
				common.Log.Info("[push]: %s", content)
				h.AllOnlineUsers = content
				common.Log.Debug(" ---> master %s: %s", message.Conn.RemoteAddr().String(), string(message.Data))
				h.submitMessage(&Message{Conn: message.Conn, Data: []byte("OK")})
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
						common.Log.Debug("clear: %s", client.UserInfo.UserName)
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
				common.Log.Debug(" ---> master %s: %s", message.Conn.RemoteAddr().String(), result)
				h.submitMessage(&Message{Conn: message.Conn, Data: result})
			}
		}
	}
}
