package util

import (
	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/time/rate"
	"math"
	"rhyus-golang/conf"
	"time"
)

var GlobalLimiter *rate.Limiter
var visitors *lru.Cache[string, *rate.Limiter]

func GetApiKeyLimiter(apikey string) *rate.Limiter {

	limiter, ok := visitors.Get(apikey)
	if ok {
		return limiter
	} else {
		limiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(conf.Conf.Server.SessionApikeyLimiter)), conf.Conf.Server.SessionApikeyLimiter)
		visitors.Add(apikey, limiter)
		return limiter
	}
}

var NormalBandwidth float64
var FastNormalBandwidth float64
var slowBandwidth float64
var NormalQueueLimiter *rate.Limiter
var FastQueueLimiter *rate.Limiter
var SlowQueueLimiter *rate.Limiter

func InitLimiter() {
	GlobalLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(conf.Conf.Server.SessionGlobalLimiter)), conf.Conf.Server.SessionGlobalLimiter)
	visitors, _ = lru.New[string, *rate.Limiter](conf.Conf.Server.SessionApikeyLimiterCacheSize)

	NormalBandwidth = math.Round(float64(conf.Conf.Server.MaxBandwidth*1024) * 0.95)
	FastNormalBandwidth = math.Round(float64(conf.Conf.Server.MaxBandwidth*1024) * 0.95 * 0.1)
	slowBandwidth = math.Round(float64(conf.Conf.Server.MaxBandwidth*1024) * 0.05)
	NormalQueueLimiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(NormalBandwidth)), int(NormalBandwidth))
	FastQueueLimiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(NormalBandwidth)), int(NormalBandwidth))
	SlowQueueLimiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(slowBandwidth)), int(slowBandwidth))
}
