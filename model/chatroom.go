package model

// ClientType 客户端类型
type ClientType string

const (
	Web         ClientType = "Web"         // 网页端
	PC          ClientType = "PC"          // 个人电脑端
	Mobile      ClientType = "Mobile"      // 移动设备端
	Windows     ClientType = "Windows"     // Windows 系统客户端
	MacOS       ClientType = "macOS"       // macOS 客户端
	IOS         ClientType = "iOS"         // iOS 系统客户端
	Android     ClientType = "Android"     // Android 系统客户端
	IDEA        ClientType = "IDEA"        // IntelliJ IDEA 编辑器客户端
	Chrome      ClientType = "Chrome"      // Chrome 浏览器
	Edge        ClientType = "Edge"        // Edge 浏览器
	VSCode      ClientType = "VSCode"      // Visual Studio Code 编辑器
	Python      ClientType = "Python"      // Python 客户端
	Golang      ClientType = "Golang"      // Go 语言客户端
	IceNet      ClientType = "IceNet"      // IceNet 客户端
	ElvesOnline ClientType = "ElvesOnline" // ElvesOnline 客户端
	Dart        ClientType = "Dart"        // Dart 语言客户端
	Bird        ClientType = "Bird"        // Bird 客户端
	Other       ClientType = "Other"       // 其他类型客户端
)

// ChatSource 消息来源
type ChatSource struct {
	Client  string `json:"client,omitempty"`  // 客户端类型
	Version string `json:"version,omitempty"` // 客户端版本
}

// ChatRoomMessage 聊天室消息
type ChatRoomMessage struct {
	OID       string            `json:"oid,omitempty"`          // 消息 ID
	UserOID   int               `json:"userOid,omitempty"`      // 用户 ID
	UserName  string            `json:"userName,omitempty"`     // 用户名
	Nickname  string            `json:"userNickname,omitempty"` // 昵称
	AvatarURL string            `json:"avatar_url,omitempty"`   // 用户头像 URL
	SysMetal  []MetalItem       `json:"sys_metal,omitempty"`    // 系统金属项
	Via       ChatSource        `json:"via"`                    // 客户端和版本信息
	Content   string            `json:"content,omitempty"`      // 消息内容
	MD        string            `json:"md,omitempty"`           // 消息内容的 Markdown 格式
	RedPacket *RedPacketMessage `json:"red_packet,omitempty"`   // 红包消息（如果有）
	Weather   *WeatherMsg       `json:"weather,omitempty"`      // 天气消息（如果有）
	Music     *MusicMsg         `json:"music,omitempty"`        // 音乐消息（如果有）
	Unknown   any               `json:"unknown,omitempty"`      // 其他未分类的信息
	Time      string            `json:"time,omitempty"`         // 时间戳
	Type      string            `json:"type,omitempty"`         // 消息类型
}

func NewChatRoomMessage() *ChatRoomMessage {
	return &ChatRoomMessage{
		OID:       "",
		UserOID:   0,
		UserName:  "",
		Nickname:  "",
		AvatarURL: "",
		SysMetal:  []MetalItem{},
		Via:       ChatSource{},
		Content:   "",
		MD:        "",
		RedPacket: nil,
		Weather:   nil,
		Music:     nil,
		Unknown:   nil,
		Time:      "",
		Type:      "ChatRoomMessageTypeMsg",
	}
}

// ChatContentType 历史消息类型
type ChatContentType string

const (
	Markdown ChatContentType = "md"   // Markdown 格式
	HTML     ChatContentType = "html" // HTML 格式
)

// 相关消息位置类型
type ChatMessageType int

const (
	Context ChatMessageType = iota
	Before
	After
)

// ChatRoomMessageType 聊天室消息类型
type ChatRoomMessageType string

const (
	Online          ChatRoomMessageType = "online"
	DiscussChanged  ChatRoomMessageType = "discussChanged"
	Revoke          ChatRoomMessageType = "revoke"
	Msg             ChatRoomMessageType = "msg"
	RedPacket       ChatRoomMessageType = "redPacket"
	RedPacketStatus ChatRoomMessageType = "redPacketStatus"
	Barrager        ChatRoomMessageType = "barrager"
	Custom          ChatRoomMessageType = "customMessage"
	Weather         ChatRoomMessageType = "weather"
	Music           ChatRoomMessageType = "music"
)

// 聊天室数据
type ChatRoomData struct {
	Type     string              `json:"type,omitempty"`     // 消息类型
	Online   *OnlineMsg          `json:"online,omitempty"`   // 在线消息
	Discuss  *DiscussMsg         `json:"discuss,omitempty"`  // 讨论组消息
	Revoke   *RevokeMsg          `json:"revoke,omitempty"`   // 撤回消息
	Msg      *ChatRoomMessage    `json:"msg,omitempty"`      // 普通消息
	Status   *RedPacketStatusMsg `json:"status,omitempty"`   // 红包状态消息
	Barrager *BarragerMsg        `json:"barrager,omitempty"` // 弹幕消息
	Custom   *CustomMsg          `json:"custom,omitempty"`   // 自定义消息
	Unknown  interface{}         `json:"unknown,omitempty"`  // 未知类型消息
}

func NewChatRoomData() *ChatRoomData {
	return &ChatRoomData{
		Type: "msg",
	}
}

// 相关类型
type MetalItem struct{}
type RedPacketMessage struct{}
type WeatherMsg struct{}
type MusicMsg struct{}
type OnlineInfo struct{}
type RedPacketStatusMsg struct{}
type BarragerMsg struct{}
type CustomMsg string
type OnlineMsg []OnlineInfo
type DiscussMsg string
type RevokeMsg string
type ChatRoomNode struct {
	Node, Name string
	Online     int
}

// 聊天室节点
type ChatRoomNodeInfo struct {
	Recommend ChatRoomNode
	Available ChatRoomNode
}
