package model

// MetalAttr 徽章属性
type MetalAttr struct {
	URL       string `json:"url,omitempty"`
	BackColor string `json:"backcolor,omitempty"`
	FontColor string `json:"fontcolor,omitempty"`
}

// MetalBase 徽章基础信息
type MetalBase struct {
	Attr        MetalAttr `json:"attr,omitempty"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	Data        string    `json:"data,omitempty"`
}

func NewMetalBase(attr MetalAttr, name, description, data string) *MetalBase {
	return &MetalBase{
		Attr:        attr,
		Name:        name,
		Description: description,
		Data:        data,
	}
}

// Metal 徽章信息
type Metal struct {
	MetalBase
	Enable *string `json:"enable,omitempty"`
}

// MetalList 徽章列表
type MetalList []Metal

// UserAppRole 应用角色
type UserAppRole string

const (
	Hack   UserAppRole = "0"
	Artist UserAppRole = "1"
)

// UserInfo 用户信息
type UserInfo struct {
	OId          string      `json:"oId,omitempty"`
	UserNo       string      `json:"userNo,omitempty"`
	UserName     string      `json:"userName,omitempty"`
	NickName     string      `json:"userNickname,omitempty"`
	UserURL      string      `json:"userURL,omitempty"`
	City         string      `json:"userCity,omitempty"`
	Intro        string      `json:"userIntro,omitempty"`
	IsOnline     bool        `json:"userOnlineFlag,omitempty"`
	Point        int         `json:"userPoint,omitempty"`
	Role         string      `json:"userRole,omitempty"`
	AppRole      UserAppRole `json:"userAppRole,omitempty"`
	AvatarURL    string      `json:"userAvatarURL,omitempty"`
	CardBg       string      `json:"cardBg,omitempty"`
	FollowingCnt int         `json:"followingUserCount,omitempty"`
	FollowerCnt  int         `json:"followerCount,omitempty"`
	OnlineMinute int         `json:"onlineMinute,omitempty"`
	CanFollow    string      `json:"canFollow,omitempty"`
	AllMetals    string      `json:"allMetalOwned,omitempty"`
	SysMetals    string      `json:"sysMetal,omitempty"`
}
