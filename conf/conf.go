package conf

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"rhyus-golang/common"
	"runtime"
	"sync"
)

type AppConf struct {
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Pprof     *Pprof `json:"pprof,omitempty"`
	Ssl       *Ssl   `json:"ssl,omitempty"`
	MasterUrl string `json:"masterUrl,omitempty"`
	AdminKey  string `json:"adminKey,omitempty"`

	SessionMaxConnection int    `json:"sessionMaxConnection,omitempty"`
	GoMaxProcs           int    `json:"goMaxProcs,omitempty"`
	LogLevel             string `json:"logLevel,omitempty"`

	m *sync.Mutex
}

type Ssl struct {
	Enabled  bool   `json:"enabled,omitempty"`
	CertPath string `json:"certPath,omitempty"`
	KeyPath  string `json:"keyPath,omitempty"`
}

type Pprof struct {
	Enable     bool `json:"enable,omitempty"`
	PporfPort  int  `json:"pporfPort,omitempty"`
	MaxFile    int  `json:"maxFile,omitempty"`
	SampleTime int  `json:"sampleTime,omitempty"`
}

var Conf *AppConf
var ConfigPath string

func init() {
	workSpace, err := os.Getwd()
	if err != nil {
		common.Log.Fatal(common.ExitCodeFileSysErr, "get current directory failed: %s", err)
	}
	host := flag.String("host", "0.0.0.0", "host of the HTTP server")
	port := flag.Int("port", 10831, "port of the HTTP server")
	pprof := flag.Bool("pprof", false, "enable pprof")
	pprofPort := flag.Int("pprofPort", 10832, "port of the pprof server")
	maxFile := flag.Int("maxFile", 6, "max file number of pprof")
	sampleTime := flag.Int("sampleTime", 10, "sample time of pprof")
	ssl := flag.Bool("ssl", false, "enable SSL")
	certPath := flag.String("certPath", "", "path of SSL certificate")
	keyPath := flag.String("keyPath", "", "path of SSL key")
	masterUrl := flag.String("masterUrl", "https://fishpi.cn", "master server URL")
	adminKey := flag.String("adminKey", "", "admin key")

	sessionMaxConnection := flag.Int("sessionMaxConnection", 10, "session max connection")
	goMaxProcs := flag.Int("goMaxProcs", runtime.NumCPU(), "go max procs")
	logLevel := flag.String("logLevel", "info", "log level")
	flag.Parse()

	Conf = &AppConf{
		LogLevel: *logLevel,
		Host:     *host,
		Port:     *port,
		Pprof: &Pprof{
			Enable:     *pprof,
			PporfPort:  *pprofPort,
			MaxFile:    *maxFile,
			SampleTime: *sampleTime,
		},
		Ssl: &Ssl{
			Enabled:  *ssl,
			CertPath: *certPath,
			KeyPath:  *keyPath,
		},
		MasterUrl: *masterUrl,
		AdminKey:  *adminKey,

		SessionMaxConnection: *sessionMaxConnection,
		GoMaxProcs:           *goMaxProcs,
		m:                    &sync.Mutex{},
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
	runtime.GOMAXPROCS(Conf.GoMaxProcs)
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
