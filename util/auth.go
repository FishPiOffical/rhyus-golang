package util

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"rhyus-golang/model"
)

func GetApiKey(c *gin.Context) string {
	apiKey := c.Request.URL.Query().Get("apiKey")
	return apiKey
}

func GetUserInfo(apiKey string) *model.UserInfo {
	var result model.Result[model.UserInfo]
	resp, err := http.Get(conf.Conf.MasterUrl + "/api/user?apiKey=" + apiKey)
	if err != nil {
		common.Log.Error("get user info failed: %s", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		common.Log.Info("Request failed with status: %s", resp.Status)
		return nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		common.Log.Error("read response failed: %s", err)
		return nil
	}
	common.Log.Debug("get user info: %s", string(data))

	err = json.Unmarshal(data, &result)
	if err != nil {
		common.Log.Error("parse response failed: %s", err)
		return nil
	}
	if result.Code != 0 {
		common.Log.Error("get user info failed: %s", result.Msg)
		return nil
	}
	return &result.Data
}

type chatroomNodePush struct {
	Msg      string `json:"msg,omitempty"`
	Data     string `json:"data,omitempty"`
	AdminKey string `json:"adminKey,omitempty"`
}

func PostMessageToMaster(adminKey string, msg string, data string) {
	requestData := chatroomNodePush{
		Msg:      msg,
		Data:     data,
		AdminKey: conf.Conf.AdminKey,
	}
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		common.Log.Error("parse response failed: %s", err)
		return
	}
	resp, err := http.Post(conf.Conf.MasterUrl+"/chat-room/node/push", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		common.Log.Error("post message to master failed: %s", err)
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		common.Log.Info("Request failed with status: %s", resp.Status)
		return
	}
}
