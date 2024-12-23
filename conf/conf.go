package conf

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"rhyus-golang/common"
	"strconv"
	"sync"
)

type AppConf struct {
	Host                   string `json:"host,omitempty"`
	Port                   int    `json:"port,omitempty"`
	PprofPort              int    `json:"pprofPort,omitempty"`
	Ssl                    *Ssl   `json:"ssl,omitempty"`
	MasterUrl              string `json:"masterUrl,omitempty"`
	AdminKey               string `json:"adminKey,omitempty"`
	MasterNodeCacheSize    int    `json:"masterNodeCacheSize,omitempty"`
	ClientNodeCacheSize    int    `json:"clientNodeCacheSize,omitempty"`
	MasterMessageCacheSize int    `json:"masterMessageCacheSize,omitempty"`
	ClientMessageCacheSize int    `json:"clientMessageCacheSize,omitempty"`
	LogLevel               string `json:"logLevel,omitempty"`

	m *sync.Mutex
}

type Ssl struct {
	Enabled  bool   `json:"enabled,omitempty"`
	CertPath string `json:"cert_path,omitempty"`
	KeyPath  string `json:"key_path,omitempty"`
}

var Conf *AppConf
var ConfigPath string

func init() {
	workSpace, err := os.Getwd()
	if err != nil {
		common.Log.Fatal(common.ExitCodeFileSysErr, "get current directory failed: %s", err)
	}
	host := flag.String("host", "0.0.0.0", "host of the HTTP server")
	portStr := flag.String("port", "10831", "port of the HTTP server")
	ssl := flag.Bool("ssl", false, "enable SSL")
	certPath := flag.String("cert-path", "", "path of SSL certificate")
	keyPath := flag.String("key-path", "", "path of SSL key")
	masterUrl := flag.String("master-url", "https://fishpi.cn", "master server URL")
	adminKey := flag.String("admin-key", "", "admin key")
	masterNodeCacheSize := flag.Int("master-node-cache-size", 8, "master node cache size")
	clientNodeCacheSize := flag.Int("client-node-cache-size", 64, "client node cache size")
	masterMessageCacheSize := flag.Int("master-message-cache-size", 64, "master message cache size")
	clientMessageCacheSize := flag.Int("client-message-cache-size", 1024, "client message cache size")
	logLevel := flag.String("log-level", "info", "log level")
	flag.Parse()

	port, err := strconv.Atoi(*portStr)
	if err != nil {
		common.Log.Fatal(common.ExitCodeFileSysErr, "invalid port: %s", *portStr)
	}
	Conf = &AppConf{
		LogLevel: *logLevel,
		Host:     *host,
		Port:     port,
		Ssl: &Ssl{
			Enabled:  *ssl,
			CertPath: *certPath,
			KeyPath:  *keyPath,
		},
		MasterUrl:              *masterUrl,
		AdminKey:               *adminKey,
		MasterNodeCacheSize:    *masterNodeCacheSize,
		ClientNodeCacheSize:    *clientNodeCacheSize,
		MasterMessageCacheSize: *masterMessageCacheSize,
		ClientMessageCacheSize: *clientMessageCacheSize,
		m:                      &sync.Mutex{},
	}
	ConfigPath = filepath.Join(workSpace, "conf.json")
	if common.File.IsExist(ConfigPath) {
		if data, err := os.ReadFile(ConfigPath); nil != err {
			common.Log.Error("load conf [%s] failed: %s", ConfigPath, err)
		} else {
			if err = json.Unmarshal(data, Conf); err != nil {
				common.Log.Error("parse conf [%s] failed: %s", ConfigPath, err)
			} else {
				common.Log.Info("loaded conf [%s]", ConfigPath)
			}
		}
	}

	common.Log.SetLogLevel(Conf.LogLevel)
	common.Log.Info("log level: [%s] save path: [%s]", Conf.LogLevel, common.LogPath)
	Conf.Save()
}

func (conf *AppConf) Save() {
	conf.m.Lock()
	defer conf.m.Unlock()

	data, _ := json.MarshalIndent(conf, "", "  ")
	if err := common.File.WriteFileSafer(ConfigPath, data, 0644); nil != err {
		common.Log.Error("write conf [%s] failed: %s", ConfigPath, err)
		return
	}
}

func (conf *AppConf) Close() {
	conf.Save()
}
