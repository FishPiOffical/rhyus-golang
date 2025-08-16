package conf

import (
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"path/filepath"
)

var Conf *Config

type Config struct {
	Server struct {
		Host  string `yaml:"host"`
		Port  int    `yaml:"port"`
		Pprof struct {
			Enable     bool `yaml:"enable"`
			PporfPort  int  `yaml:"pporfPort"`
			MaxFile    int  `yaml:"maxFile"`
			SampleTime int  `yaml:"sampleTime"`
		} `yaml:"pprof"`
		Ssl struct {
			Enabled  bool   `yaml:"enabled"`
			CertPath string `yaml:"certPath"`
			KeyPath  string `yaml:"keyPath"`
		} `yaml:"ssl"`

		MasterUrl string `yaml:"masterUrl"`
		AdminKey  string `yaml:"adminKey"`

		SessionMaxConnection          int64 `yaml:"sessionMaxConnection"`
		SessionApikeyLimiter          int   `yaml:"sessionApikeyLimiter"`
		SessionApikeyLimiterCacheSize int   `yaml:"sessionApikeyLimiterCacheSize"`
		SessionGlobalLimiter          int   `yaml:"sessionGlobalLimiter"`
		KeepaliveTime                 int64 `yaml:"keepaliveTime"`

		FastQueueThreadNum   int `yaml:"fastQueueThreadNum"`
		NormalQueueThreadNum int `yaml:"normalQueueThreadNum"`
		SlowQueueThreadNum   int `yaml:"slowQueueThreadNum"`

		MaxBandwidth int    `yaml:"maxBandwidth"`
		GoMaxProcs   int    `yaml:"goMaxProcs"`
		LogLevel     string `yaml:"logLevel"`

		TimewheelInterval int `yaml:"timewheelInterval"`
		TimewheelSlotNums int `yaml:"timewheelSlotNums"`
		PoolSize          int `yaml:"poolSize"`
		MaxPoolSize       int `yaml:"maxPoolSize"`
	} `yaml:"server"`
}

func InitConfig() {
	Conf = &Config{}
	yamlPath := filepath.Join("config.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		log.Fatalf("failed to load config: %v", err)
	} else {
		Conf.loadYAMLConfig(yamlPath)
	}
}

// 加载 YAML 配置文件
func (c *Config) loadYAMLConfig(path string) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Error opening YAML file: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("Error closing YAML file: %v", err)
		}
	}(file)

	// 读取文件内容
	data, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		log.Fatalf("Error parsing YAML file: %v", err)
	}
}
